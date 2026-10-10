//go:build darwin || linux

package procgroup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

// TestAHI049FailedExitObservationSendsNoGroupSignal injects an exit
// observation failure (ECHILD, as after another reaper) while the leader is
// live: the exit-kill record is released before BeforeStop, no group signal
// follows, and cleanup is reported unproved.
func TestAHI049FailedExitObservationSendsNoGroupSignal(t *testing.T) {
	fixture := t.TempDir()
	ready, signalled := filepath.Join(fixture, "ready"), filepath.Join(fixture, "signalled")
	previous := releaseProcessCommand
	t.Cleanup(func() { releaseProcessCommand = previous })
	var released atomic.Int64
	releaseProcessCommand = func(pid int) { released.Add(1); previous(pid) }
	var leader atomic.Int64
	t.Cleanup(func() {
		pid := int(leader.Load())
		if pid <= 0 {
			return
		}
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		for deadline := time.Now().Add(10 * time.Second); processExists(pid) && time.Now().Before(deadline); {
			time.Sleep(10 * time.Millisecond)
		}
	})
	exitObserved := make(chan error, 1)
	var releasedBeforeStop int64
	done := make(chan Observation, 1)
	go func() {
		done <- run(context.Background(), Spec{
			Argv: []string{"/bin/sh", "-c", `trap 'echo >"$1/signalled"; exit 0' TERM; echo >"$1/ready"; while :; do sleep 0.05; done`, "sh", fixture},
			Dir:  fixture, Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 10 * time.Second, ShutdownTimeout: 200 * time.Millisecond,
			AfterStart: func(_ context.Context, pid int) error { leader.Store(int64(pid)); return nil },
			BeforeStop: func(context.Context, int) error { releasedBeforeStop = released.Load(); return nil },
		}, processWaitChannels{exitObserved: exitObserved})
	}()
	waitForProcessFile(t, ready)
	exitObserved <- syscall.ECHILD
	observation := <-done
	if releasedBeforeStop == 0 {
		t.Fatal("group still recorded for the exit kill after a failed exit observation")
	}
	for _, code := range []string{"process-exit-observation-failed", "process-cleanup-failed"} {
		if !processErrorHasCode(observation.Err, code) {
			t.Fatalf("error %v does not contain code %q", observation.Err, code)
		}
	}
	if observation.OwnedProcessGroupCleanup {
		t.Fatalf("cleanup reported proved after a failed exit observation: %+v", observation)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(signalled); err == nil {
		t.Fatal("process group signalled after a failed exit observation")
	}
	if !processExists(int(leader.Load())) {
		t.Fatal("leader gone after a failed exit observation")
	}
}

func processErrorHasCode(err error, code string) bool {
	if failure, ok := err.(*processError); ok && failure.Code == code {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, inner := range joined.Unwrap() {
			if processErrorHasCode(inner, code) {
				return true
			}
		}
	}
	return false
}
