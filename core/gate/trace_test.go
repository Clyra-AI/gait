package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	"github.com/davidahmann/gait/core/sign"
)

func TestEmitSignedTraceAndVerify(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: block-external
    effect: block
    match:
      target_kinds: [host]
      target_values: [api.external.com]
    reason_codes: [blocked_external]
    violations: [external_target]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "host", Value: "api.external.com"},
	}
	intent.SkillProvenance = &schemagate.SkillProvenance{
		SkillName:      "safe-curl",
		SkillVersion:   "1.0.0",
		Source:         "registry",
		Publisher:      "acme",
		Digest:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SignatureKeyID: "key_demo",
	}
	result, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}

	tracePath := filepath.Join(t.TempDir(), "trace.json")
	emitted, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		ProducerVersion:   "test",
		CorrelationID:     "cid-123",
		LatencyMS:         12.5,
		ApprovalTokenRef:  "approval_1",
		SigningPrivateKey: keyPair.Private,
		TracePath:         tracePath,
	})
	if err != nil {
		t.Fatalf("emit trace: %v", err)
	}
	if emitted.Trace.TraceID == "" || emitted.Trace.PolicyDigest == "" || emitted.Trace.IntentDigest == "" {
		t.Fatalf("expected digests and trace id to be set: %#v", emitted.Trace)
	}
	if emitted.Trace.Verdict != "block" {
		t.Fatalf("unexpected trace verdict: %#v", emitted.Trace)
	}
	if emitted.Trace.CorrelationID != "cid-123" {
		t.Fatalf("unexpected trace correlation id: %#v", emitted.Trace)
	}
	if emitted.Trace.LatencyMS != 12.5 {
		t.Fatalf("unexpected trace latency: %#v", emitted.Trace)
	}
	if emitted.Trace.Signature == nil {
		t.Fatalf("expected trace signature to be set")
	}
	if emitted.Trace.SkillProvenance == nil || emitted.Trace.SkillProvenance.SkillName != "safe-curl" {
		t.Fatalf("expected skill provenance copied to trace: %#v", emitted.Trace.SkillProvenance)
	}

	readTrace, err := ReadTraceRecord(tracePath)
	if err != nil {
		t.Fatalf("read trace record: %v", err)
	}
	ok, err := VerifyTraceRecordSignature(readTrace, keyPair.Public)
	if err != nil {
		t.Fatalf("verify trace signature: %v", err)
	}
	if !ok {
		t.Fatalf("expected signature verification to pass")
	}
}

func TestVerifyTraceRecordTamperDetection(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	policy, err := ParsePolicyYAML([]byte(`default_verdict: allow`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	result, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}

	tracePath := filepath.Join(t.TempDir(), "trace.json")
	emitted, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		ProducerVersion:   "test",
		LatencyMS:         1.1,
		SigningPrivateKey: keyPair.Private,
		TracePath:         tracePath,
	})
	if err != nil {
		t.Fatalf("emit trace: %v", err)
	}

	tampered := emitted.Trace
	tampered.Verdict = "block"
	ok, err := VerifyTraceRecordSignature(tampered, keyPair.Public)
	if err == nil && ok {
		t.Fatalf("expected tampered trace to fail verification")
	}

	tampered.Signature = nil
	if _, err := VerifyTraceRecordSignature(tampered, keyPair.Public); err == nil {
		t.Fatalf("expected missing signature to fail verification")
	}
}

func TestTraceHelpersAndErrors(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	policy, err := ParsePolicyYAML([]byte(`default_verdict: allow`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	result, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}

	if _, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{}); err == nil {
		t.Fatalf("expected missing signing key to fail")
	}

	result.Verdict = ""
	if _, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		SigningPrivateKey: keyPair.Private,
	}); err == nil {
		t.Fatalf("expected missing verdict to fail")
	}

	if _, err := ReadTraceRecord(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatalf("expected missing trace path to fail")
	}

	workDir := t.TempDir()
	rawPath := filepath.Join(workDir, "trace.json")
	if err := os.WriteFile(rawPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid trace: %v", err)
	}
	if _, err := ReadTraceRecord(rawPath); err == nil {
		t.Fatalf("expected invalid trace json to fail")
	}

	if id := computeTraceID("a", "b", "allow"); id == "" {
		t.Fatalf("expected trace id to be non-empty")
	}
	if clamped := clampLatency(-1); clamped != 0 {
		t.Fatalf("expected negative latency to clamp to zero, got %f", clamped)
	}
	if clamped := clampLatency(2.5); clamped != 2.5 {
		t.Fatalf("expected positive latency unchanged, got %f", clamped)
	}

	// Coverage for write helper by writing and re-reading a minimal trace payload.
	minimal := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_1",
		ToolName:        "tool.demo",
		ArgsDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		IntentDigest:    "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:    "3333333333333333333333333333333333333333333333333333333333333333",
		Verdict:         "allow",
	}
	minimalPath := filepath.Join(workDir, "nested", "trace.json")
	if err := WriteTraceRecord(minimalPath, minimal); err != nil {
		t.Fatalf("write trace helper: %v", err)
	}
	reRead, err := ReadTraceRecord(minimalPath)
	if err != nil {
		t.Fatalf("read trace helper output: %v", err)
	}
	reReadJSON, err := json.Marshal(reRead)
	if err != nil {
		t.Fatalf("marshal re-read trace: %v", err)
	}
	if len(reReadJSON) == 0 {
		t.Fatalf("expected re-read trace JSON to be non-empty")
	}
}
