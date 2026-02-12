package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/runpack"
)

type handler struct {
	config Config
}

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
	}

	runpackPath := filepath.Join(workspace, "gait-out", "runpack_run_demo.zip")
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

	regressPath := filepath.Join(workspace, "regress_result.json")
	if fileExists(regressPath) {
		response.RegressResult = regressPath
	}
	junitPath := filepath.Join(workspace, "gait-out", "junit.xml")
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
		runID := strings.TrimSpace(args["run_id"])
		if runID == "" {
			runID = "run_demo"
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
		return runCommandSpec{
			Command: "policy_block_test",
			Argv: []string{
				"gait", "policy", "test",
				"examples/policy/base_high_risk.yaml",
				"examples/policy/intents/intent_delete.json",
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
