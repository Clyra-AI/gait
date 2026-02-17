package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/runpack"
)

type mcpServeConfig struct {
	PolicyPath               string
	ListenAddr               string
	DefaultAdapter           string
	Profile                  string
	AuthMode                 string
	AuthToken                string
	TraceDir                 string
	RunpackDir               string
	PackDir                  string
	SessionDir               string
	MaxRequestBytes          int64
	HTTPVerdictStatus        string
	AllowClientArtifactPaths bool
	TraceMaxAge              time.Duration
	TraceMaxCount            int
	RunpackMaxAge            time.Duration
	RunpackMaxCount          int
	PackMaxAge               time.Duration
	PackMaxCount             int
	SessionMaxAge            time.Duration
	SessionMaxCount          int
	LogExportPath            string
	OTelExport               string
	KeyMode                  string
	PrivateKey               string
	PrivateKeyEnv            string
}

type mcpServeEvaluateRequest struct {
	Adapter            string         `json:"adapter,omitempty"`
	Call               map[string]any `json:"call"`
	RunID              string         `json:"run_id,omitempty"`
	SessionID          string         `json:"session_id,omitempty"`
	SessionJournal     string         `json:"session_journal,omitempty"`
	CheckpointInterval int            `json:"checkpoint_interval,omitempty"`
	TracePath          string         `json:"trace_path,omitempty"`
	RunpackOut         string         `json:"runpack_out,omitempty"`
	EmitRunpack        bool           `json:"emit_runpack,omitempty"`
	PackOut            string         `json:"pack_out,omitempty"`
	EmitPack           bool           `json:"emit_pack,omitempty"`
}

type mcpServeEvaluateResponse struct {
	mcpProxyOutput
	ExitCode int `json:"exit_code"`
}

type mcpServeRequestError struct {
	Status  int
	Message string
}

func (requestError mcpServeRequestError) Error() string {
	return requestError.Message
}

