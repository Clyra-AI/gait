package actioncontract

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	proof "github.com/Clyra-AI/proof"
)

func TestClassifyRuntimeActionIsMonotonicAndDeterministic(t *testing.T) {
	input := ClassificationInput{
		ActionID: "act-1", ActionClass: ActionClassRead, CompositionRole: "source",
		TargetTrustClass: "external", Hints: []string{"delete_customer_record"},
		DataClasses: []string{"pii", "internal", "pii"}, ResourceLifecycleActions: []string{"cleanup"},
	}
	first := ClassifyRuntimeAction(input)
	second := ClassifyRuntimeAction(input)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("classification is not deterministic: %#v %#v", first, second)
	}
	if !first.Valid {
		t.Fatalf("expected valid classification: %v", first.ReasonCodes)
	}
	if first.Action.ActionClass != ActionClassDelete {
		t.Fatalf("inference lowered supplied action: %#v", first.Action)
	}
	if first.Action.ExpectedOutcomeClass != "production_mutation" {
		t.Fatalf("unexpected outcome: %q", first.Action.ExpectedOutcomeClass)
	}
	if got := first.Action.DataClasses; !reflect.DeepEqual(got, []string{"internal", "pii"}) {
		t.Fatalf("data classes not normalized: %#v", got)
	}
	raised := ClassifyRuntimeAction(ClassificationInput{ActionID: "act-2", ActionClass: ActionClassRead, CompositionRole: "source", TargetTrustClass: "external", TransitionClass: "read", ExpectedOutcomeClass: "read", Hints: []string{"production deploy"}})
	if !raised.Valid || raised.Action.ActionClass != ActionClassDeploy || raised.Action.ExpectedOutcomeClass != "production_deploy" || raised.Action.TargetTrustClass != "production" {
		t.Fatalf("inference lowered a supplied classification: %#v", raised)
	}
}

func TestClassifyArtifactConsumesReleasedFixtureWithoutClaimingEffect(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	artifact, result := ValidateArtifactBytes(raw, ValidationOptions{Now: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)})
	if !result.Valid {
		t.Fatalf("fixture should validate: %v", result.Reasons)
	}
	classification := ClassifyArtifact(artifact)
	if !classification.Valid {
		t.Fatalf("fixture classification should validate: %v", classification.ReasonCodes)
	}
	if classification.Action.ExpectedOutcomeClass != "data_egress" {
		t.Fatalf("intended outcome changed: %#v", classification.Action)
	}
	if len(classification.Action.Stages) != 2 || classification.Action.Stages[0].StageID == "" {
		t.Fatalf("bounded transition stages were not projected: %#v", classification.Action.Stages)
	}
	if classification.Action.ObservedEffect != nil {
		t.Fatalf("classification must not claim an observed effect")
	}
}

func TestRuntimeFixturePackClassificationAndReadinessGoldens(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected")
	paths, err := filepath.Glob(filepath.Join(root, "*", "pac-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 8 {
		t.Fatalf("released fixture pack unexpectedly small: %d", len(paths))
	}
	for _, path := range paths {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		artifact, validation := ValidateArtifactBytes(raw, ValidationOptions{Now: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)})
		if !validation.Valid {
			continue
		} // rejection dispositions remain validation failures.
		first := ClassifyArtifact(artifact)
		second := ClassifyArtifact(artifact)
		if !first.Valid || !reflect.DeepEqual(first, second) {
			t.Fatalf("non-deterministic runtime classification for %s: %#v %#v", path, first, second)
		}
		if first.Action.ObservedEffect != nil {
			t.Fatalf("runtime fixture claimed an effect: %s", path)
		}
		readiness := ReadinessFromArtifact(artifact, ReadinessInput{})
		if readiness.Ready || readiness.Status == ReadinessSatisfied {
			t.Fatalf("fixture unexpectedly became ready without trusted validators: %s %#v", path, readiness)
		}
	}
}

