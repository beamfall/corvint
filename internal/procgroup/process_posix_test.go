//go:build darwin || linux

package procgroup

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// killTestProcessGroup sends SIGKILL straight to the OS process group led by
// pgid, bypassing any grace period. It exists only so process_test.go's
// group-directed kill assertions (which are inherently POSIX process-group
// semantics) still compile on platforms where process_other.go's contract
// has no process-group equivalent (see TestRunProcessNonPOSIXContract).
func killTestProcessGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// V1-1037: a stopped or continued leader is not reported as exited. On Darwin
// waitid with WEXITED|WNOWAIT also returns for those state changes
// (golang/go#19314); treating that as an exit would terminate the group while
// the leader lives. Mirrors groupreap's TestLeaderUnreapedIgnoresStopAndContinue.
func TestWaitProcessExitUnreapedIgnoresStopAndContinue(t *testing.T) {
	command := exec.Command("/bin/sleep", "60")
	configureProcessCommand(command, false)
	if err := startProcessCommand(command); err != nil {
		t.Fatal(err)
	}
	leader := command.Process.Pid
	t.Cleanup(func() { _ = command.Process.Kill(); releaseProcessCommand(leader); _ = command.Wait() })
	if err := syscall.Kill(leader, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !processStoppedForTest(t, leader) {
		if time.Now().After(deadline) {
			t.Fatal("leader did not stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
	returned := make(chan error, 1)
	go func() { returned <- waitProcessExitUnreaped(leader) }()
	select {
	case err := <-returned:
		t.Fatalf("waitProcessExitUnreaped returned for a stopped leader: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := syscall.Kill(leader, syscall.SIGCONT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		t.Fatalf("waitProcessExitUnreaped returned for a continued leader: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := syscall.Kill(leader, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("waitProcessExitUnreaped after exit = %v", err)
		}
	case <-time.After(10 * time.Second): // hang detector, not a budget (decision 0082)
		t.Fatal("waitProcessExitUnreaped did not observe the exit")
	}
	// The exit was observed without reaping: the leader is still a zombie
	// holding its PID until Wait.
	if err := syscall.Kill(leader, 0); err != nil {
		t.Fatalf("leader was reaped by the exit observation: %v", err)
	}
}

func processStoppedForTest(t *testing.T, pid int) bool {
	t.Helper()
	out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "T")
}

// AHI-049: Run records a group it leads for the adapter exit kill and releases
// that record once, after its pre-reap group cleanup and while the exited
// leader is still unreaped, so groupreap.KillLive never names a reaped
// leader's group.
func TestAHI049RunReleasesGroupBeforeReap(t *testing.T) {
	previous := releaseProcessCommand
	t.Cleanup(func() { releaseProcessCommand = previous })
	var leader atomic.Int64
	var released []int
	var unreaped []error
	releaseProcessCommand = func(pid int) {
		released = append(released, pid)
		// Returns at once for an exited, unreaped leader; ECHILD once it is reaped.
		unreaped = append(unreaped, waitProcessExitUnreaped(pid))
		previous(pid)
	}
	observation := Run(context.Background(), Spec{
		Argv: []string{"/bin/sh", "-c", "exit 0"}, Dir: t.TempDir(), Timeout: 10 * time.Second,
		AfterStart: func(_ context.Context, pid int) error { leader.Store(int64(pid)); return nil },
	})
	if observation.Err != nil {
		t.Fatal(observation.Err)
	}
	if len(released) != 1 || int64(released[0]) != leader.Load() {
		t.Fatalf("released %v, want the leader %d once", released, leader.Load())
	}
	if unreaped[0] != nil {
		t.Fatalf("group released after its leader was reaped: %v", unreaped[0])
	}
}
