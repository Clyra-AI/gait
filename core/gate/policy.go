package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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
	Name        string      `yaml:"name"`
	Priority    int         `yaml:"priority"`
	Effect      string      `yaml:"effect"`
	Match       PolicyMatch `yaml:"match"`
	ReasonCodes []string    `yaml:"reason_codes"`
	Violations  []string    `yaml:"violations"`
}

type PolicyMatch struct {
	ToolNames         []string `yaml:"tool_names"`
	RiskClasses       []string `yaml:"risk_classes"`
	TargetKinds       []string `yaml:"target_kinds"`
	TargetValues      []string `yaml:"target_values"`
	ProvenanceSources []string `yaml:"provenance_sources"`
	Identities        []string `yaml:"identities"`
	WorkspacePrefixes []string `yaml:"workspace_prefixes"`
}

type EvalOptions struct {
	ProducerVersion string
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
	normalizedPolicy, err := normalizePolicy(policy)
	if err != nil {
		return schemagate.GateResult{}, err
	}

	normalizedIntent, err := NormalizeIntent(intent)
	if err != nil {
		if shouldFailClosed(normalizedPolicy.FailClosed, strings.ToLower(strings.TrimSpace(intent.Context.RiskClass))) {
			return buildGateResult(normalizedPolicy, intent, opts, "block", []string{"fail_closed_intent_invalid"}, []string{"intent_not_evaluable"}), nil
		}
		return schemagate.GateResult{}, fmt.Errorf("normalize intent: %w", err)
	}

	if shouldFailClosed(normalizedPolicy.FailClosed, normalizedIntent.Context.RiskClass) {
		reasons, violations := evaluateFailClosedRequiredFields(normalizedPolicy.FailClosed.RequiredFields, normalizedIntent)
		if len(reasons) > 0 {
			return buildGateResult(normalizedPolicy, normalizedIntent, opts, "block", reasons, violations), nil
		}
	}

	for _, rule := range normalizedPolicy.Rules {
		if !ruleMatches(rule.Match, normalizedIntent) {
			continue
		}
		reasons := uniqueSorted(rule.ReasonCodes)
		if len(reasons) == 0 {
			reasons = []string{"matched_rule_" + sanitizeName(rule.Name)}
		}
		return buildGateResult(normalizedPolicy, normalizedIntent, opts, rule.Effect, reasons, uniqueSorted(rule.Violations)), nil
	}

	return buildGateResult(
		normalizedPolicy,
		normalizedIntent,
		opts,
		normalizedPolicy.DefaultVerdict,
		[]string{"default_" + normalizedPolicy.DefaultVerdict},
		[]string{},
	), nil
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
		rule.Match.ProvenanceSources = normalizeStringListLower(rule.Match.ProvenanceSources)
		rule.Match.Identities = normalizeStringList(rule.Match.Identities)
		rule.Match.WorkspacePrefixes = normalizeStringList(rule.Match.WorkspacePrefixes)
		rule.ReasonCodes = uniqueSorted(rule.ReasonCodes)
		rule.Violations = uniqueSorted(rule.Violations)
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
