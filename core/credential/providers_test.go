package credential

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStubBrokerIssueDeterministic(t *testing.T) {
	request := Request{
		ToolName:  "tool.write",
		Identity:  "alice",
		Workspace: "/repo/gait",
		Reference: "egress",
		Scope:     []string{"export"},
	}
	first, err := Issue(StubBroker{}, request)
	if err != nil {
		t.Fatalf("issue with stub broker: %v", err)
	}
	second, err := Issue(StubBroker{}, request)
	if err != nil {
		t.Fatalf("issue with stub broker (second): %v", err)
	}
	if first.CredentialRef == "" || first.CredentialRef != second.CredentialRef {
		t.Fatalf("expected deterministic stub refs, first=%#v second=%#v", first, second)
	}
}

func TestEnvBrokerIssue(t *testing.T) {
	const envKey = "GAIT_TEST_BROKER_TOKEN_TOOL_WRITE"
	const envValue = "secret-token"
	if err := os.Setenv(envKey, envValue); err != nil {
		t.Fatalf("set env: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv(envKey)
	})

	response, err := Issue(EnvBroker{Prefix: "GAIT_TEST_BROKER_TOKEN_"}, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err != nil {
		t.Fatalf("issue with env broker: %v", err)
	}
	if !strings.HasPrefix(response.CredentialRef, "env:"+envKey+":") {
		t.Fatalf("unexpected env credential ref: %#v", response)
	}
}

func TestEnvBrokerUnavailable(t *testing.T) {
	_, err := Issue(EnvBroker{Prefix: "GAIT_MISSING_TOKEN_"}, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected unavailable env credential")
	}
	if !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("expected ErrCredentialUnavailable, got %v", err)
	}
}

func TestCommandBrokerIssue(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	broker := CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
	}
	t.Setenv("GAIT_TEST_COMMAND_BROKER_HELPER", "1")
	response, err := Issue(broker, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err != nil {
		t.Fatalf("command broker issue: %v", err)
	}
	if response.CredentialRef != "cmd:test-credential" {
		t.Fatalf("unexpected command broker credential ref: %#v", response)
	}
}

func TestCommandBrokerIssueProviderStyleReceipts(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("GAIT_TEST_COMMAND_BROKER_HELPER", "1")

	tests := []struct {
		name              string
		providerStyle     string
		wantSource        string
		wantIssuer        string
		wantCredentialRef string
	}{
		{
			name:              "aws_sts",
			providerStyle:     "aws_sts",
			wantSource:        "aws_sts",
			wantIssuer:        "sts.amazonaws.com",
			wantCredentialRef: "aws_sts:arn:aws:sts::123456789012:assumed-role/deploy/ci",
		},
		{
			name:              "github_oidc",
			providerStyle:     "github_oidc",
			wantSource:        "github_oidc",
			wantIssuer:        "token.actions.githubusercontent.com",
			wantCredentialRef: "github_oidc:Clyra-AI/gait:main/.github/workflows/release.yml@refs/heads/main",
		},
		{
			name:              "vault_dynamic",
			providerStyle:     "vault_dynamic",
			wantSource:        "vault_dynamic",
			wantIssuer:        "vault.example",
			wantCredentialRef: "vault_dynamic:database/creds/deploy/1234",
		},
		{
			name:              "gcp_sts",
			providerStyle:     "gcp_sts",
			wantSource:        "gcp_sts",
			wantIssuer:        "sts.googleapis.com",
			wantCredentialRef: "gcp_sts:principal://iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/subject/release-bot",
		},
		{
			name:              "azure_federated",
			providerStyle:     "azure_federated",
			wantSource:        "azure_federated",
			wantIssuer:        "login.microsoftonline.com",
			wantCredentialRef: "azure_federated:11111111-2222-3333-4444-555555555555:release-bot",
		},
		{
			name:              "okta_cyberark",
			providerStyle:     "okta_cyberark",
			wantSource:        "okta_cyberark",
			wantIssuer:        "okta.example/cyberark",
			wantCredentialRef: "okta_cyberark:session-1234",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GAIT_TEST_COMMAND_BROKER_PROVIDER_STYLE", test.providerStyle)
			response, issueErr := Issue(CommandBroker{
				Command: executable,
				Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
			}, Request{
				ToolName:      "tool.write",
				Identity:      "alice",
				RunID:         "run-1",
				JobID:         "job-1",
				Reference:     "highrisk-egress",
				Scope:         []string{"write"},
				TargetBinding: "binding-1",
			})
			if issueErr != nil {
				t.Fatalf("issue with provider-style broker: %v", issueErr)
			}
			if response.Source != test.wantSource || response.Issuer != test.wantIssuer || response.CredentialRef != test.wantCredentialRef {
				t.Fatalf("unexpected provider-style broker response: %#v", response)
			}
			if response.RequestDigest == "" || response.TargetBinding != "binding-1" || response.RunBinding != "run-1" || response.JobBinding != "job-1" {
				t.Fatalf("expected compatibility fields on provider-style broker response: %#v", response)
			}
		})
	}
}

