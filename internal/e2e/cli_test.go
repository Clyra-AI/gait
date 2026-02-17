package e2e

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Clyra-AI/gait/core/sign"
)

func TestCLIDemoVerify(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)

	workDir := t.TempDir()
	demo := exec.Command(binPath, "demo")
	demo.Dir = workDir
	demoOut, err := demo.CombinedOutput()
	if err != nil {
		t.Fatalf("gait demo failed: %v\n%s", err, string(demoOut))
	}
	if !strings.Contains(string(demoOut), "run_id=") || !strings.Contains(string(demoOut), "verify=ok") {
		t.Fatalf("unexpected demo output: %s", string(demoOut))
	}

	verify := exec.Command(binPath, "verify", "run_demo")
	verify.Dir = workDir
	verifyOut, err := verify.CombinedOutput()
	if err != nil {
		t.Fatalf("gait verify failed: %v\n%s", err, string(verifyOut))
	}
	if !strings.Contains(string(verifyOut), "verify ok") {
		t.Fatalf("unexpected verify output: %s", string(verifyOut))
	}

	regressInit := exec.Command(binPath, "regress", "init", "--from", "run_demo", "--json")
	regressInit.Dir = workDir
	regressOut, err := regressInit.CombinedOutput()
	if err != nil {
		t.Fatalf("gait regress init failed: %v\n%s", err, string(regressOut))
	}
	var regressResult struct {
		OK          bool   `json:"ok"`
		RunID       string `json:"run_id"`
		ConfigPath  string `json:"config_path"`
		RunpackPath string `json:"runpack_path"`
	}
	if err := json.Unmarshal(regressOut, &regressResult); err != nil {
		t.Fatalf("parse regress init json output: %v\n%s", err, string(regressOut))
	}
	if !regressResult.OK || regressResult.RunID != "run_demo" {
		t.Fatalf("unexpected regress result: %s", string(regressOut))
	}
	if regressResult.ConfigPath != "gait.yaml" {
		t.Fatalf("unexpected config path: %s", regressResult.ConfigPath)
	}
	if regressResult.RunpackPath != "fixtures/run_demo/runpack.zip" {
		t.Fatalf("unexpected runpack path: %s", regressResult.RunpackPath)
	}
	if _, err := os.Stat(filepath.Join(workDir, "gait.yaml")); err != nil {
		t.Fatalf("expected gait.yaml to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "fixtures", "run_demo", "runpack.zip")); err != nil {
		t.Fatalf("expected fixture runpack to exist: %v", err)
	}

	regressRun := exec.Command(binPath, "regress", "run", "--json", "--junit", "junit.xml")
	regressRun.Dir = workDir
	regressRunOut, err := regressRun.CombinedOutput()
	if err != nil {
		t.Fatalf("gait regress run failed: %v\n%s", err, string(regressRunOut))
	}
	var regressRunResult struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
		Output string `json:"output"`
		JUnit  string `json:"junit"`
	}
	if err := json.Unmarshal(regressRunOut, &regressRunResult); err != nil {
		t.Fatalf("parse regress run json output: %v\n%s", err, string(regressRunOut))
	}
	if !regressRunResult.OK || regressRunResult.Status != "pass" {
		t.Fatalf("unexpected regress run result: %s", string(regressRunOut))
	}
	if regressRunResult.Output != "regress_result.json" {
		t.Fatalf("unexpected regress output path: %s", regressRunResult.Output)
	}
	if regressRunResult.JUnit != "junit.xml" {
		t.Fatalf("unexpected junit output path: %s", regressRunResult.JUnit)
	}
	if _, err := os.Stat(filepath.Join(workDir, "regress_result.json")); err != nil {
		t.Fatalf("expected regress_result.json to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "junit.xml")); err != nil {
		t.Fatalf("expected junit.xml to exist: %v", err)
	}
}

