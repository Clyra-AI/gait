package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNormalizeNewlines(t *testing.T) {
	input := []byte("{\r\n  \"ok\": true\r\n}\r\n")
	expected := []byte("{\n  \"ok\": true\n}\n")

	actual := normalizeNewlines(input)
	if !bytes.Equal(actual, expected) {
		t.Fatalf("unexpected newline normalization: got=%q want=%q", string(actual), string(expected))
	}
}

func TestRepoRootContainsGoMod(t *testing.T) {
	root := RepoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected go.mod at repo root: %v", err)
	}
}

func TestBuildGaitBinary(t *testing.T) {
	root := RepoRoot(t)
	binPath := BuildGaitBinary(t, root)
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("expected built binary to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty binary at %s", binPath)
	}
}

func TestWriteFileAndMustReadFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "output.json")
	WriteFile(t, target, []byte(`{"ok":true}`))
	got := MustReadFile(t, target)
	if string(got) != `{"ok":true}` {
		t.Fatalf("unexpected file content: %q", string(got))
	}
}

func TestFormatJSON(t *testing.T) {
	formatted := FormatJSON([]byte(`{"ok":true}`))
	if !strings.Contains(formatted, "\"ok\": true") {
		t.Fatalf("expected pretty-printed json, got=%q", formatted)
	}

	raw := "not-json"
	if got := FormatJSON([]byte(raw)); got != raw {
		t.Fatalf("expected raw passthrough for invalid json, got=%q", got)
	}
}

func TestCommandExitCode(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "exit 7")
	} else {
		cmd = exec.Command("sh", "-c", "exit 7")
	}
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command to fail")
	}
	if code := CommandExitCode(t, err); code != 7 {
		t.Fatalf("unexpected exit code: got=%d want=7", code)
	}
}

func TestGoldenHelpersRoundTrip(t *testing.T) {
	repoRoot := RepoRoot(t)
	name := strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	relativePath := filepath.Join(
		"internal",
		"testutil",
		"testdata",
		"tmp_"+name+"_"+time.Now().UTC().Format("20060102150405")+".json",
	)
	fullPath := filepath.Join(repoRoot, relativePath)
	t.Cleanup(func() {
		_ = os.Remove(fullPath)
		_ = os.Remove(filepath.Dir(fullPath))
	})

	payload := map[string]any{"ok": true, "count": 1}
	WriteGoldenJSON(t, relativePath, payload)
	AssertGoldenJSON(t, relativePath, payload)

	t.Setenv("UPDATE_GOLDEN", "1")
	AssertGoldenJSON(t, relativePath, map[string]any{"ok": true, "count": 2})
}
