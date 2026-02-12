package ui

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestDefaultRunnerMissingCommand(t *testing.T) {
	if _, err := defaultRunner(context.Background(), t.TempDir(), nil); err == nil {
		t.Fatalf("expected missing command error")
	}
}

func TestDefaultRunnerSuccessfulCommand(t *testing.T) {
	t.Setenv("GAIT_UI_HELPER_PROCESS", "1")

	result, err := defaultRunner(context.Background(), t.TempDir(), []string{
		os.Args[0],
		"-test.run=TestDefaultRunnerHelperProcess",
		"--",
		"success",
	})
	if err != nil {
		t.Fatalf("default runner: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code: expected 0 got %d", result.ExitCode)
	}
	if result.Stdout != "ok\n" {
		t.Fatalf("stdout: expected ok, got %q", result.Stdout)
	}
	if result.Stderr != "warn\n" {
		t.Fatalf("stderr: expected warn, got %q", result.Stderr)
	}
}

func TestDefaultRunnerExitCodePropagation(t *testing.T) {
	t.Setenv("GAIT_UI_HELPER_PROCESS", "1")

	result, err := defaultRunner(context.Background(), t.TempDir(), []string{
		os.Args[0],
		"-test.run=TestDefaultRunnerHelperProcess",
		"--",
		"exit7",
	})
	if err != nil {
		t.Fatalf("default runner: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit code: expected 7 got %d", result.ExitCode)
	}
	if result.Stdout != "partial\n" {
		t.Fatalf("stdout: expected partial output, got %q", result.Stdout)
	}
}

func TestDefaultRunnerCommandStartFailure(t *testing.T) {
	if _, err := defaultRunner(context.Background(), t.TempDir(), []string{"gait-command-that-should-not-exist-123"}); err == nil {
		t.Fatalf("expected command start failure")
	}
}

func TestDefaultRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GAIT_UI_HELPER_PROCESS") != "1" {
		return
	}

	mode := ""
	for index, value := range os.Args {
		if value == "--" && index+1 < len(os.Args) {
			mode = os.Args[index+1]
			break
		}
	}

	switch mode {
	case "success":
		_, _ = fmt.Fprintln(os.Stdout, "ok")
		_, _ = fmt.Fprintln(os.Stderr, "warn")
		os.Exit(0)
	case "exit7":
		_, _ = fmt.Fprintln(os.Stdout, "partial")
		os.Exit(7)
	default:
		os.Exit(2)
	}
}
