package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestRunGuardRetainAndCryptoCommands(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	tracePath := filepath.Join(workDir, "trace_old.json")
	packPath := filepath.Join(workDir, "evidence_pack_old.zip")
	if err := os.WriteFile(tracePath, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write trace: %v", err)
	}
	if err := os.WriteFile(packPath, []byte("zip"), 0o600); err != nil {
		t.Fatalf("write pack: %v", err)
	}
	oldTime := time.Now().Add(-200 * time.Hour)
	if err := os.Chtimes(tracePath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes trace: %v", err)
	}
	if err := os.Chtimes(packPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes pack: %v", err)
	}

	if code := runGuardRetain([]string{
		"--root", workDir,
		"--trace-ttl", "24h",
		"--pack-ttl", "24h",
		"--dry-run",
		"--json",
	}); code != exitOK {
		t.Fatalf("runGuardRetain dry-run: expected %d got %d", exitOK, code)
	}

	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv("GAIT_GUARD_KEY", key)
	artifactPath := filepath.Join(workDir, "artifact.json")
	if err := os.WriteFile(artifactPath, []byte(`{"mode":"v14"}`), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	encryptedPath := filepath.Join(workDir, "artifact.gaitenc")
	if code := runGuardEncrypt([]string{
		"--in", artifactPath,
		"--out", encryptedPath,
		"--key-env", "GAIT_GUARD_KEY",
		"--json",
	}); code != exitOK {
		t.Fatalf("runGuardEncrypt: expected %d got %d", exitOK, code)
	}
	decryptedPath := filepath.Join(workDir, "artifact.decrypted.json")
	if code := runGuardDecrypt([]string{
		"--in", encryptedPath,
		"--out", decryptedPath,
		"--key-env", "GAIT_GUARD_KEY",
		"--json",
	}); code != exitOK {
		t.Fatalf("runGuardDecrypt: expected %d got %d", exitOK, code)
	}
	raw, err := os.ReadFile(decryptedPath)
	if err != nil {
		t.Fatalf("read decrypted artifact: %v", err)
	}
	if string(raw) != `{"mode":"v14"}` {
		t.Fatalf("unexpected decrypted content: %s", string(raw))
	}
}

func TestRunIncidentPackCommand(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	now := time.Date(2026, time.February, 6, 14, 0, 0, 0, time.UTC)
	runpackPath := filepath.Join(workDir, "runpack_run_incident_cmd.zip")
	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_incident_cmd",
			CreatedAt:       now,
			ProducerVersion: "0.0.0-dev",
		},
		Refs:        schemarunpack.Refs{RunID: "run_incident_cmd"},
		CaptureMode: "reference",
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	mustWriteFile(t, filepath.Join(workDir, "trace_incident.json"), strings.Join([]string{
		`{`,
		`  "schema_id":"gait.gate.trace",`,
		`  "schema_version":"1.0.0",`,
		`  "created_at":"2026-02-06T14:30:00Z",`,
		`  "producer_version":"0.0.0-dev",`,
		`  "trace_id":"trace_incident",`,
		`  "tool_name":"tool.write",`,
		`  "args_digest":"` + strings.Repeat("a", 64) + `",`,
		`  "intent_digest":"` + strings.Repeat("b", 64) + `",`,
		`  "policy_digest":"` + strings.Repeat("c", 64) + `",`,
		`  "verdict":"allow"`,
		`}`,
	}, "\n"))
	mustWriteFile(t, filepath.Join(workDir, "regress_result.json"), strings.Join([]string{
		`{`,
		`  "schema_id":"gait.regress.result",`,
		`  "schema_version":"1.0.0",`,
		`  "created_at":"2026-02-06T14:15:00Z",`,
		`  "producer_version":"0.0.0-dev",`,
		`  "fixture_set":"run_incident_cmd",`,
		`  "status":"pass",`,
		`  "graders":[]`,
		`  "summary":{}`,
		`  "manifest_digest":"` + strings.Repeat("d", 64) + `"`,
		`}`,
	}, "\n"))

	mustWriteJSONFile(t, filepath.Join(workDir, "approval_audit_trace_incident.json"), schemagate.ApprovalAuditRecord{
		SchemaID:        "gait.gate.approval_audit_record",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now.Add(20 * time.Minute),
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_incident",
		ToolName:        "tool.write",
		IntentDigest:    strings.Repeat("e", 64),
		PolicyDigest:    strings.Repeat("f", 64),
	})
	mustWriteJSONFile(t, filepath.Join(workDir, "credential_evidence_trace_incident.json"), schemagate.BrokerCredentialRecord{
		SchemaID:        "gait.gate.broker_credential_record",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now.Add(20 * time.Minute),
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_incident",
		ToolName:        "tool.write",
		Identity:        "alice",
		Broker:          "env",
	})

	outPath := filepath.Join(workDir, "incident_pack.zip")
	if code := runIncidentPack([]string{
		"--from", runpackPath,
		"--window", "2h",
		"--out", outPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runIncidentPack: expected %d got %d", exitOK, code)
	}
	if code := runGuardVerify([]string{outPath, "--json"}); code != exitOK {
		t.Fatalf("guard verify incident pack: expected %d got %d", exitOK, code)
	}
}

func TestGuardAndIncidentOutputBranches(t *testing.T) {
	if code := runIncident(nil); code != exitInvalidInput {
		t.Fatalf("runIncident no args expected %d got %d", exitInvalidInput, code)
	}
	if code := runIncident([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runIncident unknown expected %d got %d", exitInvalidInput, code)
	}
	if code := writeIncidentPackOutput(true, incidentPackOutput{OK: true, PackPath: "incident.zip"}, exitOK); code != exitOK {
		t.Fatalf("writeIncidentPackOutput json expected %d got %d", exitOK, code)
	}
	if code := writeIncidentPackOutput(false, incidentPackOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeIncidentPackOutput text expected %d got %d", exitInvalidInput, code)
	}
	printIncidentUsage()
	printIncidentPackUsage()

	if code := runGuardRetain([]string{"--trace-ttl", "bad"}); code != exitInvalidInput {
		t.Fatalf("runGuardRetain invalid ttl expected %d got %d", exitInvalidInput, code)
	}
	if code := runGuardEncrypt([]string{}); code != exitInvalidInput {
		t.Fatalf("runGuardEncrypt missing args expected %d got %d", exitInvalidInput, code)
	}
	if code := runGuardDecrypt([]string{}); code != exitInvalidInput {
		t.Fatalf("runGuardDecrypt missing args expected %d got %d", exitInvalidInput, code)
	}
	if code := writeGuardRetainOutput(true, guardRetainOutput{OK: true, ScannedFiles: 1}, exitOK); code != exitOK {
		t.Fatalf("writeGuardRetainOutput json expected %d got %d", exitOK, code)
	}
	if code := writeGuardRetainOutput(false, guardRetainOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeGuardRetainOutput text expected %d got %d", exitInvalidInput, code)
	}
	if code := writeGuardEncryptOutput(true, guardEncryptOutput{OK: true, Path: "a.gaitenc"}, exitOK); code != exitOK {
		t.Fatalf("writeGuardEncryptOutput json expected %d got %d", exitOK, code)
	}
	if code := writeGuardEncryptOutput(false, guardEncryptOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeGuardEncryptOutput text expected %d got %d", exitInvalidInput, code)
	}
	if code := writeGuardDecryptOutput(true, guardDecryptOutput{OK: true, Path: "a"}, exitOK); code != exitOK {
		t.Fatalf("writeGuardDecryptOutput json expected %d got %d", exitOK, code)
	}
	if code := writeGuardDecryptOutput(false, guardDecryptOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeGuardDecryptOutput text expected %d got %d", exitInvalidInput, code)
	}
}
