//go:build unix

package gitrun

import (
	"os/exec"
	"syscall"
)

// containChild places the child in its own process group so the whole
// descendant tree can be signalled as one unit.
func containChild(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup SIGKILLs the child's entire process group before the child is
// reaped, so the group ID cannot have been recycled.
func killGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}

// killDescendants sweeps the group after a normal exit; surviving descendants
// keep the group alive, and ESRCH means there is nothing to clean.
func killDescendants(command *exec.Cmd) {
	killGroup(command)
}
