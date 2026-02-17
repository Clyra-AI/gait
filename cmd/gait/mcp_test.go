package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/mcp"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestRunMCPProxyBlockWithArtifacts(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	privateKeyPath := filepath.Join(workDir, "trace_private.key")
	writePrivateKey(t, privateKeyPath)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, `default_verdict: allow
rules:
  - name: block-write-host
    effect: block
    match:
      tool_names: [tool.write]
      target_kinds: [host]
      target_values: [api.external.com]
`)
	callPath := filepath.Join(workDir, "call.json")
	mustWriteFile(t, callPath, `{
  "name":"tool.write",
  "args":{"path":"/tmp/out.txt"},
  "target":"api.external.com",
  "context":{"identity":"alice","workspace":"/repo/gait","risk_class":"high","run_id":"run_mcp_case"}
}`)

	tracePath := filepath.Join(workDir, "trace_mcp.json")
	runpackPath := filepath.Join(workDir, "runpack_mcp.zip")
	packPath := filepath.Join(workDir, "pack_mcp.zip")
	logPath := filepath.Join(workDir, "mcp_events.jsonl")
	otelPath := filepath.Join(workDir, "mcp_otel.jsonl")

	if code := runMCPProxy([]string{
		"--policy", policyPath,
		"--call", callPath,
		"--trace-out", tracePath,
		"--runpack-out", runpackPath,
		"--pack-out", packPath,
		"--export-log-out", logPath,
		"--export-otel-out", otelPath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitPolicyBlocked {
		t.Fatalf("runMCPProxy blocked expected %d got %d", exitPolicyBlocked, code)
	}

	if _, err := os.Stat(tracePath); err != nil {
		t.Fatalf("expected trace artifact: %v", err)
	}
	if _, err := os.Stat(runpackPath); err != nil {
		t.Fatalf("expected runpack artifact: %v", err)
	}
	if _, err := os.Stat(packPath); err != nil {
		t.Fatalf("expected pack artifact: %v", err)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log export artifact: %v", err)
	}
	if _, err := os.Stat(otelPath); err != nil {
		t.Fatalf("expected otel export artifact: %v", err)
	}
	if code := runVerify([]string{"--json", runpackPath}); code != exitOK {
		t.Fatalf("runVerify expected %d got %d", exitOK, code)
	}
	if code := runPack([]string{"verify", packPath, "--json"}); code != exitOK {
		t.Fatalf("pack verify expected %d got %d", exitOK, code)
	}
}

func TestRunMCPProxyOpenAIAdapter(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, `default_verdict: allow`)
	callPath := filepath.Join(workDir, "openai_call.json")
	mustWriteFile(t, callPath, `{
  "type":"function",
  "function":{
    "name":"tool.search",
    "arguments":"{\"query\":\"gait\"}"
  }
}`)

	if code := runMCPProxy([]string{
		"--policy", policyPath,
		"--call", callPath,
		"--adapter", "openai",
		"--json",
	}); code != exitOK {
		t.Fatalf("runMCPProxy openai expected %d got %d", exitOK, code)
	}
}

func TestShouldAutoEmitMCPPack(t *testing.T) {
	tests := []struct {
		name     string
		intent   schemagate.IntentRequest
		expected bool
	}{
		{
			name: "explicit write target emits",
			intent: schemagate.IntentRequest{
				ToolName: "tool.search",
				Targets:  []schemagate.IntentTarget{{Operation: "write"}},
			},
			expected: true,
		},
		{
			name: "explicit read target does not emit",
			intent: schemagate.IntentRequest{
				ToolName: "tool.write",
				Targets:  []schemagate.IntentTarget{{Operation: "read"}},
			},
			expected: false,
		},
		{
			name: "tool name write emits",
			intent: schemagate.IntentRequest{
				ToolName: "tool.write_file",
			},
			expected: true,
		},
		{
			name: "tool name read-only does not emit",
			intent: schemagate.IntentRequest{
				ToolName: "tool.search",
			},
			expected: false,
		},
		{
			name: "empty operations in targets falls back to tool name",
			intent: schemagate.IntentRequest{
				ToolName: "tool.search",
				Targets:  []schemagate.IntentTarget{{Operation: ""}},
			},
			expected: false,
		},
		{
			name: "unknown operation remains conservative",
			intent: schemagate.IntentRequest{
				ToolName: "tool.custom",
			},
			expected: false,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			actual := shouldAutoEmitMCPPack(testCase.intent)
			if actual != testCase.expected {
				t.Fatalf("shouldAutoEmitMCPPack expected %t got %t", testCase.expected, actual)
			}
		})
	}
}

