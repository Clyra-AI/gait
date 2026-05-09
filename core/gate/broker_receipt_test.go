package gate

import (
	"testing"

	"github.com/Clyra-AI/gait/core/credential"
)

func TestValidateBrokerCredentialReceipt(t *testing.T) {
	request := credential.Request{
		ToolName:      "tool.write",
		Identity:      "alice",
		RunID:         "run-1",
		JobID:         "job-1",
		Reference:     "egress",
		Scope:         []string{"export"},
		TargetBinding: "binding-1",
	}
	response, err := credential.Issue(credential.StubBroker{}, request)
	if err != nil {
		t.Fatalf("issue stub credential: %v", err)
	}

	reasons, violations := ValidateBrokerCredentialReceipt(PolicyRule{RequireJITCredential: true, MaxCredentialTTLSeconds: 600}, request, response, IntentBrokerBinding{
		TargetBinding: "binding-1",
		RunBinding:    "run-1",
		JobBinding:    "job-1",
	})
	if len(reasons) != 0 || len(violations) != 0 {
		t.Fatalf("expected valid broker receipt, got reasons=%#v violations=%#v", reasons, violations)
	}

	response.RequestDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	reasons, violations = ValidateBrokerCredentialReceipt(PolicyRule{}, request, response, IntentBrokerBinding{})
	if !contains(reasons, "broker_request_digest_mismatch") || !contains(violations, "broker_request_digest_mismatch") {
		t.Fatalf("expected request digest mismatch, got reasons=%#v violations=%#v", reasons, violations)
	}
}

func TestValidateBrokerCredentialReceiptCredentialPolicyReasons(t *testing.T) {
	request := credential.Request{
		ToolName:      "tool.write",
		Identity:      "alice",
		RunID:         "run-1",
		JobID:         "job-1",
		Reference:     "egress",
		Scope:         []string{"export"},
		TargetBinding: "binding-1",
	}
	response := credential.Response{
		IssuedBy:      "command",
		Source:        "command",
		AccessType:    "standing",
		Issuer:        "command",
		Scope:         []string{"export"},
		CredentialRef: "cmd:token",
		TargetBinding: "binding-1",
		RunBinding:    "run-1",
		JobBinding:    "job-1",
		RequestDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TTLSeconds:    60,
	}
	reasons, violations := ValidateBrokerCredentialReceipt(PolicyRule{
		BlockStandingCredentials:     true,
		AllowedCredentialSources:     []string{"env"},
		AllowedCredentialIssuers:     []string{"github.com"},
		AllowedCredentialAccessTypes: []string{"jit"},
	}, request, response, IntentBrokerBinding{})
	for _, reason := range []string{
		"standing_credential_disallowed",
		"credential_source_disallowed",
		"credential_issuer_disallowed",
		"credential_access_type_disallowed",
		"broker_request_digest_mismatch",
	} {
		if !contains(reasons, reason) {
			t.Fatalf("expected reason %q in %#v", reason, reasons)
		}
		if !contains(violations, reason) {
			t.Fatalf("expected violation %q in %#v", reason, violations)
		}
	}
}

