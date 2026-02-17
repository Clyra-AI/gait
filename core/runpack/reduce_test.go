package runpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestReduceToMinimalMissingResult(t *testing.T) {
	workDir := t.TempDir()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	runpackPath := filepath.Join(workDir, "runpack_run_reduce.zip")
	_, err := WriteRunpack(runpackPath, RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_reduce",
			CreatedAt:       now,
			ProducerVersion: "0.0.0-dev",
		},
		Intents: []schemarunpack.IntentRecord{
			{
				IntentID:   "intent_a",
				RunID:      "run_reduce",
				ToolName:   "tool.read",
				ArgsDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			{
				IntentID:   "intent_b",
				RunID:      "run_reduce",
				ToolName:   "tool.write",
				ArgsDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
		Results: []schemarunpack.ResultRecord{
			{
				IntentID:     "intent_a",
				RunID:        "run_reduce",
				Status:       "ok",
				ResultDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			},
		},
		Refs: schemarunpack.Refs{RunID: "run_reduce"},
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	outputPath := filepath.Join(workDir, "reduced.zip")
	result, err := ReduceToMinimal(ReduceOptions{
		InputPath:  runpackPath,
		OutputPath: outputPath,
		Predicate:  PredicateMissingResult,
	})
	if err != nil {
		t.Fatalf("reduce: %v", err)
	}
	if result.Report.SelectedIntentID != "intent_b" {
		t.Fatalf("expected selected intent intent_b got %s", result.Report.SelectedIntentID)
	}
	if result.Report.ReducedIntentCount != 1 {
		t.Fatalf("expected 1 reduced intent got %d", result.Report.ReducedIntentCount)
	}
	if result.Report.ReducedResultCount != 0 {
		t.Fatalf("expected 0 reduced results got %d", result.Report.ReducedResultCount)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected reduced runpack at %s: %v", outputPath, err)
	}
}

func TestReduceToMinimalNonOKStatus(t *testing.T) {
	workDir := t.TempDir()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	runpackPath := filepath.Join(workDir, "runpack_run_reduce_status.zip")
	_, err := WriteRunpack(runpackPath, RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_reduce_status",
			CreatedAt:       now,
			ProducerVersion: "0.0.0-dev",
		},
		Intents: []schemarunpack.IntentRecord{
			{
				IntentID:   "intent_1",
				RunID:      "run_reduce_status",
				ToolName:   "tool.read",
				ArgsDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
		Results: []schemarunpack.ResultRecord{
			{
				IntentID:     "intent_1",
				RunID:        "run_reduce_status",
				Status:       "error",
				ResultDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
		Refs: schemarunpack.Refs{RunID: "run_reduce_status"},
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	_, err = ReduceToMinimal(ReduceOptions{
		InputPath: runpackPath,
		Predicate: PredicateNonOKStatus,
	})
	if err != nil {
		t.Fatalf("reduce non-ok status: %v", err)
	}
}

func TestReduceHelpersAndPredicates(t *testing.T) {
	if predicate, err := ParseReducePredicate("missing_result"); err != nil || predicate != PredicateMissingResult {
		t.Fatalf("ParseReducePredicate missing_result mismatch: %s err=%v", predicate, err)
	}
	if predicate, err := ParseReducePredicate("NON_OK_STATUS"); err != nil || predicate != PredicateNonOKStatus {
		t.Fatalf("ParseReducePredicate non_ok_status mismatch: %s err=%v", predicate, err)
	}
	if _, err := ParseReducePredicate("unknown"); err == nil {
		t.Fatalf("expected ParseReducePredicate unknown error")
	}

	report := ReduceReport{
		SchemaID:      "gait.runpack.reduce_report",
		SchemaVersion: "1.0.0",
		RunID:         "run_x",
		ReducedRunID:  "run_x_reduced",
		Predicate:     PredicateMissingResult,
		StillFailing:  true,
	}
	encoded, err := EncodeReduceReport(report)
	if err != nil {
		t.Fatalf("EncodeReduceReport: %v", err)
	}
	var decoded ReduceReport
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal encoded report: %v", err)
	}
	if decoded.RunID != report.RunID {
		t.Fatalf("encoded report mismatch: %#v", decoded)
	}

	refIDs := collectRefIDs(
		[]schemarunpack.IntentRecord{{
			IntentID: "intent_1",
			Args: map[string]any{
				"ref_id": "ref_1",
				"nested": []any{
					map[string]any{"source_ref": "ref_2"},
				},
			},
		}},
		[]schemarunpack.ResultRecord{{
			IntentID: "intent_1",
			Result: map[string]any{
				"refs": []any{"ref_3"},
			},
		}},
	)
	if len(refIDs) < 3 {
		t.Fatalf("collectRefIDs expected >=3 refs got %d", len(refIDs))
	}

	refs := filterRefs(schemarunpack.Refs{
		RunID: "run_in",
		Receipts: []schemarunpack.RefReceipt{
			{RefID: "ref_2"},
			{RefID: "ref_1"},
		},
	}, map[string]struct{}{"ref_1": {}}, "run_out")
	if refs.RunID != "run_out" || len(refs.Receipts) != 1 || refs.Receipts[0].RefID != "ref_1" {
		t.Fatalf("filterRefs mismatch: %#v", refs)
	}

	run := filterRunMetadata(schemarunpack.Run{
		RunID: "run_in",
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "intent", Ref: "intent_1"},
			{Event: "intent", Ref: "intent_2"},
			{Event: "status"},
		},
	}, "intent_1", "run_out")
	if run.RunID != "run_out" || len(run.Timeline) != 2 {
		t.Fatalf("filterRunMetadata mismatch: %#v", run)
	}

	if got := reducedRunID("run_demo", PredicateMissingResult); !strings.Contains(got, "reduced") {
		t.Fatalf("reducedRunID missing suffix: %s", got)
	}
	if got := reducedRunID("demo", PredicateNonOKStatus); !strings.HasPrefix(got, "run_") {
		t.Fatalf("reducedRunID missing run_ prefix: %s", got)
	}
	if got := defaultReducedPath("/tmp/runpack_run_a.zip", "run_a", PredicateMissingResult); !strings.Contains(got, "runpack_run_a_reduced_missing_result.zip") {
		t.Fatalf("defaultReducedPath mismatch: %s", got)
	}
}
