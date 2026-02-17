package runpack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
	"github.com/Clyra-AI/gait/core/schema/validate"
	"github.com/Clyra-AI/gait/core/sign"
)

func TestRecordRunSignedValid(test *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		test.Fatalf("generate keypair: %v", err)
	}
	run := schemarunpack.Run{
		SchemaID:        "gait.runpack.run",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		RunID:           "run_demo",
		Env: schemarunpack.RunEnv{
			OS:      "darwin",
			Arch:    "arm64",
			Runtime: "go",
		},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	intents := []schemarunpack.IntentRecord{
		{
			SchemaID:        "gait.runpack.intent",
			SchemaVersion:   "1.0.0",
			CreatedAt:       run.CreatedAt,
			ProducerVersion: run.ProducerVersion,
			RunID:           run.RunID,
			IntentID:        "intent_1",
			ToolName:        "tool.demo",
			ArgsDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
			Args:            map[string]any{"foo": "bar"},
		},
	}
	results := []schemarunpack.ResultRecord{
		{
			SchemaID:        "gait.runpack.result",
			SchemaVersion:   "1.0.0",
			CreatedAt:       run.CreatedAt,
			ProducerVersion: run.ProducerVersion,
			RunID:           run.RunID,
			IntentID:        "intent_1",
			Status:          "ok",
			ResultDigest:    "3333333333333333333333333333333333333333333333333333333333333333",
			Result:          map[string]any{"ok": true},
		},
	}
	refs := schemarunpack.Refs{
		SchemaID:        "gait.runpack.refs",
		SchemaVersion:   "1.0.0",
		CreatedAt:       run.CreatedAt,
		ProducerVersion: run.ProducerVersion,
		RunID:           run.RunID,
		Receipts: []schemarunpack.RefReceipt{
			{
				RefID:         "ref_1",
				SourceType:    "web",
				SourceLocator: "example",
				QueryDigest:   "4444444444444444444444444444444444444444444444444444444444444444",
				ContentDigest: "5555555555555555555555555555555555555555555555555555555555555555",
				RetrievedAt:   run.CreatedAt,
				RedactionMode: "reference",
			},
		},
	}

	result, err := RecordRun(RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "",
		SignKey:     keyPair.Private,
	})
	if err != nil {
		test.Fatalf("record run: %v", err)
	}
	if result.Manifest.CaptureMode != "reference" {
		test.Fatalf("expected default capture_mode reference")
	}
	if len(result.Manifest.Signatures) != 1 {
		test.Fatalf("expected manifest signature")
	}

	files := readZipFiles(test, result.ZipBytes)
	validateSchemaFiles(test, files)

	zipPath := writeTempZip(test, result.ZipBytes)
	verifyResult, err := VerifyZip(zipPath, VerifyOptions{
		PublicKey:        keyPair.Public,
		RequireSignature: true,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if verifyResult.SignatureStatus != "verified" {
		test.Fatalf("expected verified signature")
	}
}

func TestRecordRunMissingRunID(test *testing.T) {
	_, err := RecordRun(RecordOptions{
		Run: schemarunpack.Run{},
	})
	if err == nil {
		test.Fatalf("expected error for missing run_id")
	}
}

func TestRecordRunDeterministicZip(test *testing.T) {
	ts := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	run := schemarunpack.Run{
		RunID:           "run_deterministic",
		CreatedAt:       ts,
		ProducerVersion: "0.0.0-dev",
		Env:             schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline:        []schemarunpack.TimelineEvt{{Event: "start", TS: ts}},
	}
	intents := []schemarunpack.IntentRecord{
		{
			IntentID:   "intent_1",
			ToolName:   "tool.demo",
			ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
			Args:       map[string]any{"foo": "bar"},
		},
	}
	results := []schemarunpack.ResultRecord{
		{
			IntentID:     "intent_1",
			Status:       "ok",
			ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
			Result:       map[string]any{"ok": true},
		},
	}
	refs := schemarunpack.Refs{
		RunID: run.RunID,
		Receipts: []schemarunpack.RefReceipt{
			{
				RefID:         "ref_1",
				SourceType:    "demo",
				SourceLocator: "example",
				QueryDigest:   "4444444444444444444444444444444444444444444444444444444444444444",
				ContentDigest: "5555555555555555555555555555555555555555555555555555555555555555",
				RetrievedAt:   ts,
				RedactionMode: "reference",
			},
		},
	}

	first, err := RecordRun(RecordOptions{Run: run, Intents: intents, Results: results, Refs: refs})
	if err != nil {
		test.Fatalf("record run: %v", err)
	}
	second, err := RecordRun(RecordOptions{Run: run, Intents: intents, Results: results, Refs: refs})
	if err != nil {
		test.Fatalf("record run: %v", err)
	}
	if !bytes.Equal(first.ZipBytes, second.ZipBytes) {
		test.Fatalf("expected deterministic zip bytes")
	}
}

func TestRecordRunDifferentInputManifestDigest(test *testing.T) {
	ts := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	runA := schemarunpack.Run{
		RunID:           "run_a",
		CreatedAt:       ts,
		ProducerVersion: "0.0.0-dev",
		Env:             schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline:        []schemarunpack.TimelineEvt{{Event: "start", TS: ts}},
	}
	runB := runA
	runB.RunID = "run_b"
	resultA, err := RecordRun(RecordOptions{Run: runA})
	if err != nil {
		test.Fatalf("record run A: %v", err)
	}
	resultB, err := RecordRun(RecordOptions{Run: runB})
	if err != nil {
		test.Fatalf("record run B: %v", err)
	}
	if resultA.Manifest.ManifestDigest == resultB.Manifest.ManifestDigest {
		test.Fatalf("expected different manifest digest for different inputs")
	}
}

func TestRecordRunLargeInputStress(test *testing.T) {
	ts := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	run := schemarunpack.Run{
		RunID:           "run_large_stress",
		CreatedAt:       ts,
		ProducerVersion: "0.0.0-test",
		Env:             schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline:        []schemarunpack.TimelineEvt{{Event: "start", TS: ts}},
	}

	const recordCount = 1500
	intents := make([]schemarunpack.IntentRecord, 0, recordCount)
	results := make([]schemarunpack.ResultRecord, 0, recordCount)
	for index := 0; index < recordCount; index++ {
		intentID := fmt.Sprintf("intent_%d", index)
		intents = append(intents, schemarunpack.IntentRecord{
			IntentID:   intentID,
			ToolName:   "tool.write",
			ArgsDigest: fmt.Sprintf("%064x", index+1),
			Args:       map[string]any{"path": fmt.Sprintf("/tmp/%d.txt", index), "content": "x"},
		})
		results = append(results, schemarunpack.ResultRecord{
			IntentID:     intentID,
			Status:       "ok",
			ResultDigest: fmt.Sprintf("%064x", index+4000),
			Result:       map[string]any{"ok": true},
		})
	}

	result, err := RecordRun(RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		CaptureMode: "reference",
	})
	if err != nil {
		test.Fatalf("record run large input: %v", err)
	}
	if len(result.ZipBytes) == 0 {
		test.Fatalf("expected non-empty runpack zip")
	}
	if len(result.ZipBytes) > 32*1024*1024 {
		test.Fatalf("expected stress runpack zip <= 32MiB, got %d bytes", len(result.ZipBytes))
	}

	zipPath := writeTempZip(test, result.ZipBytes)
	verifyResult, err := VerifyZip(zipPath, VerifyOptions{})
	if err != nil {
		test.Fatalf("verify large runpack zip: %v", err)
	}
	if len(verifyResult.HashMismatches) > 0 || len(verifyResult.MissingFiles) > 0 {
		test.Fatalf("expected no verification mismatches for large runpack: %#v", verifyResult)
	}
}

