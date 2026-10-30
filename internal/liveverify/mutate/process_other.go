//go:build !unix

package mutate

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
	// The bound detects a descendant holding the output pipes, not a slow reader (V1-0391).
	command.WaitDelay = time.Minute
}
