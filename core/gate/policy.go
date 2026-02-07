package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	gaitjcs "github.com/davidahmann/gait/core/jcs"
	"github.com/goccy/go-yaml"

	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
)

const (
	policySchemaID = "gait.gate.policy"
	policySchemaV1 = "1.0.0"
	defaultVerdict = "require_approval"
	gateSchemaID   = "gait.gate.result"
	gateSchemaV1   = "1.0.0"
)

var (
	allowedVerdicts = map[string]struct{}{
		"allow":            {},
		"block":            {},
		"dry_run":          {},
		"require_approval": {},
	}
	allowedRequiredFields = map[string]struct{}{
		"targets":        {},
		"arg_provenance": {},
	}
	allowedRateLimitScopes = map[string]struct{}{
		"tool":          {},
		"identity":      {},
		"tool_identity": {},
	}
	allowedRateLimitWindows = map[string]struct{}{
		"minute": {},
		"hour":   {},
	}
	allowedDataflowActions = map[string]struct{}{
		"block":            {},
		"require_approval": {},
	}
)

type Policy struct {
	SchemaID       string           `yaml:"schema_id"`
	SchemaVersion  string           `yaml:"schema_version"`
	DefaultVerdict string           `yaml:"default_verdict"`
	FailClosed     FailClosedPolicy `yaml:"fail_closed"`
	Rules          []PolicyRule     `yaml:"rules"`
}

type FailClosedPolicy struct {
	Enabled        bool     `yaml:"enabled"`
	RiskClasses    []string `yaml:"risk_classes"`
	RequiredFields []string `yaml:"required_fields"`
}

type PolicyRule struct {
	Name                     string          `yaml:"name"`
	Priority                 int             `yaml:"priority"`
	Effect                   string          `yaml:"effect"`
	Match                    PolicyMatch     `yaml:"match"`
	ReasonCodes              []string        `yaml:"reason_codes"`
	Violations               []string        `yaml:"violations"`
	MinApprovals             int             `yaml:"min_approvals"`
	RequireDistinctApprovers bool            `yaml:"require_distinct_approvers"`
	RequireBrokerCredential  bool            `yaml:"require_broker_credential"`
	BrokerReference          string          `yaml:"broker_reference"`
	BrokerScopes             []string        `yaml:"broker_scopes"`
	RateLimit                RateLimitPolicy `yaml:"rate_limit"`
	Dataflow                 DataflowPolicy  `yaml:"dataflow"`
}

type RateLimitPolicy struct {
	Requests int    `yaml:"requests"`
	Window   string `yaml:"window"`
	Scope    string `yaml:"scope"`
}

type DataflowPolicy struct {
	Enabled               bool     `yaml:"enabled"`
	TaintedSources        []string `yaml:"tainted_sources"`
	DestinationKinds      []string `yaml:"destination_kinds"`
	DestinationValues     []string `yaml:"destination_values"`
	DestinationOperations []string `yaml:"destination_operations"`
	Action                string   `yaml:"action"`
	ReasonCode            string   `yaml:"reason_code"`
	Violation             string   `yaml:"violation"`
}

type PolicyMatch struct {
	ToolNames         []string `yaml:"tool_names"`
	RiskClasses       []string `yaml:"risk_classes"`
	TargetKinds       []string `yaml:"target_kinds"`
	TargetValues      []string `yaml:"target_values"`
	DataClasses       []string `yaml:"data_classes"`
	DestinationKinds  []string `yaml:"destination_kinds"`
	DestinationValues []string `yaml:"destination_values"`
	DestinationOps    []string `yaml:"destination_operations"`
	ProvenanceSources []string `yaml:"provenance_sources"`
	Identities        []string `yaml:"identities"`
	WorkspacePrefixes []string `yaml:"workspace_prefixes"`
}

type EvalOptions struct {
	ProducerVersion string
}

