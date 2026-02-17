package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	"github.com/Clyra-AI/gait/core/scout"
)

func TestRunReportTopJSONFromRunIDAndTraceDir(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo([]string{"--json"}); code != exitOK {
		t.Fatalf("runDemo expected %d got %d", exitOK, code)
	}

	traceDir := filepath.Join(workDir, "traces")
	if err := os.MkdirAll(traceDir, 0o750); err != nil {
		t.Fatalf("mkdir traces dir: %v", err)
	}
	tracePath := filepath.Join(traceDir, "trace_run_demo.json")
	record := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 13, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_001",
		CorrelationID:   "run_demo",
		ToolName:        "tool.delete_user",
		ArgsDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		IntentDigest:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		PolicyDigest:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Verdict:         "block",
		Violations:      []string{"prompt_injection_egress_attempt"},
	}
	rawTrace, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal trace fixture: %v", err)
	}
	if err := os.WriteFile(tracePath, rawTrace, 0o600); err != nil {
		t.Fatalf("write trace fixture: %v", err)
	}

	var code int
	raw := captureStdout(t, func() {
		code = runReportTop([]string{
			"--runs", "run_demo",
			"--traces", traceDir,
			"--limit", "3",
			"--json",
		})
	})
	if code != exitOK {
		t.Fatalf("runReportTop expected %d got %d", exitOK, code)
	}

	var output reportTopOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("unmarshal report top output: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected report top output ok=true, got error=%s", output.Error)
	}
	if output.Report == nil {
		t.Fatalf("expected embedded report payload")
	}
	if output.TopActions == 0 || len(output.Report.TopActions) == 0 {
		t.Fatalf("expected non-empty top actions")
	}
	if output.TopActions > 3 {
		t.Fatalf("expected top actions to honor limit, got %d", output.TopActions)
	}
	if _, err := os.Stat(output.OutputPath); err != nil {
		t.Fatalf("expected output report to be written: %v", err)
	}
}

func TestRunReportTopFromRunpackDirectory(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo([]string{"--json"}); code != exitOK {
		t.Fatalf("runDemo expected %d got %d", exitOK, code)
	}

	var code int
	raw := captureStdout(t, func() {
		code = runReportTop([]string{
			"--runs", filepath.Join(workDir, "gait-out"),
			"--json",
		})
	})
	if code != exitOK {
		t.Fatalf("runReportTop expected %d got %d", exitOK, code)
	}

	var output reportTopOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("unmarshal report top output: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected report top output ok=true, got error=%s", output.Error)
	}
	if output.ActionCount == 0 {
		t.Fatalf("expected non-zero action count")
	}
}

func TestRunReportTopErrors(t *testing.T) {
	if code := runReportTop([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runReportTop missing inputs expected %d got %d", exitInvalidInput, code)
	}
	if code := runReportTop([]string{"--runs", "../missing", "--json"}); code != exitInvalidInput {
		t.Fatalf("runReportTop invalid path expected %d got %d", exitInvalidInput, code)
	}
}

func TestRunReportUsageAndTextOutput(t *testing.T) {
	if code := runReport([]string{}); code != exitInvalidInput {
		t.Fatalf("runReport missing subcommand expected %d got %d", exitInvalidInput, code)
	}
	if code := runReport([]string{"--explain"}); code != exitOK {
		t.Fatalf("runReport explain expected %d got %d", exitOK, code)
	}

	out := captureStdout(t, func() {
		code := writeReportTopOutput(false, reportTopOutput{
			OK:          true,
			OutputPath:  "./gait-out/report_top_actions.json",
			RunCount:    1,
			TraceCount:  1,
			ActionCount: 2,
			TopActions:  1,
			Report: &scout.TopActionsReport{
				TopActions: []scout.TopAction{{
					Rank:           1,
					Score:          350,
					ToolClass:      "destructive",
					BlastRadius:    3,
					ToolName:       "tool.delete",
					RunID:          "run_demo",
					SourceArtifact: "trace_run_demo.json",
				}},
			},
		}, exitOK)
		if code != exitOK {
			t.Fatalf("writeReportTopOutput expected %d got %d", exitOK, code)
		}
	})
	if !strings.Contains(out, "report top ok") {
		t.Fatalf("expected success text output, got: %s", out)
	}

	errOut := captureStdout(t, func() {
		code := writeReportTopOutput(false, reportTopOutput{
			OK:    false,
			Error: "boom",
		}, exitInvalidInput)
		if code != exitInvalidInput {
			t.Fatalf("writeReportTopOutput error expected %d got %d", exitInvalidInput, code)
		}
	})
	if !strings.Contains(errOut, "report top error: boom") {
		t.Fatalf("expected error text output, got: %s", errOut)
	}
}
