package actioncontract

import (
	"encoding/json"
	"sort"
	"strings"
)

type CircuitBreakerInput struct {
	SchemaID            string        `json:"schema_id"`
	SchemaVersion       string        `json:"schema_version"`
	Chain               ChainDecision `json:"chain"`
	EffectStatus        string        `json:"effect_status"`
	EffectAuthoritative bool          `json:"effect_authoritative"`
	ContainmentStatus   string        `json:"containment_status"`
	StopStatus          string        `json:"stop_status"`
	RevocationStatus    string        `json:"revocation_status"`
	AffectedScope       []string      `json:"affected_scope,omitempty"`
	InvalidationStatus  string        `json:"invalidation_status,omitempty"`
	OutOfScope          bool          `json:"out_of_scope,omitempty"`
}
type CircuitBreakerDecision struct {
	SchemaID           string   `json:"schema_id"`
	SchemaVersion      string   `json:"schema_version"`
	Allow              bool     `json:"allow"`
	Tripped            bool     `json:"tripped"`
	ReasonCodes        []string `json:"reason_codes"`
	AffectedScope      []string `json:"affected_scope,omitempty"`
	InvalidationStatus string   `json:"invalidation_status,omitempty"`
	OutOfScope         bool     `json:"out_of_scope"`
	EnforcementClaimed bool     `json:"enforcement_claimed"`
}

const CircuitInputSchemaID = "https://gait.dev/schemas/v1/action-contract/circuit-breaker-input.schema.json"
const CircuitDecisionSchemaID = "https://gait.dev/schemas/v1/action-contract/circuit-breaker-decision.schema.json"

func ValidateCircuitInput(i CircuitBreakerInput) []string {
	reasons := []string{}
	if i.SchemaID != CircuitInputSchemaID || i.SchemaVersion != "1" {
		reasons = append(reasons, "circuit_schema_invalid")
	}
	reasons = append(reasons, validateChainDecision(i.Chain)...)
	if !containsCircuit([]string{"unknown", "pass", "fail", "partial"}, i.EffectStatus) {
		reasons = append(reasons, "circuit_effect_status_invalid")
	}
	if !containsCircuit([]string{"", "completed", "partial", "unresolved", "out_of_scope"}, i.ContainmentStatus) {
		reasons = append(reasons, "circuit_containment_status_invalid")
	}
	if !containsCircuit([]string{"", "requested", "attempted", "acknowledged", "denied", "failed"}, i.StopStatus) {
		reasons = append(reasons, "circuit_stop_status_invalid")
	}
	if !containsCircuit([]string{"", "attempted", "acknowledged", "denied", "failed"}, i.RevocationStatus) {
		reasons = append(reasons, "circuit_revocation_status_invalid")
	}
	if !containsCircuit([]string{"", "capability_invalidated", "descendant_invalidated"}, i.InvalidationStatus) {
		reasons = append(reasons, "circuit_invalidation_status_invalid")
	}
	if !sort.StringsAreSorted(i.AffectedScope) || hasDuplicateCircuit(i.AffectedScope) {
		reasons = append(reasons, "circuit_scope_noncanonical")
	}
	for _, scope := range i.AffectedScope {
		if strings.TrimSpace(scope) == "" {
			reasons = append(reasons, "circuit_scope_invalid")
		}
	}
	return uniqueCircuitReasons(reasons)
}

func ValidateCircuitDecision(d CircuitBreakerDecision) []string {
	reasons := []string{}
	if d.SchemaID != CircuitDecisionSchemaID || d.SchemaVersion != "1" {
		reasons = append(reasons, "circuit_decision_schema_invalid")
	}
	if d.Allow == d.Tripped {
		reasons = append(reasons, "circuit_decision_allow_trip_inconsistent")
	}
	if !sort.StringsAreSorted(d.ReasonCodes) || hasDuplicateCircuit(d.ReasonCodes) {
		reasons = append(reasons, "circuit_decision_reason_codes_noncanonical")
	}
	if !containsCircuit([]string{"", "capability_invalidated", "descendant_invalidated"}, d.InvalidationStatus) {
		reasons = append(reasons, "circuit_decision_invalidation_status_invalid")
	}
	if !sort.StringsAreSorted(d.AffectedScope) || hasDuplicateCircuit(d.AffectedScope) {
		reasons = append(reasons, "circuit_decision_scope_noncanonical")
	}
	for _, scope := range d.AffectedScope {
		if strings.TrimSpace(scope) == "" {
			reasons = append(reasons, "circuit_decision_scope_invalid")
		}
	}
	if d.OutOfScope && d.EnforcementClaimed {
		reasons = append(reasons, "circuit_out_of_scope_authority_claim")
	}
	return uniqueCircuitReasons(reasons)
}

