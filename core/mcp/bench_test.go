package mcp

import (
	"testing"

	"github.com/Clyra-AI/gait/core/gate"
)

func BenchmarkDecodeToolCallOpenAITypical(b *testing.B) {
	payload := []byte(`{
  "type": "function",
  "function": {
    "name": "tool.write",
    "arguments": "{\"path\":\"/tmp/out.txt\"}"
  }
}`)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		call, err := DecodeToolCall("openai", payload)
		if err != nil {
			b.Fatalf("decode tool call: %v", err)
		}
		if call.Name != "tool.write" {
			b.Fatalf("unexpected tool name: %s", call.Name)
		}
	}
}

func BenchmarkEvaluateToolCallTypical(b *testing.B) {
	policy, err := gate.ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: block-external-host
    effect: block
    match:
      tool_names: [tool.write]
      target_kinds: [host]
`))
	if err != nil {
		b.Fatalf("parse policy: %v", err)
	}
	call := ToolCall{
		Name: "tool.write",
		Args: map[string]any{
			"path": "/tmp/out.txt",
		},
		Targets: []Target{{
			Kind:  "path",
			Value: "/tmp/out.txt",
		}},
		ArgProvenance: []ArgProvenance{{
			ArgPath: "$.path",
			Source:  "user",
		}},
		Context: CallContext{
			Identity:  "bench-user",
			Workspace: "/tmp/workspace",
			RiskClass: "high",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, evalErr := EvaluateToolCall(policy, call, gate.EvalOptions{
			ProducerVersion: "0.0.0-bench",
		})
		if evalErr != nil {
			b.Fatalf("evaluate tool call: %v", evalErr)
		}
		if result.Outcome.Result.Verdict == "" {
			b.Fatalf("expected verdict")
		}
	}
}
