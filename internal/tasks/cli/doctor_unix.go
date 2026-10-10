//go:build darwin || linux

package cli

import (
	"os/exec"
	"syscall"
)

// doctorPluginGroup runs a plugin in its own process group and kills the
// whole group when its timeout cancels it (TQD-V0-010).
func doctorPluginGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
}
