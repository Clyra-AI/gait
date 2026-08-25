package regress

import (
	"crypto/ed25519"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	action "github.com/Clyra-AI/gait/core/actioncontract"
	"github.com/Clyra-AI/gait/core/runpack"
	proof "github.com/Clyra-AI/proof"
)

const materializeReceiptDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func materializeReceiptKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("lifecycle-receipt-materialize-key"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func signedMaterializeReceipt(t *testing.T) (action.LifecycleReceipt, ed25519.PrivateKey) {
	t.Helper()
	key := materializeReceiptKey()
	ref := proof.RelationshipRef{Kind: "action_contract", ID: "contract:receipt", Digest: materializeReceiptDigest, SchemaID: "https://wrkr.dev/schemas/v1/proposed-action-contract-v3.schema.json", SchemaVersion: "3", SourceProduct: "wrkr"}
	receipt, err := (action.LifecycleReceipt{
		ContractFamilyID: "family:receipt",
		ContractID:       ref.ID,
		Revision:         1,
		ArtifactDigests:  []string{ref.Digest},
		ArtifactRefs:     []proof.RelationshipRef{ref},
		Correlation:      proof.ControlContainmentTelemetryProfile{ProfileVersion: "1.0", BindingMode: proof.BindingModeDigestBound, ContractRef: &ref, ContentDigest: ref.Digest},
		Authority:        "non_authoritative",
		Quarantine:       true,
		Redaction:        "reference_only",
		Outcome:          "out_of_scope",
		ObservedAt:       "2026-08-25T00:00:00Z",
		FreshUntil:       "2026-08-25T01:00:00Z",
	}).Sign(key)
	if err != nil {
		t.Fatal(err)
	}
	return receipt, key
}

func TestMaterializeLifecycleReceiptVerifyReplayAndSourceMetadata(t *testing.T) {
	receipt, key := signedMaterializeReceipt(t)
	first, err := MaterializeLifecycleReceipt(receipt, LifecycleReceiptMaterializeOptions{SigningPrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	second, err := MaterializeLifecycleReceipt(receipt, LifecycleReceiptMaterializeOptions{SigningPrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	if string(first.ZipBytes) != string(second.ZipBytes) {
		t.Fatal("materialized lifecycle receipt is nondeterministic")
	}
	path := filepath.Join(t.TempDir(), "lifecycle-receipt.zip")
	if err := os.WriteFile(path, first.ZipBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	verified, err := runpack.VerifyZip(path, runpack.VerifyOptions{PublicKey: key.Public().(ed25519.PublicKey), RequireSignature: true})
	if err != nil || verified.SignatureStatus != "verified" || verified.RunID == "" {
		t.Fatalf("materialized runpack verification: %#v %v", verified, err)
	}
	replayed, err := runpack.ReplayStub(path)
	expectedStatus := "error"
	if receipt.Outcome == "succeeded" || receipt.Outcome == "pass" {
		expectedStatus = "ok"
	}
	if err != nil || len(replayed.Steps) != 1 || replayed.Steps[0].Status != expectedStatus {
		t.Fatalf("materialized runpack replay: %#v %v", replayed, err)
	}
	pack, err := runpack.ReadRunpack(path)
	if err != nil || len(pack.Results) != 1 {
		t.Fatalf("read materialized runpack: %v", err)
	}
	if pack.Manifest.CaptureMode != "reference" || len(pack.Refs.Receipts) != 1 || pack.Refs.Receipts[0].SourceType != "lifecycle_receipt" || pack.Refs.Receipts[0].SourceLocator != receipt.ReceiptID || pack.Refs.Receipts[0].ContentDigest != receipt.ArtifactDigests[0] {
		t.Fatalf("source metadata was not preserved: manifest=%#v refs=%#v", pack.Manifest, pack.Refs)
	}
	if len(pack.Run.Timeline) != 1 || pack.Run.Timeline[0].Event != "lifecycle_receipt" || pack.Run.Timeline[0].Ref != receipt.CanonicalContentDigest {
		t.Fatalf("lifecycle source timeline was not preserved: %#v", pack.Run.Timeline)
	}
}

func TestMaterializeLifecycleReceiptRejectsMissingKeyAndTimestamp(t *testing.T) {
	receipt, key := signedMaterializeReceipt(t)
	if _, err := MaterializeLifecycleReceipt(receipt, LifecycleReceiptMaterializeOptions{}); err == nil {
		t.Fatal("missing materialization key accepted")
	}
	badTime := receipt
	badTime.ObservedAt = "not-a-timestamp"
	if _, err := MaterializeLifecycleReceipt(badTime, LifecycleReceiptMaterializeOptions{SigningPrivateKey: key}); err == nil || !strings.Contains(err.Error(), "observed_at_invalid") {
		t.Fatalf("invalid receipt timestamp: %v", err)
	}
}

func TestLifecycleReceiptValidationRejectsInvalidDigestsRefsAndAuthority(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*action.LifecycleReceipt)
	}{
		{name: "invalid digest", mutate: func(r *action.LifecycleReceipt) { r.ArtifactDigests = []string{"sha256:bad"} }},
		{name: "invalid ref", mutate: func(r *action.LifecycleReceipt) { r.ArtifactRefs[0].Digest = "" }},
		{name: "quarantined authority", mutate: func(r *action.LifecycleReceipt) { r.Authority = "authoritative" }},
		{name: "invalid authority", mutate: func(r *action.LifecycleReceipt) { r.Authority = "unknown" }},
		{name: "invalid outcome", mutate: func(r *action.LifecycleReceipt) { r.Outcome = "not-an-outcome" }},
		{name: "invalid observed time", mutate: func(r *action.LifecycleReceipt) { r.ObservedAt = "bad" }},
		{name: "stale window", mutate: func(r *action.LifecycleReceipt) { r.FreshUntil = r.ObservedAt }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			receipt, key := signedMaterializeReceipt(t)
			tc.mutate(&receipt)
			if _, err := receipt.Sign(key); err == nil {
				t.Fatal("invalid lifecycle receipt accepted")
			}
		})
	}
}
