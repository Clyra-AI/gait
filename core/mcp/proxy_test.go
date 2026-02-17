package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Clyra-AI/gait/core/gate"
)

func TestDecodeToolCallAdapters(t *testing.T) {
	openaiPayload := []byte(`{
  "type": "function",
  "function": {
    "name": "tool.write",
    "arguments": "{\"path\":\"/tmp/out.txt\"}"
  }
}`)
	call, err := DecodeToolCall("openai", openaiPayload)
	if err != nil {
		t.Fatalf("decode openai call: %v", err)
	}
	if call.Name != "tool.write" || call.Args["path"] != "/tmp/out.txt" {
		t.Fatalf("unexpected openai call: %#v", call)
	}

	anthropicPayload := []byte(`{
  "type": "tool_use",
  "name": "tool.fetch",
  "input": {"url":"https://example.local"}
}`)
	call, err = DecodeToolCall("anthropic", anthropicPayload)
	if err != nil {
		t.Fatalf("decode anthropic call: %v", err)
	}
	if call.Name != "tool.fetch" || call.Args["url"] != "https://example.local" {
		t.Fatalf("unexpected anthropic call: %#v", call)
	}

	langchainPayload := []byte(`{
  "tool": "tool.search",
  "tool_input": {"query":"gait"}
}`)
	call, err = DecodeToolCall("langchain", langchainPayload)
	if err != nil {
		t.Fatalf("decode langchain call: %v", err)
	}
	if call.Name != "tool.search" || call.Args["query"] != "gait" {
		t.Fatalf("unexpected langchain call: %#v", call)
	}

	mcpPayload := []byte(`{
  "name":"tool.read",
  "args":{"path":"/tmp/in.txt"},
  "target":"api.external.com"
}`)
	call, err = DecodeToolCall("mcp", mcpPayload)
	if err != nil {
		t.Fatalf("decode mcp call: %v", err)
	}
	if call.Name != "tool.read" || call.Target != "api.external.com" {
		t.Fatalf("unexpected mcp call: %#v", call)
	}

	if _, err := DecodeToolCall("unsupported", []byte(`{}`)); err == nil {
		t.Fatalf("expected unsupported adapter error")
	}
}

func TestEvaluateToolCallDefaultsAndLegacyTarget(t *testing.T) {
	policy, err := gate.ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: block-host
    effect: block
    match:
      tool_names: [tool.write]
      target_kinds: [host]
      target_values: [api.external.com]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	result, err := EvaluateToolCall(policy, ToolCall{
		Name:   "tool.write",
		Args:   map[string]any{"path": "/tmp/out.txt"},
		Target: "api.external.com",
	}, gate.EvalOptions{ProducerVersion: "0.0.0-test"})
	if err != nil {
		t.Fatalf("evaluate tool call: %v", err)
	}
	if result.Intent.Context.Identity != defaultIdentity || result.Intent.Context.Workspace != defaultWorkspace {
		t.Fatalf("unexpected default context: %#v", result.Intent.Context)
	}
	if result.Intent.Targets[0].Kind != "host" {
		t.Fatalf("expected inferred host target: %#v", result.Intent.Targets)
	}
	if result.Outcome.Result.Verdict != "block" {
		t.Fatalf("expected block verdict, got %#v", result.Outcome.Result)
	}
}

func TestToIntentRequestWithStrictContext(t *testing.T) {
	_, err := ToIntentRequestWithOptions(ToolCall{
		Name: "tool.write",
		Args: map[string]any{"path": "/tmp/out.txt"},
		Context: CallContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
	}, IntentOptions{RequireExplicitContext: true})
	if err == nil {
		t.Fatalf("expected strict context conversion to fail without session_id")
	}

	intent, err := ToIntentRequestWithOptions(ToolCall{
		Name: "tool.write",
		Args: map[string]any{"path": "/tmp/out.txt"},
		Context: CallContext{
			Identity:               "alice",
			Workspace:              "/repo/gait",
			RiskClass:              "high",
			SessionID:              "sess-1",
			AuthContext:            map[string]any{"provider": "oidc"},
			CredentialScopes:       []string{"tool:tool.write"},
			EnvironmentFingerprint: "env:test",
		},
		Delegation: &Delegation{
			RequesterIdentity: "agent.specialist",
			ScopeClass:        "write",
			TokenRefs:         []string{"delegation_a"},
			Chain: []DelegationLink{
				{DelegatorIdentity: "agent.lead", DelegateIdentity: "agent.specialist", ScopeClass: "write"},
			},
		},
	}, IntentOptions{RequireExplicitContext: true})
	if err != nil {
		t.Fatalf("strict context conversion failed: %v", err)
	}
	if intent.Context.SessionID != "sess-1" {
		t.Fatalf("expected session id in converted intent context")
	}
	if intent.Delegation == nil || intent.Delegation.RequesterIdentity != "agent.specialist" {
		t.Fatalf("expected delegation passthrough in converted intent: %#v", intent.Delegation)
	}
}

