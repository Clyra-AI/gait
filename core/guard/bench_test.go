package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemaregress "github.com/Clyra-AI/gait/core/schema/v1/regress"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func BenchmarkVerifyPackTypical(b *testing.B) {
	workDir := b.TempDir()
	now := time.Date(2026, time.February, 6, 12, 0, 0, 0, time.UTC)
	runpackPath := mustWriteGuardBenchmarkRunpack(b, workDir, now)
	tracePath := filepath.Join(workDir, "trace_bench.json")
	regressPath := filepath.Join(workDir, "regress_bench.json")
	mustWriteGuardBenchJSON(b, tracePath, schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-bench",
		TraceID:         "trace_bench",
		ToolName:        "tool.write",
		ArgsDigest:      strings.Repeat("a", 64),
		IntentDigest:    strings.Repeat("b", 64),
		PolicyDigest:    strings.Repeat("c", 64),
		Verdict:         "allow",
	})
	mustWriteGuardBenchJSON(b, regressPath, schemaregress.RegressResult{
		SchemaID:        "gait.regress.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: "0.0.0-bench",
		FixtureSet:      "run_guard_bench",
		Status:          "pass",
	})
	packPath := filepath.Join(workDir, "evidence_pack_bench.zip")
	if _, err := BuildPack(BuildOptions{
		RunpackPath:     runpackPath,
		OutputPath:      packPath,
		TracePaths:      []string{tracePath},
		RegressPaths:    []string{regressPath},
		TemplateID:      "soc2",
		ProducerVersion: "0.0.0-bench",
	}); err != nil {
		b.Fatalf("build benchmark pack: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := VerifyPack(packPath)
		if err != nil {
			b.Fatalf("verify pack: %v", err)
		}
		if len(result.MissingFiles) > 0 || len(result.HashMismatches) > 0 {
			b.Fatalf("unexpected verify issues: missing=%d mismatch=%d", len(result.MissingFiles), len(result.HashMismatches))
		}
	}
}

func BenchmarkBuildIncidentPackTypical(b *testing.B) {
	workDir := b.TempDir()
	now := time.Date(2026, time.February, 6, 12, 0, 0, 0, time.UTC)
	runpackPath := mustWriteGuardBenchmarkRunpack(b, workDir, now)
	mustWriteGuardBenchJSON(b, filepath.Join(workDir, "trace_bench.json"), schemagate.TraceRecord{
		SchemaID:        "gait.gate.trace",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now.Add(15 * time.Minute),
		ProducerVersion: "0.0.0-bench",
		TraceID:         "trace_bench",
		ToolName:        "tool.write",
		ArgsDigest:      strings.Repeat("a", 64),
		IntentDigest:    strings.Repeat("b", 64),
		PolicyDigest:    strings.Repeat("c", 64),
		Verdict:         "allow",
	})
	mustWriteGuardBenchJSON(b, filepath.Join(workDir, "regress_result.json"), schemaregress.RegressResult{
		SchemaID:        "gait.regress.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now.Add(10 * time.Minute),
		ProducerVersion: "0.0.0-bench",
		FixtureSet:      "run_guard_bench",
		Status:          "pass",
	})

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		outputPath := filepath.Join(workDir, fmt.Sprintf("incident_pack_%d.zip", index))
		result, err := BuildIncidentPack(IncidentPackOptions{
			RunpackPath:     runpackPath,
			OutputPath:      outputPath,
			Window:          2 * time.Hour,
			TemplateID:      "incident_response",
			ProducerVersion: "0.0.0-bench",
		})
		if err != nil {
			b.Fatalf("build incident pack: %v", err)
		}
		if result.BuildResult.Manifest.PackID == "" {
			b.Fatalf("expected pack id")
		}
		if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
			b.Fatalf("cleanup incident pack: %v", err)
		}
	}
}

func mustWriteGuardBenchmarkRunpack(b *testing.B, workDir string, now time.Time) string {
	b.Helper()
	runpackPath := filepath.Join(workDir, "runpack_guard_bench.zip")
	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_guard_bench",
			CreatedAt:       now,
			ProducerVersion: "0.0.0-bench",
		},
		Refs: schemarunpack.Refs{
			RunID:    "run_guard_bench",
			Receipts: []schemarunpack.RefReceipt{},
		},
		CaptureMode: "reference",
	})
	if err != nil {
		b.Fatalf("write benchmark runpack: %v", err)
	}
	return runpackPath
}

func mustWriteGuardBenchJSON(b *testing.B, path string, value any) {
	b.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		b.Fatalf("marshal benchmark json: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		b.Fatalf("write benchmark json %s: %v", path, err)
	}
}
