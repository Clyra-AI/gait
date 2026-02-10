package fsx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileAtomicCreatesAndOverwrites(t *testing.T) {
	target := filepath.Join(t.TempDir(), "state.json")

	if err := WriteFileAtomic(target, []byte("first\n"), 0o600); err != nil {
		t.Fatalf("first write: %v", err)
	}
	first, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read first write: %v", err)
	}
	if string(first) != "first\n" {
		t.Fatalf("unexpected first content: %q", string(first))
	}

	if err := WriteFileAtomic(target, []byte("second\n"), 0o600); err != nil {
		t.Fatalf("second write: %v", err)
	}
	second, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read second write: %v", err)
	}
	if string(second) != "second\n" {
		t.Fatalf("unexpected second content: %q", string(second))
	}
}

func TestWriteFileAtomicMode(t *testing.T) {
	target := filepath.Join(t.TempDir(), "secure.json")

	if err := WriteFileAtomic(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if runtime.GOOS == "windows" {
		if info.Mode().Perm()&0o600 != 0o600 {
			t.Fatalf("expected owner read/write bits set on windows, got %#o", info.Mode().Perm())
		}
		return
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600 got %#o", info.Mode().Perm())
	}
}

func TestWriteFileAtomicMissingParent(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "missing", "state.json")

	if err := WriteFileAtomic(target, []byte("{}\n"), 0o600); err == nil {
		t.Fatal("expected error when parent directory does not exist")
	}
}

func TestWriteFileAtomicRenameFailureKeepsTempClean(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "state.json")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatalf("write target child: %v", err)
	}

	if err := WriteFileAtomic(target, []byte("data\n"), 0o600); err == nil {
		t.Fatal("expected rename error when destination is a directory")
	}

	matches, err := filepath.Glob(filepath.Join(workDir, ".state.json.tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no temp files left behind, found: %v", matches)
	}
}

func TestRenameWithWindowsFallbackReplacesDirectoryDestination(t *testing.T) {
	workDir := t.TempDir()
	tempPath := filepath.Join(workDir, "tmp.txt")
	destPath := filepath.Join(workDir, "dest")

	if err := os.WriteFile(tempPath, []byte("payload\n"), 0o600); err != nil {
		t.Fatalf("write temp source: %v", err)
	}
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		t.Fatalf("mkdir destination directory: %v", err)
	}

	if err := renameWithWindowsFallback(tempPath, destPath, "windows"); err != nil {
		t.Fatalf("renameWithWindowsFallback: %v", err)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read destination file: %v", err)
	}
	if string(content) != "payload\n" {
		t.Fatalf("unexpected destination content: %q", string(content))
	}
}

func TestRenameWithWindowsFallbackNonEmptyDirectoryRemoveError(t *testing.T) {
	workDir := t.TempDir()
	tempPath := filepath.Join(workDir, "tmp.txt")
	destPath := filepath.Join(workDir, "dest")

	if err := os.WriteFile(tempPath, []byte("payload\n"), 0o600); err != nil {
		t.Fatalf("write temp source: %v", err)
	}
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		t.Fatalf("mkdir destination directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(destPath, "keep.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatalf("write destination child: %v", err)
	}

	if err := renameWithWindowsFallback(tempPath, destPath, "windows"); err == nil {
		t.Fatal("expected remove error for non-empty destination directory")
	}
}

func TestRenameWithWindowsFallbackSecondRenameError(t *testing.T) {
	workDir := t.TempDir()
	missingSource := filepath.Join(workDir, "missing.txt")
	destPath := filepath.Join(workDir, "dest")

	if err := os.MkdirAll(destPath, 0o755); err != nil {
		t.Fatalf("mkdir destination directory: %v", err)
	}

	if err := renameWithWindowsFallback(missingSource, destPath, "windows"); err == nil {
		t.Fatal("expected second rename error when source file is missing")
	}
}