func runMCPServe(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Run a local interception service that evaluates tool-call payloads through Gate and emits signed traces across JSON, SSE, and streamable HTTP endpoints.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"policy":                      true,
		"listen":                      true,
		"adapter":                     true,
		"profile":                     true,
		"auth-mode":                   true,
		"auth-token-env":              true,
		"trace-dir":                   true,
		"runpack-dir":                 true,
		"pack-dir":                    true,
		"session-dir":                 true,
		"max-request-bytes":           true,
		"http-verdict-status":         true,
		"allow-client-artifact-paths": false,
		"trace-max-age":               true,
		"trace-max-count":             true,
		"runpack-max-age":             true,
		"runpack-max-count":           true,
		"pack-max-age":                true,
		"pack-max-count":              true,
		"session-max-age":             true,
		"session-max-count":           true,
		"export-log-out":              true,
		"export-otel-out":             true,
		"key-mode":                    true,
		"private-key":                 true,
		"private-key-env":             true,
	})
	flagSet := flag.NewFlagSet("mcp-serve", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var policyPath string
	var listenAddr string
	var adapter string
	var profile string
	var authMode string
	var authTokenEnv string
	var traceDir string
	var runpackDir string
	var packDir string
	var sessionDir string
	var maxRequestBytes int64
	var httpVerdictStatus string
	var allowClientArtifactPaths bool
	var traceMaxAgeRaw string
	var traceMaxCount int
	var runpackMaxAgeRaw string
	var runpackMaxCount int
	var packMaxAgeRaw string
	var packMaxCount int
	var sessionMaxAgeRaw string
	var sessionMaxCount int
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
	flagSet.StringVar(&profile, "profile", "standard", "runtime profile: standard|oss-prod")
	flagSet.StringVar(&authMode, "auth-mode", "off", "serve auth mode: off|token")
	flagSet.StringVar(&authTokenEnv, "auth-token-env", "", "env var containing bearer token for --auth-mode token")
	flagSet.StringVar(&traceDir, "trace-dir", "./gait-out/mcp-serve/traces", "directory for emitted traces")
	flagSet.StringVar(&runpackDir, "runpack-dir", "", "optional directory for emitted runpacks")
	flagSet.StringVar(&packDir, "pack-dir", "", "optional directory for emitted PackSpec artifacts")
	flagSet.StringVar(&sessionDir, "session-dir", "./gait-out/mcp-serve/sessions", "directory for session journals")
	flagSet.Int64Var(&maxRequestBytes, "max-request-bytes", 1<<20, "maximum request body size in bytes")
	flagSet.StringVar(&httpVerdictStatus, "http-verdict-status", "compat", "verdict http status mode: compat|strict")
	flagSet.BoolVar(&allowClientArtifactPaths, "allow-client-artifact-paths", false, "allow client-provided trace/session/runpack output paths")
	flagSet.StringVar(&traceMaxAgeRaw, "trace-max-age", "0", "optional retention max age for trace files (for example 168h, 0 disables)")
	flagSet.IntVar(&traceMaxCount, "trace-max-count", 0, "optional retention max file count for trace files (0 disables)")
	flagSet.StringVar(&runpackMaxAgeRaw, "runpack-max-age", "0", "optional retention max age for runpack files (for example 336h, 0 disables)")
	flagSet.IntVar(&runpackMaxCount, "runpack-max-count", 0, "optional retention max file count for runpack files (0 disables)")
	flagSet.StringVar(&packMaxAgeRaw, "pack-max-age", "0", "optional retention max age for pack files (for example 336h, 0 disables)")
	flagSet.IntVar(&packMaxCount, "pack-max-count", 0, "optional retention max file count for pack files (0 disables)")
	flagSet.StringVar(&sessionMaxAgeRaw, "session-max-age", "0", "optional retention max age for session files (for example 336h, 0 disables)")
	flagSet.IntVar(&sessionMaxCount, "session-max-count", 0, "optional retention max file count for session files (0 disables)")
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
		PolicyPath:               policyPath,
		ListenAddr:               strings.TrimSpace(listenAddr),
		DefaultAdapter:           strings.ToLower(strings.TrimSpace(adapter)),
		Profile:                  strings.ToLower(strings.TrimSpace(profile)),
		AuthMode:                 strings.ToLower(strings.TrimSpace(authMode)),
		TraceDir:                 strings.TrimSpace(traceDir),
		RunpackDir:               strings.TrimSpace(runpackDir),
		PackDir:                  strings.TrimSpace(packDir),
		SessionDir:               strings.TrimSpace(sessionDir),
		MaxRequestBytes:          maxRequestBytes,
		HTTPVerdictStatus:        strings.ToLower(strings.TrimSpace(httpVerdictStatus)),
		AllowClientArtifactPaths: allowClientArtifactPaths,
		TraceMaxCount:            traceMaxCount,
		RunpackMaxCount:          runpackMaxCount,
		PackMaxCount:             packMaxCount,
		SessionMaxCount:          sessionMaxCount,
		LogExportPath:            strings.TrimSpace(logExportPath),
		OTelExport:               strings.TrimSpace(otelExportPath),
		KeyMode:                  strings.TrimSpace(keyMode),
		PrivateKey:               strings.TrimSpace(privateKeyPath),
		PrivateKeyEnv:            strings.TrimSpace(privateKeyEnv),
	}
	traceMaxAge, parseTraceErr := parseOptionalDuration(traceMaxAgeRaw)
	if parseTraceErr != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: fmt.Sprintf("parse --trace-max-age: %v", parseTraceErr)}, exitInvalidInput)
	}
	runpackMaxAge, parseRunpackErr := parseOptionalDuration(runpackMaxAgeRaw)
	if parseRunpackErr != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: fmt.Sprintf("parse --runpack-max-age: %v", parseRunpackErr)}, exitInvalidInput)
	}
	packMaxAge, parsePackErr := parseOptionalDuration(packMaxAgeRaw)
	if parsePackErr != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: fmt.Sprintf("parse --pack-max-age: %v", parsePackErr)}, exitInvalidInput)
	}
	sessionMaxAge, parseSessionErr := parseOptionalDuration(sessionMaxAgeRaw)
	if parseSessionErr != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: fmt.Sprintf("parse --session-max-age: %v", parseSessionErr)}, exitInvalidInput)
	}
	config.TraceMaxAge = traceMaxAge
	config.RunpackMaxAge = runpackMaxAge
	config.PackMaxAge = packMaxAge
	config.SessionMaxAge = sessionMaxAge
	if config.AuthMode != "off" && config.AuthMode != "token" {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "unsupported --auth-mode value (expected off or token)"}, exitInvalidInput)
	}
	isLoopback, loopbackErr := mcpServeIsLoopbackListen(config.ListenAddr)
	if loopbackErr != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: loopbackErr.Error()}, exitInvalidInput)
	}
	if !isLoopback && config.AuthMode != "token" {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "non-loopback --listen requires --auth-mode token"}, exitInvalidInput)
	}
	if config.AuthMode == "token" {
		if strings.TrimSpace(authTokenEnv) == "" {
			return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "--auth-mode token requires --auth-token-env"}, exitInvalidInput)
		}
		tokenValue := strings.TrimSpace(os.Getenv(authTokenEnv))
		if tokenValue == "" {
			return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "--auth-token-env did not resolve to a non-empty value"}, exitInvalidInput)
		}
		config.AuthToken = tokenValue
	}
	if config.MaxRequestBytes <= 0 {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "--max-request-bytes must be > 0"}, exitInvalidInput)
	}
	if config.HTTPVerdictStatus != "compat" && config.HTTPVerdictStatus != "strict" {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "unsupported --http-verdict-status value (expected compat or strict)"}, exitInvalidInput)
	}
	if config.TraceMaxCount < 0 || config.RunpackMaxCount < 0 || config.PackMaxCount < 0 || config.SessionMaxCount < 0 {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "retention max-count values must be >= 0"}, exitInvalidInput)
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
			"profile": config.Profile,
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
	if strings.TrimSpace(config.AuthMode) == "" {
		config.AuthMode = "off"
	}
	if strings.TrimSpace(config.DefaultAdapter) == "" {
		config.DefaultAdapter = "mcp"
	}
	if config.MaxRequestBytes <= 0 {
		config.MaxRequestBytes = 1 << 20
	}
	if strings.TrimSpace(config.HTTPVerdictStatus) == "" {
		config.HTTPVerdictStatus = "compat"
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
	if config.PackDir != "" {
		if err := os.MkdirAll(config.PackDir, 0o750); err != nil {
			return nil, fmt.Errorf("create pack directory: %w", err)
		}
	}
	if config.SessionDir != "" {
		if err := os.MkdirAll(config.SessionDir, 0o750); err != nil {
			return nil, fmt.Errorf("create session directory: %w", err)
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
		if err := authorizeMCPServeRequest(config, request); err != nil {
			writeMCPServeError(writer, http.StatusUnauthorized, err.Error())
			return
		}
		response, err := evaluateMCPServeRequest(config, writer, request)
		if err != nil {
			writeMCPServeError(writer, mcpServeErrorStatus(err), err.Error())
			return
		}
		writeMCPServeJSON(writer, mcpServeVerdictHTTPStatus(config, response), response)
	})
	mux.HandleFunc("/v1/evaluate/sse", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeMCPServeError(writer, http.StatusMethodNotAllowed, "expected POST")
			return
		}
		if err := authorizeMCPServeRequest(config, request); err != nil {
			writeMCPServeError(writer, http.StatusUnauthorized, err.Error())
			return
		}
		response, err := evaluateMCPServeRequest(config, writer, request)
		if err != nil {
			writeMCPServeError(writer, mcpServeErrorStatus(err), err.Error())
			return
		}
		writeMCPServeSSE(writer, mcpServeVerdictHTTPStatus(config, response), response)
	})
	mux.HandleFunc("/v1/evaluate/stream", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeMCPServeError(writer, http.StatusMethodNotAllowed, "expected POST")
			return
		}
		if err := authorizeMCPServeRequest(config, request); err != nil {
			writeMCPServeError(writer, http.StatusUnauthorized, err.Error())
			return
		}
		response, err := evaluateMCPServeRequest(config, writer, request)
		if err != nil {
			writeMCPServeError(writer, mcpServeErrorStatus(err), err.Error())
			return
		}
		writeMCPServeStream(writer, mcpServeVerdictHTTPStatus(config, response), response)
	})
	return mux, nil
}

