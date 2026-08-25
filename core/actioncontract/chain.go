package actioncontract

import (
	"errors"
	"regexp"
	"sort"
	"strings"
)

// ChainStep is a bounded, already-classified action. Evaluation is pure and
// happens before execution; State is accumulated in input order.
type ChainStep struct {
	ID      string   `json:"id"`
	Classes []string `json:"classes,omitempty"`
	Target  string   `json:"target,omitempty"`
}
type ChainPolicy struct {
	SchemaID           string   `json:"schema_id"`
	SchemaVersion      string   `json:"schema_version"`
	MaxSteps           int      `json:"max_steps"`
	ForbiddenClasses   []string `json:"forbidden_classes,omitempty"`
	RequiredClasses    []string `json:"required_classes,omitempty"`
	MaxDistinctTargets int      `json:"max_distinct_targets,omitempty"`
}
type ChainState struct {
	SchemaID      string   `json:"schema_id"`
	SchemaVersion string   `json:"schema_version"`
	StepCount     int      `json:"step_count"`
	StepIDs       []string `json:"step_ids"`
	Classes       []string `json:"classes"`
	Targets       []string `json:"targets"`
}
type ChainDecision struct {
	SchemaID      string     `json:"schema_id"`
	SchemaVersion string     `json:"schema_version"`
	Allowed       bool       `json:"allowed"`
	ReasonCodes   []string   `json:"reason_codes,omitempty"`
	State         ChainState `json:"state"`
}

const (
	ChainPolicySchemaID   = "https://gait.dev/schemas/v1/action-contract/chain-policy.schema.json"
	ChainStateSchemaID    = "https://gait.dev/schemas/v1/action-contract/chain-state.schema.json"
	ChainDecisionSchemaID = "https://gait.dev/schemas/v1/action-contract/chain-decision.schema.json"
)

var chainIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,63}$`)

func validateChainID(id string) error {
	if !chainIDPattern.MatchString(id) {
		return errors.New("chain_identifier_invalid")
	}
	return nil
}
func ValidateChainPolicy(p ChainPolicy) []string {
	r := []string{}
	if p.SchemaID != "" && p.SchemaID != ChainPolicySchemaID {
		r = append(r, "chain_schema_id_invalid")
	}
	if p.SchemaVersion != "" && p.SchemaVersion != "1" {
		r = append(r, "chain_schema_version_invalid")
	}
	if p.MaxSteps < 0 || p.MaxSteps > 1024 {
		r = append(r, "chain_max_steps_invalid")
	}
	if p.MaxDistinctTargets < 0 || p.MaxDistinctTargets > 1024 {
		r = append(r, "chain_max_targets_invalid")
	}
	seen := map[string]bool{}
	for _, c := range p.ForbiddenClasses {
		c = strings.TrimSpace(c)
		if c == "" {
			r = append(r, "chain_class_invalid")
			continue
		}
		if seen[c] {
			r = append(r, "chain_class_duplicate")
		}
		seen[c] = true
	}
	for _, c := range p.RequiredClasses {
		c = strings.TrimSpace(c)
		if c == "" {
			r = append(r, "chain_class_invalid")
			continue
		}
		if seen[c] {
			r = append(r, "chain_class_overlap")
		}
		seen[c] = true
	}
	return sortedReasonCodes(r)
}
func ValidateChainState(s ChainState) []string {
	r := []string{}
	if s.SchemaID != "" && s.SchemaID != ChainStateSchemaID {
		r = append(r, "chain_schema_id_invalid")
	}
	if s.SchemaVersion != "" && s.SchemaVersion != "1" {
		r = append(r, "chain_schema_version_invalid")
	}
	if s.StepCount < 0 || s.StepCount > 1024 {
		r = append(r, "chain_state_step_count_invalid")
	}
	if !sort.StringsAreSorted(s.Classes) || !sort.StringsAreSorted(s.Targets) {
		r = append(r, "chain_state_not_canonical")
	}
	if hasDup(s.Classes) || hasDup(s.Targets) {
		r = append(r, "chain_state_duplicate")
	}
	return sortedReasonCodes(r)
}
func ValidateChainCandidate(c ChainStep) []string {
	r := []string{}
	if err := validateChainID(strings.TrimSpace(c.ID)); err != nil {
		r = append(r, err.Error())
	}
	if strings.TrimSpace(c.Target) == "" {
		r = append(r, "chain_target_missing")
	}
	if len(c.Classes) == 0 {
		r = append(r, "chain_classes_missing")
	}
	seen := map[string]bool{}
	for _, v := range c.Classes {
		v = strings.TrimSpace(v)
		if v == "" {
			r = append(r, "chain_class_invalid")
		}
		if seen[v] {
			r = append(r, "chain_class_duplicate")
		}
		seen[v] = true
	}
	return sortedReasonCodes(r)
}
func hasDup(v []string) bool {
	for i := 1; i < len(v); i++ {
		if v[i] == v[i-1] {
			return true
		}
	}
	return false
}
func sortedReasonCodes(v []string) []string {
	m := map[string]bool{}
	for _, x := range v {
		m[x] = true
	}
	o := make([]string, 0, len(m))
	for x := range m {
		o = append(o, x)
	}
	sort.Strings(o)
	return o
}

// EvaluateCandidate applies one candidate to an immutable prior state.
func EvaluateCandidate(prior ChainState, candidate ChainStep, policy ChainPolicy) ChainDecision {
	reasons := append([]string{}, ValidateChainPolicy(policy)...)
	reasons = append(reasons, ValidateChainState(prior)...)
	reasons = append(reasons, ValidateChainCandidate(candidate)...)
	if prior.StepCount < len(prior.Classes) || prior.StepCount < len(prior.Targets) {
		reasons = append(reasons, "chain_state_inconsistent")
	}
	for _, x := range prior.Classes {
		if strings.TrimSpace(x) == "" {
			reasons = append(reasons, "chain_state_inconsistent")
		}
	}
	for _, x := range prior.Targets {
		if strings.TrimSpace(x) == "" {
			reasons = append(reasons, "chain_state_inconsistent")
		}
	}
	for _, x := range policy.ForbiddenClasses {
		for _, c := range candidate.Classes {
			if strings.TrimSpace(x) == strings.TrimSpace(c) {
				reasons = append(reasons, "action_chain_forbidden_class")
			}
		}
	}
	if prior.StepCount != len(prior.StepIDs) || hasDup(prior.StepIDs) {
		reasons = append(reasons, "chain_state_inconsistent")
	}
	if hasDup([]string{candidate.ID}) {
		reasons = append(reasons, "chain_step_id_duplicate")
	}
	for _, id := range prior.StepIDs {
		if id == candidate.ID {
			reasons = append(reasons, "chain_step_id_duplicate")
		}
	}
	if len(reasons) > 0 {
		return ChainDecision{SchemaID: ChainDecisionSchemaID, SchemaVersion: "1", Allowed: false, ReasonCodes: sortedReasonCodes(reasons), State: prior}
	}
	next := ChainState{SchemaID: ChainStateSchemaID, SchemaVersion: "1", StepCount: prior.StepCount + 1, StepIDs: append(append([]string{}, prior.StepIDs...), candidate.ID), Classes: append([]string{}, prior.Classes...), Targets: append([]string{}, prior.Targets...)}
	next.Classes = append(next.Classes, candidate.Classes...)
	next.Targets = append(next.Targets, candidate.Target)
	sort.Strings(next.Classes)
	sort.Strings(next.Targets)
	next.Classes = unique(next.Classes)
	next.Targets = unique(next.Targets)
	for _, x := range policy.ForbiddenClasses {
		for _, c := range next.Classes {
			if strings.TrimSpace(x) == c {
				reasons = append(reasons, "action_chain_forbidden_class")
			}
		}
	}
	if policy.MaxSteps > 0 && next.StepCount > policy.MaxSteps {
		reasons = append(reasons, "action_chain_step_limit")
	}
	if policy.MaxDistinctTargets > 0 && len(next.Targets) > policy.MaxDistinctTargets {
		reasons = append(reasons, "action_chain_target_limit")
	}
	if len(reasons) > 0 {
		return ChainDecision{SchemaID: ChainDecisionSchemaID, SchemaVersion: "1", Allowed: false, ReasonCodes: sortedReasonCodes(reasons), State: prior}
	}
	return ChainDecision{SchemaID: ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, ReasonCodes: nil, State: next}
}
func unique(v []string) []string {
	o := v[:0]
	for _, x := range v {
		if len(o) == 0 || o[len(o)-1] != x {
			o = append(o, x)
		}
	}
	return o
}

func EvaluateActionChain(steps []ChainStep, policy ChainPolicy) ChainDecision {
	state := ChainState{SchemaID: ChainStateSchemaID, SchemaVersion: "1"}
	stepPolicy := policy
	stepPolicy.RequiredClasses = nil
	for _, step := range steps {
		d := EvaluateCandidate(state, step, stepPolicy)
		if !d.Allowed {
			return d
		}
		state = d.State
	}
	reasons := []string{}
	for _, c := range policy.RequiredClasses {
		found := false
		for _, x := range state.Classes {
			if strings.TrimSpace(c) == x {
				found = true
			}
		}
		if !found {
			reasons = append(reasons, "action_chain_required_class_missing")
		}
	}
	return ChainDecision{SchemaID: ChainDecisionSchemaID, SchemaVersion: "1", Allowed: len(reasons) == 0, ReasonCodes: sortedReasonCodes(reasons), State: state}
}
