package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const expectedReplayExitCodeMismatch = 2

func TestRegressBootstrapOneCommand(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup: expected %d got %d", exitOK, code)
	}

	junitPath := filepath.Join(workDir, "junit_bootstrap.xml")
	var exitCode int
	output := captureStdout(t, func() {
		exitCode = runRegressBootstrap([]string{"--from", "run_demo", "--junit", junitPath, "--json"})
	})
	if exitCode != exitOK {
		t.Fatalf("runRegressBootstrap: expected %d got %d", exitOK, exitCode)
	}

	var decoded regressBootstrapOutput
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode regress bootstrap output: %v", err)
	}
	if !decoded.OK {
		t.Fatalf("expected bootstrap success: %#v", decoded)
	}
	if decoded.RunID != "run_demo" {
		t.Fatalf("expected run_id run_demo got %s", decoded.RunID)
	}
	if decoded.Status != regressStatusPass {
		t.Fatalf("expected pass status got %s", decoded.Status)
	}
	if decoded.Failed != 0 {
		t.Fatalf("expected zero failed graders got %d", decoded.Failed)
	}
	if len(decoded.ArtifactPaths) != 2 {
		t.Fatalf("expected two artifact paths got %#v", decoded.ArtifactPaths)
	}
	if _, err := os.Stat(filepath.Join(workDir, "fixtures", "run_demo", "runpack.zip")); err != nil {
		t.Fatalf("expected fixture runpack: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "gait.yaml")); err != nil {
		t.Fatalf("expected gait config: %v", err)
	}
}

func TestRegressFailureSummariesAreActionable(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup: expected %d got %d", exitOK, code)
	}
	if code := runRegressInit([]string{"--from", "run_demo", "--json"}); code != exitOK {
		t.Fatalf("runRegressInit setup: expected %d got %d", exitOK, code)
	}

	fixturePath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	mutateFixtureExpectedReplayExitCode(t, fixturePath, expectedReplayExitCodeMismatch)

	var regressRunCode int
	regressRunOutputRaw := captureStdout(t, func() {
		regressRunCode = runRegressRun([]string{"--json", "--junit", "junit_fail.xml"})
	})
	if regressRunCode != exitRegressFailed {
		t.Fatalf("runRegressRun fail: expected %d got %d", exitRegressFailed, regressRunCode)
	}

	var regressRunResult regressRunOutput
	if err := json.Unmarshal([]byte(regressRunOutputRaw), &regressRunResult); err != nil {
		t.Fatalf("decode regress run output: %v", err)
	}
	if regressRunResult.OK {
		t.Fatalf("expected regress run failure output: %#v", regressRunResult)
	}
	if regressRunResult.TopFailureReason == "" {
		t.Fatalf("expected top failure reason in regress run output")
	}
	if regressRunResult.NextCommand != "gait regress run --json" {
		t.Fatalf("unexpected next command: %s", regressRunResult.NextCommand)
	}
	if len(regressRunResult.ArtifactPaths) != 2 {
		t.Fatalf("expected artifact pointers for regress run output: %#v", regressRunResult.ArtifactPaths)
	}

	var bootstrapCode int
	bootstrapOutputRaw := captureStdout(t, func() {
		bootstrapCode = runRegressBootstrap([]string{"--from", "run_demo", "--name", "run_demo_bootstrap", "--json", "--junit", "junit_bootstrap_fail.xml"})
	})
	if bootstrapCode != exitRegressFailed {
		t.Fatalf("runRegressBootstrap fail: expected %d got %d", exitRegressFailed, bootstrapCode)
	}

	var bootstrapResult regressBootstrapOutput
	if err := json.Unmarshal([]byte(bootstrapOutputRaw), &bootstrapResult); err != nil {
		t.Fatalf("decode regress bootstrap output: %v", err)
	}
	if bootstrapResult.OK {
		t.Fatalf("expected regress bootstrap failure output: %#v", bootstrapResult)
	}
	if bootstrapResult.TopFailureReason == "" {
		t.Fatalf("expected top failure reason in bootstrap output")
	}
	if bootstrapResult.NextCommand != "gait regress run --json" {
		t.Fatalf("unexpected bootstrap next command: %s", bootstrapResult.NextCommand)
	}
	if len(bootstrapResult.ArtifactPaths) != 2 {
		t.Fatalf("expected artifact pointers for bootstrap output: %#v", bootstrapResult.ArtifactPaths)
	}
}

func mutateFixtureExpectedReplayExitCode(t *testing.T, fixturePath string, expectedExitCode int) {
	t.Helper()

	content, err := os.ReadFile(fixturePath) // #nosec G304 -- test fixture path is deterministic and local.
	if err != nil {
		t.Fatalf("read fixture metadata: %v", err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatalf("decode fixture metadata: %v", err)
	}
	fixture["expected_replay_exit_code"] = expectedExitCode
	encoded, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatalf("encode fixture metadata: %v", err)
	}
	if err := os.WriteFile(fixturePath, append(encoded, '\n'), 0o600); err != nil {
		t.Fatalf("write fixture metadata: %v", err)
	}
}
