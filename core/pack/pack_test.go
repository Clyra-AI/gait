package pack

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	coreerrors "github.com/davidahmann/gait/core/errors"
	"github.com/davidahmann/gait/core/guard"
	"github.com/davidahmann/gait/core/jobruntime"
	"github.com/davidahmann/gait/core/runpack"
	schemapack "github.com/davidahmann/gait/core/schema/v1/pack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/sign"
	"github.com/davidahmann/gait/core/zipx"
)

func TestBuildRunPackVerifyInspectAndDiff(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_pack_case")

	keys, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	result, err := BuildRunPack(BuildRunOptions{
		RunpackPath:       runpackPath,
		ProducerVersion:   "test-v24",
		SigningPrivateKey: keys.Private,
	})
	if err != nil {
		t.Fatalf("build run pack: %v", err)
	}
	if result.Path == "" {
		t.Fatalf("expected output path")
	}
	if result.Manifest.PackType != string(BuildTypeRun) {
		t.Fatalf("unexpected pack type: %s", result.Manifest.PackType)
	}
	if len(result.Manifest.Signatures) == 0 {
		t.Fatalf("expected manifest signature")
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("stat output path: %v", err)
	}

	verifyResult, err := Verify(result.Path, VerifyOptions{PublicKey: keys.Public, RequireSignature: true})
	if err != nil {
		t.Fatalf("verify run pack: %v", err)
	}
	if verifyResult.SignatureStatus != "verified" {
		t.Fatalf("expected verified signature status, got %s", verifyResult.SignatureStatus)
	}
	if len(verifyResult.HashMismatches) != 0 || len(verifyResult.MissingFiles) != 0 || len(verifyResult.UndeclaredFiles) != 0 {
		t.Fatalf("expected clean verify result: %#v", verifyResult)
	}

	inspectResult, err := Inspect(result.Path)
	if err != nil {
		t.Fatalf("inspect run pack: %v", err)
	}
	if inspectResult.RunPayload == nil {
		t.Fatalf("expected run payload")
	}
	if inspectResult.RunPayload.RunID != "run_pack_case" {
		t.Fatalf("unexpected run payload run_id: %s", inspectResult.RunPayload.RunID)
	}
	if inspectResult.RunLineage == nil || len(inspectResult.RunLineage.IntentResults) == 0 {
		t.Fatalf("expected run lineage details in inspect output")
	}

	diffResult, err := Diff(result.Path, result.Path)
	if err != nil {
		t.Fatalf("diff identical run packs: %v", err)
	}
	if diffResult.Result.Summary.Changed {
		t.Fatalf("identical pack diff should not report changed")
	}

	if _, err := LoadRunpackManifest(runpackPath); err != nil {
		t.Fatalf("load runpack manifest: %v", err)
	}
}

