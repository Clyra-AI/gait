package gate

import (
	"testing"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestEvaluatePolicyCredentialProvenanceControls(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: protect-high-risk-credential
    effect: allow
    block_standing_credentials: true
    allowed_credential_sources: [aws_sts, github_oidc, vault_dynamic]
    allowed_credential_issuers: [sts.amazonaws.com, token.actions.githubusercontent.com, vault.example]
    allowed_credential_access_types: [jit]
    max_credential_ttl_seconds: 900
    require_jit_credential: true
    match:
      risk_classes: [high]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	base := baseIntent()
	base.Context.RiskClass = "high"
	base.Context.CredentialRef = "aws:sts:assumed-role/ci/1234567890"
	base.Context.CredentialSource = "aws_sts"
	base.Context.CredentialAccessType = "jit"
	base.Context.CredentialIssuer = "sts.amazonaws.com"
	base.Context.CredentialTTLSeconds = 600
	base.Context.RunID = "run-1"
	base.Context.JobID = "job-1"
	base.Context.CredentialRunBinding = "run-1"
	base.Context.CredentialJobBinding = "job-1"
	targetBinding, err := CredentialTargetBinding(base)
	if err != nil {
		t.Fatalf("credential target binding: %v", err)
	}
	base.Context.CredentialTargetBinding = targetBinding

	tests := []struct {
		name        string
		mutate      func(*schemagate.IntentRequest)
		wantVerdict string
		wantReason  string
	}{
		{
			name:        "valid_jit_credential_allows",
			mutate:      func(intent *schemagate.IntentRequest) {},
			wantVerdict: "allow",
		},
		{
			name: "valid_github_oidc_allows",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialRef = "github:oidc:run/123456"
				intent.Context.CredentialSource = "github_oidc"
				intent.Context.CredentialIssuer = "token.actions.githubusercontent.com"
			},
			wantVerdict: "allow",
		},
		{
			name: "valid_vault_dynamic_allows",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialRef = "vault:database/creds/app"
				intent.Context.CredentialSource = "vault_dynamic"
				intent.Context.CredentialIssuer = "vault.example"
			},
			wantVerdict: "allow",
		},
		{
			name: "static_pat_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialRef = "env:GITHUB_PAT:deadbeef"
				intent.Context.CredentialSource = "github_pat"
				intent.Context.CredentialAccessType = "static"
				intent.Context.CredentialIssuer = "github.com"
			},
			wantVerdict: "block",
			wantReason:  "standing_credential_disallowed",
		},
		{
			name: "aws_iam_user_cloud_admin_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialRef = "env:AWS_ACCESS_KEY_ID:deadbeef"
				intent.Context.CredentialAccessType = "cloud_admin"
				intent.Context.CredentialSource = "aws_iam_user"
				intent.Context.CredentialIssuer = "iam.amazonaws.com"
			},
			wantVerdict: "block",
			wantReason:  "standing_credential_disallowed",
		},
		{
			name: "inherited_env_credential_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialRef = "env:DEPLOY_TOKEN:deadbeef"
				intent.Context.CredentialAccessType = "inherited"
				intent.Context.CredentialSource = "env"
				intent.Context.CredentialIssuer = "env"
			},
			wantVerdict: "block",
			wantReason:  "standing_credential_disallowed",
		},
		{
			name: "unknown_source_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialRef = "unknown:deadbeef"
				intent.Context.CredentialSource = "unknown"
				intent.Context.CredentialAccessType = "unknown"
				intent.Context.CredentialIssuer = "unknown"
			},
			wantVerdict: "block",
			wantReason:  "credential_source_disallowed",
		},
		{
			name: "missing_credential_evidence_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialRef = ""
				intent.Context.CredentialSource = ""
				intent.Context.CredentialAccessType = ""
				intent.Context.CredentialIssuer = ""
				intent.Context.CredentialTTLSeconds = 0
			},
			wantVerdict: "block",
			wantReason:  "credential_evidence_missing",
		},
		{
			name: "ttl_exceeded_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialTTLSeconds = 3600
			},
			wantVerdict: "block",
			wantReason:  "credential_ttl_exceeded",
		},
		{
			name: "target_binding_mismatch_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialTargetBinding = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			wantVerdict: "block",
			wantReason:  "credential_target_binding_mismatch",
		},
		{
			name: "run_binding_mismatch_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialRunBinding = "run-2"
			},
			wantVerdict: "block",
			wantReason:  "credential_run_binding_mismatch",
		},
		{
			name: "job_binding_mismatch_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialJobBinding = "job-2"
			},
			wantVerdict: "block",
			wantReason:  "credential_job_binding_mismatch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := base
			test.mutate(&intent)
			outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{})
			if evalErr != nil {
				t.Fatalf("evaluate policy: %v", evalErr)
			}
			if outcome.Result.Verdict != test.wantVerdict {
				t.Fatalf("unexpected verdict: %#v", outcome.Result)
			}
			if test.wantReason != "" && !contains(outcome.Result.ReasonCodes, test.wantReason) {
				t.Fatalf("expected reason %q in %#v", test.wantReason, outcome.Result.ReasonCodes)
			}
		})
	}
}
