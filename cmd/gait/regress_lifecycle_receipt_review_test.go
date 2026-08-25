package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	action "github.com/Clyra-AI/gait/core/actioncontract"
	proof "github.com/Clyra-AI/proof"
)

func TestRegressAddLifecycleReceiptUsesCurrentVerificationAndContractDigest(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	seed := sha256.Sum256([]byte("regress-receipt-review-key"))
	key := ed25519.NewKeyFromSeed(seed[:])
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	ref := proof.RelationshipRef{Kind: "action_contract", ID: "contract-id", Digest: digest, SchemaID: "https://wrkr.dev/schemas/v1/proposed-action-contract-v3.schema.json", SchemaVersion: "3", SourceProduct: "wrkr"}
	now := time.Now().UTC()
	receipt, err := (action.LifecycleReceipt{ContractFamilyID: "family-id", ContractID: "contract-id", Revision: 1, ArtifactDigests: []string{digest}, ArtifactRefs: []proof.RelationshipRef{ref}, Correlation: proof.ControlContainmentTelemetryProfile{ProfileVersion: "1.0", BindingMode: proof.BindingModeDigestBound, ContractRef: &ref, ContentDigest: digest}, Authority: "non_authoritative", Quarantine: true, Redaction: "reference_only", Outcome: "out_of_scope", ObservedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), FreshUntil: now.Add(time.Hour).Format(time.RFC3339Nano)}).Sign(key)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(workDir, "receipt.json")
	raw, _ := json.Marshal(receipt)
	if err := os.WriteFile(receiptPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	trustedPath := filepath.Join(workDir, "trusted.pub")
	privatePath := filepath.Join(workDir, "runpack.key")
	pub := key.Public().(ed25519.PublicKey)
	if err := os.WriteFile(trustedPath, []byte(base64.StdEncoding.EncodeToString(pub)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privatePath, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := runRegressAdd([]string{"--from", receiptPath, "--trusted-key", trustedPath, "--private-key", privatePath, "--expected-contract-digest", digest, "--name", "receipt-review", "--json"}); code != exitOK {
		t.Fatalf("receipt promotion expected %d got %d", exitOK, code)
	}
}
