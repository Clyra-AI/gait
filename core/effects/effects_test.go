package effects

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	proofschema "github.com/Clyra-AI/proof/schema"
)

const testDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func validSnapshot(t *testing.T) Snapshot {
	t.Helper()
	snapshot := Snapshot{
		SchemaID: SnapshotSchemaID, SchemaVersion: SchemaVersion, SnapshotID: "effect-snapshot:demo",
		ResourceKind: ResourcePostgres, Selector: Selector{Resource: "postgres.table", Scope: "public", Name: "orders"},
		Before:    Observation{State: ObservationPresent, Digest: testDigest, Count: int64ptr(2), Identity: "orders:1,2", Owner: "owner:app", TTLSeconds: int64ptr(3600), ObservedAt: "2026-08-19T00:00:00Z", EvidenceRefs: []string{"ref:before"}},
		After:     Observation{State: ObservationPresent, Digest: testDigest, Count: int64ptr(2), Identity: "orders:1,2", Owner: "owner:app", TTLSeconds: int64ptr(3600), ObservedAt: "2026-08-19T00:00:01Z", EvidenceRefs: []string{"ref:after"}},
		Collector: Collector{Name: "fixture-collector", Version: "1", Mode: "deterministic"}, Capture: Capture{Mode: "reference", SourceRef: "capture:demo", CapturedAt: "2026-08-19T00:00:02Z"}, Redaction: Redaction{Mode: "none"}, Correlation: Correlation{ActionDigest: testDigest, ProofRefs: []string{testDigest}}, Completeness: CompletenessComplete, Enforcement: EnforcementVerified, EvidenceRefs: []string{"ref:before", "ref:after"},
	}
	signed, err := snapshot.Sign(testPrivateKey(), "collector_signed")
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func validContract() Contract {
	return Contract{SchemaID: ContractSchemaID, SchemaVersion: SchemaVersion, ContractID: "effect-contract:orders", Name: "orders lifecycle", Predicates: []Predicate{
		{ID: "count-stable", Kind: PredicateInvariant, Field: "count"},
		{ID: "owner-expected", Kind: PredicateExpect, Field: "after.owner", Operator: "equals", Expected: "owner:app"},
		{ID: "forbid-drift", Kind: PredicateForbid, Field: "after.identity", Operator: "equals", Expected: "orders:deleted"},
	}}
}

func TestValidateSnapshotAndCanonicalDigest(t *testing.T) {
	snapshot := validSnapshot(t)
	if result := ValidateSnapshot(snapshot); !result.Valid {
		t.Fatalf("valid snapshot rejected: %+v", result)
	}
	first, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotCopy := snapshot
	second, err := json.Marshal(snapshotCopy)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("snapshot JSON is not deterministic")
	}
	snapshot.EvidenceRefs = []string{"ref:z", "ref:a"}
	snapshot.Before.EvidenceRefs = []string{"ref:before-z", "ref:before-a"}
	digestOne, err := snapshot.CanonicalDigest()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.EvidenceRefs = []string{"ref:a", "ref:z"}
	snapshot.Before.EvidenceRefs = []string{"ref:before-a", "ref:before-z"}
	digestTwo, err := snapshot.CanonicalDigest()
	if err != nil || digestOne != digestTwo {
		t.Fatalf("reference ordering changed digest: %s != %s", digestOne, digestTwo)
	}
	snapshot = validSnapshot(t)
	snapshot.CanonicalContentDigest = testDigest
	if result := ValidateSnapshot(snapshot); result.Valid || !hasReason(result, ReasonDigestMismatch) {
		t.Fatalf("tampered snapshot digest accepted: %+v", result)
	}
	snapshot = validSnapshot(t)
	snapshot.Redaction.Mode = "invalid"
	if result := ValidateSnapshot(snapshot); result.Valid || !hasReason(result, ReasonRedactionInvalid) {
		t.Fatalf("invalid redaction accepted: %+v", result)
	}
}

