package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Clyra-AI/gait/core/runpack"
)

type migrateOutput struct {
	OK       bool   `json:"ok"`
	Input    string `json:"input,omitempty"`
	Output   string `json:"output,omitempty"`
	Artifact string `json:"artifact,omitempty"`
	Status   string `json:"status,omitempty"`
	RunID    string `json:"run_id,omitempty"`
	Error    string `json:"error,omitempty"`
}

func runMigrate(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Migrate artifact files to the current schema generation without requiring network access.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"input":  true,
		"out":    true,
		"target": true,
	})

	flagSet := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var inputPath string
	var outputPath string
	var target string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&inputPath, "input", "", "artifact path or run_id")
	flagSet.StringVar(&outputPath, "out", "", "output path (default: sibling *_migrated artifact)")
	flagSet.StringVar(&target, "target", "v1", "target schema generation")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeMigrateOutput(jsonOutput, migrateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printMigrateUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(inputPath) == "" && len(remaining) == 1 {
		inputPath = remaining[0]
		remaining = nil
	}
	if len(remaining) > 0 {
		return writeMigrateOutput(jsonOutput, migrateOutput{
			OK:    false,
			Error: "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(target) != "v1" {
		return writeMigrateOutput(jsonOutput, migrateOutput{
			OK:    false,
			Error: "unsupported --target (only v1 is currently available)",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(inputPath) == "" {
		return writeMigrateOutput(jsonOutput, migrateOutput{
			OK:    false,
			Error: "missing required --input <artifact_path|run_id>",
		}, exitInvalidInput)
	}

	resolvedInput, err := resolveRunpackPath(inputPath)
	if err != nil {
		return writeMigrateOutput(jsonOutput, migrateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if !strings.HasSuffix(strings.ToLower(resolvedInput), ".zip") {
		return writeMigrateOutput(jsonOutput, migrateOutput{
			OK:    false,
			Error: "unsupported artifact type (only runpack zip is currently migratable)",
		}, exitInvalidInput)
	}

	runpackData, err := runpack.ReadRunpack(resolvedInput)
	if err != nil {
		return writeMigrateOutput(jsonOutput, migrateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	if strings.TrimSpace(outputPath) == "" {
		outputPath = defaultMigratedRunpackPath(resolvedInput, runpackData.Run.RunID)
	}
	if filepath.Clean(outputPath) == filepath.Clean(resolvedInput) {
		return writeMigrateOutput(jsonOutput, migrateOutput{
			OK:    false,
			Error: "--out must differ from input path",
		}, exitInvalidInput)
	}

	captureMode := strings.TrimSpace(runpackData.Manifest.CaptureMode)
	if captureMode == "" {
		captureMode = "reference"
	}
	result, err := runpack.WriteRunpack(outputPath, runpack.RecordOptions{
		Run:         runpackData.Run,
		Intents:     runpackData.Intents,
		Results:     runpackData.Results,
		Refs:        runpackData.Refs,
		CaptureMode: captureMode,
	})
	if err != nil {
		return writeMigrateOutput(jsonOutput, migrateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	verifyResult, err := runpack.VerifyZip(outputPath, runpack.VerifyOptions{RequireSignature: false})
	if err != nil {
		return writeMigrateOutput(jsonOutput, migrateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 || verifyResult.SignatureStatus == "failed" {
		return writeMigrateOutput(jsonOutput, migrateOutput{
			OK:    false,
			Error: "migrated runpack failed verification",
		}, exitVerifyFailed)
	}

	status := "migrated"
	oldBytes, oldErr := os.ReadFile(resolvedInput)
	newBytes, newErr := os.ReadFile(outputPath)
	if oldErr == nil && newErr == nil && bytes.Equal(oldBytes, newBytes) {
		status = "no_change"
	}

	_ = result
	return writeMigrateOutput(jsonOutput, migrateOutput{
		OK:       true,
		Input:    resolvedInput,
		Output:   outputPath,
		Artifact: "runpack",
		Status:   status,
		RunID:    runpackData.Run.RunID,
	}, exitOK)
}

func defaultMigratedRunpackPath(inputPath, runID string) string {
	base := filepath.Base(inputPath)
	dir := filepath.Dir(inputPath)
	if strings.TrimSpace(runID) != "" {
		return filepath.Join(dir, fmt.Sprintf("runpack_%s_migrated.zip", runID))
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, stem+"_migrated.zip")
}

func writeMigrateOutput(jsonOutput bool, output migrateOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("migrate ok: %s -> %s (%s)\n", output.Input, output.Output, output.Status)
		return exitCode
	}
	fmt.Printf("migrate error: %s\n", output.Error)
	return exitCode
}

func printMigrateUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait migrate --input <artifact_path|run_id> [--out <path>] [--target v1] [--json] [--explain]")
	fmt.Println("  gait migrate <artifact_path|run_id> [--out <path>] [--target v1] [--json] [--explain]")
}
