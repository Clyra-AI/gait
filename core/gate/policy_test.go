package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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
		{Kind: "host", Value: "api.external.com"},
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
		{Kind: "host", Value: "api.external.com"},
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
		{Kind: "host", Value: "api.external.com"},
	}

	matching := PolicyMatch{
		ToolNames:         []string{"tool.write"},
		RiskClasses:       []string{"high"},
		TargetKinds:       []string{"host"},
		TargetValues:      []string{"api.external.com"},
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
