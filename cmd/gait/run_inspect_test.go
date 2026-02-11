package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

func TestRunInspectJSONAndTextOutput(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup: expected %d got %d", exitOK, code)
	}

	jsonOutput := captureStdout(t, func() {
		if code := runInspect([]string{"--json", "--from", "run_demo"}); code != exitOK {
			t.Fatalf("runInspect json: expected %d got %d", exitOK, code)
		}
	})
	var payload runInspectOutput
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonOutput)), &payload); err != nil {
		t.Fatalf("decode runInspect json: %v", err)
	}
	if !payload.OK {
		t.Fatalf("expected ok=true payload: %#v", payload)
	}
	if payload.RunID != "run_demo" {
		t.Fatalf("unexpected run_id: %s", payload.RunID)
	}
	if payload.IntentsTotal <= 0 {
		t.Fatalf("expected intents_total > 0, got %d", payload.IntentsTotal)
	}
	if len(payload.Entries) == 0 {
		t.Fatalf("expected non-empty entries")
	}

	textOutput := captureStdout(t, func() {
		if code := runInspect([]string{"run_demo"}); code != exitOK {
			t.Fatalf("runInspect text: expected %d got %d", exitOK, code)
		}
	})
	if !strings.Contains(textOutput, "run inspect: run_id=run_demo") {
		t.Fatalf("expected run inspect summary in output: %q", textOutput)
	}

	if code := runInspect([]string{}); code != exitInvalidInput {
		t.Fatalf("runInspect missing args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runInspect([]string{"--from", "run_demo", "extra"}); code != exitInvalidInput {
		t.Fatalf("runInspect extra args: expected %d got %d", exitInvalidInput, code)
	}
}

func TestRunInspectUnmatchedAndDuplicateResultBranches(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	now := time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC)
	runID := "run_inspect_branches"
	runpackPath := filepath.Join(workDir, "gait-out", "runpack_"+runID+".zip")
	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			SchemaID:        "gait.runpack.run",
			SchemaVersion:   "1.0.0",
			CreatedAt:       now,
			ProducerVersion: "test",
			RunID:           runID,
		},
		Intents: []schemarunpack.IntentRecord{
			{
				SchemaID:        "gait.runpack.intent",
				SchemaVersion:   "1.0.0",
				CreatedAt:       now,
				ProducerVersion: "test",
				RunID:           runID,
				IntentID:        "intent_1",
				ToolName:        "tool.read",
				ArgsDigest:      "args_1",
			},
		},
		Results: []schemarunpack.ResultRecord{
			{
				SchemaID:        "gait.runpack.result",
				SchemaVersion:   "1.0.0",
				CreatedAt:       now,
				ProducerVersion: "test",
				RunID:           runID,
				IntentID:        "intent_1",
				Status:          "ok",
				ResultDigest:    "result_1",
				Result: map[string]any{
					"verdict":      "allow",
					"reason_codes": []string{"allow_rule"},
					"violations":   []string{},
				},
			},
			{
				SchemaID:        "gait.runpack.result",
				SchemaVersion:   "1.0.0",
				CreatedAt:       now,
				ProducerVersion: "test",
				RunID:           runID,
				IntentID:        "intent_1",
				Status:          "ok",
				ResultDigest:    "result_1b",
				Result: map[string]any{
					"verdict":      "allow",
					"reason_codes": []string{"duplicate"},
					"violations":   []string{},
				},
			},
			{
				SchemaID:        "gait.runpack.result",
				SchemaVersion:   "1.0.0",
				CreatedAt:       now,
				ProducerVersion: "test",
				RunID:           runID,
				IntentID:        "intent_orphan",
				Status:          "error",
				ResultDigest:    "result_orphan",
				Result: map[string]any{
					"verdict":      "block",
					"reason_codes": []string{"blocked"},
					"violations":   []string{"destructive_operation"},
				},
			},
		},
		Refs: schemarunpack.Refs{
			SchemaID:        "gait.runpack.refs",
			SchemaVersion:   "1.0.0",
			CreatedAt:       now,
			ProducerVersion: "test",
			RunID:           runID,
			Receipts:        []schemarunpack.RefReceipt{},
		},
		CaptureMode: "reference",
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	output := captureStdout(t, func() {
		if code := runInspect([]string{"--json", "--from", runpackPath}); code != exitOK {
			t.Fatalf("runInspect custom runpack: expected %d got %d", exitOK, code)
		}
	})
	var payload runInspectOutput
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &payload); err != nil {
		t.Fatalf("decode runInspect json: %v", err)
	}
	if len(payload.Warnings) == 0 {
		t.Fatalf("expected duplicate-result warning")
	}
	if len(payload.UnmatchedResults) != 1 {
		t.Fatalf("expected one unmatched result, got %d", len(payload.UnmatchedResults))
	}
	if payload.UnmatchedResults[0].IntentID != "intent_orphan" {
		t.Fatalf("unexpected unmatched intent id: %#v", payload.UnmatchedResults[0])
	}
}

func TestRunInspectHelperBranches(t *testing.T) {
	payload := map[string]any{
		"reason_codes": []any{"a", "b", "a", 42},
		"violations":   "blocked",
	}
	reasons := stringListField(payload, "reason_codes")
	if len(reasons) != 2 || reasons[0] != "a" || reasons[1] != "b" {
		t.Fatalf("unexpected reason_codes parse: %#v", reasons)
	}
	violations := stringListField(payload, "violations")
	if len(violations) != 1 || violations[0] != "blocked" {
		t.Fatalf("unexpected violations parse: %#v", violations)
	}
	if got := stringListField(payload, "missing"); got != nil {
		t.Fatalf("expected nil missing list, got %#v", got)
	}
	if got := stringField(payload, "missing"); got != "" {
		t.Fatalf("expected empty missing field, got %q", got)
	}
	if got := fallbackValue("", "fallback"); got != "fallback" {
		t.Fatalf("unexpected fallback value: %q", got)
	}
	if got := fallbackValue("ok", "fallback"); got != "ok" {
		t.Fatalf("unexpected passthrough fallback value: %q", got)
	}

	errorOutput := captureStdout(t, func() {
		code := writeRunInspectOutput(false, runInspectOutput{OK: false, Error: "boom"}, exitInvalidInput)
		if code != exitInvalidInput {
			t.Fatalf("writeRunInspectOutput error exit: expected %d got %d", exitInvalidInput, code)
		}
	})
	if !strings.Contains(errorOutput, "inspect error: boom") {
		t.Fatalf("unexpected error output: %q", errorOutput)
	}
}
