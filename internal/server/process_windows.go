//go:build windows

package server

import (
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {
	// Windows does not support POSIX process groups.
}

func terminateProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func isSignaledExit(err *exec.ExitError) bool {
	// Windows does not have POSIX signals.
	return false
}
