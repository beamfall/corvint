// SPDX-License-Identifier: Apache-2.0

package docsenvelope

import (
	"os/exec"
	"syscall"
	"time"
)

// StopOnCancel makes a cancelled command context ask the child to stop instead of killing it
// at once. corvint and corvint-docs start Git in process groups of their own and reap them on
// their way out, so a child killed outright leaves those reads running with no cleanup and no
// deadline watcher. The child gets SIGTERM, which both binaries handle, and grace to exit;
// after grace it is killed and its pipes are closed. Where SIGTERM cannot be sent (Windows)
// the child is killed at once, as before.
func StopOnCancel(command *exec.Cmd, grace time.Duration) {
	command.Cancel = func() error {
		if err := command.Process.Signal(syscall.SIGTERM); err != nil {
			return command.Process.Kill()
		}
		return nil
	}
	command.WaitDelay = grace
}
