package gate

import (
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestEvaluatePolicyAgentIdentityLifecycleControls(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: protect-high-risk-agent
    effect: allow
    require_declared_agent: true
    allowed_agent_ids: [agent.prod.writer]
    required_agent_manifest_digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    allowed_agent_manifest_publishers: [acme]
    allowed_agent_manifest_sources: [registry]
    required_agent_lifecycle_states: [approved, active]
    require_agent_owner: true
    require_unexpired_agent: true
    match:
      risk_classes: [high]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	base := baseIntent()
	base.Context.RiskClass = "high"
	base.Context.AgentID = "agent.prod.writer"
	base.Context.AgentIdentity = &schemagate.AgentIdentity{
		LifecycleStates: []string{"approved", "active"},
		Owner:           "platform-security",
		ManifestDigest:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Publisher:       "acme",
		Source:          "registry",
		IssuedAt:        time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ApprovedAt:      time.Date(2026, time.February, 5, 1, 0, 0, 0, time.UTC),
		ExpiresAt:       time.Date(2026, time.February, 6, 0, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name        string
		mutate      func(*schemagate.IntentRequest)
		wantVerdict string
		wantReason  string
	}{
		{
			name:        "valid_agent_allows",
			mutate:      func(intent *schemagate.IntentRequest) {},
			wantVerdict: "allow",
		},
		{
			name: "unknown_agent_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.AgentID = ""
				intent.Context.AgentIdentity = nil
			},
			wantVerdict: "block",
			wantReason:  "agent_unknown",
		},
		{
			name: "revoked_agent_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.AgentIdentity.Revoked = true
			},
			wantVerdict: "block",
			wantReason:  "agent_revoked",
		},
		{
			name: "expired_agent_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.AgentIdentity.IssuedAt = time.Date(2026, time.February, 3, 0, 0, 0, 0, time.UTC)
				intent.Context.AgentIdentity.ApprovedAt = time.Date(2026, time.February, 3, 1, 0, 0, 0, time.UTC)
				intent.Context.AgentIdentity.ExpiresAt = time.Date(2026, time.February, 4, 0, 0, 0, 0, time.UTC)
			},
			wantVerdict: "block",
			wantReason:  "agent_expired",
		},
		{
			name: "manifest_mismatch_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.AgentIdentity.ManifestDigest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			},
			wantVerdict: "block",
			wantReason:  "agent_manifest_mismatch",
		},
		{
			name: "owner_missing_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.AgentIdentity.Owner = ""
			},
			wantVerdict: "block",
			wantReason:  "agent_owner_missing",
		},
		{
			name: "lifecycle_state_invalid_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.AgentIdentity.LifecycleStates = []string{"approved"}
			},
			wantVerdict: "block",
			wantReason:  "agent_lifecycle_state_invalid",
		},
	}

	now := time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := base
			if base.Context.AgentIdentity != nil {
				clone := *base.Context.AgentIdentity
				intent.Context.AgentIdentity = &clone
			}
			test.mutate(&intent)
			outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{ContextEvidenceNow: now})
			if evalErr != nil {
				t.Fatalf("evaluate policy: %v", evalErr)
			}
			if outcome.Result.Verdict != test.wantVerdict {
				t.Fatalf("unexpected verdict: %#v", outcome.Result)
			}
			if test.wantReason != "" && !contains(outcome.Result.ReasonCodes, test.wantReason) {
				t.Fatalf("expected reason code %q in %#v", test.wantReason, outcome.Result.ReasonCodes)
			}
		})
	}
}
