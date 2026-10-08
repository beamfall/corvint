//go:build unix

package gitrun

import (
	"os/exec"
	"syscall"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// containChild places the child in its own process group so the whole
// descendant tree can be signalled as one unit.
func containChild(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup ends a run early through groupreap.Stop. Where waitid is
// available Stop SIGKILLs only the leader, through os.Process, which refuses a
// leader the waiting goroutine has already reaped; the leader's exit then lets
// groupreap.Wait SIGKILL the group while the exited leader is still unreaped,
// so no group signal can reach a recycled group ID. Elsewhere (not 1.0
// platforms) Stop signals the group directly and the reap race remains
// (V1-0373, V1-0652).
func killGroup(command *exec.Cmd) {
	_ = groupreap.Stop(command)
}
