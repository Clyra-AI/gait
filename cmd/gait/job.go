package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/davidahmann/gait/core/jobruntime"
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
		"env-fingerprint": true,
	})
	flagSet := flag.NewFlagSet("job-submit", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var actor string
	var envFingerprint string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.StringVar(&actor, "actor", "", "actor identity")
	flagSet.StringVar(&envFingerprint, "env-fingerprint", "", "optional environment fingerprint override")
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
	state, err := jobruntime.Submit(root, jobruntime.SubmitOptions{
		JobID:                  strings.TrimSpace(jobID),
		Actor:                  strings.TrimSpace(actor),
		ProducerVersion:        version,
		EnvironmentFingerprint: jobruntime.EnvironmentFingerprint(envFingerprint),
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
		"id":              true,
		"root":            true,
		"actor":           true,
		"reason":          true,
		"env-fingerprint": true,
	})
	flagSet := flag.NewFlagSet("job-resume", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jobID string
	var root string
	var actor string
	var reason string
	var envFingerprint string
	var allowEnvMismatch bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&jobID, "id", "", "job identifier")
	flagSet.StringVar(&root, "root", "./gait-out/jobs", "job state root directory")
	flagSet.StringVar(&actor, "actor", "", "actor identity")
	flagSet.StringVar(&reason, "reason", "", "resume reason")
	flagSet.StringVar(&envFingerprint, "env-fingerprint", "", "override current environment fingerprint")
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
	state, err := jobruntime.Resume(root, strings.TrimSpace(jobID), jobruntime.ResumeOptions{
		Actor:                         strings.TrimSpace(actor),
		Reason:                        strings.TrimSpace(reason),
		CurrentEnvironmentFingerprint: jobruntime.EnvironmentFingerprint(envFingerprint),
		AllowEnvironmentMismatch:      allowEnvMismatch,
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
	fmt.Println("  gait job submit --id <job_id> [--root ./gait-out/jobs] [--actor <id>] [--env-fingerprint <value>] [--json] [--explain]")
	fmt.Println("  gait job status --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job checkpoint add --id <job_id> --type <plan|progress|decision-needed|blocked|completed> --summary <text> [--required-action <text>] [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job checkpoint list --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job checkpoint show --id <job_id> --checkpoint <checkpoint_id> [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job pause --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job approve --id <job_id> --actor <id> [--reason <text>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job resume --id <job_id> [--actor <id>] [--reason <text>] [--env-fingerprint <value>] [--allow-env-mismatch] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job cancel --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
	fmt.Println("  gait job inspect --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobSubmitUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job submit --id <job_id> [--root ./gait-out/jobs] [--actor <id>] [--env-fingerprint <value>] [--json] [--explain]")
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

func printJobApproveUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job approve --id <job_id> --actor <id> [--reason <text>] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobResumeUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job resume --id <job_id> [--actor <id>] [--reason <text>] [--env-fingerprint <value>] [--allow-env-mismatch] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobCancelUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job cancel --id <job_id> [--actor <id>] [--root ./gait-out/jobs] [--json] [--explain]")
}

func printJobInspectUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait job inspect --id <job_id> [--root ./gait-out/jobs] [--json] [--explain]")
}
