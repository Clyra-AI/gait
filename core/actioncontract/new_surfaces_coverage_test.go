package actioncontract

import (
	"crypto/ed25519"
	"crypto/sha256"
	proof "github.com/Clyra-AI/proof"
	"testing"
	"time"
)

func TestNewSurfaceErrorMatrix(t *testing.T) {
	for _, p := range []ChainPolicy{{MaxSteps: -1}, {MaxDistinctTargets: -1}, {ForbiddenClasses: []string{""}}, {RequiredClasses: []string{"x", "x"}}} {
		if len(ValidateChainPolicy(p)) == 0 {
			t.Fatal("policy error missed")
		}
	}
	for _, s := range []ChainState{{StepCount: -1}, {StepCount: 1, StepIDs: []string{"a"}, Classes: []string{"b", "a"}}, {StepCount: 2, StepIDs: []string{"a", "a"}}} {
		if len(ValidateChainState(s)) == 0 {
			t.Fatal("state error missed")
		}
	}
	for _, c := range []ChainStep{{ID: "bad id"}, {ID: "a", Target: "x"}, {ID: "a", Classes: []string{""}, Target: "x"}} {
		if len(ValidateChainCandidate(c)) == 0 {
			t.Fatal("candidate error missed")
		}
	}
}
func TestCircuitBranchesAndDeterminism(t *testing.T) {
	base := CircuitBreakerInput{Chain: ChainDecision{Allowed: true}, EffectStatus: "pass", EffectAuthoritative: true}
	for i, x := range []CircuitBreakerInput{base, {Chain: ChainDecision{Allowed: false}}, func() CircuitBreakerInput { v := base; v.EffectAuthoritative = false; return v }(), func() CircuitBreakerInput { v := base; v.ContainmentStatus = "out_of_scope"; return v }(), func() CircuitBreakerInput { v := base; v.StopStatus = "acknowledged"; return v }(), func() CircuitBreakerInput { v := base; v.InvalidationStatus = "capability_invalidated"; return v }()} {
		d := EvaluateCircuit(x)
		if !d.Tripped && i > 0 {
			t.Fatal("circuit did not trip")
		}
	}
}
func TestReceiptAndAdvisoryCoverage(t *testing.T) {
	s := sha256.Sum256([]byte("coverage-key"))
	k := ed25519.NewKeyFromSeed(s[:])
	p := k.Public().(ed25519.PublicKey)
	a, _ := (OfflineAdvisoryEvaluator{}).Evaluate(AdvisoryInput{ActionID: "a"})
	a, _ = a.Sign(k)
	_, _ = a.MarshalDeterministic()
	_ = VerifyAdvisoryReport(a, p, "", "")
	a.Status = "bad"
	_ = VerifyAdvisoryReport(a, p, "", "")
	r := LifecycleReceipt{ContractFamilyID: "f", ContractID: "c", Revision: 1, ArtifactDigests: []string{"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}, Authority: "non_authoritative", Quarantine: true, Redaction: "reference_only", Outcome: "unknown", ObservedAt: "2026-01-01T00:00:00Z", FreshUntil: "2026-01-01T01:00:00Z"}
	cr := proof.RelationshipRef{Kind: "action_contract", ID: "c", Digest: r.ArtifactDigests[0], SchemaID: "s", SchemaVersion: "1", SourceProduct: "gait"}
	r.Correlation = proof.ControlContainmentTelemetryProfile{ProfileVersion: "1.0", BindingMode: proof.BindingModeDigestBound, ContractRef: &cr, ContentDigest: cr.Digest}
	r.ArtifactRefs = []proof.RelationshipRef{cr}
	r, _ = r.Sign(k)
	_ = VerifyLifecycleReceiptAt(r, p, time.Date(2026, 1, 1, 0, 30, 0, 0, time.UTC))
	r.Outcome = "tamper"
	_ = VerifyLifecycleReceipt(r, p)
}
