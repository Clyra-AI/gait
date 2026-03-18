package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	runpackcore "github.com/Clyra-AI/gait/core/runpack"
)

func TestRunRecordNormalizesMissingDigestsFromNormalization(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	inputPath := filepath.Join(workDir, "run_record.json")
	outDir := filepath.Join(workDir, "gait-out")

	payload := map[string]any{
		"run": map[string]any{
			"schema_id":        "gait.runpack.run",
			"schema_version":   "1.0.0",
			"created_at":       time.Date(2026, time.March, 18, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"producer_version": "0.0.0-test",
			"run_id":           "run_record_normalized",
			"env": map[string]any{
				"os":      "darwin",
				"arch":    "arm64",
				"runtime": "python3.11",
			},
			"timeline": []map[string]any{{"event": "run_started", "ts": time.Date(2026, time.March, 18, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)}},
		},
		"intents": []map[string]any{{
			"schema_id":        "gait.runpack.intent",
			"schema_version":   "1.0.0",
			"created_at":       time.Date(2026, time.March, 18, 0, 0, 1, 0, time.UTC).Format(time.RFC3339),
			"producer_version": "0.0.0-test",
			"run_id":           "run_record_normalized",
			"intent_id":        "intent_0001",
			"tool_name":        "tool.allow",
		}},
		"results": []map[string]any{{
			"schema_id":        "gait.runpack.result",
			"schema_version":   "1.0.0",
			"created_at":       time.Date(2026, time.March, 18, 0, 0, 1, 0, time.UTC).Format(time.RFC3339),
			"producer_version": "0.0.0-test",
			"run_id":           "run_record_normalized",
			"intent_id":        "intent_0001",
			"status":           "ok",
		}},
		"refs": map[string]any{
			"schema_id":        "gait.runpack.refs",
			"schema_version":   "1.0.0",
			"created_at":       time.Date(2026, time.March, 18, 0, 0, 2, 0, time.UTC).Format(time.RFC3339),
			"producer_version": "0.0.0-test",
			"run_id":           "run_record_normalized",
			"receipts": []map[string]any{{
				"ref_id":         "trace_intent_0001",
				"source_type":    "gait.trace",
				"source_locator": "trace://trace_intent_0001",
				"retrieved_at":   time.Date(2026, time.March, 18, 0, 0, 2, 0, time.UTC).Format(time.RFC3339),
				"redaction_mode": "reference",
			}},
		},
		"capture_mode": "reference",
		"normalization": map[string]any{
			"intent_args": map[string]any{
				"intent_0001": map[string]any{"path": "/tmp/out.txt"},
			},
			"result_payloads": map[string]any{
				"intent_0001": map[string]any{
					"executed":      true,
					"verdict":       "allow",
					"reason_codes":  []string{"default_allow"},
					"trace_id":      "trace_1",
					"trace_path":    "trace.json",
					"policy_digest": strings.Repeat("3", 64),
					"intent_digest": strings.Repeat("2", 64),
				},
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	mustWriteFile(t, inputPath, string(encoded)+"\n")

	var code int
	raw := captureStdout(t, func() {
		code = runRecord([]string{"--input", inputPath, "--out-dir", outDir, "--json"})
	})
	if code != exitOK {
		t.Fatalf("runRecord expected %d got %d raw=%s", exitOK, code, raw)
	}
	var output runRecordOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode output: %v raw=%s", err, raw)
	}
	if !output.OK || output.Bundle == "" {
		t.Fatalf("unexpected runRecord output: %#v", output)
	}
	if code := runVerify([]string{"--json", output.Bundle}); code != exitOK {
		t.Fatalf("runVerify expected %d got %d", exitOK, code)
	}
	recorded, err := runpackcore.ReadRunpack(output.Bundle)
	if err != nil {
		t.Fatalf("read recorded runpack: %v", err)
	}
	if recorded.Intents[0].ArgsDigest == "" || recorded.Results[0].ResultDigest == "" {
		t.Fatalf("expected normalized digests in recorded runpack: %#v %#v", recorded.Intents[0], recorded.Results[0])
	}
	if recorded.Refs.Receipts[0].QueryDigest == "" || recorded.Refs.Receipts[0].ContentDigest == "" {
		t.Fatalf("expected normalized ref digests in recorded runpack: %#v", recorded.Refs.Receipts[0])
	}
}

func TestRunRecordRejectsDigestMismatchAgainstNormalization(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	inputPath := filepath.Join(workDir, "run_record_bad.json")

	payload := map[string]any{
		"run": map[string]any{
			"schema_id":        "gait.runpack.run",
			"schema_version":   "1.0.0",
			"created_at":       time.Date(2026, time.March, 18, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"producer_version": "0.0.0-test",
			"run_id":           "run_record_bad",
			"env":              map[string]any{"os": "darwin", "arch": "arm64", "runtime": "python3.11"},
			"timeline":         []map[string]any{{"event": "run_started", "ts": time.Date(2026, time.March, 18, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)}},
		},
		"intents": []map[string]any{{
			"schema_id":        "gait.runpack.intent",
			"schema_version":   "1.0.0",
			"created_at":       time.Date(2026, time.March, 18, 0, 0, 1, 0, time.UTC).Format(time.RFC3339),
			"producer_version": "0.0.0-test",
			"run_id":           "run_record_bad",
			"intent_id":        "intent_0001",
			"tool_name":        "tool.allow",
			"args_digest":      strings.Repeat("a", 64),
		}},
		"results": []map[string]any{},
		"refs": map[string]any{
			"schema_id":        "gait.runpack.refs",
			"schema_version":   "1.0.0",
			"created_at":       time.Date(2026, time.March, 18, 0, 0, 2, 0, time.UTC).Format(time.RFC3339),
			"producer_version": "0.0.0-test",
			"run_id":           "run_record_bad",
			"receipts":         []map[string]any{},
		},
		"capture_mode": "reference",
		"normalization": map[string]any{
			"intent_args": map[string]any{
				"intent_0001": map[string]any{"path": "/tmp/out.txt"},
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	mustWriteFile(t, inputPath, string(encoded)+"\n")

	var code int
	raw := captureStdout(t, func() {
		code = runRecord([]string{"--input", inputPath, "--out-dir", filepath.Join(workDir, "gait-out"), "--json"})
	})
	if code != exitInvalidInput {
		t.Fatalf("runRecord mismatch expected %d got %d raw=%s", exitInvalidInput, code, raw)
	}
	var output runRecordOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode output: %v raw=%s", err, raw)
	}
	if !strings.Contains(output.Error, "intent intent_0001 args digest mismatch") {
		t.Fatalf("unexpected mismatch error: %#v", output)
	}
}
