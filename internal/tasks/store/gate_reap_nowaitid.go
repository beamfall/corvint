//go:build unix && !darwin && !linux

package store

import "os/exec"

// stopGate kills only the leader through os.Process, which refuses a leader
// it has reaped. Without waitid no group signal can be ordered before the
// reap (groupreap's V1-0652 limitation), so none is sent (V1-1047).
func stopGate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// reapGate reaps without a group sweep. Descendants left in the group are
// not killed here; WaitDelay bounds how long they can hold the output pipe.
func reapGate(cmd *exec.Cmd) error { return cmd.Wait() }
