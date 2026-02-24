package integration

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/jobruntime"
)

const (
	stopAckSLOMS               = int64(50)
	stopBackpressureDispatches = 48
)

func TestStopLatencySLOForEmergencyStopAcknowledgment(t *testing.T) {
	workDir := t.TempDir()
	jobsRoot := filepath.Join(workDir, "jobs")
	baseTime := time.Date(2026, time.February, 24, 0, 0, 0, 0, time.UTC)
	jobID := "job_stop_latency"

	if _, err := jobruntime.Submit(jobsRoot, jobruntime.SubmitOptions{
		JobID:                  jobID,
		ProducerVersion:        "0.0.0-test",
		EnvironmentFingerprint: "envfp:stop-latency",
		Now:                    baseTime,
	}); err != nil {
		t.Fatalf("submit job: %v", err)
	}

	stopRequestedAt := baseTime.Add(125 * time.Millisecond)
	if _, err := jobruntime.EmergencyStop(jobsRoot, jobID, jobruntime.TransitionOptions{
		Actor: "ops.stop",
		Now:   stopRequestedAt,
	}); err != nil {
		t.Fatalf("emergency stop: %v", err)
	}

	_, events, err := jobruntime.Inspect(jobsRoot, jobID)
	if err != nil {
		t.Fatalf("inspect stopped job: %v", err)
	}

	ackEvent, ok := findJobEvent(events, "emergency_stop_acknowledged")
	if !ok {
		t.Fatalf("expected emergency_stop_acknowledged event, got %d events", len(events))
	}

	stopAckMS := ackEvent.CreatedAt.Sub(stopRequestedAt).Milliseconds()
	if stopAckMS < 0 {
		t.Fatalf("expected non-negative stop_ack_ms, got=%d", stopAckMS)
	}
	if stopAckMS > stopAckSLOMS {
		t.Fatalf("stop_ack_ms exceeded slo: got=%d want<=%d", stopAckMS, stopAckSLOMS)
	}
}

func TestEmergencyStopBackpressureHasZeroPostStopSideEffects(t *testing.T) {
	workDir := t.TempDir()
	jobsRoot := filepath.Join(workDir, "jobs")
	baseTime := time.Date(2026, time.February, 24, 1, 0, 0, 0, time.UTC)
	jobID := "job_stop_backpressure"

	if _, err := jobruntime.Submit(jobsRoot, jobruntime.SubmitOptions{
		JobID:                  jobID,
		ProducerVersion:        "0.0.0-test",
		EnvironmentFingerprint: "envfp:stop-backpressure",
		Now:                    baseTime,
	}); err != nil {
		t.Fatalf("submit job: %v", err)
	}

	stopRequestedAt := baseTime.Add(200 * time.Millisecond)
	stopped, err := jobruntime.EmergencyStop(jobsRoot, jobID, jobruntime.TransitionOptions{
		Actor: "ops.stop",
		Now:   stopRequestedAt,
	})
	if err != nil {
		t.Fatalf("emergency stop: %v", err)
	}
	if !jobruntime.IsEmergencyStopped(stopped) {
		t.Fatalf("expected emergency stopped state, got %#v", stopped)
	}

	for index := 0; index < stopBackpressureDispatches; index++ {
		if _, err := jobruntime.RecordBlockedDispatch(jobsRoot, jobID, jobruntime.DispatchRecordOptions{
			Actor:        "dispatch.queue",
			DispatchPath: fmt.Sprintf("queue/%03d", index),
			ReasonCode:   "emergency_stop_preempted",
			Now:          stopRequestedAt.Add(time.Duration(index+1) * time.Millisecond),
		}); err != nil {
			t.Fatalf("record blocked dispatch %d: %v", index, err)
		}
	}

	_, events, err := jobruntime.Inspect(jobsRoot, jobID)
	if err != nil {
		t.Fatalf("inspect stopped job events: %v", err)
	}
	ackEvent, ok := findJobEvent(events, "emergency_stop_acknowledged")
	if !ok {
		t.Fatalf("expected emergency_stop_acknowledged event")
	}

	postStopSideEffects := 0
	dispatchBlockedEvents := 0
	for _, event := range events {
		if event.CreatedAt.Before(ackEvent.CreatedAt) {
			continue
		}
		switch event.Type {
		case "emergency_stop_acknowledged":
		case "dispatch_blocked":
			dispatchBlockedEvents++
		default:
			postStopSideEffects++
		}
	}

	if dispatchBlockedEvents != stopBackpressureDispatches {
		t.Fatalf("unexpected post-stop blocked dispatch count: got=%d want=%d", dispatchBlockedEvents, stopBackpressureDispatches)
	}
	if postStopSideEffects != 0 {
		t.Fatalf("post_stop_side_effects mismatch: got=%d want=0", postStopSideEffects)
	}
}

func findJobEvent(events []jobruntime.Event, eventType string) (jobruntime.Event, bool) {
	for _, event := range events {
		if event.Type == eventType {
			return event, true
		}
	}
	return jobruntime.Event{}, false
}
