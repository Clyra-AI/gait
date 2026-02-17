package gate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	coreerrors "github.com/Clyra-AI/gait/core/errors"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestEnforceRateLimitBlocksAfterLimit(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "rate_state.json")
	intent := rateLimitTestIntent()
	now := time.Date(2026, time.February, 5, 10, 0, 0, 0, time.UTC)

	limit := RateLimitPolicy{Requests: 2, Scope: "tool_identity", Window: "minute"}
	first, err := EnforceRateLimit(statePath, limit, intent, now)
	if err != nil {
		t.Fatalf("first enforce: %v", err)
	}
	if !first.Allowed || first.Used != 1 || first.Remaining != 1 {
		t.Fatalf("unexpected first decision: %#v", first)
	}

	second, err := EnforceRateLimit(statePath, limit, intent, now)
	if err != nil {
		t.Fatalf("second enforce: %v", err)
	}
	if !second.Allowed || second.Used != 2 || second.Remaining != 0 {
		t.Fatalf("unexpected second decision: %#v", second)
	}

	third, err := EnforceRateLimit(statePath, limit, intent, now)
	if err != nil {
		t.Fatalf("third enforce: %v", err)
	}
	if third.Allowed || third.Used != 2 || third.Remaining != 0 {
		t.Fatalf("unexpected third decision: %#v", third)
	}
}

func TestEnforceRateLimitIdentityScope(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "rate_state.json")
	now := time.Date(2026, time.February, 5, 11, 0, 0, 0, time.UTC)
	limit := RateLimitPolicy{Requests: 1, Scope: "identity", Window: "minute"}

	intentA := rateLimitTestIntent()
	intentB := rateLimitTestIntent()
	intentB.ToolName = "tool.other"

	decisionA, err := EnforceRateLimit(statePath, limit, intentA, now)
	if err != nil {
		t.Fatalf("identity scope first enforce: %v", err)
	}
	if !decisionA.Allowed {
		t.Fatalf("expected first identity-scoped decision to allow: %#v", decisionA)
	}

	decisionB, err := EnforceRateLimit(statePath, limit, intentB, now)
	if err != nil {
		t.Fatalf("identity scope second enforce: %v", err)
	}
	if decisionB.Allowed {
		t.Fatalf("expected second identity-scoped decision to block: %#v", decisionB)
	}
}

func TestEnforceRateLimitUnsupportedScope(t *testing.T) {
	_, err := EnforceRateLimit("", RateLimitPolicy{Requests: 1, Scope: "bad", Window: "minute"}, rateLimitTestIntent(), time.Now().UTC())
	if err == nil {
		t.Fatalf("expected unsupported scope error")
	}
}

func TestEnforceRateLimitHourWindowResets(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "rate_state.json")
	intent := rateLimitTestIntent()
	limit := RateLimitPolicy{Requests: 1, Scope: "tool_identity", Window: "hour"}

	first, err := EnforceRateLimit(statePath, limit, intent, time.Date(2026, time.February, 5, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first enforce: %v", err)
	}
	if !first.Allowed {
		t.Fatalf("expected first hour bucket request to allow: %#v", first)
	}

	second, err := EnforceRateLimit(statePath, limit, intent, time.Date(2026, time.February, 5, 10, 45, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second enforce: %v", err)
	}
	if second.Allowed {
		t.Fatalf("expected second request in same hour to block: %#v", second)
	}

	third, err := EnforceRateLimit(statePath, limit, intent, time.Date(2026, time.February, 5, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("third enforce: %v", err)
	}
	if !third.Allowed {
		t.Fatalf("expected next-hour request to allow: %#v", third)
	}
}

func TestEnforceRateLimitConcurrentLocking(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "rate_state.json")
	intent := rateLimitTestIntent()
	limit := RateLimitPolicy{Requests: 2, Scope: "tool_identity", Window: "minute"}
	now := time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC)

	const workers = 10
	allowed := 0
	blocked := 0
	contention := 0
	var mutex sync.Mutex
	var group sync.WaitGroup
	group.Add(workers)

	for index := 0; index < workers; index++ {
		go func() {
			defer group.Done()
			decision, err := EnforceRateLimit(statePath, limit, intent, now)
			if err != nil {
				if coreerrors.CategoryOf(err) == coreerrors.CategoryStateContention && coreerrors.RetryableOf(err) {
					mutex.Lock()
					blocked++
					contention++
					mutex.Unlock()
					return
				}
				t.Errorf("enforce concurrent rate limit: %v", err)
				return
			}
			mutex.Lock()
			if decision.Allowed {
				allowed++
			} else {
				blocked++
			}
			mutex.Unlock()
		}()
	}

	group.Wait()
	if allowed == 0 {
		t.Fatalf("expected at least one allowed decision, got 0 (blocked=%d contention=%d)", blocked, contention)
	}
	if allowed > 2 {
		t.Fatalf("expected at most 2 allowed decisions, got %d (blocked=%d contention=%d)", allowed, blocked, contention)
	}
	if allowed+blocked != workers {
		t.Fatalf("expected all workers accounted for, allowed=%d blocked=%d workers=%d", allowed, blocked, workers)
	}
}

