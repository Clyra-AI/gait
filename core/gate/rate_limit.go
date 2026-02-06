package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
)

const (
	rateLimitStateSchemaID = "gait.gate.rate_limit_state"
	rateLimitStateSchemaV1 = "1.0.0"
	rateLimitLockTimeout   = 3 * time.Second
	rateLimitLockRetry     = 15 * time.Millisecond
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
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
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
			_ = lockFile.Close()
			defer func() {
				_ = os.Remove(lockPath)
			}()
			return fn()
		}
		if !os.IsExist(err) {
			return RateLimitDecision{}, fmt.Errorf("acquire rate limit lock: %w", err)
		}
		if time.Now().After(deadline) {
			return RateLimitDecision{}, fmt.Errorf("acquire rate limit lock: timeout")
		}
		time.Sleep(rateLimitLockRetry)
	}
}
