//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package groupreap

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

// waitidAvailable is false: this platform has no process groups, so nothing
// is signalled before a reap and StartLive records nothing.
const waitidAvailable = false

func setpgid(*syscall.SysProcAttr) bool { return false }

func signalLiveGroup(int, syscall.Signal) error { return nil }

// Wait reaps a started command; this platform has no process groups to signal.
func Wait(command *exec.Cmd) error { return command.Wait() }

func wait(command *exec.Cmd, exited func()) error {
	err := command.Wait()
	if exited != nil {
		exited()
	}
	return err
}

// Contain leaves command unchanged: this platform has no process groups.
func Contain(*exec.Cmd) {}

// Drain starts command and returns its Wait; this platform has no process
// groups to signal. A start after KillLive is refused with ErrExiting.
func Drain(_ context.Context, command *exec.Cmd, _ time.Duration) (func() error, error) {
	if err := liveGroups.start(command); err != nil {
		return nil, err
	}
	return command.Wait, nil
}
