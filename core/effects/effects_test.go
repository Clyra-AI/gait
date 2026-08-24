package effects

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	proofschema "github.com/Clyra-AI/proof/schema"
)

func TestLoadPublicKeyAndNumericConversions(t *testing.T) {
	seed := sha256.Sum256([]byte("effects-public-key-coverage"))
	want := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	root := t.TempDir()
	keyPath := filepath.Join(root, "collector.pub")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(want)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPublicKey(keyPath)
	if err != nil || !got.Equal(want) {
		t.Fatalf("load public key: got=%v err=%v", got, err)
	}
	badPath := filepath.Join(root, "bad.pub")
	if err := os.WriteFile(badPath, []byte("not-base64\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPublicKey(badPath); err == nil {
		t.Fatal("invalid public key accepted")
	}
	if _, err := LoadPublicKey(filepath.Join(root, "missing.pub")); err == nil {
		t.Fatal("missing public key accepted")
	}
	if _, err := LoadPublicKey(""); err == nil {
		t.Fatal("empty public key path accepted")
	}
	if _, err := LoadPublicKey(root); err == nil {
		t.Fatal("public key directory accepted")
	}

	for _, test := range []struct {
		value any
		ok    bool
	}{
		{value: int64(2), ok: true}, {value: int(2), ok: true},
		{value: float64(2), ok: true}, {value: math.NaN(), ok: false},
		{value: json.Number("2.5"), ok: true}, {value: json.Number("bad"), ok: false},
		{value: "2", ok: false},
	} {
		_, ok := numberValue(test.value)
		if ok != test.ok {
			t.Fatalf("numberValue(%v) ok=%t want=%t", test.value, ok, test.ok)
		}
	}

	snapshot := validSnapshot(t)
	if err := snapshot.VerifyProvenance(); err != nil {
		t.Fatalf("self-contained provenance verification failed: %v", err)
	}
	badProvenance := snapshot
	badProvenance.Provenance.PublicKey = "not-base64"
	if err := badProvenance.VerifyProvenance(); err == nil {
		t.Fatal("malformed embedded provenance key accepted")
	}
	if _, err := snapshot.Sign(nil, "collector_signed"); err == nil {
		t.Fatal("invalid private key accepted")
	}
	if _, err := snapshot.Sign(testPrivateKey(), "unknown_mode"); err == nil {
		t.Fatal("unknown provenance mode accepted")
	}
}

func TestEffectMetadataPredicatesAndJUnitErrors(t *testing.T) {
	snapshot := validSnapshot(t)
	contract := Contract{SchemaID: ContractSchemaID, SchemaVersion: SchemaVersion, ContractID: "contract:metadata", Name: "metadata", Predicates: []Predicate{
		{ID: "resource", Kind: PredicateExpect, Field: "resource_kind", Operator: "equals", Expected: ResourcePostgres},
		{ID: "complete", Kind: PredicateExpect, Field: "completeness", Operator: "equals", Expected: CompletenessComplete},
		{ID: "verified", Kind: PredicateExpect, Field: "enforcement", Operator: "equals", Expected: EnforcementVerified},
		{ID: "selector-resource", Kind: PredicateExpect, Field: "selector.resource", Operator: "equals", Expected: "postgres.table"},
		{ID: "selector-scope", Kind: PredicateExpect, Field: "selector.scope", Operator: "equals", Expected: "public"},
		{ID: "selector-name", Kind: PredicateExpect, Field: "selector.name", Operator: "equals", Expected: "orders"},
		{ID: "digest", Kind: PredicateExpect, Field: "after.digest", Operator: "equals", Expected: testDigest},
		{ID: "ttl", Kind: PredicateExpect, Field: "after.ttl_seconds", Operator: "equals", Expected: int64(3600)},
	}}
	if result := testGrade(snapshot, contract); result.Status != GradePass {
		t.Fatalf("metadata predicates did not pass: %+v", result)
	}

	result := testGrade(snapshot, validContract())
	if err := WriteJUnit("", result); err == nil {
		t.Fatal("empty JUnit output path accepted")
	}
	missingParent := filepath.Join(t.TempDir(), "missing", "result.xml")
	if err := WriteJUnit(missingParent, result); err == nil {
		t.Fatal("missing JUnit parent directory accepted")
	}
	for name, emptyResult := range map[string]GradeResult{
		"empty-pass": {Status: GradePass},
		"empty-fail": {Status: GradeInconclusive, ReasonCodes: []string{ReasonEvidenceIncomplete}},
	} {
		path := filepath.Join(t.TempDir(), name+".xml")
		if err := WriteJUnit(path, emptyResult); err != nil {
			t.Fatalf("write empty-evaluation JUnit %s: %v", name, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if name == "empty-fail" && !strings.Contains(string(raw), "failure") {
			t.Fatalf("fail-closed empty JUnit missing failure: %s", raw)
		}
	}
	root := t.TempDir()
	target := filepath.Join(root, "target.xml")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.xml")
	if err := os.Symlink(target, link); err == nil {
		if err := WriteJUnit(link, result); err == nil {
			t.Fatal("symlink JUnit output accepted")
		}
	}
}

func TestEffectProvenanceAndSchemaErrorBranches(t *testing.T) {
	snapshot := validSnapshot(t)
	if err := snapshot.VerifyProvenanceAgainst(nil); err == nil {
		t.Fatal("missing trusted provenance key accepted")
	}
	otherSeed := sha256.Sum256([]byte("other-effects-key"))
	otherKey := ed25519.NewKeyFromSeed(otherSeed[:]).Public().(ed25519.PublicKey)
	if err := snapshot.VerifyProvenanceAgainst(otherKey); err == nil {
		t.Fatal("mismatched trusted provenance key accepted")
	}
	tampered := snapshot
	tampered.Provenance.Signature.SignedDigest = strings.Repeat("0", 64)
	if err := tampered.VerifyProvenanceAgainst(testPrivateKey().Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("mismatched provenance digest accepted")
	}
	tampered = snapshot
	tampered.Provenance.Signature.KeyID = strings.Repeat("0", 64)
	if err := tampered.VerifyProvenance(); err == nil {
		t.Fatal("mismatched embedded provenance key id accepted")
	}
	if err := validateEmbeddedSchema([]byte("{"), SnapshotSchemaID); err == nil {
		t.Fatal("malformed embedded-schema document accepted")
	}
	if err := validateEmbeddedSchema([]byte("{}"), "https://gait.dev/schemas/v1/effects/missing.schema.json"); err == nil {
		t.Fatal("missing embedded schema accepted")
	}

	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "trailing.json")
	if err := os.WriteFile(path, append(raw, []byte("\n{}")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSnapshot(path); err == nil {
		t.Fatal("trailing snapshot JSON accepted")
	}
}

func TestObservationValidationBranches(t *testing.T) {
	negative := int64(-1)
	for _, observation := range []Observation{
		{State: "invalid", ObservedAt: "2026-08-19T00:00:00Z"},
		{State: ObservationPresent, Digest: "bad", ObservedAt: "2026-08-19T00:00:00Z"},
		{State: ObservationPresent, TTLSeconds: &negative, ObservedAt: "2026-08-19T00:00:00Z"},
		{State: ObservationPresent, ObservedAt: ""},
		{State: ObservationPresent, ObservedAt: "not-a-time"},
	} {
		reasons := []string{}
		validateObservation(&reasons, observation)
		if len(reasons) == 0 {
			t.Fatalf("invalid observation produced no reasons: %+v", observation)
		}
	}
	if matched, supported := matches("value", "contains", 3); matched || supported {
		t.Fatal("non-string contains operand was supported")
	}
	if matched, supported := matches("value", "unsupported", "value"); matched || supported {
		t.Fatal("unknown effect predicate operator was supported")
	}
}

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

func TestValidateSnapshotRejectsMalformedBoundaryFields(t *testing.T) {
	negative := int64(-1)
	for _, test := range []struct {
		name   string
		reason string
		edit   func(*Snapshot)
	}{
		{name: "whitespace", reason: ReasonWhitespaceInvalid, edit: func(s *Snapshot) { s.SnapshotID = " effect-snapshot:demo " }},
		{name: "resource kind", reason: ReasonResourceKindInvalid, edit: func(s *Snapshot) { s.ResourceKind = "unknown-kind" }},
		{name: "selector", reason: ReasonSelectorMissing, edit: func(s *Snapshot) { s.Selector.Resource = "" }},
		{name: "correlation", reason: ReasonEvidenceMissing, edit: func(s *Snapshot) { s.Correlation = Correlation{} }},
		{name: "correlation digest", reason: ReasonDigestInvalid, edit: func(s *Snapshot) { s.Correlation.ActionDigest = "bad" }},
		{name: "observation", reason: ReasonObservationInvalid, edit: func(s *Snapshot) { s.After.Count = &negative }},
		{name: "collector", reason: ReasonCollectorMissing, edit: func(s *Snapshot) { s.Collector.Name = "" }},
		{name: "capture", reason: ReasonCaptureInvalid, edit: func(s *Snapshot) { s.Capture.Mode = "invalid" }},
		{name: "completeness", reason: ReasonCompletenessInvalid, edit: func(s *Snapshot) { s.Completeness = "invalid" }},
		{name: "enforcement", reason: ReasonEnforcementInvalid, edit: func(s *Snapshot) { s.Enforcement = "invalid" }},
		{name: "evidence refs", reason: ReasonEvidenceMissing, edit: func(s *Snapshot) { s.EvidenceRefs = nil }},
		{name: "provenance", reason: ReasonProvenanceMissing, edit: func(s *Snapshot) { s.Provenance.Mode = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := validSnapshot(t)
			test.edit(&snapshot)
			result := ValidateSnapshot(snapshot)
			if result.Valid || !hasReason(result, test.reason) {
				t.Fatalf("malformed snapshot missing %s: %+v", test.reason, result)
			}
		})
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

func TestEffectContractValidationRejectsMalformedBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		reason string
		edit   func(*Contract)
	}{
		{name: "schema", reason: ReasonSchemaUnsupported, edit: func(c *Contract) { c.SchemaVersion = "9" }},
		{name: "contract id", reason: ReasonContractIDMissing, edit: func(c *Contract) { c.ContractID = "" }},
		{name: "whitespace", reason: ReasonWhitespaceInvalid, edit: func(c *Contract) { c.Name = " orders " }},
		{name: "predicates missing", reason: ReasonPredicateInvalid, edit: func(c *Contract) { c.Predicates = nil }},
		{name: "predicate id", reason: ReasonPredicateInvalid, edit: func(c *Contract) { c.Predicates[0].ID = "" }},
		{name: "field", reason: ReasonPredicateInvalid, edit: func(c *Contract) { c.Predicates[1].Field = "" }},
		{name: "operator missing", reason: ReasonPredicateInvalid, edit: func(c *Contract) { c.Predicates[1].Operator = "" }},
		{name: "operator unsupported", reason: ReasonPredicateInvalid, edit: func(c *Contract) { c.Predicates[1].Operator = "execute" }},
		{name: "invariant operator", reason: ReasonPredicateInvalid, edit: func(c *Contract) { c.Predicates[0].Operator = "equals" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			contract := validContract()
			test.edit(&contract)
			result := ValidateContract(contract)
			if result.Valid || !hasReason(result, test.reason) {
				t.Fatalf("malformed contract missing %s: %+v", test.reason, result)
			}
		})
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
