package gate

import (
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestEvaluatePolicyFreezeWindowBlock(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-prod-deploy
    priority: 10
    effect: allow
    match:
      tool_names: [tool.deploy]
    freeze_window:
      timezone: America/Toronto
      effect: block
      reason: quarter-end freeze
      environments: [prod]
      risk_classes: [high, critical]
      windows:
        - name: quarter-end
          start: "2026-03-10T09:00:00"
          end: "2026-03-10T12:00:00"
`))
	if err != nil {
		t.Fatalf("parse freeze-window policy: %v", err)
	}

	intent := freezeWindowIntent("prod", "high")
	outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{
		ProducerVersion: "test",
		EvaluationTime:  time.Date(2026, time.March, 10, 14, 30, 0, 0, time.UTC),
	})
	if evalErr != nil {
		t.Fatalf("evaluate freeze-window policy: %v", evalErr)
	}
	if outcome.Result.Verdict != "block" {
		t.Fatalf("expected active freeze window to block, got %#v", outcome.Result)
	}
	if !contains(outcome.Result.ReasonCodes, "freeze_window_active_block") {
		t.Fatalf("expected freeze_window_active_block reason, got %#v", outcome.Result.ReasonCodes)
	}
	if outcome.FreezeWindow == nil || outcome.FreezeWindow.Status != "active" {
		t.Fatalf("expected active freeze window decision, got %#v", outcome.FreezeWindow)
	}
	if outcome.FreezeWindow.WindowName != "quarter-end" || outcome.FreezeWindow.Effect != "block" {
		t.Fatalf("unexpected freeze window decision: %#v", outcome.FreezeWindow)
	}
}

func TestEvaluatePolicyFreezeWindowRequireApproval(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-prod-deploy
    priority: 10
    effect: allow
    match:
      tool_names: [tool.deploy]
    freeze_window:
      timezone: America/Toronto
      effect: require_approval
      environments: [prod]
      risk_classes: [high]
      windows:
        - name: incident-freeze
          start: "2026-03-10T09:00:00"
          end: "2026-03-10T12:00:00"
`))
	if err != nil {
		t.Fatalf("parse freeze-window policy: %v", err)
	}

	intent := freezeWindowIntent("prod", "high")
	outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{
		ProducerVersion: "test",
		EvaluationTime:  time.Date(2026, time.March, 10, 14, 30, 0, 0, time.UTC),
	})
	if evalErr != nil {
		t.Fatalf("evaluate freeze-window approval policy: %v", evalErr)
	}
	if outcome.Result.Verdict != "require_approval" {
		t.Fatalf("expected active freeze window to require approval, got %#v", outcome.Result)
	}
	if !contains(outcome.Result.ReasonCodes, "freeze_window_active_require_approval") {
		t.Fatalf("expected freeze_window_active_require_approval reason, got %#v", outcome.Result.ReasonCodes)
	}
	if outcome.FreezeWindow == nil || outcome.FreezeWindow.Effect != "require_approval" {
		t.Fatalf("unexpected freeze window decision: %#v", outcome.FreezeWindow)
	}
}

func TestEvaluatePolicyFreezeWindowInactiveAllows(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-prod-deploy
    priority: 10
    effect: allow
    match:
      tool_names: [tool.deploy]
    freeze_window:
      timezone: America/Toronto
      effect: block
      environments: [prod]
      risk_classes: [high]
      windows:
        - name: release-freeze
          start: "2026-03-10T09:00:00"
          end: "2026-03-10T12:00:00"
`))
	if err != nil {
		t.Fatalf("parse freeze-window policy: %v", err)
	}

	intent := freezeWindowIntent("prod", "high")
	outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{
		ProducerVersion: "test",
		EvaluationTime:  time.Date(2026, time.March, 10, 18, 30, 0, 0, time.UTC),
	})
	if evalErr != nil {
		t.Fatalf("evaluate inactive freeze-window policy: %v", evalErr)
	}
	if outcome.Result.Verdict != "allow" {
		t.Fatalf("expected inactive freeze window to preserve allow, got %#v", outcome.Result)
	}
	if outcome.FreezeWindow == nil || outcome.FreezeWindow.Status != "inactive" {
		t.Fatalf("expected inactive freeze window decision, got %#v", outcome.FreezeWindow)
	}
}

func TestEvaluatePolicyFreezeWindowDSTBoundary(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-prod-deploy
    priority: 10
    effect: allow
    match:
      tool_names: [tool.deploy]
    freeze_window:
      timezone: America/Toronto
      effect: block
      environments: [prod]
      risk_classes: [high]
      windows:
        - name: dst-window
          start: "2026-03-08T01:00:00"
          end: "2026-03-08T04:00:00"
`))
	if err != nil {
		t.Fatalf("parse freeze-window policy: %v", err)
	}

	intent := freezeWindowIntent("prod", "high")
	outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{
		ProducerVersion: "test",
		EvaluationTime:  time.Date(2026, time.March, 8, 7, 30, 0, 0, time.UTC),
	})
	if evalErr != nil {
		t.Fatalf("evaluate DST-boundary freeze window: %v", evalErr)
	}
	if outcome.Result.Verdict != "block" {
		t.Fatalf("expected DST-boundary freeze window to block, got %#v", outcome.Result)
	}
	if outcome.FreezeWindow == nil || outcome.FreezeWindow.WindowName != "dst-window" {
		t.Fatalf("unexpected DST freeze window decision: %#v", outcome.FreezeWindow)
	}
}