func evaluateMCPServeRequest(config mcpServeConfig, writer http.ResponseWriter, request *http.Request) (mcpServeEvaluateResponse, error) {
	if err := ensureMCPServeContentType(request); err != nil {
		return mcpServeEvaluateResponse{}, err
	}
	input, err := decodeMCPServeRequest(config, writer, request)
	if err != nil {
		return mcpServeEvaluateResponse{}, err
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
	if tracePath != "" && !config.AllowClientArtifactPaths {
		return mcpServeEvaluateResponse{}, fmt.Errorf("request.trace_path is not allowed by server configuration")
	}
	if tracePath == "" && config.TraceDir != "" {
		tracePath = filepath.Join(config.TraceDir, fmt.Sprintf("trace_%s_%s.json", normalizeRunID(input.RunID), time.Now().UTC().Format("20060102T150405.000000000")))
	}
	runpackPath := strings.TrimSpace(input.RunpackOut)
	if runpackPath != "" && !config.AllowClientArtifactPaths {
		return mcpServeEvaluateResponse{}, fmt.Errorf("request.runpack_out is not allowed by server configuration")
	}
	if runpackPath == "" && input.EmitRunpack && config.RunpackDir != "" {
		runpackPath = filepath.Join(config.RunpackDir, fmt.Sprintf("runpack_%s_%s.zip", normalizeRunID(input.RunID), time.Now().UTC().Format("20060102T150405.000000000")))
	}
	packPath := strings.TrimSpace(input.PackOut)
	if packPath != "" && !config.AllowClientArtifactPaths {
		return mcpServeEvaluateResponse{}, fmt.Errorf("request.pack_out is not allowed by server configuration")
	}
	if packPath == "" && input.EmitPack {
		if config.PackDir == "" {
			return mcpServeEvaluateResponse{}, fmt.Errorf("request.emit_pack requires server --pack-dir")
		}
		packPath = filepath.Join(config.PackDir, fmt.Sprintf("pack_%s_%s.zip", normalizeRunID(input.RunID), time.Now().UTC().Format("20060102T150405.000000000")))
	}

	output, exitCode, evalErr := evaluateMCPProxyPayload(config.PolicyPath, callPayload, mcpProxyEvalOptions{
		Adapter:       adapter,
		Profile:       config.Profile,
		RunID:         input.RunID,
		TracePath:     tracePath,
		RunpackOut:    runpackPath,
		PackOut:       packPath,
		AutoPackDir:   config.PackDir,
		LogExportPath: config.LogExportPath,
		OTelExport:    config.OTelExport,
		KeyMode:       config.KeyMode,
		PrivateKey:    config.PrivateKey,
		PrivateKeyEnv: config.PrivateKeyEnv,
	})
	if evalErr != nil {
		return mcpServeEvaluateResponse{}, evalErr
	}

	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(output.SessionID)
	}
	if sessionID != "" {
		journalPath := strings.TrimSpace(input.SessionJournal)
		if journalPath != "" && !config.AllowClientArtifactPaths {
			return mcpServeEvaluateResponse{}, fmt.Errorf("request.session_journal is not allowed by server configuration")
		}
		if journalPath == "" {
			base := sanitizeSessionFileBase(sessionID)
			journalPath = filepath.Join(config.SessionDir, base+".journal.jsonl")
		}
		if _, err := runpack.StartSession(journalPath, runpack.SessionStartOptions{
			SessionID:       sessionID,
			RunID:           output.RunID,
			ProducerVersion: version,
		}); err != nil {
			return mcpServeEvaluateResponse{}, err
		}
		event, err := runpack.AppendSessionEvent(journalPath, runpack.SessionAppendOptions{
			ProducerVersion: version,
			ToolName:        output.ToolName,
			IntentDigest:    output.IntentDigest,
			PolicyDigest:    output.PolicyDigest,
			TraceID:         output.TraceID,
			TracePath:       output.TracePath,
			Verdict:         output.Verdict,
			ReasonCodes:     output.ReasonCodes,
			Violations:      output.Violations,
		})
		if err != nil {
			return mcpServeEvaluateResponse{}, err
		}
		output.SessionID = sessionID
		if output.Warnings == nil {
			output.Warnings = []string{}
		}
		output.Warnings = append(output.Warnings, fmt.Sprintf("session_journal=%s sequence=%d", journalPath, event.Sequence))
		if input.CheckpointInterval > 0 && event.Sequence%int64(input.CheckpointInterval) == 0 && config.RunpackDir != "" {
			checkpointOut := filepath.Join(config.RunpackDir, fmt.Sprintf("%s_cp_%06d.zip", sanitizeSessionFileBase(sessionID), event.Sequence))
			_, chainPath, checkpointErr := runpack.SessionCheckpointAndWriteChain(journalPath, checkpointOut, runpack.SessionCheckpointOptions{
				ProducerVersion: version,
			})
			if checkpointErr != nil {
				return mcpServeEvaluateResponse{}, checkpointErr
			}
			output.Warnings = append(output.Warnings, fmt.Sprintf("session_checkpoint=%s chain=%s", checkpointOut, chainPath))
		}
	}
	if retentionWarnings := applyMCPServeRetention(config, time.Now().UTC()); len(retentionWarnings) > 0 {
		output.Warnings = append(output.Warnings, retentionWarnings...)
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

func writeMCPServeSSE(writer http.ResponseWriter, status int, payload mcpServeEvaluateResponse) {
	writer.Header().Set("content-type", "text/event-stream")
	writer.Header().Set("cache-control", "no-cache")
	writer.Header().Set("connection", "keep-alive")
	writer.WriteHeader(status)
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

func writeMCPServeStream(writer http.ResponseWriter, status int, payload mcpServeEvaluateResponse) {
	writer.Header().Set("content-type", "application/x-ndjson")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

type mcpRetentionFile struct {
	path    string
	modTime time.Time
}

func parseOptionalDuration(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "0" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	if parsed < 0 {
		return 0, fmt.Errorf("duration must be >= 0")
	}
	return parsed, nil
}

func applyMCPServeRetention(config mcpServeConfig, now time.Time) []string {
	warnings := make([]string, 0)
	warnings = append(warnings, applyMCPServeRetentionClass("trace", config.TraceDir, config.TraceMaxAge, config.TraceMaxCount, now)...)
	warnings = append(warnings, applyMCPServeRetentionClass("runpack", config.RunpackDir, config.RunpackMaxAge, config.RunpackMaxCount, now)...)
	warnings = append(warnings, applyMCPServeRetentionClass("pack", config.PackDir, config.PackMaxAge, config.PackMaxCount, now)...)
	warnings = append(warnings, applyMCPServeRetentionClass("session", config.SessionDir, config.SessionMaxAge, config.SessionMaxCount, now)...)
	return warnings
}

func applyMCPServeRetentionClass(name string, dir string, maxAge time.Duration, maxCount int, now time.Time) []string {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" || (maxAge <= 0 && maxCount <= 0) {
		return nil
	}
	entries, err := os.ReadDir(trimmedDir)
	if err != nil {
		return []string{fmt.Sprintf("retention_%s_error=%v", name, err)}
	}
	files := make([]mcpRetentionFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !mcpRetentionMatches(name, entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return []string{fmt.Sprintf("retention_%s_error=%v", name, infoErr)}
		}
		files = append(files, mcpRetentionFile{
			path:    filepath.Join(trimmedDir, entry.Name()),
			modTime: info.ModTime().UTC(),
		})
	}
	if len(files) == 0 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].path > files[j].path
		}
		return files[i].modTime.After(files[j].modTime)
	})
	keep := map[string]struct{}{}
	if maxCount > 0 {
		for i := 0; i < len(files) && i < maxCount; i++ {
			keep[files[i].path] = struct{}{}
		}
	}
	removedByAge := 0
	removedByCount := 0
	for _, file := range files {
		ageExceeded := maxAge > 0 && now.Sub(file.modTime) > maxAge
		countExceeded := maxCount > 0
		if countExceeded {
			_, keepByCount := keep[file.path]
			countExceeded = !keepByCount
		}
		if !ageExceeded && !countExceeded {
			continue
		}
		if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
			return []string{fmt.Sprintf("retention_%s_error=%v", name, err)}
		}
		if ageExceeded {
			removedByAge++
		}
		if countExceeded {
			removedByCount++
		}
	}
	if removedByAge == 0 && removedByCount == 0 {
		return nil
	}
	return []string{fmt.Sprintf("retention_%s_removed_age=%d removed_count=%d", name, removedByAge, removedByCount)}
}

