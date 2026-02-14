package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	gatecore "github.com/davidahmann/gait/core/gate"
	"github.com/davidahmann/gait/core/jcs"
	"github.com/davidahmann/gait/core/jobruntime"
	"github.com/davidahmann/gait/core/pack"
	"github.com/davidahmann/gait/core/runpack"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

const demoRunID = "run_demo"
const demoOutDir = "gait-out"
const demoDurableJobID = "job_demo_durable"
const demoMetricsOptInCommand = "export GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl"
const demoPolicyPath = "./gait-out/policy_demo_high_risk.yaml"
const demoPolicyIntentPath = "./gait-out/intent_demo_delete.json"

type demoMode string

const (
	demoModeStandard demoMode = "standard"
	demoModeDurable  demoMode = "durable"
	demoModePolicy   demoMode = "policy"
)

type demoOutput struct {
	OK               bool     `json:"ok"`
	Mode             string   `json:"mode,omitempty"`
	RunID            string   `json:"run_id,omitempty"`
	JobID            string   `json:"job_id,omitempty"`
	JobStatus        string   `json:"job_status,omitempty"`
	Bundle           string   `json:"bundle,omitempty"`
	PackPath         string   `json:"pack_path,omitempty"`
	TicketFooter     string   `json:"ticket_footer,omitempty"`
	Verify           string   `json:"verify,omitempty"`
	PolicyVerdict    string   `json:"policy_verdict,omitempty"`
	MatchedRule      string   `json:"matched_rule,omitempty"`
	ReasonCodes      []string `json:"reason_codes,omitempty"`
	SimulatedFailure string   `json:"simulated_failure,omitempty"`
	NextCommands     []string `json:"next_commands,omitempty"`
	MetricsOptIn     string   `json:"metrics_opt_in,omitempty"`
	DurationMS       int64    `json:"duration_ms,omitempty"`
	Error            string   `json:"error,omitempty"`
}

func runDemo(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Run a fully offline deterministic demo and emit a shareable runpack receipt for verification.")
	}
	arguments = reorderInterspersedFlags(arguments, nil)

	flagSet := flag.NewFlagSet("demo", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var durableMode bool
	var policyMode bool
	var helpFlag bool

	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&durableMode, "durable", false, "run durable job lifecycle demo")
	flagSet.BoolVar(&policyMode, "policy", false, "run policy block demo")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeDemoOutput(jsonOutput, demoOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printDemoUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeDemoOutput(jsonOutput, demoOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if durableMode && policyMode {
		return writeDemoOutput(jsonOutput, demoOutput{
			OK:    false,
			Error: "choose only one mode: --durable or --policy",
		}, exitInvalidInput)
	}

	mode := demoModeStandard
	if durableMode {
		mode = demoModeDurable
	}
	if policyMode {
		mode = demoModePolicy
	}

	output, exitCode := executeDemo(mode)
	return writeDemoOutput(jsonOutput, output, exitCode)
}

func printDemoUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait demo [--durable|--policy] [--json] [--explain]")
}

func writeDemoOutput(jsonOutput bool, output demoOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		if output.Mode != "" {
			fmt.Printf("mode=%s\n", output.Mode)
		}
		if output.RunID != "" {
			fmt.Printf("run_id=%s\n", output.RunID)
		}
		if output.JobID != "" {
			fmt.Printf("job_id=%s\n", output.JobID)
		}
		if output.JobStatus != "" {
			fmt.Printf("job_status=%s\n", output.JobStatus)
		}
		if output.Bundle != "" {
			fmt.Printf("bundle=%s\n", output.Bundle)
		}
		if output.PackPath != "" {
			fmt.Printf("pack_path=%s\n", output.PackPath)
		}
		if output.TicketFooter != "" {
			fmt.Printf("ticket_footer=%s\n", output.TicketFooter)
		}
		if output.Verify != "" {
			fmt.Printf("verify=%s\n", output.Verify)
		}
		if output.PolicyVerdict != "" {
			fmt.Printf("policy_verdict=%s\n", output.PolicyVerdict)
		}
		if output.MatchedRule != "" {
			fmt.Printf("matched_rule=%s\n", output.MatchedRule)
		}
		if len(output.ReasonCodes) > 0 {
			fmt.Printf("reason_codes=%s\n", joinCSV(output.ReasonCodes))
		}
		if output.SimulatedFailure != "" {
			fmt.Printf("simulated_failure=%s\n", output.SimulatedFailure)
		}
		if len(output.NextCommands) > 0 {
			fmt.Printf("next=%s\n", joinCSV(output.NextCommands))
		}
		if output.MetricsOptIn != "" {
			fmt.Printf("metrics_opt_in=%s\n", output.MetricsOptIn)
		}
		return exitCode
	}
	fmt.Printf("demo error: %s\n", output.Error)
	return exitCode
}

