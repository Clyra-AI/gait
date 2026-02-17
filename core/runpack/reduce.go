package runpack

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

type ReducePredicate string

const (
	PredicateMissingResult ReducePredicate = "missing_result"
	PredicateNonOKStatus   ReducePredicate = "non_ok_status"
)

type ReduceOptions struct {
	InputPath  string
	OutputPath string
	Predicate  ReducePredicate
}

type ReduceReport struct {
	SchemaID            string          `json:"schema_id"`
	SchemaVersion       string          `json:"schema_version"`
	CreatedAt           time.Time       `json:"created_at"`
	ProducerVersion     string          `json:"producer_version"`
	RunID               string          `json:"run_id"`
	ReducedRunID        string          `json:"reduced_run_id"`
	Predicate           ReducePredicate `json:"predicate"`
	SelectedIntentID    string          `json:"selected_intent_id"`
	OriginalIntentCount int             `json:"original_intent_count"`
	ReducedIntentCount  int             `json:"reduced_intent_count"`
	OriginalResultCount int             `json:"original_result_count"`
	ReducedResultCount  int             `json:"reduced_result_count"`
	OriginalRefCount    int             `json:"original_ref_count"`
	ReducedRefCount     int             `json:"reduced_ref_count"`
	StillFailing        bool            `json:"still_failing"`
}

type ReduceResult struct {
	OutputPath   string
	RunID        string
	ReducedRunID string
	Report       ReduceReport
}

func ParseReducePredicate(value string) (ReducePredicate, error) {
	normalized := ReducePredicate(strings.ToLower(strings.TrimSpace(value)))
	switch normalized {
	case PredicateMissingResult, PredicateNonOKStatus:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported predicate %q", value)
	}
}

func ReduceToMinimal(options ReduceOptions) (ReduceResult, error) {
	if strings.TrimSpace(options.InputPath) == "" {
		return ReduceResult{}, fmt.Errorf("input path is required")
	}
	predicate := options.Predicate
	if predicate == "" {
		predicate = PredicateMissingResult
	}

	data, err := ReadRunpack(options.InputPath)
	if err != nil {
		return ReduceResult{}, err
	}
	selectedIntentID, err := selectIntentIDForPredicate(data, predicate)
	if err != nil {
		return ReduceResult{}, err
	}
	reducedRunID := reducedRunID(data.Run.RunID, predicate)

	reducedIntents := filterIntents(data.Intents, selectedIntentID, reducedRunID)
	reducedResults := filterResultsForPredicate(data.Results, selectedIntentID, reducedRunID, predicate)
	selectedRefIDs := collectRefIDs(reducedIntents, reducedResults)
	reducedRefs := filterRefs(data.Refs, selectedRefIDs, reducedRunID)
	reducedRun := filterRunMetadata(data.Run, selectedIntentID, reducedRunID)

	outputPath := strings.TrimSpace(options.OutputPath)
	if outputPath == "" {
		outputPath = defaultReducedPath(options.InputPath, data.Run.RunID, predicate)
	}

	_, err = WriteRunpack(outputPath, RecordOptions{
		Run:         reducedRun,
		Intents:     reducedIntents,
		Results:     reducedResults,
		Refs:        reducedRefs,
		CaptureMode: data.Manifest.CaptureMode,
	})
	if err != nil {
		return ReduceResult{}, fmt.Errorf("write reduced runpack: %w", err)
	}

	reducedData, err := ReadRunpack(outputPath)
	if err != nil {
		return ReduceResult{}, fmt.Errorf("read reduced runpack: %w", err)
	}
	stillFailing := predicateHolds(reducedData, predicate)
	if !stillFailing {
		return ReduceResult{}, fmt.Errorf("reduced runpack no longer satisfies predicate %s", predicate)
	}

	report := ReduceReport{
		SchemaID:            "gait.runpack.reduce_report",
		SchemaVersion:       "1.0.0",
		CreatedAt:           reducedRun.CreatedAt.UTC(),
		ProducerVersion:     reducedRun.ProducerVersion,
		RunID:               data.Run.RunID,
		ReducedRunID:        reducedRunID,
		Predicate:           predicate,
		SelectedIntentID:    selectedIntentID,
		OriginalIntentCount: len(data.Intents),
		ReducedIntentCount:  len(reducedIntents),
		OriginalResultCount: len(data.Results),
		ReducedResultCount:  len(reducedResults),
		OriginalRefCount:    len(data.Refs.Receipts),
		ReducedRefCount:     len(reducedRefs.Receipts),
		StillFailing:        stillFailing,
	}
	return ReduceResult{
		OutputPath:   outputPath,
		RunID:        data.Run.RunID,
		ReducedRunID: reducedRunID,
		Report:       report,
	}, nil
}

