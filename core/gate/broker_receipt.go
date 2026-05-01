package gate

import (
	"strings"

	"github.com/Clyra-AI/gait/core/credential"
)

func ValidateBrokerCredentialReceipt(rule PolicyRule, request credential.Request, response credential.Response, intentBinding IntentBrokerBinding) ([]string, []string) {
	reasons := []string{}
	violations := []string{}
	source := strings.ToLower(strings.TrimSpace(response.Source))
	accessType := strings.ToLower(strings.TrimSpace(response.AccessType))
	issuer := strings.TrimSpace(response.Issuer)

	if response.RequestDigest == "" {
		reasons = append(reasons, "broker_request_digest_missing")
		violations = append(violations, "broker_request_digest_missing")
	} else if requestDigest, err := credential.RequestDigest(request); err != nil || response.RequestDigest != requestDigest {
		reasons = append(reasons, "broker_request_digest_mismatch")
		violations = append(violations, "broker_request_digest_mismatch")
	}
	if len(request.Scope) > 0 && !containsAll(response.Scope, request.Scope) {
		reasons = append(reasons, "broker_scope_mismatch")
		violations = append(violations, "broker_scope_mismatch")
	}
	if rule.MaxCredentialTTLSeconds > 0 {
		if response.TTLSeconds <= 0 || response.TTLSeconds > rule.MaxCredentialTTLSeconds {
			reasons = append(reasons, "broker_ttl_exceeded")
			violations = append(violations, "broker_ttl_exceeded")
		}
	}
	if rule.RequireJITCredential && accessType != "jit" {
		reasons = append(reasons, "credential_not_jit")
		violations = append(violations, "credential_not_jit")
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
	if intentBinding.ExpectedCredentialRef != "" && strings.TrimSpace(response.CredentialRef) != intentBinding.ExpectedCredentialRef {
		reasons = append(reasons, "broker_credential_ref_mismatch")
		violations = append(violations, "broker_credential_ref_mismatch")
	}
	if intentBinding.TargetBinding != "" && strings.TrimSpace(response.TargetBinding) != intentBinding.TargetBinding {
		reasons = append(reasons, "broker_target_binding_mismatch")
		violations = append(violations, "broker_target_binding_mismatch")
	}
	if intentBinding.RunBinding != "" && strings.TrimSpace(response.RunBinding) != intentBinding.RunBinding {
		reasons = append(reasons, "broker_run_binding_mismatch")
		violations = append(violations, "broker_run_binding_mismatch")
	}
	if intentBinding.JobBinding != "" && strings.TrimSpace(response.JobBinding) != intentBinding.JobBinding {
		reasons = append(reasons, "broker_job_binding_mismatch")
		violations = append(violations, "broker_job_binding_mismatch")
	}
	return uniqueSorted(reasons), uniqueSorted(violations)
}

type IntentBrokerBinding struct {
	ExpectedCredentialRef string
	TargetBinding         string
	RunBinding            string
	JobBinding            string
}
