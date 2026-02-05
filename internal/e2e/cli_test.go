package e2e

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIDemoVerify(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)

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

	regressInit := exec.Command(binPath, "regress", "init", "--from", "run_demo", "--json")
	regressInit.Dir = workDir
	regressOut, err := regressInit.CombinedOutput()
	if err != nil {
		t.Fatalf("gait regress init failed: %v\n%s", err, string(regressOut))
	}
	var regressResult struct {
		OK          bool   `json:"ok"`
		RunID       string `json:"run_id"`
		ConfigPath  string `json:"config_path"`
		RunpackPath string `json:"runpack_path"`
	}
	if err := json.Unmarshal(regressOut, &regressResult); err != nil {
		t.Fatalf("parse regress init json output: %v\n%s", err, string(regressOut))
	}
	if !regressResult.OK || regressResult.RunID != "run_demo" {
		t.Fatalf("unexpected regress result: %s", string(regressOut))
	}
	if regressResult.ConfigPath != "gait.yaml" {
		t.Fatalf("unexpected config path: %s", regressResult.ConfigPath)
	}
	if regressResult.RunpackPath != "fixtures/run_demo/runpack.zip" {
		t.Fatalf("unexpected runpack path: %s", regressResult.RunpackPath)
	}
	if _, err := os.Stat(filepath.Join(workDir, "gait.yaml")); err != nil {
		t.Fatalf("expected gait.yaml to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "fixtures", "run_demo", "runpack.zip")); err != nil {
		t.Fatalf("expected fixture runpack to exist: %v", err)
	}

	regressRun := exec.Command(binPath, "regress", "run", "--json", "--junit", "junit.xml")
	regressRun.Dir = workDir
	regressRunOut, err := regressRun.CombinedOutput()
	if err != nil {
		t.Fatalf("gait regress run failed: %v\n%s", err, string(regressRunOut))
	}
	var regressRunResult struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
		Output string `json:"output"`
		JUnit  string `json:"junit"`
	}
	if err := json.Unmarshal(regressRunOut, &regressRunResult); err != nil {
		t.Fatalf("parse regress run json output: %v\n%s", err, string(regressRunOut))
	}
	if !regressRunResult.OK || regressRunResult.Status != "pass" {
		t.Fatalf("unexpected regress run result: %s", string(regressRunOut))
	}
	if regressRunResult.Output != "regress_result.json" {
		t.Fatalf("unexpected regress output path: %s", regressRunResult.Output)
	}
	if regressRunResult.JUnit != "junit.xml" {
		t.Fatalf("unexpected junit output path: %s", regressRunResult.JUnit)
	}
	if _, err := os.Stat(filepath.Join(workDir, "regress_result.json")); err != nil {
		t.Fatalf("expected regress_result.json to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "junit.xml")); err != nil {
		t.Fatalf("expected junit.xml to exist: %v", err)
	}
}

func TestCLIRegressExitCodes(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)

	workDir := t.TempDir()
	demo := exec.Command(binPath, "demo")
	demo.Dir = workDir
	if out, err := demo.CombinedOutput(); err != nil {
		t.Fatalf("gait demo failed: %v\n%s", err, string(out))
	}

	regressInit := exec.Command(binPath, "regress", "init", "--from", "run_demo", "--json")
	regressInit.Dir = workDir
	if out, err := regressInit.CombinedOutput(); err != nil {
		t.Fatalf("gait regress init failed: %v\n%s", err, string(out))
	}

	fixtureMetaPath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	rawMeta, err := os.ReadFile(fixtureMetaPath)
	if err != nil {
		t.Fatalf("read fixture metadata: %v", err)
	}
	var fixtureMeta map[string]any
	if err := json.Unmarshal(rawMeta, &fixtureMeta); err != nil {
		t.Fatalf("parse fixture metadata: %v", err)
	}
	fixtureMeta["expected_replay_exit_code"] = 2
	updatedMeta, err := json.MarshalIndent(fixtureMeta, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture metadata: %v", err)
	}
	updatedMeta = append(updatedMeta, '\n')
	if err := os.WriteFile(fixtureMetaPath, updatedMeta, 0o600); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}

	regressFail := exec.Command(binPath, "regress", "run", "--json")
	regressFail.Dir = workDir
	failOut, err := regressFail.CombinedOutput()
	if err == nil {
		t.Fatalf("expected regress run to fail with exit code 5")
	}
	if code := commandExitCode(t, err); code != 5 {
		t.Fatalf("unexpected regress failure exit code: got=%d want=5 output=%s", code, string(failOut))
	}
	var failResult struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(failOut, &failResult); err != nil {
		t.Fatalf("parse regress fail json: %v\n%s", err, string(failOut))
	}
	if failResult.OK || failResult.Status != "fail" {
		t.Fatalf("unexpected regress fail output: %s", string(failOut))
	}

	regressInvalid := exec.Command(binPath, "regress", "run", "--config", "missing.yaml", "--json")
	regressInvalid.Dir = workDir
	invalidOut, err := regressInvalid.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invalid regress invocation to fail with exit code 6")
	}
	if code := commandExitCode(t, err); code != 6 {
		t.Fatalf("unexpected invalid-input exit code: got=%d want=6 output=%s", code, string(invalidOut))
	}
}

func buildGaitBinary(t *testing.T, root string) string {
	t.Helper()
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
	return binPath
}

func commandExitCode(t *testing.T, err error) int {
	t.Helper()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected command exit error, got: %v", err)
	}
	return exitErr.ExitCode()
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
