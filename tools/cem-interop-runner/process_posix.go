//go:build unix

package main

import (
	"errors"
	"os/exec"
	"syscall"
)

// setProcessGroup makes cmd the leader of a new OS process group so
// killProcessGroup can terminate it and any descendants together. It is only
// reached on unix: runProcess returns "posix-required" before starting a
// command on any other platform.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to the process group led by cmd's PID.
func killProcessGroup(cmd *exec.Cmd) error {
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// processAlive reports whether pid is still running, by probing it with
// signal 0.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
