package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	sign "github.com/Clyra-AI/proof/signing"
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

func TestEmitSignedTraceIncludesScriptMetadata(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	policy, err := ParsePolicyYAML([]byte(`default_verdict: allow`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.ToolName = "script"
	intent.Script = &schemagate.IntentScript{
		Steps: []schemagate.IntentScriptStep{
			{ToolName: "tool.read", Args: map[string]any{"path": "/tmp/in.txt"}},
			{ToolName: "tool.write", Args: map[string]any{"path": "/tmp/out.txt"}},
		},
	}
	result := schemagate.GateResult{
		SchemaID:        "gait.gate.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Verdict:         "allow",
		ReasonCodes:     []string{"approved_script_match"},
		Violations:      []string{},
	}
	emitted, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		ProducerVersion:    "test",
		SigningPrivateKey:  keyPair.Private,
		TracePath:          filepath.Join(t.TempDir(), "trace_script.json"),
		ContextSource:      "wrkr_inventory",
		CompositeRiskClass: "medium",
		StepVerdicts: []schemagate.TraceStepVerdict{
			{Index: 0, ToolName: "tool.read", Verdict: "allow"},
			{Index: 1, ToolName: "tool.write", Verdict: "allow"},
		},
		PreApproved:    true,
		PatternID:      "pattern_demo",
		RegistryReason: "approved_script_match",
	})
	if err != nil {
		t.Fatalf("emit script trace: %v", err)
	}
	if !emitted.Trace.Script || emitted.Trace.StepCount != 2 {
		t.Fatalf("expected script metadata in trace: %#v", emitted.Trace)
	}
	if emitted.Trace.ContextSource != "wrkr_inventory" || emitted.Trace.PatternID != "pattern_demo" || !emitted.Trace.PreApproved {
		t.Fatalf("unexpected trace metadata: %#v", emitted.Trace)
	}
	if emitted.Trace.ScriptHash == "" {
		t.Fatalf("expected script hash in trace output")
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

func TestEmitSignedTraceIncludesDelegationReference(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	policy, err := ParsePolicyYAML([]byte(`default_verdict: allow`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.Delegation = &schemagate.IntentDelegation{
		RequesterIdentity: "agent.specialist",
		ScopeClass:        "write",
		TokenRefs:         []string{"delegation_demo"},
		Chain: []schemagate.DelegationLink{
			{DelegatorIdentity: "agent.lead", DelegateIdentity: "agent.specialist", ScopeClass: "write"},
		},
	}
	result, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}

	emitted, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		ProducerVersion:       "test",
		DelegationTokenRef:    "delegation_demo",
		DelegationReasonCodes: []string{"delegation_granted"},
		SigningPrivateKey:     keyPair.Private,
		TracePath:             filepath.Join(t.TempDir(), "trace_delegation.json"),
	})
	if err != nil {
		t.Fatalf("emit trace with delegation: %v", err)
	}
	if emitted.Trace.DelegationRef == nil {
		t.Fatalf("expected delegation_ref in trace")
	}
	if emitted.Trace.DelegationRef.DelegationTokenRef != "delegation_demo" {
		t.Fatalf("unexpected delegation token ref: %#v", emitted.Trace.DelegationRef)
	}
	if emitted.Trace.DelegationRef.DelegationDepth != 1 {
		t.Fatalf("expected delegation depth 1, got %#v", emitted.Trace.DelegationRef)
	}
}

func TestEmitSignedTraceRuntimeEventIdentity(t *testing.T) {
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
	firstPath := filepath.Join(t.TempDir(), "trace_event_1.json")
	first, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		ProducerVersion:   "test",
		SigningPrivateKey: keyPair.Private,
		TracePath:         firstPath,
	})
	if err != nil {
		t.Fatalf("emit first trace: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	secondPath := filepath.Join(t.TempDir(), "trace_event_2.json")
	second, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		ProducerVersion:   "test",
		SigningPrivateKey: keyPair.Private,
		TracePath:         secondPath,
	})
	if err != nil {
		t.Fatalf("emit second trace: %v", err)
	}
	if first.Trace.TraceID != second.Trace.TraceID {
		t.Fatalf("expected deterministic trace_id, got %s vs %s", first.Trace.TraceID, second.Trace.TraceID)
	}
	if first.Trace.EventID == "" || second.Trace.EventID == "" {
		t.Fatalf("expected non-empty event_id fields")
	}
	if first.Trace.EventID == second.Trace.EventID {
		t.Fatalf("expected distinct event_id values for separate emissions")
	}
	if first.Trace.ObservedAt.IsZero() || second.Trace.ObservedAt.IsZero() {
		t.Fatalf("expected observed_at on runtime traces")
	}
}

