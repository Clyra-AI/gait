package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/jobruntime"
)

const (
	jobOutputSchemaID      = "gait.job.output"
	jobOutputSchemaVersion = "1.0.0"
)

type jobOutput struct {
	SchemaID      string                  `json:"schema_id"`
	SchemaVersion string                  `json:"schema_version"`
	OK            bool                    `json:"ok"`
	Operation     string                  `json:"operation,omitempty"`
	JobID         string                  `json:"job_id,omitempty"`
	Job           *jobruntime.JobState    `json:"job,omitempty"`
	Checkpoint    *jobruntime.Checkpoint  `json:"checkpoint,omitempty"`
	Checkpoints   []jobruntime.Checkpoint `json:"checkpoints,omitempty"`
	Events        []jobruntime.Event      `json:"events,omitempty"`
	Error         string                  `json:"error,omitempty"`
}

func runJob(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Manage durable job lifecycle controls with deterministic status transitions, checkpoint interrupts, approvals, and resume gating.")
	}
	if len(arguments) == 0 {
		printJobUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "submit":
		return runJobSubmit(arguments[1:])
	case "status":
		return runJobStatus(arguments[1:])
	case "checkpoint":
		return runJobCheckpoint(arguments[1:])
	case "pause":
		return runJobPause(arguments[1:])
	case "stop":
		return runJobStop(arguments[1:])
	case "approve":
		return runJobApprove(arguments[1:])
	case "resume":
		return runJobResume(arguments[1:])
	case "cancel":
		return runJobCancel(arguments[1:])
	case "inspect":
		return runJobInspect(arguments[1:])
	default:
		printJobUsage()
		return exitInvalidInput
	}
}

func runJobSubmit(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"id":              true,
		"root":            true,
		"actor":           true,
		"identity":        true,
		"env-fingerprint": true,
		"policy":          true,
		"policy-digest":   true,
		"policy-ref":      true,
	})
	flagSet := flag.NewFlagSet("job-submit", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var actor string
	var identity string
	var envFingerprint string
	var policyPath string
	var policyDigest string
	var policyRef string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.StringVar(&actor, "actor", "", "actor identity")
	flagSet.StringVar(&identity, "identity", "", "agent identity bound to the job")
	flagSet.StringVar(&envFingerprint, "env-fingerprint", "", "optional environment fingerprint override")
	flagSet.StringVar(&policyPath, "policy", "", "path to policy yaml used for submit/resume enforcement")
	flagSet.StringVar(&policyDigest, "policy-digest", "", "policy digest override (sha256:...)")
	flagSet.StringVar(&policyRef, "policy-ref", "", "policy reference identifier")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "submit", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printJobSubmitUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "submit", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	resolvedPolicyDigest, resolvedPolicyRef, err := resolveJobPolicyMetadata(policyPath, policyDigest, policyRef)
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "submit", Error: err.Error()}, exitInvalidInput)
	}
	state, err := jobruntime.Submit(root, jobruntime.SubmitOptions{
		JobID:                  strings.TrimSpace(jobID),
		Actor:                  strings.TrimSpace(actor),
		Identity:               strings.TrimSpace(identity),
		ProducerVersion:        version,
		EnvironmentFingerprint: jobruntime.EnvironmentFingerprint(envFingerprint),
		PolicyDigest:           resolvedPolicyDigest,
		PolicyRef:              resolvedPolicyRef,
	})
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "submit", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: "submit", JobID: state.JobID, Job: &state}, exitOK)
}

func runJobStatus(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"id": true, "root": true})
	flagSet := flag.NewFlagSet("job-status", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "status", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printJobStatusUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "status", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	state, err := jobruntime.Status(root, strings.TrimSpace(jobID))
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "status", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: "status", JobID: state.JobID, Job: &state}, exitOK)
}

