package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

type runInspectEntry struct {
	Index        int       `json:"index"`
	IntentID     string    `json:"intent_id"`
	ToolName     string    `json:"tool_name"`
	CreatedAt    time.Time `json:"created_at"`
	ArgsDigest   string    `json:"args_digest,omitempty"`
	Status       string    `json:"status,omitempty"`
	ResultDigest string    `json:"result_digest,omitempty"`
	Verdict      string    `json:"verdict,omitempty"`
	ReasonCodes  []string  `json:"reason_codes,omitempty"`
	Violations   []string  `json:"violations,omitempty"`
}

type runInspectUnmatchedResult struct {
	IntentID     string   `json:"intent_id"`
	Status       string   `json:"status,omitempty"`
	ResultDigest string   `json:"result_digest,omitempty"`
	Verdict      string   `json:"verdict,omitempty"`
	ReasonCodes  []string `json:"reason_codes,omitempty"`
	Violations   []string `json:"violations,omitempty"`
}

type runInspectOutput struct {
	OK               bool                        `json:"ok"`
	RunID            string                      `json:"run_id,omitempty"`
	Path             string                      `json:"path,omitempty"`
	CaptureMode      string                      `json:"capture_mode,omitempty"`
	IntentsTotal     int                         `json:"intents_total,omitempty"`
	ResultsTotal     int                         `json:"results_total,omitempty"`
	Entries          []runInspectEntry           `json:"entries,omitempty"`
	UnmatchedResults []runInspectUnmatchedResult `json:"unmatched_results,omitempty"`
	Warnings         []string                    `json:"warnings,omitempty"`
	Error            string                      `json:"error,omitempty"`
}

