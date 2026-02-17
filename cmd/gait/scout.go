package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
	"github.com/Clyra-AI/gait/core/scout"
)

type scoutSnapshotOutput struct {
	OK           bool                  `json:"ok"`
	SnapshotPath string                `json:"snapshot_path,omitempty"`
	CoveragePath string                `json:"coverage_path,omitempty"`
	SnapshotID   string                `json:"snapshot_id,omitempty"`
	Items        int                   `json:"items,omitempty"`
	Coverage     *scout.CoverageReport `json:"coverage,omitempty"`
	Error        string                `json:"error,omitempty"`
}

type scoutDiffOutput struct {
	OK         bool                `json:"ok"`
	Left       string              `json:"left,omitempty"`
	Right      string              `json:"right,omitempty"`
	OutputPath string              `json:"output_path,omitempty"`
	Diff       *scout.SnapshotDiff `json:"diff,omitempty"`
	Error      string              `json:"error,omitempty"`
}

type scoutSignalOutput struct {
	OK          bool                      `json:"ok"`
	OutputPath  string                    `json:"output_path,omitempty"`
	RunCount    int                       `json:"run_count,omitempty"`
	FamilyCount int                       `json:"family_count,omitempty"`
	TopIssues   int                       `json:"top_issues,omitempty"`
	Report      *schemascout.SignalReport `json:"report,omitempty"`
	Error       string                    `json:"error,omitempty"`
}

func runScout(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Discover tool inventory coverage, compute policy coverage metrics, and diff snapshots deterministically.")
	}
	if len(arguments) == 0 {
		printScoutUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "snapshot":
		return runScoutSnapshot(arguments[1:])
	case "diff":
		return runScoutDiff(arguments[1:])
	case "signal":
		return runScoutSignal(arguments[1:])
	default:
		printScoutUsage()
		return exitInvalidInput
	}
}

