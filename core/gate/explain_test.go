package gate

import (
	"strings"
	"testing"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestBuildPolicyExplainDeterministicOrdering(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: rule-b
    priority: 20
    effect: block
    match:
      tool_names: [tool.write]
  - name: rule-a
    priority: 20
    effect: block
    match:
      tool_names: [tool.write]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.ToolName = "tool.write"
	outcome := EvalOutcome{
		Result: schemagate.GateResult{
			SchemaID:      "gait.gate.result",
			SchemaVersion: "1.0.0",
			Verdict:       "block",
			ReasonCodes: []string{
				"fail_closed_missing_agent_id",
				"kill_switch_state_unavailable",
				"credential_evidence_missing",
			},
			Violations: []string{"kill_switch_state_unavailable"},
		},
		PreparedIntent: intent,
		MatchedRule:    "rule-a,rule-b",
		BrokerScopes:   []string{"write"},
		KillSwitch:     &schemagate.KillSwitchDecision{Status: "unavailable", ReasonCodes: []string{"kill_switch_state_unavailable"}},
		FreezeWindow:   &schemagate.FreezeWindowDecision{Status: "inactive"},
		Sandbox:        &schemagate.SandboxDecision{Status: "missing", ReasonCodes: []string{"sandbox_metadata_missing"}},
	}
	explain := BuildPolicyExplain(policy, outcome, BuildPolicyExplainOptions{ProducerVersion: "test"})
	if len(explain.MatchedRules) != 2 || explain.MatchedRules[0].Name != "rule-a" || explain.MatchedRules[1].Name != "rule-b" {
		t.Fatalf("expected deterministic matched rule order, got %#v", explain.MatchedRules)
	}
	if len(explain.MissingFields) != 3 || explain.MissingFields[0] != "agent_id" || explain.MissingFields[1] != "credential_evidence" || explain.MissingFields[2] != "kill_switch_state" {
		t.Fatalf("unexpected deterministic missing fields ordering: %#v", explain.MissingFields)
	}
	if len(explain.FailClosedReasonCodes) != 1 || explain.FailClosedReasonCodes[0] != "fail_closed_missing_agent_id" {
		t.Fatalf("unexpected fail-closed reason codes: %#v", explain.FailClosedReasonCodes)
	}
}

func TestExplainHelpers(t *testing.T) {
	intent := baseIntent()
	if status := explainContextEvidenceStatus(intent); status != "not_required" {
		t.Fatalf("expected not_required context evidence status, got %q", status)
	}
	intent.Context.ContextSetDigest = strings.Repeat("a", 64)
	intent.Context.ContextRefs = []string{"ctx:1"}
	if status := explainContextEvidenceStatus(intent); status != "present" {
		t.Fatalf("expected present context evidence status, got %q", status)
	}
	intent.Context.ContextEvidenceMode = "required"
	if status := explainContextEvidenceStatus(intent); status != "verified" {
		t.Fatalf("expected verified context evidence status, got %q", status)
	}
	intent.Context.ContextRefs = nil
	if status := explainContextEvidenceStatus(intent); status != "missing" {
		t.Fatalf("expected missing context evidence status, got %q", status)
	}

	intent.Context.CredentialRef = "env:GAIT_TOKEN:deadbeef"
	intent.Context.CredentialSource = "env"
	intent.Context.CredentialAccessType = "jit"
	intent.Context.CredentialIssuer = "github.com"
	intent.Context.CredentialTTLSeconds = 300
	credentialPosture := explainCredentialPosture(intent)
	if credentialPosture == nil || !credentialPosture.Present || credentialPosture.Source != "env" {
		t.Fatalf("unexpected credential posture: %#v", credentialPosture)
	}
}