func TestCLIRegressExitCodes(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)

	workDir := t.TempDir()
	demo := exec.Command(binPath, "demo")
	demo.Dir = workDir
	if out, err := demo.CombinedOutput(); err != nil {
		t.Fatalf("gait demo failed: %v\n%s", err, string(out))
	}

	regressInit := exec.Command(binPath, "regress", "init", "--from", "run_demo", "--json")
	regressInit.Dir = workDir
	if out, err := regressInit.CombinedOutput(); err != nil {
		t.Fatalf("gait regress init failed: %v\n%s", err, string(out))
	}

	fixtureMetaPath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	rawMeta, err := os.ReadFile(fixtureMetaPath)
	if err != nil {
		t.Fatalf("read fixture metadata: %v", err)
	}
	var fixtureMeta map[string]any
	if err := json.Unmarshal(rawMeta, &fixtureMeta); err != nil {
		t.Fatalf("parse fixture metadata: %v", err)
	}
	fixtureMeta["expected_replay_exit_code"] = 2
	updatedMeta, err := json.MarshalIndent(fixtureMeta, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture metadata: %v", err)
	}
	updatedMeta = append(updatedMeta, '\n')
	if err := os.WriteFile(fixtureMetaPath, updatedMeta, 0o600); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}

	regressFail := exec.Command(binPath, "regress", "run", "--json")
	regressFail.Dir = workDir
	failOut, err := regressFail.CombinedOutput()
	if err == nil {
		t.Fatalf("expected regress run to fail with exit code 5")
	}
	if code := commandExitCode(t, err); code != 5 {
		t.Fatalf("unexpected regress failure exit code: got=%d want=5 output=%s", code, string(failOut))
	}
	var failResult struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(failOut, &failResult); err != nil {
		t.Fatalf("parse regress fail json: %v\n%s", err, string(failOut))
	}
	if failResult.OK || failResult.Status != "fail" {
		t.Fatalf("unexpected regress fail output: %s", string(failOut))
	}

	regressInvalid := exec.Command(binPath, "regress", "run", "--config", "missing.yaml", "--json")
	regressInvalid.Dir = workDir
	invalidOut, err := regressInvalid.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invalid regress invocation to fail with exit code 6")
	}
	if code := commandExitCode(t, err); code != 6 {
		t.Fatalf("unexpected invalid-input exit code: got=%d want=6 output=%s", code, string(invalidOut))
	}
}

