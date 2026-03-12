package jobruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
)

const (
	jobSchemaID      = "gait.job.runtime"
	jobSchemaVersion = "1.0.0"
	eventSchemaID    = "gait.job.event"
	pendingSchemaID  = "gait.job.pending_mutation"
)

const (
	StatusRunning        = "running"
	StatusPaused         = "paused"
	StatusDecisionNeeded = "decision_needed"
	StatusBlocked        = "blocked"
	StatusCompleted      = "completed"
	StatusCancelled      = "cancelled"
	StatusEmergencyStop  = "emergency_stopped"
)

const (
	StopReasonNone                   = "none"
	StopReasonPausedByUser           = "paused_by_user"
	StopReasonDecisionNeeded         = "decision_needed"
	StopReasonBlocked                = "blocked"
	StopReasonCompleted              = "completed"
	StopReasonCancelledByUser        = "cancelled_by_user"
	StopReasonEmergencyStopped       = "emergency_stopped"
	StopReasonEnvFingerprintMismatch = "env_fingerprint_mismatch"
)

const (
	CheckpointTypePlan           = "plan"
	CheckpointTypeProgress       = "progress"
	CheckpointTypeDecisionNeeded = "decision-needed"
	CheckpointTypeBlocked        = "blocked"
	CheckpointTypeCompleted      = "completed"
)

var (
	ErrJobNotFound               = errors.New("job not found")
	ErrInvalidTransition         = errors.New("invalid transition")
	ErrApprovalRequired          = errors.New("approval required")
	ErrEnvironmentMismatch       = errors.New("environment fingerprint mismatch")
	ErrInvalidCheckpoint         = errors.New("invalid checkpoint")
	ErrStateContention           = errors.New("state contention")
	ErrPolicyEvaluationRequired  = errors.New("policy evaluation required")
	ErrIdentityValidationMissing = errors.New("identity validation required")
	ErrIdentityRevoked           = errors.New("identity revoked")
	ErrIdentityBindingMismatch   = errors.New("identity binding mismatch")
)

var (
	persistJobJSON  = writeJSON
	persistJobEvent = appendEvent
	removeJobPath   = os.Remove
)

var safeJobIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type JobState struct {
	SchemaID               string       `json:"schema_id"`
	SchemaVersion          string       `json:"schema_version"`
	CreatedAt              time.Time    `json:"created_at"`
	UpdatedAt              time.Time    `json:"updated_at"`
	ProducerVersion        string       `json:"producer_version"`
	JobID                  string       `json:"job_id"`
	Status                 string       `json:"status"`
	StopReason             string       `json:"stop_reason"`
	StatusReasonCode       string       `json:"status_reason_code"`
	EnvironmentFingerprint string       `json:"environment_fingerprint"`
	SafetyInvariantVersion string       `json:"safety_invariant_version,omitempty"`
	SafetyInvariantHash    string       `json:"safety_invariant_hash,omitempty"`
	SafetyInvariants       []string     `json:"safety_invariants,omitempty"`
	PolicyDigest           string       `json:"policy_digest,omitempty"`
	PolicyRef              string       `json:"policy_ref,omitempty"`
	Identity               string       `json:"identity,omitempty"`
	Revision               int64        `json:"revision"`
	Checkpoints            []Checkpoint `json:"checkpoints"`
	Approvals              []Approval   `json:"approvals,omitempty"`
}

type Checkpoint struct {
	CheckpointID   string    `json:"checkpoint_id"`
	CreatedAt      time.Time `json:"created_at"`
	Type           string    `json:"type"`
	Summary        string    `json:"summary"`
	RequiredAction string    `json:"required_action,omitempty"`
	ReasonCode     string    `json:"reason_code"`
	Actor          string    `json:"actor,omitempty"`
}

type Approval struct {
	CreatedAt time.Time `json:"created_at"`
	Actor     string    `json:"actor"`
	Reason    string    `json:"reason"`
}

type Event struct {
	SchemaID      string         `json:"schema_id"`
	SchemaVersion string         `json:"schema_version"`
	CreatedAt     time.Time      `json:"created_at"`
	JobID         string         `json:"job_id"`
	Revision      int64          `json:"revision"`
	Type          string         `json:"type"`
	Actor         string         `json:"actor,omitempty"`
	ReasonCode    string         `json:"reason_code,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type pendingMutation struct {
	SchemaID      string    `json:"schema_id"`
	SchemaVersion string    `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	JobID         string    `json:"job_id"`
	PreviousState *JobState `json:"previous_state,omitempty"`
	UpdatedState  JobState  `json:"updated_state"`
	Event         Event     `json:"event"`
}