func TestBuildJobPackFromPathAndVerify(t *testing.T) {
	workDir := t.TempDir()
	jobsRoot := filepath.Join(workDir, "jobs")
	jobID := "job_pack_case"

	if _, err := jobruntime.Submit(jobsRoot, jobruntime.SubmitOptions{JobID: jobID}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, _, err := jobruntime.AddCheckpoint(jobsRoot, jobID, jobruntime.CheckpointOptions{
		Type:    jobruntime.CheckpointTypeProgress,
		Summary: "checkpoint progress",
	}); err != nil {
		t.Fatalf("add checkpoint: %v", err)
	}

	packPath := filepath.Join(workDir, "job_pack.zip")
	result, err := BuildJobPackFromPath(jobsRoot, jobID, packPath, "test-v24", nil)
	if err != nil {
		t.Fatalf("build job pack from path: %v", err)
	}
	if result.Manifest.PackType != string(BuildTypeJob) {
		t.Fatalf("unexpected job pack type: %s", result.Manifest.PackType)
	}
	if _, err := os.Stat(packPath); err != nil {
		t.Fatalf("stat job pack: %v", err)
	}

	verifyResult, err := Verify(packPath, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify job pack: %v", err)
	}
	if verifyResult.SignatureStatus != "missing" {
		t.Fatalf("expected missing signature status, got %s", verifyResult.SignatureStatus)
	}

	inspectResult, err := Inspect(packPath)
	if err != nil {
		t.Fatalf("inspect job pack: %v", err)
	}
	if inspectResult.JobPayload == nil {
		t.Fatalf("expected job payload")
	}
	if inspectResult.JobPayload.JobID != jobID {
		t.Fatalf("unexpected job payload job_id: %s", inspectResult.JobPayload.JobID)
	}
	if inspectResult.JobLineage == nil || inspectResult.JobLineage.EventCount == 0 {
		t.Fatalf("expected job lineage details in inspect output")
	}
}

func TestVerifyDetectsTamperAndUndeclaredFile(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_tamper_case")

	result, err := BuildRunPack(BuildRunOptions{RunpackPath: runpackPath})
	if err != nil {
		t.Fatalf("build run pack: %v", err)
	}

	tamperedPath := filepath.Join(workDir, "tampered_pack.zip")
	if err := rewriteZip(result.Path, tamperedPath, func(name string, payload []byte) (string, []byte) {
		if name == "run_payload.json" {
			return name, []byte(`{"schema_id":"gait.pack.run","schema_version":"1.0.0","run_id":"tampered"}`)
		}
		if name == "source/runpack.zip" {
			return name, append([]byte{}, payload[:len(payload)-1]...)
		}
		return name, payload
	}, map[string][]byte{"undeclared.txt": []byte("extra")}); err != nil {
		t.Fatalf("rewrite tampered zip: %v", err)
	}

	verifyResult, err := Verify(tamperedPath, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify tampered pack: %v", err)
	}
	if len(verifyResult.HashMismatches) == 0 {
		t.Fatalf("expected hash mismatches for tampered pack")
	}
	if len(verifyResult.UndeclaredFiles) == 0 {
		t.Fatalf("expected undeclared files for tampered pack")
	}

	missingManifestPath := filepath.Join(workDir, "missing_manifest.zip")
	if err := rewriteZip(result.Path, missingManifestPath, func(name string, payload []byte) (string, []byte) {
		if name == manifestFileName {
			return "", nil
		}
		return name, payload
	}, nil); err != nil {
		t.Fatalf("rewrite missing manifest zip: %v", err)
	}
	if _, err := Verify(missingManifestPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected missing manifest verify error")
	}
}

func TestVerifyLegacyArtifacts(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_legacy_case")

	legacyRunVerify, err := Verify(runpackPath, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify legacy runpack: %v", err)
	}
	if legacyRunVerify.LegacyType != "runpack" {
		t.Fatalf("expected legacy_type=runpack, got %s", legacyRunVerify.LegacyType)
	}

	guardPackPath := filepath.Join(workDir, "legacy_guard.zip")
	if _, err := guard.BuildPack(guard.BuildOptions{
		RunpackPath: runpackPath,
		OutputPath:  guardPackPath,
	}); err != nil {
		t.Fatalf("build guard pack: %v", err)
	}
	legacyGuardVerify, err := Verify(guardPackPath, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify legacy guard pack: %v", err)
	}
	if legacyGuardVerify.LegacyType != "guard" {
		t.Fatalf("expected legacy_type=guard, got %s", legacyGuardVerify.LegacyType)
	}

	legacyInspect, err := Inspect(runpackPath)
	if err != nil {
		t.Fatalf("inspect legacy runpack: %v", err)
	}
	if legacyInspect.LegacyType != "runpack" {
		t.Fatalf("expected inspect legacy_type=runpack, got %s", legacyInspect.LegacyType)
	}
}

func TestHelpersAndValidation(t *testing.T) {
	workDir := t.TempDir()

	if _, err := BuildRunPack(BuildRunOptions{}); err == nil {
		t.Fatalf("expected missing runpack path error")
	}
	if _, err := BuildJobPack(BuildJobOptions{}); err == nil {
		t.Fatalf("expected missing job state id error")
	}
	if _, err := buildPackWithFiles(buildPackOptions{PackType: "bad"}); err == nil {
		t.Fatalf("expected unsupported pack type error")
	}
	if _, err := BuildJobPackFromPath(filepath.Join(workDir, "missing"), "missing", "", "", nil); err == nil {
		t.Fatalf("expected build job pack from missing path to fail")
	}
	if _, err := LoadRunpackManifest(filepath.Join(workDir, "missing.zip")); err == nil {
		t.Fatalf("expected missing runpack path error")
	}
	if _, err := Diff(filepath.Join(workDir, "missing-left.zip"), filepath.Join(workDir, "missing-right.zip")); err == nil {
		t.Fatalf("expected diff missing artifact error")
	}
	if _, err := Inspect(filepath.Join(workDir, "missing.zip")); err == nil {
		t.Fatalf("expected inspect missing artifact error")
	}

	emptyJSONL, err := canonicalJSONL([]int{})
	if err != nil {
		t.Fatalf("canonical jsonl empty: %v", err)
	}
	if len(emptyJSONL) != 0 {
		t.Fatalf("expected empty jsonl output")
	}

	if detectEntryType("x.json") != "json" || detectEntryType("x.jsonl") != "jsonl" || detectEntryType("x.zip") != "zip" || detectEntryType("x.bin") != "blob" {
		t.Fatalf("detectEntryType returned unexpected values")
	}

	if normalizeTime(time.Time{}).IsZero() {
		t.Fatalf("normalizeTime zero should return deterministic timestamp")
	}
	past := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.FixedZone("PST", -8*60*60))
	if normalizeTime(past).Location() != time.UTC {
		t.Fatalf("normalizeTime should normalize to UTC")
	}

	m := map[string]string{"b": "2", "a": "1"}
	if got := sortedKeys(m); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("unexpected sorted keys: %#v", got)
	}

	if _, err := parsePackManifest([]byte(`{"schema_id":"bad","schema_version":"1.0.0"}`)); err == nil {
		t.Fatalf("expected parsePackManifest schema error")
	}
	if _, err := parsePackManifest([]byte(`{"schema_id":"gait.pack.manifest","schema_version":"2.0.0","pack_type":"run","source_ref":"x"}`)); err == nil {
		t.Fatalf("expected parsePackManifest schema_version error")
	}
	if _, err := parsePackManifest([]byte(`{"schema_id":"gait.pack.manifest","schema_version":"1.0.0","pack_type":"bad","source_ref":"x"}`)); err == nil {
		t.Fatalf("expected parsePackManifest pack_type error")
	}
	if _, err := parsePackManifest([]byte(`{"schema_id":"gait.pack.manifest","schema_version":"1.0.0","pack_type":"run","source_ref":" "}`)); err == nil {
		t.Fatalf("expected parsePackManifest source_ref error")
	}

	validManifest := schemapack.Manifest{
		SchemaID:        manifestSchemaID,
		SchemaVersion:   manifestSchemaVersion,
		CreatedAt:       time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		PackID:          strings.Repeat("a", 64),
		PackType:        string(BuildTypeRun),
		SourceRef:       "run_ok",
		Contents: []schemapack.PackEntry{{
			Path:   "run_payload.json",
			SHA256: strings.Repeat("b", 64),
			Type:   "json",
		}},
	}
	if _, err := parsePackManifest(mustMarshalJSON(t, validManifest)); err != nil {
		t.Fatalf("parse valid manifest: %v", err)
	}

	mismatches := convertRunpackMismatches([]runpack.HashMismatch{{Path: "a", Expected: "b", Actual: "c"}})
	if len(mismatches) != 1 || mismatches[0].Path != "a" {
		t.Fatalf("unexpected runpack mismatch conversion: %#v", mismatches)
	}
	guardMismatches := convertGuardMismatches([]guard.HashMismatch{{Path: "x", Expected: "y", Actual: "z"}})
	if len(guardMismatches) != 1 || guardMismatches[0].Path != "x" {
		t.Fatalf("unexpected guard mismatch conversion: %#v", guardMismatches)
	}
}

