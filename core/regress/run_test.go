package regress

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/runpack"
	schemaregress "github.com/davidahmann/gait/core/schema/v1/regress"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

func TestRunPassesWithDefaultFixture(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	outputPath := filepath.Join(workDir, "regress_result.json")
	result, err := Run(RunOptions{
		ConfigPath:      filepath.Join(workDir, "gait.yaml"),
		OutputPath:      outputPath,
		WorkDir:         workDir,
		ProducerVersion: "test",
	})
	if err != nil {
		t.Fatalf("run regress: %v", err)
	}

	if result.Result.Status != regressStatusPass {
		t.Fatalf("expected pass status, got %s", result.Result.Status)
	}
	if result.FailedGraders != 0 {
		t.Fatalf("expected zero failed graders, got %d", result.FailedGraders)
	}
	if len(result.Result.Graders) != 3 {
		t.Fatalf("expected 3 graders, got %d", len(result.Result.Graders))
	}

	// #nosec G304 -- test controls output path in temp dir.
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read regress output: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("parse regress output: %v", err)
	}
	if decoded["status"] != regressStatusPass {
		t.Fatalf("unexpected output status: %v", decoded["status"])
	}
}

func TestRunFailsOnExpectedExitMismatch(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	metaPath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	meta := mustReadFixtureMeta(t, metaPath)
	meta.ExpectedReplayExitCode = replayMissingExitCode
	if err := writeJSON(metaPath, meta); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}

	result, err := Run(RunOptions{
		ConfigPath: filepath.Join(workDir, "gait.yaml"),
		OutputPath: filepath.Join(workDir, "regress_result.json"),
		WorkDir:    workDir,
	})
	if err != nil {
		t.Fatalf("run regress: %v", err)
	}
	if result.Result.Status != regressStatusFail {
		t.Fatalf("expected fail status, got %s", result.Result.Status)
	}
	if !hasFailedReason(result.Result.Graders, "run_demo/expected_exit_code", "unexpected_exit_code") {
		t.Fatalf("expected unexpected_exit_code failure, got %#v", result.Result.Graders)
	}
}

func TestRunDiffToleranceRules(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	candidatePath := createVariantRunpack(t, workDir, "run_demo", "changed")
	metaPath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	meta := mustReadFixtureMeta(t, metaPath)
	meta.CandidateRunpack = candidatePath
	meta.DiffAllowChangedFiles = []string{}
	if err := writeJSON(metaPath, meta); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}

	firstRun, err := Run(RunOptions{
		ConfigPath: filepath.Join(workDir, "gait.yaml"),
		OutputPath: filepath.Join(workDir, "regress_result.json"),
		WorkDir:    workDir,
	})
	if err != nil {
		t.Fatalf("first regress run: %v", err)
	}
	if firstRun.Result.Status != regressStatusFail {
		t.Fatalf("expected diff failure, got %s", firstRun.Result.Status)
	}
	if !hasFailedReason(firstRun.Result.Graders, "run_demo/diff", "unexpected_diff") {
		t.Fatalf("expected unexpected_diff failure, got %#v", firstRun.Result.Graders)
	}

	diffResult, err := runpack.DiffRunpacks(sourceRunpack, candidatePath, runpack.DiffPrivacy("full"))
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	meta = mustReadFixtureMeta(t, metaPath)
	meta.CandidateRunpack = candidatePath
	meta.DiffAllowChangedFiles = summarizeChangedFiles(diffResult.Summary)
	if err := writeJSON(metaPath, meta); err != nil {
		t.Fatalf("write fixture metadata with tolerances: %v", err)
	}

	secondRun, err := Run(RunOptions{
		ConfigPath: filepath.Join(workDir, "gait.yaml"),
		OutputPath: filepath.Join(workDir, "regress_result.json"),
		WorkDir:    workDir,
	})
	if err != nil {
		t.Fatalf("second regress run: %v", err)
	}
	if secondRun.Result.Status != regressStatusPass {
		t.Fatalf("expected pass with diff tolerances, got %s", secondRun.Result.Status)
	}
}