func TestCommandBrokerIssueDefaultsCompatibilityFieldsFromRequest(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("GAIT_TEST_COMMAND_BROKER_HELPER", "1")
	t.Setenv("GAIT_TEST_COMMAND_BROKER_OMIT_COMPAT", "1")
	response, err := Issue(CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
	}, Request{
		ToolName:      "tool.write",
		Identity:      "alice",
		RunID:         "run-1",
		JobID:         "job-1",
		Scope:         []string{"execute"},
		TargetBinding: "binding-1",
	})
	if err != nil {
		t.Fatalf("issue with compatibility broker: %v", err)
	}
	if strings.Join(response.Scope, ",") != "execute" {
		t.Fatalf("expected scope to default from request, got %#v", response.Scope)
	}
	if response.TargetBinding != "binding-1" {
		t.Fatalf("expected target binding to default from request, got %#v", response)
	}
	if response.RunBinding != "run-1" || response.JobBinding != "job-1" {
		t.Fatalf("expected run/job binding to default from request, got %#v", response)
	}
}

func TestCommandBrokerFailure(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	broker := CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
	}
	t.Setenv("GAIT_TEST_COMMAND_BROKER_HELPER", "1")
	t.Setenv("GAIT_TEST_COMMAND_BROKER_FAIL", "1")
	_, err = Issue(broker, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected command broker failure")
	}
	if !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("expected ErrCredentialUnavailable, got %v", err)
	}
}

func TestResolveBroker(t *testing.T) {
	if _, err := ResolveBroker("bad", "", "", nil); err == nil {
		t.Fatalf("expected unsupported broker error")
	}
	if _, err := ResolveBroker("stub", "", "", nil); err != nil {
		t.Fatalf("resolve stub broker: %v", err)
	}
	if _, err := ResolveBroker("env", "GAIT_TEST_BROKER_TOKEN_", "", nil); err != nil {
		t.Fatalf("resolve env broker: %v", err)
	}
	broker, err := ResolveBroker("off", "", "", nil)
	if err != nil {
		t.Fatalf("resolve off broker: %v", err)
	}
	if broker != nil {
		t.Fatalf("expected nil broker for off")
	}
	commandBroker, err := ResolveBroker("command", "", "echo", []string{"ok"})
	if err != nil {
		t.Fatalf("resolve command broker: %v", err)
	}
	if commandBroker == nil {
		t.Fatalf("expected command broker")
	}
	if _, err := ResolveBroker("command", "", "", nil); err == nil {
		t.Fatalf("expected command broker missing-command error")
	}
}

