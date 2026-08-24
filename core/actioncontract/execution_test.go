package actioncontract

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	proof "github.com/Clyra-AI/proof"
	proofsign "github.com/Clyra-AI/proof/signing"
)

func executionTestRef(kind, id string) proof.RelationshipRef {
	return proof.RelationshipRef{Kind: kind, ID: id, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SourceProduct: "test"}
}

func executionTestBinding() EvidenceBinding {
	binding := EvidenceBinding{ContractFamilyID: "pacf-test", Revision: 1, ContractRef: executionTestRef("action_contract", "pac-1"), ActivationRef: executionTestRef("activated_action_contract", "gact-1"), RuntimeActionRef: executionTestRef("runtime_action", "action-1"), ReadinessRef: executionTestRef("readiness", "ready-1"), DecisionRef: executionTestRef("decision", "decision-1"), PolicyRef: executionTestRef("policy", "policy-1"), TargetRef: executionTestRef("target", "target-1"), EnvironmentRef: executionTestRef("environment", "production"), ProofRefs: []proof.RelationshipRef{executionTestRef("proof", "proof-1")}, CausalRefs: []proof.RelationshipRef{executionTestRef("proposal", "proposal-1")}}
	binding.ContractRef.SchemaID, binding.ContractRef.SchemaVersion, binding.ContractRef.SourceProduct = ProposedContractSchemaID, ProposedContractVersion, "wrkr"
	binding.ActivationRef.SchemaID, binding.ActivationRef.SchemaVersion, binding.ActivationRef.SourceProduct = ActivatedSchemaID, ActivatedSchemaVersion, ActivatedProducer
	binding.RuntimeActionRef.SchemaID, binding.RuntimeActionRef.SchemaVersion, binding.RuntimeActionRef.SourceProduct = RuntimeActionSchemaID, RuntimeActionSchemaVersion, EvidenceProducer
	binding.ReadinessRef.SchemaID, binding.ReadinessRef.SchemaVersion, binding.ReadinessRef.SourceProduct = RuntimeReadinessSchemaID, RuntimeActionSchemaVersion, EvidenceProducer
	binding.DecisionRef.SchemaID, binding.DecisionRef.SchemaVersion, binding.DecisionRef.SourceProduct = RuntimeReadinessSchemaID, RuntimeActionSchemaVersion, EvidenceProducer
	binding.Correlation = proof.ControlContainmentTelemetryProfile{ProfileVersion: CorrelationProfileVersion, BindingMode: proof.BindingModeDigestBound, ContractRef: &binding.ContractRef, ContentDigest: binding.ContractRef.Digest}
	return binding
}

