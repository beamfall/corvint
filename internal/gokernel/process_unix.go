//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gokernel

import (
	"os/exec"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/groupreap"
)

func configureProcess(command *exec.Cmd) {
	if gitstatus.OwnedWorker() {
		// CommandContext's default Cancel kills only its own unreaped leader.
		command.WaitDelay = pipeDrainDelay
		return
	}
	// Cancellation stops the leader; groupreap.Wait sweeps the group before the reap.
	groupreap.Contain(command)
	command.WaitDelay = pipeDrainDelay
}
