package gate

import (
	"strings"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func evaluateSandboxConstraint(rule PolicyRule, intent schemagate.IntentRequest) (bool, string, []string, []string, *schemagate.SandboxDecision) {
	if !rule.Sandbox.Enabled {
		return false, "", nil, nil, nil
	}

	sandbox := intent.Context.Sandbox
	if sandbox == nil {
		decision := &schemagate.SandboxDecision{
			Status:      "missing",
			ReasonCodes: []string{"sandbox_metadata_missing"},
		}
		return true, "block", decision.ReasonCodes, decision.ReasonCodes, decision
	}

	decision := &schemagate.SandboxDecision{
		Status:              "valid",
		NetworkMode:         sandbox.NetworkMode,
		EnvExposureMode:     sandbox.EnvExposureMode,
		TimeoutSeconds:      sandbox.TimeoutSeconds,
		FilesystemIsolation: sandbox.FilesystemIsolation,
		UserMode:            sandbox.UserMode,
		EvidenceRef:         sandbox.EvidenceRef,
		EvidenceDigest:      sandbox.EvidenceDigest,
	}

	reasons := []string{}
	violations := []string{}
	if sandbox.EvidenceRef == "" || sandbox.EvidenceDigest == "" {
		reasons = append(reasons, "sandbox_evidence_missing")
		violations = append(violations, "sandbox_evidence_missing")
	}
	if sandbox.NetworkMode == "" {
		reasons = append(reasons, "sandbox_network_mode_missing")
		violations = append(violations, "sandbox_network_mode_missing")
	} else if len(rule.Sandbox.AllowedNetworkModes) > 0 && !contains(rule.Sandbox.AllowedNetworkModes, sandbox.NetworkMode) {
		reasons = append(reasons, "sandbox_network_mode_disallowed")
		violations = append(violations, "sandbox_network_mode_disallowed")
	}
	if sandbox.EnvExposureMode == "" {
		reasons = append(reasons, "sandbox_env_exposure_mode_missing")
		violations = append(violations, "sandbox_env_exposure_mode_missing")
	} else if len(rule.Sandbox.AllowedEnvExposureModes) > 0 && !contains(rule.Sandbox.AllowedEnvExposureModes, sandbox.EnvExposureMode) {
		reasons = append(reasons, "sandbox_env_exposure_mode_disallowed")
		violations = append(violations, "sandbox_env_exposure_mode_disallowed")
	}
	if sandbox.TimeoutSeconds <= 0 {
		reasons = append(reasons, "sandbox_timeout_missing")
		violations = append(violations, "sandbox_timeout_missing")
	} else if rule.Sandbox.MaxTimeoutSeconds > 0 && sandbox.TimeoutSeconds > rule.Sandbox.MaxTimeoutSeconds {
		reasons = append(reasons, "sandbox_timeout_exceeded")
		violations = append(violations, "sandbox_timeout_exceeded")
	}
	if sandbox.FilesystemIsolation == "" {
		reasons = append(reasons, "sandbox_filesystem_isolation_missing")
		violations = append(violations, "sandbox_filesystem_isolation_missing")
	} else if len(rule.Sandbox.AllowedFilesystemIsolations) > 0 && !contains(rule.Sandbox.AllowedFilesystemIsolations, sandbox.FilesystemIsolation) {
		reasons = append(reasons, "sandbox_filesystem_isolation_disallowed")
		violations = append(violations, "sandbox_filesystem_isolation_disallowed")
	}
	if sandbox.UserMode == "" {
		reasons = append(reasons, "sandbox_user_mode_missing")
		violations = append(violations, "sandbox_user_mode_missing")
	} else if len(rule.Sandbox.AllowedUserModes) > 0 && !contains(rule.Sandbox.AllowedUserModes, sandbox.UserMode) {
		reasons = append(reasons, "sandbox_user_mode_disallowed")
		violations = append(violations, "sandbox_user_mode_disallowed")
	}
	if len(rule.Sandbox.AllowedWritablePathPrefixes) > 0 && hasDisallowedSandboxWritablePath(rule.Sandbox.AllowedWritablePathPrefixes, sandbox.WritablePaths) {
		reasons = append(reasons, "sandbox_writable_path_disallowed")
		violations = append(violations, "sandbox_writable_path_disallowed")
	}
	if len(rule.Sandbox.RequiredReadOnlyRoots) > 0 && !containsAll(rule.Sandbox.RequiredReadOnlyRoots, sandbox.ReadOnlyRoots) {
		reasons = append(reasons, "sandbox_read_only_root_missing")
		violations = append(violations, "sandbox_read_only_root_missing")
	}

	if len(reasons) == 0 {
		return false, "", nil, nil, decision
	}

	decision.ReasonCodes = uniqueSorted(reasons)
	decision.Status = classifySandboxDecisionStatus(reasons)
	return true, "block", decision.ReasonCodes, uniqueSorted(violations), decision
}

func hasDisallowedSandboxWritablePath(allowedPrefixes, writablePaths []string) bool {
	for _, writablePath := range writablePaths {
		matched := false
		for _, allowedPrefix := range allowedPrefixes {
			if sandboxPathHasPrefix(writablePath, allowedPrefix) {
				matched = true
				break
			}
		}
		if !matched {
			return true
		}
	}
	return false
}

func sandboxPathHasPrefix(pathValue, prefix string) bool {
	pathValue = strings.TrimSpace(pathValue)
	prefix = strings.TrimSpace(prefix)
	if pathValue == prefix {
		return true
	}
	if prefix == "/" {
		return true
	}
	return strings.HasPrefix(pathValue, prefix+"/")
}

func classifySandboxDecisionStatus(reasons []string) string {
	for _, reason := range reasons {
		if strings.HasSuffix(reason, "_missing") {
			return "missing"
		}
	}
	for _, reason := range reasons {
		if strings.HasSuffix(reason, "_disallowed") || strings.HasSuffix(reason, "_exceeded") {
			return "disallowed"
		}
	}
	return "invalid"
}

func pickSandboxDecision(current, candidate *schemagate.SandboxDecision) *schemagate.SandboxDecision {
	if candidate == nil {
		return current
	}
	if current == nil {
		return candidate
	}

	currentRank := sandboxDecisionRank(current.Status)
	candidateRank := sandboxDecisionRank(candidate.Status)
	if candidateRank > currentRank {
		return candidate
	}
	if candidateRank < currentRank {
		return current
	}

	if strings.Compare(strings.TrimSpace(candidate.EvidenceDigest), strings.TrimSpace(current.EvidenceDigest)) < 0 {
		return candidate
	}
	return current
}

func sandboxDecisionRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "missing":
		return 4
	case "disallowed":
		return 3
	case "invalid":
		return 2
	case "valid":
		return 1
	default:
		return 0
	}
}
