package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/scout"
)

func TestRunWritesAdoptionLogWhenEnabled(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adoption.jsonl")
	t.Setenv("GAIT_ADOPTION_LOG", logPath)

	if code := run([]string{"gait", "version"}); code != exitOK {
		t.Fatalf("run version expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "regress", "run", "--help"}); code != exitOK {
		t.Fatalf("run regress run help expected %d got %d", exitOK, code)
	}

	events, err := scout.LoadAdoptionEvents(logPath)
	if err != nil {
		t.Fatalf("load adoption events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 adoption events got %d", len(events))
	}
	if events[0].Command != "version" {
		t.Fatalf("unexpected command: %s", events[0].Command)
	}
	if events[1].Command != "regress run" {
		t.Fatalf("unexpected command: %s", events[1].Command)
	}
	if len(events[1].Milestones) != 1 || events[1].Milestones[0] != "A4" {
		t.Fatalf("expected A4 milestone, got %#v", events[1].Milestones)
	}
}

func TestRunWritesAdoptionWorkflowIDFromEnv(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adoption_workflow.jsonl")
	t.Setenv("GAIT_ADOPTION_LOG", logPath)
	t.Setenv("GAIT_ADOPTION_WORKFLOW", "gait-capture-runpack")

	if code := run([]string{"gait", "version"}); code != exitOK {
		t.Fatalf("run version expected %d got %d", exitOK, code)
	}

	events, err := scout.LoadAdoptionEvents(logPath)
	if err != nil {
		t.Fatalf("load adoption events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 adoption event got %d", len(events))
	}
	if events[0].WorkflowID != "gait-capture-runpack" {
		t.Fatalf("expected workflow_id propagation, got %q", events[0].WorkflowID)
	}
}

func TestRunWritesOperationalLogWhenEnabled(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "operational.jsonl")
	t.Setenv("GAIT_OPERATIONAL_LOG", logPath)

	if code := run([]string{"gait", "version"}); code != exitOK {
		t.Fatalf("run version expected %d got %d", exitOK, code)
	}

	events, err := scout.LoadOperationalEvents(logPath)
	if err != nil {
		t.Fatalf("load operational events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 operational events got %d", len(events))
	}
	if events[0].Phase != "start" || events[1].Phase != "end" {
		t.Fatalf("unexpected phase sequence: %#v %#v", events[0].Phase, events[1].Phase)
	}
	if events[0].CorrelationID == "" || events[0].CorrelationID != events[1].CorrelationID {
		t.Fatalf("expected matching correlation id for start/end events")
	}
	if events[1].ExitCode != exitOK {
		t.Fatalf("expected end exit code %d got %d", exitOK, events[1].ExitCode)
	}
}

func TestRunWritesOperationalFailureCategory(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "operational.jsonl")
	t.Setenv("GAIT_OPERATIONAL_LOG", logPath)

	if code := run([]string{"gait", "policy", "test"}); code != exitInvalidInput {
		t.Fatalf("run invalid policy invocation expected %d got %d", exitInvalidInput, code)
	}

	events, err := scout.LoadOperationalEvents(logPath)
	if err != nil {
		t.Fatalf("load operational events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected start/end events got %d", len(events))
	}
	end := events[1]
	if end.Phase != "end" {
		t.Fatalf("expected end phase, got %s", end.Phase)
	}
	if end.ErrorCategory != "invalid_input" {
		t.Fatalf("expected invalid_input category, got %s", end.ErrorCategory)
	}
	if end.Retryable {
		t.Fatalf("expected retryable=false for invalid input")
	}
}

func TestRunJSONOutputIncludesCorrelationID(t *testing.T) {
	root := repoRootFromPackageDir(t)
	output := captureStdout(t, func() {
		if code := run([]string{"gait", "doctor", "--json", "--workdir", root, "--output-dir", filepath.Join(t.TempDir(), "gait-out")}); code != exitOK {
			t.Fatalf("run doctor json expected %d got %d", exitOK, code)
		}
	})
	if !strings.Contains(output, `"correlation_id":"`) {
		t.Fatalf("expected correlation_id in json output: %s", output)
	}
}

