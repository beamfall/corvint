//go:build !unix

package changee2e

import "os/exec"

func configureServe(*exec.Cmd) {}

func killServe(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
