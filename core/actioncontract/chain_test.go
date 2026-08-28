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

func TestEvaluateCandidateRejectsStateAndPolicyLimits(t *testing.T) {
	validState := ChainState{SchemaID: ChainStateSchemaID, SchemaVersion: "1", StepCount: 1, StepIDs: []string{"first"}, Classes: []string{"read"}, Targets: []string{"one"}}
	valid := ChainStep{ID: "second", Classes: []string{"write"}, Target: "two"}
	for _, tc := range []struct {
		name   string
		prior  ChainState
		step   ChainStep
		policy ChainPolicy
		reason string
	}{
		{"forbidden", validState, valid, ChainPolicy{ForbiddenClasses: []string{"write"}}, "action_chain_forbidden_class"},
		{"step limit", validState, valid, ChainPolicy{MaxSteps: 1}, "action_chain_step_limit"},
		{"target limit", validState, valid, ChainPolicy{MaxDistinctTargets: 1}, "action_chain_target_limit"},
		{"duplicate", validState, ChainStep{ID: "first", Classes: []string{"write"}, Target: "two"}, ChainPolicy{}, "chain_step_id_duplicate"},
		{"bad prior count", ChainState{StepCount: 2, StepIDs: []string{"first"}}, valid, ChainPolicy{}, "chain_state_inconsistent"},
		{"empty prior class", ChainState{StepCount: 1, StepIDs: []string{"first"}, Classes: []string{""}}, valid, ChainPolicy{}, "chain_state_inconsistent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decision := EvaluateCandidate(tc.prior, tc.step, tc.policy)
			if decision.Allowed || !containsChainReason(decision.ReasonCodes, tc.reason) {
				t.Fatalf("decision=%+v, want %s", decision, tc.reason)
			}
			if decision.State.StepCount != tc.prior.StepCount {
				t.Fatalf("denied candidate mutated state: %+v", decision.State)
			}
		})
	}
}

func TestEvaluateActionChainRequiredAndTargetBranches(t *testing.T) {
	missing := EvaluateActionChain([]ChainStep{{ID: "read", Classes: []string{"read"}, Target: "one"}}, ChainPolicy{RequiredClasses: []string{"write"}})
	if missing.Allowed || !containsChainReason(missing.ReasonCodes, "action_chain_required_class_missing") {
		t.Fatalf("missing required class accepted: %+v", missing)
	}
	tooMany := EvaluateActionChain([]ChainStep{{ID: "one", Classes: []string{"read"}, Target: "one"}, {ID: "two", Classes: []string{"write"}, Target: "two"}}, ChainPolicy{MaxDistinctTargets: 1})
	if tooMany.Allowed || !containsChainReason(tooMany.ReasonCodes, "action_chain_target_limit") {
		t.Fatalf("target limit not enforced: %+v", tooMany)
	}
	if got := EvaluateActionChain(nil, ChainPolicy{}); !got.Allowed || got.State.StepCount != 0 {
		t.Fatalf("empty chain should be allowed: %+v", got)
	}
}

func containsChainReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
