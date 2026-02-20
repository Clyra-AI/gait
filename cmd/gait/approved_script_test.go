package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestApproveScriptAndListScripts(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy.yaml")
	intentPath := filepath.Join(workDir, "script_intent.json")
	registryPath := filepath.Join(workDir, "approved_scripts.json")
	privateKeyPath := filepath.Join(workDir, "approved_script_private.key")

	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	writePrivateKey(t, privateKeyPath)
	mustWriteScriptIntentFixture(t, intentPath)

	rawApprove := captureStdout(t, func() {
		if code := runApproveScript([]string{
			"--policy", policyPath,
			"--intent", intentPath,
			"--registry", registryPath,
			"--approver", "secops",
			"--key-mode", "prod",
			"--private-key", privateKeyPath,
			"--json",
		}); code != exitOK {
			t.Fatalf("runApproveScript expected %d got %d", exitOK, code)
		}
	})
	var approveOut approveScriptOutput
	if err := json.Unmarshal([]byte(rawApprove), &approveOut); err != nil {
		t.Fatalf("decode approve-script output: %v raw=%q", err, rawApprove)
	}
	if !approveOut.OK || approveOut.PatternID == "" || approveOut.ScriptHash == "" {
		t.Fatalf("unexpected approve-script output: %#v", approveOut)
	}

	rawList := captureStdout(t, func() {
		if code := runListScripts([]string{
			"--registry", registryPath,
			"--json",
		}); code != exitOK {
			t.Fatalf("runListScripts expected %d got %d", exitOK, code)
		}
	})
	var listOut listScriptsOutput
	if err := json.Unmarshal([]byte(rawList), &listOut); err != nil {
		t.Fatalf("decode list-scripts output: %v raw=%q", err, rawList)
	}
	if !listOut.OK || listOut.Count != 1 {
		t.Fatalf("unexpected list-scripts output: %#v", listOut)
	}
}

func TestGateEvalApprovedScriptFastPath(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy_require_approval.yaml")
	intentPath := filepath.Join(workDir, "script_intent.json")
	registryPath := filepath.Join(workDir, "approved_scripts.json")
	privateKeyPath := filepath.Join(workDir, "approved_script_private.key")
	tracePath := filepath.Join(workDir, "trace.json")

	mustWriteFile(t, policyPath, `
default_verdict: allow
rules:
  - name: require-approval-write
    effect: require_approval
    match:
      tool_names: [tool.write]
`)
	mustWriteScriptIntentFixture(t, intentPath)
	writePrivateKey(t, privateKeyPath)

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--trace-out", tracePath,
		"--json",
	}); code != exitApprovalRequired {
		t.Fatalf("runGateEval without registry expected %d got %d", exitApprovalRequired, code)
	}

	if code := runApproveScript([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--registry", registryPath,
		"--approver", "secops",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runApproveScript expected %d got %d", exitOK, code)
	}

	rawEval := captureStdout(t, func() {
		if code := runGateEval([]string{
			"--policy", policyPath,
			"--intent", intentPath,
			"--approved-script-registry", registryPath,
			"--trace-out", tracePath,
			"--json",
		}); code != exitOK {
			t.Fatalf("runGateEval with registry expected %d got %d", exitOK, code)
		}
	})
	var evalOut gateEvalOutput
	if err := json.Unmarshal([]byte(rawEval), &evalOut); err != nil {
		t.Fatalf("decode gate eval output: %v raw=%q", err, rawEval)
	}
	if evalOut.Verdict != "allow" || !evalOut.PreApproved || evalOut.PatternID == "" {
		t.Fatalf("expected pre-approved allow output, got %#v", evalOut)
	}
}

func TestGateEvalApprovedScriptBypassesBlockingRule(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy_block.yaml")
	intentPath := filepath.Join(workDir, "script_intent.json")
	registryPath := filepath.Join(workDir, "approved_scripts.json")
	privateKeyPath := filepath.Join(workDir, "approved_script_private.key")

	mustWriteFile(t, policyPath, `
default_verdict: allow
rules:
  - name: block-write
    effect: block
    reason_codes: [blocked_by_policy]
    match:
      tool_names: [tool.write]
`)
	mustWriteScriptIntentFixture(t, intentPath)
	writePrivateKey(t, privateKeyPath)

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--json",
	}); code != exitPolicyBlocked {
		t.Fatalf("runGateEval without registry expected %d got %d", exitPolicyBlocked, code)
	}

	if code := runApproveScript([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--registry", registryPath,
		"--approver", "secops",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runApproveScript expected %d got %d", exitOK, code)
	}

	rawEval := captureStdout(t, func() {
		if code := runGateEval([]string{
			"--policy", policyPath,
			"--intent", intentPath,
			"--approved-script-registry", registryPath,
			"--json",
		}); code != exitOK {
			t.Fatalf("runGateEval with registry expected %d got %d", exitOK, code)
		}
	})
	var evalOut gateEvalOutput
	if err := json.Unmarshal([]byte(rawEval), &evalOut); err != nil {
		t.Fatalf("decode gate eval output: %v raw=%q", err, rawEval)
	}
	if evalOut.Verdict != "allow" || !evalOut.PreApproved {
		t.Fatalf("expected pre-approved allow output, got %#v", evalOut)
	}
	if len(evalOut.ReasonCodes) != 1 || evalOut.ReasonCodes[0] != "approved_script_match" {
		t.Fatalf("expected fast-path reason only, got %#v", evalOut.ReasonCodes)
	}
}