func TestExportersWriteJSONL(t *testing.T) {
	workDir := t.TempDir()
	logPath := filepath.Join(workDir, "mcp.log.jsonl")
	otelPath := filepath.Join(workDir, "mcp.otel.jsonl")
	event := ExportEvent{
		RunID:           "run_mcp_test",
		TraceID:         "trace_1",
		ToolName:        "tool.write",
		Verdict:         "allow",
		PolicyDigest:    strings.Repeat("a", 64),
		IntentDigest:    strings.Repeat("b", 64),
		DecisionLatency: 42,
		ProducerVersion: "0.0.0-test",
	}
	if err := ExportLogEvent(logPath, event); err != nil {
		t.Fatalf("export log event: %v", err)
	}
	if err := ExportOTelEvent(otelPath, event); err != nil {
		t.Fatalf("export otel event: %v", err)
	}

	logRaw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log export: %v", err)
	}
	var logEntry map[string]any
	if err := json.Unmarshal(bytesTrimNewline(logRaw), &logEntry); err != nil {
		t.Fatalf("parse log export: %v", err)
	}
	if logEntry["run_id"] != "run_mcp_test" {
		t.Fatalf("unexpected log run id: %#v", logEntry)
	}
	if logEntry["decision_latency_ms"] != float64(42) {
		t.Fatalf("unexpected log decision latency: %#v", logEntry)
	}

	otelRaw, err := os.ReadFile(otelPath)
	if err != nil {
		t.Fatalf("read otel export: %v", err)
	}
	var otelEntry map[string]any
	if err := json.Unmarshal(bytesTrimNewline(otelRaw), &otelEntry); err != nil {
		t.Fatalf("parse otel export: %v", err)
	}
	attrs, ok := otelEntry["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected otel attributes: %#v", otelEntry)
	}
	if attrs["gait.run_id"] != "run_mcp_test" {
		t.Fatalf("unexpected otel run id: %#v", attrs)
	}
	if attrs["gait.decision_latency_ms"] != float64(42) {
		t.Fatalf("unexpected otel decision latency: %#v", attrs)
	}
}

func TestDecodeArgumentsBranches(t *testing.T) {
	cases := []struct {
		name      string
		input     any
		wantKey   string
		wantValue any
		wantErr   bool
	}{
		{name: "nil", input: nil, wantErr: false},
		{name: "map", input: map[string]any{"k": "v"}, wantKey: "k", wantValue: "v", wantErr: false},
		{name: "empty string", input: " ", wantErr: false},
		{name: "json string", input: `{"a":1}`, wantKey: "a", wantValue: float64(1), wantErr: false},
		{name: "invalid json string", input: "{", wantErr: true},
		{name: "struct-like", input: map[string]string{"x": "y"}, wantKey: "x", wantValue: "y", wantErr: false},
	}
	for _, testCase := range cases {
		got, err := decodeArguments(testCase.input)
		if testCase.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", testCase.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", testCase.name, err)
		}
		if testCase.wantKey == "" {
			continue
		}
		if got[testCase.wantKey] != testCase.wantValue {
			t.Fatalf("%s: unexpected decode result: %#v", testCase.name, got)
		}
	}
}

func TestEvaluateToolCallValidationError(t *testing.T) {
	policy, err := gate.ParsePolicyYAML([]byte("default_verdict: allow\n"))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	if _, err := EvaluateToolCall(policy, ToolCall{}, gate.EvalOptions{ProducerVersion: "0.0.0-test"}); err == nil {
		t.Fatalf("expected EvaluateToolCall to fail when name is empty")
	}
}

func TestAppendJSONLErrorBranches(t *testing.T) {
	workDir := t.TempDir()
	nestedPath := filepath.Join(workDir, "nested", "events.jsonl")
	if err := appendJSONL(nestedPath, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("appendJSONL nested path: %v", err)
	}
	dirPath := filepath.Join(workDir, "dir_as_file")
	if err := os.MkdirAll(dirPath, 0o750); err != nil {
		t.Fatalf("mkdir dir_as_file: %v", err)
	}
	if err := appendJSONL(dirPath, []byte(`{"ok":false}`)); err == nil {
		t.Fatalf("expected appendJSONL to fail when target path is a directory")
	}
}

func TestAppendJSONLConcurrentIntegrity(t *testing.T) {
	workDir := t.TempDir()
	logPath := filepath.Join(workDir, "events.jsonl")
	const workers = 200
	var group sync.WaitGroup
	group.Add(workers)
	for i := 0; i < workers; i++ {
		line := []byte(fmt.Sprintf(`{"index":%d}`, i))
		go func(payload []byte) {
			defer group.Done()
			if err := appendJSONL(logPath, payload); err != nil {
				t.Errorf("appendJSONL: %v", err)
			}
		}(line)
	}
	group.Wait()
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read events log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != workers {
		t.Fatalf("unexpected lines count: got=%d want=%d", len(lines), workers)
	}
	for index, line := range lines {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Fatalf("line %d invalid json: %v (%q)", index+1, err, line)
		}
	}
}

func bytesTrimNewline(raw []byte) []byte {
	return []byte(strings.TrimSpace(string(raw)))
}