type EvalOutcome struct {
	Result                   schemagate.GateResult
	MatchedRule              string
	MinApprovals             int
	RequireDistinctApprovers bool
	RequireBrokerCredential  bool
	BrokerReference          string
	BrokerScopes             []string
	RateLimit                RateLimitPolicy
	DataflowTriggered        bool
}

func LoadPolicyFile(path string) (Policy, error) {
	// #nosec G304 -- policy path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("read policy: %w", err)
	}
	return ParsePolicyYAML(content)
}

func ParsePolicyYAML(data []byte) (Policy, error) {
	var policy Policy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return Policy{}, fmt.Errorf("parse policy yaml: %w", err)
	}
	return normalizePolicy(policy)
}

func EvaluatePolicy(policy Policy, intent schemagate.IntentRequest, opts EvalOptions) (schemagate.GateResult, error) {
	outcome, err := EvaluatePolicyDetailed(policy, intent, opts)
	if err != nil {
		return schemagate.GateResult{}, err
	}
	return outcome.Result, nil
}

func PolicyHasHighRiskUnbrokeredActions(policy Policy) bool {
	normalizedPolicy, err := normalizePolicy(policy)
	if err != nil {
		return false
	}
	for _, rule := range normalizedPolicy.Rules {
		if !isHighRiskActionRule(rule) {
			continue
		}
		if !rule.RequireBrokerCredential {
			return true
		}
	}
	return false
}

func PolicyRequiresBrokerForHighRisk(policy Policy) bool {
	normalizedPolicy, err := normalizePolicy(policy)
	if err != nil {
		return false
	}
	for _, rule := range normalizedPolicy.Rules {
		if !isHighRiskActionRule(rule) {
			continue
		}
		if rule.RequireBrokerCredential {
			return true
		}
	}
	return false
}

func EvaluatePolicyDetailed(policy Policy, intent schemagate.IntentRequest, opts EvalOptions) (EvalOutcome, error) {
	normalizedPolicy, err := normalizePolicy(policy)
	if err != nil {
		return EvalOutcome{}, err
	}

	normalizedIntent, err := NormalizeIntent(intent)
	if err != nil {
		if shouldFailClosed(normalizedPolicy.FailClosed, strings.ToLower(strings.TrimSpace(intent.Context.RiskClass))) {
			return EvalOutcome{
				Result: buildGateResult(normalizedPolicy, intent, opts, "block", []string{"fail_closed_intent_invalid"}, []string{"intent_not_evaluable"}),
			}, nil
		}
		return EvalOutcome{}, fmt.Errorf("normalize intent: %w", err)
	}

	if shouldFailClosed(normalizedPolicy.FailClosed, normalizedIntent.Context.RiskClass) {
		reasons, violations := evaluateFailClosedRequiredFields(normalizedPolicy.FailClosed.RequiredFields, normalizedIntent)
		if len(reasons) > 0 {
			return EvalOutcome{
				Result: buildGateResult(normalizedPolicy, normalizedIntent, opts, "block", reasons, violations),
			}, nil
		}
	}

	for _, rule := range normalizedPolicy.Rules {
		if !ruleMatches(rule.Match, normalizedIntent) {
			continue
		}
		effect := rule.Effect
		reasons := uniqueSorted(rule.ReasonCodes)
		violations := uniqueSorted(rule.Violations)
		if len(reasons) == 0 {
			reasons = []string{"matched_rule_" + sanitizeName(rule.Name)}
		}
		dataflowTriggered, dataflowEffect, dataflowReasons, dataflowViolations := evaluateDataflowConstraint(rule.Dataflow, normalizedIntent)
		if dataflowTriggered {
			effect = dataflowEffect
			reasons = mergeUniqueSorted(reasons, dataflowReasons)
			violations = mergeUniqueSorted(violations, dataflowViolations)
		}
		minApprovals := rule.MinApprovals
		if effect == "require_approval" && minApprovals == 0 {
			minApprovals = 1
		}
		return EvalOutcome{
			Result:                   buildGateResult(normalizedPolicy, normalizedIntent, opts, effect, reasons, violations),
			MatchedRule:              rule.Name,
			MinApprovals:             minApprovals,
			RequireDistinctApprovers: rule.RequireDistinctApprovers,
			RequireBrokerCredential:  rule.RequireBrokerCredential,
			BrokerReference:          rule.BrokerReference,
			BrokerScopes:             uniqueSorted(rule.BrokerScopes),
			RateLimit:                rule.RateLimit,
			DataflowTriggered:        dataflowTriggered,
		}, nil
	}

	minApprovals := 0
	if normalizedPolicy.DefaultVerdict == "require_approval" {
		minApprovals = 1
	}
	return EvalOutcome{
		Result: buildGateResult(
			normalizedPolicy,
			normalizedIntent,
			opts,
			normalizedPolicy.DefaultVerdict,
			[]string{"default_" + normalizedPolicy.DefaultVerdict},
			[]string{},
		),
		MinApprovals: minApprovals,
	}, nil
}

