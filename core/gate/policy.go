package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gaitjcs "github.com/Clyra-AI/proof/canon"
	"github.com/goccy/go-yaml"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const (
	policySchemaID = "gait.gate.policy"
	policySchemaV1 = "1.0.0"
	defaultVerdict = "require_approval"
	gateSchemaID   = "gait.gate.result"
	gateSchemaV1   = "1.0.0"
	maxInt64Uint64 = ^uint64(0) >> 1

	wrkrContextToolNameKey      = "wrkr.tool_name"
	wrkrContextDataClassKey     = "wrkr.data_class"
	wrkrContextEndpointClassKey = "wrkr.endpoint_class"
	wrkrContextAutonomyLevelKey = "wrkr.autonomy_level"
)

var (
	allowedVerdicts = map[string]struct{}{
		"allow":            {},
		"block":            {},
		"dry_run":          {},
		"require_approval": {},
	}
	allowedRequiredFields = map[string]struct{}{
		"targets":          {},
		"arg_provenance":   {},
		"endpoint_class":   {},
		"delegation":       {},
		"context_evidence": {},
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
	DefaultAction  string           `yaml:"default_action"`
	Scripts        ScriptPolicy     `yaml:"scripts"`
	FailClosed     FailClosedPolicy `yaml:"fail_closed"`
	Rules          []PolicyRule     `yaml:"rules"`
}

type ScriptPolicy struct {
	MaxSteps             int  `yaml:"max_steps"`
	RequireApprovalAbove int  `yaml:"require_approval_above"`
	BlockMixedRisk       bool `yaml:"block_mixed_risk"`
}

type FailClosedPolicy struct {
	Enabled        bool     `yaml:"enabled"`
	RiskClasses    []string `yaml:"risk_classes"`
	RequiredFields []string `yaml:"required_fields"`
}

type PolicyRule struct {
	Name                        string          `yaml:"name"`
	Priority                    int             `yaml:"priority"`
	Effect                      string          `yaml:"effect"`
	Action                      string          `yaml:"action"`
	Match                       PolicyMatch     `yaml:"match"`
	Endpoint                    EndpointPolicy  `yaml:"endpoint"`
	ReasonCodes                 []string        `yaml:"reason_codes"`
	Violations                  []string        `yaml:"violations"`
	MinApprovals                int             `yaml:"min_approvals"`
	RequireDistinctApprovers    bool            `yaml:"require_distinct_approvers"`
	RequireContextEvidence      bool            `yaml:"require_context_evidence"`
	RequiredContextEvidenceMode string          `yaml:"required_context_evidence_mode"`
	MaxContextAgeSeconds        int64           `yaml:"max_context_age_seconds"`
	RequireBrokerCredential     bool            `yaml:"require_broker_credential"`
	BrokerReference             string          `yaml:"broker_reference"`
	BrokerScopes                []string        `yaml:"broker_scopes"`
	RateLimit                   RateLimitPolicy `yaml:"rate_limit"`
	Dataflow                    DataflowPolicy  `yaml:"dataflow"`
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

type EndpointPolicy struct {
	Enabled           bool     `yaml:"enabled"`
	PathAllowlist     []string `yaml:"path_allowlist"`
	PathDenylist      []string `yaml:"path_denylist"`
	DomainAllowlist   []string `yaml:"domain_allowlist"`
	DomainDenylist    []string `yaml:"domain_denylist"`
	EgressClasses     []string `yaml:"egress_classes"`
	Action            string   `yaml:"action"`
	DestructiveAction string   `yaml:"destructive_action"`
	ReasonCode        string   `yaml:"reason_code"`
	Violation         string   `yaml:"violation"`
}

type PolicyMatch struct {
	ToolName                   string              `yaml:"tool_name"`
	ToolNames                  []string            `yaml:"tool_names"`
	RiskClasses                []string            `yaml:"risk_classes"`
	TargetKinds                []string            `yaml:"target_kinds"`
	TargetValues               []string            `yaml:"target_values"`
	EndpointClass              []string            `yaml:"endpoint_class"`
	EndpointClasses            []string            `yaml:"endpoint_classes"`
	DiscoveryMethods           []string            `yaml:"discovery_method"`
	ToolAnnotations            ToolAnnotationMatch `yaml:"tool_annotations"`
	SkillPublishers            []string            `yaml:"skill_publishers"`
	SkillSources               []string            `yaml:"skill_sources"`
	DataClasses                []string            `yaml:"data_classes"`
	DestinationKinds           []string            `yaml:"destination_kinds"`
	DestinationValues          []string            `yaml:"destination_values"`
	DestinationOps             []string            `yaml:"destination_operations"`
	ProvenanceSources          []string            `yaml:"provenance_sources"`
	Identities                 []string            `yaml:"identities"`
	WorkspacePrefixes          []string            `yaml:"workspace_prefixes"`
	ContextToolNames           []string            `yaml:"context_tool_names"`
	ContextDataClasses         []string            `yaml:"context_data_classes"`
	ContextEndpointClasses     []string            `yaml:"context_endpoint_classes"`
	ContextAutonomyLevels      []string            `yaml:"context_autonomy_levels"`
	RequireDelegation          bool                `yaml:"require_delegation"`
	AllowedDelegatorIdentities []string            `yaml:"allowed_delegator_identities"`
	AllowedDelegateIdentities  []string            `yaml:"allowed_delegate_identities"`
	DelegationScopes           []string            `yaml:"delegation_scopes"`
	MaxDelegationDepth         *int                `yaml:"max_delegation_depth"`
}

type ToolAnnotationMatch struct {
	ReadOnlyHint    *bool `yaml:"readOnlyHint"`
	DestructiveHint *bool `yaml:"destructiveHint"`
	IdempotentHint  *bool `yaml:"idempotentHint"`
	OpenWorldHint   *bool `yaml:"openWorldHint"`
}

type EvalOptions struct {
	ProducerVersion string
	WrkrInventory   map[string]WrkrToolMetadata
	WrkrSource      string
}

type EvalOutcome struct {
	Result                   schemagate.GateResult
	MatchedRule              string
	MinApprovals             int
	RequireDistinctApprovers bool
	RequireBrokerCredential  bool
	RequireDelegation        bool
	BrokerReference          string
	BrokerScopes             []string
	RateLimit                RateLimitPolicy
	DataflowTriggered        bool
	Script                   bool
	StepCount                int
	ScriptHash               string
	CompositeRiskClass       string
	StepVerdicts             []schemagate.TraceStepVerdict
	ContextSource            string
	PreApproved              bool
	PatternID                string
	RegistryReason           string
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
	if err := yaml.UnmarshalWithOptions(data, &policy, yaml.Strict(), yaml.DisallowUnknownField()); err != nil {
		formatted := strings.TrimSpace(yaml.FormatError(err, false, false))
		if formatted != "" {
			return Policy{}, fmt.Errorf("parse policy yaml: %s", formatted)
		}
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
		endpointReasons, endpointViolations := evaluateFailClosedEndpointClasses(normalizedIntent)
		reasons = mergeUniqueSorted(reasons, endpointReasons)
		violations = mergeUniqueSorted(violations, endpointViolations)
		if len(reasons) > 0 {
			return EvalOutcome{
				Result: buildGateResult(normalizedPolicy, normalizedIntent, opts, "block", reasons, violations),
			}, nil
		}
	}

	if normalizedIntent.Script != nil {
		return evaluateScriptPolicyDetailed(normalizedPolicy, normalizedIntent, opts)
	}
	enrichedIntent := normalizedIntent
	contextApplied := ApplyWrkrContext(&enrichedIntent, enrichedIntent.ToolName, opts.WrkrInventory)
	outcome, err := evaluateSingleIntent(normalizedPolicy, enrichedIntent, opts)
	if err != nil {
		return EvalOutcome{}, err
	}
	if contextApplied {
		outcome.ContextSource = resolveWrkrSource(opts.WrkrSource)
	}
	return outcome, nil
}

func evaluateSingleIntent(policy Policy, intent schemagate.IntentRequest, opts EvalOptions) (EvalOutcome, error) {
	for _, rule := range policy.Rules {
		if !ruleMatches(rule.Match, intent) {
			continue
		}
		effect := rule.Effect
		reasons := uniqueSorted(rule.ReasonCodes)
		violations := uniqueSorted(rule.Violations)
		if len(reasons) == 0 {
			reasons = []string{"matched_rule_" + sanitizeName(rule.Name)}
		}
		dataflowTriggered, dataflowEffect, dataflowReasons, dataflowViolations := evaluateDataflowConstraint(rule.Dataflow, intent)
		if dataflowTriggered {
			effect = dataflowEffect
			reasons = mergeUniqueSorted(reasons, dataflowReasons)
			violations = mergeUniqueSorted(violations, dataflowViolations)
		}
		endpointTriggered, endpointEffect, endpointReasons, endpointViolations := evaluateEndpointConstraint(rule.Endpoint, intent)
		if endpointTriggered {
			effect = mostRestrictiveVerdict(effect, endpointEffect)
			reasons = mergeUniqueSorted(reasons, endpointReasons)
			violations = mergeUniqueSorted(violations, endpointViolations)
		}
		contextTriggered, contextEffect, contextReasons, contextViolations := evaluateContextConstraint(rule, intent)
		if contextTriggered {
			effect = mostRestrictiveVerdict(effect, contextEffect)
			reasons = mergeUniqueSorted(reasons, contextReasons)
			violations = mergeUniqueSorted(violations, contextViolations)
		}
		minApprovals := rule.MinApprovals
		if effect == "require_approval" && minApprovals == 0 {
			minApprovals = 1
		}
		return EvalOutcome{
			Result:                   buildGateResult(policy, intent, opts, effect, reasons, violations),
			MatchedRule:              rule.Name,
			MinApprovals:             minApprovals,
			RequireDistinctApprovers: rule.RequireDistinctApprovers,
			RequireBrokerCredential:  rule.RequireBrokerCredential,
			RequireDelegation:        rule.Match.RequireDelegation,
			BrokerReference:          rule.BrokerReference,
			BrokerScopes:             uniqueSorted(rule.BrokerScopes),
			RateLimit:                rule.RateLimit,
			DataflowTriggered:        dataflowTriggered,
		}, nil
	}

	minApprovals := 0
	if policy.DefaultVerdict == "require_approval" {
		minApprovals = 1
	}
	return EvalOutcome{
		Result: buildGateResult(
			policy,
			intent,
			opts,
			policy.DefaultVerdict,
			[]string{"default_" + policy.DefaultVerdict},
			[]string{},
		),
		MinApprovals: minApprovals,
	}, nil
}

func evaluateScriptPolicyDetailed(policy Policy, intent schemagate.IntentRequest, opts EvalOptions) (EvalOutcome, error) {
	if intent.Script == nil || len(intent.Script.Steps) == 0 {
		return EvalOutcome{}, fmt.Errorf("script intent requires at least one step")
	}
	stepVerdicts := make([]schemagate.TraceStepVerdict, 0, len(intent.Script.Steps))
	reasons := []string{}
	violations := []string{}
	matchedRules := []string{}
	verdict := "allow"
	minApprovals := 0
	requireDistinctApprovers := false
	requireBrokerCredential := false
	requireDelegation := false
	brokerScopes := []string{}
	brokerReference := ""
	dataflowTriggered := false
	riskClasses := []string{}
	aggregatedRateLimit := RateLimitPolicy{}
	contextSource := ""

	for index, step := range intent.Script.Steps {
		stepIntent := intent
		stepIntent.Context = intent.Context
		stepIntent.Context.AuthContext = cloneAuthContext(intent.Context.AuthContext)
		stepIntent.Script = nil
		stepIntent.ScriptHash = ""
		stepIntent.ToolName = step.ToolName
		stepIntent.Args = step.Args
		stepIntent.Targets = step.Targets
		stepIntent.ArgProvenance = step.ArgProvenance
		contextApplied := ApplyWrkrContext(&stepIntent, step.ToolName, opts.WrkrInventory)

		stepOutcome, err := evaluateSingleIntent(policy, stepIntent, opts)
		if err != nil {
			return EvalOutcome{}, err
		}
		stepVerdicts = append(stepVerdicts, schemagate.TraceStepVerdict{
			Index:       index,
			ToolName:    step.ToolName,
			Verdict:     stepOutcome.Result.Verdict,
			ReasonCodes: mergeUniqueSorted(nil, stepOutcome.Result.ReasonCodes),
			Violations:  mergeUniqueSorted(nil, stepOutcome.Result.Violations),
			MatchedRule: stepOutcome.MatchedRule,
		})

		verdict = mostRestrictiveVerdict(verdict, stepOutcome.Result.Verdict)
		reasons = mergeUniqueSorted(reasons, stepOutcome.Result.ReasonCodes)
		violations = mergeUniqueSorted(violations, stepOutcome.Result.Violations)
		if stepOutcome.MatchedRule != "" {
			matchedRules = append(matchedRules, stepOutcome.MatchedRule)
		}
		if stepOutcome.MinApprovals > minApprovals {
			minApprovals = stepOutcome.MinApprovals
		}
		if stepOutcome.RequireDistinctApprovers {
			requireDistinctApprovers = true
		}
		if stepOutcome.RequireBrokerCredential {
			requireBrokerCredential = true
		}
		if stepOutcome.RequireDelegation {
			requireDelegation = true
		}
		if brokerReference == "" {
			brokerReference = stepOutcome.BrokerReference
		}
		brokerScopes = mergeUniqueSorted(brokerScopes, stepOutcome.BrokerScopes)
		aggregatedRateLimit = mergeRateLimitPolicy(aggregatedRateLimit, stepOutcome.RateLimit)
		if stepOutcome.DataflowTriggered {
			dataflowTriggered = true
		}
		riskClasses = mergeUniqueSorted(riskClasses, []string{classifyScriptStepRisk(step.Targets)})
		if contextApplied {
			contextSource = resolveWrkrSource(opts.WrkrSource)
		}
	}

	maxSteps := policy.Scripts.MaxSteps
	if maxSteps <= 0 {
		maxSteps = maxScriptSteps
	}
	if len(intent.Script.Steps) > maxSteps {
		verdict = "block"
		reasons = mergeUniqueSorted(reasons, []string{"script_max_steps_exceeded"})
		violations = mergeUniqueSorted(violations, []string{"script_max_steps_exceeded"})
	}
	if policy.Scripts.RequireApprovalAbove > 0 && len(intent.Script.Steps) > policy.Scripts.RequireApprovalAbove {
		verdict = mostRestrictiveVerdict(verdict, "require_approval")
		reasons = mergeUniqueSorted(reasons, []string{"script_step_threshold_approval"})
		if minApprovals == 0 {
			minApprovals = 1
		}
	}
	if policy.Scripts.BlockMixedRisk && len(riskClasses) > 1 {
		verdict = "block"
		reasons = mergeUniqueSorted(reasons, []string{"script_mixed_risk_blocked"})
		violations = mergeUniqueSorted(violations, []string{"script_mixed_risk"})
	}

	return EvalOutcome{
		Result: buildGateResult(
			policy,
			intent,
			opts,
			verdict,
			reasons,
			violations,
		),
		MatchedRule:              strings.Join(uniqueSorted(matchedRules), ","),
		MinApprovals:             minApprovals,
		RequireDistinctApprovers: requireDistinctApprovers,
		RequireBrokerCredential:  requireBrokerCredential,
		RequireDelegation:        requireDelegation,
		BrokerReference:          brokerReference,
		BrokerScopes:             uniqueSorted(brokerScopes),
		RateLimit:                aggregatedRateLimit,
		DataflowTriggered:        dataflowTriggered,
		Script:                   true,
		StepCount:                len(intent.Script.Steps),
		ScriptHash:               intent.ScriptHash,
		CompositeRiskClass:       compositeRiskClass(riskClasses),
		StepVerdicts:             stepVerdicts,
		ContextSource:            contextSource,
	}, nil
}

func mergeRateLimitPolicy(current RateLimitPolicy, candidate RateLimitPolicy) RateLimitPolicy {
	if candidate.Requests <= 0 {
		return current
	}
	if current.Requests <= 0 {
		return candidate
	}
	if candidate.Requests < current.Requests {
		return candidate
	}
	if candidate.Requests > current.Requests {
		return current
	}
	if normalizedWindowPriority(candidate.Window) < normalizedWindowPriority(current.Window) {
		return candidate
	}
	if normalizedWindowPriority(candidate.Window) > normalizedWindowPriority(current.Window) {
		return current
	}
	if strings.ToLower(strings.TrimSpace(candidate.Scope)) < strings.ToLower(strings.TrimSpace(current.Scope)) {
		return candidate
	}
	return current
}

func cloneAuthContext(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func normalizedWindowPriority(window string) int {
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "minute":
		return 0
	case "hour":
		return 1
	default:
		return 2
	}
}

func classifyScriptStepRisk(targets []schemagate.IntentTarget) string {
	risk := "low"
	for _, target := range targets {
		switch target.EndpointClass {
		case "fs.delete", "proc.exec":
			return "high"
		case "fs.write", "net.http", "net.dns":
			if risk == "low" {
				risk = "medium"
			}
		}
		if target.Destructive {
			return "high"
		}
	}
	return risk
}

func compositeRiskClass(riskClasses []string) string {
	if contains(riskClasses, "high") {
		return "high"
	}
	if contains(riskClasses, "medium") {
		return "medium"
	}
	return "low"
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
		if len(rule.Match.EndpointClasses) > 0 {
			matchPayload["EndpointClasses"] = rule.Match.EndpointClasses
		}
		if len(rule.Match.DiscoveryMethods) > 0 {
			matchPayload["DiscoveryMethods"] = rule.Match.DiscoveryMethods
		}
		if toolAnnotationsPayload, ok := toolAnnotationDigestPayload(rule.Match.ToolAnnotations); ok {
			matchPayload["ToolAnnotations"] = toolAnnotationsPayload
		}
		if len(rule.Match.SkillPublishers) > 0 {
			matchPayload["SkillPublishers"] = rule.Match.SkillPublishers
		}
		if len(rule.Match.SkillSources) > 0 {
			matchPayload["SkillSources"] = rule.Match.SkillSources
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
		if len(rule.Match.ContextToolNames) > 0 {
			matchPayload["ContextToolNames"] = rule.Match.ContextToolNames
		}
		if len(rule.Match.ContextDataClasses) > 0 {
			matchPayload["ContextDataClasses"] = rule.Match.ContextDataClasses
		}
		if len(rule.Match.ContextEndpointClasses) > 0 {
			matchPayload["ContextEndpointClasses"] = rule.Match.ContextEndpointClasses
		}
		if len(rule.Match.ContextAutonomyLevels) > 0 {
			matchPayload["ContextAutonomyLevels"] = rule.Match.ContextAutonomyLevels
		}
		if rule.Match.RequireDelegation {
			matchPayload["RequireDelegation"] = true
		}
		if len(rule.Match.AllowedDelegatorIdentities) > 0 {
			matchPayload["AllowedDelegatorIdentities"] = rule.Match.AllowedDelegatorIdentities
		}
		if len(rule.Match.AllowedDelegateIdentities) > 0 {
			matchPayload["AllowedDelegateIdentities"] = rule.Match.AllowedDelegateIdentities
		}
		if len(rule.Match.DelegationScopes) > 0 {
			matchPayload["DelegationScopes"] = rule.Match.DelegationScopes
		}
		if rule.Match.MaxDelegationDepth != nil {
			matchPayload["MaxDelegationDepth"] = *rule.Match.MaxDelegationDepth
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
		if rule.RequireContextEvidence {
			rulePayload["RequireContextEvidence"] = rule.RequireContextEvidence
		}
		if rule.RequiredContextEvidenceMode != "" {
			rulePayload["RequiredContextEvidenceMode"] = rule.RequiredContextEvidenceMode
		}
		if rule.MaxContextAgeSeconds > 0 {
			rulePayload["MaxContextAgeSeconds"] = rule.MaxContextAgeSeconds
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
		if rule.Endpoint.Enabled {
			endpointPayload := map[string]any{
				"Enabled":           rule.Endpoint.Enabled,
				"PathAllowlist":     rule.Endpoint.PathAllowlist,
				"PathDenylist":      rule.Endpoint.PathDenylist,
				"DomainAllowlist":   rule.Endpoint.DomainAllowlist,
				"DomainDenylist":    rule.Endpoint.DomainDenylist,
				"EgressClasses":     rule.Endpoint.EgressClasses,
				"Action":            rule.Endpoint.Action,
				"DestructiveAction": rule.Endpoint.DestructiveAction,
				"ReasonCode":        rule.Endpoint.ReasonCode,
				"Violation":         rule.Endpoint.Violation,
			}
			rulePayload["Endpoint"] = endpointPayload
		}
		rules = append(rules, rulePayload)
	}

	payload := map[string]any{
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
	if policy.Scripts.MaxSteps > 0 || policy.Scripts.RequireApprovalAbove > 0 || policy.Scripts.BlockMixedRisk {
		payload["Scripts"] = map[string]any{
			"MaxSteps":             policy.Scripts.MaxSteps,
			"RequireApprovalAbove": policy.Scripts.RequireApprovalAbove,
			"BlockMixedRisk":       policy.Scripts.BlockMixedRisk,
		}
	}
	return payload
}

func toolAnnotationDigestPayload(annotations ToolAnnotationMatch) (map[string]any, bool) {
	payload := map[string]any{}
	if annotations.ReadOnlyHint != nil {
		payload["ReadOnlyHint"] = *annotations.ReadOnlyHint
	}
	if annotations.DestructiveHint != nil {
		payload["DestructiveHint"] = *annotations.DestructiveHint
	}
	if annotations.IdempotentHint != nil {
		payload["IdempotentHint"] = *annotations.IdempotentHint
	}
	if annotations.OpenWorldHint != nil {
		payload["OpenWorldHint"] = *annotations.OpenWorldHint
	}
	if len(payload) == 0 {
		return nil, false
	}
	return payload, true
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
	defaultAction := strings.ToLower(strings.TrimSpace(output.DefaultAction))
	if output.DefaultVerdict == "" {
		output.DefaultVerdict = defaultAction
	}
	if defaultAction != "" && output.DefaultVerdict != defaultAction {
		return Policy{}, fmt.Errorf("default_action must match default_verdict when both are set")
	}
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
	if output.Scripts.MaxSteps < 0 {
		return Policy{}, fmt.Errorf("scripts.max_steps must be >= 0")
	}
	if output.Scripts.RequireApprovalAbove < 0 {
		return Policy{}, fmt.Errorf("scripts.require_approval_above must be >= 0")
	}

	output.Rules = append([]PolicyRule(nil), output.Rules...)
	for index := range output.Rules {
		rule := &output.Rules[index]
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			return Policy{}, fmt.Errorf("rule name is required")
		}

		rule.Effect = strings.ToLower(strings.TrimSpace(rule.Effect))
		action := strings.ToLower(strings.TrimSpace(rule.Action))
		if rule.Effect == "" {
			rule.Effect = action
		}
		if action != "" && rule.Effect != action {
			return Policy{}, fmt.Errorf("rule action must match effect for %s", rule.Name)
		}
		if rule.Effect == "" {
			return Policy{}, fmt.Errorf("rule effect is required for %s", rule.Name)
		}
		if _, ok := allowedVerdicts[rule.Effect]; !ok {
			return Policy{}, fmt.Errorf("invalid rule effect %q for %s", rule.Effect, rule.Name)
		}

		if aliasToolName := strings.TrimSpace(rule.Match.ToolName); aliasToolName != "" {
			rule.Match.ToolNames = append(rule.Match.ToolNames, aliasToolName)
		}
		rule.Match.ToolNames = normalizeStringListLower(rule.Match.ToolNames)
		rule.Match.RiskClasses = normalizeStringListLower(rule.Match.RiskClasses)
		rule.Match.TargetKinds = normalizeStringListLower(rule.Match.TargetKinds)
		rule.Match.TargetValues = normalizeStringList(rule.Match.TargetValues)
		rule.Match.EndpointClasses = append(rule.Match.EndpointClasses, rule.Match.EndpointClass...)
		rule.Match.EndpointClasses = normalizeStringListLower(rule.Match.EndpointClasses)
		for _, endpointClass := range rule.Match.EndpointClasses {
			if _, ok := allowedEndpointClasses[endpointClass]; !ok {
				return Policy{}, fmt.Errorf("unsupported match endpoint_class %q for %s", endpointClass, rule.Name)
			}
		}
		rule.Match.DiscoveryMethods = normalizeStringListLower(rule.Match.DiscoveryMethods)
		for _, discoveryMethod := range rule.Match.DiscoveryMethods {
			if _, ok := allowedDiscoveryMethods[discoveryMethod]; !ok {
				return Policy{}, fmt.Errorf("unsupported match discovery_method %q for %s", discoveryMethod, rule.Name)
			}
		}
		rule.Match.SkillPublishers = normalizeStringListLower(rule.Match.SkillPublishers)
		rule.Match.SkillSources = normalizeStringListLower(rule.Match.SkillSources)
		rule.Match.DataClasses = normalizeStringListLower(rule.Match.DataClasses)
		rule.Match.DestinationKinds = normalizeStringListLower(rule.Match.DestinationKinds)
		rule.Match.DestinationValues = normalizeStringList(rule.Match.DestinationValues)
		rule.Match.DestinationOps = normalizeStringListLower(rule.Match.DestinationOps)
		rule.Match.ProvenanceSources = normalizeStringListLower(rule.Match.ProvenanceSources)
		rule.Match.Identities = normalizeStringList(rule.Match.Identities)
		rule.Match.WorkspacePrefixes = normalizeStringList(rule.Match.WorkspacePrefixes)
		rule.Match.ContextToolNames = normalizeStringListLower(rule.Match.ContextToolNames)
		rule.Match.ContextDataClasses = normalizeStringListLower(rule.Match.ContextDataClasses)
		rule.Match.ContextEndpointClasses = normalizeStringListLower(rule.Match.ContextEndpointClasses)
		rule.Match.ContextAutonomyLevels = normalizeStringListLower(rule.Match.ContextAutonomyLevels)
		rule.Match.AllowedDelegatorIdentities = normalizeStringList(rule.Match.AllowedDelegatorIdentities)
		rule.Match.AllowedDelegateIdentities = normalizeStringList(rule.Match.AllowedDelegateIdentities)
		rule.Match.DelegationScopes = normalizeStringListLower(rule.Match.DelegationScopes)
		if rule.Match.MaxDelegationDepth != nil && *rule.Match.MaxDelegationDepth < 0 {
			return Policy{}, fmt.Errorf("max_delegation_depth must be >= 0 for %s", rule.Name)
		}
		rule.ReasonCodes = uniqueSorted(rule.ReasonCodes)
		rule.Violations = uniqueSorted(rule.Violations)
		if rule.MinApprovals < 0 {
			return Policy{}, fmt.Errorf("min_approvals must be >= 0 for %s", rule.Name)
		}
		rule.RequiredContextEvidenceMode = strings.ToLower(strings.TrimSpace(rule.RequiredContextEvidenceMode))
		if rule.RequiredContextEvidenceMode != "" && rule.RequiredContextEvidenceMode != "required" {
			return Policy{}, fmt.Errorf("required_context_evidence_mode must be required for %s", rule.Name)
		}
		if rule.RequireContextEvidence && rule.RequiredContextEvidenceMode == "" {
			rule.RequiredContextEvidenceMode = "required"
		}
		if rule.MaxContextAgeSeconds < 0 {
			return Policy{}, fmt.Errorf("max_context_age_seconds must be >= 0 for %s", rule.Name)
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
		rule.Endpoint.PathAllowlist = normalizePathPatterns(rule.Endpoint.PathAllowlist)
		rule.Endpoint.PathDenylist = normalizePathPatterns(rule.Endpoint.PathDenylist)
		rule.Endpoint.DomainAllowlist = normalizeStringListLower(rule.Endpoint.DomainAllowlist)
		rule.Endpoint.DomainDenylist = normalizeStringListLower(rule.Endpoint.DomainDenylist)
		rule.Endpoint.EgressClasses = normalizeStringListLower(rule.Endpoint.EgressClasses)
		for _, endpointClass := range rule.Endpoint.EgressClasses {
			if _, ok := allowedEndpointClasses[endpointClass]; !ok {
				return Policy{}, fmt.Errorf("unsupported endpoint.egress_class %q for %s", endpointClass, rule.Name)
			}
			if !strings.HasPrefix(endpointClass, "net.") {
				return Policy{}, fmt.Errorf("endpoint.egress_class must be network class for %s", rule.Name)
			}
		}
		rule.Endpoint.Action = strings.ToLower(strings.TrimSpace(rule.Endpoint.Action))
		rule.Endpoint.DestructiveAction = strings.ToLower(strings.TrimSpace(rule.Endpoint.DestructiveAction))
		rule.Endpoint.ReasonCode = strings.TrimSpace(rule.Endpoint.ReasonCode)
		rule.Endpoint.Violation = strings.TrimSpace(rule.Endpoint.Violation)
		if rule.Endpoint.Enabled ||
			len(rule.Endpoint.PathAllowlist) > 0 ||
			len(rule.Endpoint.PathDenylist) > 0 ||
			len(rule.Endpoint.DomainAllowlist) > 0 ||
			len(rule.Endpoint.DomainDenylist) > 0 ||
			len(rule.Endpoint.EgressClasses) > 0 ||
			rule.Endpoint.DestructiveAction != "" {
			rule.Endpoint.Enabled = true
			if rule.Endpoint.Action == "" {
				rule.Endpoint.Action = "block"
			}
			if _, ok := allowedDataflowActions[rule.Endpoint.Action]; !ok {
				return Policy{}, fmt.Errorf("unsupported endpoint.action %q for %s", rule.Endpoint.Action, rule.Name)
			}
			if rule.Endpoint.DestructiveAction != "" {
				if _, ok := allowedDataflowActions[rule.Endpoint.DestructiveAction]; !ok {
					return Policy{}, fmt.Errorf("unsupported endpoint.destructive_action %q for %s", rule.Endpoint.DestructiveAction, rule.Name)
				}
			}
			if rule.Endpoint.ReasonCode == "" {
				rule.Endpoint.ReasonCode = "endpoint_constraint_violation"
			}
			if rule.Endpoint.Violation == "" {
				rule.Endpoint.Violation = "endpoint_constraint_violation"
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
	if len(match.ContextToolNames) > 0 {
		value := contextString(intent.Context.AuthContext, wrkrContextToolNameKey)
		if value == "" || !contains(match.ContextToolNames, value) {
			return false
		}
	}
	if len(match.ContextDataClasses) > 0 {
		value := contextString(intent.Context.AuthContext, wrkrContextDataClassKey)
		if value == "" || !contains(match.ContextDataClasses, value) {
			return false
		}
	}
	if len(match.ContextEndpointClasses) > 0 {
		value := contextString(intent.Context.AuthContext, wrkrContextEndpointClassKey)
		if value == "" || !contains(match.ContextEndpointClasses, value) {
			return false
		}
	}
	if len(match.ContextAutonomyLevels) > 0 {
		value := contextString(intent.Context.AuthContext, wrkrContextAutonomyLevelKey)
		if value == "" || !contains(match.ContextAutonomyLevels, value) {
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
	if len(match.EndpointClasses) > 0 {
		endpointClassMatched := false
		for _, target := range intent.Targets {
			if contains(match.EndpointClasses, target.EndpointClass) {
				endpointClassMatched = true
				break
			}
		}
		if !endpointClassMatched {
			return false
		}
	}
	if len(match.DiscoveryMethods) > 0 {
		discoveryMethodMatched := false
		for _, target := range intent.Targets {
			if contains(match.DiscoveryMethods, target.DiscoveryMethod) {
				discoveryMethodMatched = true
				break
			}
		}
		if !discoveryMethodMatched {
			return false
		}
	}
	if !toolAnnotationsMatch(match.ToolAnnotations, intent.Targets) {
		return false
	}
	if len(match.SkillPublishers) > 0 {
		if intent.SkillProvenance == nil || !contains(match.SkillPublishers, strings.ToLower(strings.TrimSpace(intent.SkillProvenance.Publisher))) {
			return false
		}
	}
	if len(match.SkillSources) > 0 {
		if intent.SkillProvenance == nil || !contains(match.SkillSources, strings.ToLower(strings.TrimSpace(intent.SkillProvenance.Source))) {
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
	if !delegationMatches(match, intent.Delegation) {
		return false
	}
	return true
}

func toolAnnotationsMatch(match ToolAnnotationMatch, targets []schemagate.IntentTarget) bool {
	if match.ReadOnlyHint != nil {
		matched := false
		for _, target := range targets {
			if target.ReadOnlyHint == *match.ReadOnlyHint {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if match.DestructiveHint != nil {
		matched := false
		for _, target := range targets {
			if target.DestructiveHint == *match.DestructiveHint {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if match.IdempotentHint != nil {
		matched := false
		for _, target := range targets {
			if target.IdempotentHint == *match.IdempotentHint {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if match.OpenWorldHint != nil {
		matched := false
		for _, target := range targets {
			if target.OpenWorldHint == *match.OpenWorldHint {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func delegationMatches(match PolicyMatch, delegation *schemagate.IntentDelegation) bool {
	delegationConstrained := match.RequireDelegation ||
		len(match.AllowedDelegatorIdentities) > 0 ||
		len(match.AllowedDelegateIdentities) > 0 ||
		len(match.DelegationScopes) > 0 ||
		match.MaxDelegationDepth != nil
	if !delegationConstrained {
		return true
	}
	if delegation == nil {
		return false
	}

	if match.MaxDelegationDepth != nil && len(delegation.Chain) > *match.MaxDelegationDepth {
		return false
	}

	if len(match.AllowedDelegatorIdentities) > 0 {
		matchedDelegator := false
		for _, link := range delegation.Chain {
			if contains(match.AllowedDelegatorIdentities, strings.TrimSpace(link.DelegatorIdentity)) {
				matchedDelegator = true
				break
			}
		}
		if !matchedDelegator {
			return false
		}
	}

	if len(match.AllowedDelegateIdentities) > 0 {
		matchedDelegate := contains(match.AllowedDelegateIdentities, strings.TrimSpace(delegation.RequesterIdentity))
		if !matchedDelegate {
			for _, link := range delegation.Chain {
				if contains(match.AllowedDelegateIdentities, strings.TrimSpace(link.DelegateIdentity)) {
					matchedDelegate = true
					break
				}
			}
		}
		if !matchedDelegate {
			return false
		}
	}

	if len(match.DelegationScopes) > 0 {
		matchedScope := contains(match.DelegationScopes, strings.ToLower(strings.TrimSpace(delegation.ScopeClass)))
		if !matchedScope {
			for _, link := range delegation.Chain {
				if contains(match.DelegationScopes, strings.ToLower(strings.TrimSpace(link.ScopeClass))) {
					matchedScope = true
					break
				}
			}
		}
		if !matchedScope {
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
		case "endpoint_class":
			for _, target := range intent.Targets {
				if strings.TrimSpace(target.EndpointClass) == "" || strings.TrimSpace(target.EndpointClass) == "other" {
					reasons = append(reasons, "fail_closed_missing_endpoint_class")
					violations = append(violations, "missing_endpoint_class")
					break
				}
			}
		case "delegation":
			if intent.Delegation == nil {
				reasons = append(reasons, "fail_closed_missing_delegation")
				violations = append(violations, "missing_delegation")
			}
		case "context_evidence":
			if strings.TrimSpace(intent.Context.ContextSetDigest) == "" {
				reasons = append(reasons, "context_evidence_missing")
				violations = append(violations, "missing_context_evidence")
			}
		}
	}
	return uniqueSorted(reasons), uniqueSorted(violations)
}

func evaluateFailClosedEndpointClasses(intent schemagate.IntentRequest) ([]string, []string) {
	for _, target := range intent.Targets {
		if strings.TrimSpace(target.EndpointClass) == "" || strings.TrimSpace(target.EndpointClass) == "other" {
			return []string{"fail_closed_endpoint_class_unknown"}, []string{"endpoint_class_unknown"}
		}
	}
	return []string{}, []string{}
}

func evaluateContextConstraint(rule PolicyRule, intent schemagate.IntentRequest) (bool, string, []string, []string) {
	contextRequired := rule.RequireContextEvidence || rule.RequiredContextEvidenceMode == "required" || rule.MaxContextAgeSeconds > 0
	if !contextRequired {
		return false, "", nil, nil
	}
	reasons := make([]string, 0, 3)
	violations := make([]string, 0, 3)
	contextDigest := strings.TrimSpace(intent.Context.ContextSetDigest)
	if contextDigest == "" {
		reasons = append(reasons, "context_evidence_missing", "context_set_digest_missing")
		violations = append(violations, "missing_context_evidence")
	}
	if rule.RequiredContextEvidenceMode == "required" {
		if strings.TrimSpace(intent.Context.ContextEvidenceMode) != "required" {
			reasons = append(reasons, "context_evidence_mode_mismatch")
			violations = append(violations, "context_evidence_mode_violation")
		}
	}
	if rule.MaxContextAgeSeconds > 0 {
		ageSeconds, ok := contextAgeSeconds(intent.Context.AuthContext)
		if !ok || ageSeconds > rule.MaxContextAgeSeconds {
			reasons = append(reasons, "context_freshness_exceeded")
			violations = append(violations, "context_freshness_violation")
		}
	}
	if len(reasons) == 0 {
		return false, "", nil, nil
	}
	return true, "block", uniqueSorted(reasons), uniqueSorted(violations)
}

func contextAgeSeconds(authContext map[string]any) (int64, bool) {
	if len(authContext) == 0 {
		return 0, false
	}
	value, ok := authContext["context_age_seconds"]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		return contextAgeFromUnsigned(uint64(typed))
	case uint8:
		return contextAgeFromUnsigned(uint64(typed))
	case uint16:
		return contextAgeFromUnsigned(uint64(typed))
	case uint32:
		return contextAgeFromUnsigned(uint64(typed))
	case uint64:
		return contextAgeFromUnsigned(typed)
	case float32:
		if typed < 0 {
			return 0, false
		}
		return int64(typed), true
	case float64:
		if typed < 0 {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < 0 {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := json.Number(strings.TrimSpace(typed)).Int64()
		if err != nil || parsed < 0 {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func contextAgeFromUnsigned(value uint64) (int64, bool) {
	if value > maxInt64Uint64 {
		return 0, false
	}
	return int64(value), true
}

func evaluateEndpointConstraint(endpoint EndpointPolicy, intent schemagate.IntentRequest) (bool, string, []string, []string) {
	if !endpoint.Enabled {
		return false, "", nil, nil
	}
	reasons := []string{}
	violations := []string{}

	for _, target := range intent.Targets {
		if target.Kind == "path" {
			normalizedPath := filepath.ToSlash(strings.TrimSpace(target.Value))
			if len(endpoint.PathDenylist) > 0 && matchesAnyPattern(normalizedPath, endpoint.PathDenylist) {
				reasons = append(reasons, "endpoint_path_denied")
				violations = append(violations, "endpoint_path_denied")
			}
			if len(endpoint.PathAllowlist) > 0 && !matchesAnyPattern(normalizedPath, endpoint.PathAllowlist) {
				reasons = append(reasons, "endpoint_path_not_allowlisted")
				violations = append(violations, "endpoint_path_not_allowlisted")
			}
		}

		domain := strings.ToLower(strings.TrimSpace(target.EndpointDomain))
		if domain != "" {
			if len(endpoint.DomainDenylist) > 0 && matchesAnyDomain(domain, endpoint.DomainDenylist) {
				reasons = append(reasons, "endpoint_domain_denied")
				violations = append(violations, "endpoint_domain_denied")
			}
			if len(endpoint.DomainAllowlist) > 0 && !matchesAnyDomain(domain, endpoint.DomainAllowlist) {
				reasons = append(reasons, "endpoint_domain_not_allowlisted")
				violations = append(violations, "endpoint_domain_not_allowlisted")
			}
		}

		if strings.HasPrefix(target.EndpointClass, "net.") && len(endpoint.EgressClasses) > 0 && !contains(endpoint.EgressClasses, target.EndpointClass) {
			reasons = append(reasons, "endpoint_egress_class_not_allowed")
			violations = append(violations, "endpoint_egress_class_not_allowed")
		}
	}

	effect := endpoint.Action
	if effect == "" {
		effect = "block"
	}
	if endpoint.DestructiveAction != "" && intentContainsDestructiveTarget(intent.Targets) {
		effect = mostRestrictiveVerdict(effect, endpoint.DestructiveAction)
		reasons = append(reasons, "endpoint_destructive_operation")
		violations = append(violations, "destructive_operation")
	}

	if len(reasons) == 0 {
		return false, "", nil, nil
	}
	if endpoint.ReasonCode != "" {
		reasons = append(reasons, endpoint.ReasonCode)
	}
	if endpoint.Violation != "" {
		violations = append(violations, endpoint.Violation)
	}
	return true, effect, uniqueSorted(reasons), uniqueSorted(violations)
}

func intentContainsDestructiveTarget(targets []schemagate.IntentTarget) bool {
	for _, target := range targets {
		if target.Destructive {
			return true
		}
	}
	return false
}

func matchesAnyPattern(value string, patterns []string) bool {
	for _, patternValue := range patterns {
		if matchPathPattern(value, patternValue) {
			return true
		}
	}
	return false
}

func normalizePathPatterns(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := filepath.ToSlash(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return uniqueSorted(out)
}

func matchPathPattern(value string, patternValue string) bool {
	if patternValue == "" {
		return false
	}
	normalizedValue := filepath.ToSlash(strings.TrimSpace(value))
	normalizedPattern := filepath.ToSlash(strings.TrimSpace(patternValue))
	if normalizedPattern == normalizedValue {
		return true
	}
	if strings.HasSuffix(normalizedPattern, "/**") {
		prefix := strings.TrimSuffix(normalizedPattern, "/**")
		if normalizedValue == prefix || strings.HasPrefix(normalizedValue, prefix+"/") {
			return true
		}
	}
	ok, err := path.Match(normalizedPattern, normalizedValue)
	return err == nil && ok
}

func matchesAnyDomain(value string, patterns []string) bool {
	for _, patternValue := range patterns {
		patternLower := strings.ToLower(strings.TrimSpace(patternValue))
		if patternLower == "" {
			continue
		}
		valueLower := strings.ToLower(strings.TrimSpace(value))
		if valueLower == patternLower {
			return true
		}
		if strings.HasPrefix(patternLower, "*.") && strings.HasSuffix(valueLower, strings.TrimPrefix(patternLower, "*")) {
			return true
		}
		ok, err := path.Match(patternLower, valueLower)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func mostRestrictiveVerdict(current string, candidate string) string {
	priority := map[string]int{
		"allow":            0,
		"dry_run":          1,
		"require_approval": 2,
		"block":            3,
	}
	currentPriority, ok := priority[current]
	if !ok {
		currentPriority = 0
	}
	candidatePriority, ok := priority[candidate]
	if !ok {
		candidatePriority = 0
	}
	if candidatePriority > currentPriority {
		return candidate
	}
	return current
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
	merged := append([]string{}, values...)
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

func contextString(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	raw, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(text))
}

func resolveWrkrSource(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "wrkr_inventory"
	}
	return trimmed
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