func TestEffectContractGradesExpectForbidAndInvariantDeterministically(t *testing.T) {
	snapshot := validSnapshot(t)
	contract := validContract()
	if result := Grade(snapshot, contract); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonTrustedKeyMissing) {
		t.Fatalf("untrusted self-carried key was authoritative: %+v", result)
	}
	wrongSeed := sha256.Sum256([]byte("wrong-effects-key-v1"))
	if result := GradeWithOptions(snapshot, contract, GradeOptions{TrustedCollectorPublicKey: ed25519.NewKeyFromSeed(wrongSeed[:]).Public().(ed25519.PublicKey)}); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonTrustedKeyMismatch) {
		t.Fatalf("wrong trusted collector key was accepted: %+v", result)
	}
	first := testGrade(snapshot, contract)
	second := testGrade(snapshot, contract)
	if first.Status != GradePass || first.EvidenceStatus != EnforcementVerified || len(first.Evaluations) != 3 {
		t.Fatalf("unexpected pass result: %+v", first)
	}
	firstRaw, _ := json.Marshal(first)
	secondRaw, _ := json.Marshal(second)
	if string(firstRaw) != string(secondRaw) {
		t.Fatalf("grading is not deterministic: %s != %s", firstRaw, secondRaw)
	}
	snapshot.After.Owner = "owner:other"
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradeFail || !hasGradeReason(result, ReasonPredicateFailed) {
		t.Fatalf("expect failure not reported: %+v", result)
	}
	snapshot = validSnapshot(t)
	snapshot.After.Identity = "orders:deleted"
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradeFail || !hasGradeReason(result, ReasonPredicateFailed) {
		t.Fatalf("forbid failure not reported: %+v", result)
	}
}

func TestEffectGradeRequiresExpectedCorrelationAndRejectsMismatch(t *testing.T) {
	snapshot := validSnapshot(t)
	contract := validContract()
	trusted := testPrivateKey().Public().(ed25519.PublicKey)
	if result := GradeWithOptions(snapshot, contract, GradeOptions{TrustedCollectorPublicKey: trusted}); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonCorrelationMissing) {
		t.Fatalf("missing caller correlation was authoritative: %+v", result)
	}
	wrong := testDigest
	if result := GradeWithOptions(snapshot, contract, GradeOptions{TrustedCollectorPublicKey: trusted, ExpectedCorrelation: &CorrelationExpectation{ActionDigest: wrong}}); result.Status != GradePass {
		t.Fatalf("matching caller correlation did not pass: %+v", result)
	}
	wrong = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if result := GradeWithOptions(snapshot, contract, GradeOptions{TrustedCollectorPublicKey: trusted, ExpectedCorrelation: &CorrelationExpectation{ActionDigest: wrong}}); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonCorrelationMismatch) {
		t.Fatalf("mismatched caller correlation was authoritative: %+v", result)
	}
}

func TestEffectGradeIsInconclusiveForPartialRedactedOrUnknownEvidence(t *testing.T) {
	snapshot := validSnapshot(t)
	contract := validContract()
	snapshot.Completeness = CompletenessPartial
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonEvidenceIncomplete) {
		t.Fatalf("partial evidence was authoritative: %+v", result)
	}
	snapshot = validSnapshot(t)
	snapshot.Enforcement = EnforcementUnknown
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradeInconclusive {
		t.Fatalf("unknown enforcement was authoritative: %+v", result)
	}
	snapshot = validSnapshot(t)
	snapshot.Enforcement = EnforcementObservedOnly
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonObservedOnly) {
		t.Fatalf("observed-only evidence was authoritative: %+v", result)
	}
	snapshot = validSnapshot(t)
	snapshot.Redaction.Mode = "reference_only"
	snapshot.After.Owner = ""
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonPredicateInconclusive) {
		t.Fatalf("redacted field was not inconclusive: %+v", result)
	}
	snapshot = validSnapshot(t)
	snapshot.After.State = ObservationAbsent
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonPredicateInconclusive) {
		t.Fatalf("absent observation satisfied a field predicate: %+v", result)
	}
}