func TestEnforceRateLimitStateFilePermissions(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "rate_state.json")
	intent := rateLimitTestIntent()
	limit := RateLimitPolicy{Requests: 1, Scope: "tool_identity", Window: "minute"}

	if _, err := EnforceRateLimit(statePath, limit, intent, time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("enforce rate limit: %v", err)
	}

	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if runtime.GOOS == "windows" {
		if info.Mode().Perm()&0o600 != 0o600 {
			t.Fatalf("expected owner read/write bits set on windows, got %#o", info.Mode().Perm())
		}
		return
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected state mode 0600 got %#o", info.Mode().Perm())
	}
}

func TestEnforceRateLimitRecoversStaleLock(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "rate_state.json")
	lockPath := statePath + ".lock"
	staleMetadata := fmt.Sprintf("{\"schema_id\":\"gait.gate.rate_limit_lock\",\"schema_version\":\"1.0.0\",\"pid\":1,\"created_at\":\"%s\"}\n", time.Now().UTC().Add(-2*rateLimitLockStaleAfter).Format(time.RFC3339))
	if err := os.WriteFile(lockPath, []byte(staleMetadata), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	intent := rateLimitTestIntent()
	limit := RateLimitPolicy{Requests: 1, Scope: "tool_identity", Window: "minute"}
	decision, err := EnforceRateLimit(statePath, limit, intent, time.Now().UTC())
	if err != nil {
		t.Fatalf("enforce with stale lock: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected stale lock recovery to allow request: %#v", decision)
	}
}

func TestWithRateLimitLockTimeoutCategory(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "rate_state.json")
	lockPath := statePath + ".lock"
	if err := os.WriteFile(lockPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	_, err := withRateLimitLock(statePath, func() (RateLimitDecision, error) {
		return RateLimitDecision{Allowed: true}, nil
	})
	if err == nil {
		t.Fatalf("expected lock timeout error")
	}
	if coreerrors.CategoryOf(err) != coreerrors.CategoryStateContention {
		t.Fatalf("expected state_contention category got %s", coreerrors.CategoryOf(err))
	}
	if !coreerrors.RetryableOf(err) {
		t.Fatalf("expected contention timeout to be retryable")
	}
}

func TestIsRateLimitLockContentionPermissionWithExistingLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "rate_state.json.lock")
	if err := os.WriteFile(lockPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	err := &os.PathError{Op: "open", Path: lockPath, Err: os.ErrPermission}
	if !isRateLimitLockContention(err, lockPath) {
		t.Fatalf("expected permission error with existing lock file to count as contention")
	}
}

func TestIsRateLimitLockContentionPermissionWithoutExistingLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "rate_state.json.lock")
	err := &os.PathError{Op: "open", Path: lockPath, Err: os.ErrPermission}
	if isRateLimitLockContention(err, lockPath) {
		t.Fatalf("expected permission error without lock file to be treated as io failure")
	}
}

func TestIsWindowsAccessDeniedLockError(t *testing.T) {
	t.Parallel()

	deniedErr := &os.PathError{Op: "open", Path: "rate_state.json.lock", Err: errors.New("Access is denied.")}
	expected := runtime.GOOS == "windows"
	if got := isWindowsAccessDeniedLockError(deniedErr); got != expected {
		t.Fatalf("unexpected access denied classification: got=%v want=%v", got, expected)
	}
	if isWindowsAccessDeniedLockError(nil) {
		t.Fatalf("expected nil error to be non-contention")
	}
}

func rateLimitTestIntent() schemagate.IntentRequest {
	return schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		ToolName:        "tool.write",
		Args:            map[string]any{"x": "y"},
		Targets:         []schemagate.IntentTarget{{Kind: "host", Value: "api.external.com"}},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
	}
}
