//go:build darwin || linux || freebsd

package main

import (
	"os/exec"
	"syscall"
)

func prepareContainedCommand(command *exec.Cmd) bool {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return true
}

func signalContainedCommand(command *exec.Cmd, force bool) {
	if command.Process != nil {
		signal := syscall.SIGTERM
		if force {
			signal = syscall.SIGKILL
		}
		_ = syscall.Kill(-command.Process.Pid, signal)
	}
}