func TestRunMCPProxyDefaultTracePathIsUniquePerEmission(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, `default_verdict: allow`)
	callPath := filepath.Join(workDir, "call.json")
	mustWriteFile(t, callPath, `{
  "name":"tool.search",
  "args":{"query":"gait"},
  "context":{"identity":"alice","workspace":"/repo/gait","risk_class":"high","session_id":"sess-1"}
}`)

	var firstCode int
	firstRaw := captureStdout(t, func() {
		firstCode = runMCPProxy([]string{
			"--policy", policyPath,
			"--call", callPath,
			"--json",
		})
	})
	if firstCode != exitOK {
		t.Fatalf("first runMCPProxy expected %d got %d", exitOK, firstCode)
	}
	var first mcpProxyOutput
	if err := json.Unmarshal([]byte(firstRaw), &first); err != nil {
		t.Fatalf("decode first output: %v (%s)", err, firstRaw)
	}
	time.Sleep(2 * time.Millisecond)
	var secondCode int
	secondRaw := captureStdout(t, func() {
		secondCode = runMCPProxy([]string{
			"--policy", policyPath,
			"--call", callPath,
			"--json",
		})
	})
	if secondCode != exitOK {
		t.Fatalf("second runMCPProxy expected %d got %d", exitOK, secondCode)
	}
	var second mcpProxyOutput
	if err := json.Unmarshal([]byte(secondRaw), &second); err != nil {
		t.Fatalf("decode second output: %v (%s)", err, secondRaw)
	}
	if first.TraceID == "" || second.TraceID == "" {
		t.Fatalf("expected trace ids in outputs")
	}
	if first.TraceID != second.TraceID {
		t.Fatalf("expected deterministic trace id for identical decisions")
	}
	if first.TracePath == "" || second.TracePath == "" {
		t.Fatalf("expected trace paths in outputs")
	}
	if first.TracePath == second.TracePath {
		t.Fatalf("expected unique default trace paths, got %s", first.TracePath)
	}
	if _, err := os.Stat(first.TracePath); err != nil {
		t.Fatalf("expected first trace artifact: %v", err)
	}
	if _, err := os.Stat(second.TracePath); err != nil {
		t.Fatalf("expected second trace artifact: %v", err)
	}
}

