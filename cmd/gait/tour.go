package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/regress"
)

type tourOutput struct {
	OK            bool     `json:"ok"`
	Mode          string   `json:"mode,omitempty"`
	RunID         string   `json:"run_id,omitempty"`
	VerifyPath    string   `json:"verify_path,omitempty"`
	VerifyStatus  string   `json:"verify_status,omitempty"`
	FixtureName   string   `json:"fixture_name,omitempty"`
	FixtureDir    string   `json:"fixture_dir,omitempty"`
	RegressStatus string   `json:"regress_status,omitempty"`
	RegressFailed int      `json:"regress_failed,omitempty"`
	NextCommands  []string `json:"next_commands,omitempty"`
	BranchHints   []string `json:"branch_hints,omitempty"`
	MetricsOptIn  string   `json:"metrics_opt_in,omitempty"`
	DurationMS    int64    `json:"duration_ms,omitempty"`
	Error         string   `json:"error,omitempty"`
}

func runTour(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Run the guided activation walkthrough (demo -> verify -> regress init -> regress run) fully offline.")
	}
	arguments = reorderInterspersedFlags(arguments, nil)

	flagSet := flag.NewFlagSet("tour", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var helpFlag bool
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeTourOutput(jsonOutput, tourOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printTourUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeTourOutput(jsonOutput, tourOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	startedAt := time.Now()

	demoResult, demoExit := executeDemo(demoModeStandard)
	if demoExit != exitOK {
		if demoResult.Error != "" {
			return writeTourOutput(jsonOutput, tourOutput{OK: false, Error: demoResult.Error}, demoExit)
		}
		return writeTourOutput(jsonOutput, tourOutput{OK: false, Error: "demo step failed"}, demoExit)
	}

	verifyResult, err := verifyRunpackArtifact(demoRunID, false, nil)
	if err != nil {
		return writeTourOutput(jsonOutput, tourOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if !verifyResult.OK {
		return writeTourOutput(jsonOutput, tourOutput{
			OK:           false,
			RunID:        demoResult.RunID,
			VerifyPath:   verifyResult.Path,
			VerifyStatus: "failed",
			Error:        "verify step failed",
		}, exitVerifyFailed)
	}

	runpackPath, err := resolveRunpackPath(demoRunID)
	if err != nil {
		return writeTourOutput(jsonOutput, tourOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	initResult, err := regress.InitFixture(regress.InitOptions{
		SourceRunpackPath: runpackPath,
		FixtureName:       "",
		WorkDir:           ".",
	})
	if err != nil {
		return writeTourOutput(jsonOutput, tourOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	regressOutputPath := filepath.Join(".", demoOutDir, "tour_regress_result.json")
	regressJUnitPath := filepath.Join(".", demoOutDir, "tour_junit.xml")
	runResult, err := regress.Run(regress.RunOptions{
		ConfigPath:      initResult.ConfigPath,
		OutputPath:      regressOutputPath,
		JUnitPath:       regressJUnitPath,
		WorkDir:         ".",
		ProducerVersion: version,
	})
	if err != nil {
		return writeTourOutput(jsonOutput, tourOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	ok := runResult.Result.Status == regressStatusPass
	exitCode := exitOK
	failureError := ""
	if !ok {
		exitCode = exitRegressFailed
		failureError = fmt.Sprintf("regress step failed: status=%s failed=%d", runResult.Result.Status, runResult.FailedGraders)
	}

	return writeTourOutput(jsonOutput, tourOutput{
		OK:            ok,
		Mode:          "activation",
		RunID:         demoResult.RunID,
		VerifyPath:    verifyResult.Path,
		VerifyStatus:  "ok",
		FixtureName:   initResult.FixtureName,
		FixtureDir:    initResult.FixtureDir,
		RegressStatus: runResult.Result.Status,
		RegressFailed: runResult.FailedGraders,
		NextCommands: []string{
			"gait demo --durable",
			"gait demo --policy",
			"gait doctor --summary",
		},
		BranchHints: []string{
			"durable branch: run a resumable job lifecycle demo with checkpoints",
			"policy branch: run a deterministic high-risk block demo before enforce rollout",
		},
		MetricsOptIn: demoMetricsOptInCommand,
		DurationMS:   time.Since(startedAt).Milliseconds(),
		Error:        failureError,
	}, exitCode)
}

func writeTourOutput(jsonOutput bool, output tourOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("tour mode=%s\n", output.Mode)
		fmt.Printf("a1_demo=ok run_id=%s\n", output.RunID)
		fmt.Printf("a2_verify=%s path=%s\n", output.VerifyStatus, output.VerifyPath)
		fmt.Printf("a3_regress_init=ok fixture=%s dir=%s\n", output.FixtureName, output.FixtureDir)
		fmt.Printf("a4_regress_run=%s failed=%d\n", output.RegressStatus, output.RegressFailed)
		if len(output.NextCommands) > 0 {
			fmt.Printf("next=%s\n", strings.Join(output.NextCommands, " | "))
		}
		if len(output.BranchHints) > 0 {
			fmt.Printf("branch_hints=%s\n", strings.Join(output.BranchHints, " | "))
		}
		if output.MetricsOptIn != "" {
			fmt.Printf("metrics_opt_in=%s\n", output.MetricsOptIn)
		}
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("tour error: %s\n", output.Error)
	} else {
		fmt.Println("tour failed")
	}
	if output.RunID != "" {
		fmt.Printf("run_id=%s\n", output.RunID)
	}
	if output.VerifyStatus != "" || output.VerifyPath != "" {
		fmt.Printf("a2_verify=%s path=%s\n", output.VerifyStatus, output.VerifyPath)
	}
	if output.FixtureName != "" || output.FixtureDir != "" {
		fmt.Printf("a3_regress_init=fixture=%s dir=%s\n", output.FixtureName, output.FixtureDir)
	}
	if output.RegressStatus != "" || output.RegressFailed > 0 {
		fmt.Printf("a4_regress_run=%s failed=%d\n", output.RegressStatus, output.RegressFailed)
	}
	return exitCode
}

func printTourUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait tour [--json] [--explain]")
}
