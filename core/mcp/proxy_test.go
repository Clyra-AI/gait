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

type testStringer string

func (value testStringer) String() string {
	return string(value)
}

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

	claudePayload := []byte(`{
  "session_id":"sess-claude-1",
  "tool_name":"Bash",
  "tool_input":{"command":"npm test"},
  "hook_event_name":"PreToolUse"
}`)
	call, err = DecodeToolCall("claude_code", claudePayload)
	if err != nil {
		t.Fatalf("decode claude code call: %v", err)
	}
	if call.Name != "tool.exec" || call.Args["command"] != "npm test" {
		t.Fatalf("unexpected claude code call: %#v", call)
	}
	if call.Context.SessionID != "sess-claude-1" {
		t.Fatalf("expected claude code session_id passthrough: %#v", call.Context)
	}
	if hookName, ok := call.Context.AuthContext["hook_event_name"].(string); !ok || hookName != "PreToolUse" {
		t.Fatalf("expected claude code hook_event_name passthrough: %#v", call.Context.AuthContext)
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

func TestDecodeToolCallClaudeCodeFallbackNameAndTargets(t *testing.T) {
	payload := []byte(`{
  "tool_name":"NotebookEdit",
  "tool_input":{"file_path":"/tmp/notebook.ipynb"}
}`)
	call, err := DecodeToolCall("claude-code", payload)
	if err != nil {
		t.Fatalf("decode claude code fallback: %v", err)
	}
	if call.Name != "tool.write" {
		t.Fatalf("unexpected mapped tool name: %#v", call)
	}
	if len(call.Targets) != 1 || call.Targets[0].Kind != "path" || call.Targets[0].Operation != "write" {
		t.Fatalf("expected inferred path write target, got: %#v", call.Targets)
	}

	unknownPayload := []byte(`{
  "name":"Custom Action",
  "input":{"arg":"value"}
}`)
	call, err = DecodeToolCall("claudecode", unknownPayload)
	if err != nil {
		t.Fatalf("decode claudecode unknown mapping: %v", err)
	}
	if call.Name != "tool.custom_action" {
		t.Fatalf("expected fallback tool.custom_action, got %q", call.Name)
	}
}

func TestDecodeToolCallClaudeCodeErrorBranches(t *testing.T) {
	if _, err := DecodeToolCall("claude_code", []byte(`{`)); err == nil {
		t.Fatalf("expected parse error for malformed claude_code payload")
	}
	if _, err := DecodeToolCall("claude_code", []byte(`{"tool_name":" "}`)); err == nil {
		t.Fatalf("expected name-required error for empty claude_code tool name")
	}
	if _, err := DecodeToolCall("claude_code", []byte(`{"tool_name":"Read","tool_input":["not","an","object"]}`)); err == nil {
		t.Fatalf("expected decode error for invalid claude_code tool_input")
	}

	call, err := DecodeToolCall("claude_code", []byte(`{
  "name":"Read",
  "input":{"url":"https://example.local/resource"}
}`))
	if err != nil {
		t.Fatalf("decode claude_code legacy input fallback: %v", err)
	}
	if call.Name != "tool.read" {
		t.Fatalf("expected fallback name mapping to tool.read, got %q", call.Name)
	}
	if len(call.Targets) != 1 || call.Targets[0].Kind != "url" || call.Targets[0].Operation != "read" {
		t.Fatalf("expected url read target from legacy input fallback, got %#v", call.Targets)
	}
}

func TestNormalizeClaudeCodeToolName(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "", want: ""},
		{raw: "Web Search", want: "tool.read"},
		{raw: "Notebook-Edit", want: "tool.write"},
		{raw: "tool.custom.ACTION", want: "tool.custom.action"},
		{raw: "Custom Action", want: "tool.custom_action"},
	}
	for _, testCase := range cases {
		if got := normalizeClaudeCodeToolName(testCase.raw); got != testCase.want {
			t.Fatalf("normalizeClaudeCodeToolName(%q): got=%q want=%q", testCase.raw, got, testCase.want)
		}
	}
}

