//go:build !darwin && !linux

package changee2e

import "os/exec"

func configureServe(*exec.Cmd) {}

func killServe(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// waitServeExit reaps the serve process; without a process group there is no group ID to reserve.
func waitServeExit(cmd *exec.Cmd) { _, _ = cmd.Process.Wait() }
