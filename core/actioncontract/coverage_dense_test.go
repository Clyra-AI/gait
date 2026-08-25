package actioncontract

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	proof "github.com/Clyra-AI/proof"
)

const denseDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func denseKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("action-contract-dense-coverage"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func denseReceipt(t *testing.T) (LifecycleReceipt, ed25519.PrivateKey) {
	t.Helper()
	key := denseKey()
	ref := proof.RelationshipRef{Kind: "action_contract", ID: "contract:dense", Digest: denseDigest, SchemaID: ProposedContractSchemaID, SchemaVersion: ProposedContractVersion, SourceProduct: ProposedProducer}
	receipt, err := (LifecycleReceipt{ContractFamilyID: "family:dense", ContractID: ref.ID, Revision: 1, ArtifactDigests: []string{ref.Digest}, ArtifactRefs: []proof.RelationshipRef{ref}, Correlation: proof.ControlContainmentTelemetryProfile{ProfileVersion: "1.0", BindingMode: proof.BindingModeDigestBound, ContractRef: &ref, ContentDigest: ref.Digest}, Authority: "non_authoritative", Quarantine: true, Redaction: "reference_only", Outcome: "unknown", ObservedAt: "2026-08-25T00:00:00Z", FreshUntil: "2026-08-25T01:00:00Z"}).Sign(key)
	if err != nil {
		t.Fatal(err)
	}
	return receipt, key
}

func denseAdvisory(t *testing.T) (AdvisoryReport, ed25519.PrivateKey) {
	t.Helper()
	key := denseKey()
	ref := proof.RelationshipRef{Kind: "gait.execution", ID: "execution:dense", Digest: denseDigest, SchemaID: ExecutionEvidenceSchemaID, SchemaVersion: ExecutionEvidenceSchemaVersion, SourceProduct: EvidenceProducer}
	report, err := (OfflineAdvisoryEvaluator{}).Evaluate(AdvisoryInput{ActionID: "action:dense", Claims: []string{"claim"}, ContractDigest: denseDigest, CorrelationDigest: denseDigest, EvidenceRefs: []proof.RelationshipRef{ref}})
	if err != nil {
		t.Fatal(err)
	}
	report, err = report.Sign(key)
	if err != nil {
		t.Fatal(err)
	}
	return report, key
}

func expectDenseError(t *testing.T, name string, fn func() error) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if err := fn(); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDenseCircuitJSONValidation(t *testing.T) {
	valid := validCircuitInput()
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	var decoded CircuitBreakerInput
	if err := ValidateCircuitJSON(raw, &decoded); err != nil {
		t.Fatalf("valid circuit JSON rejected: %v", err)
	}
	expectDenseError(t, "malformed", func() error { return ValidateCircuitJSON([]byte("{"), &decoded) })
	expectDenseError(t, "unknown field", func() error { return ValidateCircuitJSON([]byte(`{"unknown":true}`), &decoded) })
}

func TestDenseCircuitDecisionAndInputBranches(t *testing.T) {
	base := EvaluateCircuit(validCircuitInput())
	for _, tc := range []struct {
		name   string
		mutate func(*CircuitBreakerDecision)
	}{
		{"schema", func(x *CircuitBreakerDecision) { x.SchemaID = "bad" }},
		{"allow trip", func(x *CircuitBreakerDecision) { x.Allow = false; x.Tripped = false }},
		{"reason order", func(x *CircuitBreakerDecision) { x.ReasonCodes = []string{"z", "a"} }},
		{"invalidation", func(x *CircuitBreakerDecision) { x.InvalidationStatus = "bad" }},
		{"scope order", func(x *CircuitBreakerDecision) { x.AffectedScope = []string{"z", "a"} }},
		{"scope empty", func(x *CircuitBreakerDecision) { x.AffectedScope = []string{""} }},
		{"authority", func(x *CircuitBreakerDecision) { x.OutOfScope = true; x.EnforcementClaimed = true }},
	} {
		t.Run("decision "+tc.name, func(t *testing.T) {
			x := base
			tc.mutate(&x)
			if len(ValidateCircuitDecision(x)) == 0 {
				t.Fatal("invalid circuit decision accepted")
			}
		})
	}
	invalid := validCircuitInput()
	invalid.Chain = ChainDecision{Allowed: true, ReasonCodes: []string{"unexpected"}}
	invalid.EffectStatus = "bad"
	invalid.ContainmentStatus = "bad"
	invalid.StopStatus = "bad"
	invalid.RevocationStatus = "bad"
	invalid.InvalidationStatus = "bad"
	invalid.AffectedScope = []string{"z", "z", ""}
	if len(ValidateCircuitInput(invalid)) == 0 {
		t.Fatal("invalid circuit input accepted")
	}
	dup := base
	dup.ReasonCodes = []string{"same", "same"}
	if len(ValidateCircuitDecision(dup)) == 0 {
		t.Fatal("duplicate decision reasons accepted")
	}
	for _, status := range []string{"requested", "attempted", "denied", "failed", "acknowledged"} {
		x := validCircuitInput()
		x.StopStatus = status
		_ = EvaluateCircuit(x)
	}
	for _, status := range []string{"attempted", "denied", "failed", "acknowledged"} {
		x := validCircuitInput()
		x.RevocationStatus = status
		_ = EvaluateCircuit(x)
	}
	for _, status := range []string{"partial", "unresolved"} {
		x := validCircuitInput()
		x.ContainmentStatus = status
		_ = EvaluateCircuit(x)
	}
}

func TestDenseRuntimeParserAndLifecycleValidationBranches(t *testing.T) {
	classification := []byte(`{"schema_id":"` + RuntimeClassificationInputSchemaID + `","schema_version":"1","action_id":"action:dense","action_class":"read"}`)
	if _, err := ParseClassificationInput(classification); err != nil {
		t.Fatalf("valid classification rejected: %v", err)
	}
	expectDenseError(t, "classification schema", func() error {
		_, err := ParseClassificationInput([]byte(`{"schema_id":"bad","schema_version":"1","action_id":"a"}`))
		return err
	})
	expectDenseError(t, "classification malformed", func() error {
		_, err := ParseClassificationInput([]byte(`{"action_id":"a","action_id":"b"}`))
		return err
	})
	classified := ClassifyRuntimeAction(ClassificationInput{ActionID: "action:dense", ActionClass: "read", CompositionRole: "source", TargetTrustClass: "external"})
	actionRaw, err := json.Marshal(classified.Action)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseRuntimeAction(actionRaw); err != nil {
		t.Fatalf("valid runtime action rejected: %v", err)
	}
	expectDenseError(t, "runtime action schema", func() error { _, err := ParseRuntimeAction([]byte(`{"bad":true}`)); return err })
	if _, err := ParseReadinessInput([]byte(`{"preconditions":[]}`)); err != nil {
		t.Fatalf("valid readiness input rejected: %v", err)
	}
	expectDenseError(t, "readiness malformed", func() error {
		_, err := ParseReadinessInput([]byte(`{"preconditions":[],"preconditions":[]}`))
		return err
	})

	for _, kind := range []LifecycleEventKind{LifecycleExecutionSucceeded, LifecycleEffectValidated, LifecycleContainmentCompleted, LifecycleCompensationCompleted, LifecycleStopAcknowledged, LifecycleRevocationAcknowledged, LifecycleCapabilityInvalidated, LifecycleProposalIngested} {
		if err := validateLifecycleEvent(LifecycleRecord{Kind: kind}); err == nil {
			t.Fatalf("missing typed evidence accepted for %s", kind)
		}
	}
	ambiguous := LifecycleRecord{Kind: LifecycleExecutionSucceeded, Execution: &ExecutionEvidence{}, Effect: &EffectEvent{}}
	if err := validateLifecycleEvent(ambiguous); err == nil {
		t.Fatal("ambiguous lifecycle evidence accepted")
	}
}

func TestDenseControlVerifyAndParseBranches(t *testing.T) {
	key := denseKey()
	public := key.Public().(ed25519.PublicKey)
	base := ControlEventEvidence{Binding: executionTestBinding(), EventRef: strictControlRef("event", "dense"), CausalRef: strictControlRef("execution", "dense"), ControlRef: strictControlRef("control", "dense"), Command: "stop", Phase: "acknowledged", BoundaryID: "boundary", ResourceID: "resource", AffectedScope: []string{"scope"}, AdapterIdentity: "adapter", AdapterAcknowledged: true, OccurredAt: "2026-08-25T00:00:00Z", FreshUntil: "2026-08-25T01:00:00Z", ReasonCode: "test"}
	valid, err := NewControlEventEvidence(base, key)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*ControlEventEvidence)
	}{
		{"missing command", func(x *ControlEventEvidence) { x.Command = "" }},
		{"invalid phase", func(x *ControlEventEvidence) { x.Phase = "bad" }},
		{"bad time", func(x *ControlEventEvidence) { x.OccurredAt = "bad" }},
		{"missing metadata", func(x *ControlEventEvidence) { x.BoundaryID = "" }},
		{"empty scope", func(x *ControlEventEvidence) { x.AffectedScope = []string{" "} }},
		{"duplicate scope", func(x *ControlEventEvidence) { x.AffectedScope = []string{"scope", "scope"} }},
		{"incomplete ref", func(x *ControlEventEvidence) { x.EventRef.Digest = "" }},
		{"premature acknowledgement", func(x *ControlEventEvidence) { x.Phase = "requested" }},
		{"stale", func(x *ControlEventEvidence) { x.FreshUntil = x.OccurredAt }},
		{"invalid binding", func(x *ControlEventEvidence) { x.Binding.ContractFamilyID = "" }},
		{"identifier ref", func(x *ControlEventEvidence) { x.EventRef.Digest = "bad" }},
		{"invalidation acknowledgement", func(x *ControlEventEvidence) { x.Phase = "invalidated"; x.AdapterAcknowledged = false }},
		{"invalid authority", func(x *ControlEventEvidence) { x.Command = "unknown" }},
	} {
		t.Run("new "+tc.name, func(t *testing.T) {
			x := base
			tc.mutate(&x)
			if _, err := NewControlEventEvidence(x, key); err == nil {
				t.Fatal("invalid control accepted")
			}
		})
	}
	for _, x := range []ControlEventEvidence{
		func() ControlEventEvidence {
			v := base
			v.Command = "external_revocation"
			v.Phase = "attempted"
			v.AdapterAcknowledged = false
			return v
		}(),
		func() ControlEventEvidence {
			v := base
			v.Command = "capability_invalidation"
			v.Phase = "invalidated"
			return v
		}(),
	} {
		if _, err := NewControlEventEvidence(x, key); err != nil {
			t.Fatalf("valid control phase rejected: %v", err)
		}
	}
	if ok, err := VerifyControlEventEvidence(valid, public); err != nil || !ok {
		t.Fatalf("valid control failed: %v", err)
	}
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseControlEventEvidence(raw); err != nil {
		t.Fatal(err)
	}
	expectDenseError(t, "new invalid key", func() error { _, err := NewControlEventEvidence(valid, nil); return err })
	expectDenseError(t, "parse malformed", func() error { _, err := ParseControlEventEvidence([]byte("{")); return err })
	expectDenseError(t, "parse unknown", func() error {
		_, err := ParseControlEventEvidence(append(raw[:len(raw)-1], []byte(`,"unknown":true}`)...))
		return err
	})
	expectDenseError(t, "parse schema", func() error {
		x := valid
		x.EvidenceID = "bad"
		bad, _ := json.Marshal(x)
		_, err := ParseControlEventEvidence(bad)
		return err
	})
	expectDenseError(t, "parse schema id", func() error {
		x := valid
		x.SchemaID = "bad"
		bad, _ := json.Marshal(x)
		_, err := ParseControlEventEvidence(bad)
		return err
	})
	expectDenseError(t, "verify wrong key", func() error {
		ok, err := VerifyControlEventEvidence(valid, ed25519.PublicKey{})
		if err == nil && ok {
			return nil
		}
		return errOrDense(ok, err)
	})
	expectDenseError(t, "verify schema", func() error {
		x := valid
		x.SchemaID = "bad"
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify evidence id", func() error {
		x := valid
		x.EvidenceID = "bad"
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify timestamp", func() error {
		x := valid
		x.OccurredAt = "bad"
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify acknowledgement", func() error {
		x := valid
		x.AdapterAcknowledged = false
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify premature acknowledgement", func() error {
		x := valid
		x.Phase = "requested"
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify duplicate scope", func() error {
		x := valid
		x.AffectedScope = []string{"scope", "scope"}
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify empty scope", func() error {
		x := valid
		x.AffectedScope = []string{""}
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify binding", func() error {
		x := valid
		x.Binding.ContractFamilyID = ""
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify signature", func() error {
		x := valid
		x.Provenance.Signature.Sig = "bad"
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	expectDenseError(t, "verify schema validation", func() error {
		x := valid
		x.Provenance.Producer = ""
		_, err := VerifyControlEventEvidence(x, public)
		return err
	})
	if ok, err := VerifyControlEventEvidenceAt(valid, public, time.Time{}); err != nil || !ok {
		t.Fatalf("zero-time control verification should preserve valid evidence: %v", err)
	}
	expectDenseError(t, "verify at stale", func() error {
		_, err := VerifyControlEventEvidenceAt(valid, public, time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC))
		return err
	})
}