func TestInferClaudeCodeTargetsAndFirstNonEmptyString(t *testing.T) {
	readTargets := inferClaudeCodeTargets("tool.read", map[string]any{
		"file_path": "/tmp/read.txt",
		"url":       "https://example.local/read",
	})
	if len(readTargets) != 2 || readTargets[0].Kind != "path" || readTargets[1].Kind != "url" {
		t.Fatalf("unexpected read targets: %#v", readTargets)
	}

	writeTargets := inferClaudeCodeTargets("tool.write", map[string]any{
		"filepath": "/tmp/write.txt",
	})
	if len(writeTargets) != 1 || writeTargets[0].Kind != "path" || writeTargets[0].Operation != "write" {
		t.Fatalf("unexpected write targets: %#v", writeTargets)
	}

	execTargets := inferClaudeCodeTargets("tool.exec", map[string]any{
		"cmd": testStringer("npm test"),
	})
	if len(execTargets) != 1 || execTargets[0].Kind != "other" || execTargets[0].Value != "npm test" {
		t.Fatalf("unexpected exec targets: %#v", execTargets)
	}

	delegateTargets := inferClaudeCodeTargets("tool.delegate", map[string]any{
		"description": "review the failing run",
	})
	if len(delegateTargets) != 1 || delegateTargets[0].Operation != "delegate" {
		t.Fatalf("unexpected delegate targets: %#v", delegateTargets)
	}

	if unknown := inferClaudeCodeTargets("tool.unknown", map[string]any{"path": "/tmp/ignored"}); unknown != nil {
		t.Fatalf("expected nil targets for unknown tool, got %#v", unknown)
	}

	if got := firstNonEmptyString(map[string]any{
		"first":  " ",
		"second": testStringer("fallback"),
	}, "first", "second"); got != "fallback" {
		t.Fatalf("firstNonEmptyString did not use Stringer fallback, got %q", got)
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
			AuthMode:               "oauth_dcr",
			OAuthEvidence:          &OAuthEvidence{Issuer: "https://auth.example.com", ClientID: "cli-123"},
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
	if mode, ok := intent.Context.AuthContext["auth_mode"].(string); !ok || mode != "oauth_dcr" {
		t.Fatalf("expected auth_mode to be propagated into auth_context: %#v", intent.Context.AuthContext)
	}
	if oauth, ok := intent.Context.AuthContext["oauth_evidence"]; !ok || oauth == nil {
		t.Fatalf("expected oauth_evidence to be propagated into auth_context: %#v", intent.Context.AuthContext)
	}
}

func TestToIntentRequestWrapper(t *testing.T) {
	intent, err := ToIntentRequest(ToolCall{
		Name: "tool.read",
		Args: map[string]any{"path": "/tmp/input.txt"},
		Targets: []Target{
			{
				Kind:      "path",
				Value:     "/tmp/input.txt",
				Operation: "read",
			},
		},
	})
	if err != nil {
		t.Fatalf("ToIntentRequest wrapper failed: %v", err)
	}
	if intent.ToolName != "tool.read" {
		t.Fatalf("unexpected tool name from ToIntentRequest wrapper: %q", intent.ToolName)
	}
}

func TestToIntentRequestScriptPayload(t *testing.T) {
	intent, err := ToIntentRequest(ToolCall{
		Script: &ScriptCall{
			Steps: []ScriptStep{
				{
					Name: "tool.read",
					Args: map[string]any{"path": "/tmp/input.txt"},
					Targets: []Target{{
						Kind:      "path",
						Value:     "/tmp/input.txt",
						Operation: "read",
					}},
				},
				{
					Name: "tool.write",
					Args: map[string]any{"path": "/tmp/output.txt"},
					Targets: []Target{{
						Kind:      "path",
						Value:     "/tmp/output.txt",
						Operation: "write",
					}},
				},
			},
		},
		Context: CallContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
	})
	if err != nil {
		t.Fatalf("ToIntentRequest script payload failed: %v", err)
	}
	if intent.Script == nil || len(intent.Script.Steps) != 2 {
		t.Fatalf("expected script steps in intent conversion: %#v", intent.Script)
	}
	if intent.ToolName != "script" {
		t.Fatalf("expected script tool name for script payload, got %q", intent.ToolName)
	}
	if intent.Script.Steps[0].ToolName != "tool.read" || intent.Script.Steps[1].ToolName != "tool.write" {
		t.Fatalf("unexpected converted script steps: %#v", intent.Script.Steps)
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