func TestRunWritesStableJUnitReport(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	firstPath := filepath.Join(workDir, "junit_first.xml")
	secondPath := filepath.Join(workDir, "junit_second.xml")

	if _, err := Run(RunOptions{
		ConfigPath: filepath.Join(workDir, "gait.yaml"),
		OutputPath: filepath.Join(workDir, "regress_result.json"),
		JUnitPath:  firstPath,
		WorkDir:    workDir,
	}); err != nil {
		t.Fatalf("first run: %v", err)
	}

	if _, err := Run(RunOptions{
		ConfigPath: filepath.Join(workDir, "gait.yaml"),
		OutputPath: filepath.Join(workDir, "regress_result.json"),
		JUnitPath:  secondPath,
		WorkDir:    workDir,
	}); err != nil {
		t.Fatalf("second run: %v", err)
	}

	firstBytes := mustReadBytes(t, firstPath)
	secondBytes := mustReadBytes(t, secondPath)
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("expected stable junit output")
	}

	report := mustReadJUnit(t, firstPath)
	if report.Tests != 3 || report.Failures != 0 {
		t.Fatalf("unexpected junit summary: tests=%d failures=%d", report.Tests, report.Failures)
	}
	if len(report.Suites) != 1 || report.Suites[0].Name != "gait.regress.default" {
		t.Fatalf("unexpected junit suite: %#v", report.Suites)
	}
}

func TestRunWritesJUnitFailureDetails(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	if _, err := InitFixture(InitOptions{
		SourceRunpackPath: sourceRunpack,
		WorkDir:           workDir,
	}); err != nil {
		t.Fatalf("init fixture: %v", err)
	}

	metaPath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	meta := mustReadFixtureMeta(t, metaPath)
	meta.ExpectedReplayExitCode = replayMissingExitCode
	if err := writeJSON(metaPath, meta); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}

	junitPath := filepath.Join(workDir, "junit.xml")
	result, err := Run(RunOptions{
		ConfigPath: filepath.Join(workDir, "gait.yaml"),
		OutputPath: filepath.Join(workDir, "regress_result.json"),
		JUnitPath:  junitPath,
		WorkDir:    workDir,
	})
	if err != nil {
		t.Fatalf("run regress: %v", err)
	}
	if result.Result.Status != regressStatusFail {
		t.Fatalf("expected fail status, got %s", result.Result.Status)
	}

	report := mustReadJUnit(t, junitPath)
	if report.Failures != 1 {
		t.Fatalf("expected one junit failure, got %d", report.Failures)
	}
	found := false
	for _, testCase := range report.Suites[0].TestCases {
		if testCase.Name != "expected_exit_code" || testCase.Failure == nil {
			continue
		}
		found = strings.Contains(testCase.Failure.Message, "unexpected_exit_code")
	}
	if !found {
		t.Fatalf("expected failure message with unexpected_exit_code")
	}
}

func TestRunReturnsErrorWhenNoFixturesConfigured(t *testing.T) {
	workDir := t.TempDir()
	configPath := filepath.Join(workDir, "gait.yaml")
	if err := writeJSON(configPath, configFile{
		SchemaID:      configSchemaID,
		SchemaVersion: configSchemaV1,
		FixtureSet:    defaultFixtureSet,
		Fixtures:      []configFixture{},
	}); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Run(RunOptions{
		ConfigPath: configPath,
		OutputPath: filepath.Join(workDir, "regress_result.json"),
		WorkDir:    workDir,
	}); err == nil {
		t.Fatalf("expected run with zero fixtures to fail")
	}
}