func TestDetectContextPrivacyMode(t *testing.T) {
	if mode := detectContextPrivacyMode(nil); mode != "" {
		t.Fatalf("expected empty privacy mode when receipts are empty, got %q", mode)
	}

	unknown := []schemarunpack.RefReceipt{
		{RefID: "ctx_1", RedactionMode: " "},
		{RefID: "ctx_2", RedactionMode: ""},
	}
	if mode := detectContextPrivacyMode(unknown); mode != "unknown" {
		t.Fatalf("expected unknown privacy mode for blank redaction_mode values, got %q", mode)
	}

	reference := []schemarunpack.RefReceipt{
		{RefID: "ctx_1", RedactionMode: "reference"},
		{RefID: "ctx_2", RedactionMode: "reference"},
	}
	if mode := detectContextPrivacyMode(reference); mode != "reference" {
		t.Fatalf("expected reference privacy mode, got %q", mode)
	}

	mixed := []schemarunpack.RefReceipt{
		{RefID: "ctx_1", RedactionMode: "reference"},
		{RefID: "ctx_2", RedactionMode: "raw"},
	}
	if mode := detectContextPrivacyMode(mixed); mode != "mixed" {
		t.Fatalf("expected mixed privacy mode, got %q", mode)
	}
}

func TestBuildRunPackWithDigestOnlyContextRefs(t *testing.T) {
	workDir := t.TempDir()
	runID := "run_digest_only_context"
	runpackPath := filepath.Join(workDir, "runpack_"+runID+".zip")
	createdAt := time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC)

	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:     runID,
			CreatedAt: createdAt,
			Env:       schemarunpack.RunEnv{OS: "darwin", Arch: "arm64", Runtime: "go"},
			Timeline:  []schemarunpack.TimelineEvt{{Event: "start", TS: createdAt}},
		},
		Intents: []schemarunpack.IntentRecord{{
			IntentID:   "intent_1",
			ToolName:   "tool.echo",
			ArgsDigest: strings.Repeat("2", 64),
			Args:       map[string]any{"message": "hello"},
		}},
		Results: []schemarunpack.ResultRecord{{
			IntentID:     "intent_1",
			Status:       "ok",
			ResultDigest: strings.Repeat("3", 64),
			Result:       map[string]any{"ok": true},
		}},
		Refs: schemarunpack.Refs{
			RunID:               runID,
			ContextSetDigest:    strings.Repeat("a", 64),
			ContextEvidenceMode: "required",
			Receipts:            []schemarunpack.RefReceipt{},
		},
	})
	if err != nil {
		t.Fatalf("write runpack fixture: %v", err)
	}

	result, err := BuildRunPack(BuildRunOptions{RunpackPath: runpackPath})
	if err != nil {
		t.Fatalf("build run pack from digest-only refs runpack: %v", err)
	}

	bundle, err := openZip(result.Path)
	if err != nil {
		t.Fatalf("open built run pack: %v", err)
	}
	defer func() {
		_ = bundle.Close()
	}()
	if _, ok := bundle.Files["context_envelope.json"]; ok {
		t.Fatalf("did not expect context_envelope.json when receipts are empty")
	}

	inspectResult, err := Inspect(result.Path)
	if err != nil {
		t.Fatalf("inspect built run pack: %v", err)
	}
	if inspectResult.RunPayload == nil || inspectResult.RunPayload.ContextSetDigest != strings.Repeat("a", 64) {
		t.Fatalf("expected run payload to retain context_set_digest from refs: %#v", inspectResult.RunPayload)
	}
}

func TestVerifySignatureModesAndMissingDeclaredFiles(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_signature_modes")

	signingKeys, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate signing key pair: %v", err)
	}
	signedPack, err := BuildRunPack(BuildRunOptions{
		RunpackPath:       runpackPath,
		OutputPath:        filepath.Join(workDir, "signed_pack.zip"),
		SigningPrivateKey: signingKeys.Private,
	})
	if err != nil {
		t.Fatalf("build signed run pack: %v", err)
	}

	skippedVerify, err := Verify(signedPack.Path, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify signed pack without key: %v", err)
	}
	if skippedVerify.SignatureStatus != "skipped" {
		t.Fatalf("expected signature_status=skipped got %s", skippedVerify.SignatureStatus)
	}

	wrongKeys, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate wrong key pair: %v", err)
	}
	failedVerify, err := Verify(signedPack.Path, VerifyOptions{PublicKey: wrongKeys.Public, RequireSignature: true})
	if err != nil {
		t.Fatalf("verify signed pack with wrong key: %v", err)
	}
	if failedVerify.SignatureStatus != "failed" {
		t.Fatalf("expected signature_status=failed got %s", failedVerify.SignatureStatus)
	}

	unsignedPack, err := BuildRunPack(BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  filepath.Join(workDir, "unsigned_pack.zip"),
	})
	if err != nil {
		t.Fatalf("build unsigned run pack: %v", err)
	}
	missingSignatureVerify, err := Verify(unsignedPack.Path, VerifyOptions{RequireSignature: true})
	if err != nil {
		t.Fatalf("verify unsigned pack with required signature: %v", err)
	}
	if missingSignatureVerify.SignatureStatus != "missing" {
		t.Fatalf("expected signature_status=missing got %s", missingSignatureVerify.SignatureStatus)
	}
	if len(missingSignatureVerify.SignatureErrors) == 0 {
		t.Fatalf("expected signature errors for required signature on unsigned pack")
	}

	missingDeclaredPath := filepath.Join(workDir, "missing_declared.zip")
	if err := rewriteZip(signedPack.Path, missingDeclaredPath, func(name string, payload []byte) (string, []byte) {
		if name == "run_payload.json" {
			return "", nil
		}
		return name, payload
	}, nil); err != nil {
		t.Fatalf("rewrite missing declared zip: %v", err)
	}
	missingDeclaredVerify, err := Verify(missingDeclaredPath, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify missing declared zip: %v", err)
	}
	if len(missingDeclaredVerify.MissingFiles) == 0 {
		t.Fatalf("expected missing files for removed declared entry")
	}
}

