package actioncontract

import "testing"

func TestEvaluateActionChainUsesAccumulatedState(t *testing.T) {
	d := EvaluateActionChain([]ChainStep{{ID: "a", Classes: []string{"read"}, Target: "one"}, {ID: "b", Classes: []string{"write"}, Target: "two"}}, ChainPolicy{MaxSteps: 2, RequiredClasses: []string{"read", "write"}, MaxDistinctTargets: 2})
	if !d.Allowed || len(d.State.Classes) != 2 {
		t.Fatalf("unexpected decision: %+v", d)
	}
	d = EvaluateActionChain([]ChainStep{{ID: "a", Classes: []string{"read"}}, {ID: "b", Classes: []string{"delete"}}}, ChainPolicy{ForbiddenClasses: []string{"delete"}})
	if d.Allowed || len(d.ReasonCodes) != 1 {
		t.Fatalf("forbidden chain allowed: %+v", d)
	}
}

func TestEvaluateCandidateRejectsMalformedAndIsDeterministic(t *testing.T) {
	p := ChainPolicy{MaxSteps: 2, ForbiddenClasses: []string{"delete"}}
	d := EvaluateCandidate(ChainState{}, ChainStep{ID: "bad id", Classes: []string{"delete"}, Target: "x"}, p)
	if d.Allowed || len(d.ReasonCodes) == 0 {
		t.Fatalf("malformed candidate allowed: %+v", d)
	}
	a := EvaluateCandidate(ChainState{StepCount: 1, Classes: []string{"read"}, Targets: []string{"x"}}, ChainStep{ID: "b", Classes: []string{"write"}, Target: "x"}, p)
	b := EvaluateCandidate(ChainState{StepCount: 1, Classes: []string{"read"}, Targets: []string{"x"}}, ChainStep{ID: "b", Classes: []string{"write"}, Target: "x"}, p)
	if a.Allowed != b.Allowed || a.State.StepCount != b.State.StepCount {
		t.Fatalf("nondeterministic candidate evaluation")
	}
}

func TestEvaluateActionChainRejectsDuplicateIDs(t *testing.T) {
	d := EvaluateActionChain([]ChainStep{{ID: "a", Classes: []string{"read"}, Target: "x"}, {ID: "a", Classes: []string{"write"}, Target: "y"}}, ChainPolicy{})
	if d.Allowed || len(d.ReasonCodes) != 1 || d.ReasonCodes[0] != "chain_step_id_duplicate" {
		t.Fatalf("duplicate IDs not rejected: %+v", d)
	}
}