func TestReadRegressConfigDefaultsAndSortsFixtures(t *testing.T) {
	workDir := t.TempDir()
	configPath := filepath.Join(workDir, "gait.yaml")
	if err := writeJSON(configPath, configFile{
		SchemaID:      configSchemaID,
		SchemaVersion: configSchemaV1,
		Fixtures: []configFixture{
			{Name: "zeta", Runpack: "fixtures/zeta/runpack.zip"},
			{Name: "alpha", Runpack: "fixtures/alpha/runpack.zip"},
		},
	}); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := readRegressConfig(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.FixtureSet != defaultFixtureSet {
		t.Fatalf("expected default fixture set %q, got %q", defaultFixtureSet, cfg.FixtureSet)
	}
	if len(cfg.Fixtures) != 2 {
		t.Fatalf("expected two fixtures, got %d", len(cfg.Fixtures))
	}
	if cfg.Fixtures[0].Name != "alpha" || cfg.Fixtures[1].Name != "zeta" {
		t.Fatalf("fixtures not sorted by name: %#v", cfg.Fixtures)
	}
}

func TestReadRegressConfigValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "invalid_json",
			content: "{",
		},
		{
			name:    "unsupported_schema_id",
			content: `{"schema_id":"other","schema_version":"1.0.0","fixtures":[]}`,
		},
		{
			name:    "unsupported_schema_version",
			content: `{"schema_id":"gait.regress.config","schema_version":"2.0.0","fixtures":[]}`,
		},
		{
			name:    "fixture_name_required",
			content: `{"schema_id":"gait.regress.config","schema_version":"1.0.0","fixtures":[{"name":" ","runpack":"fixtures/demo/runpack.zip"}]}`,
		},
		{
			name:    "fixture_runpack_required",
			content: `{"schema_id":"gait.regress.config","schema_version":"1.0.0","fixtures":[{"name":"demo","runpack":" "}]}`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			workDir := t.TempDir()
			configPath := filepath.Join(workDir, "gait.yaml")
			if err := os.WriteFile(configPath, []byte(testCase.content), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := readRegressConfig(configPath); err == nil {
				t.Fatalf("expected config validation error")
			}
		})
	}
}

func TestReadRegressConfigMissingFile(t *testing.T) {
	if _, err := readRegressConfig(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatalf("expected missing config file to fail")
	}
}

func TestLoadFixtureSpecsValidationErrors(t *testing.T) {
	workDir := t.TempDir()
	cfgMissing := configFile{
		Fixtures: []configFixture{
			{
				Name:    "missing",
				Runpack: slashPath(filepath.Join("fixtures", "missing", "runpack.zip")),
			},
		},
	}
	if _, err := loadFixtureSpecs(cfgMissing, workDir); err == nil {
		t.Fatalf("expected missing runpack to fail")
	}

	fixtureDir := filepath.Join(workDir, "fixtures", "broken")
	if err := os.MkdirAll(fixtureDir, 0o750); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	badRunpackPath := filepath.Join(fixtureDir, "runpack.zip")
	if err := os.WriteFile(badRunpackPath, []byte("not a zip"), 0o600); err != nil {
		t.Fatalf("write invalid runpack: %v", err)
	}
	if err := writeJSON(filepath.Join(fixtureDir, fixtureFileName), fixtureMeta{
		SchemaID:               fixtureSchemaID,
		SchemaVersion:          fixtureSchemaV1,
		Name:                   "broken",
		RunID:                  "broken",
		Runpack:                "runpack.zip",
		ExpectedReplayExitCode: 0,
	}); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}

	cfgBroken := configFile{
		Fixtures: []configFixture{
			{
				Name:    "broken",
				Runpack: slashPath(filepath.Join("fixtures", "broken", "runpack.zip")),
			},
		},
	}
	if _, err := loadFixtureSpecs(cfgBroken, workDir); err == nil {
		t.Fatalf("expected invalid runpack to fail")
	}
}

