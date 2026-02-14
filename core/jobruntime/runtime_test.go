package jobruntime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSubmitAndStatus(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	state, err := Submit(root, SubmitOptions{JobID: "job-1", ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if state.Status != StatusRunning {
		t.Fatalf("expected running status, got %s", state.Status)
	}
	loaded, err := Status(root, "job-1")
	if err != nil {
		t.Fatalf("status job: %v", err)
	}
	if loaded.EnvironmentFingerprint == "" {
		t.Fatalf("expected environment fingerprint")
	}
}

func TestCheckpointDecisionNeededRequiresRequiredAction(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	if _, err := Submit(root, SubmitOptions{JobID: "job-2"}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	_, _, err := AddCheckpoint(root, "job-2", CheckpointOptions{Type: CheckpointTypeDecisionNeeded, Summary: "need input"})
	if err == nil {
		t.Fatalf("expected checkpoint validation error")
	}
	if !errors.Is(err, ErrInvalidCheckpoint) {
		t.Fatalf("expected invalid checkpoint, got %v", err)
	}
}

func TestDecisionNeededResumeRequiresApproval(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	if _, err := Submit(root, SubmitOptions{JobID: "job-3"}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, _, err := AddCheckpoint(root, "job-3", CheckpointOptions{Type: CheckpointTypeDecisionNeeded, Summary: "need input", RequiredAction: "approve"}); err != nil {
		t.Fatalf("add checkpoint: %v", err)
	}
	_, err := Resume(root, "job-3", ResumeOptions{})
	if err == nil {
		t.Fatalf("expected approval required")
	}
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected ErrApprovalRequired, got %v", err)
	}
	if _, err := Approve(root, "job-3", ApprovalOptions{Actor: "alice"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	state, err := Resume(root, "job-3", ResumeOptions{})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if state.Status != StatusRunning {
		t.Fatalf("expected running, got %s", state.Status)
	}
}

func TestDecisionNeededRequiresApprovalForEachCheckpoint(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	jobID := "job-multi-decision-approval"
	if _, err := Submit(root, SubmitOptions{JobID: jobID}); err != nil {
		t.Fatalf("submit job: %v", err)
	}

	if _, _, err := AddCheckpoint(root, jobID, CheckpointOptions{
		Type:           CheckpointTypeDecisionNeeded,
		Summary:        "decision 1",
		RequiredAction: "approve",
	}); err != nil {
		t.Fatalf("add first decision checkpoint: %v", err)
	}
	if _, err := Resume(root, jobID, ResumeOptions{}); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected first approval requirement, got %v", err)
	}
	if _, err := Approve(root, jobID, ApprovalOptions{Actor: "alice"}); err != nil {
		t.Fatalf("approve first decision: %v", err)
	}
	if _, err := Resume(root, jobID, ResumeOptions{}); err != nil {
		t.Fatalf("resume after first approval: %v", err)
	}

	if _, _, err := AddCheckpoint(root, jobID, CheckpointOptions{
		Type:           CheckpointTypeDecisionNeeded,
		Summary:        "decision 2",
		RequiredAction: "approve",
	}); err != nil {
		t.Fatalf("add second decision checkpoint: %v", err)
	}
	if _, err := Resume(root, jobID, ResumeOptions{}); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected second approval requirement, got %v", err)
	}
	if _, err := Approve(root, jobID, ApprovalOptions{Actor: "bob"}); err != nil {
		t.Fatalf("approve second decision: %v", err)
	}
	if state, err := Resume(root, jobID, ResumeOptions{}); err != nil || state.Status != StatusRunning {
		t.Fatalf("resume after second approval failed: state=%#v err=%v", state, err)
	}
}

func TestResumeEnvironmentMismatchFailClosed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	if _, err := Submit(root, SubmitOptions{JobID: "job-4", EnvironmentFingerprint: "env:a"}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, err := Pause(root, "job-4", TransitionOptions{Now: time.Now()}); err != nil {
		t.Fatalf("pause: %v", err)
	}
	_, err := Resume(root, "job-4", ResumeOptions{CurrentEnvironmentFingerprint: "env:b"})
	if err == nil {
		t.Fatalf("expected env mismatch error")
	}
	if !errors.Is(err, ErrEnvironmentMismatch) {
		t.Fatalf("expected ErrEnvironmentMismatch, got %v", err)
	}
	state, err := Resume(root, "job-4", ResumeOptions{CurrentEnvironmentFingerprint: "env:b", AllowEnvironmentMismatch: true, Reason: "manual-override"})
	if err != nil {
		t.Fatalf("resume override: %v", err)
	}
	if state.StatusReasonCode != "resumed_with_env_override" {
		t.Fatalf("expected override reason code, got %s", state.StatusReasonCode)
	}
}

func TestInvalidPauseTransition(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	if _, err := Submit(root, SubmitOptions{JobID: "job-5"}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, err := Cancel(root, "job-5", TransitionOptions{}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	_, err := Pause(root, "job-5", TransitionOptions{})
	if err == nil {
		t.Fatalf("expected invalid transition")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestListGetAndInspect(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	jobID := "job-list-inspect"

	if _, err := Submit(root, SubmitOptions{JobID: jobID}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, _, err := AddCheckpoint(root, jobID, CheckpointOptions{
		Type:    CheckpointTypePlan,
		Summary: "plan checkpoint",
	}); err != nil {
		t.Fatalf("add plan checkpoint: %v", err)
	}
	if _, _, err := AddCheckpoint(root, jobID, CheckpointOptions{
		Type:    CheckpointTypeProgress,
		Summary: "progress checkpoint",
	}); err != nil {
		t.Fatalf("add progress checkpoint: %v", err)
	}

	checkpoints, err := ListCheckpoints(root, jobID)
	if err != nil {
		t.Fatalf("list checkpoints: %v", err)
	}
	if len(checkpoints) != 2 {
		t.Fatalf("expected two checkpoints, got %d", len(checkpoints))
	}

	checkpoint, err := GetCheckpoint(root, jobID, checkpoints[0].CheckpointID)
	if err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if checkpoint.CheckpointID != checkpoints[0].CheckpointID {
		t.Fatalf("unexpected checkpoint returned: %#v", checkpoint)
	}

	state, events, err := Inspect(root, jobID)
	if err != nil {
		t.Fatalf("inspect job: %v", err)
	}
	if state.JobID != jobID {
		t.Fatalf("inspect state job id mismatch: %s", state.JobID)
	}
	if len(events) == 0 {
		t.Fatalf("expected inspect to include events")
	}
}

func TestListAndGetErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	if _, err := ListCheckpoints(root, "missing"); err == nil {
		t.Fatalf("expected list checkpoints missing job error")
	}
	if _, err := GetCheckpoint(root, "missing", "cp_1"); err == nil {
		t.Fatalf("expected get checkpoint missing job error")
	}
}

func TestAcquireLockStaleRecovery(t *testing.T) {
	now := time.Date(2026, time.February, 14, 12, 0, 0, 0, time.UTC)
	lockPath := filepath.Join(t.TempDir(), "state.lock")
	if err := os.WriteFile(lockPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	staleTime := now.Add(-time.Minute)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("set stale lock mtime: %v", err)
	}

	release, err := acquireLock(lockPath, now, time.Second)
	if err != nil {
		t.Fatalf("acquire lock with stale file: %v", err)
	}
	release()

	if staleLock(lockPath, now, 30*time.Second) {
		t.Fatalf("removed lock should not be stale")
	}
}

func TestReadEventsParseError(t *testing.T) {
	eventsPath := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(eventsPath, []byte("{bad-json}\n"), 0o600); err != nil {
		t.Fatalf("write invalid events: %v", err)
	}
	if _, err := readEvents(eventsPath); err == nil {
		t.Fatalf("expected readEvents parse error")
	}
}

func TestCheckpointHelpers(t *testing.T) {
	if !isCheckpointType(CheckpointTypePlan) || !isCheckpointType(CheckpointTypeCompleted) {
		t.Fatalf("expected known checkpoint types to be valid")
	}
	if isCheckpointType("invalid") {
		t.Fatalf("expected invalid checkpoint type to be rejected")
	}

	if checkpointReasonCode(CheckpointTypePlan) != "checkpoint_plan" {
		t.Fatalf("unexpected plan reason code")
	}
	if checkpointReasonCode(CheckpointTypeProgress) != "checkpoint_progress" {
		t.Fatalf("unexpected progress reason code")
	}
	if checkpointReasonCode(CheckpointTypeDecisionNeeded) != "checkpoint_decision_needed" {
		t.Fatalf("unexpected decision-needed reason code")
	}
	if checkpointReasonCode(CheckpointTypeBlocked) != "checkpoint_blocked" {
		t.Fatalf("unexpected blocked reason code")
	}
	if checkpointReasonCode(CheckpointTypeCompleted) != "checkpoint_completed" {
		t.Fatalf("unexpected completed reason code")
	}
	if checkpointReasonCode("unknown") != "checkpoint" {
		t.Fatalf("unexpected fallback reason code")
	}
}

func TestSubmitValidationAndDuplicates(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	if _, err := Submit(root, SubmitOptions{}); err == nil {
		t.Fatalf("expected missing job_id validation error")
	}
	if _, err := Submit(root, SubmitOptions{JobID: "job-dup"}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, err := Submit(root, SubmitOptions{JobID: "job-dup"}); err == nil {
		t.Fatalf("expected duplicate job submit error")
	}
}

func TestAddCheckpointValidationAndStateTransitions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	if _, err := Submit(root, SubmitOptions{JobID: "job-checkpoint-validation"}); err != nil {
		t.Fatalf("submit job: %v", err)
	}

	if _, _, err := AddCheckpoint(root, "job-checkpoint-validation", CheckpointOptions{
		Type:    "bad",
		Summary: "x",
	}); err == nil {
		t.Fatalf("expected invalid type error")
	}
	if _, _, err := AddCheckpoint(root, "job-checkpoint-validation", CheckpointOptions{
		Type:    CheckpointTypeProgress,
		Summary: "",
	}); err == nil {
		t.Fatalf("expected missing summary error")
	}
	if _, _, err := AddCheckpoint(root, "job-checkpoint-validation", CheckpointOptions{
		Type:    CheckpointTypeProgress,
		Summary: string(make([]byte, 513)),
	}); err == nil {
		t.Fatalf("expected summary length validation error")
	}

	blockedState, blockedCheckpoint, err := AddCheckpoint(root, "job-checkpoint-validation", CheckpointOptions{
		Type:    CheckpointTypeBlocked,
		Summary: "blocked checkpoint",
	})
	if err != nil {
		t.Fatalf("add blocked checkpoint: %v", err)
	}
	if blockedState.Status != StatusBlocked || blockedCheckpoint.ReasonCode != "checkpoint_blocked" {
		t.Fatalf("unexpected blocked checkpoint state=%#v checkpoint=%#v", blockedState, blockedCheckpoint)
	}

	if _, err := Submit(root, SubmitOptions{JobID: "job-checkpoint-completed"}); err != nil {
		t.Fatalf("submit completed job: %v", err)
	}
	completedState, completedCheckpoint, err := AddCheckpoint(root, "job-checkpoint-completed", CheckpointOptions{
		Type:    CheckpointTypeCompleted,
		Summary: "completed checkpoint",
	})
	if err != nil {
		t.Fatalf("add completed checkpoint: %v", err)
	}
	if completedState.Status != StatusCompleted || completedCheckpoint.ReasonCode != "checkpoint_completed" {
		t.Fatalf("unexpected completed checkpoint state=%#v checkpoint=%#v", completedState, completedCheckpoint)
	}
}

func TestApproveRequiresActor(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	if _, err := Submit(root, SubmitOptions{JobID: "job-approve"}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, err := Approve(root, "job-approve", ApprovalOptions{Actor: " "}); err == nil {
		t.Fatalf("expected missing approval actor error")
	}
}

func TestReadWriteHelperErrors(t *testing.T) {
	workDir := t.TempDir()
	statePath := filepath.Join(workDir, "state.json")
	if err := os.WriteFile(statePath, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("write invalid state json: %v", err)
	}
	if _, err := readState(statePath); err == nil {
		t.Fatalf("expected readState parse error")
	}

	invalidStatePath := filepath.Join(workDir, "state_missing_job.json")
	if err := os.WriteFile(invalidStatePath, []byte(`{"schema_id":"gait.job.runtime","schema_version":"1.0.0","job_id":" "}`), 0o600); err != nil {
		t.Fatalf("write invalid state payload: %v", err)
	}
	if _, err := readState(invalidStatePath); err == nil {
		t.Fatalf("expected readState missing job_id error")
	}

	if err := writeJSON(filepath.Join(workDir, "bad.json"), map[string]any{"bad": func() {}}); err == nil {
		t.Fatalf("expected writeJSON encode error")
	}
	if err := appendEvent(filepath.Join(workDir, "events.jsonl"), Event{Payload: map[string]any{"bad": func() {}}}); err == nil {
		t.Fatalf("expected appendEvent encode error")
	}

	events, err := readEvents(filepath.Join(workDir, "missing_events.jsonl"))
	if err != nil {
		t.Fatalf("readEvents missing path should return empty events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected empty events for missing path, got %d", len(events))
	}
}

func TestAcquireLockTimeoutAndPathErrors(t *testing.T) {
	now := time.Now().UTC()

	missingParentLock := filepath.Join(t.TempDir(), "missing", "state.lock")
	if _, err := acquireLock(missingParentLock, now, time.Second); err == nil {
		t.Fatalf("expected acquireLock error for missing parent")
	}

	lockPath := filepath.Join(t.TempDir(), "busy.lock")
	if err := os.WriteFile(lockPath, []byte("busy"), 0o600); err != nil {
		t.Fatalf("write busy lock: %v", err)
	}
	if _, err := acquireLock(lockPath, now, 0); !errors.Is(err, ErrStateContention) {
		t.Fatalf("expected ErrStateContention, got %v", err)
	}
}

func TestAcquireLockTimeoutUsesWallClock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "busy.lock")
	if err := os.WriteFile(lockPath, []byte("busy"), 0o600); err != nil {
		t.Fatalf("write busy lock: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := acquireLock(lockPath, time.Now().Add(24*time.Hour), 50*time.Millisecond)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrStateContention) {
			t.Fatalf("expected ErrStateContention, got %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("acquireLock did not honor wall-clock timeout")
	}
}

func TestJobPathDecisionAndFingerprintHelpers(t *testing.T) {
	statePath, eventsPath := jobPaths("", "job-helper")
	if filepath.Base(statePath) != "state.json" || filepath.Base(eventsPath) != "events.jsonl" {
		t.Fatalf("unexpected helper job paths: state=%s events=%s", statePath, eventsPath)
	}
	if !strings.Contains(statePath, filepath.Join("gait-out", "jobs")) {
		t.Fatalf("expected default root in state path: %s", statePath)
	}

	if hasPendingDecision(nil) {
		t.Fatalf("nil state should not have pending decisions")
	}
	if hasPendingDecision(&JobState{Checkpoints: []Checkpoint{{Type: CheckpointTypeProgress}}}) {
		t.Fatalf("progress-only checkpoints should not be pending decision")
	}
	if !hasPendingDecision(&JobState{Checkpoints: []Checkpoint{{Type: CheckpointTypeDecisionNeeded}}}) {
		t.Fatalf("decision-needed checkpoint should be pending decision")
	}
	if !requiresApprovalBeforeResume(&JobState{
		Status:      StatusDecisionNeeded,
		Checkpoints: []Checkpoint{{Type: CheckpointTypeDecisionNeeded}},
	}) {
		t.Fatalf("decision-needed checkpoint without approval should require approval")
	}
	if requiresApprovalBeforeResume(&JobState{
		Status:      StatusDecisionNeeded,
		Checkpoints: []Checkpoint{{Type: CheckpointTypeDecisionNeeded}},
		Approvals:   []Approval{{Actor: "alice", CreatedAt: time.Now().UTC(), Reason: "ok"}},
	}) {
		t.Fatalf("approved decision-needed checkpoint should not require additional approval")
	}
	if !requiresApprovalBeforeResume(&JobState{
		Status: StatusPaused,
		Checkpoints: []Checkpoint{
			{Type: CheckpointTypeDecisionNeeded},
			{Type: CheckpointTypeDecisionNeeded},
		},
		Approvals: []Approval{{Actor: "alice", CreatedAt: time.Now().UTC(), Reason: "one"}},
	}) {
		t.Fatalf("fewer approvals than decision checkpoints should require approval")
	}
	if got := countDecisionCheckpoints(&JobState{
		Checkpoints: []Checkpoint{
			{Type: CheckpointTypeProgress},
			{Type: CheckpointTypeDecisionNeeded},
			{Type: CheckpointTypeDecisionNeeded},
		},
	}); got != 2 {
		t.Fatalf("expected two decision checkpoints, got %d", got)
	}

	if got := EnvironmentFingerprint(" manual "); got != "manual" {
		t.Fatalf("unexpected override fingerprint: %s", got)
	}
	if got := EnvironmentFingerprint(""); !strings.HasPrefix(got, "envfp:") {
		t.Fatalf("expected generated fingerprint prefix, got %s", got)
	}
}
