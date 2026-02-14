package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/davidahmann/gait/core/regress"
	"github.com/davidahmann/gait/core/runpack"
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
	OK               bool     `json:"ok"`
	Status           string   `json:"status,omitempty"`
	FixtureSet       string   `json:"fixture_set,omitempty"`
	Graders          int      `json:"graders,omitempty"`
	Failed           int      `json:"failed,omitempty"`
	TopFailureReason string   `json:"top_failure_reason,omitempty"`
	NextCommand      string   `json:"next_command,omitempty"`
	ArtifactPaths    []string `json:"artifact_paths,omitempty"`
	Output           string   `json:"output,omitempty"`
	JUnit            string   `json:"junit,omitempty"`
	Error            string   `json:"error,omitempty"`
}

type regressBootstrapOutput struct {
	OK               bool     `json:"ok"`
	RunID            string   `json:"run_id,omitempty"`
	FixtureName      string   `json:"fixture_name,omitempty"`
	FixtureDir       string   `json:"fixture_dir,omitempty"`
	RunpackPath      string   `json:"runpack_path,omitempty"`
	ConfigPath       string   `json:"config_path,omitempty"`
	Status           string   `json:"status,omitempty"`
	Failed           int      `json:"failed,omitempty"`
	TopFailureReason string   `json:"top_failure_reason,omitempty"`
	NextCommand      string   `json:"next_command,omitempty"`
	ArtifactPaths    []string `json:"artifact_paths,omitempty"`
	Output           string   `json:"output,omitempty"`
	JUnit            string   `json:"junit,omitempty"`
	Error            string   `json:"error,omitempty"`
}

func runRegress(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Turn runpacks into deterministic regression fixtures and execute graders for CI-safe drift detection.")
	}
	if len(arguments) == 0 {
		printRegressUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "init":
		return runRegressInit(arguments[1:])
	case "run":
		return runRegressRun(arguments[1:])
	case "bootstrap":
		return runRegressBootstrap(arguments[1:])
	default:
		printRegressUsage()
		return exitInvalidInput
	}
}

