package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidahmann/gait/core/scout"
)

type reportTopOutput struct {
	OK          bool                    `json:"ok"`
	OutputPath  string                  `json:"output_path,omitempty"`
	RunCount    int                     `json:"run_count,omitempty"`
	TraceCount  int                     `json:"trace_count,omitempty"`
	ActionCount int                     `json:"action_count,omitempty"`
	TopActions  int                     `json:"top_actions,omitempty"`
	Report      *scout.TopActionsReport `json:"report,omitempty"`
	Error       string                  `json:"error,omitempty"`
}

func runReport(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Rank the highest-risk actions from runpacks/traces and emit a deterministic offline triage report.")
	}
	if len(arguments) == 0 {
		printReportUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "top":
		return runReportTop(arguments[1:])
	default:
		printReportUsage()
		return exitInvalidInput
	}
}

func runReportTop(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Rank top risky actions deterministically by tool class and blast radius from runpack and trace artifacts.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"runs":   true,
		"traces": true,
		"limit":  true,
		"out":    true,
	})
	flagSet := flag.NewFlagSet("report-top", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var runsCSV string
	var tracesCSV string
	var limit int
	var outPath string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&runsCSV, "runs", "", "comma-separated runpack paths, run IDs, or directories")
	flagSet.StringVar(&tracesCSV, "traces", "", "comma-separated trace paths or directories")
	flagSet.IntVar(&limit, "limit", 5, "maximum number of top actions to emit (1-20)")
	flagSet.StringVar(&outPath, "out", "", "path to write top actions report JSON")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeReportTopOutput(jsonOutput, reportTopOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printReportTopUsage()
		return exitOK
	}
	if limit < 1 {
		return writeReportTopOutput(jsonOutput, reportTopOutput{OK: false, Error: "--limit must be >= 1"}, exitInvalidInput)
	}

	runSources := parseCSVList(runsCSV)
	runSources = append(runSources, flagSet.Args()...)
	traceSources := parseCSVList(tracesCSV)
	if len(runSources) == 0 && len(traceSources) == 0 {
		return writeReportTopOutput(jsonOutput, reportTopOutput{OK: false, Error: "missing --runs and/or --traces sources"}, exitInvalidInput)
	}

	runpackPaths, err := resolveReportRunpackPaths(runSources)
	if err != nil {
		return writeReportTopOutput(jsonOutput, reportTopOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	tracePaths, err := resolveReportTracePaths(traceSources)
	if err != nil {
		return writeReportTopOutput(jsonOutput, reportTopOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if len(runpackPaths) == 0 && len(tracePaths) == 0 {
		return writeReportTopOutput(jsonOutput, reportTopOutput{OK: false, Error: "no runpack or trace artifacts found in provided sources"}, exitInvalidInput)
	}

	report, err := scout.BuildTopActionsReport(scout.TopActionsInput{
		RunpackPaths: runpackPaths,
		TracePaths:   tracePaths,
		Limit:        limit,
	}, scout.TopActionsOptions{
		ProducerVersion: version,
	})
	if err != nil {
		return writeReportTopOutput(jsonOutput, reportTopOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if strings.TrimSpace(outPath) == "" {
		outPath = "./gait-out/report_top_actions.json"
	}
	if err := writeJSONFile(outPath, report); err != nil {
		return writeReportTopOutput(jsonOutput, reportTopOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	return writeReportTopOutput(jsonOutput, reportTopOutput{
		OK:          true,
		OutputPath:  outPath,
		RunCount:    report.RunCount,
		TraceCount:  report.TraceCount,
		ActionCount: report.ActionCount,
		TopActions:  len(report.TopActions),
		Report:      &report,
	}, exitOK)
}

func resolveReportRunpackPaths(sources []string) ([]string, error) {
	resolved := make([]string, 0, len(sources))
	for _, source := range sources {
		trimmed := strings.TrimSpace(source)
		if trimmed == "" {
			continue
		}
		sanitized, err := sanitizeReportInputPath(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid run source %q: %w", trimmed, err)
		}
		info, statErr := os.Stat(sanitized)
		if statErr == nil {
			if info.IsDir() {
				collected, err := collectRunpackPathsFromDir(sanitized)
				if err != nil {
					return nil, err
				}
				resolved = append(resolved, collected...)
			} else {
				resolved = append(resolved, sanitized)
			}
			continue
		}
		if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("inspect run source %s: %w", sanitized, statErr)
		}
		if looksLikePath(trimmed) {
			return nil, fmt.Errorf("run source not found: %s", trimmed)
		}
		runpackPath, err := resolveRunpackPath(trimmed)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, runpackPath)
	}
	return uniqueSortedStrings(resolved), nil
}

func resolveReportTracePaths(sources []string) ([]string, error) {
	resolved := make([]string, 0, len(sources))
	for _, source := range sources {
		trimmed := strings.TrimSpace(source)
		if trimmed == "" {
			continue
		}
		sanitized, err := sanitizeReportInputPath(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid trace source %q: %w", trimmed, err)
		}
		info, statErr := os.Stat(sanitized)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				return nil, fmt.Errorf("trace source not found: %s", trimmed)
			}
			return nil, fmt.Errorf("inspect trace source %s: %w", sanitized, statErr)
		}
		if info.IsDir() {
			collected, err := collectTracePathsFromDir(sanitized)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, collected...)
			continue
		}
		if !strings.HasSuffix(strings.ToLower(sanitized), ".json") {
			return nil, fmt.Errorf("trace source must be a .json file or directory: %s", sanitized)
		}
		resolved = append(resolved, sanitized)
	}
	return uniqueSortedStrings(resolved), nil
}

func collectRunpackPathsFromDir(dir string) ([]string, error) {
	paths := make([]string, 0, 8)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		if strings.HasPrefix(base, "runpack_") && strings.HasSuffix(base, ".zip") {
			paths = append(paths, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk runpack directory %s: %w", dir, err)
	}
	return uniqueSortedStrings(paths), nil
}

func collectTracePathsFromDir(dir string) ([]string, error) {
	paths := make([]string, 0, 8)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		if strings.HasPrefix(base, "trace_") && strings.HasSuffix(base, ".json") {
			paths = append(paths, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk trace directory %s: %w", dir, err)
	}
	return uniqueSortedStrings(paths), nil
}

func sanitizeReportInputPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("path is required")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsLocal(cleaned) || filepath.IsAbs(cleaned) {
		return cleaned, nil
	}
	if volume := filepath.VolumeName(cleaned); volume != "" && strings.HasPrefix(cleaned, volume+string(filepath.Separator)) {
		return cleaned, nil
	}
	return "", fmt.Errorf("path must be local relative or absolute")
}

func writeReportTopOutput(jsonOutput bool, output reportTopOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("report top ok: runs=%d traces=%d actions=%d top=%d\n", output.RunCount, output.TraceCount, output.ActionCount, output.TopActions)
		fmt.Printf("output: %s\n", output.OutputPath)
		if output.Report != nil {
			for _, action := range output.Report.TopActions {
				verdict := action.Verdict
				if verdict == "" {
					verdict = "n/a"
				}
				fmt.Printf(
					"%d. score=%d class=%s blast=%d tool=%s run=%s verdict=%s source=%s\n",
					action.Rank,
					action.Score,
					action.ToolClass,
					action.BlastRadius,
					action.ToolName,
					action.RunID,
					verdict,
					action.SourceArtifact,
				)
			}
		}
		return exitCode
	}
	fmt.Printf("report top error: %s\n", output.Error)
	return exitCode
}

func printReportUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait report top --runs <csv|run_id|dir> [--traces <csv|dir>] [--limit <n>] [--out <report.json>] [--json] [--explain]")
}

func printReportTopUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait report top --runs <csv|run_id|dir> [--traces <csv|dir>] [--limit <n>] [--out <report.json>] [--json] [--explain]")
	fmt.Println("  gait report top <run_id|runpack_path|runpack_dir> [--traces <csv|dir>] [--limit <n>] [--out <report.json>] [--json] [--explain]")
}
