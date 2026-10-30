//go:build windows

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