func TestLoadFixtureSpecsRunIDMismatchErrors(t *testing.T) {
	workDir := t.TempDir()
	fixtureDir := filepath.Join(workDir, "fixtures", "demo")
	if err := os.MkdirAll(fixtureDir, 0o750); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	runpackPath := createRunpack(t, fixtureDir, "run_demo")
	metaPath := filepath.Join(fixtureDir, fixtureFileName)

	if err := writeJSON(metaPath, fixtureMeta{
		SchemaID:               fixtureSchemaID,
		SchemaVersion:          fixtureSchemaV1,
		Name:                   "demo",
		RunID:                  "run_demo",
		Runpack:                filepath.Base(runpackPath),
		ExpectedReplayExitCode: 0,
	}); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}

	cfgFixtureRunIDMismatch := configFile{
		Fixtures: []configFixture{
			{
				Name:    "demo",
				RunID:   "different_run",
				Runpack: slashPath(filepath.Join("fixtures", "demo", filepath.Base(runpackPath))),
			},
		},
	}
	if _, err := loadFixtureSpecs(cfgFixtureRunIDMismatch, workDir); err == nil {
		t.Fatalf("expected fixture run_id mismatch to fail")
	}

	if err := writeJSON(metaPath, fixtureMeta{
		SchemaID:               fixtureSchemaID,
		SchemaVersion:          fixtureSchemaV1,
		Name:                   "demo",
		RunID:                  "wrong_run_id",
		Runpack:                filepath.Base(runpackPath),
		ExpectedReplayExitCode: 0,
	}); err != nil {
		t.Fatalf("rewrite fixture metadata: %v", err)
	}
	cfgRunpackRunIDMismatch := configFile{
		Fixtures: []configFixture{
			{
				Name:    "demo",
				Runpack: slashPath(filepath.Join("fixtures", "demo", filepath.Base(runpackPath))),
			},
		},
	}
	if _, err := loadFixtureSpecs(cfgRunpackRunIDMismatch, workDir); err == nil {
		t.Fatalf("expected runpack run_id mismatch to fail")
	}
}

func TestGraderMetadata(t *testing.T) {
	if name := (schemaValidationGrader{}).Name(); name != "schema_validation" {
		t.Fatalf("unexpected schema grader name: %s", name)
	}
	if !(schemaValidationGrader{}).Deterministic() {
		t.Fatalf("schema grader should be deterministic")
	}
	if name := (expectedReplayExitCodeGrader{}).Name(); name != "expected_exit_code" {
		t.Fatalf("unexpected exit grader name: %s", name)
	}
	if !(expectedReplayExitCodeGrader{}).Deterministic() {
		t.Fatalf("exit grader should be deterministic")
	}
	if name := (diffGrader{}).Name(); name != "diff" {
		t.Fatalf("unexpected diff grader name: %s", name)
	}
	if !(diffGrader{}).Deterministic() {
		t.Fatalf("diff grader should be deterministic")
	}
}

func TestSchemaValidationGraderHandlesVerifyError(t *testing.T) {
	ctx := FixtureContext{
		Fixture: fixtureSpec{
			RunpackPath: filepath.Join(t.TempDir(), "missing.zip"),
		},
	}
	result, err := (schemaValidationGrader{}).Grade(ctx)
	if err != nil {
		t.Fatalf("schema grader returned unexpected error: %v", err)
	}
	if result.Status != regressStatusFail {
		t.Fatalf("expected fail status, got %s", result.Status)
	}
	if len(result.ReasonCodes) != 1 || result.ReasonCodes[0] != "schema_validation_error" {
		t.Fatalf("unexpected reason codes: %#v", result.ReasonCodes)
	}
}

func TestExpectedExitCodeGraderHandlesReplayError(t *testing.T) {
	ctx := FixtureContext{
		Fixture: fixtureSpec{
			Meta: fixtureMeta{
				ExpectedReplayExitCode: 0,
			},
			RunpackPath: filepath.Join(t.TempDir(), "missing.zip"),
		},
	}
	result, err := (expectedReplayExitCodeGrader{}).Grade(ctx)
	if err != nil {
		t.Fatalf("exit code grader returned unexpected error: %v", err)
	}
	if result.Status != regressStatusFail {
		t.Fatalf("expected fail status, got %s", result.Status)
	}
	if len(result.ReasonCodes) != 1 || result.ReasonCodes[0] != "replay_error" {
		t.Fatalf("unexpected reason codes: %#v", result.ReasonCodes)
	}
}

