package regress

import (
	"fmt"
	"strings"

	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

func toolSequenceFromRunpack(bundle runpack.Runpack) []string {
	sequence := make([]string, 0, len(bundle.Intents))
	for _, intent := range bundle.Intents {
		toolName := strings.TrimSpace(intent.ToolName)
		if toolName == "" {
			continue
		}
		sequence = append(sequence, toolName)
	}
	return sequence
}

func verdictSequenceFromRunpack(bundle runpack.Runpack) []string {
	sequence := make([]string, 0, len(bundle.Results))
	for _, result := range bundle.Results {
		sequence = append(sequence, deriveTrajectoryVerdict(result))
	}
	return sequence
}

func deriveTrajectoryVerdict(result schemarunpack.ResultRecord) string {
	if result.Result != nil {
		if rawVerdict, ok := result.Result["verdict"]; ok {
			if normalized := normalizeTrajectoryVerdict(fmt.Sprintf("%v", rawVerdict)); normalized != "" {
				return normalized
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(result.Status)) {
	case "ok", "pass", "passed", "success":
		return "allow"
	case "block", "blocked", "deny", "denied":
		return "block"
	case "require_approval", "approval_required", "needs_approval":
		return "require_approval"
	case "dry_run", "dry-run", "dryrun", "simulate", "simulated":
		return "dry_run"
	default:
		return "error"
	}
}

func normalizeTrajectorySequence(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizeTrajectoryVerdictSequence(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalizedVerdict := normalizeTrajectoryVerdict(trimmed)
		if normalizedVerdict == "" {
			return nil, fmt.Errorf("invalid verdict %q", trimmed)
		}
		normalized = append(normalized, normalizedVerdict)
	}
	return normalized, nil
}

func normalizeTrajectoryVerdict(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	switch normalized {
	case "allow", "ok", "pass", "passed", "success":
		return "allow"
	case "block", "blocked", "deny", "denied":
		return "block"
	case "require_approval", "approval_required", "needs_approval", "needsapproval":
		return "require_approval"
	case "dry_run", "dryrun", "simulate", "simulated":
		return "dry_run"
	case "error", "failed", "failure":
		return "error"
	default:
		return ""
	}
}
