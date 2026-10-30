// Package groupreap reaps a process-group leader started with Setpgid and
// SIGKILLs whatever is left in its group, signalling while the exited leader
// is still unreaped so the group ID cannot name a process that reused its PID.
package groupreap

import "os/exec"

// Run is exec.Cmd.Run with the leader reaped by Wait. It starts through
// StartLive, so a group the command leads is recorded for KillLive until
// Wait's pre-reap release (AHI-048, PGO-V0-009), and a start after KillLive
// is refused with ErrExiting.
func Run(command *exec.Cmd) error {
	if err := liveGroups.start(command); err != nil {
		return err
	}
	return Wait(command)
}
