package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/gate"
	"github.com/davidahmann/gait/core/mcp"
	"github.com/davidahmann/gait/core/pack"
	"github.com/davidahmann/gait/core/runpack"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/sign"
)

type mcpProxyOutput struct {
	OK                bool     `json:"ok"`
	Executed          bool     `json:"executed"`
	Adapter           string   `json:"adapter,omitempty"`
	RunID             string   `json:"run_id,omitempty"`
	SessionID         string   `json:"session_id,omitempty"`
	ToolName          string   `json:"tool_name,omitempty"`
	Verdict           string   `json:"verdict,omitempty"`
	ReasonCodes       []string `json:"reason_codes,omitempty"`
	Violations        []string `json:"violations,omitempty"`
	PolicyDigest      string   `json:"policy_digest,omitempty"`
	IntentDigest      string   `json:"intent_digest,omitempty"`
	DecisionLatencyMS int64    `json:"decision_latency_ms,omitempty"`
	TraceID           string   `json:"trace_id,omitempty"`
	TracePath         string   `json:"trace_path,omitempty"`
	RunpackPath       string   `json:"runpack_path,omitempty"`
	PackPath          string   `json:"pack_path,omitempty"`
	PackID            string   `json:"pack_id,omitempty"`
	LogExport         string   `json:"log_export,omitempty"`
	OTelExport        string   `json:"otel_export,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
	Error             string   `json:"error,omitempty"`
}

type mcpProxyEvalOptions struct {
	Adapter       string
	Profile       string
	RunID         string
	TracePath     string
	RunpackOut    string
	PackOut       string
	AutoPackDir   string
	LogExportPath string
	OTelExport    string
	KeyMode       string
	PrivateKey    string
	PrivateKeyEnv string
}

func runMCP(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Proxy tool-call protocol payloads through Gate policy evaluation and emit signed traces with optional exports.")
	}
	if len(arguments) == 0 {
		printMCPUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "proxy":
		return runMCPProxy(arguments[1:])
	case "bridge":
		return runMCPProxy(arguments[1:])
	case "serve":
		return runMCPServe(arguments[1:])
	default:
		printMCPUsage()
		return exitInvalidInput
	}
}

func runMCPProxy(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Decode an MCP or adapter-formatted tool call, evaluate policy deterministically, and emit a signed gate-compatible trace.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"policy":          true,
		"call":            true,
		"adapter":         true,
		"profile":         true,
		"trace-out":       true,
		"run-id":          true,
		"runpack-out":     true,
		"pack-out":        true,
		"export-log-out":  true,
		"export-otel-out": true,
		"key-mode":        true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("mcp-proxy", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var policyPath string
	var callPath string
	var adapter string
	var profile string
	var tracePath string
	var runID string
	var runpackOut string
	var packOut string
	var logExportPath string
	var otelExportPath string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&policyPath, "policy", "", "path to policy YAML")
	flagSet.StringVar(&callPath, "call", "", "path to tool call JSON (use '-' for stdin)")
	flagSet.StringVar(&adapter, "adapter", "mcp", "adapter payload format: mcp|openai|anthropic|langchain")
	flagSet.StringVar(&profile, "profile", string(gateProfileStandard), "runtime profile: standard|oss-prod")
	flagSet.StringVar(&tracePath, "trace-out", "", "path to emitted trace JSON (default trace_<trace_id>.json)")
	flagSet.StringVar(&runID, "run-id", "", "optional run_id override for proxy artifacts")
	flagSet.StringVar(&runpackOut, "runpack-out", "", "optional path to emit a runpack zip for this proxy decision")
	flagSet.StringVar(&packOut, "pack-out", "", "optional path to emit a PackSpec run pack for this proxy decision")
	flagSet.StringVar(&logExportPath, "export-log-out", "", "optional JSONL log export path")
	flagSet.StringVar(&otelExportPath, "export-otel-out", "", "optional OTEL-style JSONL export path")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printMCPProxyUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(policyPath) == "" && len(remaining) > 0 {
		policyPath = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(callPath) == "" && len(remaining) > 0 {
		callPath = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(policyPath) == "" || strings.TrimSpace(callPath) == "" || len(remaining) > 0 {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: "expected --policy <policy.yaml> and --call <tool_call.json|->"}, exitInvalidInput)
	}

	payload, err := readMCPPayload(callPath)
	if err != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	output, exitCode, err := evaluateMCPProxyPayload(policyPath, payload, mcpProxyEvalOptions{
		Adapter:       adapter,
		Profile:       profile,
		RunID:         runID,
		TracePath:     tracePath,
		RunpackOut:    runpackOut,
		PackOut:       packOut,
		LogExportPath: logExportPath,
		OTelExport:    otelExportPath,
		KeyMode:       keyMode,
		PrivateKey:    privateKeyPath,
		PrivateKeyEnv: privateKeyEnv,
	})
	if err != nil {
		return writeMCPProxyOutput(jsonOutput, mcpProxyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeMCPProxyOutput(jsonOutput, output, exitCode)
}

func readMCPPayload(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin payload: %w", err)
		}
		return raw, nil
	}
	// #nosec G304 -- call path is explicit local user input.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read call payload: %w", err)
	}
	return raw, nil
}

func evaluateMCPProxyPayload(policyPath string, payload []byte, options mcpProxyEvalOptions) (mcpProxyOutput, int, error) {
	decisionStarted := time.Now()
	call, err := mcp.DecodeToolCall(options.Adapter, payload)
	if err != nil {
		return mcpProxyOutput{}, exitInvalidInput, err
	}

	resolvedProfile, err := parseGateEvalProfile(options.Profile)
	if err != nil {
		return mcpProxyOutput{}, exitInvalidInput, err
	}
	if resolvedProfile == gateProfileOSSProd && sign.KeyMode(strings.ToLower(strings.TrimSpace(options.KeyMode))) != sign.ModeProd {
		return mcpProxyOutput{}, exitInvalidInput, fmt.Errorf("oss-prod profile requires --key-mode prod")
	}

	policy, err := gate.LoadPolicyFile(policyPath)
	if err != nil {
		return mcpProxyOutput{}, exitInvalidInput, err
	}

	evalResult, err := mcp.EvaluateToolCallWithIntentOptions(policy, call, gate.EvalOptions{ProducerVersion: version}, mcp.IntentOptions{
		RequireExplicitContext: resolvedProfile == gateProfileOSSProd,
	})
	if err != nil {
		return mcpProxyOutput{}, exitInvalidInput, err
	}

	keyPair, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(strings.ToLower(strings.TrimSpace(options.KeyMode))),
		PrivateKeyPath: options.PrivateKey,
		PrivateKeyEnv:  options.PrivateKeyEnv,
	})
	if err != nil {
		return mcpProxyOutput{}, exitInvalidInput, err
	}
	resolvedTracePath := strings.TrimSpace(options.TracePath)
	if resolvedTracePath == "" {
		resolvedTracePath = fmt.Sprintf("trace_%s_%s.json", normalizeRunID(options.RunID), time.Now().UTC().Format("20060102T150405.000000000"))
	}
	traceResult, err := gate.EmitSignedTrace(policy, evalResult.Intent, evalResult.Outcome.Result, gate.EmitTraceOptions{
		ProducerVersion:   version,
		SigningPrivateKey: keyPair.Private,
		TracePath:         resolvedTracePath,
	})
	if err != nil {
		return mcpProxyOutput{}, exitInvalidInput, err
	}
	if resolvedProfile == gateProfileStandard && (strings.TrimSpace(call.Context.Identity) == "" || strings.TrimSpace(call.Context.Workspace) == "" || strings.TrimSpace(call.Context.SessionID) == "") {
		warnings = append(warnings, "standard profile applied fallback intent context; use --profile oss-prod for strict context enforcement")
	}

	resolvedRunID := normalizeRunID(options.RunID)
	if resolvedRunID == "" {
		resolvedRunID = normalizeRunID(call.Context.RunID)
	}
	if resolvedRunID == "" {
		resolvedRunID = "run_mcp_" + evalResult.Intent.IntentDigest[:12]
	}

	resolvedRunpackPath := ""
	if strings.TrimSpace(options.RunpackOut) != "" {
		resolvedRunpackPath = strings.TrimSpace(options.RunpackOut)
		if err := writeMCPRunpack(resolvedRunpackPath, resolvedRunID, evalResult, traceResult.Trace.TraceID); err != nil {
			return mcpProxyOutput{}, exitInvalidInput, err
		}
	}

	resolvedPackPath := strings.TrimSpace(options.PackOut)
	if resolvedPackPath == "" && strings.TrimSpace(options.AutoPackDir) != "" && shouldAutoEmitMCPPack(evalResult.Intent) {
		resolvedPackPath = filepath.Join(strings.TrimSpace(options.AutoPackDir), fmt.Sprintf("pack_%s_%s.zip", normalizeRunID(resolvedRunID), time.Now().UTC().Format("20060102T150405.000000000")))
	}
	resolvedPackID := ""
	if resolvedPackPath != "" {
		runpackPathForPack := resolvedRunpackPath
		cleanup := func() {}
		if runpackPathForPack == "" {
			tmpDir, tmpErr := os.MkdirTemp("", "gait-mcp-pack-*")
			if tmpErr != nil {
				return mcpProxyOutput{}, exitInvalidInput, fmt.Errorf("create temp runpack directory for pack build: %w", tmpErr)
			}
			tmpRunpackPath := filepath.Join(tmpDir, "runpack.zip")
			if err := writeMCPRunpack(tmpRunpackPath, resolvedRunID, evalResult, traceResult.Trace.TraceID); err != nil {
				return mcpProxyOutput{}, exitInvalidInput, err
			}
			runpackPathForPack = tmpRunpackPath
			cleanup = func() {
				_ = os.RemoveAll(tmpDir)
			}
		}
		buildResult, buildErr := pack.BuildRunPack(pack.BuildRunOptions{
			RunpackPath:       runpackPathForPack,
			OutputPath:        resolvedPackPath,
			ProducerVersion:   version,
			SigningPrivateKey: keyPair.Private,
		})
		cleanup()
		if buildErr != nil {
			return mcpProxyOutput{}, exitInvalidInput, fmt.Errorf("build proxy pack: %w", buildErr)
		}
		resolvedPackPath = buildResult.Path
		resolvedPackID = buildResult.Manifest.PackID
	}

	exportEvent := mcp.ExportEvent{
		CreatedAt:       evalResult.Outcome.Result.CreatedAt,
		ProducerVersion: version,
		RunID:           resolvedRunID,
		SessionID:       evalResult.Intent.Context.SessionID,
		TraceID:         traceResult.Trace.TraceID,
		TracePath:       traceResult.TracePath,
		ToolName:        evalResult.Intent.ToolName,
		Verdict:         evalResult.Outcome.Result.Verdict,
		ReasonCodes:     evalResult.Outcome.Result.ReasonCodes,
		PolicyDigest:    traceResult.PolicyDigest,
		IntentDigest:    traceResult.IntentDigest,
	}
	if traceResult.Trace.DelegationRef != nil {
		exportEvent.DelegationRef = strings.TrimSpace(traceResult.Trace.DelegationRef.DelegationTokenRef)
		if exportEvent.DelegationRef == "" {
			exportEvent.DelegationRef = strings.TrimSpace(traceResult.Trace.DelegationRef.ChainDigest)
		}
		exportEvent.DelegationDepth = traceResult.Trace.DelegationRef.DelegationDepth
	}
	decisionLatencyMS := time.Since(decisionStarted).Milliseconds()
	if decisionLatencyMS < 0 {
		decisionLatencyMS = 0
	}
	exportEvent.DecisionLatency = decisionLatencyMS
	resolvedLogExport := ""
	if strings.TrimSpace(options.LogExportPath) != "" {
		resolvedLogExport = strings.TrimSpace(options.LogExportPath)
		if err := mcp.ExportLogEvent(resolvedLogExport, exportEvent); err != nil {
			return mcpProxyOutput{}, exitInvalidInput, err
		}
	}
	resolvedOTelExport := ""
	if strings.TrimSpace(options.OTelExport) != "" {
		resolvedOTelExport = strings.TrimSpace(options.OTelExport)
		if err := mcp.ExportOTelEvent(resolvedOTelExport, exportEvent); err != nil {
			return mcpProxyOutput{}, exitInvalidInput, err
		}
	}

	exitCode := exitOK
	switch evalResult.Outcome.Result.Verdict {
	case "block":
		exitCode = exitPolicyBlocked
	case "require_approval":
		exitCode = exitApprovalRequired
	}
	return mcpProxyOutput{
		OK:                true,
		Executed:          false,
		Adapter:           strings.ToLower(strings.TrimSpace(options.Adapter)),
		RunID:             resolvedRunID,
		SessionID:         evalResult.Intent.Context.SessionID,
		ToolName:          evalResult.Intent.ToolName,
		Verdict:           evalResult.Outcome.Result.Verdict,
		ReasonCodes:       evalResult.Outcome.Result.ReasonCodes,
		Violations:        evalResult.Outcome.Result.Violations,
		PolicyDigest:      traceResult.PolicyDigest,
		IntentDigest:      traceResult.IntentDigest,
		DecisionLatencyMS: decisionLatencyMS,
		TraceID:           traceResult.Trace.TraceID,
		TracePath:         traceResult.TracePath,
		RunpackPath:       resolvedRunpackPath,
		PackPath:          resolvedPackPath,
		PackID:            resolvedPackID,
		LogExport:         resolvedLogExport,
		OTelExport:        resolvedOTelExport,
		Warnings:          warnings,
	}, exitCode, nil
}

func writeMCPRunpack(path string, runID string, evalResult mcp.EvalResult, traceID string) error {
	normalizedPath, err := sanitizeRunpackOutputPath(path)
	if err != nil {
		return err
	}

	now := evalResult.Outcome.Result.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	resultPayload := map[string]any{
		"verdict":      evalResult.Outcome.Result.Verdict,
		"reason_codes": evalResult.Outcome.Result.ReasonCodes,
		"violations":   evalResult.Outcome.Result.Violations,
		"trace_id":     traceID,
	}
	resultDigest, err := digestObject(resultPayload)
	if err != nil {
		return fmt.Errorf("digest proxy result: %w", err)
	}
	resultStatus := "ok"
	if evalResult.Outcome.Result.Verdict == "block" || evalResult.Outcome.Result.Verdict == "require_approval" {
		resultStatus = "error"
	}

	dir := filepath.Dir(normalizedPath)
	if dir != "." && dir != "" {
		if filepath.IsLocal(dir) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create runpack directory: %w", err)
			}
		} else if strings.HasPrefix(dir, string(filepath.Separator)) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create runpack directory: %w", err)
			}
		} else if volume := filepath.VolumeName(dir); volume != "" && strings.HasPrefix(dir, volume+string(filepath.Separator)) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create runpack directory: %w", err)
			}
		} else {
			return fmt.Errorf("runpack output directory must be local relative or absolute")
		}
	}
	_, err = runpack.WriteRunpack(normalizedPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			SchemaID:        "gait.runpack.run",
			SchemaVersion:   "1.0.0",
			CreatedAt:       now,
			ProducerVersion: version,
			RunID:           runID,
			Timeline: []schemarunpack.TimelineEvt{
				{Event: "proxy_eval_start", TS: now},
				{Event: "proxy_eval_finish", TS: now},
			},
		},
		Intents: []schemarunpack.IntentRecord{{
			SchemaID:        "gait.runpack.intent",
			SchemaVersion:   "1.0.0",
			CreatedAt:       now,
			ProducerVersion: version,
			RunID:           runID,
			IntentID:        "intent_1",
			ToolName:        evalResult.Intent.ToolName,
			ArgsDigest:      evalResult.Intent.ArgsDigest,
			Args:            evalResult.Intent.Args,
		}},
		Results: []schemarunpack.ResultRecord{{
			SchemaID:        "gait.runpack.result",
			SchemaVersion:   "1.0.0",
			CreatedAt:       now,
			ProducerVersion: version,
			RunID:           runID,
			IntentID:        "intent_1",
			Status:          resultStatus,
			ResultDigest:    resultDigest,
			Result:          resultPayload,
		}},
		Refs: schemarunpack.Refs{
			SchemaID:        "gait.runpack.refs",
			SchemaVersion:   "1.0.0",
			CreatedAt:       now,
			ProducerVersion: version,
			RunID:           runID,
			Receipts:        []schemarunpack.RefReceipt{},
		},
		CaptureMode: "reference",
	})
	if err != nil {
		return fmt.Errorf("write proxy runpack: %w", err)
	}
	return nil
}

func shouldAutoEmitMCPPack(intent schemagate.IntentRequest) bool {
	hasExplicitOperation := false
	if len(intent.Targets) > 0 {
		for _, target := range intent.Targets {
			operation := normalizeMCPToolOperation(target.Operation)
			if operation == "" {
				continue
			}
			hasExplicitOperation = true
			if _, ok := readOnlyMCPToolOperations[operation]; ok {
				continue
			}
			return true
		}
		if hasExplicitOperation {
			return false
		}
	}
	operation := inferMCPToolOperation(intent.ToolName)
	if operation == "" {
		return false
	}
	if _, ok := readOnlyMCPToolOperations[operation]; ok {
		return false
	}
	if _, ok := writeMCPToolOperations[operation]; ok {
		return true
	}
	for _, prefix := range writeMCPToolPrefixes {
		if strings.HasPrefix(operation, prefix) {
			return true
		}
	}
	for _, prefix := range readOnlyMCPToolPrefixes {
		if strings.HasPrefix(operation, prefix) {
			return false
		}
	}
	return false
}

func normalizeMCPToolOperation(operation string) string {
	return strings.ToLower(strings.TrimSpace(operation))
}

func inferMCPToolOperation(toolName string) string {
	tokens := mcpToolNameTokenPattern.FindAllString(strings.ToLower(strings.TrimSpace(toolName)), -1)
	if len(tokens) == 0 {
		return ""
	}
	for _, token := range tokens {
		if _, ok := writeMCPToolOperations[token]; ok {
			return token
		}
		if _, ok := readOnlyMCPToolOperations[token]; ok {
			return token
		}
	}
	for _, token := range tokens {
		for _, prefix := range writeMCPToolPrefixes {
			if strings.HasPrefix(token, prefix) {
				return token
			}
		}
		for _, prefix := range readOnlyMCPToolPrefixes {
			if strings.HasPrefix(token, prefix) {
				return token
			}
		}
	}
	for _, token := range tokens {
		switch token {
		case "tool", "tools", "mcp", "function":
			continue
		default:
			return token
		}
	}
	return ""
}

var readOnlyMCPToolOperations = map[string]struct{}{
	"read":     {},
	"list":     {},
	"query":    {},
	"search":   {},
	"inspect":  {},
	"get":      {},
	"fetch":    {},
	"head":     {},
	"describe": {},
}

var writeMCPToolOperations = map[string]struct{}{
	"write":   {},
	"create":  {},
	"update":  {},
	"delete":  {},
	"remove":  {},
	"insert":  {},
	"upsert":  {},
	"patch":   {},
	"set":     {},
	"put":     {},
	"post":    {},
	"commit":  {},
	"apply":   {},
	"approve": {},
	"execute": {},
	"exec":    {},
	"run":     {},
}

var readOnlyMCPToolPrefixes = []string{
	"get", "read", "list", "query", "search", "inspect", "fetch", "describe",
}

var writeMCPToolPrefixes = []string{
	"write", "create", "update", "delete", "remove", "insert", "upsert", "patch", "set", "put", "post", "commit", "apply", "approve", "exec", "run",
}

var mcpToolNameTokenPattern = regexp.MustCompile(`[a-z0-9]+`)

func sanitizeRunpackOutputPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("runpack output path is required")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("runpack output path is required")
	}
	if !filepath.IsAbs(cleaned) {
		for _, segment := range strings.Split(filepath.ToSlash(cleaned), "/") {
			if segment == ".." {
				return "", fmt.Errorf("relative runpack output path must not traverse parent directories")
			}
		}
	}
	return cleaned, nil
}

var runIDSanitizer = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func normalizeRunID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	trimmed = runIDSanitizer.ReplaceAllString(trimmed, "_")
	trimmed = strings.Trim(trimmed, "_")
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "run_") {
		return trimmed
	}
	return "run_" + trimmed
}

func writeMCPProxyOutput(jsonOutput bool, output mcpProxyOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("mcp proxy: verdict=%s\n", output.Verdict)
		fmt.Printf("trace: %s\n", output.TracePath)
		if output.RunpackPath != "" {
			fmt.Printf("runpack: %s\n", output.RunpackPath)
		}
		if output.PackPath != "" {
			fmt.Printf("pack: %s\n", output.PackPath)
		}
		return exitCode
	}
	fmt.Printf("mcp proxy error: %s\n", output.Error)
	return exitCode
}

func printMCPUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait mcp proxy --policy <policy.yaml> --call <tool_call.json|-> [--adapter mcp|openai|anthropic|langchain] [--profile standard|oss-prod] [--trace-out trace.json] [--run-id run_...] [--runpack-out runpack.zip] [--pack-out pack_run.zip] [--export-log-out events.jsonl] [--export-otel-out otel.jsonl] [--json] [--explain]")
	fmt.Println("  gait mcp bridge --policy <policy.yaml> --call <tool_call.json|-> [--adapter mcp|openai|anthropic|langchain] [--profile standard|oss-prod] [--trace-out trace.json] [--run-id run_...] [--runpack-out runpack.zip] [--pack-out pack_run.zip] [--export-log-out events.jsonl] [--export-otel-out otel.jsonl] [--json] [--explain]")
	fmt.Println("  gait mcp serve --policy <policy.yaml> [--listen 127.0.0.1:8787] [--adapter mcp|openai|anthropic|langchain] [--auth-mode off|token] [--auth-token-env <VAR>] [--max-request-bytes <bytes>] [--http-verdict-status compat|strict] [--allow-client-artifact-paths] [--trace-dir <dir>] [--runpack-dir <dir>] [--pack-dir <dir>] [--trace-max-age <dur>] [--trace-max-count <n>] [--runpack-max-age <dur>] [--runpack-max-count <n>] [--pack-max-age <dur>] [--pack-max-count <n>] [--session-max-age <dur>] [--session-max-count <n>] [--json] [--explain]")
	fmt.Println("    serve endpoints: POST /v1/evaluate, POST /v1/evaluate/sse, POST /v1/evaluate/stream")
}

func printMCPProxyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait mcp proxy --policy <policy.yaml> --call <tool_call.json|-> [--adapter mcp|openai|anthropic|langchain] [--profile standard|oss-prod] [--trace-out trace.json] [--run-id run_...] [--runpack-out runpack.zip] [--pack-out pack_run.zip] [--export-log-out events.jsonl] [--export-otel-out otel.jsonl] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}
