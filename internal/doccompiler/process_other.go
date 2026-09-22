//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package doccompiler

import (
	"os"
	"os/exec"
	"time"
)

func configureProcess(command *exec.Cmd) {
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		return command.Process.Kill()
	}
	command.WaitDelay = time.Second
}

func terminateProcessGroup(_ int) {}

func descendantCleanupQualification() (bool, string) {
	return false, "UNKNOWN: descendant containment is not implemented on this platform"
}
