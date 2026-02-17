package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	coreerrors "github.com/Clyra-AI/gait/core/errors"
	"github.com/Clyra-AI/gait/core/fsx"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const (
	rateLimitStateSchemaID  = "gait.gate.rate_limit_state"
	rateLimitStateSchemaV1  = "1.0.0"
	rateLimitLockTimeout    = 3 * time.Second
	rateLimitLockRetry      = 15 * time.Millisecond
	rateLimitLockStaleAfter = 15 * time.Second
)

type RateLimitDecision struct {
	Allowed   bool   `json:"allowed"`
	Limit     int    `json:"limit"`
	Used      int    `json:"used"`
	Remaining int    `json:"remaining"`
	Scope     string `json:"scope"`
	Key       string `json:"key"`
}

type persistedRateLimitState struct {
	SchemaID      string                     `json:"schema_id"`
	SchemaVersion string                     `json:"schema_version"`
	Counters      []persistedRateLimitBucket `json:"counters"`
}

type persistedRateLimitBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type rateLimitLockMetadata struct {
	SchemaID      string    `json:"schema_id"`
	SchemaVersion string    `json:"schema_version"`
	PID           int       `json:"pid"`
	CreatedAt     time.Time `json:"created_at"`
}

func EnforceRateLimit(statePath string, limit RateLimitPolicy, intent schemagate.IntentRequest, now time.Time) (RateLimitDecision, error) {
	if limit.Requests <= 0 {
		return RateLimitDecision{Allowed: true}, nil
	}

	normalizedIntent, err := NormalizeIntent(intent)
	if err != nil {
		return RateLimitDecision{}, fmt.Errorf("normalize intent for rate limit: %w", err)
	}

	scope := strings.ToLower(strings.TrimSpace(limit.Scope))
	if scope == "" {
		scope = "tool_identity"
	}
	window := strings.ToLower(strings.TrimSpace(limit.Window))
	if window == "" {
		window = "minute"
	}
	if _, ok := allowedRateLimitWindows[window]; !ok {
		return RateLimitDecision{}, fmt.Errorf("unsupported rate_limit window: %s", window)
	}

	nowUTC := now.UTC()
	if nowUTC.IsZero() {
		nowUTC = time.Now().UTC()
	}
	bucketStart := rateLimitBucketStart(window, nowUTC)
	bucket := bucketStart.Format(time.RFC3339)
	scopeKey, err := rateLimitScopeKey(scope, normalizedIntent)
	if err != nil {
		return RateLimitDecision{}, err
	}
	counterKey := window + "|" + scope + "|" + bucket + "|" + scopeKey

	return withRateLimitLock(statePath, func() (RateLimitDecision, error) {
		counters, err := loadRateLimitCounters(statePath)
		if err != nil {
			return RateLimitDecision{}, err
		}
		pruneRateLimitCounters(counters, window, nowUTC)

		used := counters[counterKey]
		if used >= limit.Requests {
			return RateLimitDecision{
				Allowed:   false,
				Limit:     limit.Requests,
				Used:      used,
				Remaining: 0,
				Scope:     scope,
				Key:       scopeKey,
			}, nil
		}

		used++
		counters[counterKey] = used
		if err := writeRateLimitCounters(statePath, counters); err != nil {
			return RateLimitDecision{}, err
		}

		return RateLimitDecision{
			Allowed:   true,
			Limit:     limit.Requests,
			Used:      used,
			Remaining: limit.Requests - used,
			Scope:     scope,
			Key:       scopeKey,
		}, nil
	})
}

func rateLimitScopeKey(scope string, intent schemagate.IntentRequest) (string, error) {
	switch scope {
	case "tool":
		return intent.ToolName, nil
	case "identity":
		return intent.Context.Identity, nil
	case "tool_identity":
		return intent.ToolName + "|" + intent.Context.Identity, nil
	default:
		return "", fmt.Errorf("unsupported rate_limit scope: %s", scope)
	}
}

func loadRateLimitCounters(path string) (map[string]int, error) {
	if strings.TrimSpace(path) == "" {
		return map[string]int{}, nil
	}
	// #nosec G304 -- state path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int{}, nil
		}
		return nil, fmt.Errorf("read rate limit state: %w", err)
	}
	state := persistedRateLimitState{}
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("parse rate limit state: %w", err)
	}
	counters := make(map[string]int, len(state.Counters))
	for _, counter := range state.Counters {
		key := strings.TrimSpace(counter.Key)
		if key == "" || counter.Count <= 0 {
			continue
		}
		counters[key] = counter.Count
	}
	return counters, nil
}