func TestDiffInspectAndVerifyErrorBranches(t *testing.T) {
	workDir := t.TempDir()
	runpackA := createRunpackFixture(t, workDir, "run_diff_a")
	runpackB := createRunpackFixture(t, workDir, "run_diff_b")

	packA, err := BuildRunPack(BuildRunOptions{RunpackPath: runpackA, OutputPath: filepath.Join(workDir, "pack_a.zip")})
	if err != nil {
		t.Fatalf("build pack a: %v", err)
	}
	packB, err := BuildRunPack(BuildRunOptions{RunpackPath: runpackB, OutputPath: filepath.Join(workDir, "pack_b.zip")})
	if err != nil {
		t.Fatalf("build pack b: %v", err)
	}

	legacyInfo, err := collectArtifactInfo(runpackA)
	if err != nil {
		t.Fatalf("collect legacy artifact info: %v", err)
	}
	if legacyInfo.PackType != string(BuildTypeRun) {
		t.Fatalf("unexpected legacy artifact pack type: %s", legacyInfo.PackType)
	}

	diffAB, err := Diff(packA.Path, packB.Path)
	if err != nil {
		t.Fatalf("diff run packs: %v", err)
	}
	if !diffAB.Result.Summary.Changed || len(diffAB.Result.Summary.ChangedFiles) == 0 {
		t.Fatalf("expected changed file entries for different run packs: %#v", diffAB.Result.Summary)
	}

	if _, err := Diff(packA.Path, filepath.Join(workDir, "missing.zip")); err == nil {
		t.Fatalf("expected diff right-side error")
	}

	jobPack, err := BuildJobPack(BuildJobOptions{
		State: jobruntime.JobState{
			JobID:            "job_diff_case",
			SchemaID:         "gait.job.runtime",
			SchemaVersion:    "1.0.0",
			Status:           "running",
			StopReason:       "none",
			StatusReasonCode: "submitted",
			Checkpoints:      []jobruntime.Checkpoint{},
			Approvals:        []jobruntime.Approval{},
		},
		OutputPath: filepath.Join(workDir, "pack_job_diff.zip"),
	})
	if err != nil {
		t.Fatalf("build job pack: %v", err)
	}
	diffRunJob, err := Diff(packA.Path, jobPack.Path)
	if err != nil {
		t.Fatalf("diff run vs job packs: %v", err)
	}
	if len(diffRunJob.Result.Summary.AddedFiles) == 0 || len(diffRunJob.Result.Summary.RemovedFiles) == 0 {
		t.Fatalf("expected added and removed files in run-vs-job diff: %#v", diffRunJob.Result.Summary)
	}

	invalidManifestPath := filepath.Join(workDir, "invalid_manifest.zip")
	if err := rewriteZip(packA.Path, invalidManifestPath, func(name string, payload []byte) (string, []byte) {
		if name == manifestFileName {
			return name, []byte(`{"schema_id":"gait.pack.invalid","schema_version":"1.0.0","pack_type":"run","source_ref":"x"}`)
		}
		return name, payload
	}, nil); err != nil {
		t.Fatalf("rewrite invalid manifest zip: %v", err)
	}
	if _, err := Verify(invalidManifestPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected verify parse manifest error")
	}
	if _, err := Inspect(invalidManifestPath); err == nil {
		t.Fatalf("expected inspect parse manifest error")
	}
	if _, err := collectArtifactInfo(invalidManifestPath); err == nil {
		t.Fatalf("expected collectArtifactInfo parse manifest error")
	}

	corruptManifestPath := filepath.Join(workDir, "corrupt_manifest.zip")
	if err := corruptZipEntryByte(packA.Path, corruptManifestPath, manifestFileName); err != nil {
		t.Fatalf("corrupt manifest zip entry: %v", err)
	}
	if _, err := Verify(corruptManifestPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected verify manifest read error for corrupted manifest bytes")
	}
	if _, err := Inspect(corruptManifestPath); err == nil {
		t.Fatalf("expected inspect manifest read error for corrupted manifest bytes")
	}
	if _, err := collectArtifactInfo(corruptManifestPath); err == nil {
		t.Fatalf("expected collectArtifactInfo manifest read error for corrupted manifest bytes")
	}

	corruptPayloadPath := filepath.Join(workDir, "corrupt_payload.zip")
	if err := corruptZipEntryByte(packA.Path, corruptPayloadPath, "run_payload.json"); err != nil {
		t.Fatalf("corrupt payload zip entry: %v", err)
	}
	if _, err := Verify(corruptPayloadPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected verify hash read error for corrupted payload bytes")
	}

	missingManifestPath := filepath.Join(workDir, "missing_manifest_all.zip")
	if err := writeZipEntries(missingManifestPath, map[string][]byte{"payload.bin": []byte("x")}); err != nil {
		t.Fatalf("write zip without manifest: %v", err)
	}
	if _, err := Verify(missingManifestPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected verify missing manifest error")
	}
	if _, err := Inspect(missingManifestPath); err == nil {
		t.Fatalf("expected inspect missing manifest error")
	}
	if _, err := collectArtifactInfo(missingManifestPath); err == nil {
		t.Fatalf("expected collectArtifactInfo missing manifest error")
	}

	invalidLegacyPath := filepath.Join(workDir, "invalid_legacy.zip")
	if err := writeZipEntries(invalidLegacyPath, map[string][]byte{"manifest.json": []byte("{}")}); err != nil {
		t.Fatalf("write invalid legacy zip: %v", err)
	}
	if _, err := Verify(invalidLegacyPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected verify invalid legacy runpack error")
	}
	if _, err := Inspect(invalidLegacyPath); err == nil {
		t.Fatalf("expected inspect invalid legacy runpack error")
	}
	if _, err := collectArtifactInfo(invalidLegacyPath); err == nil {
		t.Fatalf("expected collectArtifactInfo invalid legacy runpack error")
	}
}

