package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type wrapperVerdictCount struct {
	Verdict string `json:"verdict"`
	Count   int    `json:"count"`
}

type wrapperOutput struct {
	OK            bool                  `json:"ok"`
	Mode          string                `json:"mode,omitempty"`
	Command       []string              `json:"command,omitempty"`
	Cwd           string                `json:"cwd,omitempty"`
	ChildExitCode int                   `json:"child_exit_code,omitempty"`
	TimedOut      bool                  `json:"timed_out,omitempty"`
	DurationMS    int64                 `json:"duration_ms,omitempty"`
	VerdictCounts []wrapperVerdictCount `json:"verdict_counts,omitempty"`
	TracePaths    []string              `json:"trace_paths,omitempty"`
	RunpackPaths  []string              `json:"runpack_paths,omitempty"`
	Stdout        string                `json:"stdout,omitempty"`
	Stderr        string                `json:"stderr,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
	Error         string                `json:"error,omitempty"`
}

func runTest(arguments []string) int {
	return runWrapperMode("test", arguments)
}

func runEnforce(arguments []string) int {
	return runWrapperMode("enforce", arguments)
}

func runWrapperMode(mode string, arguments []string) int {
	if hasExplainFlag(arguments) {
		if mode == "test" {
			return writeExplain("Run an explicit Gait-aware integration in observe mode, capture child stdout/stderr, and summarize emitted trace verdicts without changing existing policy contracts.")
		}
		return writeExplain("Run an explicit Gait-aware integration in enforce mode and convert emitted non-allow trace verdicts into stable wrapper exit codes.")
	}

	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"cwd":     true,
		"timeout": true,
	})

	flagSet := flag.NewFlagSet(mode, flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var cwd string
	var timeoutText string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&cwd, "cwd", ".", "working directory for child command")
	flagSet.StringVar(&timeoutText, "timeout", "30s", "child process timeout")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeWrapperOutput(jsonOutput, wrapperOutput{OK: false, Mode: mode, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printWrapperUsage(mode)
		return exitOK
	}
	command := flagSet.Args()
	if len(command) == 0 {
		return writeWrapperOutput(jsonOutput, wrapperOutput{
			OK:    false,
			Mode:  mode,
			Error: "expected child command after --",
		}, exitInvalidInput)
	}

	timeout, err := time.ParseDuration(strings.TrimSpace(timeoutText))
	if err != nil {
		return writeWrapperOutput(jsonOutput, wrapperOutput{
			OK:    false,
			Mode:  mode,
			Error: fmt.Sprintf("parse --timeout: %v", err),
		}, exitInvalidInput)
	}
	if timeout <= 0 {
		return writeWrapperOutput(jsonOutput, wrapperOutput{
			OK:    false,
			Mode:  mode,
			Error: "--timeout must be > 0",
		}, exitInvalidInput)
	}

	output, exitCode := executeWrapperCommand(wrapperOptions{
		Mode:    mode,
		Command: command,
		Cwd:     strings.TrimSpace(cwd),
		Timeout: timeout,
	})
	return writeWrapperOutput(jsonOutput, output, exitCode)
}

type wrapperOptions struct {
	Mode    string
	Command []string
	Cwd     string
	Timeout time.Duration
}

func executeWrapperCommand(opts wrapperOptions) (wrapperOutput, int) {
	startedAt := time.Now()
	cmd := exec.Command(opts.Command[0], opts.Command[1:]...) // #nosec G204 -- wrapper command is explicit user input.
	cmd.Dir = opts.Cwd
	cmd.Env = append(os.Environ(), "GAIT_WRAPPER_MODE="+opts.Mode)
	setCommandProcessGroup(cmd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return wrapperOutput{
			OK:      false,
			Mode:    opts.Mode,
			Command: append([]string(nil), opts.Command...),
			Cwd:     opts.Cwd,
			Error:   err.Error(),
		}, exitCodeForError(err, exitInternalFailure)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timedOut := false
	var waitErr error
	timer := time.NewTimer(opts.Timeout)
	defer timer.Stop()

	select {
	case waitErr = <-done:
	case <-timer.C:
		timedOut = true
		_ = terminateProcess(cmd)
		select {
		case waitErr = <-done:
		case <-time.After(2 * time.Second):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			waitErr = <-done
		}
	}

	output := wrapperOutput{
		Mode:       opts.Mode,
		Command:    append([]string(nil), opts.Command...),
		Cwd:        opts.Cwd,
		TimedOut:   timedOut,
		DurationMS: time.Since(startedAt).Milliseconds(),
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
	}
	if cmd.ProcessState != nil {
		output.ChildExitCode = cmd.ProcessState.ExitCode()
	}

	tracePaths := uniquePaths(extractKeyValuePaths(output.Stdout, output.Stderr, "trace_path"))
	runpackPaths := uniquePaths(extractKeyValuePaths(output.Stdout, output.Stderr, "runpack_path"))
	output.TracePaths = tracePaths
	output.RunpackPaths = runpackPaths
	invalidTraceCount := 0
	output.VerdictCounts, output.Warnings, invalidTraceCount = summarizeWrapperArtifacts(tracePaths, output.Warnings)

	if timedOut {
		output.OK = false
		output.Error = fmt.Sprintf("child command timed out after %s", opts.Timeout)
		return output, exitInternalFailure
	}
	if waitErr != nil && output.ChildExitCode == 0 {
		output.OK = false
		output.Error = waitErr.Error()
		return output, exitInternalFailure
	}
	if len(tracePaths) == 0 {
		output.OK = false
		output.Error = "child command did not emit a Gait trace reference; wrappers require an explicit Gait interception seam"
		return output, exitInvalidInput
	}
	if opts.Mode == "enforce" && invalidTraceCount > 0 {
		output.OK = false
		output.Error = "child command emitted an invalid Gait trace artifact; enforce mode fails closed"
		return output, exitPolicyBlocked
	}

	exitCode := output.ChildExitCode
	if exitCode == 0 && opts.Mode == "enforce" {
		exitCode = wrapperEnforceExitCode(output.VerdictCounts)
	}
	output.OK = exitCode == exitOK
	return output, exitCode
}

func summarizeWrapperArtifacts(tracePaths []string, warnings []string) ([]wrapperVerdictCount, []string, int) {
	counts := map[string]int{}
	invalidTraceCount := 0
	for _, tracePath := range tracePaths {
		record, err := readValidatedTraceRecord(tracePath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("trace ignored: %s", err.Error()))
			invalidTraceCount++
			continue
		}
		counts[record.Verdict]++
	}
	orderedVerdicts := []string{"allow", "block", "dry_run", "require_approval"}
	verdictCounts := make([]wrapperVerdictCount, 0, len(orderedVerdicts))
	for _, verdict := range orderedVerdicts {
		if counts[verdict] == 0 {
			continue
		}
		verdictCounts = append(verdictCounts, wrapperVerdictCount{Verdict: verdict, Count: counts[verdict]})
	}
	return verdictCounts, warnings, invalidTraceCount
}

func wrapperEnforceExitCode(counts []wrapperVerdictCount) int {
	for _, count := range counts {
		switch count.Verdict {
		case "block", "dry_run":
			if count.Count > 0 {
				return exitPolicyBlocked
			}
		}
	}
	for _, count := range counts {
		if count.Verdict == "require_approval" && count.Count > 0 {
			return exitApprovalRequired
		}
	}
	return exitOK
}

func extractKeyValuePaths(stdout string, stderr string, key string) []string {
	paths := []string{}
	lines := strings.Split(stdout+"\n"+stderr, "\n")
	prefix := key + "="
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		if value != "" {
			paths = append(paths, value)
		}
	}
	return paths
}

func uniquePaths(paths []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		unique = append(unique, trimmed)
	}
	sort.Strings(unique)
	return unique
}

func writeWrapperOutput(jsonOutput bool, output wrapperOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("%s ok: child_exit=%d traces=%d\n", output.Mode, output.ChildExitCode, len(output.TracePaths))
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("%s error: %s\n", output.Mode, output.Error)
		return exitCode
	}
	fmt.Printf("%s finished: child_exit=%d traces=%d\n", output.Mode, output.ChildExitCode, len(output.TracePaths))
	return exitCode
}

func printWrapperUsage(mode string) {
	fmt.Println("Usage:")
	fmt.Printf("  gait %s [--cwd .] [--timeout 30s] [--json] -- <child command...>\n", mode)
}
