//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package groupreap

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
)

// The group is signalled while the exited leader is still unreaped, so its
// PID, and with it the group ID, cannot have been reused by another process.
func TestGroupIsSignalledBeforeLeaderIsReaped(t *testing.T) {
	command := exec.Command("/bin/sh", "-c", "exit 0")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	leader := command.Process.Pid
	observed := errors.New("process group was not signalled")
	previous := signalGroup
	t.Cleanup(func() { signalGroup = previous })
	signalGroup = func(processID int, signal syscall.Signal) error {
		if processID == -leader {
			observed = leaderUnreaped(leader)
		}
		return previous(processID, signal)
	}
	if err := Wait(command); err != nil {
		t.Fatal(err)
	}
	if errors.Is(observed, errors.ErrUnsupported) {
		t.Skip("waitid is unavailable on this platform")
	}
	if observed != nil {
		t.Fatalf("group signalled after the leader was reaped: %v", observed)
	}
}
