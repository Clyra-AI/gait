package scout

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

const (
	adoptionEventSchemaID = "gait.scout.adoption_event"
	adoptionEventSchemaV1 = "1.0.0"
	maxAdoptionLineBytes  = 1024 * 1024
)

var (
	adoptionMilestoneOrder = []string{"A1", "A2", "A3", "A4", "E1", "E2", "E3"}
	activationMilestones   = []string{"A1", "A2", "A3", "A4"}
	officialSkillWorkflows = []string{
		"gait-capture-runpack",
		"gait-incident-to-regression",
		"gait-policy-test-rollout",
	}
	fixedAdoptionTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
)

type AdoptionCommandStats struct {
	Command string `json:"command"`
	Total   int    `json:"total"`
	Success int    `json:"success"`
	Failure int    `json:"failure"`
}

type AdoptionMilestoneStatus struct {
	Name     string `json:"name"`
	Achieved bool   `json:"achieved"`
}

type AdoptionSkillWorkflowStats struct {
	Workflow              string  `json:"workflow"`
	Total                 int     `json:"total"`
	Success               int     `json:"success"`
	Failure               int     `json:"failure"`
	SuccessRate           float64 `json:"success_rate"`
	MedianRuntimeMS       int64   `json:"median_runtime_ms"`
	MostCommonFailureCode int     `json:"most_common_failure_code,omitempty"`
}

type AdoptionReport struct {
	SchemaID           string                       `json:"schema_id"`
	SchemaVersion      string                       `json:"schema_version"`
	CreatedAt          time.Time                    `json:"created_at"`
	ProducerVersion    string                       `json:"producer_version"`
	Source             string                       `json:"source"`
	TotalEvents        int                          `json:"total_events"`
	SuccessEvents      int                          `json:"success_events"`
	FailedEvents       int                          `json:"failed_events"`
	FirstEventAt       time.Time                    `json:"first_event_at,omitempty"`
	LastEventAt        time.Time                    `json:"last_event_at,omitempty"`
	Commands           []AdoptionCommandStats       `json:"commands"`
	Milestones         []AdoptionMilestoneStatus    `json:"milestones"`
	ActivationTimingMS map[string]int64             `json:"activation_timing_ms,omitempty"`
	ActivationMedians  map[string]int64             `json:"activation_medians_ms,omitempty"`
	SkillWorkflows     []AdoptionSkillWorkflowStats `json:"skill_workflows,omitempty"`
	ActivationComplete bool                         `json:"activation_complete"`
	Blockers           []string                     `json:"blockers,omitempty"`
}

type adoptionWorkflowAccumulator struct {
	Total        int
	Success      int
	Failure      int
	RuntimeMS    []int64
	FailureCodes map[int]int
}

func NewAdoptionEvent(
	command string,
	exitCode int,
	elapsed time.Duration,
	producerVersion string,
	now time.Time,
	workflowID string,
) schemascout.AdoptionEvent {
	createdAt := now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if elapsed < 0 {
		elapsed = 0
	}
	trimmedCommand := strings.TrimSpace(command)
	if trimmedCommand == "" {
		trimmedCommand = "unknown"
	}
	trimmedProducerVersion := strings.TrimSpace(producerVersion)
	if trimmedProducerVersion == "" {
		trimmedProducerVersion = "0.0.0-dev"
	}
	success := exitCode == 0
	trimmedWorkflowID := strings.TrimSpace(workflowID)
	return schemascout.AdoptionEvent{
		SchemaID:        adoptionEventSchemaID,
		SchemaVersion:   adoptionEventSchemaV1,
		CreatedAt:       createdAt,
		ProducerVersion: trimmedProducerVersion,
		Command:         trimmedCommand,
		WorkflowID:      trimmedWorkflowID,
		Success:         success,
		ExitCode:        exitCode,
		ElapsedMS:       elapsed.Milliseconds(),
		Milestones:      milestoneTags(trimmedCommand, success),
		Environment: schemascout.AdoptionEnvContext{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
	}
}

func AppendAdoptionEvent(path string, event schemascout.AdoptionEvent) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("adoption log path is required")
	}
	normalized, err := normalizeAdoptionEvent(event)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal adoption event: %w", err)
	}
	if err := fsx.AppendLineLocked(trimmedPath, encoded, 0o600); err != nil {
		return fmt.Errorf("append adoption log: %w", err)
	}
	return nil
}

