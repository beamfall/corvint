//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package doccompiler

import (
	"os/exec"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

func configureProcess(command *exec.Cmd) {
	// Cancellation stops the leader; groupreap.Wait sweeps the group before the reap.
	groupreap.Contain(command)
	// The bound detects a descendant holding the output pipes, not a slow reader (V1-0391).
	command.WaitDelay = time.Minute
}

func descendantCleanupQualification() (bool, string) {
	return false, "UNQUALIFIED: process-group cleanup does not contain setsid/session escapes"
}
