package scout

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

func TestAppendLoadAndReportAdoptionEvents(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adoption.jsonl")
	base := time.Date(2026, time.February, 6, 10, 0, 0, 0, time.UTC)

	events := []schemascout.AdoptionEvent{
		NewAdoptionEvent("demo", 0, 120*time.Millisecond, "test", base, ""),
		NewAdoptionEvent("verify", 0, 130*time.Millisecond, "test", base.Add(time.Second), ""),
		NewAdoptionEvent("regress init", 0, 140*time.Millisecond, "test", base.Add(2*time.Second), ""),
		NewAdoptionEvent("regress run", 0, 150*time.Millisecond, "test", base.Add(3*time.Second), ""),
	}
	for _, event := range events {
		if err := AppendAdoptionEvent(logPath, event); err != nil {
			t.Fatalf("append adoption event: %v", err)
		}
	}

	loaded, err := LoadAdoptionEvents(logPath)
	if err != nil {
		t.Fatalf("load adoption events: %v", err)
	}
	if len(loaded) != len(events) {
		t.Fatalf("expected %d events got %d", len(events), len(loaded))
	}

	report := BuildAdoptionReport(loaded, logPath, "test", time.Time{})
	if report.SchemaID != "gait.doctor.adoption_report" {
		t.Fatalf("unexpected report schema id: %s", report.SchemaID)
	}
	if !report.ActivationComplete {
		t.Fatalf("expected activation complete")
	}
	if len(report.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %#v", report.Blockers)
	}
	if got := report.ActivationTimingMS["A1"]; got != 0 {
		t.Fatalf("expected A1 activation timing 0ms, got %d", got)
	}
	if got := report.ActivationTimingMS["A4"]; got != 3000 {
		t.Fatalf("expected A4 activation timing 3000ms, got %d", got)
	}
	if got := report.ActivationMedians["m1_demo_elapsed_ms"]; got != 120 {
		t.Fatalf("expected m1 median 120ms, got %d", got)
	}
	if got := report.ActivationMedians["m2_regress_run_elapsed_ms"]; got != 150 {
		t.Fatalf("expected m2 median 150ms, got %d", got)
	}
	if len(report.SkillWorkflows) == 0 {
		t.Fatalf("expected skill workflow metrics")
	}
	if !report.CreatedAt.Equal(base.Add(3 * time.Second)) {
		t.Fatalf("expected deterministic created_at from last event, got %s", report.CreatedAt.Format(time.RFC3339Nano))
	}
}

