package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/jcs"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestParsePolicyYAMLDefaultsAndSorting(t *testing.T) {
	policyYAML := []byte(`
rules:
  - name: allow-read
    priority: 20
    effect: allow
    match:
      tool_names: ["Tool.Read"]
      workspace_prefixes: [" /repo "]
    reason_codes: ["matched_allow"]
  - name: block-external
    priority: 10
    effect: block
    match:
      target_kinds: ["HOST"]
      target_values: ["api.external.com"]
    reason_codes: ["blocked_target"]
fail_closed:
  enabled: true
`)

	policy, err := ParsePolicyYAML(policyYAML)
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	if policy.SchemaID != policySchemaID || policy.SchemaVersion != policySchemaV1 {
		t.Fatalf("unexpected policy schema metadata: %#v", policy)
	}
	if policy.DefaultVerdict != defaultVerdict {
		t.Fatalf("expected default verdict %q, got %q", defaultVerdict, policy.DefaultVerdict)
	}
	if !policy.FailClosed.Enabled {
		t.Fatalf("expected fail_closed enabled")
	}
	if !reflect.DeepEqual(policy.FailClosed.RiskClasses, []string{"critical", "high"}) {
		t.Fatalf("unexpected default fail-closed risk classes: %#v", policy.FailClosed.RiskClasses)
	}
	if len(policy.Rules) != 2 || policy.Rules[0].Name != "block-external" || policy.Rules[1].Name != "allow-read" {
		t.Fatalf("expected rules sorted by priority then name, got %#v", policy.Rules)
	}
	if policy.Rules[1].Match.ToolNames[0] != "tool.read" {
		t.Fatalf("expected lower-cased tool names, got %#v", policy.Rules[1].Match.ToolNames)
	}
	if policy.Rules[1].Match.WorkspacePrefixes[0] != "/repo" {
		t.Fatalf("expected trimmed workspace prefix, got %#v", policy.Rules[1].Match.WorkspacePrefixes)
	}
}

func TestParsePolicyValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "invalid_default_verdict",
			yaml: `default_verdict: nope`,
		},
		{
			name: "unknown_top_level_field",
			yaml: `default_verdit: allow`,
		},
		{
			name: "invalid_rule_effect",
			yaml: `
rules:
  - name: bad-rule
    effect: nope
`,
		},
		{
			name: "empty_rule_name",
			yaml: `
rules:
  - name: ""
    effect: allow
`,
		},
		{
			name: "invalid_required_field",
			yaml: `
fail_closed:
  enabled: true
  required_fields: [targets, unknown]
`,
		},
		{
			name: "invalid_rate_limit_scope",
			yaml: `
rules:
  - name: bad-rate-limit
    effect: allow
    rate_limit:
      requests: 1
      scope: unknown
`,
		},
		{
			name: "invalid_rate_limit_window",
			yaml: `
rules:
  - name: bad-rate-limit-window
    effect: allow
    rate_limit:
      requests: 1
      window: day
`,
		},
		{
			name: "invalid_dataflow_action",
			yaml: `
rules:
  - name: bad-dataflow
    effect: allow
    dataflow:
      enabled: true
      action: allow
`,
		},
		{
			name: "invalid_endpoint_action",
			yaml: `
rules:
  - name: bad-endpoint-action
    effect: allow
    endpoint:
      enabled: true
      action: dry_run
`,
		},
		{
			name: "invalid_endpoint_class_match",
			yaml: `
rules:
  - name: bad-endpoint-class
    effect: allow
    match:
      endpoint_classes: [net.invalid]
`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := ParsePolicyYAML([]byte(testCase.yaml)); err == nil {
				t.Fatalf("expected parse failure")
			}
		})
	}
}

func TestParsePolicyYAMLRejectsUnknownFields(t *testing.T) {
	_, err := ParsePolicyYAML([]byte("default_verdit: allow\n"))
	if err == nil {
		t.Fatalf("expected parse failure for unknown field")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestEvaluatePolicyRuleMatchDeterministic(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: block-external-host
    priority: 1
    effect: block
    match:
      tool_names: [tool.write]
      target_kinds: [host]
      target_values: [api.external.com]
      risk_classes: [high]
    reason_codes: [blocked_external]
    violations: [external_target]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "TOOL.WRITE"
	intent.Context.RiskClass = "HIGH"
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "host", Value: "api.external.com", Operation: "write", Sensitivity: "confidential", EndpointClass: "net.http"},
	}

	first, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate first result: %v", err)
	}
	second, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate second result: %v", err)
	}

	if first.Verdict != "block" {
		t.Fatalf("unexpected verdict: %#v", first)
	}
	if !reflect.DeepEqual(first.ReasonCodes, []string{"blocked_external"}) || !reflect.DeepEqual(first.Violations, []string{"external_target"}) {
		t.Fatalf("unexpected reason codes or violations: %#v", first)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first result: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second result: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("expected deterministic output for same policy+intent: first=%s second=%s", string(firstJSON), string(secondJSON))
	}
}

