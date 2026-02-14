package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunJobLifecycleCommands(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	root := filepath.Join(workDir, "jobs")
	jobID := "job_cli_lifecycle"

	submitCode, submitOut := runJobJSON(t, []string{"submit", "--id", jobID, "--root", root, "--actor", "alice", "--json"})
	if submitCode != exitOK {
		t.Fatalf("job submit expected %d got %d output=%#v", exitOK, submitCode, submitOut)
	}
	if submitOut.Job == nil || submitOut.Job.JobID != jobID {
		t.Fatalf("unexpected submit output: %#v", submitOut)
	}

	statusCode, statusOut := runJobJSON(t, []string{"status", "--id", jobID, "--root", root, "--json"})
	if statusCode != exitOK {
		t.Fatalf("job status expected %d got %d output=%#v", exitOK, statusCode, statusOut)
	}
	if statusOut.Job == nil || statusOut.Job.Status != "running" {
		t.Fatalf("unexpected status output: %#v", statusOut)
	}

	checkpointCode, checkpointOut := runJobJSON(t, []string{
		"checkpoint", "add",
		"--id", jobID,
		"--root", root,
		"--type", "progress",
		"--summary", "checkpoint summary",
		"--actor", "alice",
		"--json",
	})
	if checkpointCode != exitOK {
		t.Fatalf("checkpoint add expected %d got %d output=%#v", exitOK, checkpointCode, checkpointOut)
	}
	if checkpointOut.Checkpoint == nil || checkpointOut.Checkpoint.CheckpointID == "" {
		t.Fatalf("unexpected checkpoint add output: %#v", checkpointOut)
	}

	listCode, listOut := runJobJSON(t, []string{"checkpoint", "list", "--id", jobID, "--root", root, "--json"})
	if listCode != exitOK {
		t.Fatalf("checkpoint list expected %d got %d output=%#v", exitOK, listCode, listOut)
	}
	if len(listOut.Checkpoints) == 0 {
		t.Fatalf("expected checkpoint list entries")
	}

	showCode, showOut := runJobJSON(t, []string{
		"checkpoint", "show",
		"--id", jobID,
		"--root", root,
		"--checkpoint", listOut.Checkpoints[0].CheckpointID,
		"--json",
	})
	if showCode != exitOK {
		t.Fatalf("checkpoint show expected %d got %d output=%#v", exitOK, showCode, showOut)
	}
	if showOut.Checkpoint == nil || showOut.Checkpoint.CheckpointID != listOut.Checkpoints[0].CheckpointID {
		t.Fatalf("unexpected checkpoint show output: %#v", showOut)
	}

	pauseCode, pauseOut := runJobJSON(t, []string{"pause", "--id", jobID, "--root", root, "--actor", "alice", "--json"})
	if pauseCode != exitOK {
		t.Fatalf("pause expected %d got %d output=%#v", exitOK, pauseCode, pauseOut)
	}
	if pauseOut.Job == nil || pauseOut.Job.Status != "paused" {
		t.Fatalf("unexpected pause output: %#v", pauseOut)
	}

	inspectCode, inspectOut := runJobJSON(t, []string{"inspect", "--id", jobID, "--root", root, "--json"})
	if inspectCode != exitOK {
		t.Fatalf("inspect expected %d got %d output=%#v", exitOK, inspectCode, inspectOut)
	}
	if inspectOut.Job == nil || len(inspectOut.Events) == 0 {
		t.Fatalf("unexpected inspect output: %#v", inspectOut)
	}

	cancelCode, cancelOut := runJobJSON(t, []string{"cancel", "--id", jobID, "--root", root, "--actor", "alice", "--json"})
	if cancelCode != exitOK {
		t.Fatalf("cancel expected %d got %d output=%#v", exitOK, cancelCode, cancelOut)
	}
	if cancelOut.Job == nil || cancelOut.Job.Status != "cancelled" {
		t.Fatalf("unexpected cancel output: %#v", cancelOut)
	}

	var textCode int
	textOut := captureStdout(t, func() {
		textCode = runJob([]string{"status", "--id", jobID, "--root", root})
	})
	if textCode != exitOK {
		t.Fatalf("text status expected %d got %d", exitOK, textCode)
	}
	if !strings.Contains(textOut, "job status:") {
		t.Fatalf("expected text status output, got %q", textOut)
	}
}

func TestRunJobHelpAndErrorPaths(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	root := filepath.Join(workDir, "jobs")
	jobID := "job_cli_errors"

	if code := runJob([]string{}); code != exitInvalidInput {
		t.Fatalf("job root usage expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("job unknown command expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"checkpoint"}); code != exitInvalidInput {
		t.Fatalf("job checkpoint usage expected %d got %d", exitInvalidInput, code)
	}

	helpCases := [][]string{
		{"submit", "--help"},
		{"status", "--help"},
		{"checkpoint", "add", "--help"},
		{"checkpoint", "list", "--help"},
		{"checkpoint", "show", "--help"},
		{"pause", "--help"},
		{"approve", "--help"},
		{"resume", "--help"},
		{"cancel", "--help"},
		{"inspect", "--help"},
	}
	for _, args := range helpCases {
		if code := runJob(args); code != exitOK {
			t.Fatalf("help path %v expected %d got %d", args, exitOK, code)
		}
	}

	if code := runJob([]string{"submit", "--id", jobID, "--root", root, "--json"}); code != exitOK {
		t.Fatalf("submit expected %d got %d", exitOK, code)
	}
	if code := runJob([]string{"status", "--id", jobID, "--root", root, "extra", "--json"}); code != exitInvalidInput {
		t.Fatalf("status positional-arg validation expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"checkpoint", "list", "--id", jobID, "--root", root, "extra", "--json"}); code != exitInvalidInput {
		t.Fatalf("checkpoint list positional-arg validation expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"checkpoint", "show", "--id", jobID, "--root", root, "--checkpoint", "missing", "--json"}); code != exitInvalidInput {
		t.Fatalf("checkpoint show missing checkpoint expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"pause", "--id", "missing", "--root", root, "--json"}); code != exitInternalFailure {
		t.Fatalf("pause missing job expected %d got %d", exitInternalFailure, code)
	}
	if code := runJob([]string{"cancel", "--id", "missing", "--root", root, "--json"}); code != exitInternalFailure {
		t.Fatalf("cancel missing job expected %d got %d", exitInternalFailure, code)
	}
	if code := runJob([]string{"inspect", "--id", "missing", "--root", root, "--json"}); code != exitInvalidInput {
		t.Fatalf("inspect missing job expected %d got %d", exitInvalidInput, code)
	}
}

func runJobJSON(t *testing.T, args []string) (int, jobOutput) {
	t.Helper()
	var code int
	raw := captureStdout(t, func() {
		code = runJob(args)
	})
	var output jobOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode job output: %v raw=%q", err, raw)
	}
	return code, output
}