func runScoutSnapshot(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Generate an inventory snapshot from workspace files and optionally compute gating coverage from policy files.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"roots":        true,
		"include":      true,
		"exclude":      true,
		"policy":       true,
		"out":          true,
		"coverage-out": true,
	})
	flagSet := flag.NewFlagSet("scout-snapshot", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var rootsCSV string
	var includeCSV string
	var excludeCSV string
	var policyCSV string
	var outPath string
	var coverageOutPath string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&rootsCSV, "roots", ".", "comma-separated roots to scan")
	flagSet.StringVar(&includeCSV, "include", "", "comma-separated include patterns")
	flagSet.StringVar(&excludeCSV, "exclude", "", "comma-separated exclude patterns")
	flagSet.StringVar(&policyCSV, "policy", "", "comma-separated policy files for coverage")
	flagSet.StringVar(&outPath, "out", "", "path to write snapshot JSON")
	flagSet.StringVar(&coverageOutPath, "coverage-out", "", "path to write coverage JSON")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeScoutSnapshotOutput(jsonOutput, scoutSnapshotOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printScoutSnapshotUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeScoutSnapshotOutput(jsonOutput, scoutSnapshotOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	provider := scout.DefaultProvider{Options: scout.SnapshotOptions{ProducerVersion: version}}
	snapshot, err := provider.Snapshot(context.Background(), scout.SnapshotRequest{
		Roots:   parseCSVList(rootsCSV),
		Include: parseCSVList(includeCSV),
		Exclude: parseCSVList(excludeCSV),
	})
	if err != nil {
		return writeScoutSnapshotOutput(jsonOutput, scoutSnapshotOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	if strings.TrimSpace(outPath) == "" {
		outPath = fmt.Sprintf("./gait-out/inventory_snapshot_%s.json", snapshot.SnapshotID)
	}
	if err := writeJSONFile(outPath, snapshot); err != nil {
		return writeScoutSnapshotOutput(jsonOutput, scoutSnapshotOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	var coverage *scout.CoverageReport
	policyPaths := parseCSVList(policyCSV)
	if len(policyPaths) > 0 {
		report, reportErr := scout.BuildCoverage(snapshot, policyPaths)
		if reportErr != nil {
			return writeScoutSnapshotOutput(jsonOutput, scoutSnapshotOutput{OK: false, Error: reportErr.Error()}, exitInvalidInput)
		}
		coverage = &report
		if strings.TrimSpace(coverageOutPath) == "" {
			coverageOutPath = fmt.Sprintf("./gait-out/inventory_coverage_%s.json", snapshot.SnapshotID)
		}
		if err := writeJSONFile(coverageOutPath, report); err != nil {
			return writeScoutSnapshotOutput(jsonOutput, scoutSnapshotOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
	}

	return writeScoutSnapshotOutput(jsonOutput, scoutSnapshotOutput{
		OK:           true,
		SnapshotPath: outPath,
		CoveragePath: coverageOutPath,
		SnapshotID:   snapshot.SnapshotID,
		Items:        len(snapshot.Items),
		Coverage:     coverage,
	}, exitOK)
}

func runScoutDiff(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Diff two scout inventory snapshots deterministically and optionally write a drift report.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"left":  true,
		"right": true,
		"out":   true,
	})
	flagSet := flag.NewFlagSet("scout-diff", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var leftPath string
	var rightPath string
	var outPath string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&leftPath, "left", "", "left snapshot path")
	flagSet.StringVar(&rightPath, "right", "", "right snapshot path")
	flagSet.StringVar(&outPath, "out", "", "path to write diff report")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeScoutDiffOutput(jsonOutput, scoutDiffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printScoutDiffUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if leftPath == "" && len(remaining) > 0 {
		leftPath = remaining[0]
		remaining = remaining[1:]
	}
	if rightPath == "" && len(remaining) > 0 {
		rightPath = remaining[0]
		remaining = remaining[1:]
	}
	if leftPath == "" || rightPath == "" || len(remaining) > 0 {
		return writeScoutDiffOutput(jsonOutput, scoutDiffOutput{OK: false, Error: "expected <left_snapshot.json> <right_snapshot.json>"}, exitInvalidInput)
	}

	leftSnapshot, err := readInventorySnapshot(leftPath)
	if err != nil {
		return writeScoutDiffOutput(jsonOutput, scoutDiffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	rightSnapshot, err := readInventorySnapshot(rightPath)
	if err != nil {
		return writeScoutDiffOutput(jsonOutput, scoutDiffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	diff := scout.DiffSnapshots(leftSnapshot, rightSnapshot)
	if strings.TrimSpace(outPath) != "" {
		if err := writeJSONFile(outPath, diff); err != nil {
			return writeScoutDiffOutput(jsonOutput, scoutDiffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
	}
	ok := diff.AddedCount == 0 && diff.RemovedCount == 0 && diff.ChangedCount == 0
	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writeScoutDiffOutput(jsonOutput, scoutDiffOutput{
		OK:         ok,
		Left:       leftPath,
		Right:      rightPath,
		OutputPath: outPath,
		Diff:       &diff,
	}, exitCode)
}

func runScoutSignal(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Cluster incident families from runpacks and emit deterministic offline drift/risk signals.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"runs":    true,
		"traces":  true,
		"regress": true,
		"out":     true,
	})
	flagSet := flag.NewFlagSet("scout-signal", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var runsCSV string
	var tracesCSV string
	var regressCSV string
	var outPath string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&runsCSV, "runs", "", "comma-separated run_id or runpack paths")
	flagSet.StringVar(&tracesCSV, "traces", "", "comma-separated gate trace paths")
	flagSet.StringVar(&regressCSV, "regress", "", "comma-separated regress result paths")
	flagSet.StringVar(&outPath, "out", "", "path to write signal report JSON")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeScoutSignalOutput(jsonOutput, scoutSignalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printScoutSignalUsage()
		return exitOK
	}

	runSources := parseCSVList(runsCSV)
	runSources = append(runSources, flagSet.Args()...)
	if len(runSources) == 0 {
		return writeScoutSignalOutput(jsonOutput, scoutSignalOutput{OK: false, Error: "missing required --runs <csv> or positional run sources"}, exitInvalidInput)
	}

	resolvedRunpacks := make([]string, 0, len(runSources))
	for _, runSource := range runSources {
		resolved, err := resolveRunpackPath(runSource)
		if err != nil {
			return writeScoutSignalOutput(jsonOutput, scoutSignalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
		resolvedRunpacks = append(resolvedRunpacks, resolved)
	}

	report, err := scout.BuildSignalReport(scout.SignalInput{
		RunpackPaths: resolvedRunpacks,
		TracePaths:   parseCSVList(tracesCSV),
		RegressPaths: parseCSVList(regressCSV),
	}, scout.SignalOptions{
		ProducerVersion: version,
	})
	if err != nil {
		return writeScoutSignalOutput(jsonOutput, scoutSignalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	if strings.TrimSpace(outPath) == "" {
		outPath = "./gait-out/scout_signal_report.json"
	}
	if err := writeJSONFile(outPath, report); err != nil {
		return writeScoutSignalOutput(jsonOutput, scoutSignalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	return writeScoutSignalOutput(jsonOutput, scoutSignalOutput{
		OK:          true,
		OutputPath:  outPath,
		RunCount:    report.RunCount,
		FamilyCount: report.FamilyCount,
		TopIssues:   len(report.TopIssues),
		Report:      &report,
	}, exitOK)
}

func parseCSVList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	values := strings.Split(value, ",")
	out := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func writeJSONFile(path string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("mkdir output dir: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func readInventorySnapshot(path string) (schemascout.InventorySnapshot, error) {
	// #nosec G304 -- user-supplied local file path.
	raw, err := os.ReadFile(path)
	if err != nil {
		return schemascout.InventorySnapshot{}, fmt.Errorf("read snapshot %s: %w", path, err)
	}
	var snapshot schemascout.InventorySnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return schemascout.InventorySnapshot{}, fmt.Errorf("parse snapshot %s: %w", path, err)
	}
	return snapshot, nil
}

func writeScoutSnapshotOutput(jsonOutput bool, output scoutSnapshotOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("scout snapshot ok: %s\n", output.SnapshotPath)
		if output.Coverage != nil {
			fmt.Printf("coverage: discovered=%d gated=%d high_risk_ungated=%d coverage_percent=%.2f\n",
				output.Coverage.DiscoveredTools,
				output.Coverage.GatedTools,
				output.Coverage.HighRiskUngatedTools,
				output.Coverage.CoveragePercent,
			)
		}
		return exitCode
	}
	fmt.Printf("scout snapshot error: %s\n", output.Error)
	return exitCode
}

func writeScoutDiffOutput(jsonOutput bool, output scoutDiffOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("scout diff ok: %s vs %s\n", output.Left, output.Right)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("scout diff error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("scout diff changed: %s vs %s\n", output.Left, output.Right)
	return exitCode
}

func writeScoutSignalOutput(jsonOutput bool, output scoutSignalOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("scout signal ok: runs=%d families=%d top_issues=%d\n", output.RunCount, output.FamilyCount, output.TopIssues)
		fmt.Printf("output: %s\n", output.OutputPath)
		if output.Report != nil && len(output.Report.TopIssues) > 0 {
			issue := output.Report.TopIssues[0]
			fmt.Printf(
				"top_issue: family=%s severity=%s score=%d reason=%s\n",
				issue.FamilyID,
				issue.SeverityLevel,
				issue.SeverityScore,
				issue.TopFailureReason,
			)
		}
		return exitCode
	}
	fmt.Printf("scout signal error: %s\n", output.Error)
	return exitCode
}

func printScoutUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait scout snapshot [--roots <csv>] [--policy <csv>] [--out <snapshot.json>] [--coverage-out <coverage.json>] [--json] [--explain]")
	fmt.Println("  gait scout diff <left_snapshot.json> <right_snapshot.json> [--out <diff.json>] [--json] [--explain]")
	fmt.Println("  gait scout signal --runs <csv> [--traces <csv>] [--regress <csv>] [--out <signal_report.json>] [--json] [--explain]")
}

func printScoutSnapshotUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait scout snapshot [--roots <csv>] [--include <csv>] [--exclude <csv>] [--policy <csv>] [--out <snapshot.json>] [--coverage-out <coverage.json>] [--json] [--explain]")
}

func printScoutDiffUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait scout diff <left_snapshot.json> <right_snapshot.json> [--out <diff.json>] [--json] [--explain]")
}

func printScoutSignalUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait scout signal --runs <csv> [--traces <csv>] [--regress <csv>] [--out <signal_report.json>] [--json] [--explain]")
}