func TestBuildAndUtilityErrorBranches(t *testing.T) {
	workDir := t.TempDir()
	invalidRunpack := filepath.Join(workDir, "invalid-runpack.zip")
	if err := os.WriteFile(invalidRunpack, []byte("not-a-zip"), 0o600); err != nil {
		t.Fatalf("write invalid runpack fixture: %v", err)
	}
	if _, err := BuildRunPack(BuildRunOptions{RunpackPath: invalidRunpack}); err == nil {
		t.Fatalf("expected invalid runpack build error")
	}
	if _, err := Verify(filepath.Join(workDir, "missing-pack.zip"), VerifyOptions{}); err == nil {
		t.Fatalf("expected verify missing path error")
	}

	if _, err := BuildJobPack(BuildJobOptions{
		State: jobruntime.JobState{JobID: "job-events-encode"},
		Events: []jobruntime.Event{
			{
				Payload: map[string]any{"bad": func() {}},
			},
		},
	}); err == nil {
		t.Fatalf("expected build job pack event encode error")
	}

	parentFile := filepath.Join(workDir, "parent-file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	if _, err := buildPackWithFiles(buildPackOptions{
		PackType:   string(BuildTypeRun),
		SourceRef:  "src",
		OutputPath: filepath.Join(parentFile, "pack.zip"),
		Files: []zipx.File{
			{Path: "run_payload.json", Data: []byte(`{"ok":true}`), Mode: 0o644},
		},
	}); err == nil {
		t.Fatalf("expected build pack mkdir error")
	}

	dirOutput := filepath.Join(workDir, "pack_output_dir")
	if err := os.MkdirAll(dirOutput, 0o750); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	if _, err := buildPackWithFiles(buildPackOptions{
		PackType:   string(BuildTypeRun),
		SourceRef:  "src",
		OutputPath: dirOutput,
		Files: []zipx.File{
			{Path: "run_payload.json", Data: []byte(`{"ok":true}`), Mode: 0o644},
		},
	}); err == nil {
		t.Fatalf("expected build pack write file error")
	}

	defaultOut, err := buildPackWithFiles(buildPackOptions{
		PackType:  string(BuildTypeRun),
		SourceRef: "default_out",
		Files: []zipx.File{
			{Path: "run_payload.json", Data: []byte(`{"ok":true}`), Mode: 0o644},
		},
	})
	if err != nil {
		t.Fatalf("build pack with default output fallback: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(defaultOut.Path), "gait-out/pack_") {
		t.Fatalf("expected default output path under gait-out, got %s", defaultOut.Path)
	}

	nestedOutPath := filepath.Join(workDir, "nested", "pack", "pack_nested.zip")
	nestedOut, err := buildPackWithFiles(buildPackOptions{
		PackType:   string(BuildTypeRun),
		SourceRef:  "nested_out",
		OutputPath: nestedOutPath,
		Files: []zipx.File{
			{Path: "run_payload.json", Data: []byte(`{"ok":true}`), Mode: 0o644},
		},
	})
	if err != nil {
		t.Fatalf("build pack with nested output path: %v", err)
	}
	if nestedOut.Path != nestedOutPath {
		t.Fatalf("expected nested output path %s got %s", nestedOutPath, nestedOut.Path)
	}
	if _, err := os.Stat(nestedOutPath); err != nil {
		t.Fatalf("expected nested output pack file to exist: %v", err)
	}

	if _, err := parsePackManifest([]byte("{")); err == nil {
		t.Fatalf("expected parsePackManifest invalid json error")
	}
	if _, err := canonicalJSON(map[string]any{"bad": func() {}}); err == nil {
		t.Fatalf("expected canonicalJSON encode error")
	}
	if _, err := canonicalJSONL([]map[string]any{{"bad": func() {}}}); err == nil {
		t.Fatalf("expected canonicalJSONL encode error")
	}

	if err := (&openedZip{}).Close(); err != nil {
		t.Fatalf("expected openedZip Close nil-safe behavior: %v", err)
	}
}

func TestBuildRunPackDeterministicBytes(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_deterministic")

	firstPath := filepath.Join(workDir, "pack_first.zip")
	secondPath := filepath.Join(workDir, "pack_second.zip")

	first, err := BuildRunPack(BuildRunOptions{RunpackPath: runpackPath, OutputPath: firstPath})
	if err != nil {
		t.Fatalf("build first run pack: %v", err)
	}
	second, err := BuildRunPack(BuildRunOptions{RunpackPath: runpackPath, OutputPath: secondPath})
	if err != nil {
		t.Fatalf("build second run pack: %v", err)
	}
	if first.Manifest.PackID != second.Manifest.PackID {
		t.Fatalf("expected stable pack id across identical input")
	}

	firstBytes, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatalf("read first pack bytes: %v", err)
	}
	secondBytes, err := os.ReadFile(second.Path)
	if err != nil {
		t.Fatalf("read second pack bytes: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("expected deterministic pack bytes across identical input")
	}
}

func TestVerifyRejectsSchemaInvalidRunPayload(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_schema_invalid")

	result, err := BuildRunPack(BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  filepath.Join(workDir, "pack_run.zip"),
	})
	if err != nil {
		t.Fatalf("build run pack: %v", err)
	}

	mutatedPath := filepath.Join(workDir, "pack_run_invalid_payload.zip")
	if err := rewritePackWithMutatedPayloadAndManifest(result.Path, mutatedPath, map[string][]byte{
		"run_payload.json": []byte(`{"schema_id":"gait.pack.run","schema_version":"1.0.0","created_at":"2026-02-14T00:00:00Z","run_id":"run_schema_invalid","capture_mode":"reference","manifest_digest":"` + strings.Repeat("a", 64) + `","intents_count":1,"results_count":1,"refs_count":1,"unexpected":"field"}`),
	}); err != nil {
		t.Fatalf("rewrite invalid schema payload pack: %v", err)
	}

	if _, err := Verify(mutatedPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected verify error for schema-invalid payload")
	} else if coreerrors.CategoryOf(err) != coreerrors.CategoryVerification {
		t.Fatalf("expected verification-category error, got %q (%v)", coreerrors.CategoryOf(err), err)
	}
}

