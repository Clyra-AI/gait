package actioncontract

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
)

func TestControlLifecycleUsesRealConformanceFixture(t *testing.T) {
	_, _, _, _, records, public, _ := loadConformanceFixture(t, "successful-execution-effect-containment")
	if len(records) == 0 {
		t.Fatal("fixture has no lifecycle records")
	}
	if snapshot, err := ReduceVerifiedLifecycle(records, public); err != nil || snapshot.CurrentStatus == "invalid" {
		t.Fatalf("base lifecycle reduction failed: %v %+v", err, snapshot)
	}
	seed := sha256.Sum256([]byte("gait-action-contract-activation-development-key-v1"))
	private := ed25519.NewKeyFromSeed(seed[:])
	if len(private) != ed25519.PrivateKeySize {
		t.Fatal("fixture key invalid")
	}
	result := VerifyLifecycleConformance(LifecycleConformanceInput{LifecycleRecords: records, LifecyclePublicKey: public})
	if result.AuthoritativeSuccess {
		t.Fatal("incomplete conformance unexpectedly authoritative")
	}
}
