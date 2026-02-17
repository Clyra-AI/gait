package runpack

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
	"github.com/Clyra-AI/gait/core/sign"
)

func BenchmarkVerifyZipTypical(b *testing.B) {
	workDir := b.TempDir()
	zipPath, publicKey := mustWriteBenchmarkRunpack(b, workDir, "run_verify", false)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := VerifyZip(zipPath, VerifyOptions{
			PublicKey:        publicKey,
			RequireSignature: true,
		})
		if err != nil {
			b.Fatalf("verify zip: %v", err)
		}
		if result.SignatureStatus != "verified" {
			b.Fatalf("unexpected signature status: %s", result.SignatureStatus)
		}
	}
}

func BenchmarkDiffRunpacksTypical(b *testing.B) {
	workDir := b.TempDir()
	leftPath, _ := mustWriteBenchmarkRunpack(b, workDir, "run_diff_left", false)
	rightPath, _ := mustWriteBenchmarkRunpack(b, workDir, "run_diff_right", true)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := DiffRunpacks(leftPath, rightPath, DiffPrivacyFull)
		if err != nil {
			b.Fatalf("diff runpacks: %v", err)
		}
		if result.Privacy != DiffPrivacyFull {
			b.Fatalf("unexpected diff privacy: %s", result.Privacy)
		}
	}
}

func mustWriteBenchmarkRunpack(b *testing.B, workDir, runID string, variant bool) (string, ed25519.PublicKey) {
	b.Helper()

	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		b.Fatalf("generate keypair: %v", err)
	}

	createdAt := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	run := schemarunpack.Run{
		SchemaID:        "gait.runpack.run",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: "0.0.0-bench",
		RunID:           runID,
		Env: schemarunpack.RunEnv{
			OS:      "bench",
			Arch:    "bench",
			Runtime: "go",
		},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: createdAt},
			{Event: "finish", TS: createdAt.Add(time.Second)},
		},
	}

	intents := []schemarunpack.IntentRecord{
		{
			SchemaID:        "gait.runpack.intent",
			SchemaVersion:   "1.0.0",
			CreatedAt:       createdAt,
			ProducerVersion: run.ProducerVersion,
			RunID:           run.RunID,
			IntentID:        "intent_1",
			ToolName:        "tool.search",
			ArgsDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
			Args:            map[string]any{"query": "benchmark"},
		},
		{
			SchemaID:        "gait.runpack.intent",
			SchemaVersion:   "1.0.0",
			CreatedAt:       createdAt,
			ProducerVersion: run.ProducerVersion,
			RunID:           run.RunID,
			IntentID:        "intent_2",
			ToolName:        "tool.fetch",
			ArgsDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
			Args:            map[string]any{"url": "https://example.local"},
		},
	}

	resultMessage := "ok"
	if variant {
		resultMessage = "changed"
	}
	results := []schemarunpack.ResultRecord{
		{
			SchemaID:        "gait.runpack.result",
			SchemaVersion:   "1.0.0",
			CreatedAt:       createdAt,
			ProducerVersion: run.ProducerVersion,
			RunID:           run.RunID,
			IntentID:        "intent_1",
			Status:          "ok",
			ResultDigest:    "3333333333333333333333333333333333333333333333333333333333333333",
			Result:          map[string]any{"ok": true, "message": resultMessage},
		},
		{
			SchemaID:        "gait.runpack.result",
			SchemaVersion:   "1.0.0",
			CreatedAt:       createdAt,
			ProducerVersion: run.ProducerVersion,
			RunID:           run.RunID,
			IntentID:        "intent_2",
			Status:          "ok",
			ResultDigest:    "4444444444444444444444444444444444444444444444444444444444444444",
			Result:          map[string]any{"ok": true},
		},
	}

	refs := schemarunpack.Refs{
		SchemaID:        "gait.runpack.refs",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: run.ProducerVersion,
		RunID:           run.RunID,
		Receipts: []schemarunpack.RefReceipt{
			{
				RefID:         "ref_1",
				SourceType:    "bench",
				SourceLocator: "tool.search",
				QueryDigest:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ContentDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				RetrievedAt:   createdAt,
				RedactionMode: "reference",
			},
		},
	}

	recordResult, err := RecordRun(RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
		SignKey:     keyPair.Private,
	})
	if err != nil {
		b.Fatalf("record run: %v", err)
	}

	zipPath := filepath.Join(workDir, runID+".zip")
	if err := os.WriteFile(zipPath, recordResult.ZipBytes, 0o600); err != nil {
		b.Fatalf("write zip: %v", err)
	}

	return zipPath, keyPair.Public
}
