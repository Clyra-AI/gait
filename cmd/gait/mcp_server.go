package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type mcpServeConfig struct {
	PolicyPath     string
	ListenAddr     string
	DefaultAdapter string
	TraceDir       string
	RunpackDir     string
	LogExportPath  string
	OTelExport     string
	KeyMode        string
	PrivateKey     string
	PrivateKeyEnv  string
}

type mcpServeEvaluateRequest struct {
	Adapter     string         `json:"adapter,omitempty"`
	Call        map[string]any `json:"call"`
	RunID       string         `json:"run_id,omitempty"`
	TracePath   string         `json:"trace_path,omitempty"`
	RunpackOut  string         `json:"runpack_out,omitempty"`
	EmitRunpack bool           `json:"emit_runpack,omitempty"`
}

type mcpServeEvaluateResponse struct {
	mcpProxyOutput
	ExitCode int `json:"exit_code"`
}

func runMCPServe(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Run a local interception service that evaluates tool-call payloads through Gate and emits signed traces across JSON, SSE, and streamable HTTP endpoints.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"policy":          true,
		"listen":          true,
		"adapter":         true,
		"trace-dir":       true,
		"runpack-dir":     true,
		"export-log-out":  true,
		"export-otel-out": true,
		"key-mode":        true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("mcp-serve", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var policyPath string
	var listenAddr string
	var adapter string
	var traceDir string
	var runpackDir string
	var logExportPath string
	var otelExportPath string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&policyPath, "policy", "", "path to policy YAML")
	flagSet.StringVar(&listenAddr, "listen", "127.0.0.1:8787", "listen address")
	flagSet.StringVar(&adapter, "adapter", "mcp", "default adapter: mcp|openai|anthropic|langchain")
	flagSet.StringVar(&traceDir, "trace-dir", "./gait-out/mcp-serve/traces", "directory for emitted traces")
	flagSet.StringVar(&runpackDir, "runpack-dir", "", "optional directory for emitted runpacks")
	flagSet.StringVar(&logExportPath, "export-log-out", "", "optional JSONL log export path")
	flagSet.StringVar(&otelExportPath, "export-otel-out", "", "optional OTEL-style JSONL export path")
	flagSet.StringVar(&keyMode, "key-mode", "dev", "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit startup JSON")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printMCPServeUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(policyPath) == "" && len(remaining) > 0 {
		policyPath = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(policyPath) == "" || len(remaining) > 0 {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "expected --policy <policy.yaml>"}, exitInvalidInput)
	}

	config := mcpServeConfig{
		PolicyPath:     policyPath,
		ListenAddr:     strings.TrimSpace(listenAddr),
		DefaultAdapter: strings.ToLower(strings.TrimSpace(adapter)),
		TraceDir:       strings.TrimSpace(traceDir),
		RunpackDir:     strings.TrimSpace(runpackDir),
		LogExportPath:  strings.TrimSpace(logExportPath),
		OTelExport:     strings.TrimSpace(otelExportPath),
		KeyMode:        strings.TrimSpace(keyMode),
		PrivateKey:     strings.TrimSpace(privateKeyPath),
		PrivateKeyEnv:  strings.TrimSpace(privateKeyEnv),
	}
	handler, err := newMCPServeHandler(config)
	if err != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	if jsonOutput {
		if code := writeJSONOutput(map[string]any{
			"ok":      true,
			"listen":  config.ListenAddr,
			"policy":  config.PolicyPath,
			"adapter": config.DefaultAdapter,
		}, exitOK); code != exitOK {
			return code
		}
	} else {
		fmt.Printf("mcp serve: listening=%s adapter=%s\n", config.ListenAddr, config.DefaultAdapter)
	}

	server := &http.Server{
		Addr:              config.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return exitOK
}

