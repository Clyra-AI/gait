package scout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestBuildTopActionsReportDeterministic(t *testing.T) {
	workDir := t.TempDir()
	runHighPath := writeSignalRunpack(t, workDir, "run_high", "tool.delete_customer", "error")
	runLowPath := writeSignalRunpack(t, workDir, "run_low", "tool.read_customer", "ok")
	tracePath := filepath.Join(workDir, "trace_run_high.json")
	writeTraceFixture(t, tracePath, schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 13, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_001",
		CorrelationID:   "run_high",
		ToolName:        "tool.delete_customer",
		ArgsDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		IntentDigest:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		PolicyDigest:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Verdict:         "block",
		Violations:      []string{"prompt_injection_egress_attempt"},
	})

	options := TopActionsOptions{
		ProducerVersion: "test",
		Now:             time.Date(2026, time.February, 13, 0, 0, 0, 0, time.UTC),
	}
	first, err := BuildTopActionsReport(TopActionsInput{
		RunpackPaths: []string{runHighPath, runLowPath},
		TracePaths:   []string{tracePath},
		Limit:        5,
	}, options)
	if err != nil {
		t.Fatalf("build top actions report first: %v", err)
	}
	second, err := BuildTopActionsReport(TopActionsInput{
		RunpackPaths: []string{runLowPath, runHighPath},
		TracePaths:   []string{tracePath},
		Limit:        5,
	}, options)
	if err != nil {
		t.Fatalf("build top actions report second: %v", err)
	}

	firstEncoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first report: %v", err)
	}
	secondEncoded, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second report: %v", err)
	}
	if string(firstEncoded) != string(secondEncoded) {
		t.Fatalf("expected deterministic report output")
	}

	if first.SchemaID != topActionsReportSchemaID {
		t.Fatalf("unexpected schema id: %s", first.SchemaID)
	}
	if first.ActionCount == 0 {
		t.Fatalf("expected non-zero action count")
	}
	if len(first.TopActions) == 0 {
		t.Fatalf("expected non-empty top actions")
	}
	if first.TopActions[0].ToolClass != "destructive" {
		t.Fatalf("expected destructive tool class at rank 1, got %s", first.TopActions[0].ToolClass)
	}
	if first.TopActions[0].Score < first.TopActions[len(first.TopActions)-1].Score {
		t.Fatalf("expected descending score order")
	}
}

func TestBuildTopActionsReportTraceOnly(t *testing.T) {
	workDir := t.TempDir()
	tracePath := filepath.Join(workDir, "trace_only.json")
	writeTraceFixture(t, tracePath, schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 13, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_run_trace_only",
		CorrelationID:   "run_trace_only",
		ToolName:        "tool.write_customer",
		ArgsDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		IntentDigest:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		PolicyDigest:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Verdict:         "require_approval",
		Violations:      []string{"approval_missing"},
	})

	report, err := BuildTopActionsReport(TopActionsInput{
		TracePaths: []string{tracePath},
		Limit:      1,
	}, TopActionsOptions{
		ProducerVersion: "test",
		Now:             time.Date(2026, time.February, 13, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build top actions report trace-only: %v", err)
	}
	if report.RunCount != 1 {
		t.Fatalf("expected run_count=1 got %d", report.RunCount)
	}
	if report.TraceCount != 1 {
		t.Fatalf("expected trace_count=1 got %d", report.TraceCount)
	}
	if len(report.TopActions) != 1 {
		t.Fatalf("expected one top action got %d", len(report.TopActions))
	}
	action := report.TopActions[0]
	if action.SourceType != "trace" {
		t.Fatalf("expected trace source type, got %s", action.SourceType)
	}
	if action.Verdict != "require_approval" {
		t.Fatalf("expected require_approval verdict, got %s", action.Verdict)
	}
}

func TestBuildTopActionsReportErrors(t *testing.T) {
	if _, err := BuildTopActionsReport(TopActionsInput{}, TopActionsOptions{}); err == nil {
		t.Fatalf("expected missing input to fail")
	}
	if _, err := BuildTopActionsReport(TopActionsInput{
		RunpackPaths: []string{"missing.zip"},
	}, TopActionsOptions{}); err == nil {
		t.Fatalf("expected missing runpack to fail")
	}

	workDir := t.TempDir()
	runpackPath := filepath.Join(workDir, "runpack_empty_tool.zip")
	createdAt := time.Date(2026, time.February, 13, 0, 0, 0, 0, time.UTC)
	if _, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_empty",
			CreatedAt:       createdAt,
			ProducerVersion: "test",
		},
		Intents: []schemarunpack.IntentRecord{{
			IntentID:   "intent_1",
			RunID:      "run_empty",
			ToolName:   "",
			ArgsDigest: strings.Repeat("a", 64),
		}},
		Results: []schemarunpack.ResultRecord{{
			IntentID:     "intent_1",
			RunID:        "run_empty",
			Status:       "ok",
			ResultDigest: strings.Repeat("b", 64),
		}},
		Refs: schemarunpack.Refs{
			RunID:    "run_empty",
			Receipts: []schemarunpack.RefReceipt{},
		},
	}); err != nil {
		t.Fatalf("write empty-tool runpack: %v", err)
	}
	if _, err := BuildTopActionsReport(TopActionsInput{
		RunpackPaths: []string{runpackPath},
	}, TopActionsOptions{}); err == nil {
		t.Fatalf("expected runpack with no actionable intents to fail")
	}
}