func validateChainDecision(d ChainDecision) []string {
	reasons := []string{}
	if d.SchemaID != ChainDecisionSchemaID || d.SchemaVersion != "1" {
		reasons = append(reasons, "chain_decision_schema_invalid")
	}
	if d.State.SchemaID != ChainStateSchemaID || d.State.SchemaVersion != "1" {
		reasons = append(reasons, "chain_state_schema_invalid")
	}
	if d.Allowed && len(d.ReasonCodes) > 0 {
		reasons = append(reasons, "chain_decision_allow_trip_inconsistent")
	}
	if !d.Allowed && len(d.ReasonCodes) == 0 {
		reasons = append(reasons, "chain_decision_reasons_missing")
	}
	reasons = append(reasons, ValidateChainState(d.State)...)
	if !sort.StringsAreSorted(d.ReasonCodes) || hasDuplicateCircuit(d.ReasonCodes) {
		reasons = append(reasons, "chain_decision_reason_codes_noncanonical")
	}
	return uniqueCircuitReasons(reasons)
}

func ValidateCircuitJSON(raw []byte, value any) error {
	if err := DecodeStrictRuntimeJSON(raw, value); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &map[string]any{}); err != nil {
		return err
	}
	return nil
}

func containsCircuit(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func hasDuplicateCircuit(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}
func uniqueCircuitReasons(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func EvaluateCircuit(i CircuitBreakerInput) CircuitBreakerDecision {
	r := []string{}
	r = append(r, ValidateCircuitInput(i)...)
	if !i.Chain.Allowed {
		r = append(r, "chain_denied")
	}
	if !i.EffectAuthoritative || i.EffectStatus != "pass" {
		r = append(r, "effect_not_authoritative")
	}
	if i.StopStatus == "requested" || i.StopStatus == "attempted" {
		r = append(r, "stop_pending")
	}
	if i.StopStatus == "failed" {
		r = append(r, "stop_failed")
	}
	if i.StopStatus == "denied" {
		r = append(r, "stop_denied")
	}
	if i.StopStatus == "acknowledged" {
		r = append(r, "stop_acknowledged")
	}
	if i.RevocationStatus == "attempted" {
		r = append(r, "revocation_pending")
	}
	if i.RevocationStatus == "failed" {
		r = append(r, "revocation_failed")
	}
	if i.RevocationStatus == "denied" {
		r = append(r, "revocation_denied")
	}
	if i.RevocationStatus == "acknowledged" {
		r = append(r, "revocation_acknowledged")
	}
	if i.ContainmentStatus == "partial" || i.ContainmentStatus == "unresolved" {
		r = append(r, "containment_unresolved")
	}
	if i.InvalidationStatus != "" {
		r = append(r, "invalidation_detected")
	}
	if i.OutOfScope || i.ContainmentStatus == "out_of_scope" {
		r = append(r, "containment_out_of_scope")
	}
	sort.Strings(r)
	s := append([]string{}, i.AffectedScope...)
	sort.Strings(s)
	outOfScope := i.OutOfScope || i.ContainmentStatus == "out_of_scope"
	d := CircuitBreakerDecision{SchemaID: CircuitDecisionSchemaID, SchemaVersion: "1", Allow: len(r) == 0, Tripped: len(r) > 0, ReasonCodes: r, AffectedScope: s, InvalidationStatus: i.InvalidationStatus, OutOfScope: outOfScope, EnforcementClaimed: false}
	if !outOfScope {
		d.EnforcementClaimed = false
	}
	return d
}