func TestEmitSignedTraceRelationshipEnvelope(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	policy, err := ParsePolicyYAML([]byte(`default_verdict: allow`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Context.Identity = "agent.requester"
	intent.Context.Workspace = "/repo/gait"
	intent.Context.SessionID = "sess_demo"
	intent.Delegation = &schemagate.IntentDelegation{
		RequesterIdentity: "agent.requester",
		Chain: []schemagate.DelegationLink{
			{DelegatorIdentity: "agent.lead", DelegateIdentity: "agent.worker"},
		},
	}
	result, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}

	first, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		ProducerVersion:   "test",
		SigningPrivateKey: keyPair.Private,
		TracePath:         filepath.Join(t.TempDir(), "trace_relationship_1.json"),
		StepVerdicts: []schemagate.TraceStepVerdict{
			{Index: 0, ToolName: "tool.write", Verdict: "allow", MatchedRule: "rule-z"},
			{Index: 1, ToolName: "tool.write", Verdict: "allow", MatchedRule: "rule-a"},
			{Index: 2, ToolName: "tool.write", Verdict: "allow", MatchedRule: "rule-a"},
		},
	})
	if err != nil {
		t.Fatalf("emit first trace: %v", err)
	}
	if first.Trace.Relationship == nil {
		t.Fatalf("expected relationship envelope in trace")
	}
	if first.Trace.Relationship.ParentRef == nil || first.Trace.Relationship.ParentRef.Kind != "session" || first.Trace.Relationship.ParentRef.ID != "sess_demo" {
		t.Fatalf("unexpected parent_ref: %#v", first.Trace.Relationship.ParentRef)
	}
	if first.Trace.Relationship.PolicyRef == nil || len(first.Trace.Relationship.PolicyRef.MatchedRuleIDs) != 2 {
		t.Fatalf("expected matched rule ids in relationship policy_ref: %#v", first.Trace.Relationship.PolicyRef)
	}
	if first.Trace.PolicyID != policy.SchemaID || first.Trace.PolicyVersion != policy.SchemaVersion {
		t.Fatalf("expected policy lineage fields in trace: id=%q version=%q", first.Trace.PolicyID, first.Trace.PolicyVersion)
	}
	if len(first.Trace.MatchedRuleIDs) != 2 {
		t.Fatalf("expected matched_rule_ids in trace: %#v", first.Trace.MatchedRuleIDs)
	}
	if first.Trace.Relationship.PolicyRef.MatchedRuleIDs[0] != "rule-a" || first.Trace.Relationship.PolicyRef.MatchedRuleIDs[1] != "rule-z" {
		t.Fatalf("expected deterministic matched rule ordering, got %#v", first.Trace.Relationship.PolicyRef.MatchedRuleIDs)
	}
	if len(first.Trace.Relationship.EntityRefs) == 0 || len(first.Trace.Relationship.Edges) == 0 {
		t.Fatalf("expected relationship entity refs and edges: %#v", first.Trace.Relationship)
	}

	second, err := EmitSignedTrace(policy, intent, result, EmitTraceOptions{
		ProducerVersion:   "test",
		SigningPrivateKey: keyPair.Private,
		TracePath:         filepath.Join(t.TempDir(), "trace_relationship_2.json"),
	})
	if err != nil {
		t.Fatalf("emit second trace: %v", err)
	}
	if first.Trace.TraceID != second.Trace.TraceID {
		t.Fatalf("trace_id must remain stable with/without relationship detail; first=%s second=%s", first.Trace.TraceID, second.Trace.TraceID)
	}
}

