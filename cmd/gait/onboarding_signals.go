package main

import (
	"fmt"
	"strings"

	"github.com/Clyra-AI/gait/core/gate"
)

type repoSignal struct {
	Code       string   `json:"code"`
	Category   string   `json:"category"`
	Value      string   `json:"value,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
	Evidence   []string `json:"evidence,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

type generatedRule struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Effect         string   `json:"effect"`
	Priority       int      `json:"priority"`
	MatchToolNames []string `json:"match_tool_names,omitempty"`
	ReasonCodes    []string `json:"reason_codes,omitempty"`
	Confidence     string   `json:"confidence,omitempty"`
	Rationale      string   `json:"rationale,omitempty"`
}

type readinessFinding struct {
	Code            string   `json:"code"`
	Severity        string   `json:"severity"`
	Summary         string   `json:"summary"`
	DetectedSurface string   `json:"detected_surface,omitempty"`
	Remediation     string   `json:"remediation,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`
}

func deriveRepoSignalsAndStarterRules(detection repoDetection) ([]repoSignal, []generatedRule, []repoSignal) {
	signals := make([]repoSignal, 0, len(detection.Frameworks)+3)
	generated := []generatedRule{}
	unknown := []repoSignal{}

	for _, framework := range detection.Frameworks {
		trimmed := strings.TrimSpace(framework)
		if trimmed == "" {
			continue
		}
		signals = append(signals, repoSignal{
			Code:       "framework." + trimmed,
			Category:   "framework",
			Value:      trimmed,
			Confidence: "high",
			Evidence:   []string{trimmed},
			Reason:     "supported framework tag detected by scout",
		})
	}

	candidateTools := normalizeActionCandidateTools(detection.ToolNames)
	if len(candidateTools) == 0 {
		return signals, generated, unknown
	}

	classified := map[string][]string{
		"destructive":  {},
		"state_change": {},
		"payment":      {},
		"data_store":   {},
	}
	seenClassified := map[string]struct{}{}
	for _, toolName := range candidateTools {
		switch {
		case matchesAnyFragment(toolName, "delete", "remove", "drop", "destroy", "truncate"):
			classified["destructive"] = append(classified["destructive"], toolName)
			seenClassified[toolName] = struct{}{}
		case matchesAnyFragment(toolName, "stripe", "payment", "charge", "refund", "billing", "invoice"):
			classified["payment"] = append(classified["payment"], toolName)
			seenClassified[toolName] = struct{}{}
		case matchesAnyFragment(toolName, "postgres", "database", "db", "sql"):
			classified["data_store"] = append(classified["data_store"], toolName)
			seenClassified[toolName] = struct{}{}
		case matchesAnyFragment(toolName, "write", "update", "create", "set", "save", "submit", "approve", "cancel", "pause", "resume", "restart"):
			classified["state_change"] = append(classified["state_change"], toolName)
			seenClassified[toolName] = struct{}{}
		}
	}

	appendSignal := func(code string, category string, evidence []string, reason string) {
		if len(evidence) == 0 {
			return
		}
		signals = append(signals, repoSignal{
			Code:       code,
			Category:   category,
			Value:      category,
			Confidence: "high",
			Evidence:   evidence,
			Reason:     reason,
		})
	}

	appendSignal("tool.destructive", "destructive_tools", classified["destructive"], "likely destructive tool names detected")
	appendSignal("tool.state_change", "state_change_tools", classified["state_change"], "likely state-changing tool names detected")
	appendSignal("tool.payment", "payment_tools", classified["payment"], "payment or billing tool names detected")
	appendSignal("tool.data_store", "data_store_tools", classified["data_store"], "database or data-store tool names detected")

	if len(classified["destructive"]) > 0 {
		generated = append(generated, generatedRule{
			ID:             "starter.block.destructive",
			Name:           "block-destructive-tools",
			Effect:         "block",
			Priority:       10,
			MatchToolNames: limitStrings(classified["destructive"], 6),
			ReasonCodes:    []string{"destructive_tool_blocked"},
			Confidence:     "high",
			Rationale:      "detected likely destructive tool names",
		})
	}
	approvalTools := mergeUniqueSorted(nil, classified["state_change"])
	approvalTools = mergeUniqueSorted(approvalTools, classified["payment"])
	approvalTools = mergeUniqueSorted(approvalTools, classified["data_store"])
	if len(approvalTools) > 0 {
		generated = append(generated, generatedRule{
			ID:             "starter.require_approval.state_change",
			Name:           "require-approval-state-changing-tools",
			Effect:         "require_approval",
			Priority:       20,
			MatchToolNames: limitStrings(approvalTools, 8),
			ReasonCodes:    []string{"approval_required_for_state_change"},
			Confidence:     "high",
			Rationale:      "detected likely state-changing, payment, or data-store tool names",
		})
	}

	unclassified := make([]string, 0, len(candidateTools))
	for _, toolName := range candidateTools {
		if _, ok := seenClassified[toolName]; ok {
			continue
		}
		unclassified = append(unclassified, toolName)
	}
	if len(unclassified) > 0 {
		unknown = append(unknown, repoSignal{
			Code:       "tool.unclassified",
			Category:   "unclassified_tools",
			Value:      "manual_review",
			Confidence: "low",
			Evidence:   limitStrings(unclassified, 8),
			Reason:     "candidate tool names were detected but did not match a high-confidence starter rule pattern",
		})
	}

	return signals, generated, unknown
}

