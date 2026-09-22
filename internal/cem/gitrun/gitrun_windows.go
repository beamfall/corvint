//go:build windows

package gitrun

import "os/exec"

// Windows native execution is a separate unpromoted lane; no process-group or
// Job-object containment is claimed here. The stub kills only the direct child.
func containChild(command *exec.Cmd) {}

func killGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = command.Process.Kill()
}

func killDescendants(command *exec.Cmd) {}
