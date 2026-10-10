//go:build darwin || linux

package cli

import (
	"io/fs"
	"os"
	"os/exec"
	"syscall"
)

// doctorPluginGroup runs a plugin in its own process group and kills the
// whole group when its timeout cancels it (TQD-V0-010).
func doctorPluginGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
}

// doctorPluginReap kills whatever is left of the plugin's process group
// after the plugin exits or times out, so no descendant outlives the
// doctor (TQD-V0-010). The group id cannot be reused while any member
// lives, and an empty group answers ESRCH.
func doctorPluginReap(c *exec.Cmd) {
	if c.Process != nil {
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
}

// doctorOwned reports whether the effective user owns the file.
func doctorOwned(st fs.FileInfo) bool {
	sys, ok := st.Sys().(*syscall.Stat_t)
	return ok && int(sys.Uid) == os.Geteuid()
}