func TestTopActionHelperBranches(t *testing.T) {
	highTrace := schemagate.TraceRecord{ToolName: "tool.delete_prod_customer", Violations: []string{"pii_exfiltration"}}
	if got := topActionBlastRadiusFromTrace(highTrace); got != 3 {
		t.Fatalf("expected high blast radius 3 got %d", got)
	}
	mediumTrace := schemagate.TraceRecord{ToolName: "tool.write_internal_queue"}
	if got := topActionBlastRadiusFromTrace(mediumTrace); got != 2 {
		t.Fatalf("expected medium blast radius 2 got %d", got)
	}
	lowTrace := schemagate.TraceRecord{ToolName: "tool.read_public"}
	if got := topActionBlastRadiusFromTrace(lowTrace); got != 1 {
		t.Fatalf("expected low blast radius 1 got %d", got)
	}

	base := topActionScore(4, 3, "", nil)
	requireApproval := topActionScore(4, 3, "require_approval", []string{"violation_policy"})
	blocked := topActionScore(4, 3, "block", []string{"violation_policy"})
	errored := topActionScore(4, 3, "error", []string{"result_status_error"})
	if blocked <= requireApproval {
		t.Fatalf("expected block score to exceed require_approval score")
	}
	if errored <= base {
		t.Fatalf("expected error score to exceed base score")
	}
}

func TestBuildTopActionsReportLimitClampAndTraceOnlyUnknownRun(t *testing.T) {
	workDir := t.TempDir()
	tracePath := filepath.Join(workDir, "trace_unknown.json")
	record := schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 13, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_xyz",
		CorrelationID:   "corr_xyz",
		ToolName:        "tool.write_internal_queue",
		ArgsDigest:      strings.Repeat("a", 64),
		IntentDigest:    strings.Repeat("b", 64),
		PolicyDigest:    strings.Repeat("c", 64),
		Verdict:         "error",
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal trace record: %v", err)
	}
	if err := os.WriteFile(tracePath, encoded, 0o600); err != nil {
		t.Fatalf("write trace record: %v", err)
	}

	report, err := BuildTopActionsReport(TopActionsInput{
		TracePaths: []string{tracePath},
		Limit:      99,
	}, TopActionsOptions{
		ProducerVersion: "test",
		Now:             time.Date(2026, time.February, 13, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build top actions report trace-only unknown run: %v", err)
	}
	if report.RunCount != 1 {
		t.Fatalf("expected run_count=1 got %d", report.RunCount)
	}
	if len(report.TopActions) != 1 {
		t.Fatalf("expected clamped top action length 1 got %d", len(report.TopActions))
	}
	if report.TopActions[0].RunID != "unknown" {
		t.Fatalf("expected fallback unknown run id, got %s", report.TopActions[0].RunID)
	}
}