func LoadAdoptionEvents(path string) ([]schemascout.AdoptionEvent, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, fmt.Errorf("adoption log path is required")
	}
	// #nosec G304 -- adoption log path is explicit local user input.
	file, err := os.Open(trimmedPath)
	if err != nil {
		return nil, fmt.Errorf("open adoption log: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	events := make([]schemascout.AdoptionEvent, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxAdoptionLineBytes)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var event schemascout.AdoptionEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return nil, fmt.Errorf("parse adoption log line %d: %w", line, err)
		}
		normalized, err := normalizeAdoptionEvent(event)
		if err != nil {
			return nil, fmt.Errorf("validate adoption log line %d: %w", line, err)
		}
		events = append(events, normalized)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan adoption log: %w", err)
	}
	return events, nil
}

func BuildAdoptionReport(
	events []schemascout.AdoptionEvent,
	source string,
	producerVersion string,
	now time.Time,
) AdoptionReport {
	createdAt := now.UTC()
	if createdAt.IsZero() {
		createdAt = fixedAdoptionTime
	}
	trimmedProducerVersion := strings.TrimSpace(producerVersion)
	if trimmedProducerVersion == "" {
		trimmedProducerVersion = "0.0.0-dev"
	}
	statsByCommand := map[string]AdoptionCommandStats{}
	milestonesAchieved := map[string]struct{}{}
	milestoneFirstSuccessAt := map[string]time.Time{}
	milestoneElapsedSamples := map[string][]int64{
		"A1": {},
		"A4": {},
	}
	workflowAccumulators := map[string]*adoptionWorkflowAccumulator{}
	totalSuccess := 0
	totalFailed := 0
	firstEventAt := time.Time{}
	lastEventAt := time.Time{}

	for _, event := range events {
		command := strings.TrimSpace(event.Command)
		if command == "" {
			command = "unknown"
		}
		stats := statsByCommand[command]
		stats.Command = command
		stats.Total++
		if event.Success {
			stats.Success++
			totalSuccess++
		} else {
			stats.Failure++
			totalFailed++
		}
		statsByCommand[command] = stats

		for _, tag := range event.Milestones {
			trimmedTag := strings.TrimSpace(tag)
			if trimmedTag == "" {
				continue
			}
			milestonesAchieved[trimmedTag] = struct{}{}
			if event.Success {
				if _, ok := milestoneFirstSuccessAt[trimmedTag]; !ok || event.CreatedAt.Before(milestoneFirstSuccessAt[trimmedTag]) {
					milestoneFirstSuccessAt[trimmedTag] = event.CreatedAt
				}
				if _, collect := milestoneElapsedSamples[trimmedTag]; collect {
					milestoneElapsedSamples[trimmedTag] = append(milestoneElapsedSamples[trimmedTag], event.ElapsedMS)
				}
			}
		}
		workflowID := resolveSkillWorkflowID(event)
		if workflowID != "" {
			accumulator, ok := workflowAccumulators[workflowID]
			if !ok {
				accumulator = &adoptionWorkflowAccumulator{
					RuntimeMS:    make([]int64, 0, 8),
					FailureCodes: map[int]int{},
				}
				workflowAccumulators[workflowID] = accumulator
			}
			accumulator.Total++
			accumulator.RuntimeMS = append(accumulator.RuntimeMS, event.ElapsedMS)
			if event.Success {
				accumulator.Success++
			} else {
				accumulator.Failure++
				accumulator.FailureCodes[event.ExitCode]++
			}
		}
		if firstEventAt.IsZero() || event.CreatedAt.Before(firstEventAt) {
			firstEventAt = event.CreatedAt
		}
		if lastEventAt.IsZero() || event.CreatedAt.After(lastEventAt) {
			lastEventAt = event.CreatedAt
		}
	}

	commandStats := make([]AdoptionCommandStats, 0, len(statsByCommand))
	for _, stats := range statsByCommand {
		commandStats = append(commandStats, stats)
	}
	sort.Slice(commandStats, func(i, j int) bool {
		return commandStats[i].Command < commandStats[j].Command
	})

	milestones := make([]AdoptionMilestoneStatus, 0, len(adoptionMilestoneOrder))
	for _, name := range adoptionMilestoneOrder {
		_, achieved := milestonesAchieved[name]
		milestones = append(milestones, AdoptionMilestoneStatus{
			Name:     name,
			Achieved: achieved,
		})
	}

	activationComplete := true
	blockers := make([]string, 0)
	for _, milestone := range activationMilestones {
		if _, ok := milestonesAchieved[milestone]; ok {
			continue
		}
		activationComplete = false
		blockers = append(blockers, activationBlockerHint(milestone))
	}
	blockers = uniqueSorted(blockers)

	activationTimingMS := map[string]int64{}
	if !firstEventAt.IsZero() {
		for _, milestone := range activationMilestones {
			if achievedAt, ok := milestoneFirstSuccessAt[milestone]; ok {
				delta := achievedAt.Sub(firstEventAt).Milliseconds()
				if delta < 0 {
					delta = 0
				}
				activationTimingMS[milestone] = delta
			}
		}
	}

	activationMedians := map[string]int64{}
	if median, ok := medianInt64(milestoneElapsedSamples["A1"]); ok {
		activationMedians["m1_demo_elapsed_ms"] = median
	}
	if median, ok := medianInt64(milestoneElapsedSamples["A4"]); ok {
		activationMedians["m2_regress_run_elapsed_ms"] = median
	}

	skillWorkflowStats := buildSkillWorkflowStats(workflowAccumulators)

	if !lastEventAt.IsZero() {
		createdAt = lastEventAt.UTC()
	}

	return AdoptionReport{
		SchemaID:           "gait.doctor.adoption_report",
		SchemaVersion:      "1.0.0",
		CreatedAt:          createdAt,
		ProducerVersion:    trimmedProducerVersion,
		Source:             strings.TrimSpace(source),
		TotalEvents:        len(events),
		SuccessEvents:      totalSuccess,
		FailedEvents:       totalFailed,
		FirstEventAt:       firstEventAt,
		LastEventAt:        lastEventAt,
		Commands:           commandStats,
		Milestones:         milestones,
		ActivationTimingMS: activationTimingMS,
		ActivationMedians:  activationMedians,
		SkillWorkflows:     skillWorkflowStats,
		ActivationComplete: activationComplete,
		Blockers:           blockers,
	}
}

