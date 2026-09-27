//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package groupreap

import "os/exec"

// Wait reaps a started command; this platform has no process groups to signal.
func Wait(command *exec.Cmd) error { return command.Wait() }
