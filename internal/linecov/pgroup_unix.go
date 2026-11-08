//go:build unix

// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"os"
	"os/exec"
	"syscall"
)

// isolate runs cmd in a process group of its own, so a test that signals its
// group cannot reach the collector, and a cancelled run kills every process
// the test started (GLS-V0-008).
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err == syscall.ESRCH {
			return os.ErrProcessDone
		} else {
			return err
		}
	}
}

// cleanup bounds descendants even when a test exits successfully before its
// children. Such a run cannot publish a complete coverage record.
func cleanup(cmd *exec.Cmd) bool {
	if cmd.Process == nil {
		return false
	}
	if err := syscall.Kill(-cmd.Process.Pid, 0); err != nil {
		return false
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	return true
}
