//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gokernel

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = time.Second
}

// signalProcessGroup is syscall.Kill; tests observe when the group is signalled.
var signalProcessGroup = syscall.Kill

// waitGroupLeader reaps a Setpgid command and SIGKILLs any process left in its
// group. Where waitid is available the group is signalled while the exited
// leader is still unreaped, so the group ID cannot name a reused process;
// elsewhere it is signalled after Wait, as before.
func waitGroupLeader(command *exec.Cmd) error {
	processID := command.Process.Pid
	if waitLeaderUnreaped(processID) == nil {
		_ = signalProcessGroup(-processID, syscall.SIGKILL)
		return command.Wait()
	}
	err := command.Wait()
	_ = signalProcessGroup(-processID, syscall.SIGKILL)
	return err
}