func TestExtractRunpackVariants(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_extract")

	runPackPath := filepath.Join(workDir, "pack_run_extract.zip")
	if _, err := BuildRunPack(BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  runPackPath,
	}); err != nil {
		t.Fatalf("build run pack: %v", err)
	}

	legacyBytes, err := ExtractRunpack(runpackPath)
	if err != nil {
		t.Fatalf("extract legacy runpack: %v", err)
	}
	originalLegacyBytes, err := os.ReadFile(runpackPath)
	if err != nil {
		t.Fatalf("read legacy runpack: %v", err)
	}
	if !bytes.Equal(legacyBytes, originalLegacyBytes) {
		t.Fatalf("expected extracted legacy runpack bytes to match source")
	}

	extractedFromPack, err := ExtractRunpack(runPackPath)
	if err != nil {
		t.Fatalf("extract source runpack from pack artifact: %v", err)
	}
	extractedPath := filepath.Join(workDir, "extracted_runpack.zip")
	if err := os.WriteFile(extractedPath, extractedFromPack, 0o600); err != nil {
		t.Fatalf("write extracted runpack: %v", err)
	}
	if _, err := runpack.VerifyZip(extractedPath, runpack.VerifyOptions{RequireSignature: false}); err != nil {
		t.Fatalf("verify extracted runpack: %v", err)
	}

	jobPackPath := filepath.Join(workDir, "pack_job_extract.zip")
	if _, err := BuildJobPack(BuildJobOptions{
		State: jobruntime.JobState{
			JobID:                  "job_extract",
			SchemaID:               "gait.job.runtime",
			SchemaVersion:          "1.0.0",
			Status:                 jobruntime.StatusRunning,
			StopReason:             jobruntime.StopReasonNone,
			StatusReasonCode:       "submitted",
			EnvironmentFingerprint: "envfp:test",
		},
		OutputPath: jobPackPath,
	}); err != nil {
		t.Fatalf("build job pack: %v", err)
	}
	if _, err := ExtractRunpack(jobPackPath); err == nil {
		t.Fatalf("expected ExtractRunpack to fail for job pack")
	}

	invalidZipPath := filepath.Join(workDir, "invalid_extract.zip")
	if err := writeZipEntries(invalidZipPath, map[string][]byte{"payload.bin": []byte("x")}); err != nil {
		t.Fatalf("write invalid extract zip: %v", err)
	}
	if _, err := ExtractRunpack(invalidZipPath); err == nil {
		t.Fatalf("expected ExtractRunpack to fail for zip without manifest entries")
	}
}

func TestValidationHelperCoverage(t *testing.T) {
	validRunPayload := schemapack.RunPayload{
		SchemaID:       "gait.pack.run",
		SchemaVersion:  "1.0.0",
		CreatedAt:      time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		RunID:          "run_valid",
		CaptureMode:    "reference",
		ManifestDigest: strings.Repeat("a", 64),
		IntentsCount:   1,
		ResultsCount:   1,
		RefsCount:      1,
	}
	if err := validateRunPayload(validRunPayload); err != nil {
		t.Fatalf("validateRunPayload expected success: %v", err)
	}
	invalidRunPayload := validRunPayload
	invalidRunPayload.CaptureMode = "invalid"
	if err := validateRunPayload(invalidRunPayload); err == nil {
		t.Fatalf("validateRunPayload expected capture_mode error")
	}

	validJobPayload := schemapack.JobPayload{
		SchemaID:               "gait.pack.job",
		SchemaVersion:          "1.0.0",
		CreatedAt:              time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		JobID:                  "job_valid",
		Status:                 jobruntime.StatusRunning,
		StopReason:             jobruntime.StopReasonNone,
		StatusReasonCode:       "submitted",
		EnvironmentFingerprint: "envfp:test",
		CheckpointCount:        0,
		ApprovalCount:          0,
	}
	if err := validateJobPayload(validJobPayload); err != nil {
		t.Fatalf("validateJobPayload expected success: %v", err)
	}
	invalidJobPayload := validJobPayload
	invalidJobPayload.Status = "bad"
	if err := validateJobPayload(invalidJobPayload); err == nil {
		t.Fatalf("validateJobPayload expected invalid status error")
	}

	validJobState := jobruntime.JobState{
		JobID:                  "job_state_valid",
		Status:                 jobruntime.StatusPaused,
		StopReason:             jobruntime.StopReasonPausedByUser,
		StatusReasonCode:       "paused",
		EnvironmentFingerprint: "envfp:test",
	}
	if err := validateJobState(validJobState); err != nil {
		t.Fatalf("validateJobState expected success: %v", err)
	}
	invalidJobState := validJobState
	invalidJobState.EnvironmentFingerprint = ""
	if err := validateJobState(invalidJobState); err == nil {
		t.Fatalf("validateJobState expected missing environment_fingerprint error")
	}

	if !validJobStatus(jobruntime.StatusCancelled) {
		t.Fatalf("validJobStatus should allow cancelled")
	}
	if validJobStatus("not-real") {
		t.Fatalf("validJobStatus should reject unknown status")
	}

	var parsed map[string]any
	if err := decodeStrictJSON([]byte(`{"ok":true}`), &parsed); err != nil {
		t.Fatalf("decodeStrictJSON expected success: %v", err)
	}
	if err := decodeStrictJSON([]byte(`{"ok":true}{"next":true}`), &parsed); err == nil {
		t.Fatalf("decodeStrictJSON expected multi-value error")
	}

	type strictStruct struct {
		OK bool `json:"ok"`
	}
	var strictValue strictStruct
	if err := decodeStrictJSON([]byte(`{"ok":true,"extra":1}`), &strictValue); err == nil {
		t.Fatalf("decodeStrictJSON expected unknown-field error")
	}

	events, err := parseJobEvents([]byte(`{"schema_id":"gait.job.event","schema_version":"1.0.0","created_at":"2026-02-14T00:00:00Z","job_id":"job","revision":1,"type":"submitted"}` + "\n"))
	if err != nil {
		t.Fatalf("parseJobEvents expected success: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one parsed event")
	}
	if _, err := parseJobEvents([]byte("{bad-json}\n")); err == nil {
		t.Fatalf("parseJobEvents expected parse error")
	}

	if isSHA256Hex(strings.Repeat("z", 64)) {
		t.Fatalf("isSHA256Hex should reject non-hex input")
	}
}