func TestResolveBrokerCommandAllowlist(t *testing.T) {
	t.Setenv(commandAllowlistEnv, "/bin/allowed")
	if _, err := ResolveBroker("command", "", "/bin/not-allowed", nil); err == nil {
		t.Fatalf("expected allowlist rejection")
	}
	t.Setenv(commandAllowlistEnv, "/bin/allowed,/bin/not-allowed")
	if _, err := ResolveBroker("command", "", "/bin/not-allowed", nil); err != nil {
		t.Fatalf("expected allowed command, got: %v", err)
	}
}

func TestBrokerNames(t *testing.T) {
	if (StubBroker{}).Name() != "stub" {
		t.Fatalf("unexpected stub broker name")
	}
	if (EnvBroker{}).Name() != "env" {
		t.Fatalf("unexpected env broker name")
	}
	if (CommandBroker{}).Name() != "command" {
		t.Fatalf("unexpected command broker name")
	}
}

func TestIssueRequiresBroker(t *testing.T) {
	_, err := Issue(nil, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected broker required error")
	}
}

func TestNormalizeRequestValidation(t *testing.T) {
	if _, err := normalizeRequest(Request{}); err == nil {
		t.Fatalf("expected missing tool_name validation")
	}
	if _, err := normalizeRequest(Request{ToolName: "tool.write"}); err == nil {
		t.Fatalf("expected missing identity validation")
	}
	normalized, err := normalizeRequest(Request{
		ToolName: " TOOL.WRITE ",
		Identity: " alice ",
		Scope:    []string{" export ", "export", " read "},
	})
	if err != nil {
		t.Fatalf("normalize request: %v", err)
	}
	if normalized.ToolName != "tool.write" || normalized.Identity != "alice" {
		t.Fatalf("unexpected normalized request: %#v", normalized)
	}
	if strings.Join(normalized.Scope, ",") != "export,read" {
		t.Fatalf("unexpected normalized scope: %#v", normalized.Scope)
	}
}

func TestParseInt64(t *testing.T) {
	value, err := parseInt64(" 42 ")
	if err != nil || value != 42 {
		t.Fatalf("parseInt64 expected 42, got value=%d err=%v", value, err)
	}
	if _, err := parseInt64(""); err == nil {
		t.Fatalf("expected parseInt64 to reject empty input")
	}
	if _, err := parseInt64("invalid"); err == nil {
		t.Fatalf("expected parseInt64 to reject invalid integer")
	}
}

