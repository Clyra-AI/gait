package guard

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/runpack"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	schemaregress "github.com/davidahmann/gait/core/schema/v1/regress"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

func TestBuildPackV14TemplateAndPDF(t *testing.T) {
	workDir := t.TempDir()
	now := time.Date(2026, time.February, 6, 10, 0, 0, 0, time.UTC)
	runpackPath := filepath.Join(workDir, "runpack_run_v14.zip")
	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_v14",
			CreatedAt:       now,
			ProducerVersion: "0.0.0-dev",
		},
		Refs:        schemarunpack.Refs{RunID: "run_v14"},
		CaptureMode: "reference",
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	result, err := BuildPack(BuildOptions{
		RunpackPath: runpackPath,
		OutputPath:  filepath.Join(workDir, "evidence_pack_v14.zip"),
		CaseID:      "INC-42",
		TemplateID:  "pci",
		RenderPDF:   true,
		ExtraEvidenceFiles: map[string][]byte{
			"policy_digests.json": []byte(`{"policy_digests":[]}`),
		},
		ProducerVersion: "0.0.0-dev",
	})
	if err != nil {
		t.Fatalf("build v14 pack: %v", err)
	}
	if result.Manifest.TemplateID != "pci" {
		t.Fatalf("expected template pci, got %s", result.Manifest.TemplateID)
	}
	if len(result.Manifest.ControlIndex) == 0 {
		t.Fatalf("expected control index")
	}
	if len(result.Manifest.EvidencePtrs) == 0 {
		t.Fatalf("expected evidence pointers")
	}
	if len(result.Manifest.Rendered) == 0 || result.Manifest.Rendered[0].Path != "summary.pdf" {
		t.Fatalf("expected rendered summary.pdf")
	}

	paths := make([]string, 0, len(result.Manifest.Contents))
	for _, entry := range result.Manifest.Contents {
		paths = append(paths, entry.Path)
	}
	joined := strings.Join(paths, ",")
	for _, expected := range []string{"control_index.json", "evidence_pointers.json", "summary.pdf", "policy_digests.json"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %s in contents: %s", expected, joined)
		}
	}
}

func TestApplyRetentionV14(t *testing.T) {
	workDir := t.TempDir()
	oldTrace := filepath.Join(workDir, "trace_old.json")
	oldPack := filepath.Join(workDir, "evidence_pack_old.zip")
	keepTrace := filepath.Join(workDir, "trace_keep.json")
	keepPack := filepath.Join(workDir, "incident_pack_keep.zip")
	for _, path := range []string{oldTrace, oldPack, keepTrace, keepPack} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("write artifact %s: %v", path, err)
		}
	}
	now := time.Date(2026, time.February, 6, 10, 0, 0, 0, time.UTC)
	oldTime := now.Add(-200 * time.Hour)
	keepTime := now.Add(-2 * time.Hour)
	for _, path := range []string{oldTrace, oldPack} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatalf("set old mtime %s: %v", path, err)
		}
	}
	for _, path := range []string{keepTrace, keepPack} {
		if err := os.Chtimes(path, keepTime, keepTime); err != nil {
			t.Fatalf("set keep mtime %s: %v", path, err)
		}
	}

	dryRunResult, err := ApplyRetention(RetentionOptions{
		RootPath:     workDir,
		TraceTTL:     24 * time.Hour,
		PackTTL:      48 * time.Hour,
		DryRun:       true,
		ReportOutput: filepath.Join(workDir, "retention_report.json"),
		Now:          now,
	})
	if err != nil {
		t.Fatalf("dry-run retention: %v", err)
	}
	if len(dryRunResult.DeletedFiles) != 2 {
		t.Fatalf("expected 2 dry-run deletions, got %d", len(dryRunResult.DeletedFiles))
	}
	if _, err := os.Stat(oldTrace); err != nil {
		t.Fatalf("dry run should not delete old trace: %v", err)
	}

	applyResult, err := ApplyRetention(RetentionOptions{
		RootPath: workDir,
		TraceTTL: 24 * time.Hour,
		PackTTL:  48 * time.Hour,
		DryRun:   false,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("apply retention: %v", err)
	}
	if len(applyResult.DeletedFiles) != 2 {
		t.Fatalf("expected 2 deletions, got %d", len(applyResult.DeletedFiles))
	}
	if _, err := os.Stat(oldTrace); !os.IsNotExist(err) {
		t.Fatalf("expected old trace to be deleted, err=%v", err)
	}
	if _, err := os.Stat(oldPack); !os.IsNotExist(err) {
		t.Fatalf("expected old pack to be deleted, err=%v", err)
	}
}