func TestCLIGateEval(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)

	workDir := t.TempDir()
	intentPath := filepath.Join(workDir, "intent.json")
	intentContent := []byte(`{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-05T00:00:00Z",
  "producer_version": "0.0.0-dev",
  "tool_name": "tool.write",
  "args": {"path": "/tmp/out.txt"},
  "targets": [{"kind":"host","value":"api.external.com"}],
  "arg_provenance": [{"arg_path":"args.path","source":"user"}],
  "context": {"identity":"alice","workspace":"/repo/gait","risk_class":"high"}
}`)
	if err := os.WriteFile(intentPath, intentContent, 0o600); err != nil {
		t.Fatalf("write intent file: %v", err)
	}

	policyPath := filepath.Join(workDir, "policy.yaml")
	policyContent := []byte(`default_verdict: allow
fail_closed:
  enabled: true
  risk_classes: [high]
  required_fields: [targets, arg_provenance]
rules:
  - name: block-external-host
    effect: block
    match:
      tool_names: [tool.write]
      target_kinds: [host]
      target_values: [api.external.com]
    reason_codes: [blocked_external]
    violations: [external_target]
`)
	if err := os.WriteFile(policyPath, policyContent, 0o600); err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	privateKeyPath := filepath.Join(workDir, "trace_private.key")
	if err := os.WriteFile(privateKeyPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Private)), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	publicKeyPath := filepath.Join(workDir, "trace_public.key")
	if err := os.WriteFile(publicKeyPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Public)), 0o600); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	eval := exec.Command(
		binPath,
		"gate", "eval",
		"--policy", policyPath,
		"--intent", intentPath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	)
	eval.Dir = workDir
	evalOut, err := eval.CombinedOutput()
	if err != nil {
		t.Fatalf("gait gate eval failed: %v\n%s", err, string(evalOut))
	}
	var evalResult struct {
		OK          bool     `json:"ok"`
		Verdict     string   `json:"verdict"`
		ReasonCodes []string `json:"reason_codes"`
		Violations  []string `json:"violations"`
		TracePath   string   `json:"trace_path"`
	}
	if err := json.Unmarshal(evalOut, &evalResult); err != nil {
		t.Fatalf("parse gate eval json output: %v\n%s", err, string(evalOut))
	}
	if !evalResult.OK || evalResult.Verdict != "block" {
		t.Fatalf("unexpected gate eval result: %s", string(evalOut))
	}
	if len(evalResult.ReasonCodes) != 1 || evalResult.ReasonCodes[0] != "blocked_external" {
		t.Fatalf("unexpected gate reason codes: %#v", evalResult.ReasonCodes)
	}
	if len(evalResult.Violations) != 1 || evalResult.Violations[0] != "external_target" {
		t.Fatalf("unexpected gate violations: %#v", evalResult.Violations)
	}
	if evalResult.TracePath == "" {
		t.Fatalf("expected trace path in gate eval output: %s", string(evalOut))
	}
	if _, err := os.Stat(filepath.Join(workDir, evalResult.TracePath)); err != nil {
		t.Fatalf("expected trace record to exist: %v", err)
	}

	verifyTrace := exec.Command(binPath, "trace", "verify", evalResult.TracePath, "--public-key", publicKeyPath, "--json")
	verifyTrace.Dir = workDir
	verifyOut, err := verifyTrace.CombinedOutput()
	if err != nil {
		t.Fatalf("gait trace verify failed: %v\n%s", err, string(verifyOut))
	}
	var verifyResult struct {
		OK              bool   `json:"ok"`
		SignatureStatus string `json:"signature_status"`
	}
	if err := json.Unmarshal(verifyOut, &verifyResult); err != nil {
		t.Fatalf("parse trace verify output: %v\n%s", err, string(verifyOut))
	}
	if !verifyResult.OK || verifyResult.SignatureStatus != "verified" {
		t.Fatalf("unexpected trace verify output: %s", string(verifyOut))
	}

	tracePath := filepath.Join(workDir, evalResult.TracePath)
	traceBytes, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read emitted trace: %v", err)
	}
	var traceRecord map[string]any
	if err := json.Unmarshal(traceBytes, &traceRecord); err != nil {
		t.Fatalf("parse emitted trace: %v", err)
	}
	traceRecord["verdict"] = "allow"
	tamperedBytes, err := json.MarshalIndent(traceRecord, "", "  ")
	if err != nil {
		t.Fatalf("marshal tampered trace: %v", err)
	}
	tamperedBytes = append(tamperedBytes, '\n')
	if err := os.WriteFile(tracePath, tamperedBytes, 0o600); err != nil {
		t.Fatalf("write tampered trace: %v", err)
	}

	verifyTampered := exec.Command(binPath, "trace", "verify", evalResult.TracePath, "--public-key", publicKeyPath, "--json")
	verifyTampered.Dir = workDir
	tamperedOut, err := verifyTampered.CombinedOutput()
	if err == nil {
		t.Fatalf("expected tampered trace verification to fail with exit code 2")
	}
	if code := commandExitCode(t, err); code != 2 {
		t.Fatalf("unexpected tampered trace verify exit code: got=%d want=2 output=%s", code, string(tamperedOut))
	}

	invalid := exec.Command(binPath, "gate", "eval", "--policy", policyPath, "--json")
	invalid.Dir = workDir
	invalidOut, err := invalid.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invalid gate eval invocation to fail with exit code 6")
	}
	if code := commandExitCode(t, err); code != 6 {
		t.Fatalf("unexpected gate invalid-input exit code: got=%d want=6 output=%s", code, string(invalidOut))
	}
}

