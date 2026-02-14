package runpack

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

type ReplayMode string

const (
	ReplayModeStub ReplayMode = "stub"
	ReplayModeReal ReplayMode = "real"
)

type ReplayStep struct {
	IntentID     string `json:"intent_id"`
	ToolName     string `json:"tool_name"`
	Status       string `json:"status"`
	Execution    string `json:"execution,omitempty"`
	StubType     string `json:"stub_type,omitempty"`
	ResultDigest string `json:"result_digest,omitempty"`
}

type ReplayResult struct {
	RunID          string       `json:"run_id"`
	Mode           ReplayMode   `json:"mode"`
	Steps          []ReplayStep `json:"steps"`
	MissingResults []string     `json:"missing_results,omitempty"`
}

type RealReplayOptions struct {
	AllowTools []string
}

func ReplayStub(path string) (ReplayResult, error) {
	return replay(path, ReplayModeStub, nil)
}

func ReplayReal(path string, options RealReplayOptions) (ReplayResult, error) {
	allowSet := make(map[string]struct{}, len(options.AllowTools))
	for _, tool := range options.AllowTools {
		normalized := strings.ToLower(strings.TrimSpace(tool))
		if normalized == "" {
			continue
		}
		allowSet[normalized] = struct{}{}
	}
	if len(allowSet) == 0 {
		return ReplayResult{}, fmt.Errorf("real replay requires non-empty allowlist")
	}
	return replay(path, ReplayModeReal, allowSet)
}

func replay(path string, mode ReplayMode, allowSet map[string]struct{}) (ReplayResult, error) {
	pack, err := ReadRunpack(path)
	if err != nil {
		return ReplayResult{}, err
	}
	seenIntents := make(map[string]struct{}, len(pack.Intents))
	for _, intent := range pack.Intents {
		if _, exists := seenIntents[intent.IntentID]; exists {
			return ReplayResult{}, fmt.Errorf("duplicate intent_id: %s", intent.IntentID)
		}
		seenIntents[intent.IntentID] = struct{}{}
	}
	resultsByIntent := make(map[string]ReplayStep, len(pack.Results))
	for _, result := range pack.Results {
		if _, exists := resultsByIntent[result.IntentID]; exists {
			return ReplayResult{}, fmt.Errorf("duplicate result for intent_id: %s", result.IntentID)
		}
		resultsByIntent[result.IntentID] = ReplayStep{
			IntentID:     result.IntentID,
			ToolName:     "",
			Status:       result.Status,
			ResultDigest: result.ResultDigest,
			Execution:    "recorded",
		}
	}

	steps := make([]ReplayStep, 0, len(pack.Intents))
	missing := make([]string, 0)
	for _, intent := range pack.Intents {
		step := ReplayStep{
			IntentID: intent.IntentID,
			ToolName: intent.ToolName,
		}
		if mode == ReplayModeReal && isAllowedRealTool(allowSet, intent.ToolName) {
			resultDigest, status, execErr := executeRealTool(intent)
			if execErr == nil {
				step.Status = status
				step.ResultDigest = resultDigest
				step.Execution = "executed"
				steps = append(steps, step)
				continue
			}
			step.Status = "error"
			step.ResultDigest = digestString(execErr.Error())
			step.Execution = "executed"
			steps = append(steps, step)
			continue
		}
		if recorded, ok := resultsByIntent[intent.IntentID]; ok {
			step.Status = recorded.Status
			step.ResultDigest = recorded.ResultDigest
			step.Execution = "recorded"
			steps = append(steps, step)
			continue
		}
		stubType := classifyStubType(intent.ToolName)
		if stubType == "" {
			step.Status = "missing_result"
			step.Execution = "missing"
			missing = append(missing, intent.IntentID)
		} else {
			step.Status = "stubbed"
			step.Execution = "stubbed"
			step.StubType = stubType
			step.ResultDigest = stubDigest(pack.Run.RunID, intent.IntentID, intent.ToolName, intent.ArgsDigest, stubType)
		}
		steps = append(steps, step)
	}
	sort.Strings(missing)

	return ReplayResult{
		RunID:          pack.Run.RunID,
		Mode:           mode,
		Steps:          steps,
		MissingResults: missing,
	}, nil
}

func isAllowedRealTool(allowSet map[string]struct{}, toolName string) bool {
	if len(allowSet) == 0 {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(toolName))
	_, ok := allowSet[normalized]
	return ok
}

func executeRealTool(intent schemarunpack.IntentRecord) (string, string, error) {
	tool := strings.ToLower(strings.TrimSpace(intent.ToolName))
	switch tool {
	case "echo", "tool.echo":
		message := readStringArg(intent.Args, "message")
		if strings.TrimSpace(message) == "" {
			message = "echo"
		}
		return digestString(message), "ok", nil
	case "file.write", "fs.write", "write_file":
		path := strings.TrimSpace(readStringArg(intent.Args, "path"))
		content := readStringArg(intent.Args, "content")
		if path == "" {
			return "", "error", fmt.Errorf("file.write requires args.path")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return "", "error", err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return "", "error", err
		}
		return digestString(content), "ok", nil
	case "shell.exec", "exec", "command.run":
		command := strings.TrimSpace(readStringArg(intent.Args, "command"))
		if command == "" {
			return "", "error", fmt.Errorf("shell.exec requires args.command")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "sh", "-lc", command) // #nosec G204 -- explicit unsafe replay path.
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		err := cmd.Run()
		trimmedOutput := output.String()
		if len(trimmedOutput) > 64*1024 {
			trimmedOutput = trimmedOutput[:64*1024]
		}
		digest := digestString(trimmedOutput)
		if err != nil {
			return digest, "error", err
		}
		return digest, "ok", nil
	default:
		return "", "error", fmt.Errorf("unsupported real tool: %s", intent.ToolName)
	}
}

func readStringArg(args map[string]any, key string) string {
	if len(args) == 0 {
		return ""
	}
	value, ok := args[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func digestString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func classifyStubType(toolName string) string {
	name := strings.ToLower(strings.TrimSpace(toolName))
	switch {
	case strings.Contains(name, "http"), strings.Contains(name, "fetch"), strings.Contains(name, "url"):
		return "http"
	case strings.Contains(name, "file"), strings.Contains(name, "path"), strings.Contains(name, "fs"), strings.Contains(name, "write"):
		return "file"
	case strings.Contains(name, "db"), strings.Contains(name, "sql"), strings.Contains(name, "query"), strings.Contains(name, "table"):
		return "db"
	case strings.Contains(name, "queue"), strings.Contains(name, "topic"), strings.Contains(name, "publish"), strings.Contains(name, "kafka"):
		return "queue"
	default:
		return ""
	}
}

func stubDigest(runID string, intentID string, toolName string, argsDigest string, stubType string) string {
	sum := sha256.Sum256([]byte(runID + ":" + intentID + ":" + strings.ToLower(strings.TrimSpace(toolName)) + ":" + strings.ToLower(strings.TrimSpace(argsDigest)) + ":" + stubType))
	return hex.EncodeToString(sum[:])
}