func TestEvaluateReadinessFailsClosedForUntrustedWrkrAndJudge(t *testing.T) {
	base := ReadinessInput{TrustedValidatorRefs: []string{"policy-validator"}, Preconditions: []ReadinessPrecondition{
		{RequirementID: "p1", Kind: "effect_contract", Required: true, Producer: "wrkr", ControlMode: ControlModeSelfAttested, FreshnessState: "fresh", ObservedResult: "pass"},
		{RequirementID: "p2", Kind: "policy_digest", Required: true, Producer: "judge", ControlMode: ControlModeObserved, FreshnessState: "fresh", ObservedResult: "pass"},
		{RequirementID: "p3", Kind: "sandbox", Required: true, Producer: "policy-validator", ControlMode: ControlModeEnforced, FreshnessState: "fresh", ObservedResult: "pass"},
	}}
	result := EvaluateReadiness(base)
	if result.Ready || result.Status != ReadinessInconclusive {
		t.Fatalf("expected fail-closed inconclusive result: %#v", result)
	}
	if result.Preconditions[0].Status != ReadinessInconclusive || result.Preconditions[1].Status != ReadinessInconclusive {
		t.Fatalf("untrusted producers satisfied a requirement: %#v", result.Preconditions)
	}
	if result.Preconditions[2].Status != ReadinessSatisfied {
		t.Fatalf("trusted validator did not satisfy: %#v", result.Preconditions[2])
	}
}

func TestEvaluateReadinessStatusesAndBoundaryRefsRemainSeparate(t *testing.T) {
	result := EvaluateReadiness(ReadinessInput{TrustedValidatorRefs: []string{"validator"}, Preconditions: []ReadinessPrecondition{
		{RequirementID: "none", Kind: "approval", Required: false},
		{RequirementID: "ok", Kind: "environment", Required: true, Producer: "validator", ControlMode: ControlModeEnforced, FreshnessState: "current", ObservedResult: "verified", EvidenceRefs: []string{"evidence:1"}, BoundaryRefs: []string{"boundary:1"}},
		{RequirementID: "bad", Kind: "target", Required: true, Producer: "validator", ControlMode: ControlModeEnforced, FreshnessState: "expired", ObservedResult: "verified", EvidenceRefs: []string{"boundary:wrong"}},
	}})
	if result.Status != ReadinessInconclusive || result.Ready {
		t.Fatalf("expected stale required evidence to block: %#v", result)
	}
	if result.Preconditions[0].Status != ReadinessNotRequired {
		t.Fatalf("not-required status lost: %#v", result.Preconditions[0])
	}
	if result.Preconditions[1].Status != ReadinessSatisfied {
		t.Fatalf("current evidence not satisfied: %#v", result.Preconditions[1])
	}
	if result.Preconditions[1].BoundaryRefs[0] != "boundary:1" || result.Preconditions[1].EvidenceRefs[0] != "evidence:1" {
		t.Fatalf("boundary/evidence refs were conflated: %#v", result.Preconditions[1])
	}
}

func TestLifecycleRecordsSignAndReducePurely(t *testing.T) {
	seed := sha256.Sum256([]byte("runtime-lifecycle-test-key"))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	contractRef := proof.RelationshipRef{Kind: "action_contract", ID: "pac-1", Digest: "sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SchemaID: ProposedContractSchemaID, SchemaVersion: ProposedContractVersion, SourceProduct: "gait"}
	first, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleProposalIngested, OccurredAt: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC), ContractRef: contractRef, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	if first.Correlation.BindingMode != proof.BindingModeDigestBound || first.Correlation.ContractRef == nil {
		t.Fatalf("lifecycle correlation did not retain digest-bound contract ref: %#v", first.Correlation)
	}
	if err := first.Correlation.Validate(); err != nil {
		t.Fatalf("lifecycle correlation profile should validate: %v", err)
	}
	if ok, err := VerifyLifecycleRecord(first, public); err != nil || !ok {
		t.Fatalf("lifecycle signature did not verify: %v", err)
	}
	second, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleDecisionReady, OccurredAt: time.Date(2026, 7, 19, 0, 1, 0, 0, time.UTC), ContractRef: contractRef, Decision: &ReadinessResult{Ready: true, Status: ReadinessSatisfied}, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := ReduceLifecycle([]LifecycleRecord{second, first})
	if snapshot.CurrentStatus != "ready" || !snapshot.ProposalIngested || !snapshot.DecisionReady {
		t.Fatalf("unexpected reduced lifecycle: %#v", snapshot)
	}
	if snapshot.Records[0].RecordID != first.RecordID {
		t.Fatalf("reducer did not order records deterministically")
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("snapshot should be serializable: %v", err)
	}
}
