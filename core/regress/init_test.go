package regress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/pack"
	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	schemavoice "github.com/davidahmann/gait/core/schema/v1/voice"
)

func TestInitFixtureCreatesLayout(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	result, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	})
	if err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	if result.RunID != "run_demo" {
		t.Fatalf("expected run_id run_demo, got %s", result.RunID)
	}
	if result.FixtureName != "run_demo" {
		t.Fatalf("expected fixture name run_demo, got %s", result.FixtureName)
	}
	if result.ConfigPath != "gait.yaml" {
		t.Fatalf("expected config path gait.yaml, got %s", result.ConfigPath)
	}
	if len(result.NextCommands) != 1 || result.NextCommands[0] != "gait regress run --json" {
		t.Fatalf("unexpected next commands: %#v", result.NextCommands)
	}

	fixtureRunpackPath := filepath.Join(workDir, result.RunpackPath)
	if _, err := os.Stat(fixtureRunpackPath); err != nil {
		t.Fatalf("expected fixture runpack to exist: %v", err)
	}
	fixtureMetaPath := filepath.Join(workDir, result.FixtureDir, "fixture.json")
	if _, err := os.Stat(fixtureMetaPath); err != nil {
		t.Fatalf("expected fixture metadata to exist: %v", err)
	}
	meta := mustReadFixtureMetaFromInit(t, fixtureMetaPath)
	if len(meta.ExpectedToolSequence) != 1 || meta.ExpectedToolSequence[0] != "tool.demo" {
		t.Fatalf("unexpected expected_tool_sequence defaults: %#v", meta.ExpectedToolSequence)
	}
	if len(meta.ExpectedVerdictSequence) != 1 || meta.ExpectedVerdictSequence[0] != "allow" {
		t.Fatalf("unexpected expected_verdict_sequence defaults: %#v", meta.ExpectedVerdictSequence)
	}

	rawConfig, err := os.ReadFile(filepath.Join(workDir, "gait.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg configFile
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.SchemaID != configSchemaID {
		t.Fatalf("unexpected config schema_id: %s", cfg.SchemaID)
	}
	if len(cfg.Fixtures) != 1 {
		t.Fatalf("expected one fixture in config, got %d", len(cfg.Fixtures))
	}
	if cfg.Fixtures[0].Name != "run_demo" || cfg.Fixtures[0].Runpack != "fixtures/run_demo/runpack.zip" {
		t.Fatalf("unexpected fixture entry: %#v", cfg.Fixtures[0])
	}
}

func TestInitFixtureSortsConfigFixtures(t *testing.T) {
	workDir := t.TempDir()
	sourceB := createRunpack(t, workDir, "run_b")
	sourceA := createRunpack(t, workDir, "run_a")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceB,
		FixtureName:       "zeta",
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init zeta fixture: %v", err)
	}
	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceA,
		FixtureName:       "alpha",
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init alpha fixture: %v", err)
	}

	rawConfig, err := os.ReadFile(filepath.Join(workDir, "gait.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg configFile
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if len(cfg.Fixtures) != 2 {
		t.Fatalf("expected two fixtures in config, got %d", len(cfg.Fixtures))
	}
	if cfg.Fixtures[0].Name != "alpha" || cfg.Fixtures[1].Name != "zeta" {
		t.Fatalf("fixtures not sorted by name: %#v", cfg.Fixtures)
	}
}

func TestInitFixtureInvalidSource(t *testing.T) {
	workDir := t.TempDir()
	invalidSource := filepath.Join(workDir, "not-a-runpack.zip")
	if err := os.WriteFile(invalidSource, []byte("bad"), 0o600); err != nil {
		t.Fatalf("write invalid source: %v", err)
	}

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: invalidSource,
		WorkDir:           workDir,
	}); err == nil {
		t.Fatalf("expected invalid source runpack to fail")
	}
}

func TestInitFixtureRejectsInvalidFixtureName(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		FixtureName:       "../bad",
		WorkDir:           workDir,
	}); err == nil {
		t.Fatalf("expected invalid fixture name to fail")
	}
}

