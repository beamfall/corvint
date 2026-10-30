//go:build unix

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// ownProcessGroup runs the child in its own process group and kills the whole
// group on cancellation, so Git descendants of corvint cannot outlive the run.
func ownProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = 5 * time.Second
}
