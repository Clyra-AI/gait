package integration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/jobruntime"
	"github.com/davidahmann/gait/core/pack"
	"github.com/davidahmann/gait/core/regress"
	"github.com/davidahmann/gait/core/runpack"
)

func TestJobRuntimeToPackRoundTrip(t *testing.T) {
	workDir := t.TempDir()
	jobsRoot := filepath.Join(workDir, "jobs")
	now := time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC)

	if _, err := jobruntime.Submit(jobsRoot, jobruntime.SubmitOptions{
		JobID:                  "job_integration_v24",
		ProducerVersion:        "0.0.0-test",
		EnvironmentFingerprint: "envfp:baseline",
		Now:                    now,
	}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, _, err := jobruntime.AddCheckpoint(jobsRoot, "job_integration_v24", jobruntime.CheckpointOptions{
		Type:           jobruntime.CheckpointTypeDecisionNeeded,
		Summary:        "approval required",
		RequiredAction: "approve",
		Now:            now.Add(time.Second),
	}); err != nil {
		t.Fatalf("add decision-needed checkpoint: %v", err)
	}
	if _, err := jobruntime.Approve(jobsRoot, "job_integration_v24", jobruntime.ApprovalOptions{
		Actor: "alice",
		Now:   now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("approve job: %v", err)
	}
	if _, err := jobruntime.Resume(jobsRoot, "job_integration_v24", jobruntime.ResumeOptions{
		CurrentEnvironmentFingerprint: "envfp:override",
		AllowEnvironmentMismatch:      true,
		Reason:                        "integration-test",
		Actor:                         "alice",
		Now:                           now.Add(3 * time.Second),
	}); err != nil {
		t.Fatalf("resume job: %v", err)
	}
	if _, err := jobruntime.Cancel(jobsRoot, "job_integration_v24", jobruntime.TransitionOptions{
		Actor: "alice",
		Now:   now.Add(4 * time.Second),
	}); err != nil {
		t.Fatalf("cancel job: %v", err)
	}

	jobPackPath := filepath.Join(workDir, "pack_job.zip")
	if _, err := pack.BuildJobPackFromPath(jobsRoot, "job_integration_v24", jobPackPath, "0.0.0-test", nil); err != nil {
		t.Fatalf("build job pack from runtime state: %v", err)
	}

	verifyResult, err := pack.Verify(jobPackPath, pack.VerifyOptions{})
	if err != nil {
		t.Fatalf("verify job pack: %v", err)
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 || len(verifyResult.UndeclaredFiles) > 0 {
		t.Fatalf("unexpected verify issues: %#v", verifyResult)
	}

	inspectResult, err := pack.Inspect(jobPackPath)
	if err != nil {
		t.Fatalf("inspect job pack: %v", err)
	}
	if inspectResult.JobPayload == nil || inspectResult.JobPayload.JobID != "job_integration_v24" {
		t.Fatalf("unexpected job payload in inspect output: %#v", inspectResult.JobPayload)
	}
	if inspectResult.JobLineage == nil || inspectResult.JobLineage.EventCount == 0 {
		t.Fatalf("expected job lineage details in inspect output")
	}
}

func TestRegressInitFromPackSource(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := filepath.Join(workDir, "runpack_source.zip")
	run, intents, results, refs := integrationRunpackFixture(t)
	if _, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
	}); err != nil {
		t.Fatalf("write runpack source: %v", err)
	}

	runPackPath := filepath.Join(workDir, "pack_run.zip")
	if _, err := pack.BuildRunPack(pack.BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  runPackPath,
	}); err != nil {
		t.Fatalf("build run pack source: %v", err)
	}

	initResult, err := regress.InitFixture(regress.InitOptions{
		SourceRunpackPath: runPackPath,
		FixtureName:       "fixture_from_pack_source",
		WorkDir:           workDir,
	})
	if err != nil {
		t.Fatalf("init regress fixture from run pack source: %v", err)
	}
	if initResult.RunID != "run_integration" {
		t.Fatalf("unexpected run_id from regress init: %s", initResult.RunID)
	}

	runResult, err := regress.Run(regress.RunOptions{
		ConfigPath:      filepath.Join(workDir, "gait.yaml"),
		OutputPath:      filepath.Join(workDir, "regress_result.json"),
		WorkDir:         workDir,
		ProducerVersion: "0.0.0-test",
	})
	if err != nil {
		t.Fatalf("run regress after pack-source init: %v", err)
	}
	if runResult.Result.Status != "pass" {
		t.Fatalf("expected regress run status pass, got %s", runResult.Result.Status)
	}
}
