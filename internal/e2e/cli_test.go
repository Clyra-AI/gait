package e2e

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIDemoVerify(t *testing.T) {
	root := repoRoot(t)
	binDir := t.TempDir()
	binName := "gait"
	if runtime.GOOS == "windows" {
		binName = "gait.exe"
	}
	binPath := filepath.Join(binDir, binName)

	build := exec.Command("go", "build", "-o", binPath, "./cmd/gait")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gait: %v\n%s", err, string(out))
	}

	workDir := t.TempDir()
	demo := exec.Command(binPath, "demo")
	demo.Dir = workDir
	demoOut, err := demo.CombinedOutput()
	if err != nil {
		t.Fatalf("gait demo failed: %v\n%s", err, string(demoOut))
	}
	if !strings.Contains(string(demoOut), "run_id=") || !strings.Contains(string(demoOut), "verify=ok") {
		t.Fatalf("unexpected demo output: %s", string(demoOut))
	}

	verify := exec.Command(binPath, "verify", "run_demo")
	verify.Dir = workDir
	verifyOut, err := verify.CombinedOutput()
	if err != nil {
		t.Fatalf("gait verify failed: %v\n%s", err, string(verifyOut))
	}
	if !strings.Contains(string(verifyOut), "verify ok") {
		t.Fatalf("unexpected verify output: %s", string(verifyOut))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to locate test file")
	}
	dir := filepath.Dir(filename)
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}