func TestValidationHelpersExhaustiveErrors(t *testing.T) {
	runBase := schemapack.RunPayload{
		SchemaID:       "gait.pack.run",
		SchemaVersion:  "1.0.0",
		CreatedAt:      time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		RunID:          "run_valid",
		CaptureMode:    "reference",
		ManifestDigest: strings.Repeat("a", 64),
		IntentsCount:   1,
		ResultsCount:   1,
		RefsCount:      1,
	}
	runCases := []func(*schemapack.RunPayload){
		func(value *schemapack.RunPayload) { value.SchemaID = "bad" },
		func(value *schemapack.RunPayload) { value.SchemaVersion = "2.0.0" },
		func(value *schemapack.RunPayload) { value.CreatedAt = time.Time{} },
		func(value *schemapack.RunPayload) { value.RunID = "" },
		func(value *schemapack.RunPayload) { value.CaptureMode = "bad" },
		func(value *schemapack.RunPayload) { value.ManifestDigest = "bad" },
		func(value *schemapack.RunPayload) { value.IntentsCount = -1 },
	}
	for _, mutate := range runCases {
		value := runBase
		mutate(&value)
		if err := validateRunPayload(value); err == nil {
			t.Fatalf("validateRunPayload expected error for mutation")
		}
	}

	jobBase := schemapack.JobPayload{
		SchemaID:               "gait.pack.job",
		SchemaVersion:          "1.0.0",
		CreatedAt:              time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		JobID:                  "job_valid",
		Status:                 jobruntime.StatusRunning,
		StopReason:             jobruntime.StopReasonNone,
		StatusReasonCode:       "submitted",
		EnvironmentFingerprint: "envfp:test",
		CheckpointCount:        0,
		ApprovalCount:          0,
	}
	jobPayloadCases := []func(*schemapack.JobPayload){
		func(value *schemapack.JobPayload) { value.SchemaID = "bad" },
		func(value *schemapack.JobPayload) { value.SchemaVersion = "2.0.0" },
		func(value *schemapack.JobPayload) { value.CreatedAt = time.Time{} },
		func(value *schemapack.JobPayload) { value.JobID = "" },
		func(value *schemapack.JobPayload) { value.Status = "bad" },
		func(value *schemapack.JobPayload) { value.StopReason = "" },
		func(value *schemapack.JobPayload) { value.StatusReasonCode = "" },
		func(value *schemapack.JobPayload) { value.EnvironmentFingerprint = "" },
		func(value *schemapack.JobPayload) { value.CheckpointCount = -1 },
	}
	for _, mutate := range jobPayloadCases {
		value := jobBase
		mutate(&value)
		if err := validateJobPayload(value); err == nil {
			t.Fatalf("validateJobPayload expected error for mutation")
		}
	}

	stateBase := jobruntime.JobState{
		JobID:                  "job_state_valid",
		Status:                 jobruntime.StatusRunning,
		StopReason:             jobruntime.StopReasonNone,
		StatusReasonCode:       "submitted",
		EnvironmentFingerprint: "envfp:test",
	}
	jobStateCases := []func(*jobruntime.JobState){
		func(value *jobruntime.JobState) { value.JobID = "" },
		func(value *jobruntime.JobState) { value.Status = "bad" },
		func(value *jobruntime.JobState) { value.StopReason = "" },
		func(value *jobruntime.JobState) { value.StatusReasonCode = "" },
		func(value *jobruntime.JobState) { value.EnvironmentFingerprint = "" },
	}
	for _, mutate := range jobStateCases {
		value := stateBase
		mutate(&value)
		if err := validateJobState(value); err == nil {
			t.Fatalf("validateJobState expected error for mutation")
		}
	}
}

func TestParsePackManifestValidationBranches(t *testing.T) {
	base := schemapack.Manifest{
		SchemaID:        manifestSchemaID,
		SchemaVersion:   manifestSchemaVersion,
		CreatedAt:       time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		PackID:          strings.Repeat("a", 64),
		PackType:        string(BuildTypeRun),
		SourceRef:       "run_branch",
		Contents: []schemapack.PackEntry{{
			Path:   "run_payload.json",
			SHA256: strings.Repeat("b", 64),
			Type:   "json",
		}},
	}

	cases := []struct {
		name   string
		mutate func(*schemapack.Manifest)
	}{
		{
			name: "missing_created_at",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.CreatedAt = time.Time{}
			},
		},
		{
			name: "missing_producer_version",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.ProducerVersion = ""
			},
		},
		{
			name: "invalid_pack_id",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.PackID = "bad"
			},
		},
		{
			name: "missing_contents",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.Contents = nil
			},
		},
		{
			name: "empty_entry_path",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.Contents = []schemapack.PackEntry{{Path: "", SHA256: strings.Repeat("b", 64), Type: "json"}}
			},
		},
		{
			name: "invalid_entry_sha",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.Contents = []schemapack.PackEntry{{Path: "run_payload.json", SHA256: "bad", Type: "json"}}
			},
		},
		{
			name: "empty_entry_type",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.Contents = []schemapack.PackEntry{{Path: "run_payload.json", SHA256: strings.Repeat("b", 64), Type: ""}}
			},
		},
		{
			name: "signature_missing_required_fields",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.Signatures = []schemapack.Signature{{Alg: "", KeyID: "kid", Sig: "sig"}}
			},
		},
		{
			name: "signature_invalid_signed_digest",
			mutate: func(manifest *schemapack.Manifest) {
				manifest.Signatures = []schemapack.Signature{{Alg: "ed25519", KeyID: "kid", Sig: "sig", SignedDigest: "bad"}}
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value := base
			value.Contents = append([]schemapack.PackEntry{}, base.Contents...)
			testCase.mutate(&value)
			if _, err := parsePackManifest(mustMarshalJSON(t, value)); err == nil {
				t.Fatalf("expected parsePackManifest to fail for case %s", testCase.name)
			}
		})
	}
}