func TestRunMCPProxyOSSProdRequiresExplicitContext(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	privateKeyPath := filepath.Join(workDir, "trace_private.key")
	writePrivateKey(t, privateKeyPath)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	missingContextPath := filepath.Join(workDir, "missing_context.json")
	mustWriteFile(t, missingContextPath, `{
  "name":"tool.search",
  "args":{"query":"gait"},
  "context":{"identity":"alice","workspace":"/repo/gait"}
}`)
	if code := runMCPProxy([]string{
		"--policy", policyPath,
		"--call", missingContextPath,
		"--profile", "oss-prod",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runMCPProxy oss-prod missing session expected %d got %d", exitInvalidInput, code)
	}

	validContextPath := filepath.Join(workDir, "valid_context.json")
	mustWriteFile(t, validContextPath, `{
  "name":"tool.search",
  "args":{"query":"gait"},
  "context":{"identity":"alice","workspace":"/repo/gait","risk_class":"high","session_id":"sess-1"}
}`)
	if code := runMCPProxy([]string{
		"--policy", policyPath,
		"--call", validContextPath,
		"--profile", "oss-prod",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runMCPProxy oss-prod valid context expected %d got %d", exitOK, code)
	}
}

func TestRunMCPProxyAdaptersSupportRunpackAndRegressInit(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy_allow.yaml")
	mustWriteFile(t, policyPath, `default_verdict: allow`)

	cases := []struct {
		name    string
		adapter string
		payload string
	}{
		{
			name:    "mcp",
			adapter: "mcp",
			payload: `{"name":"tool.search","args":{"query":"gait"}}`,
		},
		{
			name:    "openai",
			adapter: "openai",
			payload: `{"type":"function","function":{"name":"tool.search","arguments":"{\"query\":\"gait\"}"}}`,
		},
		{
			name:    "anthropic",
			adapter: "anthropic",
			payload: `{"type":"tool_use","name":"tool.search","input":{"query":"gait"}}`,
		},
		{
			name:    "langchain",
			adapter: "langchain",
			payload: `{"tool":"tool.search","tool_input":{"query":"gait"}}`,
		},
	}

	for _, testCase := range cases {
		callPath := filepath.Join(workDir, testCase.name+"_call.json")
		runpackPath := filepath.Join(workDir, testCase.name+"_runpack.zip")
		mustWriteFile(t, callPath, testCase.payload)

		if code := runMCPProxy([]string{
			"--policy", policyPath,
			"--call", callPath,
			"--adapter", testCase.adapter,
			"--runpack-out", runpackPath,
			"--json",
		}); code != exitOK {
			t.Fatalf("runMCPProxy %s expected %d got %d", testCase.adapter, exitOK, code)
		}

		if code := runVerify([]string{"--json", runpackPath}); code != exitOK {
			t.Fatalf("runVerify %s expected %d got %d", testCase.adapter, exitOK, code)
		}

		fixtureName := "fixture_" + testCase.name
		if code := runRegressInit([]string{
			"--from", runpackPath,
			"--name", fixtureName,
			"--json",
		}); code != exitOK {
			t.Fatalf("runRegressInit %s expected %d got %d", testCase.adapter, exitOK, code)
		}
	}
}

func TestRunMCPProxyValidation(t *testing.T) {
	if code := runMCPProxy([]string{}); code != exitInvalidInput {
		t.Fatalf("runMCPProxy missing args expected %d got %d", exitInvalidInput, code)
	}
	if code := runMCP([]string{}); code != exitInvalidInput {
		t.Fatalf("runMCP missing args expected %d got %d", exitInvalidInput, code)
	}
	if code := runMCP([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runMCP unknown expected %d got %d", exitInvalidInput, code)
	}
	if code := runMCP([]string{"bridge", "--help"}); code != exitOK {
		t.Fatalf("runMCP bridge help expected %d got %d", exitOK, code)
	}
	if code := runMCP([]string{"serve", "--help"}); code != exitOK {
		t.Fatalf("runMCP serve help expected %d got %d", exitOK, code)
	}
	if code := writeMCPProxyOutput(false, mcpProxyOutput{OK: true, Verdict: "allow"}, exitOK); code != exitOK {
		t.Fatalf("writeMCPProxyOutput text success expected %d got %d", exitOK, code)
	}
	if code := writeMCPProxyOutput(true, mcpProxyOutput{OK: true, Verdict: "allow"}, exitOK); code != exitOK {
		t.Fatalf("writeMCPProxyOutput json expected %d got %d", exitOK, code)
	}
	if code := writeMCPProxyOutput(false, mcpProxyOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeMCPProxyOutput text expected %d got %d", exitInvalidInput, code)
	}
	printMCPUsage()
	printMCPProxyUsage()
}

func TestRunMCPProxyPackOutWithoutRunpackOut(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	callPath := filepath.Join(workDir, "call.json")
	mustWriteFile(t, callPath, `{
  "name":"tool.write",
  "args":{"path":"/tmp/out.txt"},
  "target":"api.internal.local",
  "context":{"identity":"alice","workspace":"/repo/gait","risk_class":"high","run_id":"run_pack_only"}
}`)

	packPath := filepath.Join(workDir, "pack_only.zip")
	var runCode int
	raw := captureStdout(t, func() {
		runCode = runMCPProxy([]string{
			"--policy", policyPath,
			"--call", callPath,
			"--pack-out", packPath,
			"--json",
		})
	})
	if runCode != exitOK {
		t.Fatalf("runMCPProxy expected %d got %d", exitOK, runCode)
	}
	if _, err := os.Stat(packPath); err != nil {
		t.Fatalf("expected pack artifact: %v", err)
	}
	var output mcpProxyOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("parse mcp proxy output: %v (%s)", err, raw)
	}
	if strings.TrimSpace(output.PackPath) == "" || strings.TrimSpace(output.PackID) == "" {
		t.Fatalf("expected pack metadata in output, got %#v", output)
	}
	if code := runPack([]string{"verify", packPath, "--json"}); code != exitOK {
		t.Fatalf("pack verify expected %d got %d", exitOK, code)
	}
}

func TestReadMCPPayloadAndRunIDHelpers(t *testing.T) {
	workDir := t.TempDir()
	callPath := filepath.Join(workDir, "call.json")
	mustWriteFile(t, callPath, `{"name":"tool.read"}`)
	payload, err := readMCPPayload(callPath)
	if err != nil {
		t.Fatalf("readMCPPayload file: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if parsed["name"] != "tool.read" {
		t.Fatalf("unexpected payload: %#v", parsed)
	}
	if _, err := readMCPPayload(filepath.Join(workDir, "missing.json")); err == nil {
		t.Fatalf("expected readMCPPayload missing file error")
	}

	stdinPath := filepath.Join(workDir, "stdin_call.json")
	mustWriteFile(t, stdinPath, `{"name":"tool.stdin"}`)
	stdinFile, err := os.Open(stdinPath)
	if err != nil {
		t.Fatalf("open stdin fixture: %v", err)
	}
	defer func() {
		_ = stdinFile.Close()
	}()
	originalStdin := os.Stdin
	defer func() {
		os.Stdin = originalStdin
	}()
	os.Stdin = stdinFile
	stdinPayload, err := readMCPPayload("-")
	if err != nil {
		t.Fatalf("readMCPPayload stdin: %v", err)
	}
	if !strings.Contains(string(stdinPayload), "tool.stdin") {
		t.Fatalf("unexpected stdin payload: %s", string(stdinPayload))
	}

	if normalized := normalizeRunID(""); normalized != "" {
		t.Fatalf("expected empty normalized run id")
	}
	if normalized := normalizeRunID("my run id"); normalized != "run_my_run_id" {
		t.Fatalf("unexpected normalized run id: %s", normalized)
	}
	if normalized := normalizeRunID("run_existing"); normalized != "run_existing" {
		t.Fatalf("unexpected pre-normalized run id: %s", normalized)
	}
}

func TestSanitizeRunpackOutputPath(t *testing.T) {
	absoluteInput := filepath.Join(t.TempDir(), "nested", "runpack.zip")
	absolutePath, err := sanitizeRunpackOutputPath(absoluteInput)
	if err != nil {
		t.Fatalf("sanitize absolute runpack path: %v", err)
	}
	if absolutePath != filepath.Clean(absoluteInput) {
		t.Fatalf("unexpected absolute runpack path: %s", absolutePath)
	}

	relativePath, err := sanitizeRunpackOutputPath("./gait-out/runpack.zip")
	if err != nil {
		t.Fatalf("sanitize relative runpack path: %v", err)
	}
	if relativePath != filepath.Clean("./gait-out/runpack.zip") {
		t.Fatalf("unexpected relative runpack path: %s", relativePath)
	}

	if _, err := sanitizeRunpackOutputPath(""); err == nil {
		t.Fatalf("expected empty runpack path to fail")
	}
	if _, err := sanitizeRunpackOutputPath("../gait-out/runpack.zip"); err == nil {
		t.Fatalf("expected parent traversal runpack path to fail")
	}
	if _, err := sanitizeRunpackOutputPath("."); err == nil {
		t.Fatalf("expected dot runpack path to fail")
	}
}

func TestWriteMCPRunpackRelativePath(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	relativePath := filepath.Join("nested", "runpack_mcp_relative.zip")
	if err := writeMCPRunpack(relativePath, "run_mcp_relative", testMCPEvalResult(), "trace_relative"); err != nil {
		t.Fatalf("writeMCPRunpack relative path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, relativePath)); err != nil {
		t.Fatalf("stat relative runpack output: %v", err)
	}
}

func TestWriteMCPRunpackCreateDirectoryError(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if err := os.WriteFile("nested", []byte("blocker\n"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	if err := writeMCPRunpack(filepath.Join("nested", "runpack.zip"), "run_mcp_mkdir_error", testMCPEvalResult(), "trace_mkdir_error"); err == nil {
		t.Fatalf("expected create directory error")
	}
}

func TestWriteMCPRunpackWriteError(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	targetPath := filepath.Join(workDir, "existing-dir")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetPath, "keep.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatalf("write target sentinel: %v", err)
	}

	if err := writeMCPRunpack(targetPath, "run_mcp_write_error", testMCPEvalResult(), "trace_write_error"); err == nil {
		t.Fatalf("expected write error for directory destination")
	}
}

func TestWriteMCPRunpackRejectsTraversalPath(t *testing.T) {
	if err := writeMCPRunpack(filepath.Join("..", "runpack.zip"), "run_mcp_bad_path", testMCPEvalResult(), "trace_bad_path"); err == nil {
		t.Fatalf("expected traversal path error")
	}
}

func TestWriteMCPRunpackZeroCreatedAtUsesEpochDefault(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	evalResult := testMCPEvalResult()
	evalResult.Outcome.Result.CreatedAt = time.Time{}

	outputPath := filepath.Join("nested", "runpack_zero_created_at.zip")
	if err := writeMCPRunpack(outputPath, "run_mcp_zero_time", evalResult, "trace_zero_time"); err != nil {
		t.Fatalf("writeMCPRunpack zero created_at: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, outputPath)); err != nil {
		t.Fatalf("stat runpack output: %v", err)
	}
}

func testMCPEvalResult() mcp.EvalResult {
	now := time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC)
	return mcp.EvalResult{
		Intent: schemagate.IntentRequest{
			ToolName:   "tool.read",
			ArgsDigest: strings.Repeat("a", 64),
			Args:       map[string]any{"path": "README.md"},
		},
		Outcome: gate.EvalOutcome{
			Result: schemagate.GateResult{
				CreatedAt:   now,
				Verdict:     "allow",
				ReasonCodes: []string{"allowed"},
			},
		},
	}
}