func TestEffectStatePredicateCanAssertDeletionButNotUnknownState(t *testing.T) {
	snapshot := validSnapshot(t)
	contract := Contract{SchemaID: ContractSchemaID, SchemaVersion: SchemaVersion, ContractID: "contract:deletion", Name: "deletion", Predicates: []Predicate{{ID: "deleted", Kind: PredicateExpect, Field: "after.state", Operator: "equals", Expected: ObservationAbsent}}}
	snapshot.After.State = ObservationAbsent
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradePass {
		t.Fatalf("absent state was not authoritative: %+v", result)
	}
	snapshot.After.State = ObservationUnknown
	refreshSnapshot(t, &snapshot)
	if result := testGrade(snapshot, contract); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonPredicateInconclusive) {
		t.Fatalf("unknown state was authoritative: %+v", result)
	}
}

func TestEffectContractValidationRejectsDuplicatesAndUnknownKinds(t *testing.T) {
	contract := validContract()
	contract.Predicates = append(contract.Predicates, contract.Predicates[0])
	if result := ValidateContract(contract); result.Valid || !hasReason(result, ReasonPredicateInvalid) {
		t.Fatalf("duplicate predicate accepted: %+v", result)
	}
	contract = validContract()
	contract.Predicates[0].Kind = "execute"
	if result := ValidateContract(contract); result.Valid || !hasReason(result, ReasonPredicateInvalid) {
		t.Fatalf("unknown predicate kind accepted: %+v", result)
	}
	contract = validContract()
	contract.Predicates[1].Operator = "unchanged"
	if result := ValidateContract(contract); result.Valid || !hasReason(result, ReasonPredicateInvalid) {
		t.Fatalf("unchanged expect predicate accepted: %+v", result)
	}
}

func TestEffectPredicateOperatorsAndUnavailableFields(t *testing.T) {
	snapshot := validSnapshot(t)
	contract := Contract{SchemaID: ContractSchemaID, SchemaVersion: SchemaVersion, ContractID: "contract:operators", Name: "operators", Predicates: []Predicate{
		{ID: "equals", Kind: PredicateExpect, Field: "after.identity", Operator: "equals", Expected: "orders:1,2"},
		{ID: "not-equals", Kind: PredicateExpect, Field: "after.owner", Operator: "not_equals", Expected: "owner:other"},
		{ID: "contains", Kind: PredicateExpect, Field: "after.identity", Operator: "contains", Expected: "orders"},
		{ID: "exists", Kind: PredicateExpect, Field: "after.digest", Operator: "exists"},
		{ID: "gte", Kind: PredicateExpect, Field: "after.count", Operator: "gte", Expected: int64(1)},
		{ID: "lte", Kind: PredicateExpect, Field: "after.ttl_seconds", Operator: "lte", Expected: int64(3600)},
	}}
	if result := testGrade(snapshot, contract); result.Status != GradePass {
		t.Fatalf("operator contract did not pass: %+v", result)
	}
	contract.Predicates = append(contract.Predicates, Predicate{ID: "missing", Kind: PredicateExpect, Field: "after.missing", Operator: "equals", Expected: "x"})
	if result := testGrade(snapshot, contract); result.Status != GradeInconclusive || !hasGradeReason(result, ReasonPredicateInconclusive) {
		t.Fatalf("missing field was not inconclusive: %+v", result)
	}
	contract = validContract()
	contract.Predicates[0].Kind = PredicateExpect
	contract.Predicates[0].Field = "after.count"
	contract.Predicates[0].Operator = "unknown"
	if result := testGrade(snapshot, contract); result.Status != GradeInconclusive {
		t.Fatalf("unsupported operator was not inconclusive: %+v", result)
	}
}

