package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/davidahmann/gait/core/runpack"
)

type replayOutput struct {
	OK              bool                 `json:"ok"`
	RunID           string               `json:"run_id,omitempty"`
	Mode            string               `json:"mode,omitempty"`
	Steps           []runpack.ReplayStep `json:"steps,omitempty"`
	MissingResults  []string             `json:"missing_results,omitempty"`
	Warnings        []string             `json:"warnings,omitempty"`
	Error           string               `json:"error,omitempty"`
	RequestedUnsafe bool                 `json:"requested_unsafe,omitempty"`
}

type diffOutput struct {
	OK      bool                `json:"ok"`
	Privacy string              `json:"privacy,omitempty"`
	Summary runpack.DiffSummary `json:"summary,omitempty"`
	Output  string              `json:"output,omitempty"`
	Error   string              `json:"error,omitempty"`
}

type reduceOutput struct {
	OK           bool                  `json:"ok"`
	Input        string                `json:"input,omitempty"`
	Output       string                `json:"output,omitempty"`
	ReportPath   string                `json:"report_path,omitempty"`
	RunID        string                `json:"run_id,omitempty"`
	ReducedRunID string                `json:"reduced_run_id,omitempty"`
	Predicate    string                `json:"predicate,omitempty"`
	Report       *runpack.ReduceReport `json:"report,omitempty"`
	Error        string                `json:"error,omitempty"`
}

func runCommand(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Work with runpacks: record an artifact from normalized run data, replay deterministically in stub mode, and diff runs with stable output.")
	}
	if len(arguments) == 0 {
		printRunUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "record":
		return runRecord(arguments[1:])
	case "diff":
		return runDiff(arguments[1:])
	case "replay":
		return runReplay(arguments[1:])
	case "reduce":
		return runReduce(arguments[1:])
	default:
		printRunUsage()
		return exitInvalidInput
	}
}