func TestNormalizeProviderReceipt(t *testing.T) {
	request := Request{
		ToolName:      "tool.write",
		Identity:      "alice",
		RunID:         "run-1",
		JobID:         "job-1",
		Reference:     "highrisk-egress",
		Scope:         []string{"write"},
		TargetBinding: "binding-1",
	}

	tests := []struct {
		name              string
		payload           string
		wantSource        string
		wantIssuer        string
		wantCredentialRef string
	}{
		{
			name: "aws_sts",
			payload: `{
  "provider":"aws_sts",
  "issuer":"sts.amazonaws.com",
  "assumed_role_arn":"arn:aws:sts::123456789012:assumed-role/deploy/ci",
  "scope":["write"],
  "issued_at":"2026-05-09T12:00:00Z",
  "expires_at":"2026-05-09T12:10:00Z"
}`,
			wantSource:        "aws_sts",
			wantIssuer:        "sts.amazonaws.com",
			wantCredentialRef: "aws_sts:arn:aws:sts::123456789012:assumed-role/deploy/ci",
		},
		{
			name: "github_oidc",
			payload: `{
  "provider":"github_oidc",
  "repository":"Clyra-AI/gait",
  "workflow_ref":"main/.github/workflows/release.yml@refs/heads/main",
  "scope":["write"]
}`,
			wantSource:        "github_oidc",
			wantIssuer:        "token.actions.githubusercontent.com",
			wantCredentialRef: "github_oidc:Clyra-AI/gait:main/.github/workflows/release.yml@refs/heads/main",
		},
		{
			name: "vault_dynamic",
			payload: `{
  "provider":"vault_dynamic",
  "issuer":"vault.example",
  "lease_id":"database/creds/deploy/1234",
  "scope":["write"]
}`,
			wantSource:        "vault_dynamic",
			wantIssuer:        "vault.example",
			wantCredentialRef: "vault_dynamic:database/creds/deploy/1234",
		},
		{
			name: "gcp_sts",
			payload: `{
  "provider":"gcp_sts",
  "principal":"principal://iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/subject/release-bot",
  "scope":["write"]
}`,
			wantSource:        "gcp_sts",
			wantIssuer:        "sts.googleapis.com",
			wantCredentialRef: "gcp_sts:principal://iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/subject/release-bot",
		},
		{
			name: "azure_federated",
			payload: `{
  "provider":"azure_federated",
  "service_principal_id":"11111111-2222-3333-4444-555555555555",
  "federated_credential_id":"release-bot",
  "scope":["write"]
}`,
			wantSource:        "azure_federated",
			wantIssuer:        "login.microsoftonline.com",
			wantCredentialRef: "azure_federated:11111111-2222-3333-4444-555555555555:release-bot",
		},
		{
			name: "okta_cyberark",
			payload: `{
  "provider":"okta_cyberark",
  "issuer":"okta.example/cyberark",
  "broker_session_id":"session-1234",
  "scope":["write"]
}`,
			wantSource:        "okta_cyberark",
			wantIssuer:        "okta.example/cyberark",
			wantCredentialRef: "okta_cyberark:session-1234",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := NormalizeProviderReceipt("", []byte(test.payload), request)
			if err != nil {
				t.Fatalf("normalize provider receipt: %v", err)
			}
			if response.Source != test.wantSource || response.Issuer != test.wantIssuer || response.CredentialRef != test.wantCredentialRef {
				t.Fatalf("unexpected normalized provider receipt: %#v", response)
			}
			if response.RequestDigest == "" || response.TargetBinding != "binding-1" || response.RunBinding != "run-1" || response.JobBinding != "job-1" {
				t.Fatalf("expected derived request/binding fields: %#v", response)
			}
		})
	}
}

func TestNormalizeProviderReceiptRejectsSecretLikeFields(t *testing.T) {
	_, err := NormalizeProviderReceipt("", []byte(`{
  "provider":"vault_dynamic",
  "lease_id":"database/creds/deploy/1234",
  "credential_ref":"sk-secret-token-value"
}`), Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected secret-like provider receipt rejection")
	}
}

func TestNormalizeProviderReceiptUnsupportedAndTimestampValidation(t *testing.T) {
	if _, err := NormalizeProviderReceipt("", []byte(`{"scope":["write"]}`), Request{
		ToolName: "tool.write",
		Identity: "alice",
	}); !errors.Is(err, errUnsupportedProviderReceipt) {
		t.Fatalf("expected unsupported provider receipt error, got %v", err)
	}

	_, err := NormalizeProviderReceipt("", []byte(`{
  "provider":"aws_sts",
  "assumed_role_arn":"arn:aws:sts::123456789012:assumed-role/deploy/ci",
  "issued_at":"not-a-time"
}`), Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected timestamp validation error")
	}
}

