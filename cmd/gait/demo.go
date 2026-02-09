package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/davidahmann/gait/core/jcs"
	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

const demoRunID = "run_demo"
const demoOutDir = "gait-out"

type demoOutput struct {
	OK           bool   `json:"ok"`
	RunID        string `json:"run_id,omitempty"`
	Bundle       string `json:"bundle,omitempty"`
	TicketFooter string `json:"ticket_footer,omitempty"`
	Verify       string `json:"verify,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	Error        string `json:"error,omitempty"`
}

func runDemo(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Run a fully offline deterministic demo and emit a shareable runpack receipt for verification.")
	}
	arguments = reorderInterspersedFlags(arguments, nil)

	flagSet := flag.NewFlagSet("demo", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var helpFlag bool

	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
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

	startedAt := time.Now()
	outDir := filepath.Join(".", demoOutDir)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return writeDemoOutput(jsonOutput, demoOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	zipPath := filepath.Join(outDir, fmt.Sprintf("runpack_%s.zip", demoRunID))

	run, intents, results, refs, err := buildDemoRunpack()
	if err != nil {
		return writeDemoOutput(jsonOutput, demoOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	recordResult, err := runpack.WriteRunpack(zipPath, runpack.RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
	})
	if err != nil {
		return writeDemoOutput(jsonOutput, demoOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	verifyResult, err := runpack.VerifyZip(zipPath, runpack.VerifyOptions{
		RequireSignature: false,
	})
	if err != nil {
		return writeDemoOutput(jsonOutput, demoOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 || verifyResult.SignatureStatus == "failed" {
		return writeDemoOutput(jsonOutput, demoOutput{OK: false, Error: "verification failed"}, exitVerifyFailed)
	}

	return writeDemoOutput(jsonOutput, demoOutput{
		OK:           true,
		RunID:        demoRunID,
		Bundle:       fmt.Sprintf("./%s/runpack_%s.zip", demoOutDir, demoRunID),
		TicketFooter: formatTicketFooter(demoRunID, recordResult.Manifest.ManifestDigest),
		Verify:       "ok",
		DurationMS:   time.Since(startedAt).Milliseconds(),
	}, exitOK)
}

func printDemoUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait demo [--json] [--explain]")
}

func writeDemoOutput(jsonOutput bool, output demoOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("run_id=%s\n", output.RunID)
		fmt.Printf("bundle=%s\n", output.Bundle)
		fmt.Printf("ticket_footer=%s\n", output.TicketFooter)
		fmt.Printf("verify=%s\n", output.Verify)
		return exitCode
	}
	fmt.Printf("demo error: %s\n", output.Error)
	return exitCode
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
