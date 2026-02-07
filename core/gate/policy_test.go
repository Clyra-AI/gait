package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/jcs"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
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
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := ParsePolicyYAML([]byte(testCase.yaml)); err == nil {
				t.Fatalf("expected parse failure")
			}
		})
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
		{Kind: "host", Value: "api.external.com", Operation: "write", Sensitivity: "confidential"},
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
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "host", Value: "api.external.com", Operation: "write", Sensitivity: "confidential"},
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
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "host", Value: "api.external.com", Operation: "write", Sensitivity: "confidential"},
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
		ProvenanceSources: []string{"external"},
		Identities:        []string{"alice"},
		WorkspacePrefixes: []string{"/repo"},
	}
	if !ruleMatches(matching, intent) {
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
		{ProvenanceSources: []string{"user"}},
		{Identities: []string{"bob"}},
		{WorkspacePrefixes: []string{"/other"}},
	}
	for _, testCase := range cases {
		if ruleMatches(testCase, intent) {
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
      data_classes: [confidential]
      destination_kinds: [host]
      destination_values: [api.external.com]
      destination_operations: [write]
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
	if _, ok := ruleAny["BrokerScopes"]; !ok {
		t.Fatalf("expected broker_scopes payload to be present: %#v", ruleAny)
	}
	if _, ok := ruleAny["MinApprovals"]; !ok {
		t.Fatalf("expected min_approvals payload to be present: %#v", ruleAny)
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
