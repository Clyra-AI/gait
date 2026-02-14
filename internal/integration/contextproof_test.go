package integration

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/contextproof"
	"github.com/davidahmann/gait/core/gate"
	"github.com/davidahmann/gait/core/pack"
	"github.com/davidahmann/gait/core/runpack"
	schemacontext "github.com/davidahmann/gait/core/schema/v1/context"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/sign"
)

func TestContextProofCaptureGatePackFlow(t *testing.T) {
	workDir := t.TempDir()
	createdAt := time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC)

	records := []schemacontext.ReferenceRecord{
		{
			RefID:               "ctx_b",
			SourceType:          "doc_store",
			SourceLocator:       "docs://security/b",
			QueryDigest:         strings.Repeat("1", 64),
			ContentDigest:       strings.Repeat("2", 64),
			RetrievedAt:         createdAt.Add(2 * time.Minute),
			RedactionMode:       "reference",
			Immutability:        "immutable",
			FreshnessSLASeconds: 600,
		},
		{
			RefID:               "ctx_a",
			SourceType:          "doc_store",
			SourceLocator:       "docs://security/a",
			QueryDigest:         strings.Repeat("3", 64),
			ContentDigest:       strings.Repeat("4", 64),
			RetrievedAt:         createdAt,
			RedactionMode:       "reference",
			Immutability:        "immutable",
			FreshnessSLASeconds: 600,
		},
	}
	envelopeA, err := contextproof.BuildEnvelope(records, contextproof.BuildEnvelopeOptions{
		ContextSetID:    "ctx_set_integration",
		EvidenceMode:    contextproof.EvidenceModeRequired,
		ProducerVersion: "0.0.0-test",
		CreatedAt:       createdAt,
	})
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	envelopeB, err := contextproof.BuildEnvelope([]schemacontext.ReferenceRecord{records[1], records[0]}, contextproof.BuildEnvelopeOptions{
		ContextSetID:    "ctx_set_integration",
		EvidenceMode:    contextproof.EvidenceModeRequired,
		ProducerVersion: "0.0.0-test",
		CreatedAt:       createdAt,
	})
	if err != nil {
		t.Fatalf("build envelope (reordered): %v", err)
	}
	if envelopeA.ContextSetDigest != envelopeB.ContextSetDigest {
		t.Fatalf("expected deterministic digest across record ordering")
	}
	if err := contextproof.VerifyEnvelope(envelopeA); err != nil {
		t.Fatalf("verify envelope: %v", err)
	}

	run := schemarunpack.Run{
		SchemaID:        "gait.runpack.run",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: "0.0.0-test",
		RunID:           "run_ctx_integration",
		Env: schemarunpack.RunEnv{
			OS:      "test",
			Arch:    "test",
			Runtime: "go",
		},
	}
	refs := schemarunpack.Refs{
		SchemaID:        "gait.runpack.refs",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: "0.0.0-test",
		RunID:           run.RunID,
		Receipts:        []schemarunpack.RefReceipt{},
	}
	contextproof.ApplyEnvelopeToRefs(&refs, envelopeA)

	runpackPath := filepath.Join(workDir, "runpack_ctx.zip")
	if _, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run:         run,
		Intents:     nil,
		Results:     nil,
		Refs:        refs,
		CaptureMode: "reference",
	}); err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	packPath := filepath.Join(workDir, "pack_ctx.zip")
	if _, err := pack.BuildRunPack(pack.BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  packPath,
	}); err != nil {
		t.Fatalf("build run pack: %v", err)
	}
	inspectResult, err := pack.Inspect(packPath)
	if err != nil {
		t.Fatalf("inspect run pack: %v", err)
	}
	if inspectResult.RunPayload == nil {
		t.Fatalf("expected run payload in inspect result")
	}
	if inspectResult.RunPayload.ContextSetDigest != envelopeA.ContextSetDigest {
		t.Fatalf("run payload context digest mismatch")
	}
	foundContextEnvelope := false
	for _, entry := range inspectResult.Manifest.Contents {
		if entry.Path == "context_envelope.json" {
			foundContextEnvelope = true
			break
		}
	}
	if !foundContextEnvelope {
		t.Fatalf("expected context_envelope.json in pack contents")
	}

	runtimeOnlyRefs := refs
	runtimeOnlyRefs.Receipts = append([]schemarunpack.RefReceipt(nil), refs.Receipts...)
	runtimeOnlyRefs.Receipts[0].RetrievedAt = runtimeOnlyRefs.Receipts[0].RetrievedAt.Add(5 * time.Minute)
	classification, changed, runtimeOnly, err := contextproof.ClassifyRefsDrift(refs, runtimeOnlyRefs)
	if err != nil {
		t.Fatalf("classify refs drift: %v", err)
	}
	if classification != "runtime_only" || !changed || !runtimeOnly {
		t.Fatalf("unexpected runtime-only classification: class=%s changed=%t runtime_only=%t", classification, changed, runtimeOnly)
	}

	policy, err := gate.ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: require-context
    priority: 1
    effect: allow
    match:
      risk_classes: [high]
    require_context_evidence: true
    required_context_evidence_mode: required
    max_context_age_seconds: 300
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: "0.0.0-test",
		ToolName:        "tool.write",
		Args:            map[string]any{"path": "/tmp/out.txt"},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/out.txt", Operation: "write"},
		},
		Context: schemagate.IntentContext{
			Identity:            "alice",
			Workspace:           "/repo/gait",
			RiskClass:           "high",
			ContextSetDigest:    envelopeA.ContextSetDigest,
			ContextEvidenceMode: "required",
			ContextRefs:         []string{"ctx_a", "ctx_b"},
			AuthContext: map[string]any{
				"context_age_seconds": 60,
			},
		},
	}

	allowResult, err := gate.EvaluatePolicy(policy, intent, gate.EvalOptions{ProducerVersion: "0.0.0-test"})
	if err != nil {
		t.Fatalf("evaluate context-ready intent: %v", err)
	}
	if allowResult.Verdict != "allow" {
		t.Fatalf("expected allow verdict, got %s (%v)", allowResult.Verdict, allowResult.ReasonCodes)
	}

	intentMissing := intent
	intentMissing.Context.ContextSetDigest = ""
	blockResult, err := gate.EvaluatePolicy(policy, intentMissing, gate.EvalOptions{ProducerVersion: "0.0.0-test"})
	if err != nil {
		t.Fatalf("evaluate missing context intent: %v", err)
	}
	if blockResult.Verdict != "block" {
		t.Fatalf("expected block verdict, got %s", blockResult.Verdict)
	}
	if !containsReason(blockResult.ReasonCodes, "context_evidence_missing") {
		t.Fatalf("expected context_evidence_missing reason, got %v", blockResult.ReasonCodes)
	}

	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	tracePath := filepath.Join(workDir, "trace_context.json")
	traceResult, err := gate.EmitSignedTrace(policy, intent, allowResult, gate.EmitTraceOptions{
		ProducerVersion:   "0.0.0-test",
		SigningPrivateKey: keyPair.Private,
		TracePath:         tracePath,
	})
	if err != nil {
		t.Fatalf("emit trace: %v", err)
	}
	if traceResult.Trace.ContextSetDigest != envelopeA.ContextSetDigest {
		t.Fatalf("trace context digest mismatch")
	}
	if traceResult.Trace.ContextEvidenceMode != "required" || traceResult.Trace.ContextRefCount != 2 {
		t.Fatalf("trace context summary mismatch: mode=%s count=%d", traceResult.Trace.ContextEvidenceMode, traceResult.Trace.ContextRefCount)
	}
	ok, err := gate.VerifyTraceRecordSignature(traceResult.Trace, keyPair.Public)
	if err != nil {
		t.Fatalf("verify trace signature: %v", err)
	}
	if !ok {
		t.Fatalf("expected trace signature verification success")
	}
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
