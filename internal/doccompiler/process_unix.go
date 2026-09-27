//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package doccompiler

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
	// The bound detects a descendant holding the output pipes, not a slow reader (V1-0391).
	command.WaitDelay = time.Minute
}

func descendantCleanupQualification() (bool, string) {
	return false, "UNQUALIFIED: process-group cleanup does not contain setsid/session escapes"
}
