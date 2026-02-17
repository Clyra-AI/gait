package scout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemaregress "github.com/Clyra-AI/gait/core/schema/v1/regress"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestBuildSignalReportClustersDeterministically(t *testing.T) {
	workDir := t.TempDir()

	runAPath := writeSignalRunpack(t, workDir, "run_alpha", "tool.delete_user", "error")
	runBPath := writeSignalRunpack(t, workDir, "run_beta", "tool.delete_user", "error")

	regressPath := filepath.Join(workDir, "regress.json")
	writeRegressFixture(t, regressPath, map[string][]string{
		"run_alpha": {"unexpected_diff"},
		"run_beta":  {"unexpected_diff"},
	})

	options := SignalOptions{
		ProducerVersion: "test",
		Now:             time.Date(2026, time.February, 9, 0, 0, 0, 0, time.UTC),
	}
	first, err := BuildSignalReport(SignalInput{
		RunpackPaths: []string{runAPath, runBPath},
		RegressPaths: []string{regressPath},
	}, options)
	if err != nil {
		t.Fatalf("build first signal report: %v", err)
	}
	second, err := BuildSignalReport(SignalInput{
		RunpackPaths: []string{runBPath, runAPath},
		RegressPaths: []string{regressPath},
	}, options)
	if err != nil {
		t.Fatalf("build second signal report: %v", err)
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

	if first.RunCount != 2 {
		t.Fatalf("expected run_count=2 got %d", first.RunCount)
	}
	if first.FamilyCount != 1 {
		t.Fatalf("expected family_count=1 got %d", first.FamilyCount)
	}
	if len(first.Fingerprints) != 2 {
		t.Fatalf("expected 2 fingerprints got %d", len(first.Fingerprints))
	}
	if first.Fingerprints[0].Fingerprint != first.Fingerprints[1].Fingerprint {
		t.Fatalf("expected same incident family fingerprint")
	}
	if len(first.TopIssues) != 1 {
		t.Fatalf("expected one top issue got %d", len(first.TopIssues))
	}
	if first.TopIssues[0].Count != 2 {
		t.Fatalf("expected clustered issue count=2 got %d", first.TopIssues[0].Count)
	}
	if first.TopIssues[0].TopFailureReason == "" {
		t.Fatalf("expected non-empty top failure reason")
	}
}

func TestBuildSignalReportRanksAndSuggests(t *testing.T) {
	workDir := t.TempDir()

	highRiskPath := writeSignalRunpack(t, workDir, "run_high", "tool.delete_user", "blocked")
	lowRiskPath := writeSignalRunpack(t, workDir, "run_low", "tool.read_user", "error")

	regressPath := filepath.Join(workDir, "regress_rank.json")
	writeRegressFixture(t, regressPath, map[string][]string{
		"run_high": {"blocked_prompt_injection"},
		"run_low":  {"unexpected_diff"},
	})

	report, err := BuildSignalReport(SignalInput{
		RunpackPaths: []string{highRiskPath, lowRiskPath},
		RegressPaths: []string{regressPath},
	}, SignalOptions{
		ProducerVersion: "test",
		Now:             time.Date(2026, time.February, 9, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build signal report: %v", err)
	}
	if len(report.TopIssues) != 2 {
		t.Fatalf("expected two top issues got %d", len(report.TopIssues))
	}
	if report.TopIssues[0].SeverityScore < report.TopIssues[1].SeverityScore {
		t.Fatalf("expected top issues sorted by severity descending")
	}
	if report.TopIssues[0].CanonicalRunID != "run_high" {
		t.Fatalf("expected run_high as top canonical run, got %s", report.TopIssues[0].CanonicalRunID)
	}
	if !slices.Contains(report.TopIssues[0].Drivers, driverPolicyChange) {
		t.Fatalf("expected top issue to include %s driver", driverPolicyChange)
	}
	if len(report.TopIssues[0].Suggestions) == 0 {
		t.Fatalf("expected deterministic fix suggestions")
	}
	for _, suggestion := range report.TopIssues[0].Suggestions {
		if suggestion.Kind == "" || suggestion.Summary == "" || suggestion.LikelyScope == "" {
			t.Fatalf("suggestion must be bounded and non-empty: %#v", suggestion)
		}
	}
}

func TestBuildSignalReportIncludesTraceAndRegressFallbackSignals(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := writeSignalRunpack(t, workDir, "run_trace", "tool.write_user", "error")

	tracePath := filepath.Join(workDir, "trace_run_trace.json")
	writeTraceFixture(t, tracePath, schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 9, 1, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		TraceID:         "trace_001",
		CorrelationID:   "run_trace",
		ToolName:        "tool.write_user",
		ArgsDigest:      strings.Repeat("a", 64),
		IntentDigest:    strings.Repeat("b", 64),
		PolicyDigest:    strings.Repeat("c", 64),
		Verdict:         "block",
		Violations:      []string{"prompt_injection_egress_attempt"},
	})

	regressPath := filepath.Join(workDir, "regress_trace.json")
	mustWriteJSON(t, regressPath, schemaregress.RegressResult{
		SchemaID:        "gait.regress.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 9, 1, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		FixtureSet:      "default",
		Status:          "fail",
		Graders: []schemaregress.GraderResult{{
			Name:        "run_trace/diff",
			Status:      "fail",
			ReasonCodes: []string{"unexpected_diff"},
		}},
	})

	report, err := BuildSignalReport(SignalInput{
		RunpackPaths: []string{runpackPath},
		TracePaths:   []string{tracePath},
		RegressPaths: []string{regressPath},
	}, SignalOptions{
		ProducerVersion: "test",
		Now:             time.Date(2026, time.February, 9, 1, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build signal report with traces: %v", err)
	}
	if len(report.Fingerprints) != 1 {
		t.Fatalf("expected one fingerprint got %d", len(report.Fingerprints))
	}
	fingerprint := report.Fingerprints[0]
	if fingerprint.TraceCount != 1 {
		t.Fatalf("expected trace_count=1 got %d", fingerprint.TraceCount)
	}
	if !slices.Contains(fingerprint.ReasonCodeVector, "trace_verdict_block") {
		t.Fatalf("expected trace verdict reason code in fingerprint: %#v", fingerprint.ReasonCodeVector)
	}
	if !slices.Contains(fingerprint.ReasonCodeVector, "violation_prompt_injection_egress_attempt") {
		t.Fatalf("expected trace violation reason code in fingerprint: %#v", fingerprint.ReasonCodeVector)
	}
	if !slices.Contains(fingerprint.ReasonCodeVector, "unexpected_diff") {
		t.Fatalf("expected regress reason fallback in fingerprint: %#v", fingerprint.ReasonCodeVector)
	}
}

func TestSignalHelpersAndErrorBranches(t *testing.T) {
	if _, err := BuildSignalReport(SignalInput{}, SignalOptions{}); err == nil {
		t.Fatalf("expected build signal report to fail without runpacks")
	}
	if _, err := BuildSignalReport(SignalInput{RunpackPaths: []string{"missing.zip"}}, SignalOptions{}); err == nil {
		t.Fatalf("expected build signal report to fail with missing runpack")
	}

	if runIDFromTrace(schemagate.TraceRecord{CorrelationID: "corr_run_demo"}) != "run_demo" {
		t.Fatalf("expected run id extraction from correlation_id")
	}
	if runIDFromTrace(schemagate.TraceRecord{TraceID: "trace_run_other_x"}) != "run_other_x" {
		t.Fatalf("expected run id extraction from trace_id")
	}
	if runIDFromTrace(schemagate.TraceRecord{TraceID: "trace_without_id"}) != "" {
		t.Fatalf("expected empty run id when trace has no run_* token")
	}

	if runIDFromGrader(schemaregress.GraderResult{Name: "run_named/diff"}) != "run_named" {
		t.Fatalf("expected run id fallback from grader name")
	}
	if runIDFromGrader(schemaregress.GraderResult{Name: ""}) != "" {
		t.Fatalf("expected empty run id for empty grader name")
	}

	if classifyToolClass("tool.drop_table") != "destructive" {
		t.Fatalf("expected destructive tool classification")
	}
	if classifyToolClass("tool.create_user") != "write" {
		t.Fatalf("expected write tool classification")
	}
	if classifyToolClass("tool.read_user") != "read" {
		t.Fatalf("expected read tool classification")
	}
	if classifyToolClass("tool.exec_shell") != "execute" {
		t.Fatalf("expected execute tool classification")
	}
	if toolClassScore("destructive") <= toolClassScore("read") {
		t.Fatalf("expected destructive score to exceed read score")
	}

	if normalizeTargetSystem("db", strings.Repeat("a", 80)) == "" {
		t.Fatalf("expected normalized target system output")
	}
	if targetSensitivityScore([]string{"db:prod_customer_db"}) != 3 {
		t.Fatalf("expected high sensitivity score")
	}
	if targetSensitivityScore([]string{"queue:internal"}) != 2 {
		t.Fatalf("expected medium sensitivity score")
	}
	if targetSensitivityScore([]string{"cache:public"}) != 1 {
		t.Fatalf("expected low sensitivity score")
	}

	if severityLevel(160) != signalSeverityCritical {
		t.Fatalf("expected critical severity level")
	}
	if severityLevel(120) != signalSeverityHigh {
		t.Fatalf("expected high severity level")
	}
	if severityLevel(80) != signalSeverityMedium {
		t.Fatalf("expected medium severity level")
	}
	if severityLevel(20) != signalSeverityLow {
		t.Fatalf("expected low severity level")
	}

	if normalizeSignalNow(time.Time{}).Year() != defaultSignalTimeYear {
		t.Fatalf("expected deterministic default signal timestamp")
	}
	if normalizeProducerVersion("") != "0.0.0-dev" {
		t.Fatalf("expected default producer version")
	}
	if minInt(1, 2) != 1 {
		t.Fatalf("expected minInt(1,2)=1")
	}
	if dominantReasonCode([]string{"b", "a", "a"}) != "a" {
		t.Fatalf("expected deterministic dominant reason code")
	}
}

func writeSignalRunpack(t *testing.T, workDir, runID, toolName, resultStatus string) string {
	t.Helper()
	runpackPath := filepath.Join(workDir, "runpack_"+runID+".zip")
	createdAt := time.Date(2026, time.February, 9, 0, 0, 0, 0, time.UTC)
	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           runID,
			CreatedAt:       createdAt,
			ProducerVersion: "test",
		},
		Intents: []schemarunpack.IntentRecord{{
			IntentID:   "intent_1",
			RunID:      runID,
			ToolName:   toolName,
			ArgsDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
		Results: []schemarunpack.ResultRecord{{
			IntentID:     "intent_1",
			RunID:        runID,
			Status:       resultStatus,
			ResultDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}},
		Refs: schemarunpack.Refs{
			RunID: runID,
			Receipts: []schemarunpack.RefReceipt{{
				RefID:         "ref_1",
				SourceType:    "database",
				SourceLocator: "prod/customer-db",
				QueryDigest:   "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				ContentDigest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
				RetrievedAt:   createdAt,
				RedactionMode: "reference",
			}},
		},
	})
	if err != nil {
		t.Fatalf("write signal runpack: %v", err)
	}
	return runpackPath
}

func writeRegressFixture(t *testing.T, path string, reasonsByRun map[string][]string) {
	t.Helper()
	graders := make([]schemaregress.GraderResult, 0, len(reasonsByRun))
	for runID, reasons := range reasonsByRun {
		graders = append(graders, schemaregress.GraderResult{
			Name:        runID + "/diff",
			Status:      "fail",
			ReasonCodes: reasons,
			Details: map[string]any{
				"run_id": runID,
			},
		})
	}
	slices.SortFunc(graders, func(left schemaregress.GraderResult, right schemaregress.GraderResult) int {
		switch {
		case left.Name < right.Name:
			return -1
		case left.Name > right.Name:
			return 1
		default:
			return 0
		}
	})
	result := schemaregress.RegressResult{
		SchemaID:        "gait.regress.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 9, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		FixtureSet:      "default",
		Status:          "fail",
		Graders:         graders,
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal regress fixture: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatalf("write regress fixture: %v", err)
	}
}

func writeTraceFixture(t *testing.T, path string, record schemagate.TraceRecord) {
	t.Helper()
	mustWriteJSON(t, path, record)
}

func mustWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatalf("write json file: %v", err)
	}
}