func TestValidateBrokerCredentialReceiptProviderAllows(t *testing.T) {
	request := credential.Request{
		ToolName:      "tool.write",
		Identity:      "alice",
		RunID:         "run-1",
		JobID:         "job-1",
		Reference:     "highrisk-egress",
		Scope:         []string{"write"},
		TargetBinding: "binding-1",
	}
	requestDigest, err := credential.RequestDigest(request)
	if err != nil {
		t.Fatalf("request digest: %v", err)
	}

	tests := []struct {
		name     string
		response credential.Response
	}{
		{
			name: "aws_sts_jit_allows",
			response: credential.Response{
				IssuedBy:      "stub",
				Source:        "aws_sts",
				AccessType:    "jit",
				Issuer:        "sts.amazonaws.com",
				Scope:         []string{"write"},
				CredentialRef: "aws:sts:assumed-role/ci/1234567890",
				TargetBinding: "binding-1",
				RunBinding:    "run-1",
				JobBinding:    "job-1",
				RequestDigest: requestDigest,
				TTLSeconds:    600,
			},
		},
		{
			name: "github_oidc_jit_allows",
			response: credential.Response{
				IssuedBy:      "stub",
				Source:        "github_oidc",
				AccessType:    "jit",
				Issuer:        "token.actions.githubusercontent.com",
				Scope:         []string{"write"},
				CredentialRef: "github:oidc:run/123456",
				TargetBinding: "binding-1",
				RunBinding:    "run-1",
				JobBinding:    "job-1",
				RequestDigest: requestDigest,
				TTLSeconds:    600,
			},
		},
		{
			name: "vault_dynamic_jit_allows",
			response: credential.Response{
				IssuedBy:      "stub",
				Source:        "vault_dynamic",
				AccessType:    "jit",
				Issuer:        "vault.example",
				Scope:         []string{"write"},
				CredentialRef: "vault:database/creds/app",
				TargetBinding: "binding-1",
				RunBinding:    "run-1",
				JobBinding:    "job-1",
				RequestDigest: requestDigest,
				TTLSeconds:    600,
			},
		},
	}

	policyRule := PolicyRule{
		BlockStandingCredentials:     true,
		AllowedCredentialSources:     []string{"aws_sts", "github_oidc", "vault_dynamic"},
		AllowedCredentialIssuers:     []string{"sts.amazonaws.com", "token.actions.githubusercontent.com", "vault.example"},
		AllowedCredentialAccessTypes: []string{"jit"},
		MaxCredentialTTLSeconds:      900,
		RequireJITCredential:         true,
	}
	intentBinding := IntentBrokerBinding{
		ExpectedCredentialRef: "",
		TargetBinding:         "binding-1",
		RunBinding:            "run-1",
		JobBinding:            "job-1",
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reasons, violations := ValidateBrokerCredentialReceipt(policyRule, request, test.response, intentBinding)
			if len(reasons) != 0 || len(violations) != 0 {
				t.Fatalf("expected valid provider receipt, got reasons=%#v violations=%#v", reasons, violations)
			}
		})
	}
}

func TestValidateBrokerCredentialReceiptMismatchReasons(t *testing.T) {
	request := credential.Request{
		ToolName:      "tool.write",
		Identity:      "alice",
		RunID:         "run-1",
		JobID:         "job-1",
		Reference:     "egress",
		Scope:         []string{"export"},
		TargetBinding: "binding-1",
	}
	response := credential.Response{
		IssuedBy:      "command",
		Source:        "command",
		AccessType:    "standing",
		Issuer:        "command",
		Scope:         []string{"read"},
		CredentialRef: "cmd:wrong",
		TargetBinding: "binding-2",
		RunBinding:    "run-2",
		JobBinding:    "job-2",
		RequestDigest: "",
		TTLSeconds:    7200,
	}
	reasons, violations := ValidateBrokerCredentialReceipt(PolicyRule{
		RequireJITCredential:     true,
		BlockStandingCredentials: true,
		MaxCredentialTTLSeconds:  300,
	}, request, response, IntentBrokerBinding{
		ExpectedCredentialRef: "cmd:expected",
		TargetBinding:         "binding-1",
		RunBinding:            "run-1",
		JobBinding:            "job-1",
	})
	for _, reason := range []string{
		"broker_request_digest_missing",
		"broker_scope_mismatch",
		"broker_ttl_exceeded",
		"credential_not_jit",
		"standing_credential_disallowed",
		"broker_credential_ref_mismatch",
		"broker_target_binding_mismatch",
		"broker_run_binding_mismatch",
		"broker_job_binding_mismatch",
	} {
		if !contains(reasons, reason) {
			t.Fatalf("expected reason %q in %#v", reason, reasons)
		}
		if !contains(violations, reason) {
			t.Fatalf("expected violation %q in %#v", reason, violations)
		}
	}
}
