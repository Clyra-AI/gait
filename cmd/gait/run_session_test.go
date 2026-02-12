package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

func TestRunSessionFlowAndVerifySessionChain(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	journalPath := filepath.Join(workDir, "sessions", "demo.journal.jsonl")
	checkpointPath := filepath.Join(workDir, "gait-out", "runpack_demo_cp_0001.zip")

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
		"--json",
	}); code != exitOK {
		t.Fatalf("run session checkpoint expected %d got %d", exitOK, code)
	}
	if _, err := os.Stat(checkpointPath); err != nil {
		t.Fatalf("expected checkpoint runpack to exist: %v", err)
	}

	chainPath := filepath.Join(workDir, "sessions", "demo.journal_chain.json")
	if _, err := os.Stat(chainPath); err != nil {
		t.Fatalf("expected session chain to exist: %v", err)
	}

	if code := runVerify([]string{
		"session-chain",
		"--chain", chainPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("verify session-chain expected %d got %d", exitOK, code)
	}
	if code := runInspect([]string{
		"--from", chainPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("run inspect session-chain expected %d got %d", exitOK, code)
	}
}

func TestRunSessionStatusAndHelpPaths(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	journalPath := filepath.Join(workDir, "sessions", "status.journal.jsonl")

	if code := runSession([]string{}); code != exitInvalidInput {
		t.Fatalf("runSession without args expected %d got %d", exitInvalidInput, code)
	}
	if code := runSession([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runSession unknown subcommand expected %d got %d", exitInvalidInput, code)
	}
	if code := runSession([]string{"start", "--help"}); code != exitOK {
		t.Fatalf("runSession start help expected %d got %d", exitOK, code)
	}
	if code := runSession([]string{"append", "--help"}); code != exitOK {
		t.Fatalf("runSession append help expected %d got %d", exitOK, code)
	}
	if code := runSession([]string{"status", "--help"}); code != exitOK {
		t.Fatalf("runSession status help expected %d got %d", exitOK, code)
	}
	if code := runSession([]string{"checkpoint", "--help"}); code != exitOK {
		t.Fatalf("runSession checkpoint help expected %d got %d", exitOK, code)
	}
	if code := runSession([]string{"compact", "--help"}); code != exitOK {
		t.Fatalf("runSession compact help expected %d got %d", exitOK, code)
	}

	if code := runSessionStatus([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runSessionStatus missing journal expected %d got %d", exitInvalidInput, code)
	}
	if code := runSessionStart([]string{
		"--journal", journalPath,
		"--session-id", "sess_status",
		"--run-id", "run_status",
		"--json",
	}); code != exitOK {
		t.Fatalf("runSessionStart for status test expected %d got %d", exitOK, code)
	}
	if code := runSessionStatus([]string{"--journal", journalPath, "--json"}); code != exitOK {
		t.Fatalf("runSessionStatus expected %d got %d", exitOK, code)
	}

	if code := runSessionAppend([]string{
		"--journal", journalPath,
		"--tool", "tool.write",
		"--verdict", "unsupported",
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runSessionAppend unsupported verdict expected %d got %d", exitInvalidInput, code)
	}
	if code := runSessionCheckpoint([]string{
		"--journal", journalPath,
		"--out", filepath.Join(workDir, "gait-out", "status_cp.zip"),
		"--chain-out", filepath.Join(workDir, "gait-out", "status_chain.json"),
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runSessionCheckpoint without events expected %d got %d", exitInvalidInput, code)
	}
	if code := runSessionCompact([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runSessionCompact missing journal expected %d got %d", exitInvalidInput, code)
	}
}

func TestWriteRunSessionOutputTextModes(t *testing.T) {
	now := time.Date(2026, time.February, 11, 3, 0, 0, 0, time.UTC)
	text := captureStdout(t, func() {
		if code := writeRunSessionOutput(false, runSessionOutput{
			OK:        false,
			Operation: "start",
			Error:     "boom",
		}, exitInternalFailure); code != exitInternalFailure {
			t.Fatalf("writeRunSessionOutput error expected exit %d got %d", exitInternalFailure, code)
		}
		if code := writeRunSessionOutput(false, runSessionOutput{
			OK:        true,
			Operation: "status",
			Journal:   "/tmp/journal.jsonl",
			Status: &runpack.SessionStatus{
				SessionID:       "sess",
				RunID:           "run",
				EventCount:      2,
				CheckpointCount: 1,
				LastSequence:    2,
			},
		}, exitOK); code != exitOK {
			t.Fatalf("writeRunSessionOutput status expected exit %d got %d", exitOK, code)
		}
		if code := writeRunSessionOutput(false, runSessionOutput{
			OK:        true,
			Operation: "append",
			Journal:   "/tmp/journal.jsonl",
			Event: &schemarunpack.SessionEvent{
				Sequence: 3,
				ToolName: "tool.write",
				Verdict:  "allow",
			},
		}, exitOK); code != exitOK {
			t.Fatalf("writeRunSessionOutput append expected exit %d got %d", exitOK, code)
		}
		if code := writeRunSessionOutput(false, runSessionOutput{
			OK:        true,
			Operation: "checkpoint",
			ChainPath: "/tmp/chain.json",
			Checkpoint: &schemarunpack.SessionCheckpoint{
				CheckpointIndex: 2,
				RunpackPath:     "/tmp/cp2.zip",
				SequenceStart:   3,
				SequenceEnd:     4,
				CreatedAt:       now,
			},
		}, exitOK); code != exitOK {
			t.Fatalf("writeRunSessionOutput checkpoint expected exit %d got %d", exitOK, code)
		}
		if code := writeRunSessionOutput(false, runSessionOutput{
			OK:        true,
			Operation: "custom",
		}, exitOK); code != exitOK {
			t.Fatalf("writeRunSessionOutput fallback expected exit %d got %d", exitOK, code)
		}
		if code := writeRunSessionOutput(false, runSessionOutput{
			OK:        true,
			Operation: "compact",
			Compaction: &runpack.SessionCompactionResult{
				Compacted:    true,
				DryRun:       true,
				EventsBefore: 4,
				EventsAfter:  1,
				Checkpoints:  2,
				BytesBefore:  1000,
				BytesAfter:   200,
				JournalPath:  "/tmp/journal.jsonl",
				OutputPath:   "/tmp/compacted.jsonl",
			},
		}, exitOK); code != exitOK {
			t.Fatalf("writeRunSessionOutput compact expected exit %d got %d", exitOK, code)
		}
	})

	expectedSnippets := []string{
		"run session start error: boom",
		"session status:",
		"session append:",
		"session checkpoint:",
		"session compact:",
		"run session custom: ok",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(text, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, text)
		}
	}
}

func TestWriteRunSessionOutputSanitizesTextFields(t *testing.T) {
	text := captureStdout(t, func() {
		if code := writeRunSessionOutput(false, runSessionOutput{
			OK:        true,
			Operation: "append",
			Journal:   "/tmp/journal.jsonl\ninjected",
			Event: &schemarunpack.SessionEvent{
				Sequence: 10,
				ToolName: "tool.write\nINJECT",
				Verdict:  "allow\r\nBLOCK",
			},
		}, exitOK); code != exitOK {
			t.Fatalf("writeRunSessionOutput append expected exit %d got %d", exitOK, code)
		}
		if code := writeRunSessionOutput(false, runSessionOutput{
			OK:        true,
			Operation: "checkpoint",
			Checkpoint: &schemarunpack.SessionCheckpoint{
				CheckpointIndex: 1,
				RunpackPath:     "/tmp/cp.zip\nnext",
				SequenceStart:   1,
				SequenceEnd:     1,
			},
		}, exitOK); code != exitOK {
			t.Fatalf("writeRunSessionOutput checkpoint expected exit %d got %d", exitOK, code)
		}
	})

	if strings.Contains(text, "tool.write\nINJECT") {
		t.Fatalf("expected tool name control characters to be sanitized, got:\n%s", text)
	}
	if strings.Contains(text, "/tmp/cp.zip\nnext") {
		t.Fatalf("expected runpack path control characters to be sanitized, got:\n%s", text)
	}
	if !strings.Contains(text, "tool=tool.write INJECT verdict=allow BLOCK") {
		t.Fatalf("expected sanitized append line in output, got:\n%s", text)
	}
	if !strings.Contains(text, "runpack=/tmp/cp.zip next") {
		t.Fatalf("expected sanitized checkpoint line in output, got:\n%s", text)
	}
}

func TestRunSessionCompactFlow(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	journalPath := filepath.Join(workDir, "sessions", "compact.journal.jsonl")
	checkpointPath := filepath.Join(workDir, "gait-out", "compact_cp_0001.zip")

	if code := runSessionStart([]string{
		"--journal", journalPath,
		"--session-id", "sess_compact",
		"--run-id", "run_compact",
		"--json",
	}); code != exitOK {
		t.Fatalf("runSessionStart expected %d got %d", exitOK, code)
	}
	for i := 0; i < 2; i++ {
		if code := runSessionAppend([]string{
			"--journal", journalPath,
			"--tool", "tool.write",
			"--verdict", "allow",
			"--intent-id", "intent_pre_" + strconv.Itoa(i+1),
			"--json",
		}); code != exitOK {
			t.Fatalf("runSessionAppend pre-checkpoint %d expected %d got %d", i+1, exitOK, code)
		}
	}
	if code := runSessionCheckpoint([]string{
		"--journal", journalPath,
		"--out", checkpointPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runSessionCheckpoint expected %d got %d", exitOK, code)
	}
	if code := runSessionAppend([]string{
		"--journal", journalPath,
		"--tool", "tool.write",
		"--verdict", "allow",
		"--intent-id", "intent_post_1",
		"--json",
	}); code != exitOK {
		t.Fatalf("runSessionAppend post-checkpoint expected %d got %d", exitOK, code)
	}
	if code := runSessionCompact([]string{
		"--journal", journalPath,
		"--dry-run",
		"--json",
	}); code != exitOK {
		t.Fatalf("runSessionCompact dry-run expected %d got %d", exitOK, code)
	}
	if code := runSessionCompact([]string{
		"--journal", journalPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runSessionCompact apply expected %d got %d", exitOK, code)
	}
}

func TestSessionVerdictSupport(t *testing.T) {
	if !isSessionVerdictSupported("allow") {
		t.Fatalf("allow should be supported")
	}
	if !isSessionVerdictSupported(" REQUIRE_APPROVAL ") {
		t.Fatalf("require_approval should be supported case-insensitively")
	}
	if isSessionVerdictSupported("something_else") {
		t.Fatalf("unexpected support for unknown verdict")
	}
}
