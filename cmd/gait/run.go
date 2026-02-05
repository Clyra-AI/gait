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

func runCommand(arguments []string) int {
	if len(arguments) == 0 {
		printRunUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "diff":
		return runDiff(arguments[1:])
	case "replay":
		return runReplay(arguments[1:])
	default:
		printRunUsage()
		return exitInvalidInput
	}
}

func runDiff(arguments []string) int {
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
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitInvalidInput)
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
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	rightPath, err := resolveRunpackPath(remaining[1])
	if err != nil {
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	result, err := runpack.DiffRunpacks(leftPath, rightPath, runpack.DiffPrivacy(privacy))
	if err != nil {
		return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitInvalidInput)
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
			return writeDiffOutput(jsonOutput, diffOutput{OK: false, Error: err.Error()}, exitInvalidInput)
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
		encoded, err := json.Marshal(output)
		if err != nil {
			fmt.Println(`{"ok":false,"error":"failed to encode output"}`)
			return exitInvalidInput
		}
		fmt.Println(string(encoded))
		return exitCode
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
	fmt.Println("  gait run diff <left> <right> [--privacy=full|metadata] [--output diff.json] [--json]")
}

func runReplay(arguments []string) int {
	flagSet := flag.NewFlagSet("replay", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var realTools bool
	var unsafeReal bool
	var helpFlag bool

	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&realTools, "real-tools", false, "attempt real tool execution")
	flagSet.BoolVar(&unsafeReal, "unsafe-real-tools", false, "allow real tool execution")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeReplayOutput(jsonOutput, replayOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if helpFlag {
		printReplayUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if len(remaining) != 1 {
		return writeReplayOutput(jsonOutput, replayOutput{OK: false, Error: "expected run_id or path"}, exitInvalidInput)
	}

	if realTools && !unsafeReal {
		return writeReplayOutput(jsonOutput, replayOutput{
			OK:              false,
			Error:           "real tool execution requires --unsafe-real-tools",
			RequestedUnsafe: false,
		}, exitUnsafeReplay)
	}

	runpackPath, err := resolveRunpackPath(remaining[0])
	if err != nil {
		return writeReplayOutput(jsonOutput, replayOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	warnings := []string{}
	if realTools && unsafeReal {
		warnings = append(warnings, "real tools not implemented; replaying stubs")
	}

	result, err := runpack.ReplayStub(runpackPath)
	if err != nil {
		return writeReplayOutput(jsonOutput, replayOutput{OK: false, Error: err.Error()}, exitInvalidInput)
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
		encoded, err := json.Marshal(output)
		if err != nil {
			fmt.Println(`{"ok":false,"error":"failed to encode output"}`)
			return exitInvalidInput
		}
		fmt.Println(string(encoded))
		return exitCode
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
	fmt.Println("  gait run replay <run_id|path> [--json]")
}

func printReplayUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run replay <run_id|path> [--json] [--real-tools --unsafe-real-tools]")
}
