//go:build darwin

package repository

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestWaitProcessExitUnreapedIgnoresStoppedChild pins V1-1046: a stopped child is not an exit (golang/go#19314), so the
// unreaped wait must keep blocking until the child really exits, and must
// leave it unreaped.
func TestWaitProcessExitUnreapedIgnoresStoppedChild(t *testing.T) {
	command := exec.Command("sleep", "60")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pid := command.Process.Pid
	reaped := false
	t.Cleanup(func() {
		// Signal only while this test still owns the unreaped PID.
		if !reaped {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			_, _ = command.Process.Wait()
		}
	})
	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	returned := make(chan error, 1)
	go func() { returned <- waitProcessExitUnreaped(pid) }()
	select {
	case err := <-returned:
		t.Fatalf("wait returned %v for a stopped child", err)
	case <-time.After(500 * time.Millisecond):
	}
	if err := syscall.Kill(pid, syscall.SIGCONT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		t.Fatalf("wait returned %v for a continued child", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("wait after exit: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("wait did not return after the child exited")
	}
	// Still unreaped: Wait must collect the SIGKILL status itself, not ECHILD.
	err := command.Wait()
	reaped = true
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Wait = %v, want the child's own exit status", err)
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("Wait status = %v, want killed by SIGKILL", exitErr.ProcessState)
	}
}

// PGO-V0-009: an exit observation that fails while the group is being stopped
// (pinned turns false) ends every probe and signal. The grace loop used to
// probe the numeric group with signal 0 until the grace ran out without
// re-checking pinned.
func TestPGOV0009DarwinStopSendsNothingOnceUnpinned(t *testing.T) {
	previous := groupSignal
	t.Cleanup(func() { groupSignal = previous })
	unpinned, afterUnpin, probes := false, 0, 0
	groupSignal = func(pid int, signal syscall.Signal) error {
		if unpinned {
			afterUnpin++
			return nil
		}
		if signal == 0 {
			probes++
			// The exit observation fails during the second probe.
			unpinned = probes == 2
		}
		return nil
	}
	containment := &childContainment{pid: 4242}
	if containment.stopAndProveDarwin(200*time.Millisecond, func() bool { return !unpinned }) {
		t.Fatal("cleanup proven after the exit observation failed")
	}
	if afterUnpin != 0 {
		t.Fatalf("%d group signals sent after the exit observation failed", afterUnpin)
	}
}