func mcpRetentionMatches(class string, fileName string) bool {
	lowered := strings.ToLower(strings.TrimSpace(fileName))
	switch class {
	case "trace":
		return strings.HasSuffix(lowered, ".json") && strings.Contains(lowered, "trace")
	case "runpack":
		if !strings.HasSuffix(lowered, ".zip") {
			return false
		}
		// Runpack retention applies only to runpack/checkpoint artifacts to avoid
		// cross-pruning PackSpec outputs when directories are shared.
		return strings.HasPrefix(lowered, "runpack_") || strings.Contains(lowered, "_cp_")
	case "pack":
		return strings.HasSuffix(lowered, ".zip") && strings.HasPrefix(lowered, "pack_")
	case "session":
		return strings.HasSuffix(lowered, ".json") ||
			strings.HasSuffix(lowered, ".jsonl") ||
			strings.HasSuffix(lowered, ".state")
	default:
		return false
	}
}

func printMCPServeUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait mcp serve --policy <policy.yaml> [--listen 127.0.0.1:8787] [--adapter mcp|openai|anthropic|langchain] [--profile standard|oss-prod] [--auth-mode off|token] [--auth-token-env <VAR>] [--max-request-bytes <bytes>] [--http-verdict-status compat|strict] [--allow-client-artifact-paths] [--trace-dir <dir>] [--runpack-dir <dir>] [--pack-dir <dir>] [--session-dir <dir>] [--trace-max-age <dur>] [--trace-max-count <n>] [--runpack-max-age <dur>] [--runpack-max-count <n>] [--pack-max-age <dur>] [--pack-max-count <n>] [--session-max-age <dur>] [--session-max-count <n>] [--export-log-out events.jsonl] [--export-otel-out otel.jsonl] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  endpoints: POST /v1/evaluate (json), POST /v1/evaluate/sse (text/event-stream), POST /v1/evaluate/stream (application/x-ndjson)")
}

