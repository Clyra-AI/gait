package actioncontract

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
)

var expectedRuntimeGoldenDigests = map[string]string{
	"fixture-signing-key.public.b64": "sha256:4a09d7988c00029e6da1d966c512a6a515d421b5f05155d8c87e1b734e8480fa",
	"runtime-action.json":            "sha256:9b139b2071467a12b8bf2ab9e6e43dec82b93be018169faef311678bc7b8a424",
	"runtime-readiness.json":         "sha256:be846bbf532ef768ae46a68bd410592fda6dc55a3e85e28fe74ac8ed64aa541c",
	"runtime-lifecycle-record.json":  "sha256:076170542c3cad64f02175a9a68a8a8de3c4f469becfd2027769795a5ed2ba4c",
	"runtime-lifecycle-chain.json":   "sha256:d5c75d1fc66b8f2289da429e8f03bb2e818469b30f42280e9662e5e138faba85",
	"runtime-lifecycle-chain.jsonl":  "sha256:695840683e389d554245c07fb3cfae5f30ea2da506a597965185eedc89b5fed7",
}

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

func TestRuntimeActionReadinessLifecycleGoldens(t *testing.T) {
	goldenRoot := filepath.Join("..", "..", "testdata", "runtime-goldens")
	manifestRaw, err := os.ReadFile(filepath.Join(goldenRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		FixtureVersion string `json:"fixture_version"`
		Producer       struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"producer"`
		SourceCommit string `json:"source_commit"`
		Signing      struct {
			KeyID              string `json:"key_id"`
			PublicKeyPath      string `json:"public_key_path"`
			FixtureTestOnly    bool   `json:"fixture_test_only"`
			DevelopmentSigning bool   `json:"development_signing"`
			NonAuthoritative   bool   `json:"non_authoritative"`
		} `json:"signing"`
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil || manifest.FixtureVersion != "1" || manifest.SourceCommit == "" || manifest.Producer.Version != "v1.5.0" || !manifest.Signing.FixtureTestOnly || !manifest.Signing.DevelopmentSigning || !manifest.Signing.NonAuthoritative {
		t.Fatalf("invalid runtime golden manifest: %v", err)
	}
	publicRaw, err := os.ReadFile(filepath.Join(goldenRoot, manifest.Signing.PublicKeyPath))
	if err != nil {
		t.Fatal(err)
	}
	publicBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(publicRaw)))
	if err != nil || len(publicBytes) != ed25519.PublicKeySize {
		t.Fatalf("invalid runtime fixture public key: %v", err)
	}
	fixturePublic := ed25519.PublicKey(publicBytes)
	keySum := sha256.Sum256(fixturePublic)
	if hex.EncodeToString(keySum[:]) != manifest.Signing.KeyID {
		t.Fatalf("runtime fixture key id mismatch")
	}
	for _, file := range manifest.Files {
		want, known := expectedRuntimeGoldenDigests[file.Path]
		if !known || file.SHA256 != want {
			t.Fatalf("runtime golden manifest digest is not the released fixture digest: %s", file.Path)
		}
		raw, err := os.ReadFile(filepath.Join(goldenRoot, file.Path))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if got := "sha256:" + hex.EncodeToString(sum[:]); got != file.SHA256 {
			t.Fatalf("runtime golden digest drift: %s got=%s want=%s", file.Path, got, file.SHA256)
		}
	}
	if len(manifest.Files) != len(expectedRuntimeGoldenDigests) {
		t.Fatalf("runtime golden manifest file set drift: got=%d want=%d", len(manifest.Files), len(expectedRuntimeGoldenDigests))
	}
	entries, err := os.ReadDir(goldenRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "manifest.json" {
			continue
		}
		if _, known := expectedRuntimeGoldenDigests[entry.Name()]; !known {
			t.Fatalf("unmanifested runtime golden file: %s", entry.Name())
		}
	}
	action := ClassifyAction(ClassificationInput{ActionID: "runtime-golden-action", ActionClass: "read", CompositionRole: "source", DataClasses: []string{"internal"}, TargetTrustClass: "external", TransitionClass: "read", ExpectedOutcomeClass: "read", TargetRef: "target:golden"})
	validatorPrivate, validatorPublic := testGoldenValidatorKey()
	now := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	evidence := signedTestPrecondition(validatorPrivate, ReadinessPrecondition{RequirementID: "runtime-p1", Kind: "environment", Required: true, Producer: "validator", ControlMode: ControlModeEnforced, FreshnessState: "fresh", ObservedAt: "2026-07-19T00:30:00Z", MaxAgeSeconds: 3600, EvidenceState: "verified", EvidenceRefs: []string{"evidence:runtime-p1"}, ObservedResult: "pass", Environment: "production"})
	evidenceDigest := evidence.EvidenceDigest
	readiness := EvaluateReadiness(ReadinessInput{Now: now, ContractID: "pac-runtime-golden", TrustedValidatorRefs: []string{"validator"}, TrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validatorPublic}, Preconditions: []ReadinessPrecondition{evidence}})
	seed := sha256.Sum256([]byte("runtime-golden-key"))
	private := ed25519.NewKeyFromSeed(seed[:])
	contractRef := proof.RelationshipRef{Kind: "action_contract", ID: "pac-runtime-golden", Digest: "sha256:" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SchemaID: ProposedContractSchemaID, SchemaVersion: ProposedContractVersion, SourceProduct: "gait"}
	record, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleDecisionReady, Revision: 1, OccurredAt: now, ContractRef: contractRef, PreconditionRefs: []proof.RelationshipRef{{Kind: "precondition", ID: "runtime-p1", Digest: evidenceDigest, SchemaID: RuntimeReadinessSchemaID, SchemaVersion: RuntimeActionSchemaVersion, SourceProduct: "gait"}}, Decision: &readiness, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	assertGoldenJSON(t, filepath.Join(goldenRoot, "runtime-action.json"), action)
	assertGoldenJSON(t, filepath.Join(goldenRoot, "runtime-readiness.json"), readiness)
	assertGoldenJSON(t, filepath.Join(goldenRoot, "runtime-lifecycle-record.json"), record)
	proposalRef := proof.RelationshipRef{Kind: "action_contract", ID: "pac-runtime-golden", Digest: "sha256:" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SchemaID: ProposedContractSchemaID, SchemaVersion: ProposedContractVersion, SourceProduct: "gait"}
	activationRef := proof.RelationshipRef{Kind: "activated_action_contract", ID: "gact-runtime-golden", Digest: "sha256:" + "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", SchemaID: ActivatedSchemaID, SchemaVersion: ActivatedSchemaVersion, SourceProduct: "gait"}
	preconditionRef := proof.RelationshipRef{Kind: "precondition", ID: "runtime-p1", Digest: evidenceDigest, SchemaID: RuntimeReadinessSchemaID, SchemaVersion: RuntimeActionSchemaVersion, SourceProduct: "gait"}
	proposalEvent, _ := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleProposalIngested, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC), ContractRef: proposalRef, ProposalRef: &proposalRef, SigningPrivateKey: private})
	preconditionEvent, _ := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecyclePreconditionEvaluated, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 0, 30, 0, time.UTC), ContractRef: proposalRef, PreconditionRefs: []proof.RelationshipRef{preconditionRef}, SigningPrivateKey: private})
	requestEvent, _ := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleActivationRequested, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 1, 1, 30, 0, time.UTC), ContractRef: proposalRef, ProposalRef: &proposalRef, SigningPrivateKey: private})
	activatedEvent, _ := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleActivated, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 1, 2, 0, 0, time.UTC), ContractRef: proposalRef, ProposalRef: &proposalRef, ActivationRef: &activationRef, SigningPrivateKey: private})
	chainRecords := []LifecycleRecord{proposalEvent, preconditionEvent, record, requestEvent, activatedEvent}
	chainRaw, err := os.ReadFile(filepath.Join(goldenRoot, "runtime-lifecycle-chain.json"))
	if err != nil {
		t.Fatal(err)
	}
	var chain struct {
		Records []struct {
			Kind         string `json:"kind"`
			OccurredAt   string `json:"occurred_at"`
			RecordID     string `json:"record_id"`
			SignedDigest string `json:"signed_digest"`
			KeyID        string `json:"key_id"`
			Signature    string `json:"signature"`
		} `json:"records"`
	}
	if err := json.Unmarshal(chainRaw, &chain); err != nil {
		t.Fatal(err)
	}
	if len(chain.Records) != len(chainRecords) {
		t.Fatalf("lifecycle chain golden length mismatch: %d", len(chain.Records))
	}
	for i, expected := range chain.Records {
		actual := chainRecords[i]
		if expected.Kind != string(actual.Kind) || expected.RecordID != actual.RecordID || expected.SignedDigest != actual.Signature.SignedDigest || expected.KeyID != actual.Signature.KeyID || expected.Signature != actual.Signature.Sig {
			t.Fatalf("lifecycle chain golden drift at %d: expected=(%q,%q,%q,%q,%q) actual=(%q,%q,%q,%q,%q)", i, expected.Kind, expected.RecordID, expected.SignedDigest, expected.KeyID, expected.Signature, actual.Kind, actual.RecordID, actual.Signature.SignedDigest, actual.Signature.KeyID, actual.Signature.Sig)
		}
	}
	if snapshot, err := ReduceVerifiedLifecycle(chainRecords, fixturePublic); err != nil || snapshot.CurrentStatus != "activated" {
		t.Fatalf("golden lifecycle chain did not reduce: snapshot=%+v err=%v", snapshot, err)
	}
	fullRaw, err := os.ReadFile(filepath.Join(goldenRoot, "runtime-lifecycle-chain.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var fullRecords []LifecycleRecord
	for _, line := range bytes.Split(bytes.TrimSpace(fullRaw), []byte("\n")) {
		parsed, err := ParseLifecycleRecord(line)
		if err != nil {
			t.Fatalf("full lifecycle golden parse failed: %v", err)
		}
		if valid, err := VerifyLifecycleRecord(parsed, fixturePublic); err != nil || !valid {
			t.Fatalf("full lifecycle golden signature failed: valid=%t err=%v", valid, err)
		}
		fullRecords = append(fullRecords, parsed)
	}
	if len(fullRecords) != len(chainRecords) {
		t.Fatalf("full lifecycle golden count mismatch: %d", len(fullRecords))
	}
	if snapshot, err := ReduceVerifiedLifecycle(fullRecords, fixturePublic); err != nil || snapshot.CurrentStatus != "activated" {
		t.Fatalf("full lifecycle golden reduction failed: snapshot=%+v err=%v", snapshot, err)
	}
}

func assertGoldenJSON(t *testing.T, path string, value any) {
	t.Helper()
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	canonicalActual, err := proofcanon.CanonicalizeJSON(actual)
	if err != nil {
		t.Fatal(err)
	}
	canonicalExpected, err := proofcanon.CanonicalizeJSON(expected)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonicalActual, canonicalExpected) {
		t.Fatalf("runtime golden canonical drift: %s\nactual=%s\nexpected=%s", path, canonicalActual, canonicalExpected)
	}
}

func TestEvaluateReadinessFailsClosedForUntrustedWrkrAndJudge(t *testing.T) {
	validatorPrivate, validatorPublic := testValidatorKey()
	base := ReadinessInput{Now: time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC), TrustedValidatorRefs: []string{"policy-validator"}, TrustedValidatorKeys: map[string]ed25519.PublicKey{"policy-validator": validatorPublic}, Preconditions: []ReadinessPrecondition{
		{RequirementID: "p1", Kind: "effect_contract", Required: true, Producer: "wrkr", ControlMode: ControlModeSelfAttested, FreshnessState: "fresh", ObservedResult: "pass"},
		{RequirementID: "p2", Kind: "policy_digest", Required: true, Producer: "judge", ControlMode: ControlModeObserved, FreshnessState: "fresh", ObservedResult: "pass"},
		signedTestPrecondition(validatorPrivate, ReadinessPrecondition{RequirementID: "p3", Kind: "sandbox", Required: true, Producer: "policy-validator", ControlMode: ControlModeEnforced, FreshnessState: "fresh", ObservedResult: "pass", ObservedAt: "2026-07-19T00:30:00Z", MaxAgeSeconds: 3600, EvidenceState: "verified", EvidenceRefs: []string{"evidence:p3"}, SandboxStatus: "clean"}),
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
	wrongSeed := sha256.Sum256([]byte("wrong-runtime-validator-key"))
	wrongPrivate := ed25519.NewKeyFromSeed(wrongSeed[:])
	base.TrustedValidatorKeys = map[string]ed25519.PublicKey{"policy-validator": wrongPrivate.Public().(ed25519.PublicKey)}
	wrongResult := EvaluateReadiness(base)
	if wrongResult.Preconditions[2].Status == ReadinessSatisfied {
		t.Fatal("wrong validator key satisfied readiness")
	}
}

func TestEvaluateReadinessStatusesAndBoundaryRefsRemainSeparate(t *testing.T) {
	validatorPrivate, validatorPublic := testValidatorKey()
	result := EvaluateReadiness(ReadinessInput{Now: time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC), TrustedValidatorRefs: []string{"validator"}, TrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validatorPublic}, Preconditions: []ReadinessPrecondition{
		{RequirementID: "none", Kind: "approval", Required: false},
		signedTestPrecondition(validatorPrivate, ReadinessPrecondition{RequirementID: "ok", Kind: "environment", Required: true, Producer: "validator", ControlMode: ControlModeEnforced, FreshnessState: "current", ObservedResult: "verified", ObservedAt: "2026-07-19T00:30:00Z", MaxAgeSeconds: 3600, EvidenceState: "verified", EvidenceRefs: []string{"evidence:1"}, BoundaryRefs: []string{"boundary:1"}, Environment: "production"}),
		signedTestPrecondition(validatorPrivate, ReadinessPrecondition{RequirementID: "bad", Kind: "target", Required: true, Producer: "validator", ControlMode: ControlModeEnforced, FreshnessState: "expired", ObservedResult: "verified", ObservedAt: "2026-07-18T00:00:00Z", MaxAgeSeconds: 3600, EvidenceState: "verified", EvidenceRefs: []string{"boundary:wrong"}, Target: "prod"}),
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

func TestEvaluateReadinessBindsEverySemanticClaimField(t *testing.T) {
	private, public := testValidatorKey()
	base := signedTestPrecondition(private, ReadinessPrecondition{
		RequirementID: "claim", Kind: "environment", Required: true,
		Producer: "validator", ControlMode: ControlModeEnforced, FreshnessState: "fresh",
		ObservedResult: "pass", ObservedAt: "2026-07-19T00:30:00Z", MaxAgeSeconds: 3600,
		EvidenceState: "verified", EvidenceRefs: []string{"evidence:claim"}, Environment: "production",
	})
	mutations := []struct {
		name string
		edit func(*ReadinessPrecondition)
	}{
		{name: "observed_result", edit: func(item *ReadinessPrecondition) { item.ObservedResult = "fail" }},
		{name: "environment", edit: func(item *ReadinessPrecondition) { item.Environment = "staging" }},
		{name: "target", edit: func(item *ReadinessPrecondition) { item.Target = "target:other" }},
		{name: "freshness_time", edit: func(item *ReadinessPrecondition) { item.ObservedAt = "2026-07-19T00:31:00Z" }},
		{name: "evidence_refs", edit: func(item *ReadinessPrecondition) { item.EvidenceRefs = []string{"evidence:other"} }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			item := base
			mutation.edit(&item)
			result := EvaluateReadiness(ReadinessInput{Now: time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC), TrustedValidatorRefs: []string{"validator"}, TrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": public}, Preconditions: []ReadinessPrecondition{item}})
			if result.Ready || result.Preconditions[0].Status == ReadinessSatisfied {
				t.Fatalf("mutated claim satisfied readiness: %#v", result)
			}
			if !containsStringValue(result.Preconditions[0].ReasonCodes, "evidence:claim_digest_mismatch") {
				t.Fatalf("missing claim digest mismatch for %s: %#v", mutation.name, result.Preconditions[0].ReasonCodes)
			}
		})
	}
}

func TestReadinessFromContractProjectsAuthorityAndCompensationRequirements(t *testing.T) {
	now := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name     string
		contract map[string]any
	}{
		{
			name: "authority_only",
			contract: map[string]any{
				"contract_id": "authority-only",
				"authority_requirements": []any{map[string]any{
					"requirement_id": "authority-1", "kind": "policy_authority", "required_constraint": "authority:present", "evidence_state": "unknown", "freshness_state": "unknown",
				}},
			},
		},
		{
			name: "required_compensation",
			contract: map[string]any{
				"contract_id": "compensation-only",
				"compensation_requirement": map[string]any{
					"required": true, "kind": "rollback", "target": "target:production", "verification_required": true, "evidence_state": "unknown", "freshness_state": "unknown",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := ReadinessFromContract(test.contract, ReadinessInput{Now: now})
			if result.Ready || result.Status != ReadinessInconclusive || len(result.Preconditions) == 0 {
				t.Fatalf("required contract evidence was omitted or satisfied: %#v", result)
			}
		})
	}
}

func TestEvaluateReadinessKindConstraintsAreBoundedAndCompared(t *testing.T) {
	private, public := testValidatorKey()
	base := ReadinessPrecondition{RequirementID: "kind", Required: true, Producer: "validator", ControlMode: ControlModeEnforced, FreshnessState: "fresh", ObservedResult: "pass", ObservedAt: "2026-07-19T00:30:00Z", MaxAgeSeconds: 3600, EvidenceState: "verified", EvidenceRefs: []string{"evidence:kind"}}
	for _, test := range []struct {
		name   string
		item   ReadinessPrecondition
		reason string
	}{
		{name: "environment_constraint", item: func() ReadinessPrecondition {
			item := base
			item.Kind = "environment"
			item.Environment = "staging"
			item.RequiredConstraint = "environment:production"
			return item
		}(), reason: "environment:constraint_mismatch"},
		{name: "target_constraint", item: func() ReadinessPrecondition {
			item := base
			item.Kind = "target"
			item.Target = "target:other"
			item.RequiredConstraint = "target:production"
			return item
		}(), reason: "target:constraint_mismatch"},
		{name: "credential_enum", item: func() ReadinessPrecondition {
			item := base
			item.Kind = "credential_mode"
			item.CredentialMode = "arbitrary"
			return item
		}(), reason: "credential:mode_missing"},
		{name: "resource_budget", item: func() ReadinessPrecondition {
			item := base
			item.Kind = "resource_budget"
			item.ResourceStatus = "clean"
			item.TTLSeconds = 60
			return item
		}(), reason: "resource:budget_invalid"},
		{name: "cleanup_state", item: func() ReadinessPrecondition {
			item := base
			item.Kind = "cleanup"
			item.ResourceStatus = "partial"
			return item
		}(), reason: "resource:cleanup_incomplete"},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := signedTestPrecondition(private, test.item)
			result := EvaluateReadiness(ReadinessInput{Now: time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC), TrustedValidatorRefs: []string{"validator"}, TrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": public}, Preconditions: []ReadinessPrecondition{item}})
			if result.Ready || result.Preconditions[0].Status == ReadinessSatisfied || !containsStringValue(result.Preconditions[0].ReasonCodes, test.reason) {
				t.Fatalf("kind constraint was not enforced: %#v", result)
			}
		})
	}
}

func TestEvaluateReadinessEmptyInputIsNonAuthoritative(t *testing.T) {
	result := EvaluateReadiness(ReadinessInput{})
	if result.Ready || result.Status != ReadinessNotRequired {
		t.Fatalf("empty readiness input became authoritative: %#v", result)
	}
}

func TestRuntimeStrictInputsRejectDuplicateUnknownAndUnsafePaths(t *testing.T) {
	if _, err := ParseClassificationInput([]byte(`{"action_id":"a","unknown":true}`)); err == nil {
		t.Fatal("unknown runtime action field accepted")
	}
	if _, err := ParseReadinessInput([]byte(`{"contract_id":"c","contract_id":"d","preconditions":[]}`)); err == nil {
		t.Fatal("duplicate readiness field accepted")
	}
	if runtime.GOOS == "windows" {
		t.Skip("symlink safety test requires portable symlink support")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(root, "input.json")
	if err := os.WriteFile(inputPath, []byte(`{"action_id":"a"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "link.json")
	if err := os.Symlink(inputPath, linkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRuntimeInput(linkPath); err == nil {
		t.Fatal("symlink runtime input accepted")
	}
}

func TestLifecycleRecordsSignAndReducePurely(t *testing.T) {
	seed := sha256.Sum256([]byte("runtime-lifecycle-test-key"))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	contractRef := proof.RelationshipRef{Kind: "action_contract", ID: "pac-1", Digest: "sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SchemaID: ProposedContractSchemaID, SchemaVersion: ProposedContractVersion, SourceProduct: "gait"}
	first, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleProposalIngested, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC), ContractRef: contractRef, ProposalRef: &contractRef, SigningPrivateKey: private})
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
	firstRaw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseLifecycleRecord(firstRaw); err != nil {
		t.Fatalf("strict lifecycle parser rejected valid record: %v", err)
	}
	unknownRaw := append([]byte(`{"unknown":true,`), firstRaw[1:]...)
	if _, err := ParseLifecycleRecord(unknownRaw); err == nil {
		t.Fatal("strict lifecycle parser accepted unknown field")
	}
	duplicateRaw := bytes.Replace(firstRaw, []byte(`"schema_id":`), []byte(`"schema_id":"duplicate","schema_id":`), 1)
	if _, err := ParseLifecycleRecord(duplicateRaw); err == nil {
		t.Fatal("strict lifecycle parser accepted duplicate field")
	}
	preconditionRef := proof.RelationshipRef{Kind: "precondition", ID: "p1", Digest: "sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	decisionPrecondition := ReadinessPrecondition{RequirementID: "p1", Kind: "environment", Required: true, ControlMode: ControlModeUnknown, Status: ReadinessSatisfied, EvidenceDigest: preconditionRef.Digest}
	second, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleDecisionReady, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 1, 0, 0, time.UTC), ContractRef: contractRef, PreconditionRefs: []proof.RelationshipRef{preconditionRef}, Decision: &ReadinessResult{Ready: true, Status: ReadinessSatisfied, Preconditions: []ReadinessPrecondition{decisionPrecondition}}, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	request, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleActivationRequested, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 0, 30, 0, time.UTC), ContractRef: contractRef, ProposalRef: &contractRef, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	preconditionEvent, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecyclePreconditionEvaluated, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 0, 15, 0, time.UTC), ContractRef: contractRef, PreconditionRefs: []proof.RelationshipRef{preconditionRef}, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := ReduceLifecycle([]LifecycleRecord{first, preconditionEvent, request, second})
	if snapshot.CurrentStatus != "ready" || !snapshot.ProposalIngested || !snapshot.DecisionReady {
		t.Fatalf("unexpected reduced lifecycle: %#v", snapshot)
	}
	if snapshot.Records[0].RecordID != first.RecordID {
		t.Fatalf("reducer changed the validated input order")
	}
	decisionWithMissingEvaluation, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleDecisionReady, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 1, 30, 0, time.UTC), ContractRef: contractRef, Decision: &ReadinessResult{Ready: true, Status: ReadinessSatisfied, Preconditions: []ReadinessPrecondition{decisionPrecondition}}, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReduceLifecycleChecked([]LifecycleRecord{first, decisionWithMissingEvaluation}); err == nil {
		t.Fatal("decision with missing precondition evaluation accepted")
	}
	wrongPreconditionEvent, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecyclePreconditionEvaluated, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 1, 0, 0, time.UTC), ContractRef: contractRef, PreconditionRefs: []proof.RelationshipRef{{Kind: "precondition", ID: "other", Digest: decisionPrecondition.EvidenceDigest}}, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReduceLifecycleChecked([]LifecycleRecord{first, wrongPreconditionEvent, decisionWithMissingEvaluation}); err == nil {
		t.Fatal("decision with mismatched precondition evaluation accepted")
	}
	zeroPrecondition := second
	decisionCopy := *zeroPrecondition.Decision
	decisionCopy.Preconditions = nil
	zeroPrecondition.Decision = &decisionCopy
	if _, err := VerifyLifecycleRecord(zeroPrecondition, public); err == nil {
		t.Fatal("verified lifecycle accepted zero-precondition ready decision")
	}
	if _, err := ReduceLifecycleChecked([]LifecycleRecord{first, zeroPrecondition}); err == nil {
		t.Fatal("reducer accepted zero-precondition ready decision")
	}
	if snapshot := ReduceLifecycle([]LifecycleRecord{second, request, first}); snapshot.CurrentStatus != "invalid" || len(snapshot.ReasonCodes) != 1 || snapshot.ReasonCodes[0] != "lifecycle_input_reordered" {
		t.Fatalf("reducer accepted reordered input: %#v", snapshot)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("snapshot should be serializable: %v", err)
	}
	if _, err := ReduceLifecycleChecked([]LifecycleRecord{first, first}); err == nil {
		t.Fatal("duplicate lifecycle record accepted")
	}
	other := first
	other.RecordID = "gait-lr-ffffffffffffffff"
	other.ContractRef.ID = "other"
	if _, err := ReduceLifecycleChecked([]LifecycleRecord{first, other}); err == nil {
		t.Fatal("mixed lifecycle contracts accepted")
	}
	activationRef := proof.RelationshipRef{Kind: "activated_action_contract", ID: "gact-1", Digest: "sha256:" + "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", SchemaID: ActivatedSchemaID, SchemaVersion: ActivatedSchemaVersion, SourceProduct: "gait"}
	noDecisionActivation, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleActivated, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 2, 0, 0, time.UTC), ContractRef: contractRef, ProposalRef: &contractRef, ActivationRef: &activationRef, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReduceLifecycleChecked([]LifecycleRecord{first, request, noDecisionActivation}); err == nil {
		t.Fatal("activation without a ready decision accepted")
	}
	rejected, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleRejected, Revision: 1, OccurredAt: time.Date(2026, 7, 19, 0, 0, 20, 0, time.UTC), ContractRef: contractRef, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReduceLifecycleChecked([]LifecycleRecord{first, rejected, request}); err == nil {
		t.Fatal("activation request after rejection accepted")
	}
	forged := first
	forged.ReasonCodes = []string{"forged"}
	if _, err := ReduceVerifiedLifecycle([]LifecycleRecord{forged}, public); err == nil {
		t.Fatal("forged lifecycle record accepted by verified reducer")
	}
}

func testValidatorKey() (ed25519.PrivateKey, ed25519.PublicKey) {
	seed := sha256.Sum256([]byte("runtime-validator-key"))
	private := ed25519.NewKeyFromSeed(seed[:])
	return private, private.Public().(ed25519.PublicKey)
}

func testGoldenValidatorKey() (ed25519.PrivateKey, ed25519.PublicKey) {
	seed := sha256.Sum256([]byte("runtime-golden-key"))
	private := ed25519.NewKeyFromSeed(seed[:])
	return private, private.Public().(ed25519.PublicKey)
}

func signTestEvidence(private ed25519.PrivateKey, digest string) string {
	raw, err := hex.DecodeString(strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(private, raw))
}

func signedTestPrecondition(private ed25519.PrivateKey, item ReadinessPrecondition) ReadinessPrecondition {
	digest, err := CanonicalReadinessClaimDigest(item)
	if err != nil {
		panic(err)
	}
	item.EvidenceDigest = digest
	item.ValidatorSignature = signTestEvidence(private, digest)
	return item
}