func TestEvaluatePolicyFreezeWindowInvalidTimezoneFailsClosed(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-prod-deploy
    priority: 10
    effect: allow
    match:
      tool_names: [tool.deploy]
    freeze_window:
      timezone: Mars/Olympus
      effect: block
      environments: [prod]
      risk_classes: [high]
      windows:
        - name: invalid-timezone
          start: "2026-03-10T09:00:00"
          end: "2026-03-10T12:00:00"
`))
	if err != nil {
		t.Fatalf("parse freeze-window policy: %v", err)
	}

	intent := freezeWindowIntent("prod", "high")
	outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{
		ProducerVersion: "test",
		EvaluationTime:  time.Date(2026, time.March, 10, 14, 30, 0, 0, time.UTC),
	})
	if evalErr != nil {
		t.Fatalf("evaluate invalid-timezone freeze window: %v", evalErr)
	}
	if outcome.Result.Verdict != "block" {
		t.Fatalf("expected invalid-timezone freeze window to fail closed, got %#v", outcome.Result)
	}
	if !contains(outcome.Result.ReasonCodes, "freeze_window_invalid_timezone") {
		t.Fatalf("expected freeze_window_invalid_timezone reason, got %#v", outcome.Result.ReasonCodes)
	}
	if outcome.FreezeWindow == nil || outcome.FreezeWindow.Status != "invalid" {
		t.Fatalf("unexpected invalid freeze window decision: %#v", outcome.FreezeWindow)
	}
}

func TestEvaluatePolicyFreezeWindowInvalidWindowFailsClosed(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-prod-deploy
    priority: 10
    effect: allow
    match:
      tool_names: [tool.deploy]
    freeze_window:
      timezone: America/Toronto
      effect: block
      environments: [prod]
      risk_classes: [high]
      windows:
        - name: invalid-range
          start: "not-a-window"
          end: "2026-03-10T12:00:00"
`))
	if err != nil {
		t.Fatalf("parse freeze-window policy: %v", err)
	}

	intent := freezeWindowIntent("prod", "high")
	outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{
		ProducerVersion: "test",
		EvaluationTime:  time.Date(2026, time.March, 10, 14, 30, 0, 0, time.UTC),
	})
	if evalErr != nil {
		t.Fatalf("evaluate invalid-window freeze policy: %v", evalErr)
	}
	if outcome.Result.Verdict != "block" {
		t.Fatalf("expected invalid freeze window range to fail closed, got %#v", outcome.Result)
	}
	if !contains(outcome.Result.ReasonCodes, "freeze_window_invalid_window") {
		t.Fatalf("expected freeze_window_invalid_window reason, got %#v", outcome.Result.ReasonCodes)
	}
	if outcome.FreezeWindow == nil || outcome.FreezeWindow.Status != "invalid" {
		t.Fatalf("unexpected invalid freeze window decision: %#v", outcome.FreezeWindow)
	}
}

func TestFreezeWindowDecisionHelpers(t *testing.T) {
	active := &schemagate.FreezeWindowDecision{Status: "active", Effect: "block", WindowName: "b"}
	inactive := &schemagate.FreezeWindowDecision{Status: "inactive", Effect: "block", WindowName: "a"}
	if picked := pickFreezeWindowDecision(inactive, active); picked != active {
		t.Fatalf("expected active decision to outrank inactive, got %#v", picked)
	}
	requireApproval := &schemagate.FreezeWindowDecision{Status: "active", Effect: "require_approval", WindowName: "c"}
	if picked := pickFreezeWindowDecision(requireApproval, active); picked != active {
		t.Fatalf("expected block effect to outrank require_approval, got %#v", picked)
	}
	if rank := freezeWindowDecisionRank(active); rank != 2 {
		t.Fatalf("unexpected freezeWindowDecisionRank: %d", rank)
	}
	if rank := freezeWindowEffectRank("block"); rank != 2 {
		t.Fatalf("unexpected freezeWindowEffectRank: %d", rank)
	}
}

func freezeWindowIntent(environment, riskClass string) schemagate.IntentRequest {
	intent := baseIntent()
	intent.ToolName = "tool.deploy"
	intent.Context.Environment = environment
	intent.Context.RiskClass = riskClass
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "host", Value: "deploy.internal.example", Operation: "deploy"},
	}
	return intent
}
