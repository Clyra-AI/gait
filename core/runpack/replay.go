package runpack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
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
	StubType     string `json:"stub_type,omitempty"`
	ResultDigest string `json:"result_digest,omitempty"`
}

type ReplayResult struct {
	RunID          string       `json:"run_id"`
	Mode           ReplayMode   `json:"mode"`
	Steps          []ReplayStep `json:"steps"`
	MissingResults []string     `json:"missing_results,omitempty"`
}

func ReplayStub(path string) (ReplayResult, error) {
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
		}
	}

	steps := make([]ReplayStep, 0, len(pack.Intents))
	missing := make([]string, 0)
	for _, intent := range pack.Intents {
		step := ReplayStep{
			IntentID: intent.IntentID,
			ToolName: intent.ToolName,
		}
		if recorded, ok := resultsByIntent[intent.IntentID]; ok {
			step.Status = recorded.Status
			step.ResultDigest = recorded.ResultDigest
		} else {
			stubType := classifyStubType(intent.ToolName)
			if stubType == "" {
				step.Status = "missing_result"
				missing = append(missing, intent.IntentID)
			} else {
				step.Status = "stubbed"
				step.StubType = stubType
				step.ResultDigest = stubDigest(pack.Run.RunID, intent.IntentID, intent.ToolName, intent.ArgsDigest, stubType)
			}
		}
		steps = append(steps, step)
	}
	sort.Strings(missing)

	return ReplayResult{
		RunID:          pack.Run.RunID,
		Mode:           ReplayModeStub,
		Steps:          steps,
		MissingResults: missing,
	}, nil
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
