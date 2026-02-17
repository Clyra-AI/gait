package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

const (
	topActionsReportSchemaID      = "gait.report.top_actions"
	topActionsReportSchemaVersion = "1.0.0"
	defaultTopActionsLimit        = 5
	maxTopActionsLimit            = 20
)

type TopActionsInput struct {
	RunpackPaths []string
	TracePaths   []string
	Limit        int
}

type TopActionsOptions struct {
	ProducerVersion string
	Now             time.Time
}

type TopAction struct {
	Rank           int      `json:"rank"`
	Score          int      `json:"score"`
	ToolClass      string   `json:"tool_class"`
	BlastRadius    int      `json:"blast_radius"`
	RunID          string   `json:"run_id"`
	IntentID       string   `json:"intent_id,omitempty"`
	IntentDigest   string   `json:"intent_digest,omitempty"`
	TraceID        string   `json:"trace_id,omitempty"`
	ToolName       string   `json:"tool_name"`
	Verdict        string   `json:"verdict,omitempty"`
	ReasonCodes    []string `json:"reason_codes,omitempty"`
	SourceType     string   `json:"source_type"`
	SourceArtifact string   `json:"source_artifact"`
}

type TopActionsReport struct {
	SchemaID        string      `json:"schema_id"`
	SchemaVersion   string      `json:"schema_version"`
	CreatedAt       time.Time   `json:"created_at"`
	ProducerVersion string      `json:"producer_version"`
	RunCount        int         `json:"run_count"`
	TraceCount      int         `json:"trace_count"`
	ActionCount     int         `json:"action_count"`
	TopActions      []TopAction `json:"top_actions"`
}

type topActionCandidate struct {
	Action         TopAction
	toolClassScore int
}

func BuildTopActionsReport(input TopActionsInput, opts TopActionsOptions) (TopActionsReport, error) {
	runpackPaths := uniqueSorted(input.RunpackPaths)
	tracePaths := uniqueSorted(input.TracePaths)
	if len(runpackPaths) == 0 && len(tracePaths) == 0 {
		return TopActionsReport{}, fmt.Errorf("at least one runpack or trace path is required")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultTopActionsLimit
	}
	if limit > maxTopActionsLimit {
		limit = maxTopActionsLimit
	}

	candidates := make([]topActionCandidate, 0, len(runpackPaths)+len(tracePaths))
	runIDs := map[string]struct{}{}

	for _, runpackPath := range runpackPaths {
		loadedRunpack, err := runpack.ReadRunpack(runpackPath)
		if err != nil {
			return TopActionsReport{}, fmt.Errorf("read runpack %s: %w", runpackPath, err)
		}
		runID := strings.TrimSpace(loadedRunpack.Run.RunID)
		if runID == "" {
			return TopActionsReport{}, fmt.Errorf("runpack %s missing run_id", runpackPath)
		}
		runIDs[runID] = struct{}{}

		blastRadius := topActionBlastRadiusFromReceipts(loadedRunpack.Refs.Receipts)
		resultsByIntent := map[string]string{}
		for _, result := range loadedRunpack.Results {
			intentID := strings.TrimSpace(result.IntentID)
			if intentID == "" {
				continue
			}
			resultsByIntent[intentID] = normalizeIdentifier(strings.ToLower(strings.TrimSpace(result.Status)))
		}

		for _, intent := range loadedRunpack.Intents {
			toolName := strings.TrimSpace(intent.ToolName)
			if toolName == "" {
				continue
			}
			intentID := strings.TrimSpace(intent.IntentID)
			resultStatus := resultsByIntent[intentID]
			reasonCodes := topActionReasonCodesFromResultStatus(resultStatus)
			toolClass := classifyToolClass(toolName)
			toolClassScore := toolClassScore(toolClass)
			score := topActionScore(toolClassScore, blastRadius, "", reasonCodes)
			candidates = append(candidates, topActionCandidate{
				Action: TopAction{
					Score:          score,
					ToolClass:      toolClass,
					BlastRadius:    blastRadius,
					RunID:          runID,
					IntentID:       intentID,
					ToolName:       toolName,
					ReasonCodes:    reasonCodes,
					SourceType:     "runpack",
					SourceArtifact: runpackPath,
				},
				toolClassScore: toolClassScore,
			})
		}
	}

	for _, tracePath := range tracePaths {
		// #nosec G304 -- trace path list is explicit local user input validated by caller.
		raw, err := os.ReadFile(tracePath)
		if err != nil {
			return TopActionsReport{}, fmt.Errorf("read trace %s: %w", tracePath, err)
		}
		var trace schemagate.TraceRecord
		if err := json.Unmarshal(raw, &trace); err != nil {
			return TopActionsReport{}, fmt.Errorf("parse trace %s: %w", tracePath, err)
		}

		runID := runIDFromTrace(trace)
		if runID == "" {
			runID = "unknown"
		}
		runIDs[runID] = struct{}{}

		toolName := strings.TrimSpace(trace.ToolName)
		if toolName == "" {
			continue
		}
		toolClass := classifyToolClass(toolName)
		toolClassScore := toolClassScore(toolClass)
		verdict := normalizeIdentifier(strings.ToLower(strings.TrimSpace(trace.Verdict)))
		reasonCodes := topActionReasonCodesFromTrace(trace, verdict)
		blastRadius := topActionBlastRadiusFromTrace(trace)
		score := topActionScore(toolClassScore, blastRadius, verdict, reasonCodes)
		candidates = append(candidates, topActionCandidate{
			Action: TopAction{
				Score:          score,
				ToolClass:      toolClass,
				BlastRadius:    blastRadius,
				RunID:          runID,
				IntentDigest:   strings.ToLower(strings.TrimSpace(trace.IntentDigest)),
				TraceID:        strings.TrimSpace(trace.TraceID),
				ToolName:       toolName,
				Verdict:        verdict,
				ReasonCodes:    reasonCodes,
				SourceType:     "trace",
				SourceArtifact: tracePath,
			},
			toolClassScore: toolClassScore,
		})
	}

	if len(candidates) == 0 {
		return TopActionsReport{}, fmt.Errorf("no actions found in provided runpack/trace sources")
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Action.Score != candidates[j].Action.Score {
			return candidates[i].Action.Score > candidates[j].Action.Score
		}
		if candidates[i].toolClassScore != candidates[j].toolClassScore {
			return candidates[i].toolClassScore > candidates[j].toolClassScore
		}
		if candidates[i].Action.BlastRadius != candidates[j].Action.BlastRadius {
			return candidates[i].Action.BlastRadius > candidates[j].Action.BlastRadius
		}
		if candidates[i].Action.RunID != candidates[j].Action.RunID {
			return candidates[i].Action.RunID < candidates[j].Action.RunID
		}
		if candidates[i].Action.IntentID != candidates[j].Action.IntentID {
			return candidates[i].Action.IntentID < candidates[j].Action.IntentID
		}
		if candidates[i].Action.IntentDigest != candidates[j].Action.IntentDigest {
			return candidates[i].Action.IntentDigest < candidates[j].Action.IntentDigest
		}
		if candidates[i].Action.TraceID != candidates[j].Action.TraceID {
			return candidates[i].Action.TraceID < candidates[j].Action.TraceID
		}
		if candidates[i].Action.ToolName != candidates[j].Action.ToolName {
			return candidates[i].Action.ToolName < candidates[j].Action.ToolName
		}
		if candidates[i].Action.SourceType != candidates[j].Action.SourceType {
			return candidates[i].Action.SourceType < candidates[j].Action.SourceType
		}
		return candidates[i].Action.SourceArtifact < candidates[j].Action.SourceArtifact
	})

	if limit > len(candidates) {
		limit = len(candidates)
	}
	topActions := make([]TopAction, 0, limit)
	for index := 0; index < limit; index++ {
		item := candidates[index].Action
		item.Rank = index + 1
		topActions = append(topActions, item)
	}

	return TopActionsReport{
		SchemaID:        topActionsReportSchemaID,
		SchemaVersion:   topActionsReportSchemaVersion,
		CreatedAt:       normalizeSignalNow(opts.Now),
		ProducerVersion: normalizeProducerVersion(opts.ProducerVersion),
		RunCount:        len(runIDs),
		TraceCount:      len(tracePaths),
		ActionCount:     len(candidates),
		TopActions:      topActions,
	}, nil
}