func newMCPServeHandler(config mcpServeConfig) (http.Handler, error) {
	if strings.TrimSpace(config.PolicyPath) == "" {
		return nil, fmt.Errorf("mcp serve requires policy path")
	}
	if strings.TrimSpace(config.DefaultAdapter) == "" {
		config.DefaultAdapter = "mcp"
	}
	if config.TraceDir != "" {
		if err := os.MkdirAll(config.TraceDir, 0o750); err != nil {
			return nil, fmt.Errorf("create trace directory: %w", err)
		}
	}
	if config.RunpackDir != "" {
		if err := os.MkdirAll(config.RunpackDir, 0o750); err != nil {
			return nil, fmt.Errorf("create runpack directory: %w", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeMCPServeError(writer, http.StatusMethodNotAllowed, "expected GET")
			return
		}
		writeMCPServeJSON(writer, http.StatusOK, map[string]any{
			"ok":      true,
			"service": "gait.mcp.serve",
		})
	})
	mux.HandleFunc("/v1/evaluate", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeMCPServeError(writer, http.StatusMethodNotAllowed, "expected POST")
			return
		}
		response, err := evaluateMCPServeRequest(config, request)
		if err != nil {
			writeMCPServeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeMCPServeJSON(writer, http.StatusOK, response)
	})
	mux.HandleFunc("/v1/evaluate/sse", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeMCPServeError(writer, http.StatusMethodNotAllowed, "expected POST")
			return
		}
		response, err := evaluateMCPServeRequest(config, request)
		if err != nil {
			writeMCPServeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeMCPServeSSE(writer, response)
	})
	mux.HandleFunc("/v1/evaluate/stream", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeMCPServeError(writer, http.StatusMethodNotAllowed, "expected POST")
			return
		}
		response, err := evaluateMCPServeRequest(config, request)
		if err != nil {
			writeMCPServeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeMCPServeStream(writer, response)
	})
	return mux, nil
}

func evaluateMCPServeRequest(config mcpServeConfig, request *http.Request) (mcpServeEvaluateResponse, error) {
	var input mcpServeEvaluateRequest
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		return mcpServeEvaluateResponse{}, fmt.Errorf("decode request: %w", err)
	}
	if len(input.Call) == 0 {
		return mcpServeEvaluateResponse{}, fmt.Errorf("request.call is required")
	}
	adapter := strings.ToLower(strings.TrimSpace(input.Adapter))
	if adapter == "" {
		adapter = config.DefaultAdapter
	}
	callPayload, err := json.Marshal(input.Call)
	if err != nil {
		return mcpServeEvaluateResponse{}, fmt.Errorf("encode call payload: %w", err)
	}

	tracePath := strings.TrimSpace(input.TracePath)
	if tracePath == "" && config.TraceDir != "" {
		tracePath = filepath.Join(config.TraceDir, fmt.Sprintf("trace_%s_%s.json", normalizeRunID(input.RunID), time.Now().UTC().Format("20060102T150405.000000000")))
	}
	runpackPath := strings.TrimSpace(input.RunpackOut)
	if runpackPath == "" && input.EmitRunpack && config.RunpackDir != "" {
		runpackPath = filepath.Join(config.RunpackDir, fmt.Sprintf("runpack_%s_%s.zip", normalizeRunID(input.RunID), time.Now().UTC().Format("20060102T150405.000000000")))
	}

	output, exitCode, evalErr := evaluateMCPProxyPayload(config.PolicyPath, callPayload, mcpProxyEvalOptions{
		Adapter:       adapter,
		RunID:         input.RunID,
		TracePath:     tracePath,
		RunpackOut:    runpackPath,
		LogExportPath: config.LogExportPath,
		OTelExport:    config.OTelExport,
		KeyMode:       config.KeyMode,
		PrivateKey:    config.PrivateKey,
		PrivateKeyEnv: config.PrivateKeyEnv,
	})
	if evalErr != nil {
		return mcpServeEvaluateResponse{}, evalErr
	}
	return mcpServeEvaluateResponse{
		mcpProxyOutput: output,
		ExitCode:       exitCode,
	}, nil
}

func writeMCPServeError(writer http.ResponseWriter, status int, message string) {
	writeMCPServeJSON(writer, status, map[string]any{"ok": false, "error": message})
}

func writeMCPServeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("content-type", "application/json")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func writeMCPServeSSE(writer http.ResponseWriter, payload mcpServeEvaluateResponse) {
	writer.Header().Set("content-type", "text/event-stream")
	writer.Header().Set("cache-control", "no-cache")
	writer.Header().Set("connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)
	encoded, err := json.Marshal(payload)
	if err != nil {
		_, _ = writer.Write([]byte("event: evaluate\ndata: {\"ok\":false,\"error\":\"encode response\"}\n\n"))
		return
	}
	_, _ = writer.Write([]byte("event: evaluate\n"))
	_, _ = writer.Write([]byte("data: "))
	_, _ = writer.Write(encoded)
	_, _ = writer.Write([]byte("\n\n"))
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeMCPServeStream(writer http.ResponseWriter, payload mcpServeEvaluateResponse) {
	writer.Header().Set("content-type", "application/x-ndjson")
	writer.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func printMCPServeUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait mcp serve --policy <policy.yaml> [--listen 127.0.0.1:8787] [--adapter mcp|openai|anthropic|langchain] [--trace-dir <dir>] [--runpack-dir <dir>] [--export-log-out events.jsonl] [--export-otel-out otel.jsonl] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  endpoints: POST /v1/evaluate (json), POST /v1/evaluate/sse (text/event-stream), POST /v1/evaluate/stream (application/x-ndjson)")
}