func TestProviderReceiptHelperBranches(t *testing.T) {
	if got := normalizeProviderName(" AWS "); got != "aws_sts" {
		t.Fatalf("unexpected normalized provider name: %q", got)
	}
	if got := normalizeProviderName("file"); got != "file" {
		t.Fatalf("unexpected normalized provider name: %q", got)
	}
	if got := normalizeProviderName("unknown"); got != "" {
		t.Fatalf("expected unknown provider to normalize to empty string, got %q", got)
	}
	if got := defaultProviderIssuer("command"); got != "command" {
		t.Fatalf("unexpected default provider issuer: %q", got)
	}
	if got := defaultProviderIssuer("custom"); got != "custom" {
		t.Fatalf("unexpected passthrough provider issuer: %q", got)
	}

	if ref, err := deriveProviderCredentialRef("aws_sts", providerReceipt{SessionARN: "arn:aws:sts::123456789012:assumed-role/demo/session"}); err != nil || ref != "aws_sts:arn:aws:sts::123456789012:assumed-role/demo/session" {
		t.Fatalf("unexpected aws session fallback ref=%q err=%v", ref, err)
	}
	if ref, err := deriveProviderCredentialRef("github_oidc", providerReceipt{Repository: "Clyra-AI/gait"}); err != nil || ref != "github_oidc:Clyra-AI/gait" {
		t.Fatalf("unexpected github fallback ref=%q err=%v", ref, err)
	}
	if ref, err := deriveProviderCredentialRef("gcp_sts", providerReceipt{WorkloadIdentityProvider: "projects/123/locations/global/workloadIdentityPools/pool/providers/provider"}); err != nil || ref != "gcp_sts:projects/123/locations/global/workloadIdentityPools/pool/providers/provider" {
		t.Fatalf("unexpected gcp fallback ref=%q err=%v", ref, err)
	}
	if ref, err := deriveProviderCredentialRef("azure_federated", providerReceipt{ServicePrincipalID: "11111111-2222-3333-4444-555555555555"}); err != nil || ref != "azure_federated:11111111-2222-3333-4444-555555555555" {
		t.Fatalf("unexpected azure fallback ref=%q err=%v", ref, err)
	}
	if _, err := deriveProviderCredentialRef("okta_cyberark", providerReceipt{}); err == nil {
		t.Fatalf("expected missing okta/cyberark session id error")
	}
	if _, err := deriveProviderCredentialRef("command", providerReceipt{}); err == nil {
		t.Fatalf("expected command provider credential_ref requirement")
	}
	if _, err := deriveProviderCredentialRef("file", providerReceipt{}); err == nil {
		t.Fatalf("expected file provider credential_ref requirement")
	}
}

func TestCommandBrokerIssuePlainTextAndTimeout(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("GAIT_TEST_COMMAND_BROKER_HELPER", "1")
	t.Setenv("GAIT_TEST_COMMAND_BROKER_PLAIN", "1")
	broker := CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
	}
	_, err = Issue(broker, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected plain output to be rejected")
	}
	if !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("expected ErrCredentialUnavailable for plain output, got %v", err)
	}

	t.Setenv("GAIT_TEST_COMMAND_BROKER_PLAIN", "")
	t.Setenv("GAIT_TEST_COMMAND_BROKER_SLEEP", "1")
	_, err = Issue(CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
		Timeout: 20 * time.Millisecond,
	}, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("expected ErrCredentialUnavailable timeout, got %v", err)
	}
}

func TestCommandBrokerOutputLimitAndErrorRedaction(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("GAIT_TEST_COMMAND_BROKER_HELPER", "1")

	t.Setenv("GAIT_TEST_COMMAND_BROKER_LARGE", "1")
	_, err = Issue(CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
	}, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected output-size error")
	}
	if !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("expected ErrCredentialUnavailable for large output, got %v", err)
	}

	t.Setenv("GAIT_TEST_COMMAND_BROKER_LARGE", "")
	t.Setenv("GAIT_TEST_COMMAND_BROKER_FAIL", "1")
	t.Setenv("GAIT_TEST_COMMAND_BROKER_FAIL_TOKEN", "secret-token-value")
	_, err = Issue(CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
	}, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected command broker failure")
	}
	if strings.Contains(err.Error(), "secret-token-value") {
		t.Fatalf("command broker error leaked sensitive token: %v", err)
	}
}

