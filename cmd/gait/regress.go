package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/davidahmann/gait/core/regress"
)

const regressStatusPass = "pass"

type regressInitOutput struct {
	OK           bool     `json:"ok"`
	RunID        string   `json:"run_id,omitempty"`
	FixtureName  string   `json:"fixture_name,omitempty"`
	FixtureDir   string   `json:"fixture_dir,omitempty"`
	RunpackPath  string   `json:"runpack_path,omitempty"`
	ConfigPath   string   `json:"config_path,omitempty"`
	NextCommands []string `json:"next_commands,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type regressRunOutput struct {
	OK         bool   `json:"ok"`
	Status     string `json:"status,omitempty"`
	FixtureSet string `json:"fixture_set,omitempty"`
	Graders    int    `json:"graders,omitempty"`
	Failed     int    `json:"failed,omitempty"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

func runRegress(arguments []string) int {
	if len(arguments) == 0 {
		printRegressUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "init":
		return runRegressInit(arguments[1:])
	case "run":
		return runRegressRun(arguments[1:])
	default:
		printRegressUsage()
		return exitInvalidInput
	}
}

func runRegressInit(arguments []string) int {
	flagSet := flag.NewFlagSet("regress-init", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var from string
	var name string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&from, "from", "", "run_id or path")
	flagSet.StringVar(&name, "name", "", "fixture name override")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRegressInitOutput(jsonOutput, regressInitOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if helpFlag {
		printRegressInitUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRegressInitOutput(jsonOutput, regressInitOutput{
			OK:    false,
			Error: "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(from) == "" {
		return writeRegressInitOutput(jsonOutput, regressInitOutput{
			OK:    false,
			Error: "missing required --from <run_id|path>",
		}, exitInvalidInput)
	}

	runpackPath, err := resolveRunpackPath(from)
	if err != nil {
		return writeRegressInitOutput(jsonOutput, regressInitOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	result, err := regress.InitFixture(regress.InitOptions{
		SourceRunpackPath: runpackPath,
		FixtureName:       name,
		WorkDir:           ".",
	})
	if err != nil {
		return writeRegressInitOutput(jsonOutput, regressInitOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	return writeRegressInitOutput(jsonOutput, regressInitOutput{
		OK:           true,
		RunID:        result.RunID,
		FixtureName:  result.FixtureName,
		FixtureDir:   result.FixtureDir,
		RunpackPath:  result.RunpackPath,
		ConfigPath:   result.ConfigPath,
		NextCommands: result.NextCommands,
	}, exitOK)
}

func writeRegressInitOutput(jsonOutput bool, output regressInitOutput, exitCode int) int {
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
		fmt.Printf("regress init ok: fixture=%s run_id=%s\n", output.FixtureName, output.RunID)
		fmt.Printf("config: %s\n", output.ConfigPath)
		fmt.Printf("runpack: %s\n", output.RunpackPath)
		if len(output.NextCommands) > 0 {
			fmt.Printf("next: %s\n", strings.Join(output.NextCommands, " | "))
		}
		return exitCode
	}
	fmt.Printf("regress init error: %s\n", output.Error)
	return exitCode
}

func runRegressRun(arguments []string) int {
	flagSet := flag.NewFlagSet("regress-run", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var configPath string
	var outputPath string
	var jsonOutput bool
	var allowNondeterministic bool
	var helpFlag bool

	flagSet.StringVar(&configPath, "config", "gait.yaml", "path to regress config")
	flagSet.StringVar(&outputPath, "output", "regress_result.json", "path to result JSON")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&allowNondeterministic, "allow-nondeterministic", false, "allow non-deterministic graders")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRegressRunOutput(jsonOutput, regressRunOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if helpFlag {
		printRegressRunUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRegressRunOutput(jsonOutput, regressRunOutput{
			OK:    false,
			Error: "unexpected positional arguments",
		}, exitInvalidInput)
	}

	result, err := regress.Run(regress.RunOptions{
		ConfigPath:            configPath,
		OutputPath:            outputPath,
		WorkDir:               ".",
		ProducerVersion:       version,
		AllowNondeterministic: allowNondeterministic,
	})
	if err != nil {
		return writeRegressRunOutput(jsonOutput, regressRunOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	exitCode := exitOK
	ok := result.Result.Status == regressStatusPass
	if !ok {
		exitCode = exitRegressFailed
	}

	return writeRegressRunOutput(jsonOutput, regressRunOutput{
		OK:         ok,
		Status:     result.Result.Status,
		FixtureSet: result.Result.FixtureSet,
		Graders:    len(result.Result.Graders),
		Failed:     result.FailedGraders,
		Output:     result.OutputPath,
	}, exitCode)
}

func writeRegressRunOutput(jsonOutput bool, output regressRunOutput, exitCode int) int {
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
		fmt.Printf("regress run ok: fixture_set=%s graders=%d\n", output.FixtureSet, output.Graders)
		fmt.Printf("output: %s\n", output.Output)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("regress run error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("regress run failed: fixture_set=%s failed=%d\n", output.FixtureSet, output.Failed)
	fmt.Printf("output: %s\n", output.Output)
	return exitCode
}

func printRegressUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait regress init --from <run_id|path> [--name <fixture_name>] [--json]")
	fmt.Println("  gait regress run [--config gait.yaml] [--output regress_result.json] [--json]")
}

func printRegressInitUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait regress init --from <run_id|path> [--name <fixture_name>] [--json]")
}

func printRegressRunUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait regress run [--config gait.yaml] [--output regress_result.json] [--json] [--allow-nondeterministic]")
}
