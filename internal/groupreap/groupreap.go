// Package groupreap reaps a process-group leader started with Setpgid and
// SIGKILLs whatever is left in its group, signalling while the exited leader
// is still unreaped so the group ID cannot name a process that reused its PID.
package groupreap

import "os/exec"

// Run is exec.Cmd.Run with the leader reaped by Wait.
func Run(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		return err
	}
	return Wait(command)
}