func topActionBlastRadiusFromReceipts(receipts []schemarunpack.RefReceipt) int {
	if len(receipts) == 0 {
		return 1
	}
	targetSystems := make([]string, 0, len(receipts))
	for _, receipt := range receipts {
		targetSystems = append(targetSystems, normalizeTargetSystem(receipt.SourceType, receipt.SourceLocator))
	}
	return targetSensitivityScore(uniqueSorted(targetSystems))
}

func topActionBlastRadiusFromTrace(trace schemagate.TraceRecord) int {
	payload := strings.ToLower(strings.Join(append([]string{trace.ToolName}, trace.Violations...), " "))
	if strings.Contains(payload, "prod") ||
		strings.Contains(payload, "payment") ||
		strings.Contains(payload, "finance") ||
		strings.Contains(payload, "customer") ||
		strings.Contains(payload, "pii") ||
		strings.Contains(payload, "ssn") ||
		strings.Contains(payload, "delete") ||
		strings.Contains(payload, "drop") ||
		strings.Contains(payload, "destroy") {
		return 3
	}
	if strings.Contains(payload, "internal") ||
		strings.Contains(payload, "staging") ||
		strings.Contains(payload, "queue") {
		return 2
	}
	return 1
}

func topActionReasonCodesFromResultStatus(status string) []string {
	if status == "" || status == "ok" || status == "success" {
		return nil
	}
	return []string{"result_status_" + status}
}

func topActionReasonCodesFromTrace(trace schemagate.TraceRecord, verdict string) []string {
	reasonCodes := make([]string, 0, len(trace.Violations)+1)
	if verdict != "" && verdict != "allow" {
		reasonCodes = append(reasonCodes, "trace_verdict_"+verdict)
	}
	for _, violation := range trace.Violations {
		normalized := normalizeIdentifier(strings.ToLower(strings.TrimSpace(violation)))
		if normalized == "" {
			continue
		}
		reasonCodes = append(reasonCodes, "violation_"+normalized)
	}
	return uniqueSorted(reasonCodes)
}

func topActionScore(toolClass int, blastRadius int, verdict string, reasonCodes []string) int {
	score := toolClass*100 + blastRadius*10
	switch verdict {
	case "block":
		score += 15
	case "require_approval":
		score += 12
	case "error":
		score += 8
	}
	for _, reasonCode := range reasonCodes {
		lower := strings.ToLower(reasonCode)
		switch {
		case strings.Contains(lower, "violation_"),
			strings.Contains(lower, "blocked"),
			strings.Contains(lower, "approval"),
			strings.Contains(lower, "destructive"),
			strings.Contains(lower, "credential"),
			strings.Contains(lower, "prompt_injection"):
			score += 2
		default:
			score++
		}
	}
	return score
}
