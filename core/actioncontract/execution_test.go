package actioncontract

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	proof "github.com/Clyra-AI/proof"
)

func executionTestRef(kind, id string) proof.RelationshipRef {
	return proof.RelationshipRef{Kind: kind, ID: id, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SourceProduct: "test"}
}

func executionTestBinding() EvidenceBinding {
	binding := EvidenceBinding{ContractFamilyID: "pacf-test", Revision: 1, ContractRef: executionTestRef("contract", "pac-1"), ActivationRef: executionTestRef("activated_action_contract", "gact-1"), RuntimeActionRef: executionTestRef("runtime_action", "action-1"), ReadinessRef: executionTestRef("readiness", "ready-1"), DecisionRef: executionTestRef("decision", "decision-1"), PolicyRef: executionTestRef("policy", "policy-1"), TargetRef: executionTestRef("target", "target-1"), EnvironmentRef: executionTestRef("environment", "production"), ProofRefs: []proof.RelationshipRef{executionTestRef("proof", "proof-1")}, CausalRefs: []proof.RelationshipRef{executionTestRef("proposal", "proposal-1")}}
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
	if err := WriteEvidenceAtomic(path, map[string]string{"ok": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteEvidenceAtomic(path, map[string]string{"ok": "2"}); err == nil {
		t.Fatal("evidence overwrite accepted")
	}
	linkedParent := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(t.TempDir(), linkedParent); err == nil {
		if err := WriteEvidenceAtomic(filepath.Join(linkedParent, "evidence.json"), map[string]string{"ok": "3"}); err == nil {
			t.Fatal("symlinked evidence parent accepted")
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

	withoutValidation := append(cloneLifecycleRecords(t, records[:8]), cloneLifecycleRecords(t, records[9:])...)
	if _, err := ReduceLifecycleChecked(withoutValidation); err == nil {
		t.Fatal("containment before effect validation accepted")
	}
	proofDrift := cloneLifecycleRecords(t, records)
	proofDrift[6].Execution.Binding.ProofRefs[0].Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := ReduceLifecycleChecked(proofDrift); err == nil {
		t.Fatal("cross-Proof execution lineage accepted")
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
	wrongPublic, _, _ := ed25519.GenerateKey(nil)
	if _, err := ReduceVerifiedLifecycle(signed, wrongPublic); err == nil {
		t.Fatal("verified lifecycle accepted an unrelated trust key")
	}
}
