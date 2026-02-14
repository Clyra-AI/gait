package e2e

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestCLIV24JobPackLifecycle(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)
	workDir := t.TempDir()
	jobsRoot := filepath.Join(workDir, "jobs")

	// Run source for run-pack build and regress bootstrap.
	runJSONCommand(t, workDir, binPath, "demo")

	// Durable job lifecycle.
	runJSONCommand(t, workDir, binPath, "job", "submit", "--id", "job_v24_e2e", "--root", jobsRoot, "--json")
	runJSONCommand(t, workDir, binPath, "job", "checkpoint", "add", "--id", "job_v24_e2e", "--root", jobsRoot, "--type", "decision-needed", "--summary", "need approval", "--required-action", "approve", "--json")
	runJSONCommand(t, workDir, binPath, "job", "pause", "--id", "job_v24_e2e", "--root", jobsRoot, "--json")
	runJSONCommand(t, workDir, binPath, "job", "approve", "--id", "job_v24_e2e", "--root", jobsRoot, "--actor", "alice", "--reason", "approved", "--json")
	runJSONCommand(t, workDir, binPath, "job", "resume", "--id", "job_v24_e2e", "--root", jobsRoot, "--allow-env-mismatch", "--env-fingerprint", "envfp:override", "--reason", "approved", "--json")
	runJSONCommand(t, workDir, binPath, "job", "cancel", "--id", "job_v24_e2e", "--root", jobsRoot, "--json")

	statusOut := runJSONCommand(t, workDir, binPath, "job", "status", "--id", "job_v24_e2e", "--root", jobsRoot, "--json")
	var statusResult struct {
		OK  bool `json:"ok"`
		Job struct {
			Status     string `json:"status"`
			StopReason string `json:"stop_reason"`
		} `json:"job"`
	}
	if err := json.Unmarshal(statusOut, &statusResult); err != nil {
		t.Fatalf("parse job status output: %v\n%s", err, string(statusOut))
	}
	if !statusResult.OK || statusResult.Job.Status != "cancelled" || statusResult.Job.StopReason != "cancelled_by_user" {
		t.Fatalf("unexpected job lifecycle status output: %s", string(statusOut))
	}

	// Unified pack surface.
	runPackPath := filepath.Join(workDir, "pack_run.zip")
	jobPackPath := filepath.Join(workDir, "pack_job.zip")
	runJSONCommand(t, workDir, binPath, "pack", "build", "--type", "run", "--from", "run_demo", "--out", runPackPath, "--json")
	runJSONCommand(t, workDir, binPath, "pack", "build", "--type", "job", "--from", "job_v24_e2e", "--job-root", jobsRoot, "--out", jobPackPath, "--json")

	verifyRunOut := runJSONCommand(t, workDir, binPath, "pack", "verify", runPackPath, "--json")
	verifyJobOut := runJSONCommand(t, workDir, binPath, "pack", "verify", jobPackPath, "--json")
	var verifyRun struct {
		OK bool `json:"ok"`
	}
	var verifyJob struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(verifyRunOut, &verifyRun); err != nil {
		t.Fatalf("parse run verify output: %v\n%s", err, string(verifyRunOut))
	}
	if err := json.Unmarshal(verifyJobOut, &verifyJob); err != nil {
		t.Fatalf("parse job verify output: %v\n%s", err, string(verifyJobOut))
	}
	if !verifyRun.OK || !verifyJob.OK {
		t.Fatalf("expected pack verify ok for run/job packs")
	}

	inspectRunOut := runJSONCommand(t, workDir, binPath, "pack", "inspect", runPackPath, "--json")
	inspectJobOut := runJSONCommand(t, workDir, binPath, "pack", "inspect", jobPackPath, "--json")
	var inspectRun struct {
		OK      bool `json:"ok"`
		Inspect struct {
			RunLineage struct {
				IntentResults []struct {
					IntentID string `json:"intent_id"`
				} `json:"intent_results"`
			} `json:"run_lineage"`
		} `json:"inspect"`
	}
	var inspectJob struct {
		OK      bool `json:"ok"`
		Inspect struct {
			JobLineage struct {
				EventCount int `json:"event_count"`
			} `json:"job_lineage"`
		} `json:"inspect"`
	}
	if err := json.Unmarshal(inspectRunOut, &inspectRun); err != nil {
		t.Fatalf("parse run inspect output: %v\n%s", err, string(inspectRunOut))
	}
	if err := json.Unmarshal(inspectJobOut, &inspectJob); err != nil {
		t.Fatalf("parse job inspect output: %v\n%s", err, string(inspectJobOut))
	}
	if !inspectRun.OK || len(inspectRun.Inspect.RunLineage.IntentResults) == 0 {
		t.Fatalf("expected run lineage details in inspect output: %s", string(inspectRunOut))
	}
	if !inspectJob.OK || inspectJob.Inspect.JobLineage.EventCount == 0 {
		t.Fatalf("expected job lineage details in inspect output: %s", string(inspectJobOut))
	}

	bootstrapOut := runJSONCommand(t, workDir, binPath, "regress", "bootstrap", "--from", runPackPath, "--name", "v24_pack_bootstrap", "--json")
	var bootstrap struct {
		OK     bool   `json:"ok"`
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(bootstrapOut, &bootstrap); err != nil {
		t.Fatalf("parse regress bootstrap output: %v\n%s", err, string(bootstrapOut))
	}
	if !bootstrap.OK || bootstrap.RunID != "run_demo" || bootstrap.Status != "pass" {
		t.Fatalf("unexpected regress bootstrap output: %s", string(bootstrapOut))
	}
}
