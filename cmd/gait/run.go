package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

func runCommand(arguments []string) int {
	if len(arguments) == 0 {
		printRunUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "replay":
		return runReplay(arguments[1:])
	default:
		printRunUsage()
		return exitInvalidInput
	}
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
