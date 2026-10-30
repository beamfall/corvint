//go:build !unix

package store

import (
	"os"
	"os/exec"
)

func containGate(*exec.Cmd) {}

func killGate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func stopGate(cmd *exec.Cmd) { killGate(cmd) }

func reapGate(cmd *exec.Cmd) error { return cmd.Wait() }

func gateSignal(*os.ProcessState) string { return "" }