func runDiff(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Compare two runpacks deterministically and optionally write canonical diff JSON.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"privacy": true,
		"output":  true,
	})
	flagSet := flag.NewFlagSet("diff", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var privacy string
	var outputPath string
	var helpFlag bool

	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.StringVar(&privacy, "privacy", "full", "privacy mode: full|metadata")
	flagSet.StringVar(&outputPath, "output", "", "write diff JSON to path")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printDiffUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if len(remaining) != 2 {
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: "expected left and right run_id|path"}, exitInvalidInput)
	}

	leftPath, err := resolveRunpackPath(remaining[0])
	if err != nil {
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	rightPath, err := resolveRunpackPath(remaining[1])
	if err != nil {
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	result, err := runpack.DiffRunpacks(leftPath, rightPath, runpack.DiffPrivacy(privacy))
	if err != nil {
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	ok := !result.Summary.ManifestChanged && !result.Summary.IntentsChanged &&
		!result.Summary.ResultsChanged && !result.Summary.RefsChanged

	output := diffOutput{
		OK:      ok,
		Privacy: string(result.Privacy),
		Summary: result.Summary,
		Output:  outputPath,
	}

	if outputPath != "" {
		if err := writeDiffFile(outputPath, result); err != nil {
			return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
	}

	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writeDiffOutput(jsonOutput, output, exitCode)
}

func writeDiffFile(path string, result runpack.DiffResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o600)
}

func writeDiffOutput(jsonOutput bool, output diffOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("diff ok: %s vs %s\n", output.Summary.RunIDLeft, output.Summary.RunIDRight)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("diff error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("diff changed: %s vs %s\n", output.Summary.RunIDLeft, output.Summary.RunIDRight)
	if len(output.Summary.FilesChanged) > 0 {
		fmt.Printf("files changed: %s\n", strings.Join(output.Summary.FilesChanged, ", "))
	}
	return exitCode
}

func printDiffUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run diff <left> <right> [--privacy=full|metadata] [--output diff.json] [--json] [--explain]")
}

func runReplay(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Replay a runpack deterministically using recorded tool results; real tool execution requires explicit unsafe flags.")
	}
	arguments = reorderInterspersedFlags(arguments, nil)
	flagSet := flag.NewFlagSet("replay", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var realTools bool
	var unsafeReal bool
	var allowToolsCSV string
	var unsafeRealToolsEnv string
	var helpFlag bool

	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&realTools, "real-tools", false, "attempt real tool execution")
	flagSet.BoolVar(&unsafeReal, "unsafe-real-tools", false, "allow real tool execution")
	flagSet.StringVar(&allowToolsCSV, "allow-tools", "", "comma-separated tools explicitly allowed for real replay")
	flagSet.StringVar(&unsafeRealToolsEnv, "unsafe-real-tools-env", "GAIT_ALLOW_REAL_REPLAY", "env var that must be set to 1 for real replay")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeReplayOutput(jsonOutput, replayOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printReplayUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if len(remaining) != 1 {
		return writeReplayOutput(jsonOutput, replayOutput{OK: false, Error: "expected run_id or path"}, exitInvalidInput)
	}

	allowTools := parseCSV(allowToolsCSV)
	if realTools && !unsafeReal {
		return writeReplayOutput(jsonOutput, replayOutput{
			OK:              false,
			Error:           "real tool execution requires --unsafe-real-tools",
			RequestedUnsafe: false,
		}, exitUnsafeReplay)
	}
	if realTools && unsafeReal {
		if len(allowTools) == 0 {
			return writeReplayOutput(jsonOutput, replayOutput{
				OK:              false,
				Error:           "real tool execution requires --allow-tools with explicit tool names",
				RequestedUnsafe: true,
			}, exitUnsafeReplay)
		}
		if strings.TrimSpace(os.Getenv(strings.TrimSpace(unsafeRealToolsEnv))) != "1" {
			return writeReplayOutput(jsonOutput, replayOutput{
				OK:              false,
				Error:           fmt.Sprintf("real tool execution requires %s=1", strings.TrimSpace(unsafeRealToolsEnv)),
				RequestedUnsafe: true,
			}, exitUnsafeReplay)
		}
	}

	runpackPath, err := resolveRunpackPath(remaining[0])
	if err != nil {
		return writeReplayOutput(jsonOutput, replayOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	warnings := []string{}
	if realTools && unsafeReal {
		warnings = append(warnings, "real tools not implemented; replaying stubs")
		warnings = append(warnings, "allow_tools="+strings.Join(allowTools, ","))
	}

	result, err := runpack.ReplayStub(runpackPath)
	if err != nil {
		return writeReplayOutput(jsonOutput, replayOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	ok := len(result.MissingResults) == 0
	output := replayOutput{
		OK:              ok,
		RunID:           result.RunID,
		Mode:            string(result.Mode),
		Steps:           result.Steps,
		MissingResults:  result.MissingResults,
		Warnings:        warnings,
		RequestedUnsafe: unsafeReal,
	}
	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writeReplayOutput(jsonOutput, output, exitCode)
}

func writeReplayOutput(jsonOutput bool, output replayOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}

	if output.OK {
		fmt.Printf("replay ok: %s (%s)\n", output.RunID, output.Mode)
		if len(output.Warnings) > 0 {
			fmt.Printf("warnings: %s\n", strings.Join(output.Warnings, "; "))
		}
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("replay error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("replay failed: %s\n", output.RunID)
	if len(output.MissingResults) > 0 {
		fmt.Printf("missing results: %s\n", strings.Join(output.MissingResults, ", "))
	}
	if len(output.Warnings) > 0 {
		fmt.Printf("warnings: %s\n", strings.Join(output.Warnings, "; "))
	}
	return exitCode
}

func printRunUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run record --input <run_record.json> [--out-dir gait-out] [--run-id <run_id>] [--capture-mode reference|raw] [--json] [--explain]")
	fmt.Println("  gait run diff <left> <right> [--privacy=full|metadata] [--output diff.json] [--json] [--explain]")
	fmt.Println("  gait run replay <run_id|path> [--json] [--real-tools --unsafe-real-tools --allow-tools <csv> --unsafe-real-tools-env <VAR>] [--explain] (stub replay only; real tools not implemented)")
	fmt.Println("  gait run reduce --from <run_id|path> [--predicate missing_result|non_ok_status] [--out reduced.zip] [--report-out reduce_report.json] [--json] [--explain]")
}

func printReplayUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run replay <run_id|path> [--json] [--real-tools --unsafe-real-tools --allow-tools <csv> --unsafe-real-tools-env <VAR>] [--explain]")
	fmt.Println("  note: replay executes deterministic stubs; real tool execution is intentionally not implemented.")
}

func runReduce(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Reduce a runpack to the smallest deterministic artifact that still triggers a selected failure predicate.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"from":       true,
		"predicate":  true,
		"out":        true,
		"report-out": true,
	})
	flagSet := flag.NewFlagSet("reduce", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var from string
	var predicateRaw string
	var outPath string
	var reportOut string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&from, "from", "", "runpack path or run_id")
	flagSet.StringVar(&predicateRaw, "predicate", string(runpack.PredicateMissingResult), "failure predicate: missing_result|non_ok_status")
	flagSet.StringVar(&outPath, "out", "", "output path for minimized runpack")
	flagSet.StringVar(&reportOut, "report-out", "", "output path for reducer report JSON")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeReduceOutput(jsonOutput, reduceOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printReduceUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if from == "" && len(remaining) > 0 {
		from = remaining[0]
		remaining = remaining[1:]
	}
	if from == "" || len(remaining) > 0 {
		return writeReduceOutput(jsonOutput, reduceOutput{OK: false, Error: "expected --from <run_id|path>"}, exitInvalidInput)
	}

	predicate, err := runpack.ParseReducePredicate(predicateRaw)
	if err != nil {
		return writeReduceOutput(jsonOutput, reduceOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	resolvedPath, err := resolveRunpackPath(from)
	if err != nil {
		return writeReduceOutput(jsonOutput, reduceOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	result, err := runpack.ReduceToMinimal(runpack.ReduceOptions{
		InputPath:  resolvedPath,
		OutputPath: outPath,
		Predicate:  predicate,
	})
	if err != nil {
		return writeReduceOutput(jsonOutput, reduceOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if reportOut == "" {
		reportOut = result.OutputPath + ".reduce_report.json"
	}
	encodedReport, err := runpack.EncodeReduceReport(result.Report)
	if err != nil {
		return writeReduceOutput(jsonOutput, reduceOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if err := os.WriteFile(reportOut, encodedReport, 0o600); err != nil {
		return writeReduceOutput(jsonOutput, reduceOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	return writeReduceOutput(jsonOutput, reduceOutput{
		OK:           true,
		Input:        resolvedPath,
		Output:       result.OutputPath,
		ReportPath:   reportOut,
		RunID:        result.RunID,
		ReducedRunID: result.ReducedRunID,
		Predicate:    string(result.Report.Predicate),
		Report:       &result.Report,
	}, exitOK)
}

func writeReduceOutput(jsonOutput bool, output reduceOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("run reduce ok: %s -> %s\n", output.Input, output.Output)
		fmt.Printf("report: %s\n", output.ReportPath)
		return exitCode
	}
	fmt.Printf("run reduce error: %s\n", output.Error)
	return exitCode
}

func printReduceUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run reduce --from <run_id|path> [--predicate missing_result|non_ok_status] [--out reduced.zip] [--report-out reduce_report.json] [--json] [--explain]")
}
