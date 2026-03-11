//go:build windows

package main

import "os/exec"

func setCommandProcessGroup(_ *exec.Cmd) {}

func terminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
