package regress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
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

func TestWriteJSONFailsForDirectoryPath(t *testing.T) {
	workDir := t.TempDir()
	if err := writeJSON(workDir, map[string]any{"ok": true}); err == nil {
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
