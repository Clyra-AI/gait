package gate

import (
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/contextproof"
	schemacontext "github.com/Clyra-AI/gait/core/schema/v1/context"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestEvaluatePolicyDetailedIgnoresUnverifiedContextEvidenceClaims(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-with-context-proof
    priority: 10
    effect: allow
    require_context_evidence: true
    max_context_age_seconds: 30
    match:
      tool_names: [tool.write]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Args = map[string]any{"path": "/tmp/out.txt"}
	intent.Targets = []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/out.txt", Operation: "write", EndpointClass: "fs.write"}}
	intent.Context.ContextSetDigest = strings.Repeat("a", 64)
	intent.Context.ContextEvidenceMode = contextproof.EvidenceModeRequired
	intent.Context.AuthContext = map[string]any{"context_age_seconds": int64(1)}

	outcome, err := EvaluatePolicyDetailed(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy detailed: %v", err)
	}
	if outcome.Result.Verdict != "block" {
		t.Fatalf("expected unverified context evidence claims to fail closed, got %#v", outcome.Result)
	}
	if !contains(outcome.Result.ReasonCodes, "context_evidence_missing") {
		t.Fatalf("expected context_evidence_missing, got %#v", outcome.Result.ReasonCodes)
	}
	if !contains(outcome.Result.ReasonCodes, "context_freshness_exceeded") {
		t.Fatalf("expected context_freshness_exceeded, got %#v", outcome.Result.ReasonCodes)
	}
	if outcome.PreparedIntent.Context.ContextSetDigest != "" {
		t.Fatalf("expected prepared intent to strip unverified context digest, got %q", outcome.PreparedIntent.Context.ContextSetDigest)
	}
}

func TestEvaluatePolicyDetailedAcceptsVerifiedContextEnvelope(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-with-context-proof
    priority: 10
    effect: allow
    require_context_evidence: true
    max_context_age_seconds: 30
    match:
      tool_names: [tool.write]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	now := time.Date(2026, time.March, 12, 12, 0, 0, 0, time.UTC)
	envelope := mustBuildTestContextEnvelope(t, now.Add(-5*time.Second))
	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Args = map[string]any{"path": "/tmp/out.txt"}
	intent.Targets = []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/out.txt", Operation: "write", EndpointClass: "fs.write"}}

	outcome, err := EvaluatePolicyDetailed(policy, intent, EvalOptions{
		VerifiedContextEnvelope: &envelope,
		ContextEvidenceNow:      now,
	})
	if err != nil {
		t.Fatalf("evaluate policy detailed: %v", err)
	}
	if outcome.Result.Verdict != "allow" {
		t.Fatalf("expected verified context envelope to satisfy policy, got %#v", outcome.Result)
	}
	if outcome.PreparedIntent.Context.ContextSetDigest != envelope.ContextSetDigest {
		t.Fatalf("expected prepared intent digest %q, got %q", envelope.ContextSetDigest, outcome.PreparedIntent.Context.ContextSetDigest)
	}
	if outcome.PreparedIntent.Context.ContextEvidenceMode != envelope.EvidenceMode {
		t.Fatalf("expected prepared intent mode %q, got %q", envelope.EvidenceMode, outcome.PreparedIntent.Context.ContextEvidenceMode)
	}
	if len(outcome.PreparedIntent.Context.ContextRefs) != len(envelope.Records) {
		t.Fatalf("expected context refs to reflect envelope records, got %#v", outcome.PreparedIntent.Context.ContextRefs)
	}
	if outcome.ContextSource != verifiedContextSource {
		t.Fatalf("expected verified context source %q, got %q", verifiedContextSource, outcome.ContextSource)
	}
}

func TestEvaluatePolicyDetailedRejectsConflictingVerifiedContextEnvelope(t *testing.T) {
	now := time.Date(2026, time.March, 12, 12, 0, 0, 0, time.UTC)
	envelope := mustBuildTestContextEnvelope(t, now.Add(-5*time.Second))
	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Context.ContextSetDigest = strings.Repeat("b", 64)

	_, err := EvaluatePolicyDetailed(Policy{DefaultVerdict: "allow"}, intent, EvalOptions{
		VerifiedContextEnvelope: &envelope,
		ContextEvidenceNow:      now,
	})
	if err == nil || !strings.Contains(err.Error(), "context envelope digest does not match") {
		t.Fatalf("expected digest conflict error, got %v", err)
	}
}

func mustBuildTestContextEnvelope(t *testing.T, retrievedAt time.Time) schemacontext.Envelope {
	t.Helper()
	envelope, err := contextproof.BuildEnvelope([]schemacontext.ReferenceRecord{
		{
			RefID:         "ctx-1",
			SourceType:    "doc",
			SourceLocator: "file:///repo/context.md",
			QueryDigest:   strings.Repeat("1", 64),
			ContentDigest: strings.Repeat("2", 64),
			RetrievedAt:   retrievedAt,
			RedactionMode: contextproof.PrivacyModeHashes,
			Immutability:  "immutable",
		},
	}, contextproof.BuildEnvelopeOptions{
		ContextSetID:    "ctx-set-1",
		EvidenceMode:    contextproof.EvidenceModeRequired,
		ProducerVersion: "test",
		CreatedAt:       retrievedAt,
	})
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	return envelope
}