func TestInitFixtureAcceptsPackRunArtifactSource(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	runPackPath := filepath.Join(workDir, "pack_run.zip")
	if _, err := pack.BuildRunPack(pack.BuildRunOptions{
		RunpackPath: sourceRunpack,
		OutputPath:  runPackPath,
	}); err != nil {
		t.Fatalf("build run pack: %v", err)
	}

	result, err := InitFixture(InitOptions{
		SourceRunpackPath: runPackPath,
		WorkDir:           workDir,
	})
	if err != nil {
		t.Fatalf("init fixture from pack run artifact: %v", err)
	}
	if result.RunID != "run_demo" {
		t.Fatalf("expected run_id run_demo, got %s", result.RunID)
	}
	if _, err := os.Stat(filepath.Join(workDir, result.RunpackPath)); err != nil {
		t.Fatalf("expected materialized fixture runpack: %v", err)
	}
}

func TestInitFixtureAcceptsCallPackArtifactSource(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")
	callRecordPath := filepath.Join(workDir, "call_record.json")
	writeCallRecordForRegress(t, callRecordPath, sourceRunpack, "call_regress_demo")

	callPackPath := filepath.Join(workDir, "pack_call.zip")
	if _, err := pack.BuildCallPack(pack.BuildCallOptions{
		CallRecordPath: callRecordPath,
		OutputPath:     callPackPath,
	}); err != nil {
		t.Fatalf("build call pack: %v", err)
	}

	result, err := InitFixture(InitOptions{
		SourceRunpackPath: callPackPath,
		WorkDir:           workDir,
	})
	if err != nil {
		t.Fatalf("init fixture from call pack artifact: %v", err)
	}
	if result.RunID != "run_demo" {
		t.Fatalf("expected run_id run_demo, got %s", result.RunID)
	}
	if _, err := os.Stat(filepath.Join(workDir, result.RunpackPath)); err != nil {
		t.Fatalf("expected materialized fixture runpack: %v", err)
	}
}

func TestLoadFixtureEntriesRejectsMetadataMismatch(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	metaPath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	tampered := fixtureMeta{
		SchemaID:      fixtureSchemaID,
		SchemaVersion: fixtureSchemaV1,
		Name:          "different",
		RunID:         "run_demo",
		Runpack:       "runpack.zip",
	}
	raw, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("marshal tampered metadata: %v", err)
	}
	if err := os.WriteFile(metaPath, raw, 0o600); err != nil {
		t.Fatalf("write tampered metadata: %v", err)
	}

	if _, err := loadFixtureEntries(filepath.Join(workDir, "fixtures")); err == nil {
		t.Fatalf("expected metadata mismatch to fail")
	}
}

func TestSanitizeFixtureName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "run_demo", expected: "run_demo"},
		{input: "RUN/DEMO", expected: "run-demo"},
		{input: "  ", expected: "fixture"},
		{input: "run:alpha", expected: "run-alpha"},
	}

	for _, testCase := range tests {
		actual := sanitizeFixtureName(testCase.input)
		if actual != testCase.expected {
			t.Fatalf("sanitize fixture name mismatch for %q: expected %q got %q", testCase.input, testCase.expected, actual)
		}
	}
}

func TestInitFixtureRequiresSourcePath(t *testing.T) {
	if _, err := InitFixture(InitOptions{}); err == nil {
		t.Fatalf("expected missing source path to fail")
	}
}

func TestLoadFixtureEntriesRejectsNestedRunpackPath(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	metaPath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	meta := mustReadFixtureMetaFromInit(t, metaPath)
	meta.Runpack = "nested/runpack.zip"
	if err := writeJSON(metaPath, meta); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}

	if _, err := loadFixtureEntries(filepath.Join(workDir, "fixtures")); err == nil {
		t.Fatalf("expected nested runpack path to fail")
	}
}

func TestLoadFixtureEntriesSkipsNonRegressFixtureDirectories(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	externalFixtureDir := filepath.Join(workDir, "fixtures", "packspec_tck", "v1")
	if err := os.MkdirAll(externalFixtureDir, 0o750); err != nil {
		t.Fatalf("mkdir external fixture dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(externalFixtureDir, "run_record_input.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write external fixture file: %v", err)
	}

	entries, err := loadFixtureEntries(filepath.Join(workDir, "fixtures"))
	if err != nil {
		t.Fatalf("load fixture entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one regress fixture entry, got %d", len(entries))
	}
	if entries[0].Name != "run_demo" {
		t.Fatalf("unexpected fixture entry: %#v", entries[0])
	}
}

func TestLoadFixtureEntriesRejectsMissingMetadataForRunpackDirectory(t *testing.T) {
	workDir := t.TempDir()
	fixtureDir := filepath.Join(workDir, "fixtures", "broken")
	if err := os.MkdirAll(fixtureDir, 0o750); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "runpack.zip"), []byte("not-a-real-runpack"), 0o600); err != nil {
		t.Fatalf("write runpack placeholder: %v", err)
	}

	if _, err := loadFixtureEntries(filepath.Join(workDir, "fixtures")); err == nil {
		t.Fatalf("expected missing fixture metadata to fail")
	}
}

