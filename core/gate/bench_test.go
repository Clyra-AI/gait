package gate

import (
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func BenchmarkEvaluatePolicyTypical(b *testing.B) {
	policy, err := ParsePolicyYAML([]byte(`
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: require_approval
fail_closed:
  enabled: true
  risk_classes: [high, critical]
  required_fields: [targets, arg_provenance]
rules:
  - name: allow-safe-write
    priority: 10
    effect: allow
    match:
      tool_names: [tool.write]
      target_kinds: [path]
      workspace_prefixes: [/tmp]
      provenance_sources: [user]
    reason_codes: [safe_write]
  - name: block-external-host
    priority: 20
    effect: block
    match:
      tool_names: [tool.write]
      target_kinds: [host]
    reason_codes: [blocked_external]
    violations: [external_egress]
`))
	if err != nil {
		b.Fatalf("parse policy: %v", err)
	}

	intent := schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-bench",
		ToolName:        "tool.write",
		Args: map[string]any{
			"path":    "/tmp/out.txt",
			"content": "hello",
		},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/out.txt"},
		},
		ArgProvenance: []schemagate.IntentArgProvenance{
			{ArgPath: "$.path", Source: "user"},
		},
		Context: schemagate.IntentContext{
			Identity:  "bench-user",
			Workspace: "/tmp/workspace",
			RiskClass: "high",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, evalErr := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "0.0.0-bench"})
		if evalErr != nil {
			b.Fatalf("evaluate policy: %v", evalErr)
		}
		if result.Verdict == "" {
			b.Fatalf("empty verdict")
		}
	}
}
