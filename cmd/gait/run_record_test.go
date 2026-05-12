package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	runpackcore "github.com/Clyra-AI/gait/core/runpack"
	schemacontext "github.com/Clyra-AI/gait/core/schema/v1/context"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestHasRawContextHelpers(t *testing.T) {
	if hasRawContextRecord(nil) {
		t.Fatalf("expected empty context records to be non-raw")
	}
	if !hasRawContextRecord([]schemacontext.ReferenceRecord{{RefID: "ctx_1", RedactionMode: " RAW "}}) {
		t.Fatalf("expected raw context record detection")
	}
	if hasRawContextRecord([]schemacontext.ReferenceRecord{{RefID: "ctx_1", RedactionMode: "reference"}}) {
		t.Fatalf("did not expect non-raw context record detection")
	}

	if hasRawContextReceipts(nil) {
		t.Fatalf("expected empty receipts to be non-raw")
	}
	if !hasRawContextReceipts([]schemarunpack.RefReceipt{{RefID: "ctx_1", RedactionMode: "raw"}}) {
		t.Fatalf("expected raw context receipt detection")
	}
	if hasRawContextReceipts([]schemarunpack.RefReceipt{{RefID: "ctx_1", RedactionMode: "reference"}}) {
		t.Fatalf("did not expect non-raw context receipt detection")
	}
}

func TestRunRecordUnsafeContextRawGate(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	input := runRecordInput{
		Run: schemarunpack.Run{
			RunID: "run_record_raw_gate",
		},
		Intents: []schemarunpack.IntentRecord{{
			IntentID:   "intent_1",
			ToolName:   "tool.demo",
			ArgsDigest: strings.Repeat("a", 64),
			Args:       map[string]any{"x": "y"},
		}},
		Refs: schemarunpack.Refs{
			RunID: "run_record_raw_gate",
			Receipts: []schemarunpack.RefReceipt{{
				RefID:         "ctx_1",
				SourceType:    "web",
				SourceLocator: "https://example.test/context",
				QueryDigest:   strings.Repeat("b", 64),
				ContentDigest: strings.Repeat("c", 64),
				RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
				RedactionMode: "raw",
			}},
		},
	}
	inputPath := filepath.Join(workDir, "run_record.json")
	rawInput, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal run record input: %v", err)
	}
	if err := os.WriteFile(inputPath, rawInput, 0o600); err != nil {
		t.Fatalf("write run record input: %v", err)
	}

	withoutUnsafeCode, withoutUnsafeOut := runRecordJSON(t, []string{
		"--input", inputPath,
		"--out-dir", filepath.Join(workDir, "gait-out"),
		"--json",
	})
	if withoutUnsafeCode != exitInvalidInput {
		t.Fatalf("expected invalid-input exit code without --unsafe-context-raw, got %d output=%#v", withoutUnsafeCode, withoutUnsafeOut)
	}
	if withoutUnsafeOut.OK || !strings.Contains(withoutUnsafeOut.Error, "redaction_mode=raw") {
		t.Fatalf("expected raw-context rejection output, got %#v", withoutUnsafeOut)
	}

	withUnsafeCode, withUnsafeOut := runRecordJSON(t, []string{
		"--input", inputPath,
		"--out-dir", filepath.Join(workDir, "gait-out"),
		"--unsafe-context-raw",
		"--json",
	})
	if withUnsafeCode != exitOK {
		t.Fatalf("expected success with --unsafe-context-raw, got %d output=%#v", withUnsafeCode, withUnsafeOut)
	}
	if !withUnsafeOut.OK || withUnsafeOut.Bundle == "" {
		t.Fatalf("expected successful run record output with bundle path, got %#v", withUnsafeOut)
	}
}

func TestRunRecordCaptureModeWarningsAndArtifactPrivacy(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	input := runRecordInput{
		Run: schemarunpack.Run{
			RunID: "run_record_capture_privacy",
		},
		Intents: []schemarunpack.IntentRecord{{
			IntentID: "intent_1",
			ToolName: "tool.demo",
			Args: map[string]any{
				"path":   "/tmp/out.txt",
				"secret": "reference-secret-arg",
			},
		}},
		Results: []schemarunpack.ResultRecord{{
			IntentID: "intent_1",
			Status:   "ok",
			Result: map[string]any{
				"ok":     true,
				"secret": "reference-secret-result",
			},
		}},
		Refs: schemarunpack.Refs{
			RunID: "run_record_capture_privacy",
		},
	}
	inputPath := filepath.Join(workDir, "run_record_capture_privacy.json")
	rawInput, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal run record input: %v", err)
	}
	if err := os.WriteFile(inputPath, rawInput, 0o600); err != nil {
		t.Fatalf("write run record input: %v", err)
	}

	referenceCode, referenceOut := runRecordJSON(t, []string{
		"--input", inputPath,
		"--out-dir", filepath.Join(workDir, "reference-out"),
		"--capture-mode", "reference",
		"--json",
	})
	if referenceCode != exitOK {
		t.Fatalf("expected reference capture success, got %d output=%#v", referenceCode, referenceOut)
	}
	if !containsString(referenceOut.Warnings, runRecordReferenceStripWarning) {
		t.Fatalf("expected reference strip warning, got %#v", referenceOut.Warnings)
	}
	referencePack, err := runpackcore.ReadRunpack(referenceOut.Bundle)
	if err != nil {
		t.Fatalf("read reference runpack: %v", err)
	}
	if referencePack.Intents[0].Args != nil || referencePack.Results[0].Result != nil {
		t.Fatalf("expected stripped reference-mode payloads, got %#v %#v", referencePack.Intents[0].Args, referencePack.Results[0].Result)
	}

	rawCode, rawOut := runRecordJSON(t, []string{
		"--input", inputPath,
		"--out-dir", filepath.Join(workDir, "raw-out"),
		"--capture-mode", "raw",
		"--json",
	})
	if rawCode != exitOK {
		t.Fatalf("expected raw capture success, got %d output=%#v", rawCode, rawOut)
	}
	if !containsString(rawOut.Warnings, runRecordRawCaptureWarning) {
		t.Fatalf("expected raw capture warning, got %#v", rawOut.Warnings)
	}
	rawPack, err := runpackcore.ReadRunpack(rawOut.Bundle)
	if err != nil {
		t.Fatalf("read raw runpack: %v", err)
	}
	if rawPack.Intents[0].Args == nil || rawPack.Results[0].Result == nil {
		t.Fatalf("expected retained raw-mode payloads, got %#v %#v", rawPack.Intents[0].Args, rawPack.Results[0].Result)
	}
}

func runRecordJSON(t *testing.T, args []string) (int, runRecordOutput) {
	t.Helper()
	var code int
	raw := captureStdout(t, func() {
		code = runRecord(args)
	})
	var output runRecordOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode run record output: %v raw=%q", err, raw)
	}
	return code, output
}
