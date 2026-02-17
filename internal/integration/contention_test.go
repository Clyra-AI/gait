package integration

import (
	"errors"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	coreerrors "github.com/Clyra-AI/gait/core/errors"
	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestConcurrentGateRateLimitStateIsDeterministic(t *testing.T) {
	workDir := t.TempDir()
	statePath := workDir + "/rate_state.json"
	policy, err := gate.ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: high-risk-write
    effect: allow
    match:
      tool_names: [tool.write]
    rate_limit:
      requests: 2
      scope: tool_identity
      window: minute
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 6, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-test",
		ToolName:        "tool.write",
		Args:            map[string]any{"path": "/tmp/a.txt"},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
	}

	const workers = 10
	var allowCount int
	var blockCount int
	var contentionCount int
	var mutex sync.Mutex
	var group sync.WaitGroup
	group.Add(workers)
	now := time.Date(2026, time.February, 6, 12, 0, 0, 0, time.UTC)

	for i := 0; i < workers; i++ {
		go func() {
			defer group.Done()
			outcome, evalErr := gate.EvaluatePolicyDetailed(policy, intent, gate.EvalOptions{ProducerVersion: "0.0.0-test"})
			if evalErr != nil {
				t.Errorf("evaluate policy: %v", evalErr)
				return
			}
			decision, enforceErr := gate.EnforceRateLimit(statePath, outcome.RateLimit, intent, now)
			if enforceErr != nil {
				if coreerrors.CategoryOf(enforceErr) == coreerrors.CategoryStateContention && coreerrors.RetryableOf(enforceErr) {
					mutex.Lock()
					blockCount++
					contentionCount++
					mutex.Unlock()
					return
				}
				t.Errorf("enforce rate limit: %v", enforceErr)
				return
			}

			mutex.Lock()
			if decision.Allowed {
				allowCount++
			} else {
				blockCount++
			}
			mutex.Unlock()
		}()
	}

	group.Wait()
	if allowCount == 0 {
		t.Fatalf("expected at least one allowed operation, got 0 (blocked=%d contention=%d)", blockCount, contentionCount)
	}
	if allowCount > 2 {
		t.Fatalf("expected at most 2 allowed operations, got %d (blocked=%d contention=%d)", allowCount, blockCount, contentionCount)
	}
	if allowCount+blockCount != workers {
		t.Fatalf("expected all workers accounted for, allowed=%d blocked=%d workers=%d", allowCount, blockCount, workers)
	}
}

func TestConcurrentSessionAppendStateIsDeterministic(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "session.journal.jsonl")
	now := time.Date(2026, time.February, 11, 5, 0, 0, 0, time.UTC)

	if _, err := runpack.StartSession(journalPath, runpack.SessionStartOptions{
		SessionID: "sess_integration",
		RunID:     "run_integration",
		Now:       now,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}

	const workers = 10
	var group sync.WaitGroup
	group.Add(workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		worker := i
		go func() {
			defer group.Done()
			_, err := runpack.AppendSessionEvent(journalPath, runpack.SessionAppendOptions{
				CreatedAt: now.Add(time.Duration(worker+1) * time.Second),
				IntentID:  "intent_" + strconv.Itoa(worker),
				ToolName:  "tool.write",
				Verdict:   "allow",
			})
			if err != nil {
				errs <- err
			}
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("append session event: %v", err)
		}
	}

	status, err := runpack.GetSessionStatus(journalPath)
	if err != nil {
		t.Fatalf("session status: %v", err)
	}
	if status.EventCount != workers {
		t.Fatalf("unexpected event count: got=%d want=%d", status.EventCount, workers)
	}
	if status.LastSequence != workers {
		t.Fatalf("unexpected last sequence: got=%d want=%d", status.LastSequence, workers)
	}
}

func TestSessionSwarmContentionBudget(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "swarm.journal.jsonl")
	now := time.Date(2026, time.February, 11, 6, 30, 0, 0, time.UTC)
	t.Setenv("GAIT_SESSION_LOCK_PROFILE", "swarm")
	lockTimeout := "5s"
	if runtime.GOOS == "windows" {
		lockTimeout = "20s"
	}
	t.Setenv("GAIT_SESSION_LOCK_TIMEOUT", lockTimeout)
	t.Setenv("GAIT_SESSION_LOCK_RETRY", "10ms")

	if _, err := runpack.StartSession(journalPath, runpack.SessionStartOptions{
		SessionID: "sess_swarm",
		RunID:     "run_swarm",
		Now:       now,
	}); err != nil {
		t.Fatalf("start swarm session: %v", err)
	}

	const workers = 64
	var group sync.WaitGroup
	group.Add(workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		index := i
		go func() {
			defer group.Done()
			_, err := runpack.AppendSessionEvent(journalPath, runpack.SessionAppendOptions{
				CreatedAt: now.Add(time.Duration(index+1) * time.Second),
				IntentID:  "intent_swarm_" + strconv.Itoa(index),
				ToolName:  "tool.write",
				Verdict:   "allow",
			})
			if err != nil {
				errs <- err
			}
		}()
	}
	group.Wait()
	close(errs)

	contentionErrors := 0
	otherErrors := []error{}
	for err := range errs {
		var contention runpack.SessionLockContentionError
		if errors.As(err, &contention) {
			contentionErrors++
			continue
		}
		otherErrors = append(otherErrors, err)
	}
	if len(otherErrors) > 0 {
		t.Fatalf("unexpected non-contention errors: %v", otherErrors)
	}
	if contentionErrors > 1 {
		t.Fatalf("contention budget exceeded: got=%d want<=1", contentionErrors)
	}

	status, err := runpack.GetSessionStatus(journalPath)
	if err != nil {
		t.Fatalf("swarm status: %v", err)
	}
	if status.EventCount < workers-contentionErrors {
		t.Fatalf("expected at least %d events, got %d", workers-contentionErrors, status.EventCount)
	}
}
