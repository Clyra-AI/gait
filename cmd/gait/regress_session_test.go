package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestRegressInitFromSessionChain(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	journalPath := filepath.Join(workDir, "sessions", "demo.journal.jsonl")
	checkpointPath := filepath.Join(workDir, "gait-out", "runpack_demo_cp_0001.zip")
	chainPath := filepath.Join(workDir, "sessions", "demo.journal_chain.json")

	if code := runCommand([]string{
		"session", "start",
		"--journal", journalPath,
		"--session-id", "sess_demo",
		"--run-id", "run_demo",
		"--json",
	}); code != exitOK {
		t.Fatalf("run session start expected %d got %d", exitOK, code)
	}
	if code := runCommand([]string{
		"session", "append",
		"--journal", journalPath,
		"--tool", "tool.write",
		"--verdict", "allow",
		"--intent-id", "intent_1",
		"--json",
	}); code != exitOK {
		t.Fatalf("run session append expected %d got %d", exitOK, code)
	}
	if code := runCommand([]string{
		"session", "checkpoint",
		"--journal", journalPath,
		"--out", checkpointPath,
		"--chain-out", chainPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("run session checkpoint expected %d got %d", exitOK, code)
	}

	if code := runRegressInit([]string{
		"--from", chainPath,
		"--checkpoint", "latest",
		"--name", "fixture_from_chain",
		"--json",
	}); code != exitOK {
		t.Fatalf("runRegressInit from session chain expected %d got %d", exitOK, code)
	}
}

func TestRegressBootstrapFromPackArtifact(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("run demo expected %d got %d", exitOK, code)
	}

	packPath := filepath.Join(workDir, "pack_run_bootstrap.zip")
	if code := runPack([]string{
		"build",
		"--type", "run",
		"--from", "run_demo",
		"--out", packPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("pack build expected %d got %d", exitOK, code)
	}

	var bootstrapCode int
	raw := captureStdout(t, func() {
		bootstrapCode = runRegressBootstrap([]string{
			"--from", packPath,
			"--name", "fixture_from_pack",
			"--json",
		})
	})
	if bootstrapCode != exitOK {
		t.Fatalf("regress bootstrap from pack expected %d got %d output=%s", exitOK, bootstrapCode, raw)
	}
	var output regressBootstrapOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode regress bootstrap output: %v raw=%s", err, raw)
	}
	if !output.OK || output.RunID != "run_demo" {
		t.Fatalf("unexpected regress bootstrap output: %#v", output)
	}
}
