package gate

import (
	"strings"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const freezeWindowTimeLayout = "2006-01-02T15:04:05"

func evaluateFreezeWindowConstraint(rule PolicyRule, intent schemagate.IntentRequest, now time.Time) (bool, string, []string, []string, *schemagate.FreezeWindowDecision) {
	policy := rule.FreezeWindow
	if !policy.Enabled {
		return false, "", nil, nil, nil
	}

	decision := &schemagate.FreezeWindowDecision{
		Status:      "inactive",
		Effect:      policy.Effect,
		Timezone:    policy.Timezone,
		EvaluatedAt: now.UTC(),
	}

	environment := strings.ToLower(strings.TrimSpace(intent.Context.Environment))
	if len(policy.Environments) > 0 && !contains(policy.Environments, environment) {
		return false, "", nil, nil, decision
	}
	riskClass := strings.ToLower(strings.TrimSpace(intent.Context.RiskClass))
	if len(policy.RiskClasses) > 0 && !contains(policy.RiskClasses, riskClass) {
		return false, "", nil, nil, decision
	}

	location, err := time.LoadLocation(strings.TrimSpace(policy.Timezone))
	if err != nil {
		decision.Status = "invalid"
		decision.ReasonCode = "freeze_window_invalid_timezone"
		decision.Reason = "freeze window timezone could not be loaded"
		return true, "block", []string{decision.ReasonCode}, []string{decision.ReasonCode}, decision
	}

	evaluatedAt := now.In(location)
	for _, window := range policy.Windows {
		start, end, parseErr := parseFreezeWindowRange(window, location)
		if parseErr != nil || !end.After(start) {
			decision.Status = "invalid"
			decision.WindowName = window.Name
			decision.ReasonCode = "freeze_window_invalid_window"
			decision.Reason = "freeze window has an invalid range"
			return true, "block", []string{decision.ReasonCode}, []string{decision.ReasonCode}, decision
		}
		if !evaluatedAt.Before(start) && evaluatedAt.Before(end) {
			decision.Status = "active"
			decision.WindowName = window.Name
			decision.WindowStart = start.UTC()
			decision.WindowEnd = end.UTC()
			decision.Reason = policy.Reason
			if decision.Reason == "" {
				decision.Reason = window.Name
			}
			if policy.Effect == "require_approval" {
				decision.ReasonCode = "freeze_window_active_require_approval"
				return true, "require_approval", []string{decision.ReasonCode}, []string{decision.ReasonCode}, decision
			}
			decision.ReasonCode = "freeze_window_active_block"
			return true, "block", []string{decision.ReasonCode}, []string{decision.ReasonCode}, decision
		}
	}

	return false, "", nil, nil, decision
}

func parseFreezeWindowRange(window FreezeWindowRange, location *time.Location) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation(freezeWindowTimeLayout, strings.TrimSpace(window.Start), location)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := time.ParseInLocation(freezeWindowTimeLayout, strings.TrimSpace(window.End), location)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}

func pickFreezeWindowDecision(current, candidate *schemagate.FreezeWindowDecision) *schemagate.FreezeWindowDecision {
	if candidate == nil {
		return current
	}
	if current == nil {
		return candidate
	}

	currentRank := freezeWindowDecisionRank(current)
	candidateRank := freezeWindowDecisionRank(candidate)
	if candidateRank > currentRank {
		return candidate
	}
	if candidateRank < currentRank {
		return current
	}

	currentEffectRank := freezeWindowEffectRank(current.Effect)
	candidateEffectRank := freezeWindowEffectRank(candidate.Effect)
	if candidateEffectRank > currentEffectRank {
		return candidate
	}
	if candidateEffectRank < currentEffectRank {
		return current
	}

	if strings.Compare(strings.TrimSpace(candidate.WindowName), strings.TrimSpace(current.WindowName)) < 0 {
		return candidate
	}
	return current
}

func freezeWindowDecisionRank(decision *schemagate.FreezeWindowDecision) int {
	switch strings.ToLower(strings.TrimSpace(decision.Status)) {
	case "invalid":
		return 3
	case "active":
		return 2
	case "inactive":
		return 1
	default:
		return 0
	}
}

func freezeWindowEffectRank(effect string) int {
	switch strings.ToLower(strings.TrimSpace(effect)) {
	case "block":
		return 2
	case "require_approval":
		return 1
	default:
		return 0
	}
}