func TestAdoptionValidationAndBlockers(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adoption.jsonl")
	event := NewAdoptionEvent("demo", 0, 10*time.Millisecond, "test", time.Date(2026, time.February, 6, 11, 0, 0, 0, time.UTC), "")
	event.ExitCode = 300
	if err := AppendAdoptionEvent(logPath, event); err == nil {
		t.Fatalf("expected append error for invalid exit code")
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid.jsonl")
	if err := os.WriteFile(invalidPath, []byte("{\n"), 0o600); err != nil {
		t.Fatalf("write invalid jsonl: %v", err)
	}
	if _, err := LoadAdoptionEvents(invalidPath); err == nil {
		t.Fatalf("expected load error for malformed jsonl")
	}

	report := BuildAdoptionReport([]schemascout.AdoptionEvent{
		NewAdoptionEvent("demo", 0, 10*time.Millisecond, "test", time.Date(2026, time.February, 6, 11, 0, 0, 0, time.UTC), ""),
	}, "inline", "test", time.Time{})
	if report.ActivationComplete {
		t.Fatalf("expected incomplete activation")
	}
	if len(report.Blockers) == 0 {
		t.Fatalf("expected blockers for missing milestones")
	}
}

func TestAdoptionReportSkillWorkflowStats(t *testing.T) {
	base := time.Date(2026, time.February, 6, 12, 0, 0, 0, time.UTC)
	report := BuildAdoptionReport([]schemascout.AdoptionEvent{
		NewAdoptionEvent("run record", 0, 100*time.Millisecond, "test", base, "gait-capture-runpack"),
		NewAdoptionEvent("run record", 12, 200*time.Millisecond, "test", base.Add(time.Second), "gait-capture-runpack"),
		NewAdoptionEvent("run record", 0, 300*time.Millisecond, "test", base.Add(2*time.Second), "gait-capture-runpack"),
	}, "inline", "test", time.Time{})

	if len(report.SkillWorkflows) != 1 {
		t.Fatalf("expected one skill workflow stat, got %#v", report.SkillWorkflows)
	}
	stat := report.SkillWorkflows[0]
	if stat.Workflow != "gait-capture-runpack" {
		t.Fatalf("unexpected workflow name: %#v", stat)
	}
	if stat.Total != 3 || stat.Success != 2 || stat.Failure != 1 {
		t.Fatalf("unexpected workflow counts: %#v", stat)
	}
	if stat.MedianRuntimeMS != 200 {
		t.Fatalf("expected median runtime 200ms, got %d", stat.MedianRuntimeMS)
	}
	if stat.MostCommonFailureCode != 12 {
		t.Fatalf("expected failure code 12, got %d", stat.MostCommonFailureCode)
	}
}

func TestNormalizeAdoptionEventValidation(t *testing.T) {
	base := schemascout.AdoptionEvent{
		SchemaID:        "gait.scout.adoption_event",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 6, 16, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Command:         "demo",
		Success:         true,
		ExitCode:        0,
		ElapsedMS:       12,
		Environment: schemascout.AdoptionEnvContext{
			OS:   "darwin",
			Arch: "arm64",
		},
	}

	validated, err := normalizeAdoptionEvent(base)
	if err != nil {
		t.Fatalf("normalize valid event: %v", err)
	}
	if validated.SchemaID != "gait.scout.adoption_event" || validated.SchemaVersion != "1.0.0" {
		t.Fatalf("unexpected normalized schema: %#v", validated)
	}

	cases := []schemascout.AdoptionEvent{
		func() schemascout.AdoptionEvent {
			event := base
			event.SchemaID = "bad"
			return event
		}(),
		func() schemascout.AdoptionEvent {
			event := base
			event.SchemaVersion = "2.0.0"
			return event
		}(),
		func() schemascout.AdoptionEvent {
			event := base
			event.CreatedAt = time.Time{}
			return event
		}(),
		func() schemascout.AdoptionEvent {
			event := base
			event.Command = ""
			return event
		}(),
		func() schemascout.AdoptionEvent {
			event := base
			event.ExitCode = -1
			return event
		}(),
		func() schemascout.AdoptionEvent {
			event := base
			event.ExitCode = 256
			return event
		}(),
		func() schemascout.AdoptionEvent {
			event := base
			event.Environment.OS = ""
			return event
		}(),
		func() schemascout.AdoptionEvent {
			event := base
			event.Environment.Arch = ""
			return event
		}(),
	}
	for _, testCase := range cases {
		if _, err := normalizeAdoptionEvent(testCase); err == nil {
			t.Fatalf("expected normalize error for case: %#v", testCase)
		}
	}
}

func TestAdoptionHelpersAndDefaults(t *testing.T) {
	event := NewAdoptionEvent("  ", 0, -10*time.Millisecond, "", time.Time{}, "gait-capture-runpack")
	if event.Command != "unknown" {
		t.Fatalf("expected unknown command default, got %s", event.Command)
	}
	if event.ProducerVersion != "0.0.0-dev" {
		t.Fatalf("expected producer default, got %s", event.ProducerVersion)
	}
	if event.ElapsedMS != 0 {
		t.Fatalf("expected elapsed clamp to 0, got %d", event.ElapsedMS)
	}
	if event.WorkflowID != "gait-capture-runpack" {
		t.Fatalf("expected workflow id propagation, got %s", event.WorkflowID)
	}
	if event.CreatedAt.IsZero() {
		t.Fatalf("expected created_at to be set")
	}

	if tags := milestoneTags("demo", true); len(tags) != 1 || tags[0] != "A1" {
		t.Fatalf("unexpected milestone tags for demo: %#v", tags)
	}
	if tags := milestoneTags("verify", false); len(tags) != 0 {
		t.Fatalf("expected no tags on failure: %#v", tags)
	}
	if !strings.Contains(activationBlockerHint("A3"), "regress init") {
		t.Fatalf("unexpected blocker hint for A3")
	}
	if !strings.Contains(activationBlockerHint("unknown"), "unknown") {
		t.Fatalf("expected fallback blocker hint")
	}

	emptyReport := BuildAdoptionReport(nil, "source", "test", time.Time{})
	if !emptyReport.CreatedAt.Equal(fixedAdoptionTime) {
		t.Fatalf("expected fixed created_at, got %s", emptyReport.CreatedAt.Format(time.RFC3339Nano))
	}
}

func TestLoadAdoptionEventsHighVolume(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adoption_large.jsonl")
	base := time.Date(2026, time.February, 6, 15, 0, 0, 0, time.UTC)

	const eventCount = 5000
	for index := 0; index < eventCount; index++ {
		event := NewAdoptionEvent("verify", 0, 5*time.Millisecond, "stress-test", base.Add(time.Duration(index)*time.Millisecond), "")
		if err := AppendAdoptionEvent(logPath, event); err != nil {
			t.Fatalf("append adoption event %d: %v", index, err)
		}
	}

	loaded, err := LoadAdoptionEvents(logPath)
	if err != nil {
		t.Fatalf("load adoption events: %v", err)
	}
	if len(loaded) != eventCount {
		t.Fatalf("expected %d events, got %d", eventCount, len(loaded))
	}

	report := BuildAdoptionReport(loaded, logPath, "stress-test", time.Time{})
	if report.TotalEvents != eventCount {
		t.Fatalf("expected total_events=%d got %d", eventCount, report.TotalEvents)
	}
	if report.CreatedAt.IsZero() {
		t.Fatalf("expected deterministic created_at")
	}
}

func TestAppendAdoptionEventConcurrentIntegrity(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adoption_concurrent.jsonl")
	base := time.Date(2026, time.February, 7, 0, 0, 0, 0, time.UTC)
	const workers = 300
	var group sync.WaitGroup
	group.Add(workers)
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		index := i
		go func() {
			defer group.Done()
			event := NewAdoptionEvent(
				fmt.Sprintf("verify-%d", index),
				0,
				5*time.Millisecond,
				"stress-test",
				base.Add(time.Duration(index)*time.Millisecond),
				"",
			)
			if err := AppendAdoptionEvent(logPath, event); err != nil {
				errCh <- err
			}
		}()
	}
	group.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("append concurrent adoption event: %v", err)
		}
	}
	events, err := LoadAdoptionEvents(logPath)
	if err != nil {
		t.Fatalf("load concurrent adoption events: %v", err)
	}
	if len(events) != workers {
		t.Fatalf("expected %d concurrent events, got %d", workers, len(events))
	}
}
