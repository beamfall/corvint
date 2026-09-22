//go:build windows

package gokernel

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

func waitGroupLeader(command *exec.Cmd) error { return command.Wait() }
