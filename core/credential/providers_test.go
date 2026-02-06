package credential

import (
	"errors"
	"fmt"
	"os"
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
	fmt.Print(`{"issued_by":"command","credential_ref":"cmd:test-credential"}`)
	os.Exit(0)
}