func PolicyDigest(policy Policy) (string, error) {
	normalized, err := normalizePolicy(policy)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(policyDigestPayload(normalized))
	if err != nil {
		return "", fmt.Errorf("marshal normalized policy: %w", err)
	}
	digest, err := gaitjcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest policy: %w", err)
	}
	return digest, nil
}

func policyDigestPayload(policy Policy) map[string]any {
	rules := make([]any, 0, len(policy.Rules))
	for _, rule := range policy.Rules {
		matchPayload := map[string]any{
			"ToolNames":         rule.Match.ToolNames,
			"RiskClasses":       rule.Match.RiskClasses,
			"TargetKinds":       rule.Match.TargetKinds,
			"TargetValues":      rule.Match.TargetValues,
			"ProvenanceSources": rule.Match.ProvenanceSources,
			"Identities":        rule.Match.Identities,
			"WorkspacePrefixes": rule.Match.WorkspacePrefixes,
		}
		if len(rule.Match.DataClasses) > 0 {
			matchPayload["DataClasses"] = rule.Match.DataClasses
		}
		if len(rule.Match.DestinationKinds) > 0 {
			matchPayload["DestinationKinds"] = rule.Match.DestinationKinds
		}
		if len(rule.Match.DestinationValues) > 0 {
			matchPayload["DestinationValues"] = rule.Match.DestinationValues
		}
		if len(rule.Match.DestinationOps) > 0 {
			matchPayload["DestinationOps"] = rule.Match.DestinationOps
		}

		rulePayload := map[string]any{
			"Name":        rule.Name,
			"Priority":    rule.Priority,
			"Effect":      rule.Effect,
			"Match":       matchPayload,
			"ReasonCodes": rule.ReasonCodes,
			"Violations":  rule.Violations,
		}
		if rule.MinApprovals > 0 {
			rulePayload["MinApprovals"] = rule.MinApprovals
		}
		if rule.RequireDistinctApprovers {
			rulePayload["RequireDistinctApprovers"] = rule.RequireDistinctApprovers
		}
		if rule.RequireBrokerCredential {
			rulePayload["RequireBrokerCredential"] = rule.RequireBrokerCredential
		}
		if rule.BrokerReference != "" {
			rulePayload["BrokerReference"] = rule.BrokerReference
		}
		if len(rule.BrokerScopes) > 0 {
			rulePayload["BrokerScopes"] = rule.BrokerScopes
		}
		if rule.RateLimit.Requests > 0 {
			rulePayload["RateLimit"] = map[string]any{
				"Requests": rule.RateLimit.Requests,
				"Window":   rule.RateLimit.Window,
				"Scope":    rule.RateLimit.Scope,
			}
		}
		if rule.Dataflow.Enabled {
			dataflowPayload := map[string]any{
				"Enabled":        rule.Dataflow.Enabled,
				"TaintedSources": rule.Dataflow.TaintedSources,
				"Action":         rule.Dataflow.Action,
				"ReasonCode":     rule.Dataflow.ReasonCode,
				"Violation":      rule.Dataflow.Violation,
			}
			if len(rule.Dataflow.DestinationKinds) > 0 {
				dataflowPayload["DestinationKinds"] = rule.Dataflow.DestinationKinds
			}
			if len(rule.Dataflow.DestinationValues) > 0 {
				dataflowPayload["DestinationValues"] = rule.Dataflow.DestinationValues
			}
			if len(rule.Dataflow.DestinationOperations) > 0 {
				dataflowPayload["DestinationOperations"] = rule.Dataflow.DestinationOperations
			}
			rulePayload["Dataflow"] = dataflowPayload
		}
		rules = append(rules, rulePayload)
	}

	return map[string]any{
		"SchemaID":       policy.SchemaID,
		"SchemaVersion":  policy.SchemaVersion,
		"DefaultVerdict": policy.DefaultVerdict,
		"FailClosed": map[string]any{
			"Enabled":        policy.FailClosed.Enabled,
			"RiskClasses":    policy.FailClosed.RiskClasses,
			"RequiredFields": policy.FailClosed.RequiredFields,
		},
		"Rules": rules,
	}
}

