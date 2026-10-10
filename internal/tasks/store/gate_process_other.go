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

func killGateGroup(int) {}

func gateSignal(*os.ProcessState) string { return "" }