func TestRunDoctorAdoptionJSON(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adoption.jsonl")
	base := time.Date(2026, time.February, 6, 15, 0, 0, 0, time.UTC)
	if err := scout.AppendAdoptionEvent(logPath, scout.NewAdoptionEvent("demo", 0, 100*time.Millisecond, "test", base, "")); err != nil {
		t.Fatalf("append event: %v", err)
	}

	output := captureStdout(t, func() {
		if code := runDoctor([]string{"adoption", "--from", logPath, "--json"}); code != exitOK {
			t.Fatalf("runDoctor adoption expected %d got %d", exitOK, code)
		}
	})

	var decoded struct {
		OK     bool `json:"ok"`
		Report struct {
			TotalEvents        int              `json:"total_events"`
			ActivationComplete bool             `json:"activation_complete"`
			Blockers           []string         `json:"blockers"`
			ActivationTimingMS map[string]int64 `json:"activation_timing_ms"`
			ActivationMedians  map[string]int64 `json:"activation_medians_ms"`
		} `json:"report"`
	}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("unmarshal output: %v (%s)", err, output)
	}
	if !decoded.OK {
		t.Fatalf("expected ok output")
	}
	if decoded.Report.TotalEvents != 1 {
		t.Fatalf("expected total_events=1 got %d", decoded.Report.TotalEvents)
	}
	if decoded.Report.ActivationComplete {
		t.Fatalf("expected activation to be incomplete")
	}
	if len(decoded.Report.Blockers) == 0 {
		t.Fatalf("expected blockers for incomplete activation")
	}
	if decoded.Report.ActivationTimingMS["A1"] != 0 {
		t.Fatalf("expected A1 activation timing 0ms, got %d", decoded.Report.ActivationTimingMS["A1"])
	}
	if decoded.Report.ActivationMedians["m1_demo_elapsed_ms"] != 100 {
		t.Fatalf("expected m1 median 100ms, got %d", decoded.Report.ActivationMedians["m1_demo_elapsed_ms"])
	}
}

func TestRunDoctorAdoptionInputValidation(t *testing.T) {
	if code := runDoctor([]string{"adoption", "--json"}); code != exitInvalidInput {
		t.Fatalf("runDoctor adoption missing --from expected %d got %d", exitInvalidInput, code)
	}
	if code := runDoctor([]string{"adoption", "--json", "--from", "missing.jsonl"}); code != exitInvalidInput {
		t.Fatalf("runDoctor adoption missing file expected %d got %d", exitInvalidInput, code)
	}
	if code := runDoctor([]string{"adoption", "--from", "x", "y"}); code != exitInvalidInput {
		t.Fatalf("runDoctor adoption positional expected %d got %d", exitInvalidInput, code)
	}
	if code := runDoctor([]string{"adoption", "--help"}); code != exitOK {
		t.Fatalf("runDoctor adoption help expected %d got %d", exitOK, code)
	}
	if code := runDoctor([]string{"adoption", "--explain"}); code != exitOK {
		t.Fatalf("runDoctor adoption explain expected %d got %d", exitOK, code)
	}
}

func TestWriteDoctorAdoptionOutputTextPaths(t *testing.T) {
	if code := writeDoctorAdoptionOutput(false, doctorAdoptionOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeDoctorAdoptionOutput error expected %d got %d", exitInvalidInput, code)
	}
	if code := writeDoctorAdoptionOutput(false, doctorAdoptionOutput{OK: true}, exitOK); code != exitInvalidInput {
		t.Fatalf("writeDoctorAdoptionOutput missing report expected %d got %d", exitInvalidInput, code)
	}
	code := writeDoctorAdoptionOutput(false, doctorAdoptionOutput{
		OK: true,
		Report: &scout.AdoptionReport{
			Source:             "events.jsonl",
			TotalEvents:        1,
			ActivationComplete: false,
			Blockers:           []string{"missing A2"},
		},
	}, exitOK)
	if code != exitOK {
		t.Fatalf("writeDoctorAdoptionOutput text expected %d got %d", exitOK, code)
	}
}

func TestNormalizeAdoptionCommandVariants(t *testing.T) {
	cases := []struct {
		arguments []string
		expected  string
	}{
		{arguments: []string{"gait"}, expected: "version"},
		{arguments: []string{"gait", "version"}, expected: "version"},
		{arguments: []string{"gait", "--version"}, expected: "version"},
		{arguments: []string{"gait", "regress", "run"}, expected: "regress run"},
		{arguments: []string{"gait", "keys", "verify"}, expected: "keys verify"},
		{arguments: []string{"gait", "report", "top"}, expected: "report top"},
		{arguments: []string{"gait", "doctor", "adoption"}, expected: "doctor adoption"},
		{arguments: []string{"gait", "doctor", "--json"}, expected: "doctor"},
		{arguments: []string{"gait", "--explain"}, expected: "explain"},
		{arguments: []string{"gait", "unknown-command", "arg"}, expected: "unknown-command"},
	}
	for _, testCase := range cases {
		if got := normalizeAdoptionCommand(testCase.arguments); got != testCase.expected {
			t.Fatalf("normalizeAdoptionCommand got %q expected %q", got, testCase.expected)
		}
	}
}

func TestCaptureStdoutHelper(t *testing.T) {
	output := captureStdout(t, func() {
		_, _ = io.WriteString(os.Stdout, "ok")
	})
	if strings.TrimSpace(output) != "ok" {
		t.Fatalf("captureStdout mismatch: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()
	os.Stdout = writer

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	os.Stdout = original
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(raw)
}
