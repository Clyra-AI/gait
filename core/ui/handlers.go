package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/runpack"
)

type handler struct {
	config Config
}

var (
	supportedPolicyPaths = []string{
		"examples/policy/base_high_risk.yaml",
		"examples/policy/base_medium_risk.yaml",
		"examples/policy/base_low_risk.yaml",
	}
	supportedIntentPaths = []string{
		"examples/policy/intents/intent_delete.json",
		"examples/policy/intents/intent_read.json",
		"examples/policy/intents/intent_write.json",
		"examples/policy/intents/intent_tainted_egress.json",
		"examples/policy/intents/intent_delegated_egress_valid.json",
		"examples/policy/intents/intent_delegated_egress_invalid.json",
	}
	defaultPolicyPath  = supportedPolicyPaths[0]
	defaultIntentPath  = supportedIntentPaths[0]
	allowedPolicyPaths = sliceToSet(supportedPolicyPaths)
	allowedIntentPaths = sliceToSet(supportedIntentPaths)
	runIDPattern       = regexp.MustCompile(`^[a-zA-Z0-9._:-]{1,120}$`)
)

func NewHandler(config Config, staticHandler http.Handler) (http.Handler, error) {
	executable := strings.TrimSpace(config.ExecutablePath)
	if executable == "" {
		return nil, fmt.Errorf("missing executable path")
	}
	workDir := strings.TrimSpace(config.WorkDir)
	if workDir == "" {
		workDir = "."
	}
	timeout := config.CommandTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	runner := config.Runner
	if runner == nil {
		runner = defaultRunner
	}
	h := &handler{
		config: Config{
			ExecutablePath: executable,
			WorkDir:        workDir,
			CommandTimeout: timeout,
			Runner:         runner,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", h.handleHealth)
	mux.HandleFunc("/api/state", h.handleState)
	mux.HandleFunc("/api/exec", h.handleExec)
	mux.Handle("/", staticHandler)
	return mux, nil
}

func (handlerValue *handler) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(writer, http.StatusMethodNotAllowed, "expected GET")
		return
	}
	writeJSON(writer, http.StatusOK, HealthResponse{OK: true, Service: "gait.ui"})
}

func (handlerValue *handler) handleState(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(writer, http.StatusMethodNotAllowed, "expected GET")
		return
	}

	workspace, err := filepath.Abs(handlerValue.config.WorkDir)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, StateResponse{OK: false, Error: err.Error()})
		return
	}

	response := StateResponse{
		OK:               true,
		Workspace:        workspace,
		GaitConfigExists: fileExists(filepath.Join(workspace, "gait.yaml")),
		PolicyPaths:      append([]string(nil), supportedPolicyPaths...),
		IntentPaths:      append([]string(nil), supportedIntentPaths...),
		DefaultPolicy:    defaultPolicyPath,
		DefaultIntent:    defaultIntentPath,
	}

	runpackPath := filepath.Join(workspace, "gait-out", "runpack_run_demo.zip")
	regressPath := filepath.Join(workspace, "regress_result.json")
	junitPath := filepath.Join(workspace, "gait-out", "junit.xml")

	response.Artifacts = collectArtifacts([]artifactSpec{
		{Key: "runpack", Path: runpackPath},
		{Key: "regress_result", Path: regressPath},
		{Key: "junit", Path: junitPath},
	})

	if fileExists(runpackPath) {
		verifyResult, verifyErr := runpack.VerifyZip(runpackPath, runpack.VerifyOptions{RequireSignature: false})
		if verifyErr == nil {
			response.RunpackPath = runpackPath
			response.RunID = verifyResult.RunID
			response.ManifestDigest = verifyResult.ManifestDigest
		}
	}

	traceFiles, traceErr := listTraceFiles(workspace)
	if traceErr == nil {
		response.TraceFiles = traceFiles
	}

	if fileExists(regressPath) {
		response.RegressResult = regressPath
	}
	if fileExists(junitPath) {
		response.JUnitPath = junitPath
	}

	writeJSON(writer, http.StatusOK, response)
}

func (handlerValue *handler) handleExec(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "expected POST")
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	payload, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		writeError(writer, http.StatusBadRequest, "read request body")
		return
	}

	var execRequest ExecRequest
	if err := json.Unmarshal(payload, &execRequest); err != nil {
		writeError(writer, http.StatusBadRequest, "decode request JSON")
		return
	}
	execRequest.Command = strings.TrimSpace(execRequest.Command)
	spec, specErr := resolveCommand(execRequest)
	if specErr != nil {
		writeJSON(writer, http.StatusBadRequest, ExecResponse{
			OK:       false,
			Command:  execRequest.Command,
			ExitCode: exitCodeInvalidInput,
			Error:    specErr.Error(),
		})
		return
	}
	spec.Argv = withExecutablePath(handlerValue.config.ExecutablePath, spec.Argv)

	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(request.Context(), handlerValue.config.CommandTimeout)
	defer cancel()

	result, runErr := handlerValue.config.Runner(ctx, handlerValue.config.WorkDir, spec.Argv)
	response := ExecResponse{
		OK:         runErr == nil && result.ExitCode == 0,
		Command:    spec.Command,
		Argv:       append([]string(nil), spec.Argv...),
		ExitCode:   result.ExitCode,
		DurationMS: time.Since(startedAt).Milliseconds(),
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
	}
	if runErr != nil {
		response.OK = false
		response.ExitCode = exitCodeInternalFailure
		response.Error = runErr.Error()
		writeJSON(writer, http.StatusInternalServerError, response)
		return
	}

	trimmedStdout := strings.TrimSpace(result.Stdout)
	if strings.HasPrefix(trimmedStdout, "{") {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(trimmedStdout), &parsed); err == nil {
			response.JSON = parsed
		}
	}
	writeJSON(writer, http.StatusOK, response)
}

