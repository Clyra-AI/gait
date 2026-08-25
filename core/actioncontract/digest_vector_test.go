package actioncontract

import (
	"crypto/ed25519"
	"crypto/sha256"
	proof "github.com/Clyra-AI/proof"
	"strings"
	"testing"
)

func TestSignedAdvisoryAndReceiptDigestVectors(t *testing.T) {
	s := sha256.Sum256([]byte("gait-digest-vector-key"))
	k := ed25519.NewKeyFromSeed(s[:])
	p := k.Public().(ed25519.PublicKey)
	r, e := (OfflineAdvisoryEvaluator{}).Evaluate(AdvisoryInput{ActionID: "a"})
	if e != nil {
		t.Fatal(e)
	}
	r, e = r.Sign(k)
	if e != nil || !strings.HasPrefix(r.CanonicalContentDigest, "sha256:") {
		t.Fatal(e)
	}
	if e = VerifyAdvisoryReport(r, p, "", ""); e != nil {
		t.Fatal(e)
	}
	cr := proof.RelationshipRef{Kind: "action_contract", ID: "c", Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", SchemaID: "schema", SchemaVersion: "1", SourceProduct: "gait"}
	x, e := (LifecycleReceipt{ContractFamilyID: "f", ContractID: "c", Revision: 1, ArtifactDigests: []string{cr.Digest}, ArtifactRefs: []proof.RelationshipRef{cr}, Correlation: proof.ControlContainmentTelemetryProfile{ProfileVersion: "1.0", BindingMode: proof.BindingModeDigestBound, ContractRef: &cr, ContentDigest: cr.Digest}, Authority: "non_authoritative", Quarantine: true, Redaction: "reference_only", Outcome: "unknown", ObservedAt: "2026-07-20T00:00:00Z", FreshUntil: "2026-07-20T01:00:00Z"}).Sign(k)
	if e != nil {
		t.Fatal(e)
	}
	if e = VerifyLifecycleReceipt(x, p); e != nil {
		t.Fatal(e)
	}
}
