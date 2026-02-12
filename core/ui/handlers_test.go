package ui

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNewHandlerRequiresExecutable(t *testing.T) {
	staticHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	_, err := NewHandler(Config{}, staticHandler)
	if err == nil {
		t.Fatalf("expected missing executable path error")
	}
}

func TestHealthAndStateRoutes(t *testing.T) {
	staticHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok"))
	})
	handler, err := NewHandler(Config{
		ExecutablePath: "/tmp/gait",
		WorkDir:        t.TempDir(),
		Runner: func(_ context.Context, _ string, _ []string) (runResult, error) {
			return runResult{}, nil
		},
	}, staticHandler)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	healthRequest := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, healthRequest)
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("health status code: expected %d got %d", http.StatusOK, healthResponse.Code)
	}
	if !strings.Contains(healthResponse.Body.String(), `"service":"gait.ui"`) {
		t.Fatalf("expected health service marker in response")
	}

	stateRequest := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	stateResponse := httptest.NewRecorder()
	handler.ServeHTTP(stateResponse, stateRequest)
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("state status code: expected %d got %d", http.StatusOK, stateResponse.Code)
	}
	if !strings.Contains(stateResponse.Body.String(), `"workspace":`) {
		t.Fatalf("expected workspace in state response")
	}
}

func TestExecRouteValidationAndCommandResolution(t *testing.T) {
	staticHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok"))
	})

	var capturedArgv []string
	handler, err := NewHandler(Config{
		ExecutablePath: "/tmp/gait-binary",
		WorkDir:        t.TempDir(),
		Runner: func(_ context.Context, _ string, argv []string) (runResult, error) {
			capturedArgv = append([]string(nil), argv...)
			return runResult{
				ExitCode: 0,
				Stdout:   `{"ok":true,"verdict":"allow"}`,
				Stderr:   "",
			}, nil
		},
	}, staticHandler)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	unknownRequest := httptest.NewRequest(http.MethodPost, "/api/exec", strings.NewReader(`{"command":"nope"}`))
	unknownRequest.Header.Set("Content-Type", "application/json")
	unknownResponse := httptest.NewRecorder()
	handler.ServeHTTP(unknownResponse, unknownRequest)
	if unknownResponse.Code != http.StatusBadRequest {
		t.Fatalf("unknown command status: expected %d got %d", http.StatusBadRequest, unknownResponse.Code)
	}

	validRequest := httptest.NewRequest(http.MethodPost, "/api/exec", strings.NewReader(`{"command":"verify_demo"}`))
	validRequest.Header.Set("Content-Type", "application/json")
	validResponse := httptest.NewRecorder()
	handler.ServeHTTP(validResponse, validRequest)
	if validResponse.Code != http.StatusOK {
		t.Fatalf("valid command status: expected %d got %d", http.StatusOK, validResponse.Code)
	}
	if len(capturedArgv) == 0 {
		t.Fatalf("expected runner invocation")
	}
	if capturedArgv[0] != "/tmp/gait-binary" {
		t.Fatalf("expected executable substitution, got %s", capturedArgv[0])
	}
	if !strings.Contains(validResponse.Body.String(), `"verdict":"allow"`) {
		t.Fatalf("expected parsed json payload in response")
	}
}

func TestMethodValidation(t *testing.T) {
	t.Parallel()
	handler, err := NewHandler(Config{
		ExecutablePath: "/tmp/gait",
		WorkDir:        t.TempDir(),
		Runner: func(_ context.Context, _ string, _ []string) (runResult, error) {
			return runResult{}, nil
		},
	}, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "health", method: http.MethodPost, path: "/api/health"},
		{name: "state", method: http.MethodPost, path: "/api/state"},
		{name: "exec", method: http.MethodGet, path: "/api/exec"},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status: expected %d got %d", http.StatusMethodNotAllowed, response.Code)
			}
			if !strings.Contains(response.Body.String(), `"ok":false`) {
				t.Fatalf("expected error payload")
			}
		})
	}
}