func sanitizeSessionFileBase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "session"
	}
	mapped := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_").Replace(trimmed)
	return strings.Trim(mapped, "._")
}

func mcpServeErrorStatus(err error) int {
	var requestErr mcpServeRequestError
	if errors.As(err, &requestErr) {
		return requestErr.Status
	}
	return http.StatusBadRequest
}

func mcpServeVerdictHTTPStatus(config mcpServeConfig, response mcpServeEvaluateResponse) int {
	if strings.TrimSpace(config.HTTPVerdictStatus) != "strict" {
		return http.StatusOK
	}
	switch strings.TrimSpace(response.Verdict) {
	case "block":
		return http.StatusForbidden
	case "require_approval", "dry_run":
		return http.StatusConflict
	default:
		return http.StatusOK
	}
}

func authorizeMCPServeRequest(config mcpServeConfig, request *http.Request) error {
	if strings.TrimSpace(config.AuthMode) != "token" {
		return nil
	}
	if strings.TrimSpace(config.AuthToken) == "" {
		return fmt.Errorf("auth token is not configured")
	}
	rawHeader := strings.TrimSpace(request.Header.Get("Authorization"))
	if !strings.HasPrefix(rawHeader, "Bearer ") {
		return fmt.Errorf("missing bearer authorization")
	}
	provided := strings.TrimSpace(strings.TrimPrefix(rawHeader, "Bearer "))
	if subtle.ConstantTimeCompare([]byte(provided), []byte(strings.TrimSpace(config.AuthToken))) != 1 {
		return fmt.Errorf("invalid bearer authorization")
	}
	return nil
}