func TestCLIApproveAndGateRequireApprovalFlow(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)

	workDir := t.TempDir()
	intentPath := filepath.Join(workDir, "intent.json")
	intentContent := []byte(`{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-05T00:00:00Z",
  "producer_version": "0.0.0-dev",
  "tool_name": "tool.write",
  "args": {"path": "/tmp/out.txt"},
  "targets": [{"kind":"host","value":"api.external.com"}],
  "arg_provenance": [{"arg_path":"args.path","source":"user"}],
  "context": {"identity":"alice","workspace":"/repo/gait","risk_class":"high"}
}`)
	if err := os.WriteFile(intentPath, intentContent, 0o600); err != nil {
		t.Fatalf("write intent file: %v", err)
	}

	policyPath := filepath.Join(workDir, "policy.yaml")
	policyContent := []byte(`default_verdict: require_approval
rules:
  - name: require-approval-write
    effect: require_approval
    match:
      tool_names: [tool.write]
    reason_codes: [approval_required]
`)
	if err := os.WriteFile(policyPath, policyContent, 0o600); err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	traceKeyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate trace key pair: %v", err)
	}
	tracePrivateKeyPath := filepath.Join(workDir, "trace_private.key")
	if err := os.WriteFile(tracePrivateKeyPath, []byte(base64.StdEncoding.EncodeToString(traceKeyPair.Private)), 0o600); err != nil {
		t.Fatalf("write trace private key: %v", err)
	}

	approvalKeyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate approval key pair: %v", err)
	}
	approvalPrivateKeyPath := filepath.Join(workDir, "approval_private.key")
	if err := os.WriteFile(approvalPrivateKeyPath, []byte(base64.StdEncoding.EncodeToString(approvalKeyPair.Private)), 0o600); err != nil {
		t.Fatalf("write approval private key: %v", err)
	}
	approvalPublicKeyPath := filepath.Join(workDir, "approval_public.key")
	if err := os.WriteFile(approvalPublicKeyPath, []byte(base64.StdEncoding.EncodeToString(approvalKeyPair.Public)), 0o600); err != nil {
		t.Fatalf("write approval public key: %v", err)
	}

	evalMissingApproval := exec.Command(
		binPath,
		"gate", "eval",
		"--policy", policyPath,
		"--intent", intentPath,
		"--key-mode", "prod",
		"--private-key", tracePrivateKeyPath,
		"--json",
	)
	evalMissingApproval.Dir = workDir
	missingOut, err := evalMissingApproval.CombinedOutput()
	if err == nil {
		t.Fatalf("expected gate eval to require approval with exit code 4")
	}
	if code := commandExitCode(t, err); code != 4 {
		t.Fatalf("unexpected require-approval exit code: got=%d want=4 output=%s", code, string(missingOut))
	}

	var missingResult struct {
		OK           bool     `json:"ok"`
		Verdict      string   `json:"verdict"`
		ReasonCodes  []string `json:"reason_codes"`
		TracePath    string   `json:"trace_path"`
		PolicyDigest string   `json:"policy_digest"`
		IntentDigest string   `json:"intent_digest"`
	}
	if err := json.Unmarshal(missingOut, &missingResult); err != nil {
		t.Fatalf("parse require-approval output: %v\n%s", err, string(missingOut))
	}
	if !missingResult.OK || missingResult.Verdict != "require_approval" {
		t.Fatalf("unexpected require-approval result: %s", string(missingOut))
	}
	if !containsString(missingResult.ReasonCodes, "approval_token_missing") {
		t.Fatalf("expected approval_token_missing reason, got: %#v", missingResult.ReasonCodes)
	}
	if missingResult.TracePath == "" {
		t.Fatalf("expected trace path from require-approval run")
	}

	approve := exec.Command(
		binPath,
		"approve",
		"--intent-digest", missingResult.IntentDigest,
		"--policy-digest", missingResult.PolicyDigest,
		"--ttl", "1h",
		"--scope", "tool:tool.write",
		"--approver", "alice",
		"--reason-code", "change_ticket",
		"--key-mode", "prod",
		"--private-key", approvalPrivateKeyPath,
		"--json",
	)
	approve.Dir = workDir
	approveOut, err := approve.CombinedOutput()
	if err != nil {
		t.Fatalf("gait approve failed: %v\n%s", err, string(approveOut))
	}
	var approveResult struct {
		OK        bool   `json:"ok"`
		TokenID   string `json:"token_id"`
		TokenPath string `json:"token_path"`
	}
	if err := json.Unmarshal(approveOut, &approveResult); err != nil {
		t.Fatalf("parse approve output: %v\n%s", err, string(approveOut))
	}
	if !approveResult.OK || approveResult.TokenPath == "" || approveResult.TokenID == "" {
		t.Fatalf("unexpected approve output: %s", string(approveOut))
	}

	evalApproved := exec.Command(
		binPath,
		"gate", "eval",
		"--policy", policyPath,
		"--intent", intentPath,
		"--approval-token", approveResult.TokenPath,
		"--approval-public-key", approvalPublicKeyPath,
		"--key-mode", "prod",
		"--private-key", tracePrivateKeyPath,
		"--json",
	)
	evalApproved.Dir = workDir
	approvedOut, err := evalApproved.CombinedOutput()
	if err != nil {
		t.Fatalf("gate eval with valid approval failed: %v\n%s", err, string(approvedOut))
	}
	var approvedResult struct {
		OK          bool     `json:"ok"`
		Verdict     string   `json:"verdict"`
		ReasonCodes []string `json:"reason_codes"`
		ApprovalRef string   `json:"approval_ref"`
	}
	if err := json.Unmarshal(approvedOut, &approvedResult); err != nil {
		t.Fatalf("parse approved output: %v\n%s", err, string(approvedOut))
	}
	if !approvedResult.OK || approvedResult.Verdict != "allow" {
		t.Fatalf("unexpected approved gate result: %s", string(approvedOut))
	}
	if !containsString(approvedResult.ReasonCodes, "approval_granted") {
		t.Fatalf("expected approval_granted reason, got: %#v", approvedResult.ReasonCodes)
	}
	if approvedResult.ApprovalRef != approveResult.TokenID {
		t.Fatalf("unexpected approval ref: got=%s want=%s", approvedResult.ApprovalRef, approveResult.TokenID)
	}

	approveMismatch := exec.Command(
		binPath,
		"approve",
		"--intent-digest", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--policy-digest", missingResult.PolicyDigest,
		"--ttl", "1h",
		"--scope", "tool:tool.write",
		"--approver", "alice",
		"--reason-code", "mismatch_check",
		"--key-mode", "prod",
		"--private-key", approvalPrivateKeyPath,
		"--json",
	)
	approveMismatch.Dir = workDir
	mismatchTokenOut, err := approveMismatch.CombinedOutput()
	if err != nil {
		t.Fatalf("gait approve mismatch token failed: %v\n%s", err, string(mismatchTokenOut))
	}
	var mismatchToken struct {
		OK        bool   `json:"ok"`
		TokenPath string `json:"token_path"`
	}
	if err := json.Unmarshal(mismatchTokenOut, &mismatchToken); err != nil {
		t.Fatalf("parse mismatch token output: %v\n%s", err, string(mismatchTokenOut))
	}
	if !mismatchToken.OK || mismatchToken.TokenPath == "" {
		t.Fatalf("unexpected mismatch token output: %s", string(mismatchTokenOut))
	}

	evalMismatch := exec.Command(
		binPath,
		"gate", "eval",
		"--policy", policyPath,
		"--intent", intentPath,
		"--approval-token", mismatchToken.TokenPath,
		"--approval-public-key", approvalPublicKeyPath,
		"--key-mode", "prod",
		"--private-key", tracePrivateKeyPath,
		"--json",
	)
	evalMismatch.Dir = workDir
	mismatchOut, err := evalMismatch.CombinedOutput()
	if err == nil {
		t.Fatalf("expected gate eval mismatch token to fail with exit code 4")
	}
	if code := commandExitCode(t, err); code != 4 {
		t.Fatalf("unexpected mismatch approval exit code: got=%d want=4 output=%s", code, string(mismatchOut))
	}
	var mismatchResult struct {
		OK          bool     `json:"ok"`
		Verdict     string   `json:"verdict"`
		ReasonCodes []string `json:"reason_codes"`
	}
	if err := json.Unmarshal(mismatchOut, &mismatchResult); err != nil {
		t.Fatalf("parse mismatch gate output: %v\n%s", err, string(mismatchOut))
	}
	if !mismatchResult.OK || mismatchResult.Verdict != "require_approval" {
		t.Fatalf("unexpected mismatch gate output: %s", string(mismatchOut))
	}
	if !containsString(mismatchResult.ReasonCodes, "approval_token_intent_mismatch") {
		t.Fatalf("expected mismatch reason code, got: %#v", mismatchResult.ReasonCodes)
	}
}