func TestEvaluatePolicyFailClosedForHighRiskMissingFields(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
fail_closed:
  enabled: true
  risk_classes: [high]
  required_fields: [targets, arg_provenance]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.Context.RiskClass = "high"
	intent.Targets = nil
	intent.ArgProvenance = nil

	result, err := EvaluatePolicy(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}
	if result.Verdict != "block" {
		t.Fatalf("expected fail-closed block verdict, got %#v", result)
	}
	expectedReasons := []string{"fail_closed_missing_arg_provenance", "fail_closed_missing_targets"}
	if !reflect.DeepEqual(result.ReasonCodes, expectedReasons) {
		t.Fatalf("unexpected fail-closed reasons: got=%#v want=%#v", result.ReasonCodes, expectedReasons)
	}
}

func TestEvaluatePolicyFailClosedUnknownEndpointClass(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
fail_closed:
  enabled: true
  risk_classes: [high]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.Context.RiskClass = "high"
	intent.ToolName = "tool.unknown"
	intent.Targets = []schemagate.IntentTarget{{
		Kind:  "path",
		Value: "/tmp/out.txt",
	}}

	result, err := EvaluatePolicy(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}
	if result.Verdict != "block" {
		t.Fatalf("expected fail-closed block for unknown endpoint class, got %#v", result)
	}
	if !reflect.DeepEqual(result.ReasonCodes, []string{"fail_closed_endpoint_class_unknown"}) {
		t.Fatalf("unexpected reason codes: %#v", result.ReasonCodes)
	}
}

func TestEvaluatePolicyFailClosedNormalizationError(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
fail_closed:
  enabled: true
  risk_classes: [high]
  required_fields: [targets]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.Context.Workspace = ""
	intent.Context.RiskClass = "high"

	result, err := EvaluatePolicy(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("expected fail-closed block result, got error: %v", err)
	}
	if result.Verdict != "block" || !reflect.DeepEqual(result.ReasonCodes, []string{"fail_closed_intent_invalid"}) {
		t.Fatalf("unexpected fail-closed invalid-intent result: %#v", result)
	}
}

func TestEvaluatePolicyNormalizationErrorLowRiskReturnsError(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`default_verdict: allow`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.Context.Workspace = ""
	intent.Context.RiskClass = "low"

	if _, err := EvaluatePolicy(policy, intent, EvalOptions{}); err == nil {
		t.Fatalf("expected normalization error for low-risk intent")
	}
}

func TestEvaluatePolicyDefaultVerdict(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`default_verdict: dry_run`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	result, err := EvaluatePolicy(policy, baseIntent(), EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}
	if result.Verdict != "dry_run" {
		t.Fatalf("unexpected default verdict result: %#v", result)
	}
	if !reflect.DeepEqual(result.ReasonCodes, []string{"default_dry_run"}) {
		t.Fatalf("unexpected default reason codes: %#v", result.ReasonCodes)
	}
}

func TestLoadPolicyFileAndParseErrors(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte("default_verdict: allow\n"), 0o600); err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	policy, err := LoadPolicyFile(policyPath)
	if err != nil {
		t.Fatalf("load policy file: %v", err)
	}
	if policy.DefaultVerdict != "allow" {
		t.Fatalf("unexpected loaded policy: %#v", policy)
	}

	if _, err := LoadPolicyFile(filepath.Join(workDir, "missing.yaml")); err == nil {
		t.Fatalf("expected missing policy file to fail")
	}

	if _, err := ParsePolicyYAML([]byte("default_verdict: [")); err == nil {
		t.Fatalf("expected invalid YAML to fail")
	}
}

func TestEvaluatePolicyRuleFallbackReasonCodeUsesSanitizedRuleName(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
rules:
  - name: "Block External Host-1"
    effect: block
    match:
      tool_names: [tool.write]
      risk_classes: [high]
      target_kinds: [host]
      target_values: [api.external.com]
      provenance_sources: [external]
      identities: [alice]
      workspace_prefixes: [/repo]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Context.RiskClass = "high"
	intent.ArgProvenance = []schemagate.IntentArgProvenance{
		{ArgPath: "args.path", Source: "external"},
	}
	intent.SkillProvenance = &schemagate.SkillProvenance{
		SkillName:    "safe-curl",
		Source:       "registry",
		Publisher:    "acme",
		SkillVersion: "1.2.0",
	}
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "host", Value: "api.external.com", Operation: "write", Sensitivity: "confidential", EndpointClass: "net.http"},
	}

	result, err := EvaluatePolicy(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}
	if result.Verdict != "block" {
		t.Fatalf("expected block verdict, got %#v", result)
	}
	if !reflect.DeepEqual(result.ReasonCodes, []string{"matched_rule_block_external_host_1"}) {
		t.Fatalf("unexpected fallback reason codes: %#v", result.ReasonCodes)
	}
}

