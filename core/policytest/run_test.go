package policytest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemapolicytest "github.com/Clyra-AI/gait/core/schema/v1/policytest"
)

func TestRunDeterministic(t *testing.T) {
	policy, err := gate.ParsePolicyYAML([]byte(`
default_verdict: require_approval
rules:
  - name: require-write
    effect: require_approval
    match:
      tool_names: [tool.write]
    reason_codes: [approval_required]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := schemagate.IntentRequest{
		ToolName: "tool.write",
		Args: map[string]any{
			"path": "/tmp/out.txt",
		},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/out.txt"},
		},
		ArgProvenance: []schemagate.IntentArgProvenance{
			{ArgPath: "args.path", Source: "user"},
		},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
	}

	first, err := Run(RunOptions{Policy: policy, Intent: intent, ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("run first: %v", err)
	}
	second, err := Run(RunOptions{Policy: policy, Intent: intent, ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("run second: %v", err)
	}

	firstJSON, err := json.Marshal(first.Result)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second.Result)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("expected deterministic result json")
	}
	if first.Result.Verdict != "require_approval" {
		t.Fatalf("unexpected verdict: %#v", first.Result)
	}
	if first.Result.PolicyDigest == "" || first.Result.IntentDigest == "" {
		t.Fatalf("expected non-empty digests: %#v", first.Result)
	}
	if first.Result.MatchedRule != "require-write" {
		t.Fatalf("expected matched rule require-write, got %q", first.Result.MatchedRule)
	}
	if first.Summary == "" {
		t.Fatalf("expected summary to be set")
	}
}

func TestBoundedSummary(t *testing.T) {
	result := schemapolicytest.PolicyTestResult{
		Verdict: "block",
		ReasonCodes: []string{
			strings.Repeat("r", 200),
			strings.Repeat("s", 200),
		},
		Violations: []string{
			strings.Repeat("v", 200),
		},
	}
	summary := boundedSummary(result, 80)
	if len(summary) > 80 {
		t.Fatalf("summary exceeds bound: %d", len(summary))
	}
	if !strings.Contains(summary, "policy test verdict=block") {
		t.Fatalf("unexpected summary prefix: %s", summary)
	}
}

func TestRunValidationErrors(t *testing.T) {
	validPolicy, err := gate.ParsePolicyYAML([]byte(`default_verdict: allow`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	validIntent := schemagate.IntentRequest{
		ToolName: "tool.read",
		Args:     map[string]any{},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "low",
		},
	}

	if _, err := Run(RunOptions{
		Policy: gate.Policy{
			SchemaID: "invalid",
		},
		Intent: validIntent,
	}); err == nil {
		t.Fatalf("expected invalid policy error")
	}

	invalidIntent := validIntent
	invalidIntent.Context.Workspace = ""
	if _, err := Run(RunOptions{
		Policy: validPolicy,
		Intent: invalidIntent,
	}); err == nil {
		t.Fatalf("expected invalid intent error")
	}
}
