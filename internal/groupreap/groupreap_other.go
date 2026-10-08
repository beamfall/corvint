//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package groupreap

import (
	"os/exec"
	"syscall"
)

// liveTracking is false: this platform has no process groups to record.
const liveTracking = false

func setpgid(*syscall.SysProcAttr) bool { return false }

func signalLiveGroup(int, syscall.Signal) error { return nil }

// Wait reaps a started command; this platform has no process groups to signal.
func Wait(command *exec.Cmd) error { return command.Wait() }

// WaitPipes reaps a started command with command.Wait.
func WaitPipes(command *exec.Cmd) error { return command.Wait() }

func wait(command *exec.Cmd, exited func()) error {
	err := command.Wait()
	if exited != nil {
		exited()
	}
	return err
}
