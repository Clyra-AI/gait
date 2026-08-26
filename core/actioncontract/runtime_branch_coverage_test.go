package actioncontract

import (
	"strings"
	"testing"
)

func TestRuntimeBindingAndLifecycleValidationBranches(t *testing.T) {
	proposal, activation, action, readiness, _, _, _ := loadConformanceFixture(t, "successful-execution-effect-containment")
	digest := "sha256:" + strings.Repeat("a", 64)
	if _, err := BuildContractEvidenceBinding(Artifact{}, activation, action, readiness, "trace", digest, digest, digest); err == nil {
		t.Fatal("missing proposal accepted")
	}
	if _, err := BuildContractEvidenceBinding(proposal, activation, RuntimeAction{}, readiness, "trace", digest, digest, digest); err == nil {
		t.Fatal("missing action accepted")
	}
	if _, err := BuildContractEvidenceBinding(proposal, activation, action, readiness, "", "bad", digest, digest); err == nil {
		t.Fatal("bad trace accepted")
	}
	if _, err := BuildContractEvidenceBinding(proposal, activation, action, readiness, "trace", digest, digest, "bad"); err == nil {
		t.Fatal("bad activation accepted")
	}
	base := LifecycleResultOptions{JournalPath: t.TempDir() + "/journal", Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, PrivateKey: mustKey(t), TraceDigest: digest, TraceID: "trace", ResultDigest: digest, Outcome: "succeeded"}
	for _, mutate := range []func(*LifecycleResultOptions){func(v *LifecycleResultOptions) { v.JournalPath = "" }, func(v *LifecycleResultOptions) { v.Outcome = "bad" }, func(v *LifecycleResultOptions) { v.EffectOutcome = "validated"; v.Outcome = "failed" }, func(v *LifecycleResultOptions) { v.PrivateKey = nil }, func(v *LifecycleResultOptions) { v.RuntimeAction = RuntimeAction{} }, func(v *LifecycleResultOptions) { v.TraceDigest = "bad" }, func(v *LifecycleResultOptions) { v.TraceID = "" }, func(v *LifecycleResultOptions) { v.ResultDigest = "bad" }} {
		value := base
		mutate(&value)
		if _, err := AppendLifecycleResult(value); err == nil {
			t.Fatal("invalid lifecycle input accepted")
		}
	}
}

func TestAdvisoryProviderResponseBranches(t *testing.T) {
	if _, err := parseProviderReport([]byte("not-json"), "command"); err == nil {
		t.Fatal("malformed provider response accepted")
	}
	report := AdvisoryReport{Status: "pass", ProviderName: "x", ProviderVersion: "1", ActionID: "a", AdvisoryOnly: false}
	raw, _ := report.MarshalDeterministic()
	if _, err := parseProviderReport(raw, "command"); err == nil {
		t.Fatal("authoritative provider response accepted")
	}
	if _, err := EvaluateAdvisory(AdvisoryMode("bad"), OfflineAdvisoryEvaluator{}, AdvisoryInput{ActionID: "a"}); err == nil {
		t.Fatal("invalid advisory mode accepted")
	}
}

func TestChainCircuitValidationVocabularyBranches(t *testing.T) {
	for _, value := range [][]string{RuntimeActionClassVocabulary(), RuntimeCompositionRoleVocabulary(), RuntimeDataClassVocabulary(), RuntimeTargetTrustClassVocabulary(), RuntimeTransitionClassVocabulary(), RuntimeOutcomeClassVocabulary(), RuntimeResourceActionVocabulary()} {
		if len(value) == 0 {
			t.Fatal("runtime vocabulary unexpectedly empty")
		}
	}
	if len(ValidateChainPolicy(ChainPolicy{MaxSteps: -1, MaxDistinctTargets: -1, ForbiddenClasses: []string{"", "read"}, RequiredClasses: []string{"read"}})) == 0 {
		t.Fatal("invalid chain policy accepted")
	}
	if len(ValidateChainState(ChainState{StepCount: 2, StepIDs: []string{"bad", "bad"}, Classes: []string{"z", "a"}, Targets: []string{"z", "a"}})) == 0 {
		t.Fatal("invalid chain state accepted")
	}
	if len(ValidateChainCandidate(ChainStep{ID: "", Target: "", Classes: nil})) == 0 {
		t.Fatal("invalid chain candidate accepted")
	}
	base := CircuitBreakerInput{SchemaID: CircuitInputSchemaID, SchemaVersion: "1", Chain: ChainDecision{SchemaID: ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: ChainState{SchemaID: ChainStateSchemaID, SchemaVersion: "1"}}, EffectStatus: "pass", EffectAuthoritative: true}
	for _, mutate := range []func(*CircuitBreakerInput){func(v *CircuitBreakerInput) { v.EffectStatus = "bad" }, func(v *CircuitBreakerInput) { v.ContainmentStatus = "bad" }, func(v *CircuitBreakerInput) { v.StopStatus = "bad" }, func(v *CircuitBreakerInput) { v.RevocationStatus = "bad" }, func(v *CircuitBreakerInput) { v.InvalidationStatus = "bad" }, func(v *CircuitBreakerInput) { v.AffectedScope = []string{"z", "a"} }} {
		value := base
		mutate(&value)
		if len(ValidateCircuitInput(value)) == 0 {
			t.Fatal("invalid circuit input accepted")
		}
	}
	if d := EvaluateCircuit(base); !d.Allow {
		t.Fatalf("valid empty chain circuit denied: %#v", d)
	}
}

func TestCircuitDecisionReasonBranches(t *testing.T) {
	base := CircuitBreakerInput{SchemaID: CircuitInputSchemaID, SchemaVersion: "1", Chain: ChainDecision{SchemaID: ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: ChainState{SchemaID: ChainStateSchemaID, SchemaVersion: "1"}}, EffectStatus: "pass", EffectAuthoritative: true}
	mutations := []func(*CircuitBreakerInput){func(v *CircuitBreakerInput) { v.Chain.Allowed = false; v.Chain.ReasonCodes = []string{"denied"} }, func(v *CircuitBreakerInput) { v.EffectAuthoritative = false }, func(v *CircuitBreakerInput) { v.StopStatus = "requested" }, func(v *CircuitBreakerInput) { v.StopStatus = "failed" }, func(v *CircuitBreakerInput) { v.StopStatus = "denied" }, func(v *CircuitBreakerInput) { v.StopStatus = "acknowledged" }, func(v *CircuitBreakerInput) { v.RevocationStatus = "attempted" }, func(v *CircuitBreakerInput) { v.RevocationStatus = "failed" }, func(v *CircuitBreakerInput) { v.RevocationStatus = "denied" }, func(v *CircuitBreakerInput) { v.RevocationStatus = "acknowledged" }, func(v *CircuitBreakerInput) { v.ContainmentStatus = "partial" }, func(v *CircuitBreakerInput) { v.ContainmentStatus = "unresolved" }, func(v *CircuitBreakerInput) { v.InvalidationStatus = "capability_invalidated" }, func(v *CircuitBreakerInput) { v.OutOfScope = true }}
	for _, mutate := range mutations {
		value := base
		mutate(&value)
		decision := EvaluateCircuit(value)
		if decision.Allow {
			t.Fatal("circuit reason branch unexpectedly allowed")
		}
		if len(decision.ReasonCodes) == 0 {
			t.Fatal("circuit branch omitted reason")
		}
		if len(ValidateCircuitDecision(decision)) != 0 {
			t.Fatalf("decision invalid: %#v", decision)
		}
	}
}
