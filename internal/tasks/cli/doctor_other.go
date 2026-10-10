//go:build !darwin && !linux

package cli

import (
	"io/fs"
	"os/exec"
)

// doctorPluginGroup: elsewhere the timeout kills the plugin process only.
func doctorPluginGroup(c *exec.Cmd) {}

// doctorPluginReap: elsewhere there is no process group to kill.
func doctorPluginReap(c *exec.Cmd) {}

// doctorOwned: elsewhere ownership is not checked.
func doctorOwned(st fs.FileInfo) bool { return true }
