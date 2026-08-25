package actioncontract

import "testing"

func validCircuitInput() CircuitBreakerInput {
	return CircuitBreakerInput{SchemaID: CircuitInputSchemaID, SchemaVersion: "1", Chain: ChainDecision{SchemaID: ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: ChainState{SchemaID: ChainStateSchemaID, SchemaVersion: "1", StepCount: 0, StepIDs: []string{}, Classes: []string{}, Targets: []string{}}}, EffectStatus: "pass", EffectAuthoritative: true, ContainmentStatus: "completed", StopStatus: "", RevocationStatus: "", AffectedScope: []string{"repo:a"}}
}

func TestEvaluateCircuitControlAndAuthoritySemantics(t *testing.T) {
	stop := validCircuitInput()
	stop.StopStatus = "acknowledged"
	decision := EvaluateCircuit(stop)
	if decision.Allow || !decision.Tripped || !hasCircuitReason(decision.ReasonCodes, "stop_acknowledged") {
		t.Fatalf("acknowledged stop not tripped: %+v", decision)
	}
	revocation := validCircuitInput()
	revocation.RevocationStatus = "acknowledged"
	decision = EvaluateCircuit(revocation)
	if decision.Allow || !hasCircuitReason(decision.ReasonCodes, "revocation_acknowledged") {
		t.Fatalf("acknowledged revocation not tripped: %+v", decision)
	}
	out := validCircuitInput()
	out.ContainmentStatus = "out_of_scope"
	decision = EvaluateCircuit(out)
	if !decision.OutOfScope || decision.EnforcementClaimed || !hasCircuitReason(decision.ReasonCodes, "containment_out_of_scope") {
		t.Fatalf("out-of-scope semantics wrong: %+v", decision)
	}
	invalidated := validCircuitInput()
	invalidated.InvalidationStatus = "capability_invalidated"
	decision = EvaluateCircuit(invalidated)
	if decision.OutOfScope || !hasCircuitReason(decision.ReasonCodes, "invalidation_detected") || hasCircuitReason(decision.ReasonCodes, "containment_out_of_scope") {
		t.Fatalf("invalidation conflated with containment: %+v", decision)
	}
}

func TestValidateCircuitRejectsInvalidStatusesAndNonCanonicalScope(t *testing.T) {
	i := validCircuitInput()
	i.StopStatus = "bogus"
	i.AffectedScope = []string{"z", "a", "a"}
	reasons := ValidateCircuitInput(i)
	if !hasCircuitReason(reasons, "circuit_stop_status_invalid") || !hasCircuitReason(reasons, "circuit_scope_noncanonical") {
		t.Fatalf("invalid circuit accepted: %v", reasons)
	}
	i.EffectStatus = "PASS"
	if !hasCircuitReason(ValidateCircuitInput(i), "circuit_effect_status_invalid") {
		t.Fatal("non-canonical effect status accepted")
	}
	d := EvaluateCircuit(validCircuitInput())
	d.EnforcementClaimed = true
	d.OutOfScope = true
	if !hasCircuitReason(ValidateCircuitDecision(d), "circuit_out_of_scope_authority_claim") {
		t.Fatal("out-of-scope authority claim accepted")
	}
	d = EvaluateCircuit(validCircuitInput())
	d.InvalidationStatus = "invalid"
	if !hasCircuitReason(ValidateCircuitDecision(d), "circuit_decision_invalidation_status_invalid") {
		t.Fatal("invalid decision invalidation status accepted")
	}
}

func hasCircuitReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