func cloneLifecycleRecords(t *testing.T, records []LifecycleRecord) []LifecycleRecord {
	t.Helper()
	raw, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	var cloned []LifecycleRecord
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func resignLifecycleRecord(t *testing.T, record *LifecycleRecord, private ed25519.PrivateKey) {
	t.Helper()
	digest, err := lifecycleDigest(*record)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := proofsign.SignDigestHex(private, strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		t.Fatal(err)
	}
	record.Signature = signature
	record.RecordID = "gait-lr-" + strings.TrimPrefix(digest, "sha256:")[:16]
}

func TestTypedEvidenceSignsVerifiesAndRejectsTampering(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 8, 24, 1, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	item, err := NewExecutionEvidence(ExecutionEvidence{Binding: executionTestBinding(), EventRef: executionTestRef("event", "event-1"), OccurredAt: now, FreshUntil: now, Outcome: "succeeded"}, private)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyExecutionEvidence(item, public); err != nil || !ok {
		t.Fatalf("verify: %v", err)
	}
	item.ReasonCode = "tampered"
	if ok, _ := VerifyExecutionEvidence(item, public); ok {
		t.Fatal("tampered evidence verified")
	}
	wrongPublic, _, _ := ed25519.GenerateKey(nil)
	if ok, _ := VerifyExecutionEvidence(item, wrongPublic); ok {
		t.Fatal("wrong-key evidence verified")
	}
	item, err = NewExecutionEvidence(ExecutionEvidence{Binding: executionTestBinding(), EventRef: executionTestRef("event", "id"), OccurredAt: now, FreshUntil: now, Outcome: "succeeded"}, private)
	if err != nil {
		t.Fatal(err)
	}
	item.EvidenceID = "gait-exec-0000000000000000"
	if ok, _ := VerifyExecutionEvidence(item, public); ok {
		t.Fatal("evidence ID substitution verified")
	}
	item, err = NewExecutionEvidence(ExecutionEvidence{Binding: executionTestBinding(), EventRef: executionTestRef("event", "binding"), OccurredAt: now, FreshUntil: now, Outcome: "succeeded"}, private)
	if err != nil {
		t.Fatal(err)
	}
	item.Binding.ProofRefs[0].Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if ok, _ := VerifyExecutionEvidence(item, public); ok {
		t.Fatal("mutated Proof binding verified")
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw[:len(raw)-1], []byte(`,"unknown":true}`)...)
	if _, err := ParseExecutionEvidence(raw); err == nil {
		t.Fatal("unknown evidence field accepted")
	}
}

func TestEvidenceWriteIsBoundedNoFollowNoOverwrite(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "evidence.json")
	if err := WriteEvidenceExclusive(path, map[string]string{"ok": "1"}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := ReadEvidenceFile(file); err != nil || string(raw) != `{"ok":"1"}` {
		t.Fatalf("stable evidence read failed: raw=%q err=%v", raw, err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := WriteEvidenceExclusive(path, map[string]string{"ok": "2"}); err == nil {
		t.Fatal("evidence overwrite accepted")
	}
	if err := WriteEvidenceExclusive("", map[string]string{"ok": "2"}); err == nil {
		t.Fatal("empty evidence path accepted")
	}
	if err := WriteEvidenceExclusive(filepath.Join(dir, "large.json"), strings.Repeat("x", int(MaxEvidenceBytes)+1)); err == nil {
		t.Fatal("oversized evidence accepted")
	}
	if err := WriteEvidenceExclusive(filepath.Join(dir, "unsupported.json"), make(chan int)); err == nil {
		t.Fatal("unsupported evidence value accepted")
	}
	if err := WriteEvidenceExclusive(filepath.Join(dir, "missing", "evidence.json"), map[string]string{"ok": "4"}); err == nil {
		t.Fatal("unsafe missing evidence parent accepted")
	}
	if _, err := ReadEvidenceFile(nil); err == nil {
		t.Fatal("nil evidence descriptor accepted")
	}
	directory, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEvidenceFile(directory); err == nil {
		t.Fatal("evidence reader accepted a directory")
	}
	_ = directory.Close()
	oversizedPath := filepath.Join(dir, "oversized-read.json")
	if err := os.WriteFile(oversizedPath, []byte(strings.Repeat("x", int(MaxEvidenceBytes)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	oversized, err := os.Open(oversizedPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEvidenceFile(oversized); err == nil {
		t.Fatal("evidence reader accepted an oversized file")
	}
	_ = oversized.Close()
	linkedParent := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(t.TempDir(), linkedParent); err == nil {
		if err := WriteEvidenceExclusive(filepath.Join(linkedParent, "evidence.json"), map[string]string{"ok": "3"}); err == nil {
			t.Fatal("symlinked evidence parent accepted")
		}
	}
}

func TestEvidenceRootAndReferenceIdentityBranches(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := openVerifiedEvidenceRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := openVerifiedEvidenceRoot(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing evidence root accepted")
	}
	filePath := filepath.Join(dir, "file")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openVerifiedEvidenceRoot(filePath); err == nil {
		t.Fatal("regular file accepted as evidence root")
	}
	base := executionTestBinding().ContractRef
	if !sameLifecycleRefIdentity(&base, &base) || sameLifecycleRefIdentity(nil, &base) {
		t.Fatal("relationship identity nil/equality handling failed")
	}
	for _, edit := range []func(*proof.RelationshipRef){
		func(ref *proof.RelationshipRef) { ref.Kind = "other" },
		func(ref *proof.RelationshipRef) { ref.ID = "other" },
		func(ref *proof.RelationshipRef) {
			ref.Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		},
		func(ref *proof.RelationshipRef) { ref.SchemaID = "other" },
		func(ref *proof.RelationshipRef) { ref.SchemaVersion = "other" },
		func(ref *proof.RelationshipRef) { ref.SourceProduct = "other" },
	} {
		candidate := base
		edit(&candidate)
		if sameLifecycleRefIdentity(&candidate, &base) {
			t.Fatal("mismatched relationship identity accepted")
		}
	}
}

func TestEvidenceBindingRejectsIdentifierOnlyAndWrongFamily(t *testing.T) {
	binding := executionTestBinding()
	binding.PolicyRef.Digest = ""
	if err := binding.Validate(); err == nil {
		t.Fatal("identifier-only policy ref accepted")
	}
	binding = executionTestBinding()
	binding.ProofRefs = nil
	if err := binding.Validate(); err == nil {
		t.Fatal("missing proof refs accepted")
	}
	other := executionTestBinding()
	other.ContractFamilyID = "pacf-other"
	if binding.sameIdentity(other) {
		t.Fatal("cross-family binding considered identical")
	}
	if len(executionTestBinding().RelationshipRefs()) != 9 {
		t.Fatal("evidence binding relationship refs are incomplete")
	}
	for _, test := range []struct {
		name string
		edit func(*EvidenceBinding)
	}{
		{"family", func(b *EvidenceBinding) { b.ContractFamilyID = "" }},
		{"revision", func(b *EvidenceBinding) { b.Revision = 0 }},
		{"causal", func(b *EvidenceBinding) { b.CausalRefs = nil }},
		{"contract producer", func(b *EvidenceBinding) { b.ContractRef.SourceProduct = "gait" }},
		{"contract kind", func(b *EvidenceBinding) { b.ContractRef.Kind = "contract" }},
		{"activation schema", func(b *EvidenceBinding) { b.ActivationRef.SchemaID = "wrong" }},
		{"runtime kind", func(b *EvidenceBinding) { b.RuntimeActionRef.Kind = "decision" }},
		{"runtime schema", func(b *EvidenceBinding) { b.RuntimeActionRef.SchemaID = "wrong" }},
		{"readiness kind", func(b *EvidenceBinding) { b.ReadinessRef.Kind = "decision" }},
		{"readiness schema", func(b *EvidenceBinding) { b.ReadinessRef.SchemaID = "wrong" }},
		{"decision kind", func(b *EvidenceBinding) { b.DecisionRef.Kind = "readiness" }},
		{"decision producer", func(b *EvidenceBinding) { b.DecisionRef.SourceProduct = "other" }},
		{"correlation mode", func(b *EvidenceBinding) { b.Correlation.BindingMode = proof.BindingModeIdentifierOnly }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := executionTestBinding()
			test.edit(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("invalid evidence binding accepted")
			}
		})
	}
}

func TestTypedEvidenceParsersAndValidationBranches(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(nil)
	when := func(n int) string { return time.Date(2026, 8, 24, 2, 0, n, 0, time.UTC).Format(time.RFC3339Nano) }
	execution, err := NewExecutionEvidence(ExecutionEvidence{Binding: executionTestBinding(), EventRef: executionTestRef("event", "parser-execution"), OccurredAt: when(1), FreshUntil: when(2), Outcome: "succeeded"}, private)
	if err != nil {
		t.Fatal(err)
	}
	effectBinding := executionTestBinding()
	effectBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForExecution(execution)}
	effect, err := NewEffectEvent(EffectEvent{Binding: effectBinding, EventRef: executionTestRef("event", "parser-effect"), ExecutionRef: evidenceRefForExecution(execution), EffectRef: executionTestRef("effect", "parser-effect"), OccurredAt: when(3), FreshUntil: when(4), Outcome: "recorded"}, private)
	if err != nil {
		t.Fatal(err)
	}
	containmentBinding := executionTestBinding()
	containmentBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForEffect(effect)}
	containment, err := NewContainmentEvidence(ContainmentEvidence{Binding: containmentBinding, EventRef: executionTestRef("event", "parser-containment"), ExecutionRef: evidenceRefForExecution(execution), EffectRef: evidenceRefForEffect(effect), ContainmentRef: executionTestRef("containment", "parser-scope"), OccurredAt: when(5), FreshUntil: when(6), Outcome: "requested"}, private)
	if err != nil {
		t.Fatal(err)
	}
	compensationBinding := executionTestBinding()
	compensationBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForExecution(execution)}
	compensation, err := NewCompensationEvidence(CompensationEvidence{Binding: compensationBinding, EventRef: executionTestRef("event", "parser-compensation"), RequirementRef: executionTestRef("compensation_requirement", "parser-requirement"), ExecutionRef: evidenceRefForExecution(execution), OccurredAt: when(7), FreshUntil: when(8), Outcome: "required"}, private)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name   string
		value  any
		parse  func([]byte) error
		verify func() (bool, error)
	}{
		{"execution", execution, func(raw []byte) error { _, err := ParseExecutionEvidence(raw); return err }, func() (bool, error) { return VerifyExecutionEvidence(execution, public) }},
		{"effect", effect, func(raw []byte) error { _, err := ParseEffectEvent(raw); return err }, func() (bool, error) { return VerifyEffectEvent(effect, public) }},
		{"containment", containment, func(raw []byte) error { _, err := ParseContainmentEvidence(raw); return err }, func() (bool, error) { return VerifyContainmentEvidence(containment, public) }},
		{"compensation", compensation, func(raw []byte) error { _, err := ParseCompensationEvidence(raw); return err }, func() (bool, error) { return VerifyCompensationEvidence(compensation, public) }},
	} {
		t.Run(item.name, func(t *testing.T) {
			raw, marshalErr := json.Marshal(item.value)
			parseErr := item.parse(raw)
			if marshalErr != nil || parseErr != nil {
				t.Fatalf("parse failed: marshal=%v parse=%v", marshalErr, parseErr)
			}
			if ok, verifyErr := item.verify(); verifyErr != nil || !ok {
				t.Fatalf("verify failed: ok=%t err=%v", ok, verifyErr)
			}
			if parseErr := item.parse([]byte(`{"schema_id":`)); parseErr == nil {
				t.Fatal("malformed evidence accepted")
			}
			if parseErr := item.parse([]byte(`{}`)); parseErr == nil {
				t.Fatal("incomplete evidence accepted")
			}
		})
	}
	invalidExecution := execution
	invalidExecution.CanonicalContentDigest = "bad"
	if err := validateExecutionEvidence(invalidExecution); err == nil {
		t.Fatal("execution evidence accepted an invalid canonical digest")
	}
	invalidExecution = execution
	invalidExecution.Provenance.Signature.Alg = ""
	if err := validateExecutionEvidence(invalidExecution); err == nil {
		t.Fatal("execution evidence accepted missing provenance")
	}
	invalidExecution = execution
	invalidExecution.Binding.ProofRefs = nil
	if err := validateExecutionEvidence(invalidExecution); err == nil {
		t.Fatal("execution evidence accepted an incomplete binding")
	}
	invalidEffect := effect
	invalidEffect.ReasonCode = ""
	if err := validateEffectEvent(invalidEffect); err == nil {
		t.Fatal("effect event accepted a missing reason code")
	}
	invalidEffect = effect
	invalidEffect.Outcome = "unknown"
	if err := validateEffectEvent(invalidEffect); err == nil {
		t.Fatal("effect event accepted an unknown outcome")
	}
	invalidEffect = effect
	invalidEffect.ExecutionRef = proof.RelationshipRef{}
	if err := validateEffectEvent(invalidEffect); err == nil {
		t.Fatal("effect event accepted a missing execution reference")
	}
	invalidEffect = effect
	invalidEffect.ExecutionRef.Kind = "runtime_action"
	if err := validateEffectEvent(invalidEffect); err == nil {
		t.Fatal("effect event accepted a non-execution typed reference")
	}
	invalidEffect = effect
	invalidEffect.Provenance.PublicKey = ""
	if err := validateEffectEvent(invalidEffect); err == nil {
		t.Fatal("effect event accepted incomplete provenance")
	}
	invalidContainment := containment
	invalidContainment.ReasonCode = ""
	if err := validateContainmentEvidence(invalidContainment); err == nil {
		t.Fatal("containment evidence accepted a missing reason code")
	}
	invalidContainment = containment
	invalidContainment.Outcome = "unknown"
	if err := validateContainmentEvidence(invalidContainment); err == nil {
		t.Fatal("containment evidence accepted an unknown outcome")
	}
	invalidContainment = containment
	invalidContainment.ContainmentRef = proof.RelationshipRef{}
	if err := validateContainmentEvidence(invalidContainment); err == nil {
		t.Fatal("containment evidence accepted a missing scope reference")
	}
	invalidContainment = containment
	invalidContainment.ExecutionRef.SchemaID = RuntimeActionSchemaID
	if err := validateContainmentEvidence(invalidContainment); err == nil {
		t.Fatal("containment evidence accepted a non-execution typed reference")
	}
	invalidCompensation := compensation
	invalidCompensation.ReasonCode = ""
	if err := validateCompensationEvidence(invalidCompensation); err == nil {
		t.Fatal("compensation evidence accepted a missing reason code")
	}
	invalidCompensation = compensation
	invalidCompensation.Outcome = "unknown"
	if err := validateCompensationEvidence(invalidCompensation); err == nil {
		t.Fatal("compensation evidence accepted an unknown outcome")
	}
	invalidCompensation = compensation
	invalidCompensation.RequirementRef = proof.RelationshipRef{}
	if err := validateCompensationEvidence(invalidCompensation); err == nil {
		t.Fatal("compensation evidence accepted a missing requirement reference")
	}
	invalidCompensation = compensation
	invalidCompensation.ExecutionRef.SourceProduct = "other"
	if err := validateCompensationEvidence(invalidCompensation); err == nil {
		t.Fatal("compensation evidence accepted a non-execution typed reference")
	}
	brokenExecution := execution
	brokenExecution.ReasonCode = ""
	if err := validateLifecycleEvidence(LifecycleRecord{Execution: &brokenExecution}); err == nil {
		t.Fatal("lifecycle accepted invalid execution evidence")
	}
	brokenEffect := effect
	brokenEffect.ReasonCode = ""
	if err := validateLifecycleEvidence(LifecycleRecord{Effect: &brokenEffect}); err == nil {
		t.Fatal("lifecycle accepted invalid effect evidence")
	}
	brokenContainment := containment
	brokenContainment.ReasonCode = ""
	if err := validateLifecycleEvidence(LifecycleRecord{Containment: &brokenContainment}); err == nil {
		t.Fatal("lifecycle accepted invalid containment evidence")
	}
	brokenCompensation := compensation
	brokenCompensation.ReasonCode = ""
	if err := validateLifecycleEvidence(LifecycleRecord{Compensation: &brokenCompensation}); err == nil {
		t.Fatal("lifecycle accepted invalid compensation evidence")
	}
	if _, err := canonicalEvidence(make(chan int)); err == nil {
		t.Fatal("canonical evidence accepted an unsupported value")
	}
	if _, err := canonicalEvidence([]string{"not-an-object"}); err == nil {
		t.Fatal("canonical evidence accepted a non-object")
	}
	if _, _, err := signEvidence(execution, nil); err == nil {
		t.Fatal("sign evidence accepted an invalid key")
	}
	if err := validateTypedEvidenceSchema(make(chan int), ExecutionEvidenceSchemaID); err == nil {
		t.Fatal("typed schema validation accepted an unsupported value")
	}
	for _, item := range []struct {
		kind        LifecycleEventKind
		execution   *ExecutionEvidence
		containment *ContainmentEvidence
	}{
		{kind: LifecycleExecutionFailed, execution: func() *ExecutionEvidence { value := execution; value.Outcome = "failed"; return &value }()},
		{kind: LifecycleExecutionBlocked, execution: func() *ExecutionEvidence { value := execution; value.Outcome = "blocked"; return &value }()},
		{kind: LifecycleContainmentCompleted, containment: func() *ContainmentEvidence { value := containment; value.Outcome = "completed"; return &value }()},
		{kind: LifecycleContainmentPartial, containment: func() *ContainmentEvidence { value := containment; value.Outcome = "partial"; return &value }()},
		{kind: LifecycleContainmentUnresolved, containment: func() *ContainmentEvidence { value := containment; value.Outcome = "unresolved"; return &value }()},
	} {
		if err := validateLifecycleEvent(LifecycleRecord{Kind: item.kind, Execution: item.execution, Containment: item.containment}); err != nil {
			t.Fatalf("valid lifecycle event %s rejected: %v", item.kind, err)
		}
	}
	mismatched := execution
	mismatched.Outcome = "blocked"
	if err := validateLifecycleEvent(LifecycleRecord{Kind: LifecycleExecutionFailed, Execution: &mismatched}); err == nil {
		t.Fatal("mismatched lifecycle execution outcome accepted")
	}
	if _, err := NewExecutionEvidence(ExecutionEvidence{}, nil); err == nil {
		t.Fatal("execution evidence accepted an invalid signing key")
	}
	if _, err := NewEffectEvent(EffectEvent{}, nil); err == nil {
		t.Fatal("effect evidence accepted an invalid signing key")
	}
	if _, err := NewContainmentEvidence(ContainmentEvidence{}, nil); err == nil {
		t.Fatal("containment evidence accepted an invalid signing key")
	}
	if _, err := NewCompensationEvidence(CompensationEvidence{}, nil); err == nil {
		t.Fatal("compensation evidence accepted an invalid signing key")
	}
	if _, err := NewExecutionEvidence(ExecutionEvidence{Binding: executionTestBinding(), EventRef: executionTestRef("event", "bad-time"), OccurredAt: "bad", FreshUntil: when(1), Outcome: "started"}, private); err == nil {
		t.Fatal("execution evidence accepted an invalid timestamp")
	}
	if _, err := NewExecutionEvidence(ExecutionEvidence{Binding: executionTestBinding(), EventRef: executionTestRef("event", "reversed-time"), OccurredAt: when(2), FreshUntil: when(1), Outcome: "started"}, private); err == nil {
		t.Fatal("execution evidence accepted a reversed freshness window")
	}
	if _, err := NewEffectEvent(EffectEvent{Binding: effectBinding, EventRef: executionTestRef("event", "bad-effect-time"), ExecutionRef: evidenceRefForExecution(execution), EffectRef: effect.EffectRef, OccurredAt: "bad", FreshUntil: when(1), Outcome: "recorded"}, private); err == nil {
		t.Fatal("effect evidence accepted an invalid timestamp")
	}
	if _, err := NewContainmentEvidence(ContainmentEvidence{Binding: containmentBinding, EventRef: executionTestRef("event", "bad-containment-time"), ExecutionRef: evidenceRefForExecution(execution), EffectRef: evidenceRefForEffect(effect), ContainmentRef: containment.ContainmentRef, OccurredAt: "bad", FreshUntil: when(1), Outcome: "requested"}, private); err == nil {
		t.Fatal("containment evidence accepted an invalid timestamp")
	}
	if _, err := NewCompensationEvidence(CompensationEvidence{Binding: compensationBinding, EventRef: executionTestRef("event", "bad-compensation-time"), RequirementRef: compensation.RequirementRef, ExecutionRef: evidenceRefForExecution(execution), OccurredAt: "bad", FreshUntil: when(1), Outcome: "required"}, private); err == nil {
		t.Fatal("compensation evidence accepted an invalid timestamp")
	}
}

func TestLifecycleTypedEvidenceOrderAndLineage(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(nil)
	contract := executionTestBinding().ContractRef
	proposal := contract
	activation := executionTestRef("activated_action_contract", "gact-1")
	activation.SchemaID, activation.SchemaVersion, activation.SourceProduct = ActivatedSchemaID, ActivatedSchemaVersion, ActivatedProducer
	profile := proof.ControlContainmentTelemetryProfile{ProfileVersion: CorrelationProfileVersion, BindingMode: proof.BindingModeDigestBound, ContractRef: &contract, ContentDigest: contract.Digest}
	when := func(n int) string { return time.Date(2026, 8, 24, 1, 0, n, 0, time.UTC).Format(time.RFC3339Nano) }
	base := func(id string, kind LifecycleEventKind, at string) LifecycleRecord {
		return LifecycleRecord{SchemaID: RuntimeLifecycleSchemaID, SchemaVersion: RuntimeLifecycleVersion, RecordID: id, Kind: kind, OccurredAt: at, ContractRef: contract, ContractFamilyID: "pacf-test", Revision: 1, ProposalRef: &proposal, ActivationRef: &activation, Correlation: profile}
	}
	pre := executionTestRef("precondition", "ready-1")
	decision := base("r5", LifecycleDecisionReady, when(5))
	decision.PreconditionRefs = []proof.RelationshipRef{pre}
	decision.Decision = &ReadinessResult{SchemaID: RuntimeReadinessSchemaID, SchemaVersion: RuntimeActionSchemaVersion, ContractID: contract.ID, PolicyDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", Ready: true, Status: ReadinessSatisfied, Preconditions: []ReadinessPrecondition{{RequirementID: "ready-1", Kind: "runtime", Required: true, ControlMode: ControlModeEnforced, Status: ReadinessSatisfied, EvidenceDigest: pre.Digest}}}
	started, err := NewExecutionEvidence(ExecutionEvidence{Binding: executionTestBinding(), EventRef: executionTestRef("event", "start"), OccurredAt: when(6), FreshUntil: when(6), Outcome: "started"}, private)
	if err != nil {
		t.Fatal(err)
	}
	succeededBinding := executionTestBinding()
	succeededBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForExecution(started)}
	succeeded, err := NewExecutionEvidence(ExecutionEvidence{Binding: succeededBinding, EventRef: executionTestRef("event", "success"), OccurredAt: when(7), FreshUntil: when(7), Outcome: "succeeded"}, private)
	if err != nil {
		t.Fatal(err)
	}
	effectBinding := executionTestBinding()
	effectBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForExecution(succeeded)}
	effect, err := NewEffectEvent(EffectEvent{Binding: effectBinding, EventRef: executionTestRef("event", "effect"), ExecutionRef: evidenceRefForExecution(succeeded), EffectRef: executionTestRef("effect", "effect-1"), OccurredAt: when(8), FreshUntil: when(8), Outcome: "recorded"}, private)
	if err != nil {
		t.Fatal(err)
	}
	validatedBinding := executionTestBinding()
	validatedBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForEffect(effect)}
	validated, err := NewEffectEvent(EffectEvent{Binding: validatedBinding, EventRef: executionTestRef("event", "effect-validated"), ExecutionRef: evidenceRefForExecution(succeeded), EffectRef: effect.EffectRef, OccurredAt: when(9), FreshUntil: when(9), Outcome: "validated"}, private)
	if err != nil {
		t.Fatal(err)
	}
	containmentBinding := executionTestBinding()
	containmentBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForEffect(validated)}
	containment, err := NewContainmentEvidence(ContainmentEvidence{Binding: containmentBinding, EventRef: executionTestRef("event", "contain"), ExecutionRef: evidenceRefForExecution(succeeded), EffectRef: evidenceRefForEffect(validated), ContainmentRef: executionTestRef("containment", "scope-1"), OccurredAt: when(10), FreshUntil: when(10), Outcome: "requested"}, private)
	if err != nil {
		t.Fatal(err)
	}
	records := []LifecycleRecord{base("r1", LifecycleProposalIngested, when(1)), base("r2", LifecyclePreconditionEvaluated, when(2)), base("r3", LifecycleActivationRequested, when(3)), decision, base("r6", LifecycleActivated, when(6)), base("r7", LifecycleExecutionStarted, when(10)), base("r8", LifecycleExecutionSucceeded, when(11)), base("r9", LifecycleEffectRecorded, when(12)), base("r10", LifecycleEffectValidated, when(13)), base("r11", LifecycleContainmentRequested, when(14))}
	records[1].PreconditionRefs = []proof.RelationshipRef{pre}
	records[5].Execution = &started
	records[5].EvidenceRefs = []proof.RelationshipRef{evidenceRefForExecution(started)}
	records[6].Execution = &succeeded
	records[6].EvidenceRefs = []proof.RelationshipRef{evidenceRefForExecution(succeeded)}
	records[7].Effect = &effect
	records[7].EvidenceRefs = []proof.RelationshipRef{evidenceRefForEffect(effect)}
	records[8].Effect = &validated
	records[8].EvidenceRefs = []proof.RelationshipRef{evidenceRefForEffect(validated)}
	records[9].Containment = &containment
	records[9].EvidenceRefs = []proof.RelationshipRef{evidenceRefForContainment(containment)}
	// Keep the externally supplied record times strictly ordered.
	for i := range records {
		records[i].OccurredAt = when(i + 1)
		records[i].Correlation = profile
	}
	if snapshot, err := ReduceLifecycleChecked(records); err != nil || snapshot.ExecutionStatus != "succeeded" || snapshot.EffectStatus != "validated" || snapshot.ContainmentStatus != "requested" || snapshot.CurrentStatus != "containment_requested" {
		t.Fatalf("typed lifecycle reduction failed: snapshot=%#v err=%v", snapshot, err)
	}
	completedContainmentBinding := executionTestBinding()
	completedContainmentBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForContainment(containment)}
	completedContainment, err := NewContainmentEvidence(ContainmentEvidence{Binding: completedContainmentBinding, EventRef: executionTestRef("event", "contain-completed"), ExecutionRef: evidenceRefForExecution(succeeded), EffectRef: evidenceRefForEffect(validated), ContainmentRef: containment.ContainmentRef, OccurredAt: when(11), FreshUntil: when(11), Outcome: "completed"}, private)
	if err != nil {
		t.Fatal(err)
	}
	completedRecord := base("r12", LifecycleContainmentCompleted, when(11))
	completedRecord.Containment = &completedContainment
	completedRecord.EvidenceRefs = []proof.RelationshipRef{evidenceRefForContainment(completedContainment)}
	completedRecords := append(cloneLifecycleRecords(t, records), completedRecord)
	if snapshot, err := ReduceLifecycleChecked(completedRecords); err != nil || snapshot.CurrentStatus != "containment_completed" {
		t.Fatalf("completed containment lifecycle failed: snapshot=%#v err=%v", snapshot, err)
	}

	withoutValidation := append(cloneLifecycleRecords(t, records[:8]), cloneLifecycleRecords(t, records[9:])...)
	if _, err := ReduceLifecycleChecked(withoutValidation); err == nil {
		t.Fatal("containment before effect validation accepted")
	}
	proofDrift := cloneLifecycleRecords(t, records)
	proofDrift[6].Execution.Binding.ProofRefs[0].Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := ReduceLifecycleChecked(proofDrift); err == nil {
		t.Fatal("cross-Proof execution lineage accepted")
	}
	causalIdentityDrift := cloneLifecycleRecords(t, records)
	causalIdentityDrift[6].Execution.Binding.CausalRefs[0].ID = "unrelated-execution"
	if _, err := ReduceLifecycleChecked(causalIdentityDrift); err == nil {
		t.Fatal("causal predecessor with only a matching digest accepted")
	}
	explicitIdentityDrift := cloneLifecycleRecords(t, records)
	explicitIdentityDrift[7].Effect.ExecutionRef.ID = "unrelated-execution"
	if _, err := ReduceLifecycleChecked(explicitIdentityDrift); err == nil {
		t.Fatal("effect execution ref with only a matching digest accepted")
	}
	futureEvidence := cloneLifecycleRecords(t, records)
	futureEvidence[5].OccurredAt = when(5)
	if _, err := ReduceLifecycleChecked(futureEvidence); err == nil {
		t.Fatal("future evidence accepted for an earlier lifecycle event")
	}
	expiredEvidence := cloneLifecycleRecords(t, records)
	expiredEvidence[5].OccurredAt = when(7)
	if _, err := ReduceLifecycleChecked(expiredEvidence); err == nil {
		t.Fatal("expired evidence accepted for a later lifecycle event")
	}
	wrongContainmentExecution := cloneLifecycleRecords(t, records)
	wrongContainmentExecution[9].Containment.ExecutionRef.Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := ReduceLifecycleChecked(wrongContainmentExecution); err == nil {
		t.Fatal("containment bound to unrelated execution accepted")
	}

	compSucceededBinding := executionTestBinding()
	compSucceededBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForExecution(started)}
	compSucceeded, err := NewExecutionEvidence(ExecutionEvidence{Binding: compSucceededBinding, EventRef: executionTestRef("event", "success-compensation"), OccurredAt: when(7), FreshUntil: when(7), Outcome: "succeeded", CompensationRequired: true}, private)
	if err != nil {
		t.Fatal(err)
	}
	compEffectBinding := executionTestBinding()
	compEffectBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForExecution(compSucceeded)}
	compEffect, err := NewEffectEvent(EffectEvent{Binding: compEffectBinding, EventRef: executionTestRef("event", "effect-compensation"), ExecutionRef: evidenceRefForExecution(compSucceeded), EffectRef: effect.EffectRef, OccurredAt: when(8), FreshUntil: when(8), Outcome: "recorded"}, private)
	if err != nil {
		t.Fatal(err)
	}
	compValidatedBinding := executionTestBinding()
	compValidatedBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForEffect(compEffect)}
	compValidated, err := NewEffectEvent(EffectEvent{Binding: compValidatedBinding, EventRef: executionTestRef("event", "effect-compensation-validated"), ExecutionRef: evidenceRefForExecution(compSucceeded), EffectRef: effect.EffectRef, OccurredAt: when(9), FreshUntil: when(9), Outcome: "validated"}, private)
	if err != nil {
		t.Fatal(err)
	}
	compContainmentBinding := executionTestBinding()
	compContainmentBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForEffect(compValidated)}
	compContainment, err := NewContainmentEvidence(ContainmentEvidence{Binding: compContainmentBinding, EventRef: executionTestRef("event", "contain-compensation"), ExecutionRef: evidenceRefForExecution(compSucceeded), EffectRef: evidenceRefForEffect(compValidated), ContainmentRef: containment.ContainmentRef, OccurredAt: when(10), FreshUntil: when(10), Outcome: "requested"}, private)
	if err != nil {
		t.Fatal(err)
	}
	requirement := executionTestRef("compensation_requirement", "compensation-1")
	requiredBinding := executionTestBinding()
	requiredBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForExecution(compSucceeded)}
	requiredEvidence, err := NewCompensationEvidence(CompensationEvidence{Binding: requiredBinding, EventRef: executionTestRef("event", "compensation-required"), RequirementRef: requirement, ExecutionRef: evidenceRefForExecution(compSucceeded), OccurredAt: when(11), FreshUntil: when(11), Outcome: "required"}, private)
	if err != nil {
		t.Fatal(err)
	}
	startedBinding := executionTestBinding()
	startedBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForCompensation(requiredEvidence)}
	startedEvidence, err := NewCompensationEvidence(CompensationEvidence{Binding: startedBinding, EventRef: executionTestRef("event", "compensation-started"), RequirementRef: requirement, ExecutionRef: evidenceRefForExecution(compSucceeded), OccurredAt: when(12), FreshUntil: when(12), Outcome: "started"}, private)
	if err != nil {
		t.Fatal(err)
	}
	completedBinding := executionTestBinding()
	completedBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForCompensation(startedEvidence)}
	completedEvidence, err := NewCompensationEvidence(CompensationEvidence{Binding: completedBinding, EventRef: executionTestRef("event", "compensation-completed"), RequirementRef: requirement, ExecutionRef: evidenceRefForExecution(compSucceeded), OccurredAt: when(13), FreshUntil: when(13), Outcome: "completed"}, private)
	if err != nil {
		t.Fatal(err)
	}
	compensationRecords := cloneLifecycleRecords(t, records)
	compensationRecords[6].Execution, compensationRecords[6].EvidenceRefs = &compSucceeded, []proof.RelationshipRef{evidenceRefForExecution(compSucceeded)}
	compensationRecords[7].Effect, compensationRecords[7].EvidenceRefs = &compEffect, []proof.RelationshipRef{evidenceRefForEffect(compEffect)}
	compensationRecords[8].Effect, compensationRecords[8].EvidenceRefs = &compValidated, []proof.RelationshipRef{evidenceRefForEffect(compValidated)}
	compensationRecords[9].Containment, compensationRecords[9].EvidenceRefs = &compContainment, []proof.RelationshipRef{evidenceRefForContainment(compContainment)}
	for _, item := range []struct {
		id       string
		kind     LifecycleEventKind
		evidence CompensationEvidence
		at       int
	}{{"r12", LifecycleCompensationRequired, requiredEvidence, 11}, {"r13", LifecycleCompensationStarted, startedEvidence, 12}, {"r14", LifecycleCompensationCompleted, completedEvidence, 13}} {
		record := base(item.id, item.kind, when(item.at))
		record.Compensation = &item.evidence
		record.EvidenceRefs = []proof.RelationshipRef{evidenceRefForCompensation(item.evidence)}
		compensationRecords = append(compensationRecords, record)
	}
	if snapshot, err := ReduceLifecycleChecked(compensationRecords); err != nil || snapshot.CurrentStatus != "compensation_completed" {
		t.Fatalf("complete compensation lifecycle failed: snapshot=%#v err=%v", snapshot, err)
	}
	signedCompensation := make([]LifecycleRecord, 0, len(compensationRecords))
	for _, record := range compensationRecords {
		created, createErr := NewLifecycleRecord(LifecycleRecordOptions{
			Kind: record.Kind, OccurredAt: mustParseTime(record.OccurredAt), ContractRef: record.ContractRef,
			ContractFamilyID: record.ContractFamilyID, Revision: record.Revision, ProposalRef: record.ProposalRef,
			ActivationRef: record.ActivationRef, PreconditionRefs: record.PreconditionRefs, Decision: record.Decision,
			Execution: record.Execution, Effect: record.Effect, Containment: record.Containment, Compensation: record.Compensation,
			Correlation: record.Correlation, SigningPrivateKey: private,
		})
		if createErr != nil {
			t.Fatalf("sign compensation lifecycle %s: %v", record.Kind, createErr)
		}
		signedCompensation = append(signedCompensation, created)
	}
	if snapshot, err := ReduceVerifiedLifecycle(signedCompensation, public); err != nil || snapshot.CurrentStatus != "compensation_completed" {
		t.Fatalf("verified compensation lifecycle failed: snapshot=%#v err=%v", snapshot, err)
	}
	if _, err := ReduceLifecycleChecked(compensationRecords[:len(compensationRecords)-1]); err == nil {
		t.Fatal("required compensation without completion accepted")
	}
	wrongRequirement := cloneLifecycleRecords(t, compensationRecords)
	wrongRequirement[len(wrongRequirement)-1].Compensation.RequirementRef.Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := ReduceLifecycleChecked(wrongRequirement); err == nil {
		t.Fatal("compensation completion for unrelated requirement accepted")
	}

	signed := make([]LifecycleRecord, 0, len(records))
	for _, record := range records {
		created, createErr := NewLifecycleRecord(LifecycleRecordOptions{
			Kind: record.Kind, OccurredAt: mustParseTime(record.OccurredAt), ContractRef: record.ContractRef,
			ContractFamilyID: record.ContractFamilyID, Revision: record.Revision, ProposalRef: record.ProposalRef,
			ActivationRef: record.ActivationRef, PreconditionRefs: record.PreconditionRefs, Decision: record.Decision,
			Execution: record.Execution, Effect: record.Effect, Containment: record.Containment, Compensation: record.Compensation,
			Correlation: record.Correlation, SigningPrivateKey: private,
		})
		if createErr != nil {
			t.Fatalf("sign lifecycle %s: %v", record.Kind, createErr)
		}
		signed = append(signed, created)
	}
	if snapshot, err := ReduceVerifiedLifecycle(signed, public); err != nil || snapshot.CurrentStatus != "containment_requested" {
		t.Fatalf("verified typed lifecycle failed: snapshot=%#v err=%v", snapshot, err)
	}
	legacyAuxiliaryRef := signed[0]
	legacyAuxiliaryRef.Correlation.ActionRef = &proof.RelationshipRef{Kind: "runtime_action", ID: "legacy-identifier-only"}
	resignLifecycleRecord(t, &legacyAuxiliaryRef, private)
	legacyRaw, err := json.Marshal(legacyAuxiliaryRef)
	if err != nil {
		t.Fatal(err)
	}
	parsedLegacy, err := ParseLifecycleRecord(legacyRaw)
	if err != nil {
		t.Fatalf("legacy identifier-only auxiliary correlation ref rejected: %v", err)
	}
	if ok, err := VerifyLifecycleRecord(parsedLegacy, public); err != nil || !ok {
		t.Fatalf("legacy identifier-only auxiliary correlation ref failed verification: ok=%t err=%v", ok, err)
	}
	invalidInnerSignature := cloneLifecycleRecords(t, signed)[5]
	invalidInnerSignature.Execution.Provenance.Signature.Sig = "invalid"
	resignLifecycleRecord(t, &invalidInnerSignature, private)
	if ok, err := VerifyLifecycleRecord(invalidInnerSignature, public); err == nil || ok {
		t.Fatal("direct lifecycle verification accepted invalid embedded evidence")
	}
	tamperedForConstruction := *records[5].Execution
	tamperedForConstruction.Provenance.Signature.Sig = "invalid"
	if _, err := NewLifecycleRecord(LifecycleRecordOptions{
		Kind: records[5].Kind, OccurredAt: mustParseTime(records[5].OccurredAt), ContractRef: records[5].ContractRef,
		ContractFamilyID: records[5].ContractFamilyID, Revision: records[5].Revision, ProposalRef: records[5].ProposalRef,
		ActivationRef: records[5].ActivationRef, Execution: &tamperedForConstruction, Correlation: records[5].Correlation,
		SigningPrivateKey: private,
	}); err == nil {
		t.Fatal("lifecycle constructor accepted invalid embedded evidence")
	}
	outsideEvidenceWindow := cloneLifecycleRecords(t, signed)[5]
	outsideEvidenceWindow.OccurredAt = when(7)
	resignLifecycleRecord(t, &outsideEvidenceWindow, private)
	if ok, err := VerifyLifecycleRecord(outsideEvidenceWindow, public); err == nil || ok {
		t.Fatal("direct lifecycle verification accepted expired embedded evidence")
	}
	mismatchedOutcome := signed[6]
	mismatchedOutcome.Kind = LifecycleExecutionFailed
	mismatchedRaw, err := json.Marshal(mismatchedOutcome)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseLifecycleRecord(mismatchedRaw); err == nil {
		t.Fatal("lifecycle schema accepted an event/evidence outcome mismatch")
	}
	extraTypedEvidence := signed[5]
	extraTypedEvidence.Effect = &effect
	extraRaw, err := json.Marshal(extraTypedEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseLifecycleRecord(extraRaw); err == nil {
		t.Fatal("lifecycle schema accepted multiple typed evidence members")
	}
	legacyWithTypedEvidence := signed[0]
	legacyWithTypedEvidence.Execution = &started
	legacyTypedRaw, err := json.Marshal(legacyWithTypedEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseLifecycleRecord(legacyTypedRaw); err == nil {
		t.Fatal("lifecycle schema accepted typed evidence on a legacy event kind")
	}
	wrongPublic, _, _ := ed25519.GenerateKey(nil)
	if _, err := ReduceVerifiedLifecycle(signed, wrongPublic); err == nil {
		t.Fatal("verified lifecycle accepted an unrelated trust key")
	}
}