func TestCommandBrokerCredentialRefValidation(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("GAIT_TEST_COMMAND_BROKER_HELPER", "1")

	t.Setenv("GAIT_TEST_COMMAND_BROKER_LONG_REF", "1")
	_, err = Issue(CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
	}, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected long credential_ref error")
	}
	if !strings.Contains(err.Error(), "credential_ref too long") {
		t.Fatalf("expected long credential_ref error, got: %v", err)
	}

	t.Setenv("GAIT_TEST_COMMAND_BROKER_LONG_REF", "")
	t.Setenv("GAIT_TEST_COMMAND_BROKER_CONTROL_REF", "1")
	_, err = Issue(CommandBroker{
		Command: executable,
		Args:    []string{"-test.run", "TestCommandBrokerHelperProcess", "--"},
	}, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected invalid credential_ref whitespace error")
	}
	if !strings.Contains(err.Error(), "credential_ref contains invalid whitespace") {
		t.Fatalf("expected invalid whitespace credential_ref error, got: %v", err)
	}
}

func TestCommandBrokerCommandValidation(t *testing.T) {
	_, err := Issue(CommandBroker{
		Command: "invalid command",
	}, Request{
		ToolName: "tool.write",
		Identity: "alice",
	})
	if err == nil {
		t.Fatalf("expected invalid command whitespace rejection")
	}
	if !strings.Contains(err.Error(), "must not contain whitespace") {
		t.Fatalf("expected whitespace command error, got: %v", err)
	}
}

func TestNormalizeCommandAllowlistAndMatch(t *testing.T) {
	allowlist := normalizeCommandAllowlist(" /usr/local/bin/broker , broker ,,/usr/local/bin/broker ")
	if len(allowlist) != 2 {
		t.Fatalf("unexpected allowlist normalization: %#v", allowlist)
	}
	if !isCommandAllowed("/usr/local/bin/broker", allowlist) {
		t.Fatalf("expected full path allowlist match")
	}
	if !isCommandAllowed("broker", allowlist) {
		t.Fatalf("expected basename allowlist match")
	}
	if isCommandAllowed("/usr/local/bin/other", allowlist) {
		t.Fatalf("unexpected allowlist match for other command")
	}
}

