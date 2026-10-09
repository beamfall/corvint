//go:build unix

package mutate

import (
	"os/exec"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// configureProcess puts the run in its own process group so everything a
// cited test starts dies with it, on timeout and after a normal exit.
func configureProcess(command *exec.Cmd) {
	// Cancellation stops the leader; groupreap.Run sweeps the group before the reap.
	groupreap.Contain(command)
	// The bound detects a descendant holding the output pipes, not a slow reader (V1-0391).
	command.WaitDelay = time.Minute
}
