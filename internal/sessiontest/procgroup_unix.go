//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package sessiontest

import (
	"os/exec"
	"syscall"
)

// groupsSupported reports whether an adapter can run in a process group the
// run owns and kills as a whole (STR-V0-014).
const groupsSupported = true

// ownGroup starts cmd as the leader of a new process group and makes its
// cancellation kill that whole group.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { killGroup(cmd); return nil }
}

// killGroup kills every process left in cmd's group.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