func ensureMCPServeContentType(request *http.Request) error {
	contentType := strings.TrimSpace(request.Header.Get("Content-Type"))
	if contentType == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return mcpServeRequestError{Status: http.StatusUnsupportedMediaType, Message: "invalid content-type header"}
	}
	if mediaType != "application/json" {
		return mcpServeRequestError{Status: http.StatusUnsupportedMediaType, Message: "content-type must be application/json"}
	}
	return nil
}

func decodeMCPServeRequest(config mcpServeConfig, writer http.ResponseWriter, request *http.Request) (mcpServeEvaluateRequest, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, config.MaxRequestBytes)
	defer func() {
		_ = request.Body.Close()
	}()
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input mcpServeEvaluateRequest
	if err := decoder.Decode(&input); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return mcpServeEvaluateRequest{}, mcpServeRequestError{
				Status:  http.StatusRequestEntityTooLarge,
				Message: "request body exceeds max-request-bytes",
			}
		}
		return mcpServeEvaluateRequest{}, mcpServeRequestError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("decode request: %v", err),
		}
	}
	var tail struct{}
	if err := decoder.Decode(&tail); err != io.EOF {
		return mcpServeEvaluateRequest{}, mcpServeRequestError{
			Status:  http.StatusBadRequest,
			Message: "request body must contain a single JSON object",
		}
	}
	return input, nil
}

func mcpServeIsLoopbackListen(listenAddr string) (bool, error) {
	trimmed := strings.TrimSpace(listenAddr)
	if trimmed == "" {
		return false, fmt.Errorf("listen address is required")
	}
	host, _, err := net.SplitHostPort(trimmed)
	if err != nil {
		return false, fmt.Errorf("invalid --listen address: %w", err)
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true, nil
	}
	parsed := net.ParseIP(host)
	if parsed == nil {
		return false, nil
	}
	return parsed.IsLoopback(), nil
}