func TestCommandBrokerHelperProcess(t *testing.T) {
	if os.Getenv("GAIT_TEST_COMMAND_BROKER_HELPER") != "1" {
		t.Skip("helper process")
	}
	if os.Getenv("GAIT_TEST_COMMAND_BROKER_SLEEP") == "1" {
		time.Sleep(100 * time.Millisecond)
	}
	if os.Getenv("GAIT_TEST_COMMAND_BROKER_LARGE") == "1" {
		fmt.Print(strings.Repeat("x", defaultCommandOutputMaxBytes+512))
		os.Exit(0)
	}
	if os.Getenv("GAIT_TEST_COMMAND_BROKER_LONG_REF") == "1" {
		fmt.Printf(`{"issued_by":"command","credential_ref":"cmd:%s"}`, strings.Repeat("x", defaultCredentialRefMaxLength))
		os.Exit(0)
	}
	if os.Getenv("GAIT_TEST_COMMAND_BROKER_CONTROL_REF") == "1" {
		fmt.Print("{\"issued_by\":\"command\",\"credential_ref\":\"cmd:bad\\nref\"}")
		os.Exit(0)
	}
	if os.Getenv("GAIT_TEST_COMMAND_BROKER_FAIL") == "1" {
		token := os.Getenv("GAIT_TEST_COMMAND_BROKER_FAIL_TOKEN")
		if token == "" {
			token = "missing-token"
		}
		fmt.Fprintf(os.Stderr, "forced failure token=%s\n", token)
		os.Exit(2)
	}
	if os.Getenv("GAIT_TEST_COMMAND_BROKER_PLAIN") == "1" {
		fmt.Print("cmd:plain-ref")
		os.Exit(0)
	}
	if os.Getenv("GAIT_TEST_COMMAND_BROKER_OMIT_COMPAT") == "1" {
		fmt.Print(`{"issued_by":"command","credential_ref":"cmd:test-credential"}`)
		os.Exit(0)
	}
	if providerStyle := os.Getenv("GAIT_TEST_COMMAND_BROKER_PROVIDER_STYLE"); providerStyle != "" {
		request := Request{}
		_ = json.NewDecoder(os.Stdin).Decode(&request)
		requestDigest, _ := RequestDigest(request)
		switch providerStyle {
		case "aws_sts":
			fmt.Print(`{"provider":"aws_sts","issuer":"sts.amazonaws.com","assumed_role_arn":"arn:aws:sts::123456789012:assumed-role/deploy/ci","scope":["write"],"request_digest":"` + requestDigest + `"}`)
		case "github_oidc":
			fmt.Print(`{"provider":"github_oidc","repository":"Clyra-AI/gait","workflow_ref":"main/.github/workflows/release.yml@refs/heads/main","scope":["write"],"request_digest":"` + requestDigest + `"}`)
		case "vault_dynamic":
			fmt.Print(`{"provider":"vault_dynamic","issuer":"vault.example","lease_id":"database/creds/deploy/1234","scope":["write"],"request_digest":"` + requestDigest + `"}`)
		case "gcp_sts":
			fmt.Print(`{"provider":"gcp_sts","principal":"principal://iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/subject/release-bot","scope":["write"],"request_digest":"` + requestDigest + `"}`)
		case "azure_federated":
			fmt.Print(`{"provider":"azure_federated","service_principal_id":"11111111-2222-3333-4444-555555555555","federated_credential_id":"release-bot","scope":["write"],"request_digest":"` + requestDigest + `"}`)
		case "okta_cyberark":
			fmt.Print(`{"provider":"okta_cyberark","issuer":"okta.example/cyberark","broker_session_id":"session-1234","scope":["write"],"request_digest":"` + requestDigest + `"}`)
		default:
			fmt.Print(`{"provider":"` + providerStyle + `"}`)
		}
		os.Exit(0)
	}
	request := Request{}
	_ = json.NewDecoder(os.Stdin).Decode(&request)
	response := Response{
		IssuedBy:      "command",
		Source:        "command",
		AccessType:    "jit",
		Issuer:        "command",
		Subject:       request.Identity,
		Owner:         request.Identity,
		Scope:         request.Scope,
		CredentialRef: "cmd:test-credential",
		TargetBinding: request.TargetBinding,
		RunBinding:    request.RunID,
		JobBinding:    request.JobID,
	}
	_ = json.NewEncoder(os.Stdout).Encode(response)
	os.Exit(0)
}

func TestCredentialBrokerExampleFixturesParse(t *testing.T) {
	repoRoot := repoRootFromCredentialPackageDir(t)
	request := Request{
		ToolName:      "tool.write",
		Identity:      "alice",
		RunID:         "run-1",
		JobID:         "job-1",
		Reference:     "highrisk-egress",
		Scope:         []string{"write"},
		TargetBinding: "binding-1",
	}
	fixtures := []struct {
		name       string
		path       string
		wantSource string
	}{
		{name: "aws_sts", path: "examples/credential-brokers/aws_sts_receipt.json", wantSource: "aws_sts"},
		{name: "github_oidc", path: "examples/credential-brokers/github_oidc_receipt.json", wantSource: "github_oidc"},
		{name: "vault_dynamic", path: "examples/credential-brokers/vault_dynamic_receipt.json", wantSource: "vault_dynamic"},
		{name: "gcp_sts", path: "examples/credential-brokers/gcp_sts_receipt.json", wantSource: "gcp_sts"},
		{name: "azure_federated", path: "examples/credential-brokers/azure_federated_receipt.json", wantSource: "azure_federated"},
		{name: "okta_cyberark", path: "examples/credential-brokers/okta_cyberark_receipt.json", wantSource: "okta_cyberark"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join(repoRoot, fixture.path))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			response, err := NormalizeProviderReceipt("", payload, request)
			if err != nil {
				t.Fatalf("normalize fixture: %v", err)
			}
			if response.Source != fixture.wantSource {
				t.Fatalf("unexpected fixture source: %#v", response)
			}
		})
	}
}

func repoRootFromCredentialPackageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
