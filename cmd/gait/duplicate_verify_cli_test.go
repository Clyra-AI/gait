package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/jobruntime"
	packcore "github.com/Clyra-AI/gait/core/pack"
	runpackcore "github.com/Clyra-AI/gait/core/runpack"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestRunPackVerifyRejectsDuplicateEntries(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	jobsRoot := filepath.Join(workDir, "jobs")
	jobID := "job_pack_duplicate_cli"

	if _, err := jobruntime.Submit(jobsRoot, jobruntime.SubmitOptions{JobID: jobID}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	packPath := filepath.Join(workDir, "job_pack.zip")
	if _, err := packcore.BuildJobPackFromPath(jobsRoot, jobID, packPath, "test-v24", nil); err != nil {
		t.Fatalf("build job pack: %v", err)
	}

	duplicatePath := filepath.Join(workDir, "job_pack_duplicate.zip")
	if err := writeDuplicateEntryZip(packPath, duplicatePath, "job_state.json", []byte(`{"job_id":"evil"}`), false); err != nil {
		t.Fatalf("write duplicate pack zip: %v", err)
	}

	code, output := runPackJSON(t, []string{"verify", duplicatePath, "--json"})
	if code != exitVerifyFailed {
		t.Fatalf("pack verify duplicate expected %d got %d output=%#v", exitVerifyFailed, code, output)
	}
	if output.OK {
		t.Fatalf("expected pack verify output ok=false: %#v", output)
	}
	if !strings.Contains(output.Error, "zip contains duplicate entries: job_state.json") {
		t.Fatalf("unexpected pack verify error: %#v", output)
	}
}

func TestRunVerifyRejectsDuplicateEntries(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	now := time.Date(2026, time.March, 18, 0, 0, 0, 0, time.UTC)
	runpackPath := filepath.Join(workDir, "runpack_valid.zip")

	if _, err := runpackcore.WriteRunpack(runpackPath, runpackcore.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_duplicate_cli",
			CreatedAt:       now,
			ProducerVersion: "0.0.0-test",
		},
		Intents: []schemarunpack.IntentRecord{{
			IntentID:   "intent_1",
			RunID:      "run_duplicate_cli",
			ToolName:   "tool.write",
			ArgsDigest: strings.Repeat("a", 64),
		}},
		Results: []schemarunpack.ResultRecord{{
			IntentID:     "intent_1",
			RunID:        "run_duplicate_cli",
			Status:       "ok",
			ResultDigest: strings.Repeat("b", 64),
		}},
		Refs: schemarunpack.Refs{
			RunID:    "run_duplicate_cli",
			Receipts: []schemarunpack.RefReceipt{},
		},
		CaptureMode: "reference",
	}); err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	duplicatePath := filepath.Join(workDir, "runpack_duplicate.zip")
	if err := writeDuplicateEntryZip(runpackPath, duplicatePath, "run.json", []byte(`{"run":"evil"}`), true); err != nil {
		t.Fatalf("write duplicate runpack zip: %v", err)
	}

	var code int
	raw := captureStdout(t, func() {
		code = runVerify([]string{"--json", duplicatePath})
	})
	if code != exitVerifyFailed {
		t.Fatalf("verify duplicate expected %d got %d raw=%s", exitVerifyFailed, code, raw)
	}
	var output verifyOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode verify output: %v raw=%s", err, raw)
	}
	if output.OK {
		t.Fatalf("expected verify output ok=false: %#v", output)
	}
	if !strings.Contains(output.Error, "zip contains duplicate entries: run.json") {
		t.Fatalf("unexpected verify error output: %#v", output)
	}
}

func writeDuplicateEntryZip(srcPath string, dstPath string, entryName string, duplicatePayload []byte, duplicateFirst bool) error {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	inserted := false
	for _, file := range reader.File {
		if file.Name == entryName && duplicateFirst && !inserted {
			target, err := writer.Create(entryName)
			if err != nil {
				_ = writer.Close()
				return err
			}
			if _, err := target.Write(duplicatePayload); err != nil {
				_ = writer.Close()
				return err
			}
			inserted = true
		}

		fileReader, err := file.Open()
		if err != nil {
			_ = writer.Close()
			return err
		}
		payload, err := io.ReadAll(fileReader)
		_ = fileReader.Close()
		if err != nil {
			_ = writer.Close()
			return err
		}

		target, err := writer.Create(file.Name)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if _, err := target.Write(payload); err != nil {
			_ = writer.Close()
			return err
		}

		if file.Name == entryName && !duplicateFirst && !inserted {
			target, err := writer.Create(entryName)
			if err != nil {
				_ = writer.Close()
				return err
			}
			if _, err := target.Write(duplicatePayload); err != nil {
				_ = writer.Close()
				return err
			}
			inserted = true
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(dstPath, buffer.Bytes(), 0o600)
}
