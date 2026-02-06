package guard

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/runpack"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	schemaguard "github.com/davidahmann/gait/core/schema/v1/guard"
	schemaregress "github.com/davidahmann/gait/core/schema/v1/regress"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	schemascout "github.com/davidahmann/gait/core/schema/v1/scout"
	"github.com/davidahmann/gait/core/zipx"
)

func TestBuildAndVerifyPack(t *testing.T) {
	workDir := t.TempDir()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	runpackPath := filepath.Join(workDir, "runpack_run_guard.zip")
	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_guard",
			CreatedAt:       now,
			ProducerVersion: "0.0.0-dev",
		},
		Intents: []schemarunpack.IntentRecord{{
			IntentID:   "intent_1",
			RunID:      "run_guard",
			ToolName:   "tool.delete",
			ArgsDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
		Results: []schemarunpack.ResultRecord{{
			IntentID:     "intent_1",
			RunID:        "run_guard",
			Status:       "ok",
			ResultDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}},
		Refs: schemarunpack.Refs{
			RunID: "run_guard",
			Receipts: []schemarunpack.RefReceipt{{
				RefID:         "ref_1",
				SourceType:    "web",
				SourceLocator: "https://example.com",
				QueryDigest:   "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				ContentDigest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
				RetrievedAt:   now,
				RedactionMode: "reference",
			}},
		},
		CaptureMode: "reference",
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	inventory := schemascout.InventorySnapshot{
		SchemaID:        "gait.scout.inventory_snapshot",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-dev",
		SnapshotID:      "snap_test",
		Items: []schemascout.InventoryItem{{
			ID:      "tool:framework:langchain:delete_user",
			Kind:    "tool",
			Name:    "delete_user",
			Locator: "agent.py",
		}},
	}
	inventoryPath := filepath.Join(workDir, "inventory.json")
	mustWriteJSON(t, inventoryPath, inventory)

	trace := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_1",
		ToolName:        "tool.delete",
		ArgsDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		IntentDigest:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		PolicyDigest:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Verdict:         "allow",
	}
	tracePath := filepath.Join(workDir, "trace.json")
	mustWriteJSON(t, tracePath, trace)

	regress := schemaregress.RegressResult{
		SchemaID:        "gait.regress.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-dev",
		FixtureSet:      "run_guard",
		Status:          "pass",
	}
	regressPath := filepath.Join(workDir, "regress.json")
	mustWriteJSON(t, regressPath, regress)

	approvalAuditPath := filepath.Join(workDir, "approval_audit_trace_1.json")
	mustWriteJSON(t, approvalAuditPath, schemagate.ApprovalAuditRecord{
		SchemaID:          "gait.gate.approval_audit_record",
		SchemaVersion:     "1.0.0",
		CreatedAt:         now,
		ProducerVersion:   "0.0.0-dev",
		TraceID:           "trace_1",
		ToolName:          "tool.delete",
		IntentDigest:      strings.Repeat("a", 64),
		PolicyDigest:      strings.Repeat("b", 64),
		RequiredApprovals: 1,
		ValidApprovals:    1,
		Approved:          true,
		Approvers:         []string{"alice"},
		Entries: []schemagate.ApprovalAuditEntry{{
			TokenID:          "token_1",
			ApproverIdentity: "alice",
			ReasonCode:       "ticket",
			Scope:            []string{"tool:tool.delete"},
			ExpiresAt:        now.Add(time.Hour),
			Valid:            true,
		}},
	})
	credentialEvidencePath := filepath.Join(workDir, "credential_evidence_trace_1.json")
	mustWriteJSON(t, credentialEvidencePath, schemagate.BrokerCredentialRecord{
		SchemaID:        "gait.gate.broker_credential_record",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_1",
		ToolName:        "tool.delete",
		Identity:        "alice",
		Broker:          "env",
		Reference:       "egress",
		Scope:           []string{"export"},
		CredentialRef:   "env:GAIT_BROKER_TOKEN_EGRESS:deadbeef",
	})

	packPath := filepath.Join(workDir, "evidence_pack.zip")
	buildResult, err := BuildPack(BuildOptions{
		RunpackPath:    runpackPath,
		OutputPath:     packPath,
		CaseID:         "case_1",
		InventoryPaths: []string{inventoryPath},
		TracePaths:     []string{tracePath},
		RegressPaths:   []string{regressPath},
	})
	if err != nil {
		t.Fatalf("build pack: %v", err)
	}
	if buildResult.Manifest.PackID == "" {
		t.Fatalf("expected pack id")
	}
	foundAudit := false
	foundCredential := false
	for _, entry := range buildResult.Manifest.Contents {
		if entry.Path == "approval_audit_01.json" {
			foundAudit = true
		}
		if entry.Path == "credential_evidence_01.json" {
			foundCredential = true
		}
	}
	if !foundAudit || !foundCredential {
		t.Fatalf("expected auto-discovered v1.2 evidence files in pack manifest: %#v", buildResult.Manifest.Contents)
	}

	verifyResult, err := VerifyPack(packPath)
	if err != nil {
		t.Fatalf("verify pack: %v", err)
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 {
		t.Fatalf("expected clean verify result, got missing=%d mismatches=%d", len(verifyResult.MissingFiles), len(verifyResult.HashMismatches))
	}

	tamperedPath := filepath.Join(workDir, "evidence_pack_tampered.zip")
	tamperPackMissingFile(t, packPath, tamperedPath, "runpack_summary.json")
	tamperedVerify, err := VerifyPack(tamperedPath)
	if err != nil {
		t.Fatalf("verify tampered pack: %v", err)
	}
	if len(tamperedVerify.MissingFiles) == 0 {
		t.Fatalf("expected missing file in tampered pack")
	}
}

func tamperPackMissingFile(t *testing.T, source string, destination string, remove string) {
	t.Helper()
	reader, err := zip.OpenReader(source)
	if err != nil {
		t.Fatalf("open pack: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()
	files := make([]zipx.File, 0, len(reader.File))
	for _, zipFile := range reader.File {
		if zipFile.Name == remove {
			continue
		}
		fileReader, openErr := zipFile.Open()
		if openErr != nil {
			t.Fatalf("open zip entry %s: %v", zipFile.Name, openErr)
		}
		content := new(bytes.Buffer)
		if _, copyErr := content.ReadFrom(fileReader); copyErr != nil {
			_ = fileReader.Close()
			t.Fatalf("read zip entry %s: %v", zipFile.Name, copyErr)
		}
		_ = fileReader.Close()
		files = append(files, zipx.File{
			Path: zipFile.Name,
			Data: content.Bytes(),
			Mode: 0o644,
		})
	}
	var out bytes.Buffer
	if err := zipx.WriteDeterministicZip(&out, files); err != nil {
		t.Fatalf("write tampered zip: %v", err)
	}
	if err := os.WriteFile(destination, out.Bytes(), 0o600); err != nil {
		t.Fatalf("write tampered zip: %v", err)
	}
}

func mustWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatalf("write json file %s: %v", path, err)
	}
}

func TestGuardHelperBranches(t *testing.T) {
	workDir := t.TempDir()
	now := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)

	if _, err := BuildPack(BuildOptions{}); err == nil {
		t.Fatalf("expected BuildPack missing runpack path error")
	}
	if _, err := VerifyPack(filepath.Join(workDir, "missing.zip")); err == nil {
		t.Fatalf("expected VerifyPack missing file error")
	}
	if _, err := readTraceRecord(filepath.Join(workDir, "missing.trace")); err == nil {
		t.Fatalf("expected readTraceRecord missing path error")
	}

	inventoryPath := filepath.Join(workDir, "inventory.json")
	mustWriteJSON(t, inventoryPath, schemascout.InventorySnapshot{
		SchemaID:        "gait.scout.inventory_snapshot",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-dev",
		SnapshotID:      "snap_one",
		Items:           []schemascout.InventoryItem{},
	})
	if payloads, err := readInventorySnapshots([]string{inventoryPath + ", " + inventoryPath}); err != nil || len(payloads) != 1 {
		t.Fatalf("readInventorySnapshots dedupe: len=%d err=%v", len(payloads), err)
	}
	invalidInventoryPath := filepath.Join(workDir, "invalid_inventory.json")
	if err := os.WriteFile(invalidInventoryPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid inventory: %v", err)
	}
	if _, err := readInventorySnapshots([]string{invalidInventoryPath}); err == nil {
		t.Fatalf("expected invalid inventory parse error")
	}
	if _, _, err := discoverV12EvidencePaths(filepath.Join(workDir, "missing", "dir")); err != nil {
		t.Fatalf("discoverV12EvidencePaths should tolerate missing directories, got: %v", err)
	}

	tracePath := filepath.Join(workDir, "trace.json")
	mustWriteJSON(t, tracePath, schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_x",
		ToolName:        "tool.a",
		ArgsDigest:      strings.Repeat("a", 64),
		IntentDigest:    strings.Repeat("b", 64),
		PolicyDigest:    strings.Repeat("c", 64),
		Verdict:         "allow",
	})
	if summary, err := buildTraceSummary([]string{tracePath}); err != nil || len(summary) == 0 {
		t.Fatalf("buildTraceSummary: len=%d err=%v", len(summary), err)
	}
	if _, err := buildTraceSummary([]string{filepath.Join(workDir, "missing_trace.json")}); err == nil {
		t.Fatalf("expected buildTraceSummary missing trace error")
	}

	regressPath := filepath.Join(workDir, "regress.json")
	mustWriteJSON(t, regressPath, schemaregress.RegressResult{
		SchemaID:        "gait.regress.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-dev",
		FixtureSet:      "fixture",
		Status:          "pass",
	})
	if summary, err := buildRegressSummary([]string{regressPath}); err != nil || len(summary) == 0 {
		t.Fatalf("buildRegressSummary: len=%d err=%v", len(summary), err)
	}
	if _, err := buildRegressSummary([]string{filepath.Join(workDir, "missing_regress.json")}); err == nil {
		t.Fatalf("expected buildRegressSummary missing file error")
	}
	if records, err := readApprovalAuditRecords(nil); err != nil || records != nil {
		t.Fatalf("readApprovalAuditRecords nil input: records=%#v err=%v", records, err)
	}
	if records, err := readBrokerCredentialRecords(nil); err != nil || records != nil {
		t.Fatalf("readBrokerCredentialRecords nil input: records=%#v err=%v", records, err)
	}

	if got := inferPackEntryType("runpack_summary.json"); got != "runpack" {
		t.Fatalf("inferPackEntryType runpack: %s", got)
	}
	if got := inferPackEntryType("trace_summary.json"); got != "trace" {
		t.Fatalf("inferPackEntryType trace: %s", got)
	}
	if got := inferPackEntryType("regress_summary.json"); got != "report" {
		t.Fatalf("inferPackEntryType regress: %s", got)
	}
	if got := inferPackEntryType("x.json"); got != "evidence" {
		t.Fatalf("inferPackEntryType evidence: %s", got)
	}

	paths := normalizePaths([]string{"a.json,b.json", "b.json", " "})
	if strings.Join(paths, ",") != "a.json,b.json" {
		t.Fatalf("normalizePaths mismatch: %#v", paths)
	}

	referenced, err := buildReferencedRunpackSummary(runpack.Runpack{
		Run: schemarunpack.Run{RunID: "run_r"},
		Refs: schemarunpack.Refs{Receipts: []schemarunpack.RefReceipt{
			{RefID: "ref_b", SourceType: "web", SourceLocator: "b", ContentDigest: strings.Repeat("d", 64), RetrievedAt: now},
			{RefID: "ref_a", SourceType: "web", SourceLocator: "a", ContentDigest: strings.Repeat("c", 64), RetrievedAt: now},
		}},
	})
	if err != nil || len(referenced) == 0 {
		t.Fatalf("buildReferencedRunpackSummary: len=%d err=%v", len(referenced), err)
	}

	contents := []schemaguard.PackEntry{{Path: "a", SHA256: strings.Repeat("a", 64), Type: "evidence"}}
	packID, err := computePackID("run_1", contents)
	if err != nil || !strings.HasPrefix(packID, "pack_") {
		t.Fatalf("computePackID: %s err=%v", packID, err)
	}
	if _, err := marshalCanonicalJSON(map[string]any{"k": "v"}); err != nil {
		t.Fatalf("marshalCanonicalJSON: %v", err)
	}
	if got := sha256Hex([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha256Hex mismatch: %s", got)
	}

	longData := bytes.Repeat([]byte("a"), maxEvidenceZipEntryBytes+1)
	var tooLarge bytes.Buffer
	if err := zipx.WriteDeterministicZip(&tooLarge, []zipx.File{{Path: "big.bin", Data: longData, Mode: 0o644}}); err != nil {
		t.Fatalf("write deterministic zip for big file: %v", err)
	}
	tooLargePath := filepath.Join(workDir, "too_large.zip")
	if err := os.WriteFile(tooLargePath, tooLarge.Bytes(), 0o600); err != nil {
		t.Fatalf("write too_large zip: %v", err)
	}
	reader, err := zip.OpenReader(tooLargePath)
	if err != nil {
		t.Fatalf("open too_large zip: %v", err)
	}
	defer func() { _ = reader.Close() }()
	if len(reader.File) != 1 {
		t.Fatalf("expected one file in too_large zip")
	}
	if _, err := readZipFile(reader.File[0]); err == nil {
		t.Fatalf("expected readZipFile size error")
	}
	if _, err := hashZipFile(reader.File[0]); err == nil {
		t.Fatalf("expected hashZipFile size error")
	}

	builder := Builder{ProducerVersion: "0.0.0-dev"}
	if _, err := builder.Build(context.Background(), BuildRequest{}); err == nil {
		t.Fatalf("expected Builder.Build missing runpack error")
	}
}
