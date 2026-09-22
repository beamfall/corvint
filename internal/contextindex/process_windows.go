//go:build windows

package contextindex

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