func TestEffectSnapshotValidationRejectsMalformedFields(t *testing.T) {
	snapshot := validSnapshot(t)
	snapshot.Before.State = "bad"
	badCount := int64(-1)
	snapshot.Before.Count = &badCount
	snapshot.Before.ObservedAt = "bad-time"
	snapshot.Capture.Mode = "bad"
	snapshot.Capture.CapturedAt = "bad-time"
	snapshot.Redaction.Mode = "bad"
	snapshot.Completeness = "bad"
	snapshot.Enforcement = "bad"
	snapshot.EvidenceRefs = nil
	if result := ValidateSnapshot(snapshot); result.Valid || len(result.ReasonCodes) < 5 {
		t.Fatalf("malformed snapshot reasons incomplete: %+v", result)
	}
}

func TestEffectSnapshotValidationRejectsReversedTemporalOrder(t *testing.T) {
	snapshot := validSnapshot(t)
	snapshot.Before.ObservedAt = "2026-08-19T00:00:02Z"
	snapshot.After.ObservedAt = "2026-08-19T00:00:01Z"
	snapshot.Capture.CapturedAt = "2026-08-19T00:00:00Z"
	if result := ValidateSnapshot(snapshot); result.Valid || !hasReason(result, ReasonTemporalOrderInvalid) {
		t.Fatalf("reversed effect timestamps accepted: %+v", result)
	}
}

func TestEffectLoadersAndJUnitFailurePath(t *testing.T) {
	dir := t.TempDir()
	snapshot := validSnapshot(t)
	contract := validContract()
	write := func(name string, value any) string {
		path := filepath.Join(dir, name)
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(path, raw, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		return path
	}
	snapshotPath := write("snapshot.json", snapshot)
	contractPath := write("contract.json", contract)
	if loaded, err := LoadSnapshot(snapshotPath); err != nil || loaded.SnapshotID != snapshot.SnapshotID {
		t.Fatalf("load snapshot: %+v %v", loaded, err)
	}
	if loaded, err := LoadContract(contractPath); err != nil || loaded.ContractID != contract.ContractID {
		t.Fatalf("load contract: %+v %v", loaded, err)
	}
	if _, err := LoadSnapshot(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing snapshot accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadContract(filepath.Join(dir, "bad.json")); err == nil {
		t.Fatal("malformed contract accepted")
	}
	invalidSnapshot := snapshot
	invalidSnapshot.SchemaVersion = "9.9.9"
	invalidSnapshotPath := write("invalid-snapshot.json", invalidSnapshot)
	if _, err := LoadSnapshot(invalidSnapshotPath); err == nil || !strings.Contains(err.Error(), ReasonSchemaUnsupported) {
		t.Fatalf("schema-invalid snapshot accepted: %v", err)
	}
	invalidContract := contract
	invalidContract.SchemaVersion = "9.9.9"
	invalidContractPath := write("invalid-contract.json", invalidContract)
	if _, err := LoadContract(invalidContractPath); err == nil || !strings.Contains(err.Error(), ReasonSchemaUnsupported) {
		t.Fatalf("schema-invalid contract accepted: %v", err)
	}
	validRaw, _ := json.Marshal(snapshot)
	unknownRaw := strings.Replace(string(validRaw), `"schema_id":"`+SnapshotSchemaID+`"`, `"unknown":true,"schema_id":"`+SnapshotSchemaID+`"`, 1)
	duplicateRaw := strings.Replace(string(validRaw), `"schema_id":"`+SnapshotSchemaID+`"`, `"schema_id":"`+SnapshotSchemaID+`","schema_id":"`+SnapshotSchemaID+`"`, 1)
	if err := os.WriteFile(filepath.Join(dir, "unknown.json"), []byte(unknownRaw), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "duplicate.json"), []byte(duplicateRaw), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSnapshot(filepath.Join(dir, "unknown.json")); err == nil {
		t.Fatal("unknown snapshot field accepted")
	}
	if _, err := LoadSnapshot(filepath.Join(dir, "duplicate.json")); err == nil {
		t.Fatal("duplicate snapshot field accepted")
	}
	link := filepath.Join(dir, "snapshot-link.json")
	if err := os.Symlink(snapshotPath, link); err == nil {
		if _, err := LoadSnapshot(link); err == nil {
			t.Fatal("symlink snapshot path accepted")
		}
	}
	snapshot.Completeness = CompletenessPartial
	refreshSnapshot(t, &snapshot)
	result := testGrade(snapshot, contract)
	junitPath := filepath.Join(dir, "failure.xml")
	if err := WriteJUnit(junitPath, result); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(junitPath)
	if !strings.Contains(string(raw), `failures="1"`) {
		t.Fatalf("inconclusive JUnit did not fail closed: %s", raw)
	}
}

func TestEffectInputIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(path, make([]byte, maxEffectInputBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSelectedFile(path); err == nil {
		t.Fatal("oversized effect input accepted")
	}
}

func TestEffectSchemasValidateRepresentativeObjects(t *testing.T) {
	root := filepath.Join("..", "..", "schemas", "v1", "effects")
	snapshot := validSnapshot(t)
	contract := validContract()
	for _, test := range []struct {
		name, file string
		value      any
	}{
		{"snapshot", "effect_snapshot.schema.json", snapshot}, {"contract", "effect_contract.schema.json", contract}, {"grade", "effect-grading-result.schema.json", testGrade(snapshot, contract)},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, test.file)
			raw, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if err := proofschema.ValidateJSON(path, raw); err != nil {
				t.Fatalf("schema validation: %v", err)
			}
		})
	}
}

