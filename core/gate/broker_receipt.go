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
	for _, x := range []struct {
		name      string
		want, got string
	}{{"contract_family", intentBinding.ContractFamilyID, response.ContractFamilyID}, {"contract_id", intentBinding.ContractID, response.ContractID}, {"proposal_digest", intentBinding.ProposalDigest, response.ProposalDigest}, {"activation_digest", intentBinding.ActivationDigest, response.ActivationDigest}, {"policy_digest", intentBinding.PolicyDigest, response.PolicyDigest}, {"approval_digest", intentBinding.ApprovalTokenDigest, response.ApprovalTokenDigest}, {"delegation_digest", intentBinding.DelegationDigest, response.DelegationDigest}, {"expected_outcome", intentBinding.ExpectedOutcome, response.ExpectedOutcome}} {
		if x.want != "" && x.got != x.want {
			code := "broker_" + x.name + "_mismatch"
			reasons = append(reasons, code)
			violations = append(violations, code)
		}
	}
	if intentBinding.ContractRevision > 0 && response.ContractRevision != intentBinding.ContractRevision {
		reasons = append(reasons, "broker_contract_revision_mismatch")
		violations = append(violations, "broker_contract_revision_mismatch")
	}
	for _, x := range []struct {
		name      string
		want, got []string
	}{{"effect_scope", intentBinding.EffectScope, response.EffectScope}, {"containment_scope", intentBinding.ContainmentScope, response.ContainmentScope}} {
		if len(x.want) > 0 && !containsAll(x.got, x.want) {
			code := "broker_" + x.name + "_mismatch"
			reasons = append(reasons, code)
			violations = append(violations, code)
		}
	}
	return uniqueSorted(reasons), uniqueSorted(violations)
}

type IntentBrokerBinding struct {
	ExpectedCredentialRef         string
	TargetBinding                 string
	RunBinding                    string
	JobBinding                    string
	ContractFamilyID              string
	ContractID                    string
	ContractRevision              int
	ProposalDigest                string
	ActivationDigest              string
	PolicyDigest                  string
	ApprovalTokenDigest           string
	DelegationDigest              string
	ExpectedOutcome               string
	EffectScope, ContainmentScope []string
}