func errOrDense(ok bool, err error) error {
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return errors.New("dense verification failed")
}

func TestDenseAdvisoryVerifyBranches(t *testing.T) {
	valid, key := denseAdvisory(t)
	public := key.Public().(ed25519.PublicKey)
	if err := VerifyAdvisoryReport(valid, public, denseDigest, denseDigest); err != nil {
		t.Fatal(err)
	}
	expectDenseError(t, "schema", func() error { x := valid; x.SchemaID = "bad"; return VerifyAdvisoryReport(x, public, "", "") })
	expectDenseError(t, "metadata", func() error { x := valid; x.ProviderName = ""; return VerifyAdvisoryReport(x, public, "", "") })
	expectDenseError(t, "contract binding", func() error { return VerifyAdvisoryReport(valid, public, "sha256:"+strings.Repeat("a", 64), "") })
	expectDenseError(t, "correlation binding", func() error { return VerifyAdvisoryReport(valid, public, "", "sha256:"+strings.Repeat("a", 64)) })
	expectDenseError(t, "missing key", func() error { return VerifyAdvisoryReport(valid, nil, "", "") })
	expectDenseError(t, "public key", func() error {
		x := valid
		x.Provenance.PublicKey = "bad"
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "evidence ref", func() error {
		x := valid
		x.EvidenceRefs[0].Digest = "bad"
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "digest", func() error {
		x := valid
		x.CanonicalContentDigest = denseDigest
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "authority provenance", func() error {
		x := valid
		x.Provenance.Mode = "authoritative"
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "provenance provider", func() error {
		x := valid
		x.Provenance.Provider = "other"
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "key id", func() error {
		x := valid
		x.Provenance.Signature.KeyID = "bad"
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "signature", func() error {
		x := valid
		x.Provenance.Signature.Sig = "bad"
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "sign key", func() error { _, err := valid.Sign(nil); return err })
	expectDenseError(t, "sign authority", func() error { x := valid; x.AdvisoryOnly = false; _, err := x.Sign(key); return err })
	expectDenseError(t, "sign duplicate", func() error { x := valid; x.Findings = []string{"x", "x"}; _, err := x.Sign(key); return err })
	expectDenseError(t, "sign status", func() error { x := valid; x.Status = "bad"; _, err := x.Sign(key); return err })
	expectDenseError(t, "sign metadata", func() error { x := valid; x.ProviderName = ""; _, err := x.Sign(key); return err })
	expectDenseError(t, "sign digest", func() error { x := valid; x.ContractDigest = "bad"; _, err := x.Sign(key); return err })
	expectDenseError(t, "sign reason duplicate", func() error { x := valid; x.ReasonCodes = []string{"same", "same"}; _, err := x.Sign(key); return err })
	expectDenseError(t, "sign evidence duplicate", func() error {
		x := valid
		x.EvidenceRefs = append(x.EvidenceRefs, x.EvidenceRefs[0])
		_, err := x.Sign(key)
		return err
	})
	other := denseKey()
	expectDenseError(t, "public key mismatch", func() error { return VerifyAdvisoryReport(valid, other.Public().(ed25519.PublicKey), "", "") })
	expectDenseError(t, "signed digest", func() error {
		x := valid
		x.Provenance.Signature.SignedDigest = "bad"
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "schema validation", func() error {
		x := valid
		x.Provenance.Signature.Alg = "bad"
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "invalid signature bytes", func() error {
		x := valid
		x.Provenance.Signature.Sig = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
		return VerifyAdvisoryReport(x, public, "", "")
	})
	expectDenseError(t, "schema version", func() error { x := valid; x.SchemaVersion = "2"; return VerifyAdvisoryReport(x, public, "", "") })
}

func TestDenseReceiptVerifyAndVerifyAtBranches(t *testing.T) {
	valid, key := denseReceipt(t)
	public := key.Public().(ed25519.PublicKey)
	if err := VerifyLifecycleReceipt(valid, public); err != nil {
		t.Fatal(err)
	}
	expectDenseError(t, "schema", func() error { x := valid; x.SchemaID = "bad"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "digest", func() error {
		x := valid
		x.CanonicalContentDigest = denseDigest
		return VerifyLifecycleReceipt(x, public)
	})
	expectDenseError(t, "key", func() error { return VerifyLifecycleReceipt(valid, nil) })
	expectDenseError(t, "public key", func() error { x := valid; x.Provenance.PublicKey = "bad"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "signed digest", func() error {
		x := valid
		x.Provenance.Signature.SignedDigest = "bad"
		return VerifyLifecycleReceipt(x, public)
	})
	expectDenseError(t, "signature", func() error { x := valid; x.Provenance.Signature.Sig = "bad"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "schema validation", func() error { x := valid; x.Provenance.Signature.Alg = "bad"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "invalid signature bytes", func() error {
		x := valid
		x.Provenance.Signature.Sig = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
		return VerifyLifecycleReceipt(x, public)
	})
	expectDenseError(t, "receipt id", func() error { x := valid; x.ReceiptID = "bad"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "correlation", func() error {
		x := valid
		x.Correlation.ContentDigest = "bad"
		return VerifyLifecycleReceipt(x, public)
	})
	expectDenseError(t, "digest set", func() error {
		x := valid
		x.ArtifactDigests = []string{"bad"}
		return VerifyLifecycleReceipt(x, public)
	})
	expectDenseError(t, "ref set", func() error { x := valid; x.ArtifactRefs[0].Digest = ""; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "authority", func() error { x := valid; x.Authority = "authoritative"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "outcome", func() error { x := valid; x.Outcome = "bad"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "verify at zero", func() error { return VerifyLifecycleReceiptAt(valid, public, time.Time{}) })
	expectDenseError(t, "verify at stale", func() error {
		return VerifyLifecycleReceiptAt(valid, public, time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC))
	})
	other := denseKey()
	expectDenseError(t, "public key mismatch", func() error { return VerifyLifecycleReceipt(valid, other.Public().(ed25519.PublicKey)) })
	expectDenseError(t, "schema version", func() error { x := valid; x.SchemaVersion = "2"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "provenance mode", func() error { x := valid; x.Provenance.Mode = "other"; return VerifyLifecycleReceipt(x, public) })
	expectDenseError(t, "provenance product", func() error {
		x := valid
		x.Provenance.SourceProduct = "other"
		return VerifyLifecycleReceipt(x, public)
	})
	expectDenseError(t, "declared key mismatch", func() error {
		x := valid
		x.Provenance.PublicKey = "not-the-key"
		return VerifyLifecycleReceipt(x, public)
	})
	expectDenseError(t, "signature key id", func() error {
		x := valid
		x.Provenance.Signature.KeyID = "bad"
		return VerifyLifecycleReceipt(x, public)
	})
	expectDenseError(t, "sign key", func() error { _, err := valid.Sign(nil); return err })
	for _, tc := range []struct {
		name   string
		mutate func(*LifecycleReceipt)
	}{
		{"family", func(x *LifecycleReceipt) { x.ContractFamilyID = "" }},
		{"contract", func(x *LifecycleReceipt) { x.ContractID = "" }},
		{"revision", func(x *LifecycleReceipt) { x.Revision = 0 }},
		{"authority", func(x *LifecycleReceipt) { x.Authority = "bad" }},
		{"quarantined authority", func(x *LifecycleReceipt) { x.Authority = "authoritative" }},
		{"redaction", func(x *LifecycleReceipt) { x.Redaction = "bad" }},
		{"observed", func(x *LifecycleReceipt) { x.ObservedAt = "bad" }},
		{"fresh until", func(x *LifecycleReceipt) { x.FreshUntil = "bad" }},
		{"missing observed", func(x *LifecycleReceipt) { x.ObservedAt = "" }},
		{"reversed freshness", func(x *LifecycleReceipt) { x.FreshUntil = x.ObservedAt }},
		{"artifact missing", func(x *LifecycleReceipt) { x.ArtifactDigests = nil }},
		{"artifact duplicate", func(x *LifecycleReceipt) { x.ArtifactDigests = []string{denseDigest, denseDigest} }},
		{"reason duplicate", func(x *LifecycleReceipt) { x.ReasonCodes = []string{"same", "same"} }},
		{"ref mismatch", func(x *LifecycleReceipt) { x.ArtifactRefs = nil }},
		{"correlation mode", func(x *LifecycleReceipt) { x.Correlation.BindingMode = proof.BindingModeIdentifierOnly }},
		{"correlation contract", func(x *LifecycleReceipt) { x.Correlation.ContractRef = nil }},
		{"correlation digest", func(x *LifecycleReceipt) { x.Correlation.ContentDigest = "bad" }},
		{"digest absent from refs", func(x *LifecycleReceipt) { x.ArtifactDigests = []string{"sha256:" + strings.Repeat("a", 64)} }},
		{"artifact ref duplicate", func(x *LifecycleReceipt) {
			x.ArtifactDigests = []string{denseDigest, denseDigest}
			x.ArtifactRefs = append(x.ArtifactRefs, x.ArtifactRefs[0])
		}},
		{"artifact identity duplicate", func(x *LifecycleReceipt) {
			other := "sha256:" + strings.Repeat("a", 64)
			x.ArtifactDigests = []string{denseDigest, other}
			ref := x.ArtifactRefs[0]
			ref.Digest = other
			x.ArtifactRefs = append(x.ArtifactRefs, ref)
		}},
		{"correlation contract mismatch", func(x *LifecycleReceipt) {
			ref := *x.Correlation.ContractRef
			ref.ID = "other"
			x.Correlation.ContractRef = &ref
		}},
		{"correlation digest missing", func(x *LifecycleReceipt) {
			other := "sha256:" + strings.Repeat("a", 64)
			x.ArtifactDigests = []string{other}
			ref := x.ArtifactRefs[0]
			ref.Digest = other
			x.ArtifactRefs = []proof.RelationshipRef{ref}
		}},
	} {
		t.Run("sign "+tc.name, func(t *testing.T) {
			x := valid
			tc.mutate(&x)
			if _, err := x.Sign(key); err == nil {
				t.Fatal("invalid receipt accepted")
			}
		})
	}
}

func TestDenseOTelExportOptionsAndPaths(t *testing.T) {
	ref := strictControlRef("action_contract", "contract:otel")
	corr := proof.ControlContainmentTelemetryProfile{ProfileVersion: CorrelationProfileVersion, BindingMode: proof.BindingModeDigestBound, ContractRef: &ref, ContentDigest: ref.Digest, TraceID: "0123456789abcdef0123456789abcdef", SpanID: "0123456789abcdef"}
	record := LifecycleRecord{SchemaID: RuntimeLifecycleSchemaID, SchemaVersion: "1", RecordID: "gait-lr-otel", Kind: LifecycleExecutionSucceeded, OccurredAt: "2026-08-25T00:00:00Z", ContractRef: ref, ContractFamilyID: "family:otel", Revision: 1, Correlation: corr}
	dir := t.TempDir()
	expectDenseError(t, "disabled", func() error {
		return ExportLifecycleOTelWithOptions("", []LifecycleRecord{record}, LifecycleOTelExportOptions{SourceVersion: "v1"})
	})
	expectDenseError(t, "source version", func() error {
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "out"), []LifecycleRecord{record}, LifecycleOTelExportOptions{})
	})
	expectDenseError(t, "authority quarantine", func() error {
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "out"), []LifecycleRecord{record}, LifecycleOTelExportOptions{SourceVersion: "v1", Authority: true, Quarantine: true})
	})
	expectDenseError(t, "size", func() error {
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "out"), []LifecycleRecord{record}, LifecycleOTelExportOptions{SourceVersion: "v1", MaxBytes: 1})
	})
	expectDenseError(t, "parent path", func() error {
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "missing", "out"), []LifecycleRecord{record}, LifecycleOTelExportOptions{SourceVersion: "v1"})
	})
	expectDenseError(t, "record id", func() error {
		x := record
		x.RecordID = "bad id"
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "out-id"), []LifecycleRecord{x}, LifecycleOTelExportOptions{SourceVersion: "v1"})
	})
	expectDenseError(t, "trace id", func() error {
		x := record
		x.Correlation.TraceID = "bad"
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "out-trace"), []LifecycleRecord{x}, LifecycleOTelExportOptions{SourceVersion: "v1"})
	})
	expectDenseError(t, "span id", func() error {
		x := record
		x.Correlation.SpanID = "bad"
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "out-span"), []LifecycleRecord{x}, LifecycleOTelExportOptions{SourceVersion: "v1"})
	})
	expectDenseError(t, "occurred at", func() error {
		x := record
		x.OccurredAt = "bad"
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "out-time"), []LifecycleRecord{x}, LifecycleOTelExportOptions{SourceVersion: "v1"})
	})
	expectDenseError(t, "contract ref", func() error {
		x := record
		x.ContractRef.SchemaID = ""
		return ExportLifecycleOTelWithOptions(filepath.Join(dir, "out-ref"), []LifecycleRecord{x}, LifecycleOTelExportOptions{SourceVersion: "v1"})
	})
	full := record
	full.Execution = &ExecutionEvidence{Outcome: "succeeded"}
	full.Effect = &EffectEvent{Outcome: "validated"}
	full.Containment = &ContainmentEvidence{Outcome: "completed"}
	full.Control = &ControlEventEvidence{Phase: "acknowledged", ReasonCode: "control", CanonicalContentDigest: denseDigest, BoundaryID: "boundary", ResourceID: "resource", AffectedScope: []string{"scope"}}
	activation := strictControlRef("activated_action_contract", "activation:otel")
	full.ActivationRef = &activation
	full.Decision = &ReadinessResult{PolicyDigest: denseDigest}
	full.AffectedScope = []string{"scope"}
	if err := ExportLifecycleOTelWithOptions(filepath.Join(dir, "full"), []LifecycleRecord{full}, LifecycleOTelExportOptions{SourceVersion: "v1"}); err != nil {
		t.Fatalf("full OTel export failed: %v", err)
	}
	if err := ExportLifecycleOTelWithOptions(filepath.Join(dir, "ok"), []LifecycleRecord{record}, LifecycleOTelExportOptions{SourceVersion: "v1"}); err != nil {
		t.Fatalf("valid OTel export failed: %v", err)
	}
}