func normalizeAdoptionEvent(event schemascout.AdoptionEvent) (schemascout.AdoptionEvent, error) {
	output := event
	if strings.TrimSpace(output.SchemaID) == "" {
		output.SchemaID = adoptionEventSchemaID
	}
	if output.SchemaID != adoptionEventSchemaID {
		return schemascout.AdoptionEvent{}, fmt.Errorf("unsupported schema_id %q", output.SchemaID)
	}
	if strings.TrimSpace(output.SchemaVersion) == "" {
		output.SchemaVersion = adoptionEventSchemaV1
	}
	if output.SchemaVersion != adoptionEventSchemaV1 {
		return schemascout.AdoptionEvent{}, fmt.Errorf("unsupported schema_version %q", output.SchemaVersion)
	}
	if output.CreatedAt.IsZero() {
		return schemascout.AdoptionEvent{}, fmt.Errorf("created_at is required")
	}
	output.CreatedAt = output.CreatedAt.UTC()
	output.ProducerVersion = strings.TrimSpace(output.ProducerVersion)
	if output.ProducerVersion == "" {
		output.ProducerVersion = "0.0.0-dev"
	}
	output.Command = strings.TrimSpace(output.Command)
	if output.Command == "" {
		return schemascout.AdoptionEvent{}, fmt.Errorf("command is required")
	}
	output.WorkflowID = strings.TrimSpace(output.WorkflowID)
	if output.ExitCode < 0 {
		return schemascout.AdoptionEvent{}, fmt.Errorf("exit_code must be >= 0")
	}
	if output.ExitCode > 255 {
		return schemascout.AdoptionEvent{}, fmt.Errorf("exit_code must be <= 255")
	}
	if output.ElapsedMS < 0 {
		output.ElapsedMS = 0
	}
	output.Milestones = uniqueSorted(output.Milestones)
	output.Environment.OS = strings.TrimSpace(output.Environment.OS)
	output.Environment.Arch = strings.TrimSpace(output.Environment.Arch)
	if output.Environment.OS == "" {
		return schemascout.AdoptionEvent{}, fmt.Errorf("environment.os is required")
	}
	if output.Environment.Arch == "" {
		return schemascout.AdoptionEvent{}, fmt.Errorf("environment.arch is required")
	}
	return output, nil
}