func runRegressInit(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Initialize a regression fixture from a recorded runpack artifact.")
	}
	flagSet := flag.NewFlagSet("regress-init", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var from string
	var name string
	var checkpointRef string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&from, "from", "", "run_id or path")
	flagSet.StringVar(&name, "name", "", "fixture name override")
	flagSet.StringVar(&checkpointRef, "checkpoint", "latest", "checkpoint index or latest when --from is session_chain.json")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRegressInitOutput(jsonOutput, regressInitOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
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

	runpackPath, sessionChainPath, err := resolveRegressSource(from, checkpointRef)
	if err != nil {
		return writeRegressInitOutput(jsonOutput, regressInitOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	result, err := regress.InitFixture(regress.InitOptions{
		SourceRunpackPath: runpackPath,
		SessionChainPath:  sessionChainPath,
		CheckpointRef:     checkpointRef,
		FixtureName:       name,
		WorkDir:           ".",
	})
	if err != nil {
		return writeRegressInitOutput(jsonOutput, regressInitOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
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
		return writeJSONOutput(output, exitCode)
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
	if hasExplainFlag(arguments) {
		return writeExplain("Execute configured regress fixtures and graders with stable JSON output and exit codes.")
	}
	flagSet := flag.NewFlagSet("regress-run", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var configPath string
	var outputPath string
	var junitPath string
	var jsonOutput bool
	var allowNondeterministic bool
	var contextConformance bool
	var allowContextRuntimeDrift bool
	var helpFlag bool

	flagSet.StringVar(&configPath, "config", "gait.yaml", "path to regress config")
	flagSet.StringVar(&outputPath, "output", "regress_result.json", "path to result JSON")
	flagSet.StringVar(&junitPath, "junit", "", "path to optional junit xml report")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&allowNondeterministic, "allow-nondeterministic", false, "allow non-deterministic graders")
	flagSet.BoolVar(&contextConformance, "context-conformance", false, "enforce context conformance grader for all fixtures")
	flagSet.BoolVar(&allowContextRuntimeDrift, "allow-context-runtime-drift", false, "allow runtime-only context drift when context conformance is enforced")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRegressRunOutput(jsonOutput, regressRunOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
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
		ConfigPath:               configPath,
		OutputPath:               outputPath,
		JUnitPath:                junitPath,
		WorkDir:                  ".",
		ProducerVersion:          version,
		AllowNondeterministic:    allowNondeterministic,
		ContextConformance:       contextConformance,
		AllowContextRuntimeDrift: allowContextRuntimeDrift,
	})
	if err != nil {
		return writeRegressRunOutput(jsonOutput, regressRunOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	ok := result.Result.Status == regressStatusPass
	exitCode := exitOK
	if !ok {
		exitCode = exitRegressFailed
	}
	topFailureReason := firstFailureReason(result)
	nextCommand := ""
	if !ok {
		nextCommand = regressNextCommand(configPath)
	}
	artifactPaths := regressArtifactPaths(result.OutputPath, result.JUnitPath)

	return writeRegressRunOutput(jsonOutput, regressRunOutput{
		OK:               ok,
		Status:           result.Result.Status,
		FixtureSet:       result.Result.FixtureSet,
		Graders:          len(result.Result.Graders),
		Failed:           result.FailedGraders,
		TopFailureReason: topFailureReason,
		NextCommand:      nextCommand,
		ArtifactPaths:    artifactPaths,
		Output:           result.OutputPath,
		JUnit:            result.JUnitPath,
	}, exitCode)
}

func runRegressBootstrap(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Initialize a fixture from a runpack and execute regress in one command for incident-to-regression bootstrap.")
	}
	flagSet := flag.NewFlagSet("regress-bootstrap", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var from string
	var name string
	var checkpointRef string
	var configPath string
	var outputPath string
	var junitPath string
	var jsonOutput bool
	var allowNondeterministic bool
	var contextConformance bool
	var allowContextRuntimeDrift bool
	var helpFlag bool

	flagSet.StringVar(&from, "from", "", "run_id or path")
	flagSet.StringVar(&name, "name", "", "fixture name override")
	flagSet.StringVar(&checkpointRef, "checkpoint", "latest", "checkpoint index or latest when --from is session_chain.json")
	flagSet.StringVar(&configPath, "config", "gait.yaml", "path to regress config")
	flagSet.StringVar(&outputPath, "output", "regress_result.json", "path to result JSON")
	flagSet.StringVar(&junitPath, "junit", "", "path to optional junit xml report")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&allowNondeterministic, "allow-nondeterministic", false, "allow non-deterministic graders")
	flagSet.BoolVar(&contextConformance, "context-conformance", false, "enforce context conformance grader for all fixtures")
	flagSet.BoolVar(&allowContextRuntimeDrift, "allow-context-runtime-drift", false, "allow runtime-only context drift when context conformance is enforced")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRegressBootstrapOutput(jsonOutput, regressBootstrapOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRegressBootstrapUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRegressBootstrapOutput(jsonOutput, regressBootstrapOutput{
			OK:    false,
			Error: "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(from) == "" {
		return writeRegressBootstrapOutput(jsonOutput, regressBootstrapOutput{
			OK:    false,
			Error: "missing required --from <run_id|path>",
		}, exitInvalidInput)
	}

	runpackPath, sessionChainPath, err := resolveRegressSource(from, checkpointRef)
	if err != nil {
		return writeRegressBootstrapOutput(jsonOutput, regressBootstrapOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	initResult, err := regress.InitFixture(regress.InitOptions{
		SourceRunpackPath: runpackPath,
		SessionChainPath:  sessionChainPath,
		CheckpointRef:     checkpointRef,
		FixtureName:       name,
		WorkDir:           ".",
	})
	if err != nil {
		return writeRegressBootstrapOutput(jsonOutput, regressBootstrapOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	runResult, err := regress.Run(regress.RunOptions{
		ConfigPath:               configPath,
		OutputPath:               outputPath,
		JUnitPath:                junitPath,
		WorkDir:                  ".",
		ProducerVersion:          version,
		AllowNondeterministic:    allowNondeterministic,
		ContextConformance:       contextConformance,
		AllowContextRuntimeDrift: allowContextRuntimeDrift,
	})
	if err != nil {
		return writeRegressBootstrapOutput(jsonOutput, regressBootstrapOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	ok := runResult.Result.Status == regressStatusPass
	exitCode := exitOK
	if !ok {
		exitCode = exitRegressFailed
	}
	topFailureReason := firstFailureReason(runResult)
	nextCommand := ""
	if !ok {
		nextCommand = regressNextCommand(configPath)
	}
	artifactPaths := regressArtifactPaths(runResult.OutputPath, runResult.JUnitPath)

	return writeRegressBootstrapOutput(jsonOutput, regressBootstrapOutput{
		OK:               ok,
		RunID:            initResult.RunID,
		FixtureName:      initResult.FixtureName,
		FixtureDir:       initResult.FixtureDir,
		RunpackPath:      initResult.RunpackPath,
		ConfigPath:       initResult.ConfigPath,
		Status:           runResult.Result.Status,
		Failed:           runResult.FailedGraders,
		TopFailureReason: topFailureReason,
		NextCommand:      nextCommand,
		ArtifactPaths:    artifactPaths,
		Output:           runResult.OutputPath,
		JUnit:            runResult.JUnitPath,
	}, exitCode)
}

func writeRegressRunOutput(jsonOutput bool, output regressRunOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}

	if output.OK {
		fmt.Printf("regress run ok: fixture_set=%s graders=%d\n", output.FixtureSet, output.Graders)
		fmt.Printf("output: %s\n", output.Output)
		if output.JUnit != "" {
			fmt.Printf("junit: %s\n", output.JUnit)
		}
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("regress run error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("regress run failed: fixture_set=%s failed=%d\n", output.FixtureSet, output.Failed)
	if output.TopFailureReason != "" {
		fmt.Printf("top_failure_reason: %s\n", output.TopFailureReason)
	}
	if output.NextCommand != "" {
		fmt.Printf("next: %s\n", output.NextCommand)
	}
	if len(output.ArtifactPaths) > 0 {
		fmt.Printf("artifacts: %s\n", strings.Join(output.ArtifactPaths, ", "))
	}
	fmt.Printf("output: %s\n", output.Output)
	if output.JUnit != "" {
		fmt.Printf("junit: %s\n", output.JUnit)
	}
	return exitCode
}

func writeRegressBootstrapOutput(jsonOutput bool, output regressBootstrapOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}

	if output.OK {
		fmt.Printf("regress bootstrap ok: run_id=%s fixture=%s\n", output.RunID, output.FixtureName)
		fmt.Printf("output: %s\n", output.Output)
		if output.JUnit != "" {
			fmt.Printf("junit: %s\n", output.JUnit)
		}
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("regress bootstrap error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("regress bootstrap failed: run_id=%s fixture=%s failed=%d\n", output.RunID, output.FixtureName, output.Failed)
	if output.TopFailureReason != "" {
		fmt.Printf("top_failure_reason: %s\n", output.TopFailureReason)
	}
	if output.NextCommand != "" {
		fmt.Printf("next: %s\n", output.NextCommand)
	}
	if len(output.ArtifactPaths) > 0 {
		fmt.Printf("artifacts: %s\n", strings.Join(output.ArtifactPaths, ", "))
	}
	return exitCode
}

func firstFailureReason(result regress.RunResult) string {
	for _, grader := range result.Result.Graders {
		if grader.Status == regressStatusPass {
			continue
		}
		if len(grader.ReasonCodes) > 0 {
			return grader.ReasonCodes[0]
		}
		return "grader_failed"
	}
	return ""
}

func regressNextCommand(configPath string) string {
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" || trimmed == "gait.yaml" {
		return "gait regress run --json"
	}
	return fmt.Sprintf("gait regress run --config %s --json", trimmed)
}

func regressArtifactPaths(outputPath string, junitPath string) []string {
	paths := []string{}
	if strings.TrimSpace(outputPath) != "" {
		paths = append(paths, strings.TrimSpace(outputPath))
	}
	if strings.TrimSpace(junitPath) != "" {
		paths = append(paths, strings.TrimSpace(junitPath))
	}
	return paths
}

func resolveRegressSource(from string, checkpointRef string) (string, string, error) {
	resolvedPath, err := resolveRunpackPath(from)
	if err != nil {
		return "", "", err
	}
	trimmed := strings.TrimSpace(resolvedPath)
	if strings.HasSuffix(strings.ToLower(trimmed), ".json") {
		checkpoint, checkpointErr := runpack.ResolveSessionCheckpointRunpack(trimmed, strings.TrimSpace(checkpointRef))
		if checkpointErr == nil {
			return checkpoint.RunpackPath, trimmed, nil
		}
	}
	return trimmed, "", nil
}

func printRegressUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait regress init --from <run_id|path|session_chain.json> [--checkpoint latest|<index>] [--name <fixture_name>] [--json] [--explain]")
	fmt.Println("  gait regress run [--config gait.yaml] [--output regress_result.json] [--junit junit.xml] [--json] [--allow-nondeterministic] [--context-conformance] [--allow-context-runtime-drift] [--explain]")
	fmt.Println("  gait regress bootstrap --from <run_id|path|session_chain.json> [--checkpoint latest|<index>] [--name <fixture_name>] [--config gait.yaml] [--output regress_result.json] [--junit junit.xml] [--json] [--allow-nondeterministic] [--context-conformance] [--allow-context-runtime-drift] [--explain]")
}

func printRegressInitUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait regress init --from <run_id|path|session_chain.json> [--checkpoint latest|<index>] [--name <fixture_name>] [--json] [--explain]")
}

func printRegressRunUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait regress run [--config gait.yaml] [--output regress_result.json] [--junit junit.xml] [--json] [--allow-nondeterministic] [--context-conformance] [--allow-context-runtime-drift] [--explain]")
}

func printRegressBootstrapUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait regress bootstrap --from <run_id|path|session_chain.json> [--checkpoint latest|<index>] [--name <fixture_name>] [--config gait.yaml] [--output regress_result.json] [--junit junit.xml] [--json] [--allow-nondeterministic] [--context-conformance] [--allow-context-runtime-drift] [--explain]")
}
