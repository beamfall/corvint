//go:build darwin || linux

package store

import (
	"os/exec"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// stopGate kills a started lease gate early. It signals only the leader
// through os.Process, which refuses a reaped leader; reapGate SIGKILLs the
// group while the exited leader is unreaped (V1-1047, PGO-V0-008).
func stopGate(cmd *exec.Cmd) { _ = groupreap.Stop(cmd) }

// reapGate is groupreap.Wait: waitid observes the exit without reaping, so
// the group SIGKILL precedes the reap, and a failed observation sends none.
func reapGate(cmd *exec.Cmd) error { return groupreap.Wait(cmd) }
