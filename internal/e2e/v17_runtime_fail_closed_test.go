package e2e

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/jcs"
	"github.com/Clyra-AI/gait/core/sign"
)

func TestCLIV17FailClosedMatrix(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)
	workDir := t.TempDir()

	policyAllowPath := filepath.Join(workDir, "policy_allow.yaml")
	if err := os.WriteFile(policyAllowPath, []byte("default_verdict: allow\n"), 0o600); err != nil {
		t.Fatalf("write allow policy: %v", err)
	}

	invalidIntentPath := filepath.Join(workDir, "intent_invalid.json")
	if err := os.WriteFile(invalidIntentPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write invalid intent: %v", err)
	}
	invalidEval := exec.Command(
		binPath,
		"gate",
		"eval",
		"--policy",
		policyAllowPath,
		"--intent",
		invalidIntentPath,
		"--json",
	)
	invalidEval.Dir = workDir
	invalidOutput, err := invalidEval.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invalid intent gate eval to fail with exit code 6")
	}
	if code := commandExitCode(t, err); code != 6 {
		t.Fatalf("invalid intent exit code mismatch: got=%d want=6 output=%s", code, string(invalidOutput))
	}
	var invalidResult struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(invalidOutput, &invalidResult); err != nil {
		t.Fatalf("parse invalid intent output: %v\n%s", err, string(invalidOutput))
	}
	if invalidResult.OK || strings.TrimSpace(invalidResult.Error) == "" {
		t.Fatalf("expected invalid intent to fail-closed with error: %s", string(invalidOutput))
	}

	traceKeyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate trace key pair: %v", err)
	}
	tracePrivateKeyPath := filepath.Join(workDir, "trace_private.key")
	if err := os.WriteFile(tracePrivateKeyPath, []byte(base64.StdEncoding.EncodeToString(traceKeyPair.Private)), 0o600); err != nil {
		t.Fatalf("write trace private key: %v", err)
	}

	highRiskNoBrokerPolicy := filepath.Join(workDir, "policy_high_risk_no_broker.yaml")
	if err := os.WriteFile(highRiskNoBrokerPolicy, []byte(strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: high-risk-allow-without-broker",
		"    effect: allow",
		"    match:",
		"      tool_names: [tool.delete]",
		"      risk_classes: [high]",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write high-risk no-broker policy: %v", err)
	}
	highRiskIntentPath := filepath.Join(workDir, "intent_high_risk.json")
	if err := os.WriteFile(highRiskIntentPath, []byte(strings.Join([]string{
		"{",
		`  "schema_id": "gait.gate.intent_request",`,
		`  "schema_version": "1.0.0",`,
		`  "created_at": "2026-02-09T00:00:00Z",`,
		`  "producer_version": "0.0.0-e2e",`,
		`  "tool_name": "tool.delete",`,
		`  "args": {"path": "/tmp/v17-delete.txt"},`,
		`  "targets": [{"kind":"path","value":"/tmp/v17-delete.txt","operation":"delete","endpoint_class":"fs.delete","destructive":true}],`,
		`  "arg_provenance": [{"arg_path":"$.path","source":"user"}],`,
		`  "context": {"identity":"alice","workspace":"/repo/gait","risk_class":"high"}`,
		"}",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write high-risk intent: %v", err)
	}

	highRiskEval := exec.Command(
		binPath,
		"gate",
		"eval",
		"--policy",
		highRiskNoBrokerPolicy,
		"--intent",
		highRiskIntentPath,
		"--profile",
		"oss-prod",
		"--key-mode",
		"prod",
		"--private-key",
		tracePrivateKeyPath,
		"--json",
	)
	highRiskEval.Dir = workDir
	highRiskOutput, err := highRiskEval.CombinedOutput()
	if err == nil {
		t.Fatalf("expected oss-prod missing-broker-policy check to fail with exit code 6")
	}
	if code := commandExitCode(t, err); code != 6 {
		t.Fatalf("oss-prod policy precondition exit code mismatch: got=%d want=6 output=%s", code, string(highRiskOutput))
	}
	var highRiskResult struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(highRiskOutput, &highRiskResult); err != nil {
		t.Fatalf("parse high-risk policy output: %v\n%s", err, string(highRiskOutput))
	}
	if highRiskResult.OK || !strings.Contains(highRiskResult.Error, "require_broker_credential") {
		t.Fatalf("expected broker precondition error, got: %s", string(highRiskOutput))
	}

	manifestPath, publicKeyPath, cacheDir := writeSignedSkillRegistryPack(t, workDir)
	registryInstall := exec.Command(
		binPath,
		"registry",
		"install",
		"--source",
		manifestPath,
		"--cache-dir",
		cacheDir,
		"--public-key",
		publicKeyPath,
		"--json",
	)
	registryInstall.Dir = workDir
	installOutput, err := registryInstall.CombinedOutput()
	if err != nil {
		t.Fatalf("registry install for skill pack failed: %v\n%s", err, string(installOutput))
	}
	var installResult struct {
		OK           bool   `json:"ok"`
		MetadataPath string `json:"metadata_path"`
	}
	if err := json.Unmarshal(installOutput, &installResult); err != nil {
		t.Fatalf("parse registry install output: %v\n%s", err, string(installOutput))
	}
	if !installResult.OK || installResult.MetadataPath == "" {
		t.Fatalf("unexpected registry install output: %s", string(installOutput))
	}

	registryVerify := exec.Command(
		binPath,
		"registry",
		"verify",
		"--path",
		installResult.MetadataPath,
		"--cache-dir",
		cacheDir,
		"--public-key",
		publicKeyPath,
		"--publisher-allowlist",
		"trusted-inc",
		"--json",
	)
	registryVerify.Dir = workDir
	verifyOutput, err := registryVerify.CombinedOutput()
	if err == nil {
		t.Fatalf("expected skill registry verification to fail with exit code 2")
	}
	if code := commandExitCode(t, err); code != 2 {
		t.Fatalf("registry verify exit code mismatch: got=%d want=2 output=%s", code, string(verifyOutput))
	}
	var verifyResult struct {
		OK                bool `json:"ok"`
		PublisherAllowed  bool `json:"publisher_allowed"`
		SignatureVerified bool `json:"signature_verified"`
	}
	if err := json.Unmarshal(verifyOutput, &verifyResult); err != nil {
		t.Fatalf("parse registry verify output: %v\n%s", err, string(verifyOutput))
	}
	if verifyResult.OK || verifyResult.PublisherAllowed || !verifyResult.SignatureVerified {
		t.Fatalf("expected publisher allowlist failure with valid signature, got: %s", string(verifyOutput))
	}

	highRiskWithBrokerPolicy := filepath.Join(workDir, "policy_high_risk_with_broker.yaml")
	if err := os.WriteFile(highRiskWithBrokerPolicy, []byte(strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: high-risk-allow-with-broker",
		"    effect: allow",
		"    require_broker_credential: true",
		"    broker_reference: egress",
		"    broker_scopes: [export]",
		"    match:",
		"      tool_names: [tool.write]",
		"      risk_classes: [high]",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write high-risk broker policy: %v", err)
	}
	brokerIntentPath := filepath.Join(workDir, "intent_broker.json")
	if err := os.WriteFile(brokerIntentPath, []byte(strings.Join([]string{
		"{",
		`  "schema_id": "gait.gate.intent_request",`,
		`  "schema_version": "1.0.0",`,
		`  "created_at": "2026-02-09T00:00:00Z",`,
		`  "producer_version": "0.0.0-e2e",`,
		`  "tool_name": "tool.write",`,
		`  "args": {"path": "/tmp/v17-write.txt"},`,
		`  "targets": [{"kind":"path","value":"/tmp/v17-write.txt","operation":"write","endpoint_class":"fs.write"}],`,
		`  "arg_provenance": [{"arg_path":"$.path","source":"user"}],`,
		`  "context": {"identity":"alice","workspace":"/repo/gait","risk_class":"high"}`,
		"}",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write broker intent: %v", err)
	}

	brokerPath := filepath.Join(workDir, "broker_fail.sh")
	brokerScript := "#!/bin/sh\necho 'forced failure token=secret-broker-token' 1>&2\nexit 2\n"
	if runtime.GOOS == "windows" {
		brokerPath = filepath.Join(workDir, "broker_fail.cmd")
		brokerScript = "@echo forced failure token=secret-broker-token 1>&2\r\n@exit /b 2\r\n"
	}
	if err := os.WriteFile(brokerPath, []byte(brokerScript), 0o700); err != nil {
		t.Fatalf("write broker failure script: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(brokerPath, 0o700); err != nil {
			t.Fatalf("chmod broker failure script: %v", err)
		}
	}

	brokerEval := exec.Command(
		binPath,
		"gate",
		"eval",
		"--policy",
		highRiskWithBrokerPolicy,
		"--intent",
		brokerIntentPath,
		"--profile",
		"oss-prod",
		"--key-mode",
		"prod",
		"--private-key",
		tracePrivateKeyPath,
		"--credential-broker",
		"command",
		"--credential-command",
		brokerPath,
		"--credential-evidence-out",
		filepath.Join(workDir, "credential_evidence_matrix.json"),
		"--json",
	)
	brokerEval.Dir = workDir
	brokerOutput, err := brokerEval.CombinedOutput()
	if err != nil {
		t.Fatalf("expected broker failure to fail-closed with block verdict and exit 0, got error: %v\n%s", err, string(brokerOutput))
	}
	if strings.Contains(string(brokerOutput), "secret-broker-token") {
		t.Fatalf("broker failure output leaked secret token: %s", string(brokerOutput))
	}
	var brokerResult struct {
		OK                     bool     `json:"ok"`
		Verdict                string   `json:"verdict"`
		ReasonCodes            []string `json:"reason_codes"`
		CredentialEvidencePath string   `json:"credential_evidence_path"`
	}
	if err := json.Unmarshal(brokerOutput, &brokerResult); err != nil {
		t.Fatalf("parse broker fail-closed output: %v\n%s", err, string(brokerOutput))
	}
	if !brokerResult.OK || brokerResult.Verdict != "block" {
		t.Fatalf("expected broker failure to degrade to block, got: %s", string(brokerOutput))
	}
	if !containsString(brokerResult.ReasonCodes, "broker_credential_missing") {
		t.Fatalf("expected broker_credential_missing reason, got: %s", string(brokerOutput))
	}
	if strings.TrimSpace(brokerResult.CredentialEvidencePath) != "" {
		t.Fatalf("unexpected credential evidence path on broker failure: %s", brokerResult.CredentialEvidencePath)
	}
}

func writeSignedSkillRegistryPack(t *testing.T, workDir string) (string, string, string) {
	t.Helper()
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate registry key pair: %v", err)
	}
	publicKeyPath := filepath.Join(workDir, "registry_skill_public.key")
	if err := os.WriteFile(publicKeyPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Public)), 0o600); err != nil {
		t.Fatalf("write registry public key: %v", err)
	}

	manifest := map[string]any{
		"schema_id":        "gait.registry.pack",
		"schema_version":   "1.0.0",
		"created_at":       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"producer_version": "0.0.0-e2e",
		"pack_name":        "skill-guardrails",
		"pack_version":     "1.0.0",
		"pack_type":        "skill",
		"publisher":        "acme",
		"source":           "registry",
		"artifacts": []map[string]string{
			{"path": "skill.yaml", "sha256": strings.Repeat("a", 64)},
		},
	}
	signable, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal skill manifest: %v", err)
	}
	digest, err := jcs.DigestJCS(signable)
	if err != nil {
		t.Fatalf("digest skill manifest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		t.Fatalf("sign skill manifest: %v", err)
	}
	manifest["signatures"] = []map[string]string{{
		"alg":           signature.Alg,
		"key_id":        signature.KeyID,
		"sig":           signature.Sig,
		"signed_digest": signature.SignedDigest,
	}}

	manifestPath := filepath.Join(workDir, "registry_skill_pack.json")
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal signed skill manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestRaw, 0o600); err != nil {
		t.Fatalf("write signed skill manifest: %v", err)
	}
	return manifestPath, publicKeyPath, filepath.Join(workDir, "registry_skill_cache")
}