func selectIntentIDForPredicate(data Runpack, predicate ReducePredicate) (string, error) {
	intentIDs := make([]string, 0, len(data.Intents))
	for _, intent := range data.Intents {
		if intent.IntentID == "" {
			continue
		}
		intentIDs = append(intentIDs, intent.IntentID)
	}
	sort.Strings(intentIDs)
	if len(intentIDs) == 0 {
		return "", fmt.Errorf("runpack has no intent IDs")
	}

	resultByIntent := map[string][]schemarunpack.ResultRecord{}
	for _, result := range data.Results {
		resultByIntent[result.IntentID] = append(resultByIntent[result.IntentID], result)
	}

	for _, intentID := range intentIDs {
		results := resultByIntent[intentID]
		switch predicate {
		case PredicateMissingResult:
			if len(results) == 0 {
				return intentID, nil
			}
		case PredicateNonOKStatus:
			for _, result := range results {
				if !isResultOK(result.Status) {
					return intentID, nil
				}
			}
		}
	}
	return "", fmt.Errorf("predicate %s not triggered by runpack", predicate)
}

func predicateHolds(data Runpack, predicate ReducePredicate) bool {
	_, err := selectIntentIDForPredicate(data, predicate)
	return err == nil
}

func filterIntents(records []schemarunpack.IntentRecord, intentID string, runID string) []schemarunpack.IntentRecord {
	out := make([]schemarunpack.IntentRecord, 0, 1)
	for _, record := range records {
		if record.IntentID != intentID {
			continue
		}
		record.RunID = runID
		out = append(out, record)
		break
	}
	return out
}

func filterResultsForPredicate(
	records []schemarunpack.ResultRecord,
	intentID string,
	runID string,
	predicate ReducePredicate,
) []schemarunpack.ResultRecord {
	if predicate == PredicateMissingResult {
		return nil
	}
	out := make([]schemarunpack.ResultRecord, 0, 1)
	for _, record := range records {
		if record.IntentID != intentID {
			continue
		}
		if !isResultOK(record.Status) {
			record.RunID = runID
			out = append(out, record)
			break
		}
	}
	return out
}

func isResultOK(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == "ok" || normalized == "success"
}

func collectRefIDs(intents []schemarunpack.IntentRecord, results []schemarunpack.ResultRecord) map[string]struct{} {
	refs := map[string]struct{}{}
	for _, intent := range intents {
		collectRefIDsFromValue(intent.Args, refs)
	}
	for _, result := range results {
		collectRefIDsFromValue(result.Result, refs)
	}
	return refs
}

func collectRefIDsFromValue(value any, refs map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			lowerKey := strings.ToLower(strings.TrimSpace(key))
			if strings.Contains(lowerKey, "ref") {
				if asString, ok := nested.(string); ok {
					trimmed := strings.TrimSpace(asString)
					if trimmed != "" {
						refs[trimmed] = struct{}{}
					}
				}
			}
			collectRefIDsFromValue(nested, refs)
		}
	case []any:
		for _, nested := range typed {
			collectRefIDsFromValue(nested, refs)
		}
	case string:
		if strings.HasPrefix(typed, "ref_") {
			refs[typed] = struct{}{}
		}
	}
}

func filterRefs(refs schemarunpack.Refs, selected map[string]struct{}, runID string) schemarunpack.Refs {
	reduced := refs
	reduced.RunID = runID
	if len(selected) == 0 {
		reduced.Receipts = nil
		return reduced
	}
	receipts := make([]schemarunpack.RefReceipt, 0, len(refs.Receipts))
	for _, receipt := range refs.Receipts {
		if _, ok := selected[receipt.RefID]; ok {
			receipts = append(receipts, receipt)
		}
	}
	sort.Slice(receipts, func(i, j int) bool {
		return receipts[i].RefID < receipts[j].RefID
	})
	reduced.Receipts = receipts
	return reduced
}

func filterRunMetadata(run schemarunpack.Run, selectedIntentID string, runID string) schemarunpack.Run {
	reduced := run
	reduced.RunID = runID
	filteredTimeline := make([]schemarunpack.TimelineEvt, 0, len(run.Timeline))
	for _, event := range run.Timeline {
		if event.Ref == "" || event.Ref == selectedIntentID {
			filteredTimeline = append(filteredTimeline, event)
		}
	}
	reduced.Timeline = filteredTimeline
	return reduced
}

func reducedRunID(original string, predicate ReducePredicate) string {
	predicatePart := strings.ReplaceAll(string(predicate), "_", "")
	base := strings.TrimSpace(original)
	if !strings.HasPrefix(base, "run_") {
		base = "run_" + base
	}
	return fmt.Sprintf("%s_reduced_%s", base, predicatePart)
}

func defaultReducedPath(inputPath string, runID string, predicate ReducePredicate) string {
	baseDir := filepath.Dir(inputPath)
	fileName := fmt.Sprintf("runpack_%s_reduced_%s.zip", runID, predicate)
	return filepath.Join(baseDir, fileName)
}

func EncodeReduceReport(report ReduceReport) ([]byte, error) {
	return json.Marshal(report)
}