func TestCLIDelegateAndGateRequireDelegationFlow(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)
	workDir := t.TempDir()

	intentPath := filepath.Join(workDir, "intent_delegation.json")
	intentContent := []byte(`{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-11T00:00:00Z",
  "producer_version": "0.0.0-dev",
  "tool_name": "tool.write",
  "args": {"path": "/tmp/out.txt"},
  "targets": [{"kind":"path","value":"/tmp/out.txt","operation":"write","endpoint_class":"fs.write"}],
  "arg_provenance": [{"arg_path":"$.path","source":"user"}],
  "delegation": {
    "requester_identity":"agent.specialist",
    "scope_class":"write",
    "token_refs":["delegation_e2e"],
    "chain":[{"delegator_identity":"agent.lead","delegate_identity":"agent.specialist","scope_class":"write","token_ref":"delegation_e2e"}]
  },
  "context": {"identity":"agent.specialist","workspace":"/repo/gait","risk_class":"high","session_id":"sess-e2e"}
}`)
	if err := os.WriteFile(intentPath, intentContent, 0o600); err != nil {
		t.Fatalf("write delegation intent file: %v", err)
	}

	policyPath := filepath.Join(workDir, "delegation_policy.yaml")
	policyContent := []byte(`default_verdict: block
rules:
  - name: allow-delegated-write
    effect: allow
    match:
      tool_names: [tool.write]
      require_delegation: true
      allowed_delegator_identities: [agent.lead]
      allowed_delegate_identities: [agent.specialist]
      delegation_scopes: [write]
`)
	if err := os.WriteFile(policyPath, policyContent, 0o600); err != nil {
		t.Fatalf("write delegation policy file: %v", err)
	}

	traceKeyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate trace key pair: %v", err)
	}
	tracePrivateKeyPath := filepath.Join(workDir, "trace_private.key")
	if err := os.WriteFile(tracePrivateKeyPath, []byte(base64.StdEncoding.EncodeToString(traceKeyPair.Private)), 0o600); err != nil {
		t.Fatalf("write trace private key: %v", err)
	}

	delegationKeyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate delegation key pair: %v", err)
	}
	delegationPrivateKeyPath := filepath.Join(workDir, "delegation_private.key")
	if err := os.WriteFile(delegationPrivateKeyPath, []byte(base64.StdEncoding.EncodeToString(delegationKeyPair.Private)), 0o600); err != nil {
		t.Fatalf("write delegation private key: %v", err)
	}
	delegationPublicKeyPath := filepath.Join(workDir, "delegation_public.key")
	if err := os.WriteFile(delegationPublicKeyPath, []byte(base64.StdEncoding.EncodeToString(delegationKeyPair.Public)), 0o600); err != nil {
		t.Fatalf("write delegation public key: %v", err)
	}

	evalMissingDelegation := exec.Command(
		binPath,
		"gate", "eval",
		"--policy", policyPath,
		"--intent", intentPath,
		"--key-mode", "prod",
		"--private-key", tracePrivateKeyPath,
		"--json",
	)
	evalMissingDelegation.Dir = workDir
	missingOut, err := evalMissingDelegation.CombinedOutput()
	if err == nil {
		t.Fatalf("expected gate eval to require delegation token")
	}
	if code := commandExitCode(t, err); code != 3 {
		t.Fatalf("unexpected missing delegation exit code: got=%d want=3 output=%s", code, string(missingOut))
	}
	var missingResult struct {
		OK          bool     `json:"ok"`
		Verdict     string   `json:"verdict"`
		ReasonCodes []string `json:"reason_codes"`
	}
	if err := json.Unmarshal(missingOut, &missingResult); err != nil {
		t.Fatalf("parse missing delegation output: %v\n%s", err, string(missingOut))
	}
	if !missingResult.OK || missingResult.Verdict != "block" {
		t.Fatalf("unexpected missing delegation output: %s", string(missingOut))
	}
	if !containsString(missingResult.ReasonCodes, "delegation_token_missing") {
		t.Fatalf("expected delegation_token_missing reason, got: %#v", missingResult.ReasonCodes)
	}

	delegateMint := exec.Command(
		binPath,
		"delegate",
		"mint",
		"--delegator", "agent.lead",
		"--delegate", "agent.specialist",
		"--scope", "tool:tool.write",
		"--scope-class", "write",
		"--ttl", "1h",
		"--key-mode", "prod",
		"--private-key", delegationPrivateKeyPath,
		"--out", filepath.Join(workDir, "delegation_token.json"),
		"--json",
	)
	delegateMint.Dir = workDir
	delegateOut, err := delegateMint.CombinedOutput()
	if err != nil {
		t.Fatalf("delegate mint failed: %v\n%s", err, string(delegateOut))
	}
	var delegateResult struct {
		OK        bool   `json:"ok"`
		TokenPath string `json:"token_path"`
	}
	if err := json.Unmarshal(delegateOut, &delegateResult); err != nil {
		t.Fatalf("parse delegate output: %v\n%s", err, string(delegateOut))
	}
	if !delegateResult.OK || delegateResult.TokenPath == "" {
		t.Fatalf("unexpected delegate output: %s", string(delegateOut))
	}

	evalDelegated := exec.Command(
		binPath,
		"gate", "eval",
		"--policy", policyPath,
		"--intent", intentPath,
		"--delegation-token", delegateResult.TokenPath,
		"--delegation-public-key", delegationPublicKeyPath,
		"--key-mode", "prod",
		"--private-key", tracePrivateKeyPath,
		"--json",
	)
	evalDelegated.Dir = workDir
	delegatedOut, err := evalDelegated.CombinedOutput()
	if err != nil {
		t.Fatalf("gate eval with valid delegation failed: %v\n%s", err, string(delegatedOut))
	}
	var delegatedResult struct {
		OK          bool     `json:"ok"`
		Verdict     string   `json:"verdict"`
		ReasonCodes []string `json:"reason_codes"`
	}
	if err := json.Unmarshal(delegatedOut, &delegatedResult); err != nil {
		t.Fatalf("parse delegated gate output: %v\n%s", err, string(delegatedOut))
	}
	if !delegatedResult.OK || delegatedResult.Verdict != "allow" {
		t.Fatalf("unexpected delegated gate output: %s", string(delegatedOut))
	}
	if !containsString(delegatedResult.ReasonCodes, "delegation_granted") {
		t.Fatalf("expected delegation_granted reason, got: %#v", delegatedResult.ReasonCodes)
	}
}

