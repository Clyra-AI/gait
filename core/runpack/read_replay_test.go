package runpack

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/zipx"
)

func TestReadRunpackSuccess(t *testing.T) {
	path := writeTestRunpack(t, "run_read", buildIntents("intent_1"), buildResults("intent_1"))

	pack, err := ReadRunpack(path)
	if err != nil {
		t.Fatalf("read runpack: %v", err)
	}
	if pack.Run.RunID != "run_read" {
		t.Fatalf("expected run_id")
	}
	if len(pack.Intents) != 1 || len(pack.Results) != 1 {
		t.Fatalf("expected intents and results")
	}
}

func TestReadRunpackMissingFile(t *testing.T) {
	manifest := schemarunpack.Manifest{
		SchemaID:        "gait.runpack.manifest",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		RunID:           "run_missing",
		CaptureMode:     "reference",
		Files: []schemarunpack.ManifestFile{
			{Path: "run.json", SHA256: "1111111111111111111111111111111111111111111111111111111111111111"},
		},
		ManifestDigest: "2222222222222222222222222222222222222222222222222222222222222222",
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var buf bytes.Buffer
	if err := zipx.WriteDeterministicZip(&buf, []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	path := filepath.Join(t.TempDir(), "runpack_missing.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if _, err := ReadRunpack(path); err == nil {
		t.Fatalf("expected error for missing files")
	}
}

func TestReplayStubSuccess(t *testing.T) {
	path := writeTestRunpack(t, "run_replay", buildIntents("intent_1", "intent_2"), buildResults("intent_1", "intent_2"))

	result, err := ReplayStub(path)
	if err != nil {
		t.Fatalf("replay stub: %v", err)
	}
	if result.RunID != "run_replay" {
		t.Fatalf("unexpected run_id")
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps")
	}
	if len(result.MissingResults) != 0 {
		t.Fatalf("expected no missing results")
	}
}

func TestReplayStubMissingResult(t *testing.T) {
	path := writeTestRunpack(t, "run_missing_result", buildIntents("intent_1"), nil)

	result, err := ReplayStub(path)
	if err != nil {
		t.Fatalf("replay stub: %v", err)
	}
	if len(result.MissingResults) != 1 {
		t.Fatalf("expected missing results")
	}
	if result.Steps[0].Status != "missing_result" {
		t.Fatalf("expected missing_result status")
	}
}

func TestReplayStubDuplicateIntent(t *testing.T) {
	intents := []schemarunpack.IntentRecord{
		buildIntent("intent_dup"),
		buildIntent("intent_dup"),
	}
	path := writeTestRunpackWithIntents(t, "run_dup_intent", intents, buildResults("intent_dup"))
	if _, err := ReplayStub(path); err == nil {
		t.Fatalf("expected duplicate intent error")
	}
}

func TestReplayStubDuplicateResult(t *testing.T) {
	results := []schemarunpack.ResultRecord{
		buildResult("intent_dup"),
		buildResult("intent_dup"),
	}
	path := writeTestRunpackWithResults(t, "run_dup_result", buildIntents("intent_dup"), results)
	if _, err := ReplayStub(path); err == nil {
		t.Fatalf("expected duplicate result error")
	}
}

func writeTestRunpack(t *testing.T, runID string, intents []schemarunpack.IntentRecord, results []schemarunpack.ResultRecord) string {
	return writeTestRunpackWithIntents(t, runID, intents, results)
}

func writeTestRunpackWithIntents(t *testing.T, runID string, intents []schemarunpack.IntentRecord, results []schemarunpack.ResultRecord) string {
	run := schemarunpack.Run{
		RunID:     runID,
		CreatedAt: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		Env:       schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	path := filepath.Join(t.TempDir(), "runpack.zip")
	_, err := WriteRunpack(path, RecordOptions{
		Run:     run,
		Intents: intents,
		Results: results,
		Refs: schemarunpack.Refs{
			RunID: runID,
		},
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}
	return path
}

func writeTestRunpackWithResults(t *testing.T, runID string, intents []schemarunpack.IntentRecord, results []schemarunpack.ResultRecord) string {
	return writeTestRunpackWithIntents(t, runID, intents, results)
}

func buildIntents(intentIDs ...string) []schemarunpack.IntentRecord {
	intents := make([]schemarunpack.IntentRecord, len(intentIDs))
	for i, id := range intentIDs {
		intents[i] = buildIntent(id)
	}
	return intents
}

func buildResults(intentIDs ...string) []schemarunpack.ResultRecord {
	results := make([]schemarunpack.ResultRecord, len(intentIDs))
	for i, id := range intentIDs {
		results[i] = buildResult(id)
	}
	return results
}

func buildIntent(intentID string) schemarunpack.IntentRecord {
	return schemarunpack.IntentRecord{
		IntentID:   intentID,
		ToolName:   "tool.demo",
		ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
		Args:       map[string]any{"foo": "bar"},
	}
}

func buildResult(intentID string) schemarunpack.ResultRecord {
	return schemarunpack.ResultRecord{
		IntentID:     intentID,
		Status:       "ok",
		ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
		Result:       map[string]any{"ok": true},
	}
}
