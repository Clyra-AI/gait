package gate

import (
	"encoding/json"
	"fmt"
	"strings"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	jcs "github.com/Clyra-AI/proof/canon"
)

func evaluateCredentialConstraint(rule PolicyRule, intent schemagate.IntentRequest) (bool, string, []string, []string) {
	if !credentialPolicyEnabled(rule) {
		return false, "", nil, nil
	}

	ctx := intent.Context
	reasons := []string{}
	violations := []string{}
	credentialPresent := strings.TrimSpace(ctx.CredentialRef) != ""
	source := strings.ToLower(strings.TrimSpace(ctx.CredentialSource))
	accessType := strings.ToLower(strings.TrimSpace(ctx.CredentialAccessType))
	issuer := strings.TrimSpace(ctx.CredentialIssuer)

	if !credentialPresent {
		reasons = append(reasons, "credential_evidence_missing")
		violations = append(violations, "credential_evidence_missing")
	}
	if source == "" {
		reasons = append(reasons, "credential_source_unknown")
		violations = append(violations, "credential_source_unknown")
	}
	if accessType == "" {
		reasons = append(reasons, "credential_access_type_unknown")
		violations = append(violations, "credential_access_type_unknown")
	}
	if rule.RequireJITCredential {
		if accessType == "" {
			reasons = append(reasons, "credential_evidence_missing")
			violations = append(violations, "credential_evidence_missing")
		} else if accessType != "jit" {
			reasons = append(reasons, "credential_not_jit")
			violations = append(violations, "credential_not_jit")
		}
	}
	if rule.BlockStandingCredentials && disallowedStandingAccessType(accessType) {
		reasons = append(reasons, "standing_credential_disallowed")
		violations = append(violations, "standing_credential_disallowed")
	}
	if len(rule.AllowedCredentialSources) > 0 && (source == "" || !contains(rule.AllowedCredentialSources, source)) {
		reasons = append(reasons, "credential_source_disallowed")
		violations = append(violations, "credential_source_disallowed")
	}
	if len(rule.AllowedCredentialIssuers) > 0 && (issuer == "" || !contains(rule.AllowedCredentialIssuers, issuer)) {
		reasons = append(reasons, "credential_issuer_disallowed")
		violations = append(violations, "credential_issuer_disallowed")
	}
	if len(rule.AllowedCredentialAccessTypes) > 0 && (accessType == "" || !contains(rule.AllowedCredentialAccessTypes, accessType)) {
		reasons = append(reasons, "credential_access_type_disallowed")
		violations = append(violations, "credential_access_type_disallowed")
	}
	if rule.MaxCredentialTTLSeconds > 0 {
		if ctx.CredentialTTLSeconds <= 0 {
			reasons = append(reasons, "credential_evidence_missing")
			violations = append(violations, "credential_evidence_missing")
		} else if ctx.CredentialTTLSeconds > rule.MaxCredentialTTLSeconds {
			reasons = append(reasons, "credential_ttl_exceeded")
			violations = append(violations, "credential_ttl_exceeded")
		}
	}
	if targetBinding := strings.TrimSpace(ctx.CredentialTargetBinding); targetBinding != "" {
		expectedTargetBinding, err := CredentialTargetBinding(intent)
		if err != nil || targetBinding != expectedTargetBinding {
			reasons = append(reasons, "credential_target_binding_mismatch")
			violations = append(violations, "credential_target_binding_mismatch")
		}
	}
	if strings.TrimSpace(ctx.RunID) != "" && strings.TrimSpace(ctx.CredentialRunBinding) != "" && strings.TrimSpace(ctx.CredentialRunBinding) != strings.TrimSpace(ctx.RunID) {
		reasons = append(reasons, "credential_run_binding_mismatch")
		violations = append(violations, "credential_run_binding_mismatch")
	}
	if strings.TrimSpace(ctx.JobID) != "" && strings.TrimSpace(ctx.CredentialJobBinding) != "" && strings.TrimSpace(ctx.CredentialJobBinding) != strings.TrimSpace(ctx.JobID) {
		reasons = append(reasons, "credential_job_binding_mismatch")
		violations = append(violations, "credential_job_binding_mismatch")
	}

	if len(reasons) == 0 {
		return false, "", nil, nil
	}
	return true, "block", uniqueSorted(reasons), uniqueSorted(violations)
}

func credentialPolicyEnabled(rule PolicyRule) bool {
	return rule.BlockStandingCredentials ||
		len(rule.AllowedCredentialSources) > 0 ||
		len(rule.AllowedCredentialIssuers) > 0 ||
		len(rule.AllowedCredentialAccessTypes) > 0 ||
		rule.MaxCredentialTTLSeconds > 0 ||
		rule.RequireJITCredential
}

func disallowedStandingAccessType(accessType string) bool {
	switch accessType {
	case "", "cloud_admin", "inherited", "standing", "static", "unknown":
		return true
	default:
		return false
	}
}

func CredentialTargetBinding(intent schemagate.IntentRequest) (string, error) {
	raw, err := json.Marshal(intent.Targets)
	if err != nil {
		return "", fmt.Errorf("marshal intent targets: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest intent targets: %w", err)
	}
	return digest, nil
}