func executeDemo(mode demoMode) (demoOutput, int) {
	switch mode {
	case demoModeDurable:
		return executeDurableDemo()
	case demoModePolicy:
		return executePolicyDemo()
	default:
		return executeStandardDemo()
	}
}

func executeStandardDemo() (demoOutput, int) {
	startedAt := time.Now()
	outDir := filepath.Join(".", demoOutDir)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeStandard), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}
	zipPath := filepath.Join(outDir, fmt.Sprintf("runpack_%s.zip", demoRunID))

	run, intents, results, refs, err := buildDemoRunpack()
	if err != nil {
		return demoOutput{OK: false, Mode: string(demoModeStandard), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	recordResult, err := runpack.WriteRunpack(zipPath, runpack.RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
	})
	if err != nil {
		return demoOutput{OK: false, Mode: string(demoModeStandard), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	verifyResult, err := runpack.VerifyZip(zipPath, runpack.VerifyOptions{
		RequireSignature: false,
	})
	if err != nil {
		return demoOutput{OK: false, Mode: string(demoModeStandard), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 || verifyResult.SignatureStatus == "failed" {
		return demoOutput{OK: false, Mode: string(demoModeStandard), Error: "verification failed"}, exitVerifyFailed
	}

	return demoOutput{
		OK:           true,
		Mode:         string(demoModeStandard),
		RunID:        demoRunID,
		Bundle:       fmt.Sprintf("./%s/runpack_%s.zip", demoOutDir, demoRunID),
		TicketFooter: formatTicketFooter(demoRunID, recordResult.Manifest.ManifestDigest),
		Verify:       "ok",
		NextCommands: demoNextCommands(demoModeStandard),
		MetricsOptIn: demoMetricsOptInCommand,
		DurationMS:   time.Since(startedAt).Milliseconds(),
	}, exitOK
}

func executeDurableDemo() (demoOutput, int) {
	startedAt := time.Now()
	outDir := filepath.Join(".", demoOutDir)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}
	jobRoot := filepath.Join(outDir, "jobs")
	if err := os.MkdirAll(jobRoot, 0o750); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	jobDir := filepath.Join(jobRoot, demoDurableJobID)
	if err := os.RemoveAll(jobDir); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	baseNow := time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC)
	if _, err := jobruntime.Submit(jobRoot, jobruntime.SubmitOptions{
		JobID:                  demoDurableJobID,
		ProducerVersion:        version,
		EnvironmentFingerprint: "envfp:demo-durable",
		Actor:                  "demo.user",
		Now:                    baseNow,
	}); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	if _, _, err := jobruntime.AddCheckpoint(jobRoot, demoDurableJobID, jobruntime.CheckpointOptions{
		Type:           jobruntime.CheckpointTypeDecisionNeeded,
		Summary:        "approval required to continue",
		RequiredAction: "security_review",
		Actor:          "demo.user",
		Now:            baseNow.Add(1 * time.Minute),
	}); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	_, resumeErr := jobruntime.Resume(jobRoot, demoDurableJobID, jobruntime.ResumeOptions{
		CurrentEnvironmentFingerprint: "envfp:demo-durable",
		Reason:                        "resume_without_approval",
		Actor:                         "demo.user",
		Now:                           baseNow.Add(2 * time.Minute),
	})
	if !errors.Is(resumeErr, jobruntime.ErrApprovalRequired) {
		if resumeErr != nil {
			return demoOutput{OK: false, Mode: string(demoModeDurable), Error: resumeErr.Error()}, exitCodeForError(resumeErr, exitInvalidInput)
		}
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: "expected approval requirement before resume"}, exitVerifyFailed
	}

	if _, err := jobruntime.Approve(jobRoot, demoDurableJobID, jobruntime.ApprovalOptions{
		Actor:  "demo.approver",
		Reason: "approved_for_resume",
		Now:    baseNow.Add(3 * time.Minute),
	}); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	if _, err := jobruntime.Resume(jobRoot, demoDurableJobID, jobruntime.ResumeOptions{
		CurrentEnvironmentFingerprint: "envfp:demo-durable",
		Reason:                        "approved_resume",
		Actor:                         "demo.user",
		Now:                           baseNow.Add(4 * time.Minute),
	}); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	if _, _, err := jobruntime.AddCheckpoint(jobRoot, demoDurableJobID, jobruntime.CheckpointOptions{
		Type:    jobruntime.CheckpointTypeCompleted,
		Summary: "durable demo complete",
		Actor:   "demo.user",
		Now:     baseNow.Add(5 * time.Minute),
	}); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	packPath := filepath.Join(outDir, fmt.Sprintf("pack_%s.zip", demoDurableJobID))
	if err := os.Remove(packPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}
	if _, err := pack.BuildJobPackFromPath(jobRoot, demoDurableJobID, packPath, version, nil); err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}
	verifyResult, err := pack.Verify(packPath, pack.VerifyOptions{})
	if err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 || verifyResult.SignatureStatus == "failed" {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: "durable pack verification failed"}, exitVerifyFailed
	}
	state, err := jobruntime.Status(jobRoot, demoDurableJobID)
	if err != nil {
		return demoOutput{OK: false, Mode: string(demoModeDurable), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	return demoOutput{
		OK:               true,
		Mode:             string(demoModeDurable),
		RunID:            demoRunID,
		JobID:            state.JobID,
		JobStatus:        state.Status,
		Bundle:           fmt.Sprintf("./%s/jobs/%s/state.json", demoOutDir, demoDurableJobID),
		PackPath:         fmt.Sprintf("./%s/pack_%s.zip", demoOutDir, demoDurableJobID),
		Verify:           "ok",
		SimulatedFailure: "resume blocked until approval",
		NextCommands:     demoNextCommands(demoModeDurable),
		MetricsOptIn:     demoMetricsOptInCommand,
		DurationMS:       time.Since(startedAt).Milliseconds(),
	}, exitOK
}

func executePolicyDemo() (demoOutput, int) {
	startedAt := time.Now()
	outDir := filepath.Join(".", demoOutDir)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return demoOutput{OK: false, Mode: string(demoModePolicy), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	policyBody := strings.Join([]string{
		"schema_id: gait.gate.policy",
		"schema_version: 1.0.0",
		"default_verdict: allow",
		"rules:",
		"  - name: block-destructive-tool-delete",
		"    priority: 10",
		"    effect: block",
		"    match:",
		"      tool_names: [tool.delete]",
		"      risk_classes: [high]",
		"    reason_codes: [destructive_tool_blocked]",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(outDir, "policy_demo_high_risk.yaml"), []byte(policyBody), 0o600); err != nil {
		return demoOutput{OK: false, Mode: string(demoModePolicy), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	policy, err := gatecore.ParsePolicyYAML([]byte(policyBody))
	if err != nil {
		return demoOutput{OK: false, Mode: string(demoModePolicy), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	intent := schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		ProducerVersion: version,
		ToolName:        "tool.delete",
		Args:            map[string]any{"path": "/tmp/demo/delete-me.txt"},
		Targets: []schemagate.IntentTarget{{
			Kind:          "path",
			Value:         "/tmp/demo/delete-me.txt",
			Operation:     "delete",
			EndpointClass: "fs.delete",
			Destructive:   true,
		}},
		Context: schemagate.IntentContext{
			Identity:  "demo.user",
			Workspace: "/tmp/demo",
			RiskClass: "high",
		},
	}
	intentBytes, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		return demoOutput{OK: false, Mode: string(demoModePolicy), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}
	intentBytes = append(intentBytes, '\n')
	if err := os.WriteFile(filepath.Join(outDir, "intent_demo_delete.json"), intentBytes, 0o600); err != nil {
		return demoOutput{OK: false, Mode: string(demoModePolicy), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}

	evalResult, err := gatecore.EvaluatePolicyDetailed(policy, intent, gatecore.EvalOptions{ProducerVersion: version})
	if err != nil {
		return demoOutput{OK: false, Mode: string(demoModePolicy), Error: err.Error()}, exitCodeForError(err, exitInvalidInput)
	}
	if evalResult.Result.Verdict != "block" {
		return demoOutput{OK: false, Mode: string(demoModePolicy), Error: fmt.Sprintf("expected block verdict, got %s", evalResult.Result.Verdict)}, exitVerifyFailed
	}

	return demoOutput{
		OK:            true,
		Mode:          string(demoModePolicy),
		PolicyVerdict: evalResult.Result.Verdict,
		MatchedRule:   evalResult.MatchedRule,
		ReasonCodes:   evalResult.Result.ReasonCodes,
		Verify:        "ok",
		Bundle:        demoPolicyIntentPath,
		PackPath:      demoPolicyPath,
		NextCommands:  demoNextCommands(demoModePolicy),
		MetricsOptIn:  demoMetricsOptInCommand,
		DurationMS:    time.Since(startedAt).Milliseconds(),
	}, exitOK
}

func demoNextCommands(mode demoMode) []string {
	switch mode {
	case demoModeDurable:
		return []string{
			fmt.Sprintf("gait job inspect --id %s --json", demoDurableJobID),
			fmt.Sprintf("gait pack inspect ./%s/pack_%s.zip --json", demoOutDir, demoDurableJobID),
			"gait demo --policy",
		}
	case demoModePolicy:
		return []string{
			fmt.Sprintf("gait gate eval --policy %s --intent %s --simulate --json", demoPolicyPath, demoPolicyIntentPath),
			fmt.Sprintf("gait gate eval --policy %s --intent %s --json", demoPolicyPath, demoPolicyIntentPath),
			"gait policy simulate --baseline examples/policy/base_medium_risk.yaml --policy examples/policy/base_high_risk.yaml --fixtures examples/policy/intents --json",
		}
	default:
		return []string{
			fmt.Sprintf("gait verify %s --json", demoRunID),
			fmt.Sprintf("gait regress bootstrap --from %s --json --junit ./%s/junit.xml", demoRunID, demoOutDir),
			"gait demo --durable",
			"gait demo --policy",
		}
	}
}

func buildDemoRunpack() (schemarunpack.Run, []schemarunpack.IntentRecord, []schemarunpack.ResultRecord, schemarunpack.Refs, error) {
	ts := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)

	run := schemarunpack.Run{
		SchemaID:        "gait.runpack.run",
		SchemaVersion:   "1.0.0",
		CreatedAt:       ts,
		ProducerVersion: "0.0.0-dev",
		RunID:           demoRunID,
		Env: schemarunpack.RunEnv{
			OS:      "demo",
			Arch:    "demo",
			Runtime: "go",
		},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: ts},
			{Event: "finish", TS: ts.Add(2 * time.Second)},
		},
	}

	intentArgs := []map[string]any{
		{"query": "gait demo: offline verification"},
		{"url": "https://example.local/demo"},
		{"input_ref": "ref_1"},
	}
	intentNames := []string{"tool.search", "tool.fetch", "tool.summarize"}

	intents := make([]schemarunpack.IntentRecord, 3)
	results := make([]schemarunpack.ResultRecord, 3)
	receipts := make([]schemarunpack.RefReceipt, 3)

	for i := 0; i < 3; i++ {
		intentID := fmt.Sprintf("intent_%d", i+1)
		argsDigest, err := digestObject(intentArgs[i])
		if err != nil {
			return schemarunpack.Run{}, nil, nil, schemarunpack.Refs{}, err
		}
		resultObj := map[string]any{
			"ok":      true,
			"message": fmt.Sprintf("demo result %d", i+1),
		}
		resultDigest, err := digestObject(resultObj)
		if err != nil {
			return schemarunpack.Run{}, nil, nil, schemarunpack.Refs{}, err
		}

		intents[i] = schemarunpack.IntentRecord{
			SchemaID:        "gait.runpack.intent",
			SchemaVersion:   "1.0.0",
			CreatedAt:       ts,
			ProducerVersion: run.ProducerVersion,
			RunID:           run.RunID,
			IntentID:        intentID,
			ToolName:        intentNames[i],
			ArgsDigest:      argsDigest,
			Args:            intentArgs[i],
		}
		results[i] = schemarunpack.ResultRecord{
			SchemaID:        "gait.runpack.result",
			SchemaVersion:   "1.0.0",
			CreatedAt:       ts,
			ProducerVersion: run.ProducerVersion,
			RunID:           run.RunID,
			IntentID:        intentID,
			Status:          "ok",
			ResultDigest:    resultDigest,
			Result:          resultObj,
		}

		receipts[i] = schemarunpack.RefReceipt{
			RefID:         fmt.Sprintf("ref_%d", i+1),
			SourceType:    "demo",
			SourceLocator: intentNames[i],
			QueryDigest:   digestString(fmt.Sprintf("query-%d", i+1)),
			ContentDigest: digestString(fmt.Sprintf("content-%d", i+1)),
			RetrievedAt:   ts,
			RedactionMode: "reference",
		}
	}

	refs := schemarunpack.Refs{
		SchemaID:        "gait.runpack.refs",
		SchemaVersion:   "1.0.0",
		CreatedAt:       ts,
		ProducerVersion: run.ProducerVersion,
		RunID:           run.RunID,
		Receipts:        receipts,
	}

	return run, intents, results, refs, nil
}

func digestObject(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return jcs.DigestJCS(raw)
}

func digestString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