func TestRenameWithWindowsFallbackNonWindowsSuccess(t *testing.T) {
	workDir := t.TempDir()
	tempPath := filepath.Join(workDir, "tmp.txt")
	destPath := filepath.Join(workDir, "dest.txt")

	if err := os.WriteFile(tempPath, []byte("data\n"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}

	if err := renameWithWindowsFallback(tempPath, destPath, "linux"); err != nil {
		t.Fatalf("rename linux: %v", err)
	}
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(content) != "data\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestRenameWithWindowsFallbackNonWindowsError(t *testing.T) {
	workDir := t.TempDir()
	tempPath := filepath.Join(workDir, "tmp.txt")
	destPath := filepath.Join(workDir, "nodir", "dest.txt")

	if err := os.WriteFile(tempPath, []byte("data\n"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}

	if err := renameWithWindowsFallback(tempPath, destPath, "linux"); err == nil {
		t.Fatal("expected rename error for missing parent on linux")
	}
}

func TestWriteFileAtomicEmptyContent(t *testing.T) {
	target := filepath.Join(t.TempDir(), "empty.json")

	if err := WriteFileAtomic(target, []byte{}, 0o600); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read empty: %v", err)
	}
	if len(content) != 0 {
		t.Fatalf("expected empty file, got %d bytes", len(content))
	}
}

func TestWriteFileAtomicNoTempLeftOnSuccess(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "clean.json")

	if err := WriteFileAtomic(target, []byte("ok\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(workDir, ".clean.json.tmp-*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no temp files after success, found: %v", matches)
	}
}

func TestWriteFileAtomicOverwritePreservesMode(t *testing.T) {
	target := filepath.Join(t.TempDir(), "mode.json")

	if err := WriteFileAtomic(target, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteFileAtomic(target, []byte("second\n"), 0o600); err != nil {
		t.Fatalf("second write: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600 after overwrite, got %#o", info.Mode().Perm())
	}
}

func TestWriteFileAtomicRelativePath(t *testing.T) {
	workDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.MkdirAll("nested", 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	if err := WriteFileAtomic(filepath.Join("nested", "state.json"), []byte("ok\n"), 0o600); err != nil {
		t.Fatalf("write relative path: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(workDir, "nested", "state.json"))
	if err != nil {
		t.Fatalf("read relative path output: %v", err)
	}
	if string(content) != "ok\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestWriteFileAtomicRejectsTraversalPath(t *testing.T) {
	if err := WriteFileAtomic(filepath.Join("..", "escape.json"), []byte("x"), 0o600); err == nil {
		t.Fatal("expected traversal path error")
	}
}

func TestRenameWithWindowsFallbackRelativePaths(t *testing.T) {
	workDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	tempPath := "tmp.txt"
	destPath := "dest.txt"
	if err := os.WriteFile(tempPath, []byte("data\n"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := renameWithWindowsFallback(tempPath, destPath, "linux"); err != nil {
		t.Fatalf("rename relative: %v", err)
	}
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(content) != "data\n" {
		t.Fatalf("unexpected destination content: %q", string(content))
	}
}

func TestRenameWithWindowsFallbackRejectsTraversalDestination(t *testing.T) {
	workDir := t.TempDir()
	tempPath := filepath.Join(workDir, "tmp.txt")
	if err := os.WriteFile(tempPath, []byte("data\n"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := renameWithWindowsFallback(tempPath, filepath.Join("..", "dest.txt"), "linux"); err == nil {
		t.Fatal("expected destination path validation error")
	}
}

func TestRenameWithWindowsFallbackRejectsTraversalTempPath(t *testing.T) {
	workDir := t.TempDir()
	destPath := filepath.Join(workDir, "dest.txt")
	if err := renameWithWindowsFallback(filepath.Join("..", "tmp.txt"), destPath, "linux"); err == nil {
		t.Fatal("expected temp path validation error")
	}
}