func TestRuleMatchesCoverage(t *testing.T) {
	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Context.RiskClass = "high"
	intent.ArgProvenance = []schemagate.IntentArgProvenance{
		{ArgPath: "args.path", Source: "external"},
	}
	intent.SkillProvenance = &schemagate.SkillProvenance{
		SkillName:    "safe-curl",
		SkillVersion: "1.2.0",
		Source:       "registry",
		Publisher:    "acme",
	}
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "host", Value: "api.external.com", Operation: "write", Sensitivity: "confidential", EndpointClass: "net.http"},
	}
	normalizedIntent, err := NormalizeIntent(intent)
	if err != nil {
		t.Fatalf("normalize intent: %v", err)
	}

	matching := PolicyMatch{
		ToolNames:         []string{"tool.write"},
		RiskClasses:       []string{"high"},
		TargetKinds:       []string{"host"},
		TargetValues:      []string{"api.external.com"},
		DataClasses:       []string{"confidential"},
		DestinationKinds:  []string{"host"},
		DestinationValues: []string{"api.external.com"},
		DestinationOps:    []string{"write"},
		EndpointClasses:   []string{"net.http"},
		SkillPublishers:   []string{"acme"},
		SkillSources:      []string{"registry"},
		ProvenanceSources: []string{"external"},
		Identities:        []string{"alice"},
		WorkspacePrefixes: []string{"/repo"},
	}
	if !ruleMatches(matching, normalizedIntent) {
		t.Fatalf("expected match to pass")
	}

	cases := []PolicyMatch{
		{ToolNames: []string{"tool.other"}},
		{RiskClasses: []string{"low"}},
		{TargetKinds: []string{"path"}},
		{TargetValues: []string{"/tmp/out.txt"}},
		{DataClasses: []string{"public"}},
		{DestinationKinds: []string{"bucket"}},
		{DestinationValues: []string{"internal.local"}},
		{DestinationOps: []string{"read"}},
		{EndpointClasses: []string{"fs.delete"}},
		{SkillPublishers: []string{"unknown"}},
		{SkillSources: []string{"git"}},
		{ProvenanceSources: []string{"user"}},
		{Identities: []string{"bob"}},
		{WorkspacePrefixes: []string{"/other"}},
	}
	for _, testCase := range cases {
		if ruleMatches(testCase, normalizedIntent) {
			t.Fatalf("expected non-match for %#v", testCase)
		}
	}
}

func TestShouldFailClosedAndBuildGateResultDefaults(t *testing.T) {
	if shouldFailClosed(FailClosedPolicy{Enabled: true, RiskClasses: nil}, "high") {
		t.Fatalf("expected fail-closed to be disabled with empty risk classes")
	}

	result := buildGateResult(
		Policy{},
		schemagate.IntentRequest{},
		EvalOptions{},
		"allow",
		[]string{"reason_b", "reason_a"},
		nil,
	)
	if result.ProducerVersion != "0.0.0-dev" {
		t.Fatalf("unexpected default producer version: %#v", result)
	}
	if !result.CreatedAt.Equal(time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected default created_at: %s", result.CreatedAt)
	}
	if !reflect.DeepEqual(result.ReasonCodes, []string{"reason_a", "reason_b"}) {
		t.Fatalf("unexpected sorted reason codes: %#v", result.ReasonCodes)
	}
}

func TestEvaluatePolicyDetailedRuleMetadata(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: approval-with-controls
    priority: 1
    effect: require_approval
    min_approvals: 2
    require_broker_credential: true
    broker_reference: "egress"
    broker_scopes: [export]
    rate_limit:
      requests: 3
      scope: tool_identity
      window: minute
    match:
      tool_names: [tool.write]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	outcome, err := EvaluatePolicyDetailed(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy detailed: %v", err)
	}
	if outcome.Result.Verdict != "require_approval" {
		t.Fatalf("unexpected verdict: %#v", outcome.Result)
	}
	if outcome.MatchedRule != "approval-with-controls" {
		t.Fatalf("unexpected matched rule: %s", outcome.MatchedRule)
	}
	if outcome.MinApprovals != 2 {
		t.Fatalf("unexpected min approvals: %d", outcome.MinApprovals)
	}
	if !outcome.RequireDistinctApprovers {
		t.Fatalf("expected distinct approvers for min_approvals > 1")
	}
	if !outcome.RequireBrokerCredential {
		t.Fatalf("expected broker credential requirement")
	}
	if outcome.BrokerReference != "egress" {
		t.Fatalf("unexpected broker reference: %s", outcome.BrokerReference)
	}
	if !reflect.DeepEqual(outcome.BrokerScopes, []string{"export"}) {
		t.Fatalf("unexpected broker scopes: %#v", outcome.BrokerScopes)
	}
	if outcome.RateLimit.Requests != 3 || outcome.RateLimit.Scope != "tool_identity" || outcome.RateLimit.Window != "minute" {
		t.Fatalf("unexpected rate limit: %#v", outcome.RateLimit)
	}
}