func isHighRiskActionRule(rule PolicyRule) bool {
	if rule.Effect == "block" {
		return false
	}
	for _, riskClass := range rule.Match.RiskClasses {
		if riskClass == "high" || riskClass == "critical" {
			return true
		}
	}
	return false
}

func normalizePolicy(input Policy) (Policy, error) {
	output := input
	if output.SchemaID == "" {
		output.SchemaID = policySchemaID
	}
	if output.SchemaID != policySchemaID {
		return Policy{}, fmt.Errorf("unsupported policy schema_id: %s", output.SchemaID)
	}
	if output.SchemaVersion == "" {
		output.SchemaVersion = policySchemaV1
	}
	if output.SchemaVersion != policySchemaV1 {
		return Policy{}, fmt.Errorf("unsupported policy schema_version: %s", output.SchemaVersion)
	}

	output.DefaultVerdict = strings.ToLower(strings.TrimSpace(output.DefaultVerdict))
	if output.DefaultVerdict == "" {
		output.DefaultVerdict = defaultVerdict
	}
	if _, ok := allowedVerdicts[output.DefaultVerdict]; !ok {
		return Policy{}, fmt.Errorf("invalid default_verdict: %s", output.DefaultVerdict)
	}

	output.FailClosed.RiskClasses = normalizeStringListLower(output.FailClosed.RiskClasses)
	if output.FailClosed.Enabled && len(output.FailClosed.RiskClasses) == 0 {
		output.FailClosed.RiskClasses = []string{"critical", "high"}
	}
	output.FailClosed.RequiredFields = normalizeStringListLower(output.FailClosed.RequiredFields)
	for _, field := range output.FailClosed.RequiredFields {
		if _, ok := allowedRequiredFields[field]; !ok {
			return Policy{}, fmt.Errorf("unsupported fail_closed required_field: %s", field)
		}
	}

	output.Rules = append([]PolicyRule(nil), output.Rules...)
	for index := range output.Rules {
		rule := &output.Rules[index]
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			return Policy{}, fmt.Errorf("rule name is required")
		}

		rule.Effect = strings.ToLower(strings.TrimSpace(rule.Effect))
		if rule.Effect == "" {
			return Policy{}, fmt.Errorf("rule effect is required for %s", rule.Name)
		}
		if _, ok := allowedVerdicts[rule.Effect]; !ok {
			return Policy{}, fmt.Errorf("invalid rule effect %q for %s", rule.Effect, rule.Name)
		}

		rule.Match.ToolNames = normalizeStringListLower(rule.Match.ToolNames)
		rule.Match.RiskClasses = normalizeStringListLower(rule.Match.RiskClasses)
		rule.Match.TargetKinds = normalizeStringListLower(rule.Match.TargetKinds)
		rule.Match.TargetValues = normalizeStringList(rule.Match.TargetValues)
		rule.Match.DataClasses = normalizeStringListLower(rule.Match.DataClasses)
		rule.Match.DestinationKinds = normalizeStringListLower(rule.Match.DestinationKinds)
		rule.Match.DestinationValues = normalizeStringList(rule.Match.DestinationValues)
		rule.Match.DestinationOps = normalizeStringListLower(rule.Match.DestinationOps)
		rule.Match.ProvenanceSources = normalizeStringListLower(rule.Match.ProvenanceSources)
		rule.Match.Identities = normalizeStringList(rule.Match.Identities)
		rule.Match.WorkspacePrefixes = normalizeStringList(rule.Match.WorkspacePrefixes)
		rule.ReasonCodes = uniqueSorted(rule.ReasonCodes)
		rule.Violations = uniqueSorted(rule.Violations)
		if rule.MinApprovals < 0 {
			return Policy{}, fmt.Errorf("min_approvals must be >= 0 for %s", rule.Name)
		}
		if rule.MinApprovals > 1 && !rule.RequireDistinctApprovers {
			rule.RequireDistinctApprovers = true
		}
		rule.BrokerReference = strings.TrimSpace(rule.BrokerReference)
		rule.BrokerScopes = normalizeStringListLower(rule.BrokerScopes)
		if rule.RateLimit.Requests < 0 {
			return Policy{}, fmt.Errorf("rate_limit.requests must be >= 0 for %s", rule.Name)
		}
		rule.RateLimit.Window = strings.ToLower(strings.TrimSpace(rule.RateLimit.Window))
		rule.RateLimit.Scope = strings.ToLower(strings.TrimSpace(rule.RateLimit.Scope))
		if rule.RateLimit.Requests > 0 {
			if rule.RateLimit.Window == "" {
				rule.RateLimit.Window = "minute"
			}
			if _, ok := allowedRateLimitWindows[rule.RateLimit.Window]; !ok {
				return Policy{}, fmt.Errorf("unsupported rate_limit.window %q for %s", rule.RateLimit.Window, rule.Name)
			}
			if rule.RateLimit.Scope == "" {
				rule.RateLimit.Scope = "tool_identity"
			}
			if _, ok := allowedRateLimitScopes[rule.RateLimit.Scope]; !ok {
				return Policy{}, fmt.Errorf("unsupported rate_limit.scope %q for %s", rule.RateLimit.Scope, rule.Name)
			}
		}
		rule.Dataflow.TaintedSources = normalizeStringListLower(rule.Dataflow.TaintedSources)
		rule.Dataflow.DestinationKinds = normalizeStringListLower(rule.Dataflow.DestinationKinds)
		rule.Dataflow.DestinationValues = normalizeStringList(rule.Dataflow.DestinationValues)
		rule.Dataflow.DestinationOperations = normalizeStringListLower(rule.Dataflow.DestinationOperations)
		rule.Dataflow.Action = strings.ToLower(strings.TrimSpace(rule.Dataflow.Action))
		rule.Dataflow.ReasonCode = strings.TrimSpace(rule.Dataflow.ReasonCode)
		rule.Dataflow.Violation = strings.TrimSpace(rule.Dataflow.Violation)
		if rule.Dataflow.Enabled ||
			len(rule.Dataflow.TaintedSources) > 0 ||
			len(rule.Dataflow.DestinationKinds) > 0 ||
			len(rule.Dataflow.DestinationValues) > 0 ||
			len(rule.Dataflow.DestinationOperations) > 0 {
			rule.Dataflow.Enabled = true
			if len(rule.Dataflow.TaintedSources) == 0 {
				rule.Dataflow.TaintedSources = []string{"external", "tool_output"}
			}
			if rule.Dataflow.Action == "" {
				rule.Dataflow.Action = "require_approval"
			}
			if _, ok := allowedDataflowActions[rule.Dataflow.Action]; !ok {
				return Policy{}, fmt.Errorf("unsupported dataflow.action %q for %s", rule.Dataflow.Action, rule.Name)
			}
			if rule.Dataflow.ReasonCode == "" {
				rule.Dataflow.ReasonCode = "dataflow_tainted_destination"
			}
			if rule.Dataflow.Violation == "" {
				rule.Dataflow.Violation = "tainted_dataflow"
			}
		}
	}

	sort.Slice(output.Rules, func(i, j int) bool {
		if output.Rules[i].Priority != output.Rules[j].Priority {
			return output.Rules[i].Priority < output.Rules[j].Priority
		}
		return output.Rules[i].Name < output.Rules[j].Name
	})
	return output, nil
}