func writeRateLimitCounters(path string, counters map[string]int) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create rate limit state directory: %w", err)
		}
	}

	keys := make([]string, 0, len(counters))
	for key, count := range counters {
		if strings.TrimSpace(key) == "" || count <= 0 {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]persistedRateLimitBucket, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, persistedRateLimitBucket{Key: key, Count: counters[key]})
	}
	state := persistedRateLimitState{
		SchemaID:      rateLimitStateSchemaID,
		SchemaVersion: rateLimitStateSchemaV1,
		Counters:      entries,
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal rate limit state: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := fsx.WriteFileAtomic(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write rate limit state: %w", err)
	}
	return nil
}

func pruneRateLimitCounters(counters map[string]int, window string, now time.Time) {
	keepBucket := rateLimitBucketStart(window, now).Format(time.RFC3339)
	for key := range counters {
		parts := strings.SplitN(key, "|", 4)
		if len(parts) != 4 {
			delete(counters, key)
			continue
		}
		if parts[0] != window {
			continue
		}
		if parts[2] == keepBucket {
			continue
		}
		delete(counters, key)
	}
}

func rateLimitBucketStart(window string, now time.Time) time.Time {
	switch window {
	case "hour":
		return now.UTC().Truncate(time.Hour)
	default:
		return now.UTC().Truncate(time.Minute)
	}
}

func withRateLimitLock(statePath string, fn func() (RateLimitDecision, error)) (RateLimitDecision, error) {
	if strings.TrimSpace(statePath) == "" {
		return fn()
	}
	lockPath := statePath + ".lock"
	deadline := time.Now().Add(rateLimitLockTimeout)
	for {
		// #nosec G304 -- lock path is derived from explicit local state path configuration.
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if writeErr := writeRateLimitLockMetadata(lockFile, time.Now().UTC()); writeErr != nil {
				_ = lockFile.Close()
				_ = os.Remove(lockPath)
				return RateLimitDecision{}, coreerrors.Wrap(
					fmt.Errorf("write rate limit lock metadata: %w", writeErr),
					coreerrors.CategoryIOFailure,
					"rate_limit_lock_write_failed",
					"check write permissions for the rate limit state directory",
					false,
				)
			}
			_ = lockFile.Close()
			defer func() {
				_ = os.Remove(lockPath)
			}()
			return fn()
		}
		if !isRateLimitLockContention(err, lockPath) && !isWindowsAccessDeniedLockError(err) {
			return RateLimitDecision{}, coreerrors.Wrap(
				fmt.Errorf("acquire rate limit lock: %w", err),
				coreerrors.CategoryIOFailure,
				"rate_limit_lock_acquire_failed",
				"check write permissions for the rate limit state directory",
				false,
			)
		}
		stale, staleErr := isRateLimitLockStale(lockPath, time.Now().UTC())
		if staleErr == nil && stale {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return RateLimitDecision{}, coreerrors.Wrap(
				fmt.Errorf("acquire rate limit lock: timeout"),
				coreerrors.CategoryStateContention,
				"rate_limit_lock_timeout",
				"retry after contention subsides",
				true,
			)
		}
		time.Sleep(rateLimitLockRetry)
	}
}

func isRateLimitLockContention(acquireErr error, lockPath string) bool {
	if os.IsExist(acquireErr) {
		return true
	}
	if !os.IsPermission(acquireErr) {
		return false
	}
	_, statErr := os.Stat(lockPath)
	return statErr == nil
}

func isWindowsAccessDeniedLockError(acquireErr error) bool {
	if runtime.GOOS != "windows" || acquireErr == nil {
		return false
	}
	return strings.Contains(strings.ToLower(acquireErr.Error()), "access is denied")
}

func writeRateLimitLockMetadata(lockFile *os.File, createdAt time.Time) error {
	metadata := rateLimitLockMetadata{
		SchemaID:      "gait.gate.rate_limit_lock",
		SchemaVersion: "1.0.0",
		PID:           os.Getpid(),
		CreatedAt:     createdAt.UTC(),
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal lock metadata: %w", err)
	}
	encoded = append(encoded, '\n')
	if _, err := lockFile.Write(encoded); err != nil {
		return fmt.Errorf("write lock metadata: %w", err)
	}
	if err := lockFile.Sync(); err != nil {
		return fmt.Errorf("sync lock metadata: %w", err)
	}
	return nil
}

func isRateLimitLockStale(lockPath string, now time.Time) (bool, error) {
	info, err := os.Stat(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	lockAge := now.Sub(info.ModTime().UTC())
	if lockAge > rateLimitLockStaleAfter {
		return true, nil
	}
	// #nosec G304 -- lock path is derived from explicit local state path configuration.
	content, err := os.ReadFile(lockPath)
	if err != nil {
		return false, err
	}
	metadata := rateLimitLockMetadata{}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return lockAge > rateLimitLockStaleAfter, nil
	}
	if metadata.CreatedAt.IsZero() {
		return lockAge > rateLimitLockStaleAfter, nil
	}
	return now.Sub(metadata.CreatedAt.UTC()) > rateLimitLockStaleAfter, nil
}
