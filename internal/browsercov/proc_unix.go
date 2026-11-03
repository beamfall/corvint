// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package browsercov

import (
	"os/exec"
	"syscall"
)

func unsupportedPlatform() error { return nil }

// setGroup puts a command in its own process group, so stopping the group
// stops everything it started.
func setGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalGroup(pid int, sig syscall.Signal) error { return syscall.Kill(-pid, sig) }

func signalPid(pid int, sig syscall.Signal) error { return syscall.Kill(pid, sig) }

func signalUsr2(pid int) error { return syscall.Kill(pid, syscall.SIGUSR2) }

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// groupAlive reports whether any process of the group led by pid remains.
func groupAlive(pid int) bool { return syscall.Kill(-pid, 0) == nil }

const sigTerm, sigKill = syscall.SIGTERM, syscall.SIGKILL
