package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	proofrecord "github.com/Clyra-AI/proof/core/record"
	sign "github.com/Clyra-AI/proof/signing"
)

func TestIngestLogsKongProducesSignedPolicyEnforcementRecords(t *testing.T) {
	workDir := t.TempDir()
	logPath := filepath.Join(workDir, "kong.log.jsonl")
	outPath := filepath.Join(workDir, "proof_records.jsonl")
	logPayload := strings.Join([]string{
		`{"time":"2026-02-20T10:00:00Z","route":{"name":"tool.read"},"consumer":{"username":"alice"},"status":200,"request_id":"req-1","reason_codes":["allowed"]}`,
		`{"time":"2026-02-20T10:00:01Z","route":{"name":"tool.write"},"consumer":{"username":"alice"},"status":403,"request_id":"req-2","reason_code":"blocked_by_policy"}`,
	}, "\n")
	if err := os.WriteFile(logPath, []byte(logPayload), 0o600); err != nil {
		t.Fatalf("write kong logs: %v", err)
	}
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate signing key pair: %v", err)
	}

	result, err := IngestLogs(IngestOptions{
		Source:            SourceKong,
		LogPath:           logPath,
		OutputPath:        outPath,
		ProducerVersion:   "0.0.0-test",
		SigningPrivateKey: keyPair.Private,
	})
	if err != nil {
		t.Fatalf("ingest kong logs: %v", err)
	}
	if result.InputEvents != 2 || result.OutputRecords != 2 {
		t.Fatalf("unexpected ingest counts: %#v", result)
	}

	// #nosec G304 -- test reads explicit artifact path from TempDir.
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read proof records: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected proof record line count: %d", len(lines))
	}
	records := make([]proofrecord.Record, 0, len(lines))
	for index, line := range lines {
		var recordItem proofrecord.Record
		if err := json.Unmarshal([]byte(line), &recordItem); err != nil {
			t.Fatalf("parse record line %d: %v", index+1, err)
		}
		records = append(records, recordItem)
		if recordItem.RecordType != "policy_enforcement" {
			t.Fatalf("expected policy_enforcement record type, got %#v", recordItem)
		}
		if recordItem.Source != "gait.gateway.kong" {
			t.Fatalf("expected gateway source tag, got %#v", recordItem.Source)
		}
		if strings.TrimSpace(recordItem.Integrity.Signature) == "" || strings.TrimSpace(recordItem.Integrity.SigningKeyID) == "" {
			t.Fatalf("expected signed record integrity payload, got %#v", recordItem.Integrity)
		}
		ok, verifyErr := sign.VerifyBytes(keyPair.Public, sign.Signature{
			Alg:   sign.AlgEd25519,
			KeyID: recordItem.Integrity.SigningKeyID,
			Sig:   strings.TrimPrefix(recordItem.Integrity.Signature, "base64:"),
		}, []byte(recordItem.Integrity.RecordHash))
		if verifyErr != nil {
			t.Fatalf("verify signature line %d: %v", index+1, verifyErr)
		}
		if !ok {
			t.Fatalf("signature verification failed line %d", index+1)
		}
	}
	if records[0].Integrity.PreviousRecordHash != "" {
		t.Fatalf("expected first record previous hash to be empty, got %#v", records[0].Integrity)
	}
	if records[1].Integrity.PreviousRecordHash != records[0].Integrity.RecordHash {
		t.Fatalf("expected second record previous hash to chain to first record hash")
	}
}

func TestIngestLogsDockerNestedPayload(t *testing.T) {
	workDir := t.TempDir()
	logPath := filepath.Join(workDir, "docker.log.jsonl")
	inner := `{"tool_name":"tool.write","verdict":"block","request_id":"req-docker-1","reason_codes":["policy_blocked"]}`
	if err := os.WriteFile(logPath, []byte(`{"log":"`+strings.ReplaceAll(inner, `"`, `\"`)+`\n","time":"2026-02-20T10:00:00Z"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write docker logs: %v", err)
	}

	result, err := IngestLogs(IngestOptions{
		Source:          SourceDocker,
		LogPath:         logPath,
		ProducerVersion: "0.0.0-test",
	})
	if err != nil {
		t.Fatalf("ingest docker logs: %v", err)
	}
	if result.OutputRecords != 1 {
		t.Fatalf("expected one output record, got %#v", result)
	}
	// #nosec G304 -- test reads explicit artifact path from TempDir.
	raw, err := os.ReadFile(result.ProofRecordsOut)
	if err != nil {
		t.Fatalf("read proof records: %v", err)
	}
	var recordItem proofrecord.Record
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &recordItem); err != nil {
		t.Fatalf("parse docker proof record: %v", err)
	}
	if verdict, _ := recordItem.Event["verdict"].(string); verdict != "block" {
		t.Fatalf("expected block verdict in proof record event, got %#v", recordItem.Event)
	}
	if toolName, _ := recordItem.Event["tool_name"].(string); toolName != "tool.write" {
		t.Fatalf("expected tool_name to be carried into proof record event, got %#v", recordItem.Event)
	}
}

func TestIngestLogsErrors(t *testing.T) {
	workDir := t.TempDir()
	logPath := filepath.Join(workDir, "bad.log")
	if err := os.WriteFile(logPath, []byte("not-json\n"), 0o600); err != nil {
		t.Fatalf("write bad logs: %v", err)
	}
	if _, err := IngestLogs(IngestOptions{Source: "unknown", LogPath: logPath}); err == nil {
		t.Fatalf("expected unknown source error")
	}
	if _, err := IngestLogs(IngestOptions{Source: SourceKong, LogPath: logPath}); err == nil {
		t.Fatalf("expected parse error for invalid kong log format")
	}
}
