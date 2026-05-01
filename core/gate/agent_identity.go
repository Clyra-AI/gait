package gate

import (
	"strings"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func evaluateAgentIdentityConstraint(rule PolicyRule, intent schemagate.IntentRequest, now time.Time) (bool, string, []string, []string) {
	if !agentIdentityPolicyEnabled(rule) {
		return false, "", nil, nil
	}
	agentID := strings.TrimSpace(intent.Context.AgentID)
	identity := intent.Context.AgentIdentity
	reasons := []string{}
	violations := []string{}

	if rule.RequireDeclaredAgent {
		if agentID == "" || identity == nil {
			reasons = append(reasons, "agent_unknown")
			violations = append(violations, "agent_identity_missing")
		}
	}
	if len(rule.DeniedAgentIDs) > 0 && agentID != "" && contains(rule.DeniedAgentIDs, agentID) {
		reasons = append(reasons, "agent_revoked")
		violations = append(violations, "agent_revoked")
	}
	if identity != nil && identity.Revoked {
		reasons = append(reasons, "agent_revoked")
		violations = append(violations, "agent_revoked")
	}
	if len(rule.AllowedAgentIDs) > 0 {
		if agentID == "" || !contains(rule.AllowedAgentIDs, agentID) {
			reasons = append(reasons, "agent_unknown")
			violations = append(violations, "agent_not_declared")
		}
	}
	if rule.RequireAgentOwner {
		if identity == nil || strings.TrimSpace(identity.Owner) == "" {
			reasons = append(reasons, "agent_owner_missing")
			violations = append(violations, "agent_owner_missing")
		}
	}
	if len(rule.RequiredAgentLifecycleStates) > 0 {
		if identity == nil || !containsAll(identity.LifecycleStates, rule.RequiredAgentLifecycleStates) {
			reasons = append(reasons, "agent_lifecycle_state_invalid")
			violations = append(violations, "agent_lifecycle_state_invalid")
		}
	}
	if rule.RequireUnexpiredAgent {
		if identity == nil || identity.ExpiresAt.IsZero() || !identity.ExpiresAt.After(now.UTC()) {
			reasons = append(reasons, "agent_expired")
			violations = append(violations, "agent_expired")
		}
	}
	if rule.RequiredAgentManifestDigest != "" {
		if identity == nil || strings.TrimSpace(identity.ManifestDigest) != rule.RequiredAgentManifestDigest {
			reasons = append(reasons, "agent_manifest_mismatch")
			violations = append(violations, "agent_manifest_mismatch")
		}
	}
	if len(rule.AllowedAgentManifestPublishers) > 0 {
		if identity == nil || !contains(rule.AllowedAgentManifestPublishers, strings.ToLower(strings.TrimSpace(identity.Publisher))) {
			reasons = append(reasons, "agent_manifest_mismatch")
			violations = append(violations, "agent_manifest_mismatch")
		}
	}
	if len(rule.AllowedAgentManifestSources) > 0 {
		if identity == nil || !contains(rule.AllowedAgentManifestSources, strings.ToLower(strings.TrimSpace(identity.Source))) {
			reasons = append(reasons, "agent_manifest_mismatch")
			violations = append(violations, "agent_manifest_mismatch")
		}
	}
	if len(reasons) == 0 {
		return false, "", nil, nil
	}
	return true, "block", uniqueSorted(reasons), uniqueSorted(violations)
}

func agentIdentityPolicyEnabled(rule PolicyRule) bool {
	return rule.RequireDeclaredAgent ||
		len(rule.AllowedAgentIDs) > 0 ||
		len(rule.DeniedAgentIDs) > 0 ||
		rule.RequiredAgentManifestDigest != "" ||
		len(rule.AllowedAgentManifestPublishers) > 0 ||
		len(rule.AllowedAgentManifestSources) > 0 ||
		len(rule.RequiredAgentLifecycleStates) > 0 ||
		rule.RequireAgentOwner ||
		rule.RequireUnexpiredAgent
}

func containsAll(values []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if len(values) == 0 {
		return false
	}
	for _, want := range required {
		if !contains(values, want) {
			return false
		}
	}
	return true
}