func ruleMatches(match PolicyMatch, intent schemagate.IntentRequest) bool {
	if len(match.ToolNames) > 0 && !contains(match.ToolNames, intent.ToolName) {
		return false
	}
	if len(match.RiskClasses) > 0 && !contains(match.RiskClasses, intent.Context.RiskClass) {
		return false
	}
	if len(match.Identities) > 0 && !contains(match.Identities, intent.Context.Identity) {
		return false
	}
	if len(match.WorkspacePrefixes) > 0 {
		workspaceMatched := false
		for _, prefix := range match.WorkspacePrefixes {
			if strings.HasPrefix(intent.Context.Workspace, prefix) {
				workspaceMatched = true
				break
			}
		}
		if !workspaceMatched {
			return false
		}
	}
	if len(match.TargetKinds) > 0 {
		targetKindMatched := false
		for _, target := range intent.Targets {
			if contains(match.TargetKinds, target.Kind) {
				targetKindMatched = true
				break
			}
		}
		if !targetKindMatched {
			return false
		}
	}
	if len(match.TargetValues) > 0 {
		targetValueMatched := false
		for _, target := range intent.Targets {
			if contains(match.TargetValues, target.Value) {
				targetValueMatched = true
				break
			}
		}
		if !targetValueMatched {
			return false
		}
	}
	if len(match.DataClasses) > 0 {
		dataClassMatched := false
		for _, target := range intent.Targets {
			if contains(match.DataClasses, target.Sensitivity) {
				dataClassMatched = true
				break
			}
		}
		if !dataClassMatched {
			return false
		}
	}
	if len(match.DestinationKinds) > 0 {
		destinationKindMatched := false
		for _, target := range intent.Targets {
			if contains(match.DestinationKinds, target.Kind) {
				destinationKindMatched = true
				break
			}
		}
		if !destinationKindMatched {
			return false
		}
	}
	if len(match.DestinationValues) > 0 {
		destinationValueMatched := false
		for _, target := range intent.Targets {
			if contains(match.DestinationValues, target.Value) {
				destinationValueMatched = true
				break
			}
		}
		if !destinationValueMatched {
			return false
		}
	}
	if len(match.DestinationOps) > 0 {
		destinationOpsMatched := false
		for _, target := range intent.Targets {
			if contains(match.DestinationOps, target.Operation) {
				destinationOpsMatched = true
				break
			}
		}
		if !destinationOpsMatched {
			return false
		}
	}
	if len(match.ProvenanceSources) > 0 {
		provenanceMatched := false
		for _, provenance := range intent.ArgProvenance {
			if contains(match.ProvenanceSources, provenance.Source) {
				provenanceMatched = true
				break
			}
		}
		if !provenanceMatched {
			return false
		}
	}
	return true
}

