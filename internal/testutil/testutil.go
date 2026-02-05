package testutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func RepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to locate testutil source file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func BuildGaitBinary(t *testing.T, root string) string {
	t.Helper()
	binDir := t.TempDir()
	binName := "gait"
	if runtime.GOOS == "windows" {
		binName = "gait.exe"
	}
	binPath := filepath.Join(binDir, binName)

	// #nosec G204 -- arguments are fixed and used only in test binaries.
	build := exec.Command("go", "build", "-o", binPath, "./cmd/gait")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gait binary: %v\n%s", err, string(out))
	}
	return binPath
}

func CommandExitCode(t *testing.T, err error) int {
	t.Helper()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected command exit error, got: %v", err)
	}
	return exitErr.ExitCode()
}

func WriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create parent directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func AssertGoldenJSON(t *testing.T, repoRelativePath string, value any) {
	t.Helper()
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden json: %v", err)
	}
	encoded = append(encoded, '\n')

	goldenPath := filepath.Join(RepoRoot(t), filepath.FromSlash(repoRelativePath))
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o750); err != nil {
			t.Fatalf("create golden directory: %v", err)
		}
		if err := os.WriteFile(goldenPath, encoded, 0o600); err != nil {
			t.Fatalf("update golden fixture: %v", err)
		}
		return
	}

	// #nosec G304 -- path is resolved from repo root plus test-owned relative fixture.
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden fixture %s: %v", goldenPath, err)
	}
	if bytes.Equal(normalizeNewlines(expected), normalizeNewlines(encoded)) {
		return
	}

	t.Fatalf(
		"golden mismatch for %s\nexpected:\n%s\nactual:\n%s\nset UPDATE_GOLDEN=1 to refresh fixtures",
		goldenPath,
		string(expected),
		string(encoded),
	)
}

func normalizeNewlines(raw []byte) []byte {
	return bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
}

func WriteGoldenJSON(t *testing.T, repoRelativePath string, value any) {
	t.Helper()
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden json: %v", err)
	}
	encoded = append(encoded, '\n')
	fullPath := filepath.Join(RepoRoot(t), filepath.FromSlash(repoRelativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		t.Fatalf("create golden fixture directory: %v", err)
	}
	if err := os.WriteFile(fullPath, encoded, 0o600); err != nil {
		t.Fatalf("write golden fixture: %v", err)
	}
}

func MustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- test helper for controlled paths.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

func FormatJSON(raw []byte) string {
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return string(raw)
	}
	encoded, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return string(raw)
	}
	return fmt.Sprintf("%s\n", string(encoded))
}