func TestPolicyHighRiskBrokerRequirements(t *testing.T) {
	withoutBroker, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: high-risk-allow
    effect: allow
    match:
      risk_classes: [high]
`))
	if err != nil {
		t.Fatalf("parse policy without broker: %v", err)
	}
	if !PolicyHasHighRiskUnbrokeredActions(withoutBroker) {
		t.Fatalf("expected unbrokered high-risk detection")
	}
	if PolicyRequiresBrokerForHighRisk(withoutBroker) {
		t.Fatalf("unexpected broker-required detection")
	}

	withBroker, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: high-risk-approval
    effect: require_approval
    require_broker_credential: true
    match:
      risk_classes: [critical]
  - name: high-risk-block
    effect: block
    match:
      risk_classes: [high]
`))
	if err != nil {
		t.Fatalf("parse policy with broker: %v", err)
	}
	if PolicyHasHighRiskUnbrokeredActions(withBroker) {
		t.Fatalf("unexpected unbrokered high-risk detection")
	}
	if !PolicyRequiresBrokerForHighRisk(withBroker) {
		t.Fatalf("expected broker-required high-risk detection")
	}
}

func TestEvaluatePolicyDataflowConstraint(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: tainted-egress
    effect: allow
    dataflow:
      enabled: true
      tainted_sources: [external, tool_output]
      destination_kinds: [host]
      destination_operations: [write]
      action: require_approval
      reason_code: dataflow_tainted_egress
      violation: tainted_egress
    match:
      tool_names: [tool.write]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Targets = []schemagate.IntentTarget{{
		Kind:      "host",
		Value:     "api.external.com",
		Operation: "write",
	}}
	intent.ArgProvenance = []schemagate.IntentArgProvenance{{
		ArgPath: "$.path",
		Source:  "external",
	}}

	outcome, err := EvaluatePolicyDetailed(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy with dataflow: %v", err)
	}
	if outcome.Result.Verdict != "require_approval" {
		t.Fatalf("expected dataflow-triggered require_approval, got %#v", outcome.Result)
	}
	if !contains(outcome.Result.ReasonCodes, "dataflow_tainted_egress") {
		t.Fatalf("expected dataflow reason code in result: %#v", outcome.Result.ReasonCodes)
	}
	if !contains(outcome.Result.Violations, "tainted_egress") {
		t.Fatalf("expected dataflow violation in result: %#v", outcome.Result.Violations)
	}
	if !outcome.DataflowTriggered {
		t.Fatalf("expected dataflow trigger metadata")
	}
}