func TestVerifyPayloadContractsBranchCoverage(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_contract")
	runPackPath := filepath.Join(workDir, "pack_run_contract.zip")
	if _, err := BuildRunPack(BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  runPackPath,
	}); err != nil {
		t.Fatalf("build run pack: %v", err)
	}
	jobPackPath := filepath.Join(workDir, "pack_job_contract.zip")
	if _, err := BuildJobPack(BuildJobOptions{
		State: jobruntime.JobState{
			JobID:                  "job_contract",
			SchemaID:               "gait.job.runtime",
			SchemaVersion:          "1.0.0",
			Status:                 jobruntime.StatusRunning,
			StopReason:             jobruntime.StopReasonNone,
			StatusReasonCode:       "submitted",
			EnvironmentFingerprint: "envfp:test",
		},
		OutputPath: jobPackPath,
	}); err != nil {
		t.Fatalf("build job pack: %v", err)
	}

	runBundle, runManifest := openPackBundleAndManifest(t, runPackPath)
	if err := verifyPayloadContracts(runBundle, runManifest); err != nil {
		t.Fatalf("verifyPayloadContracts run expected success: %v", err)
	}
	delete(runBundle.Files, "source/runpack.zip")
	if err := verifyPayloadContracts(runBundle, runManifest); err == nil {
		t.Fatalf("verifyPayloadContracts run expected missing source runpack error")
	}
	_ = runBundle.Close()

	jobBundle, jobManifest := openPackBundleAndManifest(t, jobPackPath)
	if err := verifyPayloadContracts(jobBundle, jobManifest); err != nil {
		t.Fatalf("verifyPayloadContracts job expected success: %v", err)
	}
	delete(jobBundle.Files, "job_events.jsonl")
	if err := verifyPayloadContracts(jobBundle, jobManifest); err == nil {
		t.Fatalf("verifyPayloadContracts job expected missing events error")
	}
	_ = jobBundle.Close()

	jobBundleMismatch, jobManifestMismatch := openPackBundleAndManifest(t, jobPackPath)
	jobManifestMismatch.SourceRef = "job_other"
	if err := verifyPayloadContracts(jobBundleMismatch, jobManifestMismatch); err == nil {
		t.Fatalf("verifyPayloadContracts expected source_ref mismatch error")
	}
	_ = jobBundleMismatch.Close()

	if _, err := readRunpackFromBytes([]byte("not-a-zip")); err == nil {
		t.Fatalf("readRunpackFromBytes expected invalid runpack bytes error")
	}
}

func createRunpackFixture(t *testing.T, dir string, runID string) string {
	t.Helper()
	path := filepath.Join(dir, "runpack_"+runID+".zip")
	createdAt := time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC)

	run := schemarunpack.Run{
		RunID:     runID,
		CreatedAt: createdAt,
		Env:       schemarunpack.RunEnv{OS: "darwin", Arch: "arm64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: createdAt},
		},
	}

	_, err := runpack.WriteRunpack(path, runpack.RecordOptions{
		Run: run,
		Intents: []schemarunpack.IntentRecord{
			{
				IntentID:   "intent_1",
				ToolName:   "tool.echo",
				ArgsDigest: strings.Repeat("2", 64),
				Args:       map[string]any{"message": "hello"},
			},
		},
		Results: []schemarunpack.ResultRecord{
			{
				IntentID:     "intent_1",
				Status:       "ok",
				ResultDigest: strings.Repeat("3", 64),
				Result:       map[string]any{"ok": true},
			},
		},
		Refs: schemarunpack.Refs{
			RunID: runID,
		},
	})
	if err != nil {
		t.Fatalf("write runpack fixture: %v", err)
	}
	return path
}

func rewriteZip(srcPath string, dstPath string, mutate func(name string, payload []byte) (string, []byte), extraFiles map[string][]byte) error {
	src, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = src.Close()
	}()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range src.File {
		reader, err := entry.Open()
		if err != nil {
			_ = writer.Close()
			return err
		}
		payload, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			_ = writer.Close()
			return err
		}
		name, updated := mutate(entry.Name, payload)
		if name == "" {
			continue
		}
		target, err := writer.Create(name)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if _, err := target.Write(updated); err != nil {
			_ = writer.Close()
			return err
		}
	}
	for name, payload := range extraFiles {
		target, err := writer.Create(name)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if _, err := target.Write(payload); err != nil {
			_ = writer.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(dstPath, buffer.Bytes(), 0o600)
}

func writeZipEntries(path string, entries map[string][]byte) error {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, payload := range entries {
		target, err := writer.Create(name)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if _, err := target.Write(payload); err != nil {
			_ = writer.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o600)
}

func corruptZipEntryByte(srcPath string, dstPath string, entryName string) error {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	var offset int64 = -1
	for _, file := range reader.File {
		if file.Name != entryName {
			continue
		}
		entryOffset, err := file.DataOffset()
		if err != nil {
			return err
		}
		offset = entryOffset
		break
	}
	if offset < 0 {
		return os.ErrNotExist
	}

	payload, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	if int(offset) >= len(payload) {
		return io.ErrUnexpectedEOF
	}
	payload[offset] ^= 0xFF
	return os.WriteFile(dstPath, payload, 0o600)
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

func rewritePackWithMutatedPayloadAndManifest(srcPath string, dstPath string, replacements map[string][]byte) error {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	files := map[string][]byte{}
	for _, entry := range reader.File {
		entryReader, err := entry.Open()
		if err != nil {
			return err
		}
		content, err := io.ReadAll(entryReader)
		_ = entryReader.Close()
		if err != nil {
			return err
		}
		files[entry.Name] = content
	}
	for path, content := range replacements {
		files[path] = content
	}

	manifestBytes, ok := files[manifestFileName]
	if !ok {
		return os.ErrNotExist
	}
	var manifest schemapack.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return err
	}
	manifest.Signatures = nil
	for index := range manifest.Contents {
		content, exists := files[manifest.Contents[index].Path]
		if !exists {
			continue
		}
		sum := sha256.Sum256(content)
		manifest.Contents[index].SHA256 = hex.EncodeToString(sum[:])
	}
	packID, err := computePackID(manifest)
	if err != nil {
		return err
	}
	manifest.PackID = packID
	canonicalManifest, err := canonicalJSON(manifest)
	if err != nil {
		return err
	}
	files[manifestFileName] = canonicalManifest

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target, err := writer.Create(name)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if _, err := target.Write(files[name]); err != nil {
			_ = writer.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(dstPath, buffer.Bytes(), 0o600)
}

func openPackBundleAndManifest(t *testing.T, path string) (*openedZip, schemapack.Manifest) {
	t.Helper()
	bundle, err := openZip(path)
	if err != nil {
		t.Fatalf("open pack bundle: %v", err)
	}
	manifestFile, ok := bundle.Files[manifestFileName]
	if !ok {
		t.Fatalf("bundle missing manifest file")
	}
	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		t.Fatalf("read manifest bytes: %v", err)
	}
	manifest, err := parsePackManifest(manifestBytes)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return bundle, manifest
}