func runJobCheckpoint(arguments []string) int {
	if len(arguments) == 0 {
		printJobCheckpointUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "add":
		return runJobCheckpointAdd(arguments[1:])
	case "list":
		return runJobCheckpointList(arguments[1:])
	case "show":
		return runJobCheckpointShow(arguments[1:])
	default:
		printJobCheckpointUsage()
		return exitInvalidInput
	}
}

func runJobCheckpointAdd(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"id":              true,
		"root":            true,
		"type":            true,
		"summary":         true,
		"required-action": true,
		"actor":           true,
	})
	flagSet := flag.NewFlagSet("job-checkpoint-add", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var checkpointType string
	var summary string
	var requiredAction string
	var actor string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.StringVar(&checkpointType, "type", "progress", "checkpoint type: plan|progress|decision-needed|blocked|completed")
	flagSet.StringVar(&summary, "summary", "", "checkpoint summary (max 512 chars)")
	flagSet.StringVar(&requiredAction, "required-action", "", "required action for decision-needed checkpoints")
	flagSet.StringVar(&actor, "actor", "", "actor identity")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint add", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printJobCheckpointAddUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint add", Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	state, checkpoint, err := jobruntime.AddCheckpoint(root, strings.TrimSpace(jobID), jobruntime.CheckpointOptions{
		Type:           strings.TrimSpace(checkpointType),
		Summary:        strings.TrimSpace(summary),
		RequiredAction: strings.TrimSpace(requiredAction),
		Actor:          strings.TrimSpace(actor),
	})
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint add", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: "checkpoint add", JobID: state.JobID, Job: &state, Checkpoint: &checkpoint}, exitOK)
}

func runJobCheckpointList(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"id": true, "root": true})
	flagSet := flag.NewFlagSet("job-checkpoint-list", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint list", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printJobCheckpointListUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint list", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	checkpoints, err := jobruntime.ListCheckpoints(root, strings.TrimSpace(jobID))
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint list", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: "checkpoint list", JobID: strings.TrimSpace(jobID), Checkpoints: checkpoints}, exitOK)
}

func runJobCheckpointShow(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"id": true, "root": true, "checkpoint": true})
	flagSet := flag.NewFlagSet("job-checkpoint-show", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var checkpointID string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.StringVar(&checkpointID, "checkpoint", "", "checkpoint identifier")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint show", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printJobCheckpointShowUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint show", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	checkpoint, err := jobruntime.GetCheckpoint(root, strings.TrimSpace(jobID), strings.TrimSpace(checkpointID))
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "checkpoint show", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: "checkpoint show", JobID: strings.TrimSpace(jobID), Checkpoint: &checkpoint}, exitOK)
}

func runJobPause(arguments []string) int {
	return runSimpleJobTransition(arguments, "pause", func(root, jobID, actor string) (jobruntime.JobState, error) {
		return jobruntime.Pause(root, jobID, jobruntime.TransitionOptions{Actor: actor})
	})
}

func runJobCancel(arguments []string) int {
	return runSimpleJobTransition(arguments, "cancel", func(root, jobID, actor string) (jobruntime.JobState, error) {
		return jobruntime.Cancel(root, jobID, jobruntime.TransitionOptions{Actor: actor})
	})
}

func runJobStop(arguments []string) int {
	return runSimpleJobTransition(arguments, "stop", func(root, jobID, actor string) (jobruntime.JobState, error) {
		return jobruntime.EmergencyStop(root, jobID, jobruntime.TransitionOptions{Actor: actor})
	})
}

func runSimpleJobTransition(arguments []string, operation string, action func(root, jobID, actor string) (jobruntime.JobState, error)) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"id": true, "root": true, "actor": true})
	flagSet := flag.NewFlagSet("job-"+operation, flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var actor string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.StringVar(&actor, "actor", "", "actor identity")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: operation, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		switch operation {
		case "pause":
			printJobPauseUsage()
		case "stop":
			printJobStopUsage()
		case "cancel":
			printJobCancelUsage()
		}
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: operation, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	state, err := action(root, strings.TrimSpace(jobID), strings.TrimSpace(actor))
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: operation, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: operation, JobID: state.JobID, Job: &state}, exitOK)
}

