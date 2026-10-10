//go:build !darwin && !linux

package cli

import "os/exec"

// doctorPluginGroup: elsewhere the timeout kills the plugin process only.
func doctorPluginGroup(c *exec.Cmd) {}
