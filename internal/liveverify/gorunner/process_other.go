//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package gorunner

import (
	"os/exec"
	"time"
)

func platformContainment() (Containment, bool) {
	return ContainmentUnsupported, false
}

func configureCommand(_ *exec.Cmd)                  {}
func terminateCommand(_ *exec.Cmd) error            { return nil }
func cleanupCommand(_ *exec.Cmd, _ time.Time) error { return nil }