func TestEncryptDecryptArtifactV14(t *testing.T) {
	workDir := t.TempDir()
	sourcePath := filepath.Join(workDir, "artifact.json")
	if err := os.WriteFile(sourcePath, []byte(`{"k":"v"}`), 0o600); err != nil {
		t.Fatalf("write source artifact: %v", err)
	}
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv("GAIT_ENCRYPTION_KEY", key)

	encryptResult, err := EncryptArtifact(EncryptOptions{
		InputPath:       sourcePath,
		KeyEnv:          "GAIT_ENCRYPTION_KEY",
		ProducerVersion: "0.0.0-dev",
	})
	if err != nil {
		t.Fatalf("encrypt artifact: %v", err)
	}
	if encryptResult.Artifact.Algorithm != "aes-256-gcm" {
		t.Fatalf("unexpected algorithm: %s", encryptResult.Artifact.Algorithm)
	}

	decryptPath := filepath.Join(workDir, "artifact.decrypted.json")
	decryptResult, err := DecryptArtifact(DecryptOptions{
		InputPath:  encryptResult.Path,
		OutputPath: decryptPath,
		KeyEnv:     "GAIT_ENCRYPTION_KEY",
	})
	if err != nil {
		t.Fatalf("decrypt artifact: %v", err)
	}
	if decryptResult.Path != decryptPath {
		t.Fatalf("unexpected decrypt output path: %s", decryptResult.Path)
	}
	raw, err := os.ReadFile(decryptPath)
	if err != nil {
		t.Fatalf("read decrypted file: %v", err)
	}
	if string(raw) != `{"k":"v"}` {
		t.Fatalf("unexpected decrypted payload: %s", string(raw))
	}
}

func TestBuildIncidentPackV14(t *testing.T) {
	workDir := t.TempDir()
	now := time.Date(2026, time.February, 6, 12, 0, 0, 0, time.UTC)
	runpackPath := filepath.Join(workDir, "runpack_run_incident.zip")
	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_incident",
			CreatedAt:       now,
			ProducerVersion: "0.0.0-dev",
		},
		Refs:        schemarunpack.Refs{RunID: "run_incident"},
		CaptureMode: "reference",
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	mustWriteJSON(t, filepath.Join(workDir, "trace_1.json"), schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now.Add(30 * time.Minute),
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_1",
		ToolName:        "tool.write",
		ArgsDigest:      strings.Repeat("a", 64),
		IntentDigest:    strings.Repeat("b", 64),
		PolicyDigest:    strings.Repeat("c", 64),
		Verdict:         "allow",
	})
	mustWriteJSON(t, filepath.Join(workDir, "regress_result.json"), schemaregress.RegressResult{
		SchemaID:        "gait.regress.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now.Add(20 * time.Minute),
		ProducerVersion: "0.0.0-dev",
		FixtureSet:      "run_incident",
		Status:          "pass",
	})
	mustWriteJSON(t, filepath.Join(workDir, "approval_audit_trace_1.json"), schemagate.ApprovalAuditRecord{
		SchemaID:        "gait.gate.approval_audit_record",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now.Add(25 * time.Minute),
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_1",
		ToolName:        "tool.write",
		IntentDigest:    strings.Repeat("d", 64),
		PolicyDigest:    strings.Repeat("e", 64),
	})
	mustWriteJSON(t, filepath.Join(workDir, "credential_evidence_trace_1.json"), schemagate.BrokerCredentialRecord{
		SchemaID:        "gait.gate.broker_credential_record",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now.Add(25 * time.Minute),
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_1",
		ToolName:        "tool.write",
		Identity:        "alice",
		Broker:          "env",
	})

	result, err := BuildIncidentPack(IncidentPackOptions{
		RunpackPath:     runpackPath,
		OutputPath:      filepath.Join(workDir, "incident_pack.zip"),
		CaseID:          "INC-2026-42",
		Window:          2 * time.Hour,
		TemplateID:      "incident_response",
		ProducerVersion: "0.0.0-dev",
	})
	if err != nil {
		t.Fatalf("build incident pack: %v", err)
	}
	if result.TraceCount != 1 || result.RegressCount != 1 {
		t.Fatalf("unexpected incident counts: traces=%d regress=%d", result.TraceCount, result.RegressCount)
	}
	if result.ApprovalAuditCount != 1 || result.CredentialEvidenceCount != 1 {
		t.Fatalf("unexpected approval/credential counts: approvals=%d credentials=%d", result.ApprovalAuditCount, result.CredentialEvidenceCount)
	}
	if len(result.PolicyDigests) != 1 {
		t.Fatalf("expected one policy digest, got %d", len(result.PolicyDigests))
	}
	if result.BuildResult.Manifest.IncidentWindow == nil {
		t.Fatalf("expected incident window metadata")
	}
}