type DurableStateIssue struct {
	JobID   string `json:"job_id"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type jobFiles struct {
	statePath   string
	eventsPath  string
	pendingPath string
	lockPath    string
}

type SubmitOptions struct {
	JobID                  string
	ProducerVersion        string
	EnvironmentFingerprint string
	PolicyDigest           string
	PolicyRef              string
	Identity               string
	Actor                  string
	Now                    time.Time
}

type CheckpointOptions struct {
	Type           string
	Summary        string
	RequiredAction string
	Actor          string
	Now            time.Time
}

type ResumeOptions struct {
	CurrentEnvironmentFingerprint string
	AllowEnvironmentMismatch      bool
	PolicyDigest                  string
	PolicyRef                     string
	RequirePolicyEvaluation       bool
	Identity                      string
	RequireIdentityValidation     bool
	IdentityValidationSource      string
	IdentityRevoked               bool
	Reason                        string
	Actor                         string
	Now                           time.Time
}

type ApprovalOptions struct {
	Actor  string
	Reason string
	Now    time.Time
}

type TransitionOptions struct {
	Actor string
	Now   time.Time
}

type DispatchRecordOptions struct {
	Actor        string
	DispatchPath string
	ReasonCode   string
	Now          time.Time
}

func Submit(root string, opts SubmitOptions) (JobState, error) {
	jobID := strings.TrimSpace(opts.JobID)
	if jobID == "" {
		return JobState{}, fmt.Errorf("job_id is required")
	}
	now := normalizeNow(opts.Now)
	producer := strings.TrimSpace(opts.ProducerVersion)
	if producer == "" {
		producer = "0.0.0-dev"
	}
	envfp := strings.TrimSpace(opts.EnvironmentFingerprint)
	if envfp == "" {
		envfp = EnvironmentFingerprint("")
	}
	policyDigest := strings.TrimSpace(opts.PolicyDigest)
	policyRef := strings.TrimSpace(opts.PolicyRef)
	identity := strings.TrimSpace(opts.Identity)
	if identity == "" {
		identity = strings.TrimSpace(opts.Actor)
	}
	files, err := resolveJobFiles(root, jobID)
	if err != nil {
		return JobState{}, err
	}
	if err := os.MkdirAll(filepath.Dir(files.statePath), 0o750); err != nil {
		return JobState{}, fmt.Errorf("create job directory: %w", err)
	}

	release, err := acquireLock(files.lockPath, now, 2*time.Second)
	if err != nil {
		return JobState{}, err
	}
	defer release()

	if err := recoverPendingMutation(files); err != nil {
		return JobState{}, err
	}

	if _, err := os.Stat(files.statePath); err == nil {
		return JobState{}, fmt.Errorf("job already exists: %s", jobID)
	} else if err != nil && !os.IsNotExist(err) {
		return JobState{}, fmt.Errorf("stat job state: %w", err)
	}

	state := JobState{
		SchemaID:               jobSchemaID,
		SchemaVersion:          jobSchemaVersion,
		CreatedAt:              now,
		UpdatedAt:              now,
		ProducerVersion:        producer,
		JobID:                  jobID,
		Status:                 StatusRunning,
		StopReason:             StopReasonNone,
		StatusReasonCode:       "submitted",
		EnvironmentFingerprint: envfp,
		SafetyInvariantVersion: "1",
		PolicyDigest:           policyDigest,
		PolicyRef:              policyRef,
		Identity:               identity,
		Revision:               1,
		Checkpoints:            []Checkpoint{},
	}
	state.SafetyInvariants = deriveSafetyInvariants(state)
	state.SafetyInvariantHash = hashSafetyInvariants(state.SafetyInvariants)
	eventPayload := map[string]any{
		"environment_fingerprint": envfp,
	}
	if policyDigest != "" {
		eventPayload["policy_digest"] = policyDigest
	}
	if policyRef != "" {
		eventPayload["policy_ref"] = policyRef
	}
	if identity != "" {
		eventPayload["identity"] = identity
	}
	event := Event{
		SchemaID:      eventSchemaID,
		SchemaVersion: jobSchemaVersion,
		CreatedAt:     now,
		JobID:         jobID,
		Revision:      state.Revision,
		Type:          "submitted",
		Actor:         strings.TrimSpace(opts.Actor),
		ReasonCode:    "submitted",
		Payload:       eventPayload,
	}
	if err := commitMutation(files, pendingMutation{
		SchemaID:      pendingSchemaID,
		SchemaVersion: jobSchemaVersion,
		CreatedAt:     now,
		JobID:         jobID,
		UpdatedState:  state,
		Event:         event,
	}); err != nil {
		return JobState{}, err
	}
	return state, nil
}

func Status(root string, jobID string) (JobState, error) {
	files, err := resolveJobFiles(root, jobID)
	if err != nil {
		return JobState{}, err
	}
	if err := recoverJobIfNeeded(files); err != nil {
		return JobState{}, err
	}
	state, err := readState(files.statePath)
	if err != nil {
		return JobState{}, err
	}
	return state, nil
}

func ListCheckpoints(root string, jobID string) ([]Checkpoint, error) {
	state, err := Status(root, jobID)
	if err != nil {
		return nil, err
	}
	checkpoints := append([]Checkpoint{}, state.Checkpoints...)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].CheckpointID < checkpoints[j].CheckpointID
	})
	return checkpoints, nil
}

func GetCheckpoint(root string, jobID string, checkpointID string) (Checkpoint, error) {
	checkpoints, err := ListCheckpoints(root, jobID)
	if err != nil {
		return Checkpoint{}, err
	}
	needle := strings.TrimSpace(checkpointID)
	for _, checkpoint := range checkpoints {
		if checkpoint.CheckpointID == needle {
			return checkpoint, nil
		}
	}
	return Checkpoint{}, fmt.Errorf("checkpoint not found: %s", needle)
}

func AddCheckpoint(root string, jobID string, opts CheckpointOptions) (JobState, Checkpoint, error) {
	var emitted Checkpoint
	updated, err := mutate(root, jobID, func(state *JobState, now time.Time) (Event, error) {
		typeValue := strings.TrimSpace(opts.Type)
		if !isCheckpointType(typeValue) {
			return Event{}, fmt.Errorf("%w: type must be one of plan|progress|decision-needed|blocked|completed", ErrInvalidCheckpoint)
		}
		summary := strings.TrimSpace(opts.Summary)
		if summary == "" {
			return Event{}, fmt.Errorf("%w: summary is required", ErrInvalidCheckpoint)
		}
		if len(summary) > 512 {
			return Event{}, fmt.Errorf("%w: summary exceeds max length 512", ErrInvalidCheckpoint)
		}
		requiredAction := strings.TrimSpace(opts.RequiredAction)
		if typeValue == CheckpointTypeDecisionNeeded && requiredAction == "" {
			return Event{}, fmt.Errorf("%w: required_action is required for decision-needed checkpoints", ErrInvalidCheckpoint)
		}
		if typeValue != CheckpointTypeDecisionNeeded {
			requiredAction = ""
		}

		checkpoint := Checkpoint{
			CheckpointID:   fmt.Sprintf("cp_%04d", len(state.Checkpoints)+1),
			CreatedAt:      now,
			Type:           typeValue,
			Summary:        summary,
			RequiredAction: requiredAction,
			ReasonCode:     checkpointReasonCode(typeValue),
			Actor:          strings.TrimSpace(opts.Actor),
		}
		state.Checkpoints = append(state.Checkpoints, checkpoint)
		emitted = checkpoint

		switch typeValue {
		case CheckpointTypeDecisionNeeded:
			state.Status = StatusDecisionNeeded
			state.StopReason = StopReasonDecisionNeeded
			state.StatusReasonCode = "decision_needed"
		case CheckpointTypeBlocked:
			state.Status = StatusBlocked
			state.StopReason = StopReasonBlocked
			state.StatusReasonCode = "blocked"
		case CheckpointTypeCompleted:
			state.Status = StatusCompleted
			state.StopReason = StopReasonCompleted
			state.StatusReasonCode = "completed"
		}

		return Event{
			Type:       "checkpoint_added",
			Actor:      strings.TrimSpace(opts.Actor),
			ReasonCode: checkpoint.ReasonCode,
			Payload: map[string]any{
				"checkpoint_id":   checkpoint.CheckpointID,
				"checkpoint_type": checkpoint.Type,
				"required_action": checkpoint.RequiredAction,
			},
		}, nil
	}, opts.Now)
	if err != nil {
		return JobState{}, Checkpoint{}, err
	}
	return updated, emitted, nil
}

func Pause(root string, jobID string, opts TransitionOptions) (JobState, error) {
	return simpleTransition(root, jobID, opts.Now, opts.Actor, "paused", []string{StatusRunning, StatusDecisionNeeded}, StatusPaused, StopReasonPausedByUser, "paused")
}

func Cancel(root string, jobID string, opts TransitionOptions) (JobState, error) {
	return simpleTransition(root, jobID, opts.Now, opts.Actor, "cancelled", []string{StatusRunning, StatusPaused, StatusDecisionNeeded, StatusBlocked, StatusEmergencyStop}, StatusCancelled, StopReasonCancelledByUser, "cancelled")
}

func EmergencyStop(root string, jobID string, opts TransitionOptions) (JobState, error) {
	return simpleTransition(
		root,
		jobID,
		opts.Now,
		opts.Actor,
		"emergency_stop_acknowledged",
		[]string{StatusRunning, StatusPaused, StatusDecisionNeeded, StatusBlocked},
		StatusEmergencyStop,
		StopReasonEmergencyStopped,
		"emergency_stop_preempted",
	)
}

func RecordBlockedDispatch(root string, jobID string, opts DispatchRecordOptions) (JobState, error) {
	return mutateWithResult(root, jobID, opts.Now, func(state *JobState, now time.Time) (JobState, Event, error) {
		reasonCode := strings.TrimSpace(opts.ReasonCode)
		if reasonCode == "" {
			reasonCode = "emergency_stop_preempted"
		}
		dispatchPath := strings.TrimSpace(opts.DispatchPath)
		if dispatchPath == "" {
			dispatchPath = "runtime.dispatch"
		}
		if !IsEmergencyStopped(*state) {
			return JobState{}, Event{}, fmt.Errorf("%w: blocked dispatch requires emergency stopped state", ErrInvalidTransition)
		}
		updated := *state
		return updated, Event{
			Type:       "dispatch_blocked",
			Actor:      strings.TrimSpace(opts.Actor),
			ReasonCode: reasonCode,
			Payload: map[string]any{
				"dispatch_path": dispatchPath,
			},
		}, nil
	})
}

func IsEmergencyStopped(state JobState) bool {
	return strings.TrimSpace(state.Status) == StatusEmergencyStop && strings.TrimSpace(state.StopReason) == StopReasonEmergencyStopped
}

func Approve(root string, jobID string, opts ApprovalOptions) (JobState, error) {
	return mutateWithResult(root, jobID, opts.Now, func(state *JobState, now time.Time) (JobState, Event, error) {
		actor := strings.TrimSpace(opts.Actor)
		if actor == "" {
			return JobState{}, Event{}, fmt.Errorf("approval actor is required")
		}
		reason := strings.TrimSpace(opts.Reason)
		if reason == "" {
			reason = "approved"
		}
		state.Approvals = append(state.Approvals, Approval{
			CreatedAt: now,
			Actor:     actor,
			Reason:    reason,
		})
		state.StatusReasonCode = "approval_recorded"
		updated := *state
		return updated, Event{
			Type:       "approved",
			Actor:      actor,
			ReasonCode: "approval_recorded",
			Payload: map[string]any{
				"reason": reason,
			},
		}, nil
	})
}

func Resume(root string, jobID string, opts ResumeOptions) (JobState, error) {
	return mutateWithResult(root, jobID, opts.Now, func(state *JobState, now time.Time) (JobState, Event, error) {
		if state.Status != StatusPaused && state.Status != StatusDecisionNeeded && state.Status != StatusBlocked {
			return JobState{}, Event{}, fmt.Errorf("%w: resume requires paused, decision_needed, or blocked state", ErrInvalidTransition)
		}
		if requiresApprovalBeforeResume(state) {
			return JobState{}, Event{}, fmt.Errorf("%w: approval required before resume", ErrApprovalRequired)
		}

		current := strings.TrimSpace(opts.CurrentEnvironmentFingerprint)
		if current == "" {
			current = EnvironmentFingerprint("")
		}
		reason := strings.TrimSpace(opts.Reason)
		if reason == "" {
			reason = "resume"
		}
		previousPolicyDigest := strings.TrimSpace(state.PolicyDigest)
		previousPolicyRef := strings.TrimSpace(state.PolicyRef)
		ensureSafetyInvariantLedger(state)
		policyDigest := strings.TrimSpace(opts.PolicyDigest)
		policyRef := strings.TrimSpace(opts.PolicyRef)
		policyEvaluationRequired := opts.RequirePolicyEvaluation || previousPolicyDigest != "" || previousPolicyRef != ""
		if policyEvaluationRequired && policyDigest == "" {
			return JobState{}, Event{}, fmt.Errorf("%w: policy digest is required when policy evaluation is enabled", ErrPolicyEvaluationRequired)
		}
		policyChanged := false
		if policyDigest != "" {
			policyChanged = previousPolicyDigest != "" && previousPolicyDigest != policyDigest
			state.PolicyDigest = policyDigest
		}
		if policyRef != "" {
			state.PolicyRef = policyRef
		}
		boundIdentity := strings.TrimSpace(state.Identity)
		providedIdentity := strings.TrimSpace(opts.Identity)
		if boundIdentity != "" && providedIdentity != "" && providedIdentity != boundIdentity {
			return JobState{}, Event{}, fmt.Errorf("%w: expected=%s provided=%s", ErrIdentityBindingMismatch, boundIdentity, providedIdentity)
		}
		identity := boundIdentity
		if identity == "" {
			identity = providedIdentity
		}
		identityValidationRequired := opts.RequireIdentityValidation || boundIdentity != ""
		if identityValidationRequired && identity == "" {
			return JobState{}, Event{}, fmt.Errorf("%w: identity is required for resume validation", ErrIdentityValidationMissing)
		}
		if identity != "" && opts.IdentityRevoked {
			return JobState{}, Event{}, fmt.Errorf("%w: %s", ErrIdentityRevoked, identity)
		}
		if boundIdentity == "" && identity != "" {
			state.Identity = identity
		}
		reasonCode := "resumed"
		if policyChanged {
			reasonCode = "resumed_with_policy_transition"
		}
		payload := buildResumePayload(resumePayloadOptions{
			Reason:                     reason,
			ExpectedFingerprint:        state.EnvironmentFingerprint,
			ActualFingerprint:          current,
			PolicyEvaluationRequired:   policyEvaluationRequired,
			PreviousPolicyDigest:       previousPolicyDigest,
			CurrentPolicyDigest:        strings.TrimSpace(state.PolicyDigest),
			PreviousPolicyRef:          previousPolicyRef,
			CurrentPolicyRef:           strings.TrimSpace(state.PolicyRef),
			PolicyChanged:              policyChanged,
			Identity:                   identity,
			IdentityValidationRequired: identityValidationRequired,
			IdentityValidationSource:   strings.TrimSpace(opts.IdentityValidationSource),
		})
		if state.EnvironmentFingerprint != "" && current != state.EnvironmentFingerprint {
			if !opts.AllowEnvironmentMismatch {
				return JobState{}, Event{}, fmt.Errorf("%w: expected=%s actual=%s", ErrEnvironmentMismatch, state.EnvironmentFingerprint, current)
			}
			if policyChanged {
				reasonCode = "resumed_with_env_override_policy_transition"
			} else {
				reasonCode = "resumed_with_env_override"
			}
			state.StatusReasonCode = reasonCode
			state.StopReason = StopReasonNone
			state.Status = StatusRunning
			updated := *state
			return updated, Event{
				Type:       "resumed",
				Actor:      strings.TrimSpace(opts.Actor),
				ReasonCode: reasonCode,
				Payload:    payload,
			}, nil
		}

		state.Status = StatusRunning
		state.StopReason = StopReasonNone
		state.StatusReasonCode = reasonCode
		updated := *state
		return updated, Event{
			Type:       "resumed",
			Actor:      strings.TrimSpace(opts.Actor),
			ReasonCode: reasonCode,
			Payload:    payload,
		}, nil
	})
}

func Inspect(root string, jobID string) (JobState, []Event, error) {
	files, err := resolveJobFiles(root, jobID)
	if err != nil {
		return JobState{}, nil, err
	}
	if err := recoverJobIfNeeded(files); err != nil {
		return JobState{}, nil, err
	}
	state, err := readState(files.statePath)
	if err != nil {
		return JobState{}, nil, err
	}
	events, err := readEvents(files.eventsPath)
	if err != nil {
		return JobState{}, nil, err
	}
	return state, events, nil
}

func simpleTransition(root string, jobID string, now time.Time, actor string, eventType string, allowedFrom []string, nextStatus string, nextStopReason string, reasonCode string) (JobState, error) {
	return mutateWithResult(root, jobID, now, func(state *JobState, _ time.Time) (JobState, Event, error) {
		if !contains(allowedFrom, state.Status) {
			return JobState{}, Event{}, fmt.Errorf("%w: %s from %s", ErrInvalidTransition, eventType, state.Status)
		}
		state.Status = nextStatus
		state.StopReason = nextStopReason
		state.StatusReasonCode = reasonCode
		updated := *state
		return updated, Event{
			Type:       eventType,
			Actor:      strings.TrimSpace(actor),
			ReasonCode: reasonCode,
		}, nil
	})
}

type resumePayloadOptions struct {
	Reason                     string
	ExpectedFingerprint        string
	ActualFingerprint          string
	PolicyEvaluationRequired   bool
	PreviousPolicyDigest       string
	CurrentPolicyDigest        string
	PreviousPolicyRef          string
	CurrentPolicyRef           string
	PolicyChanged              bool
	Identity                   string
	IdentityValidationRequired bool
	IdentityValidationSource   string
}

func buildResumePayload(options resumePayloadOptions) map[string]any {
	payload := map[string]any{
		"reason": options.Reason,
	}
	if options.ExpectedFingerprint != "" {
		payload["expected_fingerprint"] = options.ExpectedFingerprint
	}
	if options.ActualFingerprint != "" {
		payload["actual_fingerprint"] = options.ActualFingerprint
	}
	if options.PolicyEvaluationRequired {
		payload["policy_evaluation_required"] = true
	}
	if options.PreviousPolicyDigest != "" || options.CurrentPolicyDigest != "" {
		payload["previous_policy_digest"] = options.PreviousPolicyDigest
		payload["current_policy_digest"] = options.CurrentPolicyDigest
		payload["policy_changed"] = options.PolicyChanged
	}
	if options.PreviousPolicyRef != "" || options.CurrentPolicyRef != "" {
		payload["previous_policy_ref"] = options.PreviousPolicyRef
		payload["current_policy_ref"] = options.CurrentPolicyRef
	}
	if options.IdentityValidationRequired {
		payload["identity_validation_required"] = true
	}
	if options.Identity != "" {
		payload["identity"] = options.Identity
	}
	if options.IdentityValidationSource != "" {
		payload["identity_validation_source"] = options.IdentityValidationSource
	}
	return payload
}

func mutate(root string, jobID string, mutator func(*JobState, time.Time) (Event, error), now time.Time) (JobState, error) {
	return mutateWithResult(root, jobID, now, func(state *JobState, ts time.Time) (JobState, Event, error) {
		event, err := mutator(state, ts)
		if err != nil {
			return JobState{}, Event{}, err
		}
		updated := *state
		return updated, event, nil
	})
}

func mutateWithResult(root string, jobID string, now time.Time, mutator func(*JobState, time.Time) (JobState, Event, error)) (JobState, error) {
	files, err := resolveJobFiles(root, jobID)
	if err != nil {
		return JobState{}, err
	}

	release, err := acquireLock(files.lockPath, normalizeNow(now), 2*time.Second)
	if err != nil {
		return JobState{}, err
	}
	defer release()

	if err := recoverPendingMutation(files); err != nil {
		return JobState{}, err
	}

	state, err := readState(files.statePath)
	if err != nil {
		return JobState{}, err
	}
	previous := state

	ts := normalizeNow(now)
	updated, event, err := mutator(&state, ts)
	if err != nil {
		return JobState{}, err
	}
	updated.UpdatedAt = ts
	updated.Revision = previous.Revision + 1
	event.SchemaID = eventSchemaID
	event.SchemaVersion = jobSchemaVersion
	event.CreatedAt = ts
	event.JobID = previous.JobID
	event.Revision = updated.Revision

	if err := commitMutation(files, pendingMutation{
		SchemaID:      pendingSchemaID,
		SchemaVersion: jobSchemaVersion,
		CreatedAt:     ts,
		JobID:         previous.JobID,
		PreviousState: &previous,
		UpdatedState:  updated,
		Event:         event,
	}); err != nil {
		return JobState{}, err
	}
	return updated, nil
}

func DiagnoseDurableState(root string) ([]DurableStateIssue, error) {
	cleanRoot := strings.TrimSpace(root)
	if cleanRoot == "" {
		cleanRoot = filepath.Join(".", "gait-out", "jobs")
	}
	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve job root: %w", err)
	}
	entries, err := os.ReadDir(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []DurableStateIssue{}, nil
		}
		return nil, fmt.Errorf("read job root: %w", err)
	}

	issues := make([]DurableStateIssue, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !safeJobIDPattern.MatchString(entry.Name()) {
			continue
		}
		files, err := resolveJobFiles(absRoot, entry.Name())
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(files.pendingPath); err == nil {
			issues = append(issues, DurableStateIssue{
				JobID:   entry.Name(),
				Kind:    "pending_mutation",
				Message: "pending durable mutation marker requires reconciliation",
			})
		} else if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat pending mutation for %s: %w", entry.Name(), err)
		}

		state, stateErr := readState(files.statePath)
		if stateErr != nil && !errors.Is(stateErr, ErrJobNotFound) {
			return nil, fmt.Errorf("read durable state for %s: %w", entry.Name(), stateErr)
		}
		events, err := readEvents(files.eventsPath)
		if err != nil {
			return nil, fmt.Errorf("read durable events for %s: %w", entry.Name(), err)
		}

		lastRevision := int64(0)
		if len(events) > 0 {
			lastRevision = events[len(events)-1].Revision
		}
		if errors.Is(stateErr, ErrJobNotFound) {
			if len(events) > 0 {
				issues = append(issues, DurableStateIssue{
					JobID:   entry.Name(),
					Kind:    "missing_state_with_events",
					Message: fmt.Sprintf("event log revision %d exists without a durable state snapshot", lastRevision),
				})
			}
			continue
		}
		if state.Revision > lastRevision {
			issues = append(issues, DurableStateIssue{
				JobID:   entry.Name(),
				Kind:    "state_ahead_of_event_log",
				Message: fmt.Sprintf("state revision %d is ahead of event log revision %d", state.Revision, lastRevision),
			})
		}
		if lastRevision > state.Revision {
			issues = append(issues, DurableStateIssue{
				JobID:   entry.Name(),
				Kind:    "event_log_ahead_of_state",
				Message: fmt.Sprintf("event log revision %d is ahead of state revision %d", lastRevision, state.Revision),
			})
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].JobID == issues[j].JobID {
			return issues[i].Kind < issues[j].Kind
		}
		return issues[i].JobID < issues[j].JobID
	})
	return issues, nil
}

func readState(path string) (JobState, error) {
	// #nosec G304 -- path is derived from local job root
	// lgtm[go/path-injection] path is derived from explicit local runtime root/job id inputs.
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return JobState{}, ErrJobNotFound
		}
		return JobState{}, fmt.Errorf("read job state: %w", err)
	}
	var state JobState
	if err := json.Unmarshal(payload, &state); err != nil {
		return JobState{}, fmt.Errorf("parse job state: %w", err)
	}
	if strings.TrimSpace(state.JobID) == "" {
		return JobState{}, fmt.Errorf("invalid job state: missing job_id")
	}
	ensureSafetyInvariantLedger(&state)
	if state.Checkpoints == nil {
		state.Checkpoints = []Checkpoint{}
	}
	return state, nil
}

func readEvents(path string) ([]Event, error) {
	// #nosec G304 -- path is derived from local job root
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, fmt.Errorf("read job events: %w", err)
	}
	lines := strings.Split(string(payload), "\n")
	events := make([]Event, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(trimmed), &event); err != nil {
			return nil, fmt.Errorf("parse job event: %w", err)
		}
		events = append(events, event)
	}
	return events, nil
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	payload = append(payload, '\n')
	if err := fsx.WriteFileAtomic(path, payload, 0o600); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

func appendEvent(path string, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	if err := fsx.AppendLineLocked(path, payload, 0o600); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

func resolveJobFiles(root string, jobID string) (jobFiles, error) {
	statePath, eventsPath, err := jobPaths(root, jobID)
	if err != nil {
		return jobFiles{}, err
	}
	return jobFiles{
		statePath:   statePath,
		eventsPath:  eventsPath,
		pendingPath: filepath.Join(filepath.Dir(statePath), "pending_mutation.json"),
		lockPath:    statePath + ".lock",
	}, nil
}

func recoverJobIfNeeded(files jobFiles) error {
	if _, err := os.Stat(files.pendingPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat pending mutation: %w", err)
	}
	release, err := acquireLock(files.lockPath, time.Time{}, 2*time.Second)
	if err != nil {
		return err
	}
	defer release()
	return recoverPendingMutation(files)
}

func commitMutation(files jobFiles, mutation pendingMutation) error {
	if err := persistJobJSON(files.pendingPath, mutation); err != nil {
		return err
	}
	if err := materializePendingState(files.statePath, mutation.UpdatedState); err != nil {
		_ = removePendingMarker(files.pendingPath)
		return err
	}
	if err := persistJobEvent(files.eventsPath, mutation.Event); err != nil {
		if rollbackErr := restorePendingBaseline(files.statePath, mutation.PreviousState); rollbackErr != nil {
			return fmt.Errorf("append event: %w (rollback failed: %v)", err, rollbackErr)
		}
		_ = removePendingMarker(files.pendingPath)
		return err
	}
	_ = removePendingMarker(files.pendingPath)
	return nil
}

func recoverPendingMutation(files jobFiles) error {
	mutation, err := readPendingMutation(files.pendingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	events, err := readEvents(files.eventsPath)
	if err != nil {
		return err
	}
	if pendingEventExists(events, mutation.Event.Revision) {
		if err := materializePendingState(files.statePath, mutation.UpdatedState); err != nil {
			return err
		}
	} else if err := restorePendingBaseline(files.statePath, mutation.PreviousState); err != nil {
		return err
	}
	_ = removePendingMarker(files.pendingPath)
	return nil
}

func readPendingMutation(path string) (pendingMutation, error) {
	// #nosec G304 -- path is derived from local job root
	// lgtm[go/path-injection] path is derived from explicit local runtime root/job id inputs.
	payload, err := os.ReadFile(path)
	if err != nil {
		return pendingMutation{}, err
	}
	var mutation pendingMutation
	if err := json.Unmarshal(payload, &mutation); err != nil {
		return pendingMutation{}, fmt.Errorf("parse pending mutation: %w", err)
	}
	if strings.TrimSpace(mutation.JobID) == "" {
		return pendingMutation{}, fmt.Errorf("invalid pending mutation: missing job_id")
	}
	return mutation, nil
}

func pendingEventExists(events []Event, revision int64) bool {
	for _, event := range events {
		if event.Revision == revision {
			return true
		}
	}
	return false
}

func materializePendingState(path string, desired JobState) error {
	current, err := readState(path)
	if err == nil && reflect.DeepEqual(current, desired) {
		return nil
	}
	if err != nil && !errors.Is(err, ErrJobNotFound) {
		return err
	}
	return persistJobJSON(path, desired)
}

func restorePendingBaseline(path string, previous *JobState) error {
	if previous == nil {
		if err := removeJobPath(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove job state: %w", err)
		}
		return nil
	}
	return materializePendingState(path, *previous)
}

func removePendingMarker(path string) error {
	if err := removeJobPath(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pending mutation: %w", err)
	}
	return nil
}

func jobPaths(root string, jobID string) (statePath string, eventsPath string, err error) {
	cleanRoot := strings.TrimSpace(root)
	if cleanRoot == "" {
		cleanRoot = filepath.Join(".", "gait-out", "jobs")
	}
	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve job root: %w", err)
	}
	cleanID := strings.TrimSpace(jobID)
	if !safeJobIDPattern.MatchString(cleanID) {
		return "", "", fmt.Errorf("job_id must match %s", safeJobIDPattern.String())
	}
	jobDir := filepath.Join(absRoot, cleanID)
	relPath, err := filepath.Rel(absRoot, jobDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve job path: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("job path escapes root")
	}
	return filepath.Join(jobDir, "state.json"), filepath.Join(jobDir, "events.jsonl"), nil
}

func acquireLock(path string, _ time.Time, timeout time.Duration) (func(), error) {
	start := time.Now().UTC()
	for {
		// #nosec G304 -- lock path derived from local root
		// lgtm[go/path-injection] lock path is derived from explicit local runtime root/job id inputs.
		fd, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = fd.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquire lock: %w", err)
		}
		now := time.Now().UTC()
		if now.Sub(start) > timeout {
			return nil, fmt.Errorf("%w: lock timeout", ErrStateContention)
		}
		if staleLock(path, now, 30*time.Second) {
			// lgtm[go/path-injection] lock path is derived from explicit local runtime root/job id inputs.
			_ = os.Remove(path)
			continue
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func staleLock(path string, now time.Time, staleAfter time.Duration) bool {
	// #nosec G304 -- lock path derived from local root
	// lgtm[go/path-injection] lock path is derived from explicit local runtime root/job id inputs.
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return now.Sub(info.ModTime().UTC()) > staleAfter
}

func normalizeNow(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func isCheckpointType(value string) bool {
	switch value {
	case CheckpointTypePlan, CheckpointTypeProgress, CheckpointTypeDecisionNeeded, CheckpointTypeBlocked, CheckpointTypeCompleted:
		return true
	default:
		return false
	}
}

func checkpointReasonCode(value string) string {
	switch value {
	case CheckpointTypePlan:
		return "checkpoint_plan"
	case CheckpointTypeProgress:
		return "checkpoint_progress"
	case CheckpointTypeDecisionNeeded:
		return "checkpoint_decision_needed"
	case CheckpointTypeBlocked:
		return "checkpoint_blocked"
	case CheckpointTypeCompleted:
		return "checkpoint_completed"
	default:
		return "checkpoint"
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func hasPendingDecision(state *JobState) bool {
	if state == nil {
		return false
	}
	return countDecisionCheckpoints(state) > 0
}

func requiresApprovalBeforeResume(state *JobState) bool {
	if state == nil {
		return false
	}
	decisionCount := countDecisionCheckpoints(state)
	if decisionCount > len(state.Approvals) {
		return true
	}
	return state.Status == StatusDecisionNeeded && decisionCount == 0
}

func countDecisionCheckpoints(state *JobState) int {
	if state == nil {
		return 0
	}
	count := 0
	for index := len(state.Checkpoints) - 1; index >= 0; index-- {
		checkpoint := state.Checkpoints[index]
		if checkpoint.Type == CheckpointTypeDecisionNeeded {
			count++
		}
	}
	return count
}

func EnvironmentFingerprint(override string) string {
	trimmed := strings.TrimSpace(override)
	if trimmed != "" {
		return trimmed
	}
	parts := []string{
		"goos=" + runtime.GOOS,
		"goarch=" + runtime.GOARCH,
		"goversion=" + runtime.Version(),
		"shell=" + strings.TrimSpace(os.Getenv("SHELL")),
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "envfp:" + hex.EncodeToString(sum[:])
}

func ensureSafetyInvariantLedger(state *JobState) {
	if state == nil {
		return
	}
	if strings.TrimSpace(state.SafetyInvariantVersion) == "" {
		state.SafetyInvariantVersion = "1"
	}
	if len(state.SafetyInvariants) == 0 {
		state.SafetyInvariants = deriveSafetyInvariants(*state)
	}
	if strings.TrimSpace(state.SafetyInvariantHash) == "" {
		state.SafetyInvariantHash = hashSafetyInvariants(state.SafetyInvariants)
	}
}

func deriveSafetyInvariants(state JobState) []string {
	values := []string{
		"control_boundary=runtime_go",
		"fail_closed=true",
		"default_privacy=reference_receipts",
	}
	if strings.TrimSpace(state.PolicyDigest) != "" {
		values = append(values, "policy_digest="+strings.TrimSpace(state.PolicyDigest))
	}
	if strings.TrimSpace(state.PolicyRef) != "" {
		values = append(values, "policy_ref="+strings.TrimSpace(state.PolicyRef))
	}
	if strings.TrimSpace(state.Identity) != "" {
		values = append(values, "identity="+strings.TrimSpace(state.Identity))
	}
	sort.Strings(values)
	return values
}

func hashSafetyInvariants(values []string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	sort.Strings(filtered)
	sum := sha256.Sum256([]byte(strings.Join(filtered, "|")))
	return hex.EncodeToString(sum[:])
}
