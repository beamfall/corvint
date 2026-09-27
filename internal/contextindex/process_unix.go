//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package contextindex

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
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
	command.WaitDelay = pipeDrainDelay
}

func terminateProcessGroup(processID int) {
	if processID > 0 {
		_ = syscall.Kill(-processID, syscall.SIGKILL)
	}
}
