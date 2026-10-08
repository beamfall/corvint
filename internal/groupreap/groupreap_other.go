//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package groupreap

import (
	"context"
	"os/exec"
	"time"
)

// Wait reaps a started command; this platform has no process groups to signal.
func Wait(command *exec.Cmd) error { return command.Wait() }

func wait(command *exec.Cmd, exited func()) error {
	err := command.Wait()
	if exited != nil {
		exited()
	}
	return err
}

// Drain starts command and returns its Wait; this platform has no process
// groups to signal.
func Drain(_ context.Context, command *exec.Cmd, _ time.Duration) (func() error, error) {
	if err := command.Start(); err != nil {
		return nil, err
	}
	return command.Wait, nil
}