func evaluateDataflowConstraint(dataflow DataflowPolicy, intent schemagate.IntentRequest) (bool, string, []string, []string) {
	if !dataflow.Enabled {
		return false, "", nil, nil
	}
	if !hasTaintedProvenance(intent.ArgProvenance, dataflow.TaintedSources) {
		return false, "", nil, nil
	}
	if !matchesDataflowDestination(dataflow, intent.Targets) {
		return false, "", nil, nil
	}
	return true, dataflow.Action, []string{dataflow.ReasonCode}, []string{dataflow.Violation}
}

func hasTaintedProvenance(provenance []schemagate.IntentArgProvenance, taintedSources []string) bool {
	for _, entry := range provenance {
		if contains(taintedSources, entry.Source) {
			return true
		}
	}
	return false
}

func matchesDataflowDestination(dataflow DataflowPolicy, targets []schemagate.IntentTarget) bool {
	if len(targets) == 0 {
		return false
	}
	if len(dataflow.DestinationKinds) == 0 && len(dataflow.DestinationValues) == 0 && len(dataflow.DestinationOperations) == 0 {
		for _, target := range targets {
			if isDefaultEgressTargetKind(target.Kind) {
				return true
			}
		}
		return false
	}

	for _, target := range targets {
		if len(dataflow.DestinationKinds) > 0 && !contains(dataflow.DestinationKinds, target.Kind) {
			continue
		}
		if len(dataflow.DestinationValues) > 0 && !contains(dataflow.DestinationValues, target.Value) {
			continue
		}
		if len(dataflow.DestinationOperations) > 0 && !contains(dataflow.DestinationOperations, target.Operation) {
			continue
		}
		return true
	}
	return false
}