func runJobApprove(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"id": true, "root": true, "actor": true, "reason": true})
	flagSet := flag.NewFlagSet("job-approve", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var actor string
	var reason string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.StringVar(&actor, "actor", "", "approver identity")
	flagSet.StringVar(&reason, "reason", "", "approval reason")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "approve", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printJobApproveUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "approve", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	state, err := jobruntime.Approve(root, strings.TrimSpace(jobID), jobruntime.ApprovalOptions{Actor: strings.TrimSpace(actor), Reason: strings.TrimSpace(reason)})
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "approve", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: "approve", JobID: state.JobID, Job: &state}, exitOK)
}

func runJobResume(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"id":                         true,
		"root":                       true,
		"actor":                      true,
		"identity":                   true,
		"reason":                     true,
		"env-fingerprint":            true,
		"policy":                     true,
		"policy-digest":              true,
		"policy-ref":                 true,
		"identity-validation-source": true,
		"identity-revocations":       true,
	})
	flagSet := flag.NewFlagSet("job-resume", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var actor string
	var identity string
	var reason string
	var envFingerprint string
	var policyPath string
	var policyDigest string
	var policyRef string
	var identityValidationSource string
	var identityRevocationsPath string
	var identityRevoked bool
	var allowEnvMismatch bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.StringVar(&actor, "actor", "", "actor identity")
	flagSet.StringVar(&identity, "identity", "", "agent identity to validate on resume")
	flagSet.StringVar(&reason, "reason", "", "resume reason")
	flagSet.StringVar(&envFingerprint, "env-fingerprint", "", "override current environment fingerprint")
	flagSet.StringVar(&policyPath, "policy", "", "path to policy yaml for resume re-evaluation")
	flagSet.StringVar(&policyDigest, "policy-digest", "", "policy digest override (sha256:...)")
	flagSet.StringVar(&policyRef, "policy-ref", "", "policy reference identifier")
	flagSet.StringVar(&identityValidationSource, "identity-validation-source", "", "identity validation source label (for audit payload)")
	flagSet.StringVar(&identityRevocationsPath, "identity-revocations", "", "path to revoked identities file (json array/object or newline list)")
	flagSet.BoolVar(&identityRevoked, "identity-revoked", false, "mark identity as revoked and block resume")
	flagSet.BoolVar(&allowEnvMismatch, "allow-env-mismatch", false, "allow resume when environment fingerprint differs")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "resume", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printJobResumeUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "resume", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	resolvedPolicyDigest, resolvedPolicyRef, err := resolveJobPolicyMetadata(policyPath, policyDigest, policyRef)
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "resume", Error: err.Error()}, exitInvalidInput)
	}
	normalizedJobID := strings.TrimSpace(jobID)
	normalizedIdentity := strings.TrimSpace(identity)
	if strings.TrimSpace(identityRevocationsPath) != "" {
		revokedSet, err := loadRevokedIdentities(identityRevocationsPath)
		if err != nil {
			return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "resume", Error: err.Error()}, exitInvalidInput)
		}
		if normalizedIdentity == "" {
			state, statusErr := jobruntime.Status(root, normalizedJobID)
			if statusErr != nil {
				return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "resume", Error: statusErr.Error()}, exitCodeForError(statusErr, exitInvalidInput))
			}
			normalizedIdentity = strings.TrimSpace(state.Identity)
		}
		if identityIsRevoked(revokedSet, normalizedIdentity) {
			identityRevoked = true
		}
		if strings.TrimSpace(identityValidationSource) == "" {
			identityValidationSource = "revocation_list"
		}
	}
	requirePolicyEvaluation := strings.TrimSpace(policyPath) != "" || strings.TrimSpace(resolvedPolicyDigest) != "" || strings.TrimSpace(resolvedPolicyRef) != ""
	requireIdentityValidation := normalizedIdentity != "" || strings.TrimSpace(identityRevocationsPath) != ""
	state, err := jobruntime.Resume(root, strings.TrimSpace(jobID), jobruntime.ResumeOptions{
		Actor:                         strings.TrimSpace(actor),
		Identity:                      normalizedIdentity,
		Reason:                        strings.TrimSpace(reason),
		CurrentEnvironmentFingerprint: jobruntime.EnvironmentFingerprint(envFingerprint),
		AllowEnvironmentMismatch:      allowEnvMismatch,
		PolicyDigest:                  resolvedPolicyDigest,
		PolicyRef:                     resolvedPolicyRef,
		RequirePolicyEvaluation:       requirePolicyEvaluation,
		RequireIdentityValidation:     requireIdentityValidation,
		IdentityValidationSource:      strings.TrimSpace(identityValidationSource),
		IdentityRevoked:               identityRevoked,
	})
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "resume", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: "resume", JobID: state.JobID, Job: &state}, exitOK)
}

