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

func TestResumePolicyTransitionAndIdentityValidation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	jobID := "job-policy-transition"
	if _, err := Submit(root, SubmitOptions{
		JobID:                  jobID,
		EnvironmentFingerprint: "env:a",
		PolicyDigest:           "policy-digest-a",
		PolicyRef:              "policy-a.yaml",
		Identity:               "agent.alice",
	}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, err := Pause(root, jobID, TransitionOptions{}); err != nil {
		t.Fatalf("pause: %v", err)
	}
	state, err := Resume(root, jobID, ResumeOptions{
		CurrentEnvironmentFingerprint: "env:a",
		PolicyDigest:                  "policy-digest-b",
		PolicyRef:                     "policy-b.yaml",
		RequirePolicyEvaluation:       true,
		RequireIdentityValidation:     true,
		IdentityValidationSource:      "revocation_list",
		Identity:                      "agent.alice",
	})
	if err != nil {
		t.Fatalf("resume with policy transition: %v", err)
	}
	if state.StatusReasonCode != "resumed_with_policy_transition" {
		t.Fatalf("expected policy transition reason code, got %s", state.StatusReasonCode)
	}
	if state.PolicyDigest != "policy-digest-b" || state.PolicyRef != "policy-b.yaml" {
		t.Fatalf("expected updated policy metadata in state, got %#v", state)
	}
	_, events, err := Inspect(root, jobID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected events in journal")
	}
	last := events[len(events)-1]
	if last.ReasonCode != "resumed_with_policy_transition" {
		t.Fatalf("unexpected last event reason code: %#v", last)
	}
	if got, _ := last.Payload["previous_policy_digest"].(string); got != "policy-digest-a" {
		t.Fatalf("unexpected previous policy digest in event payload: %#v", last.Payload)
	}
	if got, _ := last.Payload["current_policy_digest"].(string); got != "policy-digest-b" {
		t.Fatalf("unexpected current policy digest in event payload: %#v", last.Payload)
	}
	if got, _ := last.Payload["identity_validation_source"].(string); got != "revocation_list" {
		t.Fatalf("unexpected identity validation source in event payload: %#v", last.Payload)
	}
}

func TestResumeRequiresPolicyEvaluationWhenBoundToPolicy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	jobID := "job-policy-required"
	if _, err := Submit(root, SubmitOptions{
		JobID:                  jobID,
		EnvironmentFingerprint: "env:a",
		PolicyDigest:           "policy-digest-a",
		PolicyRef:              "policy-a.yaml",
	}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, err := Pause(root, jobID, TransitionOptions{}); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if _, err := Resume(root, jobID, ResumeOptions{
		CurrentEnvironmentFingerprint: "env:a",
	}); !errors.Is(err, ErrPolicyEvaluationRequired) {
		t.Fatalf("expected policy evaluation required error, got %v", err)
	}
}

func TestResumeIdentityValidationErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")

	if _, err := Submit(root, SubmitOptions{
		JobID:                  "job-identity-required",
		EnvironmentFingerprint: "env:a",
	}); err != nil {
		t.Fatalf("submit identity-required job: %v", err)
	}
	if _, err := Pause(root, "job-identity-required", TransitionOptions{}); err != nil {
		t.Fatalf("pause identity-required job: %v", err)
	}
	if _, err := Resume(root, "job-identity-required", ResumeOptions{
		CurrentEnvironmentFingerprint: "env:a",
		RequireIdentityValidation:     true,
	}); !errors.Is(err, ErrIdentityValidationMissing) {
		t.Fatalf("expected missing identity validation error, got %v", err)
	}

	if _, err := Submit(root, SubmitOptions{
		JobID:                  "job-identity-revoked",
		EnvironmentFingerprint: "env:a",
		Identity:               "agent.revoked",
	}); err != nil {
		t.Fatalf("submit identity-revoked job: %v", err)
	}
	if _, err := Pause(root, "job-identity-revoked", TransitionOptions{}); err != nil {
		t.Fatalf("pause identity-revoked job: %v", err)
	}
	if _, err := Resume(root, "job-identity-revoked", ResumeOptions{
		CurrentEnvironmentFingerprint: "env:a",
		IdentityRevoked:               true,
	}); !errors.Is(err, ErrIdentityRevoked) {
		t.Fatalf("expected identity revoked error, got %v", err)
	}

	if _, err := Submit(root, SubmitOptions{
		JobID:                  "job-identity-mismatch",
		EnvironmentFingerprint: "env:a",
		Identity:               "agent.alice",
	}); err != nil {
		t.Fatalf("submit identity-mismatch job: %v", err)
	}
	if _, err := Pause(root, "job-identity-mismatch", TransitionOptions{}); err != nil {
		t.Fatalf("pause identity-mismatch job: %v", err)
	}
	if _, err := Resume(root, "job-identity-mismatch", ResumeOptions{
		CurrentEnvironmentFingerprint: "env:a",
		Identity:                      "agent.bob",
	}); !errors.Is(err, ErrIdentityBindingMismatch) {
		t.Fatalf("expected identity binding mismatch error, got %v", err)
	}
	state, err := Status(root, "job-identity-mismatch")
	if err != nil {
		t.Fatalf("status identity-mismatch job: %v", err)
	}
	if state.Identity != "agent.alice" {
		t.Fatalf("expected bound identity to remain unchanged, got %#v", state)
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

func TestEmergencyStopPreemptsAndBlocksDispatch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	jobID := "job-emergency-stop"
	if _, err := Submit(root, SubmitOptions{JobID: jobID, PolicyDigest: "policy-a", Identity: "agent.alice"}); err != nil {
		t.Fatalf("submit job: %v", err)
	}

	stopped, err := EmergencyStop(root, jobID, TransitionOptions{Actor: "alice"})
	if err != nil {
		t.Fatalf("emergency stop: %v", err)
	}
	if stopped.Status != StatusEmergencyStop {
		t.Fatalf("expected emergency stopped status, got %#v", stopped)
	}
	if stopped.StatusReasonCode != "emergency_stop_preempted" {
		t.Fatalf("unexpected emergency stop reason code: %#v", stopped)
	}
	if !IsEmergencyStopped(stopped) {
		t.Fatalf("expected emergency stopped helper to return true")
	}

	if _, err := RecordBlockedDispatch(root, jobID, DispatchRecordOptions{
		Actor:        "mcp-proxy",
		DispatchPath: "mcp.proxy",
		ReasonCode:   "emergency_stop_preempted",
	}); err != nil {
		t.Fatalf("record blocked dispatch: %v", err)
	}
	_, events, err := Inspect(root, jobID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected emergency stop + blocked dispatch events, got %d", len(events))
	}
	last := events[len(events)-1]
	if last.Type != "dispatch_blocked" || last.ReasonCode != "emergency_stop_preempted" {
		t.Fatalf("unexpected blocked dispatch event: %#v", last)
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
	statePath, eventsPath, err := jobPaths("", "job-helper")
	if err != nil {
		t.Fatalf("jobPaths: %v", err)
	}
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

func TestSafetyInvariantLedgerDefaultsAndResume(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	jobID := "job-invariants"
	submitted, err := Submit(root, SubmitOptions{
		JobID:        jobID,
		PolicyDigest: "policy-digest-a",
		PolicyRef:    "policy-a.yaml",
		Identity:     "agent.alice",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.SafetyInvariantVersion == "" || submitted.SafetyInvariantHash == "" || len(submitted.SafetyInvariants) == 0 {
		t.Fatalf("expected safety invariant ledger on submit: %#v", submitted)
	}
	if _, err := Pause(root, jobID, TransitionOptions{}); err != nil {
		t.Fatalf("pause: %v", err)
	}
	resumed, err := Resume(root, jobID, ResumeOptions{
		CurrentEnvironmentFingerprint: submitted.EnvironmentFingerprint,
		PolicyDigest:                  "policy-digest-a",
		PolicyRef:                     "policy-a.yaml",
		RequirePolicyEvaluation:       true,
		Identity:                      "agent.alice",
		RequireIdentityValidation:     true,
	})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumed.SafetyInvariantVersion != submitted.SafetyInvariantVersion || resumed.SafetyInvariantHash != submitted.SafetyInvariantHash {
		t.Fatalf("expected invariant ledger to persist across resume: submitted=%#v resumed=%#v", submitted, resumed)
	}
}

func TestRecordBlockedDispatchDefaultsAndValidation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")
	jobID := "job-blocked-dispatch-defaults"
	if _, err := Submit(root, SubmitOptions{JobID: jobID}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := RecordBlockedDispatch(root, jobID, DispatchRecordOptions{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition before emergency stop, got %v", err)
	}

	if _, err := EmergencyStop(root, jobID, TransitionOptions{Actor: "operator"}); err != nil {
		t.Fatalf("emergency stop: %v", err)
	}
	if _, err := RecordBlockedDispatch(root, jobID, DispatchRecordOptions{}); err != nil {
		t.Fatalf("record blocked dispatch with defaults: %v", err)
	}

	_, events, err := Inspect(root, jobID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	last := events[len(events)-1]
	if last.ReasonCode != "emergency_stop_preempted" {
		t.Fatalf("unexpected reason code: %#v", last)
	}
	if got, _ := last.Payload["dispatch_path"].(string); got != "runtime.dispatch" {
		t.Fatalf("expected default dispatch path, got %#v", last.Payload)
	}
}

func TestResumeValidationAndEnvOverridePolicyTransition(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs")

	if _, err := Submit(root, SubmitOptions{JobID: "job-resume-invalid"}); err != nil {
		t.Fatalf("submit job-resume-invalid: %v", err)
	}
	if _, err := Resume(root, "job-resume-invalid", ResumeOptions{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition for resume while running, got %v", err)
	}

	if _, err := Submit(root, SubmitOptions{
		JobID:                  "job-bind-identity",
		EnvironmentFingerprint: "env:a",
	}); err != nil {
		t.Fatalf("submit job-bind-identity: %v", err)
	}
	if _, err := Pause(root, "job-bind-identity", TransitionOptions{}); err != nil {
		t.Fatalf("pause job-bind-identity: %v", err)
	}
	identityBound, err := Resume(root, "job-bind-identity", ResumeOptions{
		CurrentEnvironmentFingerprint: "env:a",
		Identity:                      "agent.bound",
		RequireIdentityValidation:     true,
	})
	if err != nil {
		t.Fatalf("resume job-bind-identity: %v", err)
	}
	if identityBound.Identity != "agent.bound" {
		t.Fatalf("expected resume to bind identity, got %#v", identityBound)
	}

	if _, err := Submit(root, SubmitOptions{
		JobID:                  "job-env-override-policy-transition",
		EnvironmentFingerprint: "env:a",
		PolicyDigest:           "policy-a",
		PolicyRef:              "policy-a.yaml",
	}); err != nil {
		t.Fatalf("submit job-env-override-policy-transition: %v", err)
	}
	if _, err := Pause(root, "job-env-override-policy-transition", TransitionOptions{}); err != nil {
		t.Fatalf("pause job-env-override-policy-transition: %v", err)
	}
	overridden, err := Resume(root, "job-env-override-policy-transition", ResumeOptions{
		CurrentEnvironmentFingerprint: "env:b",
		AllowEnvironmentMismatch:      true,
		PolicyDigest:                  "policy-b",
		PolicyRef:                     "policy-b.yaml",
		RequirePolicyEvaluation:       true,
	})
	if err != nil {
		t.Fatalf("resume job-env-override-policy-transition: %v", err)
	}
	if overridden.StatusReasonCode != "resumed_with_env_override_policy_transition" {
		t.Fatalf("unexpected reason code after env override policy transition: %#v", overridden)
	}
}

func TestJobPathAndInvariantHelperValidationBranches(t *testing.T) {
	if _, _, err := jobPaths("", "bad/id"); err == nil {
		t.Fatalf("expected invalid job_id error for path separator")
	}

	if requiresApprovalBeforeResume(nil) {
		t.Fatalf("nil state should not require approval")
	}
	if got := countDecisionCheckpoints(nil); got != 0 {
		t.Fatalf("expected zero checkpoints for nil state, got %d", got)
	}

	ensureSafetyInvariantLedger(nil)
	state := JobState{}
	ensureSafetyInvariantLedger(&state)
	if state.SafetyInvariantVersion != "1" || state.SafetyInvariantHash == "" || len(state.SafetyInvariants) == 0 {
		t.Fatalf("expected default invariant ledger fields to be populated: %#v", state)
	}

	withBlanks := hashSafetyInvariants([]string{"keep", "", " "})
	withoutBlanks := hashSafetyInvariants([]string{"keep"})
	if withBlanks != withoutBlanks {
		t.Fatalf("expected blank invariants to be ignored, with=%s without=%s", withBlanks, withoutBlanks)
	}
}
