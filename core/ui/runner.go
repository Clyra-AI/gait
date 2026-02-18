package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type runResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type Runner func(ctx context.Context, workDir string, argv []string) (runResult, error)

func defaultRunner(ctx context.Context, workDir string, argv []string) (runResult, error) {
	if len(argv) == 0 {
		return runResult{}, fmt.Errorf("missing command")
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...) // #nosec G204,G702
	command.Dir = strings.TrimSpace(workDir)
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf
	err := command.Run()
	exitCode := 0
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return runResult{}, err
		}
	}
	return runResult{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
	}, nil
}