func TestDiffGraderErrorPaths(t *testing.T) {
	workDir := t.TempDir()
	sourceRunpack := createRunpack(t, workDir, "run_demo")

	candidateMissing := FixtureContext{
		Fixture: fixtureSpec{
			FixtureDir:  workDir,
			RunpackPath: sourceRunpack,
			Meta: fixtureMeta{
				CandidateRunpack: "missing.zip",
			},
		},
	}
	missingResult, err := (diffGrader{}).Grade(candidateMissing)
	if err != nil {
		t.Fatalf("diff grader returned unexpected error: %v", err)
	}
	if missingResult.Status != regressStatusFail {
		t.Fatalf("expected fail status, got %s", missingResult.Status)
	}
	if len(missingResult.ReasonCodes) != 1 || missingResult.ReasonCodes[0] != "candidate_path_invalid" {
		t.Fatalf("unexpected reason codes: %#v", missingResult.ReasonCodes)
	}

	badCandidatePath := filepath.Join(workDir, "candidate.zip")
	if err := os.WriteFile(badCandidatePath, []byte("invalid"), 0o600); err != nil {
		t.Fatalf("write invalid candidate runpack: %v", err)
	}
	diffErrorCtx := FixtureContext{
		Fixture: fixtureSpec{
			FixtureDir:  workDir,
			RunpackPath: sourceRunpack,
			Meta: fixtureMeta{
				CandidateRunpack: badCandidatePath,
			},
		},
	}
	diffResult, err := (diffGrader{}).Grade(diffErrorCtx)
	if err != nil {
		t.Fatalf("diff grader returned unexpected error: %v", err)
	}
	if diffResult.Status != regressStatusFail {
		t.Fatalf("expected fail status, got %s", diffResult.Status)
	}
	if len(diffResult.ReasonCodes) != 1 || diffResult.ReasonCodes[0] != "diff_error" {
		t.Fatalf("unexpected reason codes: %#v", diffResult.ReasonCodes)
	}
}

func TestSummarizeChangedFiles(t *testing.T) {
	changed := summarizeChangedFiles(runpack.DiffSummary{
		FilesChanged:    []string{"z.txt", "a.txt", "a.txt"},
		ManifestChanged: true,
		IntentsChanged:  true,
		ResultsChanged:  true,
		RefsChanged:     true,
	})
	expected := []string{
		"a.txt",
		"intents.jsonl",
		"manifest.json",
		"refs.json",
		"results.jsonl",
		"z.txt",
	}
	if len(changed) != len(expected) {
		t.Fatalf("unexpected changed file count: got=%d want=%d values=%#v", len(changed), len(expected), changed)
	}
	for i := range expected {
		if changed[i] != expected[i] {
			t.Fatalf("unexpected changed files: got=%#v want=%#v", changed, expected)
		}
	}
}

func TestResolveCandidatePath(t *testing.T) {
	workDir := t.TempDir()
	fixtureDir := filepath.Join(workDir, "fixtures", "demo")
	if err := os.MkdirAll(fixtureDir, 0o750); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	runpackPath := createRunpack(t, fixtureDir, "run_demo")

	defaultPath, err := resolveCandidatePath(fixtureSpec{
		FixtureDir:  fixtureDir,
		RunpackPath: runpackPath,
	})
	if err != nil {
		t.Fatalf("resolve default candidate path: %v", err)
	}
	if defaultPath != runpackPath {
		t.Fatalf("expected default candidate path to be runpack path, got %s", defaultPath)
	}

	relativeCandidate := createVariantRunpack(t, fixtureDir, "run_demo", "candidate")
	resolvedRelative, err := resolveCandidatePath(fixtureSpec{
		FixtureDir:  fixtureDir,
		RunpackPath: runpackPath,
		Meta: fixtureMeta{
			CandidateRunpack: filepath.Base(relativeCandidate),
		},
	})
	if err != nil {
		t.Fatalf("resolve relative candidate path: %v", err)
	}
	if resolvedRelative != relativeCandidate {
		t.Fatalf("unexpected resolved relative candidate path: %s", resolvedRelative)
	}

	if _, err := resolveCandidatePath(fixtureSpec{
		FixtureDir:  fixtureDir,
		RunpackPath: runpackPath,
		Meta: fixtureMeta{
			CandidateRunpack: "missing.zip",
		},
	}); err == nil {
		t.Fatalf("expected missing candidate path to fail")
	}
}