func TestWriteTraceRecordRejectsParentTraversal(t *testing.T) {
	minimal := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_invalid",
		ToolName:        "tool.demo",
		ArgsDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		IntentDigest:    "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:    "3333333333333333333333333333333333333333333333333333333333333333",
		Verdict:         "allow",
	}
	if err := WriteTraceRecord("../trace.json", minimal); err == nil {
		t.Fatalf("expected parent traversal trace path to fail")
	}
}

func TestWriteTraceRecordRelativePath(t *testing.T) {
	workDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	minimal := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_relative",
		ToolName:        "tool.demo",
		ArgsDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		IntentDigest:    "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:    "3333333333333333333333333333333333333333333333333333333333333333",
		Verdict:         "allow",
	}
	relativePath := filepath.Join("nested", "trace_relative.json")
	if err := WriteTraceRecord(relativePath, minimal); err != nil {
		t.Fatalf("write relative trace: %v", err)
	}
	absolutePath := filepath.Join(workDir, relativePath)
	if _, err := os.Stat(absolutePath); err != nil {
		t.Fatalf("stat relative trace: %v", err)
	}
}

func TestWriteTraceRecordCreateDirectoryError(t *testing.T) {
	workDir := t.TempDir()
	blockerPath := filepath.Join(workDir, "nested")
	if err := os.WriteFile(blockerPath, []byte("blocker\n"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	minimal := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_mkdir_error",
		ToolName:        "tool.demo",
		ArgsDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		IntentDigest:    "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:    "3333333333333333333333333333333333333333333333333333333333333333",
		Verdict:         "allow",
	}
	if err := WriteTraceRecord(filepath.Join(blockerPath, "trace.json"), minimal); err == nil {
		t.Fatalf("expected create directory error")
	}
}

func TestWriteTraceRecordCreateDirectoryErrorRelative(t *testing.T) {
	workDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.WriteFile("nested", []byte("blocker\n"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	minimal := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_mkdir_error_relative",
		ToolName:        "tool.demo",
		ArgsDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		IntentDigest:    "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:    "3333333333333333333333333333333333333333333333333333333333333333",
		Verdict:         "allow",
	}
	if err := WriteTraceRecord(filepath.Join("nested", "trace.json"), minimal); err == nil {
		t.Fatalf("expected create directory error for relative path")
	}
}

func TestWriteTraceRecordWriteFileError(t *testing.T) {
	workDir := t.TempDir()
	targetPath := filepath.Join(workDir, "existing-dir")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetPath, "keep.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatalf("write target sentinel: %v", err)
	}

	minimal := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_write_error",
		ToolName:        "tool.demo",
		ArgsDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		IntentDigest:    "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:    "3333333333333333333333333333333333333333333333333333333333333333",
		Verdict:         "allow",
	}
	if err := WriteTraceRecord(targetPath, minimal); err == nil {
		t.Fatalf("expected write error for directory destination")
	}
}

func TestNormalizeTracePath(t *testing.T) {
	absoluteInput := filepath.Join(t.TempDir(), "nested", "trace.json")
	absolutePath, err := normalizeTracePath(absoluteInput)
	if err != nil {
		t.Fatalf("normalize absolute trace path: %v", err)
	}
	if absolutePath != filepath.Clean(absoluteInput) {
		t.Fatalf("unexpected absolute trace path: %s", absolutePath)
	}

	relativePath, err := normalizeTracePath("./gait-out/trace.json")
	if err != nil {
		t.Fatalf("normalize relative trace path: %v", err)
	}
	if relativePath != filepath.Clean("./gait-out/trace.json") {
		t.Fatalf("unexpected relative trace path: %s", relativePath)
	}

	if _, err := normalizeTracePath(""); err == nil {
		t.Fatalf("expected empty trace path to fail")
	}
	if _, err := normalizeTracePath("../gait-out/trace.json"); err == nil {
		t.Fatalf("expected parent traversal trace path to fail")
	}
	if _, err := normalizeTracePath("."); err == nil {
		t.Fatalf("expected dot trace path to fail")
	}
}
