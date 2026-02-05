package runpack

import (
	"fmt"
	"sort"
)

type ReplayMode string

const (
	ReplayModeStub ReplayMode = "stub"
	ReplayModeReal ReplayMode = "real"
)

type ReplayStep struct {
	IntentID     string `json:"intent_id"`
	ToolName     string `json:"tool_name"`
	Status       string `json:"status"`
	ResultDigest string `json:"result_digest,omitempty"`
}

type ReplayResult struct {
	RunID          string       `json:"run_id"`
	Mode           ReplayMode   `json:"mode"`
	Steps          []ReplayStep `json:"steps"`
	MissingResults []string     `json:"missing_results,omitempty"`
}

func ReplayStub(path string) (ReplayResult, error) {
	pack, err := ReadRunpack(path)
	if err != nil {
		return ReplayResult{}, err
	}
	seenIntents := make(map[string]struct{}, len(pack.Intents))
	for _, intent := range pack.Intents {
		if _, exists := seenIntents[intent.IntentID]; exists {
			return ReplayResult{}, fmt.Errorf("duplicate intent_id: %s", intent.IntentID)
		}
		seenIntents[intent.IntentID] = struct{}{}
	}
	resultsByIntent := make(map[string]ReplayStep, len(pack.Results))
	for _, result := range pack.Results {
		if _, exists := resultsByIntent[result.IntentID]; exists {
			return ReplayResult{}, fmt.Errorf("duplicate result for intent_id: %s", result.IntentID)
		}
		resultsByIntent[result.IntentID] = ReplayStep{
			IntentID:     result.IntentID,
			ToolName:     "",
			Status:       result.Status,
			ResultDigest: result.ResultDigest,
		}
	}

	steps := make([]ReplayStep, 0, len(pack.Intents))
	missing := make([]string, 0)
	for _, intent := range pack.Intents {
		step := ReplayStep{
			IntentID: intent.IntentID,
			ToolName: intent.ToolName,
		}
		if recorded, ok := resultsByIntent[intent.IntentID]; ok {
			step.Status = recorded.Status
			step.ResultDigest = recorded.ResultDigest
		} else {
			step.Status = "missing_result"
			missing = append(missing, intent.IntentID)
		}
		steps = append(steps, step)
	}
	sort.Strings(missing)

	return ReplayResult{
		RunID:          pack.Run.RunID,
		Mode:           ReplayModeStub,
		Steps:          steps,
		MissingResults: missing,
	}, nil
}