func TestRecordRunUnsigned(test *testing.T) {
	run := schemarunpack.Run{
		CreatedAt: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		RunID:     "run_unsigned",
		Env: schemarunpack.RunEnv{
			OS:      "linux",
			Arch:    "amd64",
			Runtime: "go",
		},
		Timeline: []schemarunpack.TimelineEvt{{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)}},
	}
	result, err := RecordRun(RecordOptions{Run: run})
	if err != nil {
		test.Fatalf("record run: %v", err)
	}
	if len(result.Manifest.Signatures) != 0 {
		test.Fatalf("expected no signatures")
	}
}

func TestRecordRunDefaults(test *testing.T) {
	run := schemarunpack.Run{
		RunID: "run_defaults",
		Env:   schemarunpack.RunEnv{},
	}
	intents := []schemarunpack.IntentRecord{
		{
			IntentID:   "intent_1",
			ToolName:   "tool.demo",
			ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
		},
	}
	results := []schemarunpack.ResultRecord{
		{
			IntentID:     "intent_1",
			Status:       "ok",
			ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
		},
	}
	result, err := RecordRun(RecordOptions{
		Run:     run,
		Intents: intents,
		Results: results,
	})
	if err != nil {
		test.Fatalf("record run: %v", err)
	}
	files := readZipFiles(test, result.ZipBytes)
	var decoded schemarunpack.Run
	if err := json.Unmarshal(files["run.json"], &decoded); err != nil {
		test.Fatalf("unmarshal run: %v", err)
	}
	if decoded.SchemaID != "gait.runpack.run" || decoded.SchemaVersion != "1.0.0" {
		test.Fatalf("run defaults not applied")
	}
	if decoded.Env.OS == "" || decoded.Env.Arch == "" || decoded.Env.Runtime == "" {
		test.Fatalf("env defaults not applied")
	}
}

