package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIV25ContextProofFlow(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)
	workDir := t.TempDir()

	runRecordPath := filepath.Join(workDir, "run_record.json")
	mustWriteE2EFile(t, runRecordPath, `{
  "run": {
    "schema_id": "gait.runpack.run",
    "schema_version": "1.0.0",
    "created_at": "2026-02-14T00:00:00Z",
    "producer_version": "0.0.0-e2e",
    "run_id": "run_v25_e2e",
    "env": {"os":"darwin","arch":"arm64","runtime":"go"},
    "timeline": [{"event":"run_started","ts":"2026-02-14T00:00:00Z"}]
  },
  "intents": [],
  "results": [],
  "refs": {
    "schema_id": "gait.runpack.refs",
    "schema_version": "1.0.0",
    "created_at": "2026-02-14T00:00:00Z",
    "producer_version": "0.0.0-e2e",
    "run_id": "run_v25_e2e",
    "receipts": []
  },
  "capture_mode": "reference"
}`)

	contextEnvelopePath := filepath.Join(workDir, "context_envelope.json")
	mustWriteE2EFile(t, contextEnvelopePath, `{
  "schema_id": "gait.context.envelope",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "0.0.0-e2e",
  "context_set_id": "ctx_set_e2e",
  "context_set_digest": "",
  "evidence_mode": "required",
  "records": [
    {
      "ref_id": "ctx_001",
      "source_type": "doc_store",
      "source_locator": "docs://policy/security",
      "query_digest": "1111111111111111111111111111111111111111111111111111111111111111",
      "content_digest": "2222222222222222222222222222222222222222222222222222222222222222",
      "retrieved_at": "2026-02-14T00:00:00Z",
      "redaction_mode": "reference",
      "immutability": "immutable"
    }
  ]
}`)

	recordOutput := runJSONCommand(
		t,
		workDir,
		binPath,
		"run",
		"record",
		"--input",
		runRecordPath,
		"--out-dir",
		workDir,
		"--context-envelope",
		contextEnvelopePath,
		"--context-evidence-mode",
		"required",
		"--json",
	)
	var recordResult struct {
		OK     bool   `json:"ok"`
		Bundle string `json:"bundle"`
	}
	if err := json.Unmarshal(recordOutput, &recordResult); err != nil {
		t.Fatalf("parse run record output: %v\n%s", err, string(recordOutput))
	}
	if !recordResult.OK || recordResult.Bundle == "" {
		t.Fatalf("unexpected run record output: %s", string(recordOutput))
	}

	_ = runJSONCommandExpectCode(
		t,
		workDir,
		binPath,
		6,
		"run",
		"record",
		"--input",
		runRecordPath,
		"--out-dir",
		workDir,
		"--run-id",
		"run_v25_e2e_missing",
		"--context-evidence-mode",
		"required",
		"--json",
	)

	packPath := filepath.Join(workDir, "pack_v25_e2e.zip")
	_ = runJSONCommand(t, workDir, binPath, "pack", "build", "--type", "run", "--from", recordResult.Bundle, "--out", packPath, "--json")
	inspectOutput := runJSONCommand(t, workDir, binPath, "pack", "inspect", packPath, "--json")
	var inspectResult struct {
		OK      bool `json:"ok"`
		Inspect struct {
			RunPayload struct {
				ContextSetDigest    string `json:"context_set_digest"`
				ContextEvidenceMode string `json:"context_evidence_mode"`
				ContextRefCount     int    `json:"context_ref_count"`
			} `json:"run_payload"`
		} `json:"inspect"`
	}
	if err := json.Unmarshal(inspectOutput, &inspectResult); err != nil {
		t.Fatalf("parse pack inspect output: %v\n%s", err, string(inspectOutput))
	}
	if !inspectResult.OK {
		t.Fatalf("unexpected pack inspect output: %s", string(inspectOutput))
	}
	if inspectResult.Inspect.RunPayload.ContextSetDigest == "" || inspectResult.Inspect.RunPayload.ContextEvidenceMode != "required" || inspectResult.Inspect.RunPayload.ContextRefCount != 1 {
		t.Fatalf("unexpected context summary in pack inspect: %s", string(inspectOutput))
	}

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteE2EFile(t, policyPath, `schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
rules:
  - name: require-context
    priority: 1
    effect: allow
    match:
      risk_classes: [high]
    require_context_evidence: true
    required_context_evidence_mode: required`)

	intentMissingPath := filepath.Join(workDir, "intent_missing.json")
	mustWriteE2EFile(t, intentMissingPath, `{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "0.0.0-e2e",
  "tool_name": "tool.demo",
  "args": {"x":"y"},
  "targets": [{"kind":"path","value":"/tmp/demo","operation":"write"}],
  "context": {"identity":"alice","workspace":"/repo/gait","risk_class":"high"}
}`)
	missingGateOut := runJSONCommand(t, workDir, binPath, "gate", "eval", "--policy", policyPath, "--intent", intentMissingPath, "--json")
	var missingGate struct {
		OK          bool     `json:"ok"`
		Verdict     string   `json:"verdict"`
		ReasonCodes []string `json:"reason_codes"`
	}
	if err := json.Unmarshal(missingGateOut, &missingGate); err != nil {
		t.Fatalf("parse missing gate output: %v\n%s", err, string(missingGateOut))
	}
	if !missingGate.OK || missingGate.Verdict != "block" {
		t.Fatalf("expected missing context block, got: %s", string(missingGateOut))
	}

	intentPresentPath := filepath.Join(workDir, "intent_present.json")
	// Fill context digest from inspected pack output for deterministic allow result.
	var intentPresent map[string]any
	if err := json.Unmarshal([]byte(`{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "0.0.0-e2e",
  "tool_name": "tool.demo",
  "args": {"x":"y"},
  "targets": [{"kind":"path","value":"/tmp/demo","operation":"write"}],
  "context": {
    "identity":"alice",
    "workspace":"/repo/gait",
    "risk_class":"high",
    "context_set_digest": "",
    "context_evidence_mode":"required",
    "context_refs":["ctx_001"]
  }
}`), &intentPresent); err != nil {
		t.Fatalf("build intent present payload: %v", err)
	}
	contextMap := intentPresent["context"].(map[string]any)
	contextMap["context_set_digest"] = inspectResult.Inspect.RunPayload.ContextSetDigest
	intentPresentBytes, err := json.MarshalIndent(intentPresent, "", "  ")
	if err != nil {
		t.Fatalf("marshal intent present payload: %v", err)
	}
	if err := writeFileE2E(intentPresentPath, append(intentPresentBytes, '\n')); err != nil {
		t.Fatalf("write intent present payload: %v", err)
	}

	presentGateOut := runJSONCommand(t, workDir, binPath, "gate", "eval", "--policy", policyPath, "--intent", intentPresentPath, "--json")
	var presentGate struct {
		OK      bool   `json:"ok"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(presentGateOut, &presentGate); err != nil {
		t.Fatalf("parse present gate output: %v\n%s", err, string(presentGateOut))
	}
	if !presentGate.OK || presentGate.Verdict != "allow" {
		t.Fatalf("expected present context allow, got: %s", string(presentGateOut))
	}

	_ = runJSONCommand(t, workDir, binPath, "regress", "bootstrap", "--from", recordResult.Bundle, "--name", "v25_e2e_bootstrap", "--json")
	regressOut := runJSONCommand(t, workDir, binPath, "regress", "run", "--context-conformance", "--json")
	var regressResult struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(regressOut, &regressResult); err != nil {
		t.Fatalf("parse regress output: %v\n%s", err, string(regressOut))
	}
	if !regressResult.OK || regressResult.Status != "pass" {
		t.Fatalf("expected regress context conformance pass, got: %s", string(regressOut))
	}
}

func writeFileE2E(path string, content []byte) error {
	return os.WriteFile(path, content, 0o600)
}