func TestRunApproveScriptValidation(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy.yaml")
	intentPath := filepath.Join(workDir, "script_intent.json")
	registryPath := filepath.Join(workDir, "approved_scripts.json")

	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	mustWriteScriptIntentFixture(t, intentPath)

	rawMissingRequired := captureStdout(t, func() {
		if code := runApproveScript([]string{"--json"}); code != exitInvalidInput {
			t.Fatalf("runApproveScript missing required flags expected %d got %d", exitInvalidInput, code)
		}
	})
	var missingRequiredOut approveScriptOutput
	if err := json.Unmarshal([]byte(rawMissingRequired), &missingRequiredOut); err != nil {
		t.Fatalf("decode missing-required output: %v raw=%q", err, rawMissingRequired)
	}
	if missingRequiredOut.OK || !strings.Contains(missingRequiredOut.Error, "are required") {
		t.Fatalf("unexpected missing-required output: %#v", missingRequiredOut)
	}

	rawInvalidTTL := captureStdout(t, func() {
		if code := runApproveScript([]string{
			"--policy", policyPath,
			"--intent", intentPath,
			"--registry", registryPath,
			"--approver", "secops",
			"--ttl", "not-a-duration",
			"--json",
		}); code != exitInvalidInput {
			t.Fatalf("runApproveScript invalid ttl expected %d got %d", exitInvalidInput, code)
		}
	})
	var invalidTTLOut approveScriptOutput
	if err := json.Unmarshal([]byte(rawInvalidTTL), &invalidTTLOut); err != nil {
		t.Fatalf("decode invalid ttl output: %v raw=%q", err, rawInvalidTTL)
	}
	if invalidTTLOut.OK || invalidTTLOut.Error != "invalid --ttl duration" {
		t.Fatalf("unexpected invalid ttl output: %#v", invalidTTLOut)
	}
}

func TestRunApproveScriptRejectsIntentWithoutScript(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy.yaml")
	intentPath := filepath.Join(workDir, "intent_no_script.json")
	registryPath := filepath.Join(workDir, "approved_scripts.json")

	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	writeIntentFixture(t, intentPath, "tool.write")

	raw := captureStdout(t, func() {
		if code := runApproveScript([]string{
			"--policy", policyPath,
			"--intent", intentPath,
			"--registry", registryPath,
			"--approver", "secops",
			"--json",
		}); code != exitInvalidInput {
			t.Fatalf("runApproveScript intent without script expected %d got %d", exitInvalidInput, code)
		}
	})
	var out approveScriptOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode approve-script output: %v raw=%q", err, raw)
	}
	if out.OK || out.Error != "intent must include script.steps" {
		t.Fatalf("unexpected approve-script output: %#v", out)
	}
}

func TestRunApproveAndListScriptsHelpAndValidation(t *testing.T) {
	approveHelp := captureStdout(t, func() {
		if code := runApproveScript([]string{"--help"}); code != exitOK {
			t.Fatalf("runApproveScript help expected %d got %d", exitOK, code)
		}
	})
	if !strings.Contains(approveHelp, "gait approve-script") {
		t.Fatalf("approve-script help missing usage: %q", approveHelp)
	}

	listHelp := captureStdout(t, func() {
		if code := runListScripts([]string{"--help"}); code != exitOK {
			t.Fatalf("runListScripts help expected %d got %d", exitOK, code)
		}
	})
	if !strings.Contains(listHelp, "gait list-scripts") {
		t.Fatalf("list-scripts help missing usage: %q", listHelp)
	}

	rawMissingRegistry := captureStdout(t, func() {
		if code := runListScripts([]string{"--json"}); code != exitInvalidInput {
			t.Fatalf("runListScripts missing registry expected %d got %d", exitInvalidInput, code)
		}
	})
	var missingRegistryOut listScriptsOutput
	if err := json.Unmarshal([]byte(rawMissingRegistry), &missingRegistryOut); err != nil {
		t.Fatalf("decode list-scripts missing-registry output: %v raw=%q", err, rawMissingRegistry)
	}
	if missingRegistryOut.OK || missingRegistryOut.Error != "--registry is required" {
		t.Fatalf("unexpected missing-registry output: %#v", missingRegistryOut)
	}
}

func mustWriteScriptIntentFixture(t *testing.T, path string) {
	t.Helper()
	intent := schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		ToolName:        "script",
		Args:            map[string]any{},
		Targets:         []schemagate.IntentTarget{},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
		Script: &schemagate.IntentScript{
			Steps: []schemagate.IntentScriptStep{
				{
					ToolName: "tool.write",
					Args:     map[string]any{"path": "/tmp/out.txt"},
					Targets: []schemagate.IntentTarget{
						{Kind: "path", Value: "/tmp/out.txt", Operation: "write"},
					},
				},
			},
		},
	}
	raw, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatalf("marshal script intent: %v", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write script intent fixture: %v", err)
	}
}
