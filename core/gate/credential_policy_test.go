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
    allowed_credential_sources: [github_pat, aws_sts, env]
    allowed_credential_issuers: [github.com, sts.amazonaws.com]
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
	base.Context.CredentialRef = "env:GAIT_EXPORT:deadbeef"
	base.Context.CredentialSource = "github_pat"
	base.Context.CredentialAccessType = "jit"
	base.Context.CredentialIssuer = "github.com"
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
			name: "static_pat_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialAccessType = "static"
			},
			wantVerdict: "block",
			wantReason:  "standing_credential_disallowed",
		},
		{
			name: "cloud_admin_standing_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialAccessType = "cloud_admin"
				intent.Context.CredentialSource = "aws_sts"
				intent.Context.CredentialIssuer = "sts.amazonaws.com"
			},
			wantVerdict: "block",
			wantReason:  "standing_credential_disallowed",
		},
		{
			name: "unknown_source_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.CredentialSource = "unknown"
			},
			wantVerdict: "block",
			wantReason:  "credential_source_disallowed",
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