func isDefaultEgressTargetKind(kind string) bool {
	switch kind {
	case "host", "url", "bucket", "queue", "topic":
		return true
	default:
		return false
	}
}

func shouldFailClosed(policy FailClosedPolicy, riskClass string) bool {
	if !policy.Enabled {
		return false
	}
	if len(policy.RiskClasses) == 0 {
		return false
	}
	return contains(policy.RiskClasses, strings.ToLower(strings.TrimSpace(riskClass)))
}

func evaluateFailClosedRequiredFields(requiredFields []string, intent schemagate.IntentRequest) ([]string, []string) {
	reasons := make([]string, 0, len(requiredFields))
	violations := make([]string, 0, len(requiredFields))
	for _, field := range requiredFields {
		switch field {
		case "targets":
			if len(intent.Targets) == 0 {
				reasons = append(reasons, "fail_closed_missing_targets")
				violations = append(violations, "missing_targets")
			}
		case "arg_provenance":
			if len(intent.ArgProvenance) == 0 {
				reasons = append(reasons, "fail_closed_missing_arg_provenance")
				violations = append(violations, "missing_arg_provenance")
			}
		}
	}
	return uniqueSorted(reasons), uniqueSorted(violations)
}

func buildGateResult(
	_ Policy,
	intent schemagate.IntentRequest,
	opts EvalOptions,
	verdict string,
	reasonCodes []string,
	violations []string,
) schemagate.GateResult {
	createdAt := intent.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	producerVersion := opts.ProducerVersion
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	return schemagate.GateResult{
		SchemaID:        gateSchemaID,
		SchemaVersion:   gateSchemaV1,
		CreatedAt:       createdAt,
		ProducerVersion: producerVersion,
		Verdict:         verdict,
		ReasonCodes:     uniqueSorted(reasonCodes),
		Violations:      uniqueSorted(violations),
	}
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return uniqueSorted(out)
}

func normalizeStringListLower(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return uniqueSorted(out)
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func mergeUniqueSorted(values []string, extra []string) []string {
	merged := make([]string, 0, len(values)+len(extra))
	merged = append(merged, values...)
	merged = append(merged, extra...)
	return uniqueSorted(merged)
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func sanitizeName(value string) string {
	if value == "" {
		return "rule"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "rule"
	}
	clean := strings.Trim(string(raw), `"`)
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, "-", "_")
	return strings.ToLower(clean)
}
