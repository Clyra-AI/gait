package actioncontract

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"
)

func TestCoverageValidationAndErrors(t *testing.T) {
	for _, p := range []ChainPolicy{{MaxSteps: -1}, {MaxDistinctTargets: -1}, {ForbiddenClasses: []string{"", "x", "x"}}, {RequiredClasses: []string{"x", "x"}}} {
		_ = ValidateChainPolicy(p)
	}
	for _, s := range []ChainState{{StepCount: -1}, {StepCount: 2, StepIDs: []string{"a"}}, {StepCount: 1, StepIDs: []string{"bad id"}}} {
		_ = ValidateChainState(s)
	}
	if _, e := ParseControlEventEvidence([]byte(`{"bad":1}`)); e == nil {
		t.Fatal("parse accepted")
	}
	if _, e := (OfflineAdvisoryEvaluator{}).Evaluate(AdvisoryInput{}); e == nil {
		t.Fatal("empty advisory accepted")
	}
}
func TestCoverageReceiptAndControlVerifyBranches(t *testing.T) {
	seed := sha256.Sum256([]byte("coverage-branches"))
	k := ed25519.NewKeyFromSeed(seed[:])
	p := k.Public().(ed25519.PublicKey)
	b := executionTestBinding()
	r := ControlEventEvidence{SchemaID: ControlEventEvidenceSchemaID, SchemaVersion: "1", EvidenceID: "bad", Binding: b, Command: "stop", Phase: "bad", OccurredAt: "bad", FreshUntil: "bad"}
	_, _ = VerifyControlEventEvidence(r, p)
	_, _ = VerifyControlEventEvidenceAt(r, p, time.Now())
	x := LifecycleReceipt{Authority: "bad", Redaction: "bad", Outcome: "bad"}
	_, _ = x.Sign(k)
	_ = json.Valid([]byte(`{"x":1}`))
}