func resolveCommand(request ExecRequest) (runCommandSpec, error) {
	args := request.Args
	if args == nil {
		args = map[string]string{}
	}
	switch strings.ToLower(request.Command) {
	case "demo":
		return runCommandSpec{
			Command: "demo",
			Argv:    []string{"gait", "demo", "--json"},
		}, nil
	case "verify_demo":
		return runCommandSpec{
			Command: "verify_demo",
			Argv:    []string{"gait", "verify", "run_demo", "--json"},
		}, nil
	case "receipt_demo":
		return runCommandSpec{
			Command: "receipt_demo",
			Argv:    []string{"gait", "run", "receipt", "--from", "run_demo", "--json"},
		}, nil
	case "regress_init":
		runID, runIDErr := sanitizeRunID(args["run_id"])
		if runIDErr != nil {
			return runCommandSpec{}, runIDErr
		}
		return runCommandSpec{
			Command: "regress_init",
			Argv:    []string{"gait", "regress", "init", "--from", runID, "--json"},
		}, nil
	case "regress_run":
		return runCommandSpec{
			Command: "regress_run",
			Argv:    []string{"gait", "regress", "run", "--json", "--junit", "./gait-out/junit.xml"},
		}, nil
	case "policy_block_test":
		policyPath, policyErr := sanitizePolicyPath(args["policy_path"])
		if policyErr != nil {
			return runCommandSpec{}, policyErr
		}
		intentPath, intentErr := sanitizeIntentPath(args["intent_path"])
		if intentErr != nil {
			return runCommandSpec{}, intentErr
		}
		return runCommandSpec{
			Command: "policy_block_test",
			Argv: []string{
				"gait", "policy", "test",
				policyPath,
				intentPath,
				"--json",
			},
		}, nil
	default:
		return runCommandSpec{}, fmt.Errorf("unsupported command %q", request.Command)
	}
}

func withExecutablePath(executable string, argv []string) []string {
	if len(argv) == 0 {
		return []string{executable}
	}
	result := append([]string(nil), argv...)
	result[0] = executable
	return result
}

func listTraceFiles(workspace string) ([]string, error) {
	pattern := filepath.Join(workspace, "trace_*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	if len(matches) > 5 {
		matches = matches[len(matches)-5:]
	}
	return matches, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

type artifactSpec struct {
	Key  string
	Path string
}

func collectArtifacts(specs []artifactSpec) []ArtifactState {
	artifacts := make([]ArtifactState, 0, len(specs))
	for _, spec := range specs {
		modifiedAt := ""
		exists := false
		if info, err := os.Stat(spec.Path); err == nil && !info.IsDir() {
			exists = true
			modifiedAt = info.ModTime().UTC().Format(time.RFC3339Nano)
		}
		artifacts = append(artifacts, ArtifactState{
			Key:        spec.Key,
			Path:       spec.Path,
			Exists:     exists,
			ModifiedAt: modifiedAt,
		})
	}
	return artifacts
}

func sanitizeRunID(raw string) (string, error) {
	runID := strings.TrimSpace(raw)
	if runID == "" {
		runID = "run_demo"
	}
	if !runIDPattern.MatchString(runID) {
		return "", fmt.Errorf("invalid run_id %q", runID)
	}
	return runID, nil
}

func sanitizePolicyPath(raw string) (string, error) {
	pathValue := normalizePathValue(raw)
	if pathValue == "" {
		return defaultPolicyPath, nil
	}
	if _, ok := allowedPolicyPaths[pathValue]; !ok {
		return "", fmt.Errorf("unsupported policy_path %q", pathValue)
	}
	return pathValue, nil
}

func sanitizeIntentPath(raw string) (string, error) {
	pathValue := normalizePathValue(raw)
	if pathValue == "" {
		return defaultIntentPath, nil
	}
	if _, ok := allowedIntentPaths[pathValue]; !ok {
		return "", fmt.Errorf("unsupported intent_path %q", pathValue)
	}
	return pathValue, nil
}

func normalizePathValue(raw string) string {
	normalized := strings.TrimSpace(raw)
	normalized = strings.ReplaceAll(normalized, `\`, "/")
	return normalized
}

func sliceToSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]any{
		"ok":    false,
		"error": strings.TrimSpace(message),
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		http.Error(writer, `{"ok":false,"error":"encode response"}`, http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write(append(encoded, '\n'))
}

const (
	exitCodeInvalidInput    = 6
	exitCodeInternalFailure = 1
)