func TestHelperUtilities(t *testing.T) {
	fail := failResult("grader", "problem", map[string]any{"key": "value"})
	if fail.Name != "grader" || fail.Status != regressStatusFail {
		t.Fatalf("unexpected fail result: %#v", fail)
	}
	if len(fail.ReasonCodes) != 1 || fail.ReasonCodes[0] != "problem" {
		t.Fatalf("unexpected fail reason codes: %#v", fail.ReasonCodes)
	}

	deduped := uniqueSortedStrings([]string{" z ", "a", "a", "", "b"})
	expectedDeduped := []string{"a", "b", "z"}
	if len(deduped) != len(expectedDeduped) {
		t.Fatalf("unexpected deduped length: %#v", deduped)
	}
	for i := range expectedDeduped {
		if deduped[i] != expectedDeduped[i] {
			t.Fatalf("unexpected deduped values: got=%#v want=%#v", deduped, expectedDeduped)
		}
	}
	if got := uniqueSortedStrings(nil); len(got) != 0 {
		t.Fatalf("expected empty unique set for nil input, got %#v", got)
	}

	if fixture, grader := splitFixtureGraderName("demo/schema_validation"); fixture != "demo" || grader != "schema_validation" {
		t.Fatalf("unexpected split with fixture/grader: fixture=%s grader=%s", fixture, grader)
	}
	if fixture, grader := splitFixtureGraderName("schema_validation"); fixture != "regress" || grader != "schema_validation" {
		t.Fatalf("unexpected split fallback: fixture=%s grader=%s", fixture, grader)
	}
}

func TestBuildJUnitHandlesFailureWithoutReasonCodes(t *testing.T) {
	report := buildJUnit(schemaregress.RegressResult{
		FixtureSet: "default",
		Graders: []schemaregress.GraderResult{
			{
				Name:        "demo/schema_validation",
				Status:      regressStatusFail,
				ReasonCodes: []string{},
			},
		},
	})
	if report.Failures != 1 {
		t.Fatalf("expected one junit failure, got %d", report.Failures)
	}
	if len(report.Suites) != 1 || len(report.Suites[0].TestCases) != 1 {
		t.Fatalf("unexpected junit suites: %#v", report.Suites)
	}
	failure := report.Suites[0].TestCases[0].Failure
	if failure == nil || failure.Message != "failed" || failure.Body != "failed" {
		t.Fatalf("unexpected junit failure payload: %#v", failure)
	}
}

func TestWriteJUnitReportPaths(t *testing.T) {
	workDir := t.TempDir()
	result := schemaregress.RegressResult{
		FixtureSet: "default",
		Graders: []schemaregress.GraderResult{
			{
				Name:        "demo/schema_validation",
				Status:      regressStatusPass,
				ReasonCodes: []string{},
			},
		},
	}

	reportPath := filepath.Join(workDir, "reports", "junit.xml")
	if err := writeJUnitReport(reportPath, result); err != nil {
		t.Fatalf("write junit report: %v", err)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected junit report to exist: %v", err)
	}

	blocker := filepath.Join(workDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	if err := writeJUnitReport(filepath.Join(blocker, "nested", "junit.xml"), result); err == nil {
		t.Fatalf("expected junit write with invalid directory path to fail")
	}
}

func mustReadFixtureMeta(t *testing.T, path string) fixtureMeta {
	t.Helper()
	meta, err := readFixtureMeta(path)
	if err != nil {
		t.Fatalf("read fixture metadata: %v", err)
	}
	return meta
}

func mustReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	// #nosec G304 -- test controls path in temp dir.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return content
}

func mustReadJUnit(t *testing.T, path string) junitTestSuites {
	t.Helper()
	raw := mustReadBytes(t, path)
	var report junitTestSuites
	if err := xml.Unmarshal(raw, &report); err != nil {
		t.Fatalf("parse junit report: %v", err)
	}
	return report
}

func hasFailedReason(results []schemaregress.GraderResult, name, reason string) bool {
	for _, result := range results {
		if result.Name != name || result.Status != regressStatusFail {
			continue
		}
		for _, code := range result.ReasonCodes {
			if code == reason {
				return true
			}
		}
	}
	return false
}

func createVariantRunpack(t *testing.T, dir, runID, variant string) string {
	t.Helper()
	path := filepath.Join(dir, "candidate_"+variant+".zip")
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
				Result:       map[string]any{"ok": true, "variant": variant},
			},
		},
		Refs: schemarunpack.Refs{
			RunID: runID,
		},
	})
	if err != nil {
		t.Fatalf("write variant runpack: %v", err)
	}
	return path
}