func TestCLIPolicyTestExitCodes(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)
	workDir := t.TempDir()

	intentPath := filepath.Join(workDir, "intent.json")
	intentContent := []byte(`{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-05T00:00:00Z",
  "producer_version": "0.0.0-dev",
  "tool_name": "tool.write",
  "args": {"path": "/tmp/out.txt"},
  "targets": [{"kind":"path","value":"/tmp/out.txt"}],
  "arg_provenance": [{"arg_path":"args.path","source":"user"}],
  "context": {"identity":"alice","workspace":"/repo/gait","risk_class":"high"}
}`)
	if err := os.WriteFile(intentPath, intentContent, 0o600); err != nil {
		t.Fatalf("write intent fixture: %v", err)
	}

	allowPolicyPath := filepath.Join(workDir, "allow.yaml")
	if err := os.WriteFile(allowPolicyPath, []byte(`default_verdict: allow`), 0o600); err != nil {
		t.Fatalf("write allow policy: %v", err)
	}
	blockPolicyPath := filepath.Join(workDir, "block.yaml")
	if err := os.WriteFile(blockPolicyPath, []byte(`default_verdict: block`), 0o600); err != nil {
		t.Fatalf("write block policy: %v", err)
	}
	approvalPolicyPath := filepath.Join(workDir, "approval.yaml")
	if err := os.WriteFile(approvalPolicyPath, []byte(`default_verdict: require_approval`), 0o600); err != nil {
		t.Fatalf("write approval policy: %v", err)
	}

	allowA := exec.Command(binPath, "policy", "test", allowPolicyPath, intentPath, "--json")
	allowA.Dir = workDir
	allowAOut, err := allowA.CombinedOutput()
	if err != nil {
		t.Fatalf("policy test allow run A failed: %v\n%s", err, string(allowAOut))
	}
	allowB := exec.Command(binPath, "policy", "test", allowPolicyPath, intentPath, "--json")
	allowB.Dir = workDir
	allowBOut, err := allowB.CombinedOutput()
	if err != nil {
		t.Fatalf("policy test allow run B failed: %v\n%s", err, string(allowBOut))
	}
	if string(allowAOut) != string(allowBOut) {
		t.Fatalf("expected deterministic policy test JSON output for same inputs")
	}
	var allowResult struct {
		OK      bool   `json:"ok"`
		Verdict string `json:"verdict"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(allowAOut, &allowResult); err != nil {
		t.Fatalf("parse allow output: %v\n%s", err, string(allowAOut))
	}
	if !allowResult.OK || allowResult.Verdict != "allow" {
		t.Fatalf("unexpected allow output: %s", string(allowAOut))
	}
	if allowResult.Summary == "" || len(allowResult.Summary) > 240 {
		t.Fatalf("unexpected allow summary length: %d", len(allowResult.Summary))
	}

	blockCmd := exec.Command(binPath, "policy", "test", blockPolicyPath, intentPath, "--json")
	blockCmd.Dir = workDir
	blockOut, err := blockCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected block policy test to return exit code 3")
	}
	if code := commandExitCode(t, err); code != 3 {
		t.Fatalf("unexpected block exit code: got=%d want=3 output=%s", code, string(blockOut))
	}
	var blockResult struct {
		OK      bool   `json:"ok"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(blockOut, &blockResult); err != nil {
		t.Fatalf("parse block output: %v\n%s", err, string(blockOut))
	}
	if !blockResult.OK || blockResult.Verdict != "block" {
		t.Fatalf("unexpected block output: %s", string(blockOut))
	}

	approvalCmd := exec.Command(binPath, "policy", "test", approvalPolicyPath, intentPath, "--json")
	approvalCmd.Dir = workDir
	approvalOut, err := approvalCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected approval policy test to return exit code 4")
	}
	if code := commandExitCode(t, err); code != 4 {
		t.Fatalf("unexpected approval exit code: got=%d want=4 output=%s", code, string(approvalOut))
	}
	var approvalResult struct {
		OK      bool   `json:"ok"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(approvalOut, &approvalResult); err != nil {
		t.Fatalf("parse approval output: %v\n%s", err, string(approvalOut))
	}
	if !approvalResult.OK || approvalResult.Verdict != "require_approval" {
		t.Fatalf("unexpected approval output: %s", string(approvalOut))
	}

	invalid := exec.Command(binPath, "policy", "test", allowPolicyPath, "--json")
	invalid.Dir = workDir
	invalidOut, err := invalid.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invalid policy test invocation to fail with exit code 6")
	}
	if code := commandExitCode(t, err); code != 6 {
		t.Fatalf("unexpected invalid-input exit code: got=%d want=6 output=%s", code, string(invalidOut))
	}
}

func TestCLIDoctor(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)

	outputDir := filepath.Join(t.TempDir(), "gait-out")
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	doctorOK := exec.Command(
		binPath,
		"doctor",
		"--workdir", root,
		"--output-dir", outputDir,
		"--json",
	)
	doctorOK.Dir = root
	okOut, err := doctorOK.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor check failed unexpectedly: %v\n%s", err, string(okOut))
	}

	var okResult struct {
		OK         bool `json:"ok"`
		NonFixable bool `json:"non_fixable"`
		Checks     []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(okOut, &okResult); err != nil {
		t.Fatalf("parse doctor ok output: %v\n%s", err, string(okOut))
	}
	if !okResult.OK || okResult.NonFixable {
		t.Fatalf("unexpected doctor ok output: %s", string(okOut))
	}
	if !hasCheckStatus(okResult.Checks, "schema_files", "pass") {
		t.Fatalf("expected schema_files pass check in doctor output: %s", string(okOut))
	}

	missingWorkDir := t.TempDir()
	doctorMissing := exec.Command(
		binPath,
		"doctor",
		"--workdir", missingWorkDir,
		"--json",
	)
	doctorMissing.Dir = root
	missingOut, err := doctorMissing.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing schema doctor run to fail with exit code 7")
	}
	if code := commandExitCode(t, err); code != 7 {
		t.Fatalf("unexpected doctor missing dependency exit code: got=%d want=7 output=%s", code, string(missingOut))
	}
	var missingResult struct {
		OK         bool `json:"ok"`
		NonFixable bool `json:"non_fixable"`
	}
	if err := json.Unmarshal(missingOut, &missingResult); err != nil {
		t.Fatalf("parse doctor missing output: %v\n%s", err, string(missingOut))
	}
	if missingResult.OK || !missingResult.NonFixable {
		t.Fatalf("unexpected doctor missing output: %s", string(missingOut))
	}
}

func buildGaitBinary(t *testing.T, root string) string {
	t.Helper()
	binDir := t.TempDir()
	binName := "gait"
	if runtime.GOOS == "windows" {
		binName = "gait.exe"
	}
	binPath := filepath.Join(binDir, binName)

	build := exec.Command("go", "build", "-o", binPath, "./cmd/gait")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gait: %v\n%s", err, string(out))
	}
	return binPath
}

func commandExitCode(t *testing.T, err error) int {
	t.Helper()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected command exit error, got: %v", err)
	}
	return exitErr.ExitCode()
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to locate test file")
	}
	dir := filepath.Dir(filename)
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasCheckStatus(checks []struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}