func TestRecordRunInvalidIntent(test *testing.T) {
	run := schemarunpack.Run{
		RunID: "run_invalid",
		Env:   schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	intents := []schemarunpack.IntentRecord{
		{
			IntentID:   "intent_bad",
			ToolName:   "tool.demo",
			ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
			Args:       map[string]any{"bad": make(chan int)},
		},
	}
	if _, err := RecordRun(RecordOptions{Run: run, Intents: intents}); err == nil {
		test.Fatalf("expected error for invalid intent args")
	}
}

func TestWriteRunpack(test *testing.T) {
	run := schemarunpack.Run{
		RunID: "run_write",
		Env:   schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	path := filepath.Join(test.TempDir(), "runpack_write.zip")
	result, err := WriteRunpack(path, RecordOptions{Run: run})
	if err != nil {
		test.Fatalf("write runpack: %v", err)
	}
	if result.RunID != "run_write" {
		test.Fatalf("unexpected run_id")
	}
	info, err := os.Stat(path)
	if err != nil {
		test.Fatalf("stat runpack: %v", err)
	}
	if info.Size() == 0 {
		test.Fatalf("expected non-empty zip file")
	}
}

func TestWriteRunpackRelativePath(test *testing.T) {
	workDir := test.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		test.Fatalf("getwd: %v", err)
	}
	test.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(workDir); err != nil {
		test.Fatalf("chdir: %v", err)
	}

	run := schemarunpack.Run{
		RunID: "run_write_relative",
		Env:   schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	relativePath := filepath.Join("nested", "runpack_write_relative.zip")
	result, err := WriteRunpack(relativePath, RecordOptions{Run: run})
	if err != nil {
		test.Fatalf("write relative runpack: %v", err)
	}
	if result.RunID != "run_write_relative" {
		test.Fatalf("unexpected run_id")
	}
	absolutePath := filepath.Join(workDir, relativePath)
	info, err := os.Stat(absolutePath)
	if err != nil {
		test.Fatalf("stat relative runpack: %v", err)
	}
	if info.Size() == 0 {
		test.Fatalf("expected non-empty relative zip file")
	}
}

func TestWriteRunpackCreateDirectoryError(test *testing.T) {
	workDir := test.TempDir()
	blockerPath := filepath.Join(workDir, "nested")
	if err := os.WriteFile(blockerPath, []byte("blocker\n"), 0o600); err != nil {
		test.Fatalf("write blocker: %v", err)
	}

	run := schemarunpack.Run{
		RunID: "run_write_mkdir_error",
		Env:   schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	if _, err := WriteRunpack(filepath.Join(blockerPath, "runpack.zip"), RecordOptions{Run: run}); err == nil {
		test.Fatalf("expected create directory error")
	}
}

func TestWriteRunpackCreateDirectoryErrorRelative(test *testing.T) {
	workDir := test.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		test.Fatalf("getwd: %v", err)
	}
	test.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(workDir); err != nil {
		test.Fatalf("chdir: %v", err)
	}
	if err := os.WriteFile("nested", []byte("blocker\n"), 0o600); err != nil {
		test.Fatalf("write blocker: %v", err)
	}

	run := schemarunpack.Run{
		RunID: "run_write_mkdir_error_relative",
		Env:   schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	if _, err := WriteRunpack(filepath.Join("nested", "runpack.zip"), RecordOptions{Run: run}); err == nil {
		test.Fatalf("expected create directory error for relative path")
	}
}

func TestWriteRunpackWriteFileError(test *testing.T) {
	workDir := test.TempDir()
	targetPath := filepath.Join(workDir, "existing-dir")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		test.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetPath, "keep.txt"), []byte("keep\n"), 0o600); err != nil {
		test.Fatalf("write target sentinel: %v", err)
	}

	run := schemarunpack.Run{
		RunID: "run_write_file_error",
		Env:   schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	if _, err := WriteRunpack(targetPath, RecordOptions{Run: run}); err == nil {
		test.Fatalf("expected write file error for directory destination")
	}
}

func TestWriteRunpackRecordError(test *testing.T) {
	if _, err := WriteRunpack(filepath.Join(test.TempDir(), "bad-runpack.zip"), RecordOptions{Run: schemarunpack.Run{}}); err == nil {
		test.Fatalf("expected record validation error")
	}
}

func TestWriteRunpackRejectsParentTraversal(test *testing.T) {
	run := schemarunpack.Run{
		RunID: "run_write_invalid_path",
		Env:   schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	if _, err := WriteRunpack("../runpack.zip", RecordOptions{Run: run}); err == nil {
		test.Fatalf("expected relative parent traversal path to fail")
	}
}

func TestNormalizeOutputPath(test *testing.T) {
	absoluteInput := filepath.Join(test.TempDir(), "nested", "runpack.zip")
	absolutePath, err := normalizeOutputPath(absoluteInput)
	if err != nil {
		test.Fatalf("normalize absolute path: %v", err)
	}
	if absolutePath != filepath.Clean(absoluteInput) {
		test.Fatalf("unexpected absolute path: %s", absolutePath)
	}

	relativePath, err := normalizeOutputPath("./gait-out/runpack.zip")
	if err != nil {
		test.Fatalf("normalize relative path: %v", err)
	}
	if relativePath != filepath.Clean("./gait-out/runpack.zip") {
		test.Fatalf("unexpected relative path: %s", relativePath)
	}

	if _, err := normalizeOutputPath(""); err == nil {
		test.Fatalf("expected empty output path to fail")
	}
	if _, err := normalizeOutputPath("../gait-out/runpack.zip"); err == nil {
		test.Fatalf("expected parent traversal output path to fail")
	}
	if _, err := normalizeOutputPath("."); err == nil {
		test.Fatalf("expected dot output path to fail")
	}
}

func validateSchemaFiles(test *testing.T, files map[string][]byte) {
	test.Helper()
	root := repoRoot(test)
	schemaDir := filepath.Join(root, "schemas", "v1", "runpack")

	mustValidateJSON(test, filepath.Join(schemaDir, "manifest.schema.json"), files["manifest.json"])
	mustValidateJSON(test, filepath.Join(schemaDir, "run.schema.json"), files["run.json"])
	mustValidateJSONL(test, filepath.Join(schemaDir, "intent.schema.json"), files["intents.jsonl"])
	mustValidateJSONL(test, filepath.Join(schemaDir, "result.schema.json"), files["results.jsonl"])
	mustValidateJSON(test, filepath.Join(schemaDir, "refs.schema.json"), files["refs.json"])
}

func mustValidateJSON(test *testing.T, schemaPath string, data []byte) {
	test.Helper()
	if err := validate.ValidateJSON(schemaPath, data); err != nil {
		test.Fatalf("validate json: %v", err)
	}
}

func mustValidateJSONL(test *testing.T, schemaPath string, data []byte) {
	test.Helper()
	if err := validate.ValidateJSONL(schemaPath, data); err != nil {
		test.Fatalf("validate jsonl: %v", err)
	}
}

func readZipFiles(test *testing.T, zipBytes []byte) map[string][]byte {
	test.Helper()
	reader := bytes.NewReader(zipBytes)
	zipReader, err := zip.NewReader(reader, int64(len(zipBytes)))
	if err != nil {
		test.Fatalf("open zip: %v", err)
	}
	files := make(map[string][]byte, len(zipReader.File))
	for _, zipFile := range zipReader.File {
		rc, err := zipFile.Open()
		if err != nil {
			test.Fatalf("open zip entry: %v", err)
		}
		data, err := io.ReadAll(rc)
		if closeErr := rc.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			test.Fatalf("read zip entry: %v", err)
		}
		files[zipFile.Name] = data
	}
	return files
}

func writeTempZip(test *testing.T, zipBytes []byte) string {
	test.Helper()
	path := filepath.Join(test.TempDir(), "runpack.zip")
	if err := os.WriteFile(path, zipBytes, 0o600); err != nil {
		test.Fatalf("write zip: %v", err)
	}
	return path
}

func repoRoot(test *testing.T) string {
	test.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		test.Fatalf("unable to locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