func TestEvaluatePolicyEndpointConstraintPathAndDomain(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: endpoint-guard
    effect: allow
    match:
      tool_names: [tool.write]
    endpoint:
      enabled: true
      path_allowlist: [/tmp/safe/**]
      path_denylist: [/tmp/safe/blocked/**]
      domain_allowlist: [api.internal.local]
      egress_classes: [net.http]
      action: block
      reason_code: endpoint_policy_violation
      violation: endpoint_policy_violation
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "path", Value: "/tmp/safe/blocked/file.txt", Operation: "write", EndpointClass: "fs.write"},
		{Kind: "url", Value: "https://api.external.com/export", Operation: "write", EndpointClass: "net.http", EndpointDomain: "api.external.com"},
	}

	outcome, err := EvaluatePolicyDetailed(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy detailed: %v", err)
	}
	if outcome.Result.Verdict != "block" {
		t.Fatalf("expected endpoint constraint block verdict, got %#v", outcome.Result)
	}
	for _, reasonCode := range []string{"endpoint_domain_not_allowlisted", "endpoint_path_denied", "endpoint_policy_violation"} {
		if !contains(outcome.Result.ReasonCodes, reasonCode) {
			t.Fatalf("expected reason code %q in %#v", reasonCode, outcome.Result.ReasonCodes)
		}
	}
}

func TestEvaluatePolicyEndpointConstraintDestructiveAction(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: endpoint-destructive
    effect: allow
    match:
      tool_names: [tool.write]
    endpoint:
      enabled: true
      destructive_action: require_approval
      action: block
      reason_code: endpoint_policy_violation
      violation: endpoint_policy_violation
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "path", Value: "/tmp/data.txt", Operation: "delete", EndpointClass: "fs.delete", Destructive: true},
	}

	outcome, err := EvaluatePolicyDetailed(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy detailed: %v", err)
	}
	if outcome.Result.Verdict != "block" {
		t.Fatalf("expected most restrictive endpoint verdict to be block, got %#v", outcome.Result)
	}
	if !contains(outcome.Result.ReasonCodes, "endpoint_destructive_operation") {
		t.Fatalf("expected destructive reason code, got %#v", outcome.Result.ReasonCodes)
	}
}

func TestEvaluatePolicySkillTrustHooks(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: trusted-skill
    priority: 1
    effect: allow
    match:
      skill_publishers: [acme]
      skill_sources: [registry]
      tool_names: [tool.write]
    reason_codes: [skill_trusted]
  - name: untrusted-skill
    priority: 5
    effect: require_approval
    match:
      skill_sources: [git]
      tool_names: [tool.write]
    reason_codes: [skill_untrusted_source]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.SkillProvenance = &schemagate.SkillProvenance{
		SkillName:    "safe-curl",
		SkillVersion: "1.0.0",
		Source:       "registry",
		Publisher:    "acme",
		Digest:       strings.Repeat("a", 64),
	}

	allowResult, err := EvaluatePolicy(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy allow: %v", err)
	}
	if allowResult.Verdict != "allow" {
		t.Fatalf("expected allow for trusted skill, got %#v", allowResult)
	}
	if !contains(allowResult.ReasonCodes, "skill_trusted") {
		t.Fatalf("expected trusted reason code, got %#v", allowResult.ReasonCodes)
	}

	intent.SkillProvenance.Source = "git"
	approvalResult, err := EvaluatePolicy(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy approval: %v", err)
	}
	if approvalResult.Verdict != "require_approval" {
		t.Fatalf("expected require_approval for untrusted skill source, got %#v", approvalResult)
	}
}

func TestPolicyDigestLegacyCompatibility(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
fail_closed:
  enabled: true
  risk_classes: [high]
  required_fields: [targets]
rules:
  - name: allow-tool
    priority: 1
    effect: allow
    match:
      tool_names: [tool.write]
      risk_classes: [high]
      target_kinds: [host]
      target_values: [api.external.com]
      provenance_sources: [external]
      identities: [alice]
      workspace_prefixes: [/repo]
    reason_codes: [allow_tool]
    violations: [none]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	type legacyFailClosed struct {
		Enabled        bool
		RiskClasses    []string
		RequiredFields []string
	}
	type legacyPolicyMatch struct {
		ToolNames         []string
		RiskClasses       []string
		TargetKinds       []string
		TargetValues      []string
		ProvenanceSources []string
		Identities        []string
		WorkspacePrefixes []string
	}
	type legacyPolicyRule struct {
		Name        string
		Priority    int
		Effect      string
		Match       legacyPolicyMatch
		ReasonCodes []string
		Violations  []string
	}
	type legacyPolicy struct {
		SchemaID       string
		SchemaVersion  string
		DefaultVerdict string
		FailClosed     legacyFailClosed
		Rules          []legacyPolicyRule
	}

	legacy := legacyPolicy{
		SchemaID:       policy.SchemaID,
		SchemaVersion:  policy.SchemaVersion,
		DefaultVerdict: policy.DefaultVerdict,
		FailClosed: legacyFailClosed{
			Enabled:        policy.FailClosed.Enabled,
			RiskClasses:    policy.FailClosed.RiskClasses,
			RequiredFields: policy.FailClosed.RequiredFields,
		},
		Rules: make([]legacyPolicyRule, 0, len(policy.Rules)),
	}
	for _, rule := range policy.Rules {
		legacy.Rules = append(legacy.Rules, legacyPolicyRule{
			Name:     rule.Name,
			Priority: rule.Priority,
			Effect:   rule.Effect,
			Match: legacyPolicyMatch{
				ToolNames:         rule.Match.ToolNames,
				RiskClasses:       rule.Match.RiskClasses,
				TargetKinds:       rule.Match.TargetKinds,
				TargetValues:      rule.Match.TargetValues,
				ProvenanceSources: rule.Match.ProvenanceSources,
				Identities:        rule.Match.Identities,
				WorkspacePrefixes: rule.Match.WorkspacePrefixes,
			},
			ReasonCodes: rule.ReasonCodes,
			Violations:  rule.Violations,
		})
	}

	legacyRaw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy policy: %v", err)
	}
	legacyDigest, err := jcs.DigestJCS(legacyRaw)
	if err != nil {
		t.Fatalf("digest legacy policy: %v", err)
	}
	digest, err := PolicyDigest(policy)
	if err != nil {
		t.Fatalf("policy digest: %v", err)
	}
	if digest != legacyDigest {
		t.Fatalf("expected digest compatibility with legacy payload, got=%s want=%s", digest, legacyDigest)
	}
}

func TestEvaluatePolicyDataflowDefaultEgressKinds(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: tainted-default-egress
    effect: allow
    dataflow:
      enabled: true
      tainted_sources: [external]
      action: block
      reason_code: tainted_default_egress
      violation: tainted_egress
    match:
      tool_names: [tool.write]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Targets = []schemagate.IntentTarget{{
		Kind:      "url",
		Value:     "https://external.invalid",
		Operation: "write",
	}}
	intent.ArgProvenance = []schemagate.IntentArgProvenance{{
		ArgPath: "$.payload",
		Source:  "external",
	}}

	outcome, err := EvaluatePolicyDetailed(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}
	if outcome.Result.Verdict != "block" {
		t.Fatalf("expected block verdict from default egress dataflow, got %#v", outcome.Result)
	}
	if !contains(outcome.Result.ReasonCodes, "tainted_default_egress") {
		t.Fatalf("expected default egress reason code, got %#v", outcome.Result.ReasonCodes)
	}
}

func TestEvaluatePolicyDataflowDoesNotTriggerForNonEgressKind(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: tainted-default-egress
    effect: allow
    dataflow:
      enabled: true
      tainted_sources: [external]
      action: block
      reason_code: tainted_default_egress
      violation: tainted_egress
    match:
      tool_names: [tool.write]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Targets = []schemagate.IntentTarget{{
		Kind:      "path",
		Value:     "/tmp/out.txt",
		Operation: "write",
	}}
	intent.ArgProvenance = []schemagate.IntentArgProvenance{{
		ArgPath: "$.payload",
		Source:  "external",
	}}

	outcome, err := EvaluatePolicyDetailed(policy, intent, EvalOptions{})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}
	if outcome.Result.Verdict != "allow" {
		t.Fatalf("expected allow verdict when destination is non-egress kind, got %#v", outcome.Result)
	}
	if contains(outcome.Result.ReasonCodes, "tainted_default_egress") {
		t.Fatalf("unexpected dataflow reason code for non-egress destination: %#v", outcome.Result.ReasonCodes)
	}
}

func TestPolicyDigestPayloadIncludesExtendedFields(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: rich-rule
    effect: require_approval
    min_approvals: 2
    require_distinct_approvers: true
    require_broker_credential: true
    broker_reference: egress
    broker_scopes: [export]
    rate_limit:
      requests: 2
      window: hour
      scope: tool_identity
    dataflow:
      enabled: true
      tainted_sources: [external, tool_output]
      destination_kinds: [host]
      destination_values: [api.external.com]
      destination_operations: [write]
      action: require_approval
      reason_code: tainted_route
      violation: tainted_route
    match:
      tool_names: [tool.write]
      endpoint_classes: [net.http]
      data_classes: [confidential]
      destination_kinds: [host]
      destination_values: [api.external.com]
      destination_operations: [write]
    endpoint:
      enabled: true
      path_allowlist: [/tmp/safe/**]
      domain_allowlist: [api.internal.local]
      egress_classes: [net.http]
      action: block
      reason_code: endpoint_policy_violation
      violation: endpoint_policy_violation
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	payload := policyDigestPayload(policy)
	rulesAny, ok := payload["Rules"].([]any)
	if !ok || len(rulesAny) != 1 {
		t.Fatalf("unexpected payload rules: %#v", payload["Rules"])
	}
	ruleAny, ok := rulesAny[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected rule payload type: %#v", rulesAny[0])
	}
	if _, ok := ruleAny["RateLimit"]; !ok {
		t.Fatalf("expected rate_limit payload to be present: %#v", ruleAny)
	}
	if _, ok := ruleAny["Dataflow"]; !ok {
		t.Fatalf("expected dataflow payload to be present: %#v", ruleAny)
	}
	if _, ok := ruleAny["Endpoint"]; !ok {
		t.Fatalf("expected endpoint payload to be present: %#v", ruleAny)
	}
	if _, ok := ruleAny["BrokerScopes"]; !ok {
		t.Fatalf("expected broker_scopes payload to be present: %#v", ruleAny)
	}
	if _, ok := ruleAny["MinApprovals"]; !ok {
		t.Fatalf("expected min_approvals payload to be present: %#v", ruleAny)
	}
}

func TestEvaluatePolicyDelegationConstraints(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-delegated-write
    effect: allow
    match:
      tool_names: [tool.write]
      require_delegation: true
      allowed_delegator_identities: [agent.lead]
      allowed_delegate_identities: [agent.specialist]
      delegation_scopes: [write]
      max_delegation_depth: 2
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.write"
	intent.Delegation = &schemagate.IntentDelegation{
		RequesterIdentity: "agent.specialist",
		ScopeClass:        "write",
		Chain: []schemagate.DelegationLink{
			{DelegatorIdentity: "agent.lead", DelegateIdentity: "agent.specialist", ScopeClass: "write"},
		},
	}
	result, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}
	if result.Verdict != "allow" {
		t.Fatalf("expected allow verdict, got %#v", result)
	}

	intent.Delegation.Chain[0].DelegatorIdentity = "agent.other"
	result, err = EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate policy mismatched delegator: %v", err)
	}
	if result.Verdict != "block" {
		t.Fatalf("expected block verdict when delegation mismatches, got %#v", result)
	}
}

func TestEvaluatePolicyFailClosedMissingDelegation(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
fail_closed:
  enabled: true
  risk_classes: [high]
  required_fields: [delegation]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.Context.RiskClass = "high"
	intent.Delegation = nil

	result, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate fail-closed delegation policy: %v", err)
	}
	if result.Verdict != "block" {
		t.Fatalf("expected block verdict for missing delegation, got %#v", result)
	}
	if !contains(result.ReasonCodes, "fail_closed_missing_delegation") {
		t.Fatalf("expected fail-closed delegation reason code, got %#v", result.ReasonCodes)
	}
}

func TestPolicyHelperMatchersAndSanitizers(t *testing.T) {
	if !intentContainsDestructiveTarget([]schemagate.IntentTarget{{Destructive: true}}) {
		t.Fatalf("expected destructive target detection")
	}
	if intentContainsDestructiveTarget([]schemagate.IntentTarget{{Destructive: false}}) {
		t.Fatalf("did not expect destructive target detection")
	}

	if !matchPathPattern("/tmp/safe/file.txt", "/tmp/safe/**") {
		t.Fatalf("expected /** prefix pattern match")
	}
	if !matchPathPattern("/tmp/safe/file.txt", "/tmp/safe/*.txt") {
		t.Fatalf("expected glob path pattern match")
	}
	if matchPathPattern("/tmp/safe/file.txt", "") {
		t.Fatalf("did not expect empty path pattern to match")
	}
	if matchPathPattern("/tmp/safe/file.txt", "[") {
		t.Fatalf("did not expect invalid path glob to match")
	}
	if !matchesAnyPattern("/tmp/safe/file.txt", []string{"/tmp/other/**", "/tmp/safe/**"}) {
		t.Fatalf("expected matchesAnyPattern positive match")
	}
	if matchesAnyPattern("/tmp/safe/file.txt", []string{"/tmp/other/**", "["}) {
		t.Fatalf("did not expect matchesAnyPattern false match")
	}

	if !matchesAnyDomain("API.INTERNAL.LOCAL", []string{"api.internal.local"}) {
		t.Fatalf("expected exact domain match to be case-insensitive")
	}
	if !matchesAnyDomain("svc.example.com", []string{"*.example.com"}) {
		t.Fatalf("expected wildcard domain match")
	}
	if !matchesAnyDomain("api.dev.example.com", []string{"api.*.example.com"}) {
		t.Fatalf("expected glob domain match")
	}
	if matchesAnyDomain("svc.example.com", []string{"", "["}) {
		t.Fatalf("did not expect empty or invalid domain patterns to match")
	}

	if got := sanitizeName(""); got != "rule" {
		t.Fatalf("expected empty sanitizeName fallback, got %q", got)
	}
	if got := sanitizeName("My-Rule Name"); got != "my_rule_name" {
		t.Fatalf("unexpected sanitizeName output: %q", got)
	}
}

func TestEvaluateContextConstraintBranches(t *testing.T) {
	intent := baseIntent()

	blocked, verdict, reasons, violations := evaluateContextConstraint(PolicyRule{}, intent)
	if blocked || verdict != "" || len(reasons) != 0 || len(violations) != 0 {
		t.Fatalf("expected no context constraint without context requirements: blocked=%v verdict=%q reasons=%#v violations=%#v", blocked, verdict, reasons, violations)
	}

	rule := PolicyRule{
		RequireContextEvidence:      true,
		RequiredContextEvidenceMode: "required",
		MaxContextAgeSeconds:        30,
	}
	intent.Context.ContextSetDigest = ""
	intent.Context.ContextEvidenceMode = "best_effort"
	intent.Context.AuthContext = map[string]any{"context_age_seconds": int64(120)}

	blocked, verdict, reasons, violations = evaluateContextConstraint(rule, intent)
	if !blocked || verdict != "block" {
		t.Fatalf("expected context constraint block, got blocked=%v verdict=%q", blocked, verdict)
	}
	for _, reason := range []string{
		"context_evidence_missing",
		"context_set_digest_missing",
		"context_evidence_mode_mismatch",
		"context_freshness_exceeded",
	} {
		if !contains(reasons, reason) {
			t.Fatalf("expected reason %q in %#v", reason, reasons)
		}
	}
	for _, violation := range []string{
		"missing_context_evidence",
		"context_evidence_mode_violation",
		"context_freshness_violation",
	} {
		if !contains(violations, violation) {
			t.Fatalf("expected violation %q in %#v", violation, violations)
		}
	}

	intent.Context.ContextSetDigest = strings.Repeat("a", 64)
	intent.Context.ContextEvidenceMode = "required"
	intent.Context.AuthContext = map[string]any{"context_age_seconds": int64(5)}

	blocked, verdict, reasons, violations = evaluateContextConstraint(rule, intent)
	if blocked || verdict != "" || len(reasons) != 0 || len(violations) != 0 {
		t.Fatalf("expected no context constraint block for valid context evidence: blocked=%v verdict=%q reasons=%#v violations=%#v", blocked, verdict, reasons, violations)
	}

	rule = PolicyRule{MaxContextAgeSeconds: 30}
	intent.Context.AuthContext = map[string]any{}
	blocked, verdict, reasons, violations = evaluateContextConstraint(rule, intent)
	if !blocked || verdict != "block" {
		t.Fatalf("expected freshness block when age is unavailable, got blocked=%v verdict=%q", blocked, verdict)
	}
	if !contains(reasons, "context_freshness_exceeded") || !contains(violations, "context_freshness_violation") {
		t.Fatalf("expected freshness reason+violation, got reasons=%#v violations=%#v", reasons, violations)
	}
}

func TestContextAgeSecondsParsesSupportedTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected int64
		ok       bool
	}{
		{name: "int", value: int(3), expected: 3, ok: true},
		{name: "int8", value: int8(4), expected: 4, ok: true},
		{name: "int16", value: int16(5), expected: 5, ok: true},
		{name: "int32", value: int32(6), expected: 6, ok: true},
		{name: "int64", value: int64(7), expected: 7, ok: true},
		{name: "uint", value: uint(8), expected: 8, ok: true},
		{name: "uint8", value: uint8(9), expected: 9, ok: true},
		{name: "uint16", value: uint16(10), expected: 10, ok: true},
		{name: "uint32", value: uint32(11), expected: 11, ok: true},
		{name: "uint64", value: uint64(12), expected: 12, ok: true},
		{name: "float32", value: float32(13.7), expected: 13, ok: true},
		{name: "float64", value: float64(14.9), expected: 14, ok: true},
		{name: "json_number", value: json.Number("15"), expected: 15, ok: true},
		{name: "string_number", value: " 16 ", expected: 16, ok: true},
		{name: "overflow_uint64", value: ^uint64(0), expected: 0, ok: false},
		{name: "negative_float32", value: float32(-1), expected: 0, ok: false},
		{name: "negative_float64", value: float64(-2), expected: 0, ok: false},
		{name: "negative_json_number", value: json.Number("-3"), expected: 0, ok: false},
		{name: "invalid_json_number", value: json.Number("nope"), expected: 0, ok: false},
		{name: "invalid_string_number", value: "nope", expected: 0, ok: false},
		{name: "unsupported_type", value: []string{"x"}, expected: 0, ok: false},
	}

	if value, ok := contextAgeSeconds(nil); ok || value != 0 {
		t.Fatalf("expected nil auth_context to be unsupported, got value=%d ok=%v", value, ok)
	}
	if value, ok := contextAgeSeconds(map[string]any{}); ok || value != 0 {
		t.Fatalf("expected empty auth_context to be unsupported, got value=%d ok=%v", value, ok)
	}
	if value, ok := contextAgeSeconds(map[string]any{"other": 1}); ok || value != 0 {
		t.Fatalf("expected missing context_age_seconds to be unsupported, got value=%d ok=%v", value, ok)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			age, ok := contextAgeSeconds(map[string]any{"context_age_seconds": tc.value})
			if ok != tc.ok || age != tc.expected {
				t.Fatalf("unexpected context age parse for %s: age=%d ok=%v expected_age=%d expected_ok=%v", tc.name, age, ok, tc.expected, tc.ok)
			}
		})
	}
}

func TestEvaluateFailClosedRequiredFieldsContextEvidence(t *testing.T) {
	intent := baseIntent()
	intent.Context.ContextSetDigest = ""
	reasons, violations := evaluateFailClosedRequiredFields([]string{"context_evidence"}, intent)
	if !reflect.DeepEqual(reasons, []string{"context_evidence_missing"}) {
		t.Fatalf("unexpected context evidence reasons: %#v", reasons)
	}
	if !reflect.DeepEqual(violations, []string{"missing_context_evidence"}) {
		t.Fatalf("unexpected context evidence violations: %#v", violations)
	}

	intent.Context.ContextSetDigest = strings.Repeat("b", 64)
	reasons, violations = evaluateFailClosedRequiredFields([]string{"context_evidence"}, intent)
	if len(reasons) != 0 || len(violations) != 0 {
		t.Fatalf("expected no context evidence violations when digest exists, got reasons=%#v violations=%#v", reasons, violations)
	}
}

func baseIntent() schemagate.IntentRequest {
	return schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		ToolName:        "tool.demo",
		Args:            map[string]any{"x": "y"},
		Targets:         []schemagate.IntentTarget{},
		ArgProvenance:   []schemagate.IntentArgProvenance{},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "low",
		},
	}
}
