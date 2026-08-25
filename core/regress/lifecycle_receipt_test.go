package regress

import (
	"crypto/ed25519"
	"crypto/sha256"
	action "github.com/Clyra-AI/gait/core/actioncontract"
	proof "github.com/Clyra-AI/proof"
	"testing"
)

func TestMaterializeLifecycleReceiptDeterministic(t *testing.T) {
	s := sha256.Sum256([]byte("receipt-test"))
	k := ed25519.NewKeyFromSeed(s[:])
	cr := proof.RelationshipRef{Kind: "action_contract", ID: "c", Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", SchemaID: "schema", SchemaVersion: "1", SourceProduct: "gait"}
	r, e := (action.LifecycleReceipt{ContractFamilyID: "f", ContractID: "c", Revision: 1, ArtifactDigests: []string{cr.Digest}, ArtifactRefs: []proof.RelationshipRef{cr}, Correlation: proof.ControlContainmentTelemetryProfile{ProfileVersion: "1.0", BindingMode: proof.BindingModeDigestBound, ContractRef: &cr, ContentDigest: cr.Digest}, Authority: "non_authoritative", Quarantine: true, Redaction: "reference_only", Outcome: "out_of_scope", ObservedAt: "2026-07-20T00:00:00Z", FreshUntil: "2026-07-20T01:00:00Z"}).Sign(k)
	if e != nil {
		t.Fatal(e)
	}
	a, e := MaterializeLifecycleReceipt(r, LifecycleReceiptMaterializeOptions{SigningPrivateKey: k})
	if e != nil {
		t.Fatal(e)
	}
	b, e := MaterializeLifecycleReceipt(r, LifecycleReceiptMaterializeOptions{SigningPrivateKey: k})
	if e != nil || string(a.ZipBytes) != string(b.ZipBytes) {
		t.Fatalf("nondeterministic materialization: %v", e)
	}
}
