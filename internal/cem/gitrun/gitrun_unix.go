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

// killGroup SIGKILLs the child's entire process group to end a run early,
// while the leader is normally still running. On every exit, groupreap.Wait in
// the waiting goroutine SIGKILLs the group again while the exited leader is
// still unreaped, so that sweep cannot reach a recycled group ID.
func killGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}