func buildPolicyReadinessFindings(policy gate.Policy, detection repoDetection, signals []repoSignal, generated []generatedRule, unknown []repoSignal) []readinessFinding {
	findings := []readinessFinding{}
	if len(policy.Rules) == 0 {
		findings = append(findings, readinessFinding{
			Code:            "policy.no_rules",
			Severity:        "warning",
			Summary:         "policy defines no explicit rules; only default_verdict will apply",
			DetectedSurface: "policy.rules",
			Remediation:     "add explicit rules or review generated starter suggestions from `gait init --json`",
		})
	}
	if strings.EqualFold(strings.TrimSpace(policy.DefaultVerdict), "allow") {
		findings = append(findings, readinessFinding{
			Code:            "policy.default_allow",
			Severity:        "warning",
			Summary:         "default_verdict=allow is permissive; high-risk repos usually prefer require_approval or block",
			DetectedSurface: "policy.default_verdict",
			Remediation:     "set `default_verdict` to `require_approval` or `block` before production rollout",
		})
	}

	hasNonAllowRule := false
	matchedToolNames := map[string]struct{}{}
	for _, rule := range policy.Rules {
		effect := strings.ToLower(strings.TrimSpace(rule.Effect))
		if effect == "" {
			effect = strings.ToLower(strings.TrimSpace(rule.Action))
		}
		if effect != "" && effect != "allow" {
			hasNonAllowRule = true
		}
		if toolName := strings.TrimSpace(rule.Match.ToolName); toolName != "" {
			matchedToolNames[toolName] = struct{}{}
		}
		for _, toolName := range rule.Match.ToolNames {
			trimmed := strings.TrimSpace(toolName)
			if trimmed != "" {
				matchedToolNames[trimmed] = struct{}{}
			}
		}
	}

	if !hasNonAllowRule {
		findings = append(findings, readinessFinding{
			Code:            "policy.no_non_allow_rules",
			Severity:        "warning",
			Summary:         "policy has no non-allow rules; enforce paths will not block or require approval",
			DetectedSurface: "policy.rules",
			Remediation:     "add `block` or `require_approval` rules for state-changing or destructive tool classes",
		})
	}
	if len(detection.Frameworks) == 0 {
		findings = append(findings, readinessFinding{
			Code:            "repo.no_supported_framework_detected",
			Severity:        "info",
			Summary:         "no supported framework tools were detected locally; review whether this repo has an explicit Gait interception seam yet",
			DetectedSurface: "repo.frameworks",
			Remediation:     "wire one explicit tool boundary before relying on runtime enforcement",
		})
	}

	if len(detection.ToolNames) > 0 && len(matchedToolNames) == 0 {
		findings = append(findings, readinessFinding{
			Code:            "repo.tool_inventory_uncovered",
			Severity:        "warning",
			Summary:         "detected tool inventory but no explicit tool_name/tool_names coverage was found",
			DetectedSurface: "repo.tools",
			Remediation:     "add explicit `match.tool_name` or `match.tool_names` coverage for detected tool surfaces",
			Evidence:        limitStrings(normalizeActionCandidateTools(detection.ToolNames), 8),
		})
	}
	if len(detection.ToolNames) > 0 && len(matchedToolNames) > 0 {
		covered := false
		for _, detectedTool := range detection.ToolNames {
			if _, ok := matchedToolNames[strings.TrimSpace(detectedTool)]; ok {
				covered = true
				break
			}
		}
		if !covered {
			findings = append(findings, readinessFinding{
				Code:            "repo.tool_inventory_name_mismatch",
				Severity:        "warning",
				Summary:         "detected tool inventory names do not match explicit policy tool_name/tool_names entries",
				DetectedSurface: "repo.tools",
				Remediation:     "normalize detected tool names and policy `match.tool_name` / `match.tool_names` entries before rollout",
				Evidence:        limitStrings(normalizeActionCandidateTools(detection.ToolNames), 8),
			})
		}
	}
	if len(generated) > 0 {
		ids := make([]string, 0, len(generated))
		for _, rule := range generated {
			ids = append(ids, rule.ID)
		}
		findings = append(findings, readinessFinding{
			Code:            "repo.generated_rules_available",
			Severity:        "info",
			Summary:         "repo-aware starter rule suggestions are available from local detection",
			DetectedSurface: "repo.signals",
			Remediation:     "review the generated starter rules in `.gait.yaml` comments and either adopt or discard them explicitly",
			Evidence:        ids,
		})
	}
	if len(unknown) > 0 {
		findings = append(findings, readinessFinding{
			Code:            "repo.manual_review_required",
			Severity:        "info",
			Summary:         "some detected surfaces require manual review because they did not match a high-confidence starter rule pattern",
			DetectedSurface: "repo.signals",
			Remediation:     "review `unknown_signals` and decide whether to add custom policy rules",
			Evidence:        limitStrings(unknown[0].Evidence, 8),
		})
	}
	if len(signals) == 0 {
		findings = append(findings, readinessFinding{
			Code:            "repo.no_high_confidence_signals",
			Severity:        "info",
			Summary:         "no high-confidence repo signals were detected for starter rule synthesis",
			DetectedSurface: "repo.signals",
			Remediation:     "continue with the baseline template and add custom rules manually",
		})
	}

	return findings
}

