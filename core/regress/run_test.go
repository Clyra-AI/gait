package regress

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func mustReadFixtureMeta(t *testing.T, path string) fixtureMeta {
	t.Helper()
	meta, err := readFixtureMeta(path)
	if err != nil {
		t.Fatalf("read fixture metadata: %v", err)
	}
	return meta
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
