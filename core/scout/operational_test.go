package scout

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

func TestAppendLoadOperationalEvents(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "ops.jsonl")
	base := time.Date(2026, time.February, 6, 16, 0, 0, 0, time.UTC)

	start := NewOperationalStartEvent("verify", "cid-123", "test", base)
	end := NewOperationalEndEvent("verify", "cid-123", "test", 0, "none", false, 120*time.Millisecond, base.Add(120*time.Millisecond))
	if err := AppendOperationalEvent(logPath, start); err != nil {
		t.Fatalf("append start event: %v", err)
	}
	if err := AppendOperationalEvent(logPath, end); err != nil {
		t.Fatalf("append end event: %v", err)
	}

	events, err := LoadOperationalEvents(logPath)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events got %d", len(events))
	}
	if events[0].Phase != "start" || events[1].Phase != "end" {
		t.Fatalf("unexpected phases: %#v %#v", events[0].Phase, events[1].Phase)
	}
	if events[1].ElapsedMS != 120 {
		t.Fatalf("unexpected elapsed: %d", events[1].ElapsedMS)
	}
}

func TestOperationalEventValidation(t *testing.T) {
	valid := schemascout.OperationalEvent{
		SchemaID:        "gait.scout.operational_event",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 6, 16, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		CorrelationID:   "cid-1",
		Command:         "verify",
		Phase:           "end",
		ExitCode:        1,
		ErrorCategory:   "invalid_input",
		Retryable:       false,
		ElapsedMS:       1,
		Environment: schemascout.AdoptionEnvContext{
			OS:   "darwin",
			Arch: "arm64",
		},
	}
	if _, err := normalizeOperationalEvent(valid); err != nil {
		t.Fatalf("normalize valid event: %v", err)
	}

	cases := []schemascout.OperationalEvent{
		func() schemascout.OperationalEvent {
			item := valid
			item.SchemaID = "bad"
			return item
		}(),
		func() schemascout.OperationalEvent {
			item := valid
			item.Phase = "bad"
			return item
		}(),
		func() schemascout.OperationalEvent {
			item := valid
			item.ErrorCategory = "bad"
			return item
		}(),
		func() schemascout.OperationalEvent {
			item := valid
			item.CorrelationID = ""
			return item
		}(),
		func() schemascout.OperationalEvent {
			item := valid
			item.ExitCode = 256
			return item
		}(),
	}
	for _, item := range cases {
		if _, err := normalizeOperationalEvent(item); err == nil {
			t.Fatalf("expected validation error for %#v", item)
		}
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid_ops.jsonl")
	if err := os.WriteFile(invalidPath, []byte("{\n"), 0o600); err != nil {
		t.Fatalf("write invalid jsonl: %v", err)
	}
	if _, err := LoadOperationalEvents(invalidPath); err == nil {
		t.Fatalf("expected load error for malformed jsonl")
	}
}

func TestOperationalEventHelpersAndErrors(t *testing.T) {
	base := time.Date(2026, time.February, 6, 16, 30, 0, 0, time.UTC)
	start := NewOperationalStartEvent(" ", "", "", time.Time{})
	if start.Command != "unknown" {
		t.Fatalf("expected unknown command default, got %s", start.Command)
	}
	if start.CorrelationID != "unknown" {
		t.Fatalf("expected unknown correlation default, got %s", start.CorrelationID)
	}
	if start.ProducerVersion != "0.0.0-dev" {
		t.Fatalf("expected producer default, got %s", start.ProducerVersion)
	}
	if start.Phase != "start" {
		t.Fatalf("expected phase start, got %s", start.Phase)
	}

	end := NewOperationalEndEvent("verify", "cid", "test", 1, "invalid_input", true, -5*time.Millisecond, base)
	if end.ElapsedMS != 0 {
		t.Fatalf("expected negative elapsed clamp to 0, got %d", end.ElapsedMS)
	}
	if end.Phase != "end" {
		t.Fatalf("expected phase end, got %s", end.Phase)
	}

	if err := AppendOperationalEvent("", end); err == nil {
		t.Fatalf("expected append error for empty path")
	}
	if _, err := LoadOperationalEvents(""); err == nil {
		t.Fatalf("expected load error for empty path")
	}
}

func TestAppendOperationalEventConcurrentIntegrity(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "operational_concurrent.jsonl")
	base := time.Date(2026, time.February, 7, 1, 0, 0, 0, time.UTC)
	const workers = 300
	var group sync.WaitGroup
	group.Add(workers)
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		index := i
		go func() {
			defer group.Done()
			event := NewOperationalEndEvent(
				fmt.Sprintf("verify-%d", index),
				fmt.Sprintf("cid-%d", index),
				"stress-test",
				0,
				"none",
				false,
				10*time.Millisecond,
				base.Add(time.Duration(index)*time.Millisecond),
			)
			if err := AppendOperationalEvent(logPath, event); err != nil {
				errCh <- err
			}
		}()
	}
	group.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("append concurrent operational event: %v", err)
		}
	}
	events, err := LoadOperationalEvents(logPath)
	if err != nil {
		t.Fatalf("load concurrent operational events: %v", err)
	}
	if len(events) != workers {
		t.Fatalf("expected %d concurrent events, got %d", workers, len(events))
	}
}