func runJobInspect(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"id": true, "root": true})
	flagSet := flag.NewFlagSet("job-inspect", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "inspect", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printJobInspectUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "inspect", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	state, events, err := jobruntime.Inspect(root, strings.TrimSpace(jobID))
	if err != nil {
		return writeJobOutput(jsonOutput, jobOutput{OK: false, Operation: "inspect", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeJobOutput(jsonOutput, jobOutput{OK: true, Operation: "inspect", JobID: state.JobID, Job: &state, Events: events}, exitOK)
}

func writeJobOutput(jsonOutput bool, output jobOutput, exitCode int) int {
	output.SchemaID = jobOutputSchemaID
	output.SchemaVersion = jobOutputSchemaVersion
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if !output.OK {
		fmt.Printf("job %s error: %s\n", output.Operation, output.Error)
		return exitCode
	}
	if output.Job != nil {
		fmt.Printf("job %s: id=%s status=%s stop_reason=%s reason_code=%s\n", output.Operation, output.Job.JobID, output.Job.Status, output.Job.StopReason, output.Job.StatusReasonCode)
	}
	if output.Checkpoint != nil {
		fmt.Printf("checkpoint: %s (%s)\n", output.Checkpoint.CheckpointID, output.Checkpoint.Type)
	}
	if len(output.Checkpoints) > 0 {
		fmt.Printf("checkpoints=%d\n", len(output.Checkpoints))
	}
	if len(output.Events) > 0 {
		fmt.Printf("events=%d\n", len(output.Events))
	}
	return exitCode
}

func printJobUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job submit --id <job_id> [--root ./gait-out/jobs] [--actor <id>] [--identity <id>] [--policy <policy.yaml>|--policy-digest <sha256>] [--policy-ref <ref>] [--env-fingerprint <value>] [--json] [--explain]")
	fmt.Println("  gait job status --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job checkpoint add --id <job_id> --type <plan|progress|decision-needed|blocked|completed> --summary <text> [--required-action <text>] [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job checkpoint list --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job checkpoint show --id <job_id> --checkpoint <checkpoint_id> [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job pause --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job stop --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job approve --id <job_id> --actor <id> [--reason <text>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job resume --id <job_id> [--actor <id>] [--identity <id>] [--reason <text>] [--policy <policy.yaml>|--policy-digest <sha256>] [--policy-ref <ref>] [--identity-revocations <path>|--identity-revoked] [--identity-validation-source <source>] [--env-fingerprint <value>] [--allow-env-mismatch] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job cancel --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job inspect --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobSubmitUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job submit --id <job_id> [--root ./gait-out/jobs] [--actor <id>] [--identity <id>] [--policy <policy.yaml>|--policy-digest <sha256>] [--policy-ref <ref>] [--env-fingerprint <value>] [--json] [--explain]")
}

func printJobStatusUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job status --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobCheckpointUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job checkpoint add --id <job_id> --type <plan|progress|decision-needed|blocked|completed> --summary <text> [--required-action <text>] [--json] [--explain]")
	fmt.Println("  gait job checkpoint list --id <job_id> [--json] [--explain]")
	fmt.Println("  gait job checkpoint show --id <job_id> --checkpoint <checkpoint_id> [--json] [--explain]")
}

func printJobCheckpointAddUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job checkpoint add --id <job_id> --type <plan|progress|decision-needed|blocked|completed> --summary <text> [--required-action <text>] [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobCheckpointListUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job checkpoint list --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobCheckpointShowUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job checkpoint show --id <job_id> --checkpoint <checkpoint_id> [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobPauseUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job pause --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobStopUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job stop --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobApproveUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job approve --id <job_id> --actor <id> [--reason <text>] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobResumeUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job resume --id <job_id> [--actor <id>] [--identity <id>] [--reason <text>] [--policy <policy.yaml>|--policy-digest <sha256>] [--policy-ref <ref>] [--identity-revocations <path>|--identity-revoked] [--identity-validation-source <source>] [--env-fingerprint <value>] [--allow-env-mismatch] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobCancelUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job cancel --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobInspectUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job inspect --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
}

func resolveJobPolicyMetadata(policyPath string, policyDigest string, policyRef string) (string, string, error) {
	trimmedPolicyPath := strings.TrimSpace(policyPath)
	resolvedDigest := strings.TrimSpace(policyDigest)
	resolvedRef := strings.TrimSpace(policyRef)
	if trimmedPolicyPath != "" {
		policy, err := gate.LoadPolicyFile(trimmedPolicyPath)
		if err != nil {
			return "", "", fmt.Errorf("load policy: %w", err)
		}
		computedDigest, err := gate.PolicyDigest(policy)
		if err != nil {
			return "", "", fmt.Errorf("digest policy: %w", err)
		}
		if resolvedDigest != "" && resolvedDigest != computedDigest {
			return "", "", fmt.Errorf("policy digest mismatch: provided=%s computed=%s", resolvedDigest, computedDigest)
		}
		resolvedDigest = computedDigest
		if resolvedRef == "" {
			resolvedRef = filepath.Clean(trimmedPolicyPath)
		}
	}
	return resolvedDigest, resolvedRef, nil
}

func loadRevokedIdentities(path string) (map[string]struct{}, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" || cleanPath == "." {
		return nil, fmt.Errorf("identity revocations path is required")
	}
	// #nosec G304 -- explicit local CLI path.
	payload, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read identity revocations: %w", err)
	}
	trimmed := strings.TrimSpace(string(payload))
	revoked := map[string]struct{}{}
	if trimmed == "" {
		return revoked, nil
	}

	var list []string
	if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
		for _, identity := range list {
			value := strings.TrimSpace(identity)
			if value != "" {
				revoked[value] = struct{}{}
			}
		}
		return revoked, nil
	}
	var object struct {
		RevokedIdentities []string `json:"revoked_identities"`
		Identities        []string `json:"identities"`
	}
	if err := json.Unmarshal([]byte(trimmed), &object); err == nil {
		for _, identity := range append(object.RevokedIdentities, object.Identities...) {
			value := strings.TrimSpace(identity)
			if value != "" {
				revoked[value] = struct{}{}
			}
		}
		return revoked, nil
	}

	for _, line := range strings.Split(trimmed, "\n") {
		value := strings.TrimSpace(line)
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		revoked[value] = struct{}{}
	}
	return revoked, nil
}

func identityIsRevoked(revoked map[string]struct{}, identity string) bool {
	value := strings.TrimSpace(identity)
	if value == "" || len(revoked) == 0 {
		return false
	}
	_, ok := revoked[value]
	return ok
}