func runInspect(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Render a deterministic timeline of intents and outcomes from an existing runpack.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"from": true,
	})
	flagSet := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var from string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&from, "from", "", "run_id or runpack path")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRunInspectOutput(jsonOutput, runInspectOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRunInspectUsage()
		return exitOK
	}

	remaining := flagSet.Args()
	if strings.TrimSpace(from) == "" && len(remaining) == 1 {
		from = remaining[0]
		remaining = nil
	}
	if strings.TrimSpace(from) == "" {
		return writeRunInspectOutput(jsonOutput, runInspectOutput{OK: false, Error: "missing required --from <run_id|path>"}, exitInvalidInput)
	}
	if len(remaining) > 0 {
		return writeRunInspectOutput(jsonOutput, runInspectOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	resolvedPath, err := resolveRunpackPath(from)
	if err != nil {
		return writeRunInspectOutput(jsonOutput, runInspectOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	pack, err := runpack.ReadRunpack(resolvedPath)
	if err != nil {
		return writeRunInspectOutput(jsonOutput, runInspectOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	entries := make([]runInspectEntry, 0, len(pack.Intents))
	intentIDs := make(map[string]struct{}, len(pack.Intents))
	resultsByIntent := make(map[string]runInspectUnmatchedResult, len(pack.Results))
	duplicateResultByIntent := make(map[string]int)

	for _, result := range pack.Results {
		resultEntry := buildUnmatchedResult(result)
		intentID := strings.TrimSpace(result.IntentID)
		if intentID == "" {
			intentID = "(empty)"
		}
		if _, exists := resultsByIntent[result.IntentID]; exists {
			duplicateResultByIntent[intentID]++
			continue
		}
		resultsByIntent[result.IntentID] = resultEntry
	}

	for idx, intent := range pack.Intents {
		intentIDs[intent.IntentID] = struct{}{}
		entry := runInspectEntry{
			Index:      idx + 1,
			IntentID:   intent.IntentID,
			ToolName:   intent.ToolName,
			CreatedAt:  intent.CreatedAt.UTC(),
			ArgsDigest: intent.ArgsDigest,
		}
		if result, ok := resultsByIntent[intent.IntentID]; ok {
			entry.Status = result.Status
			entry.ResultDigest = result.ResultDigest
			entry.Verdict = result.Verdict
			entry.ReasonCodes = result.ReasonCodes
			entry.Violations = result.Violations
		}
		entries = append(entries, entry)
	}

	unmatched := make([]runInspectUnmatchedResult, 0)
	for _, result := range pack.Results {
		if _, ok := intentIDs[result.IntentID]; ok {
			continue
		}
		unmatched = append(unmatched, buildUnmatchedResult(result))
	}
	sort.Slice(unmatched, func(i, j int) bool {
		if unmatched[i].IntentID == unmatched[j].IntentID {
			return unmatched[i].ResultDigest < unmatched[j].ResultDigest
		}
		return unmatched[i].IntentID < unmatched[j].IntentID
	})

	warnings := make([]string, 0, len(duplicateResultByIntent))
	if len(duplicateResultByIntent) > 0 {
		keys := make([]string, 0, len(duplicateResultByIntent))
		for intentID := range duplicateResultByIntent {
			keys = append(keys, intentID)
		}
		sort.Strings(keys)
		for _, intentID := range keys {
			warnings = append(warnings, fmt.Sprintf("multiple result records for intent_id=%s (kept first)", intentID))
		}
	}

	return writeRunInspectOutput(jsonOutput, runInspectOutput{
		OK:               true,
		RunID:            pack.Run.RunID,
		Path:             resolvedPath,
		CaptureMode:      pack.Manifest.CaptureMode,
		IntentsTotal:     len(pack.Intents),
		ResultsTotal:     len(pack.Results),
		Entries:          entries,
		UnmatchedResults: unmatched,
		Warnings:         warnings,
	}, exitOK)
}

func buildUnmatchedResult(result schemarunpack.ResultRecord) runInspectUnmatchedResult {
	return runInspectUnmatchedResult{
		IntentID:     result.IntentID,
		Status:       result.Status,
		ResultDigest: result.ResultDigest,
		Verdict:      stringField(result.Result, "verdict"),
		ReasonCodes:  stringListField(result.Result, "reason_codes"),
		Violations:   stringListField(result.Result, "violations"),
	}
}

func stringField(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func stringListField(payload map[string]any, key string) []string {
	if payload == nil {
		return nil
	}
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return uniqueSortedStringsInspect(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			trimmed := strings.TrimSpace(text)
			if trimmed == "" {
				continue
			}
			values = append(values, trimmed)
		}
		return uniqueSortedStringsInspect(values)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	default:
		return nil
	}
}

func uniqueSortedStringsInspect(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func writeRunInspectOutput(jsonOutput bool, output runInspectOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if !output.OK {
		fmt.Printf("inspect error: %s\n", output.Error)
		return exitCode
	}

	fmt.Printf("run inspect: run_id=%s intents=%d results=%d capture_mode=%s\n", output.RunID, output.IntentsTotal, output.ResultsTotal, output.CaptureMode)
	fmt.Printf("path: %s\n", output.Path)
	for _, entry := range output.Entries {
		line := fmt.Sprintf("%d. intent=%s tool=%s status=%s", entry.Index, entry.IntentID, entry.ToolName, fallbackValue(entry.Status, "missing_result"))
		if entry.Verdict != "" {
			line += fmt.Sprintf(" verdict=%s", entry.Verdict)
		}
		fmt.Println(line)
		if len(entry.ReasonCodes) > 0 {
			fmt.Printf("   reason_codes=%s\n", strings.Join(entry.ReasonCodes, ","))
		}
		if len(entry.Violations) > 0 {
			fmt.Printf("   violations=%s\n", strings.Join(entry.Violations, ","))
		}
	}
	if len(output.UnmatchedResults) > 0 {
		fmt.Printf("unmatched_results=%d\n", len(output.UnmatchedResults))
		for _, unmatched := range output.UnmatchedResults {
			fmt.Printf("   intent=%s status=%s\n", unmatched.IntentID, fallbackValue(unmatched.Status, "unknown"))
		}
	}
	if len(output.Warnings) > 0 {
		fmt.Printf("warnings: %s\n", strings.Join(output.Warnings, "; "))
	}
	return exitCode
}

func fallbackValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func printRunInspectUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run inspect --from <run_id|path> [--json] [--explain]")
	fmt.Println("  gait run inspect <run_id|path> [--json] [--explain]")
}