func TestEmbeddedEffectSchemasMatchCheckedInSchemas(t *testing.T) {
	for _, name := range []string{"effect_snapshot.schema.json", "effect_contract.schema.json", "effect-grading-result.schema.json"} {
		embedded, err := schemaAssets.ReadFile("schemaassets/" + name)
		if err != nil {
			t.Fatal(err)
		}
		checkedIn, err := os.ReadFile(filepath.Join("..", "..", "schemas", "v1", "effects", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(embedded) != string(checkedIn) {
			t.Fatalf("embedded effects schema drift: %s", name)
		}
	}
}

func TestWriteJUnitIsStableAndReportsFailures(t *testing.T) {
	result := testGrade(validSnapshot(t), validContract())
	first := filepath.Join(t.TempDir(), "first.xml")
	second := filepath.Join(t.TempDir(), "second.xml")
	if err := WriteJUnit(first, result); err != nil {
		t.Fatal(err)
	}
	if err := WriteJUnit(second, result); err != nil {
		t.Fatal(err)
	}
	if err := WriteJUnit(first, result); err == nil {
		t.Fatal("JUnit writer overwrote an existing output")
	}
	left, _ := os.ReadFile(first)
	right, _ := os.ReadFile(second)
	if string(left) != string(right) || !strings.Contains(string(left), "testsuite") {
		t.Fatalf("JUnit output is not stable: %s", left)
	}
}

func hasReason(result ValidationResult, reason string) bool {
	for _, got := range result.ReasonCodes {
		if got == reason {
			return true
		}
	}
	return false
}
func hasGradeReason(result GradeResult, reason string) bool {
	for _, got := range result.ReasonCodes {
		if got == reason {
			return true
		}
	}
	return false
}

func refreshSnapshot(t *testing.T, snapshot *Snapshot) {
	t.Helper()
	signed, err := snapshot.Sign(testPrivateKey(), snapshot.Provenance.Mode)
	if err != nil {
		t.Fatal(err)
	}
	*snapshot = signed
}

func int64ptr(value int64) *int64 { return &value }

func testPrivateKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("gait-effects-test-collector-key-v1"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func testGrade(snapshot Snapshot, contract Contract) GradeResult {
	return GradeWithOptions(snapshot, contract, GradeOptions{TrustedCollectorPublicKey: testPrivateKey().Public().(ed25519.PublicKey), ExpectedCorrelation: &CorrelationExpectation{ActionDigest: snapshot.Correlation.ActionDigest, ActivationDigest: snapshot.Correlation.ActivationDigest, ProofDigest: snapshot.Correlation.ProofDigest}})
}