func resolveSkillWorkflowID(event schemascout.AdoptionEvent) string {
	workflowID := strings.TrimSpace(event.WorkflowID)
	if workflowID != "" {
		return workflowID
	}
	switch strings.TrimSpace(event.Command) {
	case "run record", "verify":
		return "gait-capture-runpack"
	case "regress init", "regress run", "regress bootstrap":
		return "gait-incident-to-regression"
	case "policy test", "policy simulate", "gate eval":
		return "gait-policy-test-rollout"
	default:
		return ""
	}
}

func buildSkillWorkflowStats(accumulators map[string]*adoptionWorkflowAccumulator) []AdoptionSkillWorkflowStats {
	stats := make([]AdoptionSkillWorkflowStats, 0, len(accumulators))
	for _, workflow := range officialSkillWorkflows {
		accumulator, ok := accumulators[workflow]
		if !ok || accumulator == nil || accumulator.Total == 0 {
			continue
		}
		successRate := 0.0
		if accumulator.Total > 0 {
			successRate = float64(accumulator.Success) / float64(accumulator.Total)
		}
		medianRuntimeMS, _ := medianInt64(accumulator.RuntimeMS)
		stats = append(stats, AdoptionSkillWorkflowStats{
			Workflow:              workflow,
			Total:                 accumulator.Total,
			Success:               accumulator.Success,
			Failure:               accumulator.Failure,
			SuccessRate:           successRate,
			MedianRuntimeMS:       medianRuntimeMS,
			MostCommonFailureCode: mostCommonFailureCode(accumulator.FailureCodes),
		})
	}
	return stats
}

func medianInt64(values []int64) (int64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	copied := append([]int64(nil), values...)
	sort.Slice(copied, func(i, j int) bool {
		return copied[i] < copied[j]
	})
	mid := len(copied) / 2
	if len(copied)%2 == 1 {
		return copied[mid], true
	}
	return (copied[mid-1] + copied[mid]) / 2, true
}

func mostCommonFailureCode(failureCodes map[int]int) int {
	if len(failureCodes) == 0 {
		return 0
	}
	selectedCode := 0
	selectedCount := -1
	for code, count := range failureCodes {
		if count > selectedCount {
			selectedCount = count
			selectedCode = code
			continue
		}
		if count == selectedCount && code < selectedCode {
			selectedCode = code
		}
	}
	return selectedCode
}

func milestoneTags(command string, success bool) []string {
	if !success {
		return nil
	}
	switch command {
	case "demo":
		return []string{"A1"}
	case "verify":
		return []string{"A2"}
	case "regress init":
		return []string{"A3"}
	case "regress run":
		return []string{"A4"}
	default:
		return nil
	}
}

func activationBlockerHint(milestone string) string {
	switch milestone {
	case "A1":
		return "missing A1: run `gait demo`"
	case "A2":
		return "missing A2: run `gait verify <run_id>`"
	case "A3":
		return "missing A3: run `gait regress init --from <run_id>`"
	case "A4":
		return "missing A4: run `gait regress run`"
	default:
		return "missing milestone: " + milestone
	}
}