func TestReadFixtureMetaRejectsNegativeExpectedExitCode(t *testing.T) {
	workDir := t.TempDir()
	metaPath := filepath.Join(workDir, "fixture.json")
	meta := fixtureMeta{
		SchemaID:               fixtureSchemaID,
		SchemaVersion:          fixtureSchemaV1,
		Name:                   "demo",
		RunID:                  "run_demo",
		Runpack:                "runpack.zip",
		ExpectedReplayExitCode: -1,
	}
	if err := writeJSON(metaPath, meta); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}
	if _, err := readFixtureMeta(metaPath); err == nil {
		t.Fatalf("expected negative expected_replay_exit_code to fail")
	}
}

func TestCopyRunpackMissingSource(t *testing.T) {
	workDir := t.TempDir()
	dest := filepath.Join(workDir, "runpack.zip")
	if err := copyRunpack(filepath.Join(workDir, "missing.zip"), dest); err == nil {
		t.Fatalf("expected missing source copy to fail")
	}
}

func TestMaterializeRunpackSourceBranches(t *testing.T) {
	workDir := t.TempDir()
	if _, _, err := materializeRunpackSource(" "); err == nil {
		t.Fatalf("expected missing source path error")
	}

	runpackPath := createRunpack(t, workDir, "run_materialize")
	materializedPath, cleanup, err := materializeRunpackSource(runpackPath)
	if err != nil {
		t.Fatalf("materialize legacy runpack source: %v", err)
	}
	if materializedPath != runpackPath {
		t.Fatalf("expected legacy runpack source path passthrough")
	}
	cleanup()

	runPackPath := filepath.Join(workDir, "pack_run_materialize.zip")
	if _, err := pack.BuildRunPack(pack.BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  runPackPath,
	}); err != nil {
		t.Fatalf("build run pack source: %v", err)
	}
	extractedPath, cleanupExtracted, err := materializeRunpackSource(runPackPath)
	if err != nil {
		t.Fatalf("materialize run pack source: %v", err)
	}
	if extractedPath == runPackPath {
		t.Fatalf("expected run pack source to materialize to temp runpack path")
	}
	if _, err := os.Stat(extractedPath); err != nil {
		t.Fatalf("expected materialized runpack path to exist: %v", err)
	}
	cleanupExtracted()
	if _, err := os.Stat(extractedPath); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup to remove materialized runpack path")
	}

	invalidPath := filepath.Join(workDir, "invalid_source.zip")
	if err := os.WriteFile(invalidPath, []byte("not-a-zip"), 0o600); err != nil {
		t.Fatalf("write invalid source: %v", err)
	}
	if _, _, err := materializeRunpackSource(invalidPath); err == nil {
		t.Fatalf("expected invalid source materialization to fail")
	}
}

func TestReadFixtureMetaRejectsNegativeCheckpointIndex(t *testing.T) {
	workDir := t.TempDir()
	metaPath := filepath.Join(workDir, "fixture.json")
	meta := fixtureMeta{
		SchemaID:               fixtureSchemaID,
		SchemaVersion:          fixtureSchemaV1,
		Name:                   "demo",
		RunID:                  "run_demo",
		Runpack:                "runpack.zip",
		ExpectedReplayExitCode: 0,
		CheckpointIndex:        -1,
	}
	if err := writeJSON(metaPath, meta); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}
	if _, err := readFixtureMeta(metaPath); err == nil {
		t.Fatalf("expected negative checkpoint_index to fail")
	}
}

func TestReadFixtureMetaRejectsInvalidExpectedVerdictSequence(t *testing.T) {
	workDir := t.TempDir()
	metaPath := filepath.Join(workDir, "fixture.json")
	meta := fixtureMeta{
		SchemaID:                fixtureSchemaID,
		SchemaVersion:           fixtureSchemaV1,
		Name:                    "demo",
		RunID:                   "run_demo",
		Runpack:                 "runpack.zip",
		ExpectedReplayExitCode:  0,
		ExpectedToolSequence:    []string{"tool.demo"},
		ExpectedVerdictSequence: []string{"not_a_verdict"},
	}
	if err := writeJSON(metaPath, meta); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}
	if _, err := readFixtureMeta(metaPath); err == nil {
		t.Fatalf("expected invalid expected_verdict_sequence to fail")
	}
}