func TestExecRouteDecodeAndRunnerFailures(t *testing.T) {
	t.Parallel()
	handler, err := NewHandler(Config{
		ExecutablePath: "/tmp/gait",
		WorkDir:        t.TempDir(),
		Runner: func(_ context.Context, _ string, _ []string) (runResult, error) {
			return runResult{}, errors.New("boom")
		},
	}, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/exec", strings.NewReader("{"))
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("status: expected %d got %d", http.StatusBadRequest, invalidResponse.Code)
	}

	runnerFailureRequest := httptest.NewRequest(http.MethodPost, "/api/exec", strings.NewReader(`{"command":"demo"}`))
	runnerFailureResponse := httptest.NewRecorder()
	handler.ServeHTTP(runnerFailureResponse, runnerFailureRequest)
	if runnerFailureResponse.Code != http.StatusInternalServerError {
		t.Fatalf("status: expected %d got %d", http.StatusInternalServerError, runnerFailureResponse.Code)
	}
	if !strings.Contains(runnerFailureResponse.Body.String(), `"exit_code":1`) {
		t.Fatalf("expected internal failure exit code")
	}
}

func TestResolveCommandCoverage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		request  ExecRequest
		command  string
		contains []string
	}{
		{
			request:  ExecRequest{Command: "demo"},
			command:  "demo",
			contains: []string{"demo", "--json"},
		},
		{
			request:  ExecRequest{Command: "receipt_demo"},
			command:  "receipt_demo",
			contains: []string{"run", "receipt", "--from", "run_demo"},
		},
		{
			request:  ExecRequest{Command: "regress_init", Args: map[string]string{"run_id": "run_123"}},
			command:  "regress_init",
			contains: []string{"regress", "init", "--from", "run_123"},
		},
		{
			request:  ExecRequest{Command: "regress_init"},
			command:  "regress_init",
			contains: []string{"--from", "run_demo"},
		},
		{
			request:  ExecRequest{Command: "regress_run"},
			command:  "regress_run",
			contains: []string{"--junit", "./gait-out/junit.xml"},
		},
		{
			request:  ExecRequest{Command: "policy_block_test"},
			command:  "policy_block_test",
			contains: []string{"policy", "test", "examples/policy/intents/intent_delete.json"},
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.command, func(t *testing.T) {
			t.Parallel()
			spec, err := resolveCommand(testCase.request)
			if err != nil {
				t.Fatalf("resolve command: %v", err)
			}
			if spec.Command != testCase.command {
				t.Fatalf("command: expected %s got %s", testCase.command, spec.Command)
			}
			for _, fragment := range testCase.contains {
				if !slices.Contains(spec.Argv, fragment) {
					t.Fatalf("expected argv to contain %q, got %v", fragment, spec.Argv)
				}
			}
		})
	}

	if _, err := resolveCommand(ExecRequest{Command: "unknown"}); err == nil {
		t.Fatalf("expected unsupported command error")
	}
}

func TestSupportFunctions(t *testing.T) {
	t.Parallel()

	argv := withExecutablePath("/tmp/gait", nil)
	if len(argv) != 1 || argv[0] != "/tmp/gait" {
		t.Fatalf("unexpected executable argv: %v", argv)
	}

	workspace := t.TempDir()
	files := []string{
		"trace_01.json",
		"trace_02.json",
		"trace_03.json",
		"trace_04.json",
		"trace_05.json",
		"trace_06.json",
	}
	for _, name := range files {
		path := filepath.Join(workspace, name)
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatalf("write trace: %v", err)
		}
	}

	traces, err := listTraceFiles(workspace)
	if err != nil {
		t.Fatalf("list trace files: %v", err)
	}
	if len(traces) != 5 {
		t.Fatalf("expected latest 5 traces, got %d", len(traces))
	}
	if strings.Contains(traces[0], "trace_01.json") {
		t.Fatalf("expected oldest trace to be trimmed, got %v", traces)
	}

	if _, err := listTraceFiles("["); err == nil {
		t.Fatalf("expected invalid glob pattern error")
	}
}

func TestWriteJSONEncodeFailure(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	writeJSON(response, http.StatusOK, map[string]float64{"value": math.NaN()})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status: expected %d got %d", http.StatusInternalServerError, response.Code)
	}
	if !strings.Contains(response.Body.String(), "encode response") {
		t.Fatalf("expected encoding failure payload")
	}
}

func TestStateRouteArtifacts(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "gait-out"), 0o755); err != nil {
		t.Fatalf("mkdir gait-out: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "gait.yaml"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("write gait.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "trace_a.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write trace_a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "trace_b.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write trace_b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "regress_result.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write regress result: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "gait-out", "junit.xml"), []byte("<testsuite/>"), 0o600); err != nil {
		t.Fatalf("write junit: %v", err)
	}
	// Invalid zip is sufficient for branch coverage of runpack existence without requiring a full artifact.
	if err := os.WriteFile(filepath.Join(workspace, "gait-out", "runpack_run_demo.zip"), []byte("invalid"), 0o600); err != nil {
		t.Fatalf("write invalid runpack: %v", err)
	}

	handler, err := NewHandler(Config{
		ExecutablePath: "/tmp/gait",
		WorkDir:        workspace,
		Runner: func(_ context.Context, _ string, _ []string) (runResult, error) {
			return runResult{}, nil
		},
	}, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("state status code: expected %d got %d", http.StatusOK, response.Code)
	}

	var payload StateResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode state response: %v", err)
	}
	if !payload.GaitConfigExists {
		t.Fatalf("expected gait config marker")
	}
	if payload.RegressResult == "" || payload.JUnitPath == "" {
		t.Fatalf("expected regress and junit paths in state response: %+v", payload)
	}
	if len(payload.TraceFiles) != 2 {
		t.Fatalf("expected trace files in state response: %+v", payload.TraceFiles)
	}
	if payload.RunpackPath != "" {
		t.Fatalf("expected no verified runpack path for invalid artifact")
	}
}
