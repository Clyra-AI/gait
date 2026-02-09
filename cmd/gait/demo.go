package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/davidahmann/gait/core/jcs"
	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

const demoRunID = "run_demo"
const demoOutDir = "gait-out"

func runDemo(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Run a fully offline deterministic demo and emit a shareable runpack receipt for verification.")
	}
	if len(arguments) > 0 && (arguments[0] == "-h" || arguments[0] == "--help") {
		printDemoUsage()
		return exitOK
	}

	outDir := filepath.Join(".", demoOutDir)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		fmt.Printf("demo error: %v\n", err)
		return exitInvalidInput
	}
	zipPath := filepath.Join(outDir, fmt.Sprintf("runpack_%s.zip", demoRunID))

	run, intents, results, refs, err := buildDemoRunpack()
	if err != nil {
		fmt.Printf("demo error: %v\n", err)
		return exitInvalidInput
	}

	recordResult, err := runpack.WriteRunpack(zipPath, runpack.RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
	})
	if err != nil {
		fmt.Printf("demo error: %v\n", err)
		return exitInvalidInput
	}

	verifyResult, err := runpack.VerifyZip(zipPath, runpack.VerifyOptions{
		RequireSignature: false,
	})
	if err != nil {
		fmt.Printf("demo error: %v\n", err)
		return exitInvalidInput
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 || verifyResult.SignatureStatus == "failed" {
		fmt.Printf("demo error: verification failed\n")
		return exitVerifyFailed
	}

	fmt.Printf("run_id=%s\n", demoRunID)
	fmt.Printf("bundle=./%s/runpack_%s.zip\n", demoOutDir, demoRunID)
	fmt.Printf("ticket_footer=%s\n", formatTicketFooter(demoRunID, recordResult.Manifest.ManifestDigest))
	fmt.Println("verify=ok")

	return exitOK
}

func printDemoUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait demo [--explain]")
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