func TestWriteJSONFailsForDirectoryPath(t *testing.T) {
	workDir := t.TempDir()
	directoryPath := filepath.Join(workDir, "write-target")
	if err := os.MkdirAll(directoryPath, 0o750); err != nil {
		t.Fatalf("mkdir write target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directoryPath, "keep.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write keep file: %v", err)
	}
	if err := writeJSON(directoryPath, map[string]any{"ok": true}); err == nil {
		t.Fatalf("expected write json to directory path to fail")
	}
}

func mustReadFixtureMetaFromInit(t *testing.T, path string) fixtureMeta {
	t.Helper()
	meta, err := readFixtureMeta(path)
	if err != nil {
		t.Fatalf("read fixture metadata: %v", err)
	}
	return meta
}

func createRunpack(t *testing.T, dir, runID string) string {
	t.Helper()
	path := filepath.Join(dir, runID+".zip")
	ts := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	run := schemarunpack.Run{
		RunID:     runID,
		CreatedAt: ts,
		Env:       schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: ts},
		},
	}

	_, err := runpack.WriteRunpack(path, runpack.RecordOptions{
		Run: run,
		Intents: []schemarunpack.IntentRecord{
			{
				IntentID:   "intent_1",
				ToolName:   "tool.demo",
				ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
				Args:       map[string]any{"input": "demo"},
			},
		},
		Results: []schemarunpack.ResultRecord{
			{
				IntentID:     "intent_1",
				Status:       "ok",
				ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
				Result:       map[string]any{"ok": true},
			},
		},
		Refs: schemarunpack.Refs{
			RunID: runID,
		},
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}
	return path
}

func writeCallRecordForRegress(t *testing.T, path string, runpackPath string, callID string) {
	t.Helper()
	createdAt := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	record := map[string]any{
		"schema_id":        "gait.voice.call_record",
		"schema_version":   "1.0.0",
		"created_at":       createdAt.Format(time.RFC3339Nano),
		"producer_version": "test",
		"call_id":          callID,
		"runpack_path":     runpackPath,
		"privacy_mode":     "hash_only",
		"events": []map[string]any{
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Format(time.RFC3339Nano), "call_id": callID, "call_seq": 1, "turn_index": 1, "event_type": "asr.final", "payload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(1 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 2, "turn_index": 2, "event_type": "commitment.declared", "commitment_class": "quote"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(2 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 3, "turn_index": 2, "event_type": "gate.decision", "commitment_class": "quote"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(3 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 4, "turn_index": 2, "event_type": "tts.request", "commitment_class": "quote"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(4 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 5, "turn_index": 2, "event_type": "tts.emitted", "commitment_class": "quote", "say_token_id": "say_demo"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(5 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 6, "turn_index": 2, "event_type": "tool.intent"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(6 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 7, "turn_index": 2, "event_type": "tool.result"},
		},
		"commitments": []schemavoice.CommitmentIntent{
			{
				SchemaID:        "gait.voice.commitment_intent",
				SchemaVersion:   "1.0.0",
				CreatedAt:       createdAt.Add(1 * time.Second),
				ProducerVersion: "test",
				CallID:          callID,
				TurnIndex:       2,
				CallSeq:         2,
				CommitmentClass: "quote",
				Context: schemavoice.CommitmentContext{
					Identity:  "agent.voice",
					Workspace: "/srv/voice",
					RiskClass: "high",
				},
				QuoteMinCents: 1000,
				QuoteMaxCents: 1200,
			},
		},
		"gate_decisions": []schemavoice.GateDecision{
			{
				CallID:          callID,
				CallSeq:         3,
				TurnIndex:       2,
				CommitmentClass: "quote",
				Verdict:         "allow",
			},
		},
		"speak_receipts": []schemavoice.SpeakReceipt{
			{
				CallID:          callID,
				CallSeq:         5,
				TurnIndex:       2,
				CommitmentClass: "quote",
				SayTokenID:      "say_demo",
				SpokenDigest:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				EmittedAt:       createdAt.Add(4 * time.Second),
			},
		},
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal call record: %v", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write call record: %v", err)
	}
}
