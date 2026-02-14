package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/doctor"
	"github.com/davidahmann/gait/core/scout"
	"github.com/davidahmann/gait/core/sign"
)

type doctorOutput struct {
	OK              bool           `json:"ok"`
	SummaryMode     bool           `json:"summary_mode,omitempty"`
	SchemaID        string         `json:"schema_id,omitempty"`
	SchemaVersion   string         `json:"schema_version,omitempty"`
	CreatedAt       string         `json:"created_at,omitempty"`
	ProducerVersion string         `json:"producer_version,omitempty"`
	Status          string         `json:"status,omitempty"`
	NonFixable      bool           `json:"non_fixable,omitempty"`
	Summary         string         `json:"summary,omitempty"`
	FixCommands     []string       `json:"fix_commands,omitempty"`
	Checks          []doctor.Check `json:"checks,omitempty"`
	Error           string         `json:"error,omitempty"`
}

type doctorAdoptionOutput struct {
	OK     bool                  `json:"ok"`
	Report *scout.AdoptionReport `json:"report,omitempty"`
	Error  string                `json:"error,omitempty"`
}

func runDoctor(arguments []string) int {
	if len(arguments) > 0 && strings.TrimSpace(arguments[0]) == "adoption" {
		return runDoctorAdoption(arguments[1:])
	}
	if hasExplainFlag(arguments) {
		return writeExplain("Diagnose local environment and onboarding issues for Gait workflows and return stable fix suggestions.")
	}
	flagSet := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var workDir string
	var outputDir string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var publicKeyPath string
	var publicKeyEnv string
	var productionReadiness bool
	var summaryMode bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&workDir, "workdir", ".", "workspace path for checks")
	flagSet.StringVar(&outputDir, "output-dir", "./gait-out", "default output directory to check")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "key mode to validate: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.BoolVar(&productionReadiness, "production-readiness", false, "run strict production readiness checks (fails on unsafe configuration)")
	flagSet.BoolVar(&summaryMode, "summary", false, "emit concise summary output")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeDoctorOutput(jsonOutput, doctorOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printDoctorUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeDoctorOutput(jsonOutput, doctorOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	result := doctor.Run(doctor.Options{
		WorkDir:         workDir,
		OutputDir:       outputDir,
		ProducerVersion: version,
		KeyMode:         sign.KeyMode(strings.ToLower(strings.TrimSpace(keyMode))),
		KeyConfig: sign.KeyConfig{
			PrivateKeyPath: privateKeyPath,
			PrivateKeyEnv:  privateKeyEnv,
			PublicKeyPath:  publicKeyPath,
			PublicKeyEnv:   publicKeyEnv,
		},
		ProductionReadiness: productionReadiness,
	})

	exitCode := exitOK
	ok := !result.NonFixable
	if result.NonFixable {
		exitCode = exitMissingDependency
	}
	if productionReadiness && result.Status == "fail" {
		exitCode = exitVerifyFailed
		ok = false
	}
	return writeDoctorOutput(jsonOutput, doctorOutput{
		OK:              ok,
		SummaryMode:     summaryMode,
		SchemaID:        result.SchemaID,
		SchemaVersion:   result.SchemaVersion,
		CreatedAt:       result.CreatedAt,
		ProducerVersion: result.ProducerVersion,
		Status:          result.Status,
		NonFixable:      result.NonFixable,
		Summary:         result.Summary,
		FixCommands:     result.FixCommands,
		Checks:          result.Checks,
	}, exitCode)
}

func runDoctorAdoption(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Summarize local activation milestones and blockers from GAIT_ADOPTION_LOG events.")
	}
	flagSet := flag.NewFlagSet("doctor-adoption", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var fromPath string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&fromPath, "from", "", "path to adoption events jsonl")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeDoctorAdoptionOutput(jsonOutput, doctorAdoptionOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printDoctorAdoptionUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeDoctorAdoptionOutput(jsonOutput, doctorAdoptionOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(fromPath) == "" {
		return writeDoctorAdoptionOutput(jsonOutput, doctorAdoptionOutput{OK: false, Error: "missing required --from <path>"}, exitInvalidInput)
	}

	events, err := scout.LoadAdoptionEvents(fromPath)
	if err != nil {
		return writeDoctorAdoptionOutput(jsonOutput, doctorAdoptionOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	report := scout.BuildAdoptionReport(events, fromPath, version, time.Time{})
	return writeDoctorAdoptionOutput(jsonOutput, doctorAdoptionOutput{
		OK:     true,
		Report: &report,
	}, exitOK)
}

func writeDoctorOutput(jsonOutput bool, output doctorOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}

	if output.Error != "" {
		fmt.Printf("doctor error: %s\n", output.Error)
		return exitCode
	}
	fmt.Println(output.Summary)
	if output.SummaryMode {
		if output.Status == "pass" && !output.NonFixable {
			fmt.Printf("tip: %s\n", demoMetricsOptInCommand)
			fmt.Println("tip: gait doctor adoption --from ./gait-out/adoption.jsonl --json")
			return exitCode
		}
		for _, check := range output.Checks {
			if check.Status == "pass" {
				continue
			}
			fmt.Printf("- %s: %s (%s)\n", check.Name, check.Status, check.Message)
			if check.FixCommand != "" {
				fmt.Printf("  fix: %s\n", check.FixCommand)
			}
		}
		return exitCode
	}
	for _, check := range output.Checks {
		fmt.Printf("- %s: %s (%s)\n", check.Name, check.Status, check.Message)
		if check.FixCommand != "" {
			fmt.Printf("  fix: %s\n", check.FixCommand)
		}
	}
	return exitCode
}

func writeDoctorAdoptionOutput(jsonOutput bool, output doctorAdoptionOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}

	if output.Error != "" {
		fmt.Printf("doctor adoption error: %s\n", output.Error)
		return exitCode
	}
	if output.Report == nil {
		fmt.Println("doctor adoption error: missing report")
		return exitInvalidInput
	}
	fmt.Printf("doctor adoption: source=%s total_events=%d activation_complete=%t\n", output.Report.Source, output.Report.TotalEvents, output.Report.ActivationComplete)
	if len(output.Report.Blockers) > 0 {
		fmt.Printf("blockers: %s\n", strings.Join(output.Report.Blockers, "; "))
	}
	return exitCode
}

func printDoctorUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait doctor [--workdir <path>] [--output-dir <path>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--public-key <path>|--public-key-env <VAR>] [--production-readiness] [--summary] [--json] [--explain]")
	fmt.Println("  gait doctor adoption --from <events.jsonl> [--json] [--explain]")
}

func printDoctorAdoptionUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait doctor adoption --from <events.jsonl> [--json] [--explain]")
}