func findingsToWarnings(findings []readinessFinding) []string {
	warnings := make([]string, 0, len(findings))
	for _, finding := range findings {
		if strings.TrimSpace(finding.Summary) == "" {
			continue
		}
		warnings = append(warnings, finding.Summary)
	}
	return warnings
}

func renderStarterRuleComments(rules []generatedRule, unknown []repoSignal) []string {
	lines := []string{}
	if len(rules) > 0 {
		lines = append(lines, "# generated_rules:")
		for _, rule := range rules {
			lines = append(lines, fmt.Sprintf("#   - id: %s", rule.ID))
			lines = append(lines, fmt.Sprintf("#     rationale: %s", rule.Rationale))
			lines = append(lines, "#     suggested_yaml:")
			lines = append(lines, fmt.Sprintf("#       - name: %s", rule.Name))
			lines = append(lines, fmt.Sprintf("#         priority: %d", rule.Priority))
			lines = append(lines, fmt.Sprintf("#         effect: %s", rule.Effect))
			if len(rule.MatchToolNames) > 0 {
				lines = append(lines, fmt.Sprintf("#         match.tool_names: [%s]", strings.Join(rule.MatchToolNames, ", ")))
			}
			if len(rule.ReasonCodes) > 0 {
				lines = append(lines, fmt.Sprintf("#         reason_codes: [%s]", strings.Join(rule.ReasonCodes, ", ")))
			}
		}
	}
	if len(unknown) > 0 {
		lines = append(lines, "# unknown_signals:")
		for _, signal := range unknown {
			lines = append(lines, fmt.Sprintf("#   - code: %s", signal.Code))
			if len(signal.Evidence) > 0 {
				lines = append(lines, fmt.Sprintf("#     evidence: [%s]", strings.Join(signal.Evidence, ", ")))
			}
			if signal.Reason != "" {
				lines = append(lines, fmt.Sprintf("#     reason: %s", signal.Reason))
			}
		}
	}
	return lines
}

func normalizeActionCandidateTools(toolNames []string) []string {
	candidates := make([]string, 0, len(toolNames))
	for _, toolName := range toolNames {
		trimmed := strings.TrimSpace(toolName)
		if trimmed == "" || strings.HasPrefix(trimmed, "_") || strings.HasPrefix(trimmed, "__") {
			continue
		}
		if strings.Contains(trimmed, ".") {
			candidates = append(candidates, strings.ToLower(trimmed))
			continue
		}
		if isAllUpper(trimmed) {
			continue
		}
		if len(trimmed) > 48 {
			continue
		}
		if matchesAnyFragment(trimmed, "write", "update", "create", "set", "save", "delete", "remove", "drop", "destroy", "truncate", "approve", "cancel", "pause", "resume", "restart", "stripe", "payment", "charge", "refund", "billing", "invoice", "postgres", "database", "db", "sql") {
			candidates = append(candidates, strings.ToLower(trimmed))
		}
	}
	return uniqueSortedStrings(candidates)
}

func matchesAnyFragment(value string, fragments ...string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, fragment := range fragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func isAllUpper(value string) bool {
	hasLetter := false
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' {
			return false
		}
		if ch >= 'A' && ch <= 'Z' {
			hasLetter = true
		}
	}
	return hasLetter
}

func limitStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}
