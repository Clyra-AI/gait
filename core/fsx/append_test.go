package fsx

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAppendLineLockedWritesOneLinePerCall(t *testing.T) {
	workDir := t.TempDir()
	targetPath := filepath.Join(workDir, "events.jsonl")
	if err := AppendLineLocked(targetPath, []byte(`{"event":"a"}`), 0o600); err != nil {
		t.Fatalf("append first line: %v", err)
	}
	if err := AppendLineLocked(targetPath, []byte(`{"event":"b"}`), 0o600); err != nil {
		t.Fatalf("append second line: %v", err)
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	expected := "{\"event\":\"a\"}\n{\"event\":\"b\"}\n"
	if string(raw) != expected {
		t.Fatalf("unexpected append output:\n%s", string(raw))
	}
}

func TestAppendLineLockedRejectsTraversal(t *testing.T) {
	if err := AppendLineLocked(filepath.Join("..", "escape.jsonl"), []byte(`{"ok":true}`), 0o600); err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}
}

func TestAppendLineLockedConcurrentJSONLIntegrity(t *testing.T) {
	workDir := t.TempDir()
	targetPath := filepath.Join(workDir, "concurrent.jsonl")
	const writers = 200
	var group sync.WaitGroup
	group.Add(writers)
	for index := 0; index < writers; index++ {
		line := []byte(fmt.Sprintf(`{"idx":%d}`, index))
		go func(payload []byte) {
			defer group.Done()
			if err := AppendLineLocked(targetPath, payload, 0o600); err != nil {
				t.Errorf("append line: %v", err)
			}
		}(line)
	}
	group.Wait()

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read concurrent target: %v", err)
	}
	lines := 0
	for _, entry := range splitLines(raw) {
		lines++
		var parsed map[string]any
		if err := json.Unmarshal([]byte(entry), &parsed); err != nil {
			t.Fatalf("invalid json line %d: %v (%q)", lines, err, entry)
		}
	}
	if lines != writers {
		t.Fatalf("unexpected line count: got=%d want=%d", lines, writers)
	}
}

func TestAppendPayloadCapacity(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		lineLength int
		want       int
		expectErr  bool
	}{
		{name: "zero", lineLength: 0, want: 1},
		{name: "normal", lineLength: 42, want: 43},
		{name: "negative", lineLength: -1, expectErr: true},
		{name: "maxint", lineLength: maxInt, expectErr: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := appendPayloadCapacity(testCase.lineLength)
			if testCase.expectErr {
				if err == nil {
					t.Fatalf("expected error for lineLength=%d", testCase.lineLength)
				}
				return
			}
			if err != nil {
				t.Fatalf("appendPayloadCapacity(%d): %v", testCase.lineLength, err)
			}
			if got != testCase.want {
				t.Fatalf("capacity mismatch: expected %d got %d", testCase.want, got)
			}
		})
	}
}

func TestIsAppendLockContention(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "append.lock")
	permissionErr := &os.PathError{Op: "open", Path: lockPath, Err: os.ErrPermission}
	accessDeniedErr := &os.PathError{Op: "open", Path: lockPath, Err: errors.New("Access is denied.")}

	if !isAppendLockContention(os.ErrExist, lockPath) {
		t.Fatalf("expected os.ErrExist to be treated as lock contention")
	}
	if isAppendLockContention(permissionErr, lockPath) {
		t.Fatalf("expected permission error without lock file to be non-contention")
	}
	if isAppendLockContention(accessDeniedErr, lockPath) {
		t.Fatalf("expected non-exist lock path to be non-contention")
	}
	if err := os.WriteFile(lockPath, []byte("lock"), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	if !isAppendLockContention(permissionErr, lockPath) {
		t.Fatalf("expected permission error with existing lock file to be contention")
	}
	if !isAppendLockContention(accessDeniedErr, lockPath) {
		t.Fatalf("expected access denied error with existing lock file to be contention")
	}
	missingLockPath := filepath.Join(t.TempDir(), "missing.lock")
	if isAppendLockContention(os.ErrNotExist, missingLockPath) {
		t.Fatalf("expected unrelated error to be non-contention")
	}
}

func splitLines(raw []byte) []string {
	lines := make([]string, 0)
	current := make([]byte, 0)
	for _, b := range raw {
		if b == '\n' {
			if len(current) > 0 {
				lines = append(lines, string(current))
				current = current[:0]
			}
			continue
		}
		current = append(current, b)
	}
	if len(current) > 0 {
		lines = append(lines, string(current))
	}
	return lines
}
