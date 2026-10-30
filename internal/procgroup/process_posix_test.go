//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
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

// V1-1055: once Run releases the group for the reap, ownership of the numeric
// PGID has ended and the PGID may be reused, so no numeric group operation —
// not even the signal-0 quiescence probe — may follow. The group's sleeping
// member is killed and its quiescence is proved before the reap.
func TestV11055NoNumericGroupProbeAfterReap(t *testing.T) {
	previous := releaseProcessCommand
	t.Cleanup(func() { releaseProcessCommand = previous })
	var released atomic.Bool
	releaseProcessCommand = func(pid int) { released.Store(true); previous(pid) }
	var late atomic.Int64
	observation := run(context.Background(), Spec{
		Argv: []string{"/bin/sh", "-c", "/bin/sleep 30 & exit 0"}, Dir: t.TempDir(),
		Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 10 * time.Second, ShutdownTimeout: 5 * time.Second,
	}, processWaitChannels{groupProbe: func(pid int, signal syscall.Signal) error {
		if released.Load() {
			late.Add(1)
		}
		return signalOwnedProcessGroup(pid, signal)
	}})
	if n := late.Load(); n != 0 {
		t.Fatalf("%d numeric group operations after the group was released for the reap", n)
	}
	if observation.Err != nil || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("pre-reap cleanup not proved: %+v", observation)
	}
}

// V1-1055: the quiescence proof runs while the exited leader is still
// unreaped and before the group is released, and a failed proof leaves the
// cleanup unproved.
func TestV11055QuiescenceProvedWhileLeaderHeld(t *testing.T) {
	previous := releaseProcessCommand
	t.Cleanup(func() { releaseProcessCommand = previous })
	var released atomic.Bool
	releaseProcessCommand = func(pid int) { released.Store(true); previous(pid) }
	spec := Spec{Argv: []string{"/bin/sh", "-c", "exit 0"}, Dir: t.TempDir(), Timeout: 10 * time.Second, ShutdownTimeout: 5 * time.Second}
	var proofs, unheld atomic.Int64
	observation := run(context.Background(), spec, processWaitChannels{quietProof: func(pid int) (bool, error) {
		proofs.Add(1)
		if released.Load() || waitProcessExitUnreaped(pid) != nil {
			unheld.Add(1)
		}
		return groupreap.HeldGroupQuiet(pid)
	}})
	if proofs.Load() == 0 || unheld.Load() != 0 {
		t.Fatalf("proofs %d, %d after the leader was released or reaped", proofs.Load(), unheld.Load())
	}
	if observation.Err != nil || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("held-group proof did not prove cleanup: %+v", observation)
	}

	failed := run(context.Background(), spec, processWaitChannels{quietProof: func(int) (bool, error) {
		return false, errors.New("proof unavailable")
	}})
	if failed.OwnedProcessGroupCleanup || !processErrorHasCode(failed.Err, "process-cleanup-failed") {
		t.Fatalf("a failed proof was not reported unproved: %+v", failed)
	}
}

// V1-1055 review: a foreign in-process reaper (unsupported, AHI-050) collects
// the leader during timeout cleanup, so the waitid goroutine fails and
// releases the group while run still signals it. No numeric group operation
// may follow that release, and the cleanup stays unproved.
//
// V1-1378: the interleaving is forced, not raced. The waitid goroutine starts
// only after BeforeStop's foreign reap, so it always fails; BeforeStop returns
// only after that goroutine released the group; and run's own exit-observation
// channel is withheld, so run proceeds to signal the group as if the release
// had landed just after its non-blocking check. Every group operation run then
// attempts must be refused, not sent.
func TestV11055NoGroupOperationAfterConcurrentRelease(t *testing.T) {
	previous := releaseProcessCommand
	t.Cleanup(func() { releaseProcessCommand = previous })
	released := make(chan struct{})
	var releases atomic.Int64
	releaseProcessCommand = func(pid int) {
		if releases.Add(1) == 1 {
			close(released)
		}
		previous(pid)
	}
	reaped := make(chan struct{})
	var exitWaitErr error
	var sent atomic.Int64
	observation := run(context.Background(), Spec{
		Argv: []string{"/bin/sleep", "30"}, Dir: t.TempDir(), Env: []string{"PATH=/usr/bin:/bin"},
		Timeout: 20 * time.Millisecond, ShutdownTimeout: 200 * time.Millisecond,
		BeforeStop: func(_ context.Context, pid int) error {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			var status syscall.WaitStatus
			_, _ = syscall.Wait4(pid, &status, 0, nil)
			close(reaped)
			select { // hang detector only: the release follows the reap at once
			case <-released:
			case <-time.After(30 * time.Second):
				t.Error("the waitid goroutine never released the group after the foreign reap")
			}
			return nil
		},
	}, processWaitChannels{
		exitObserved: make(chan error), // withheld: run does not see the failure before signalling
		exitWait: func(pid int) error {
			<-reaped
			exitWaitErr = waitProcessExitUnreaped(pid)
			return exitWaitErr
		},
		groupProbe: func(pid int, signal syscall.Signal) error {
			sent.Add(1)
			return signalOwnedProcessGroup(pid, signal)
		},
	})
	if exitWaitErr == nil {
		t.Fatal("the exit observation succeeded after the foreign reap")
	}
	if n := sent.Load(); n != 0 {
		t.Fatalf("%d numeric group operations sent after the group was released", n)
	}
	if !errors.Is(observation.Err, errGroupOwnershipReleased) {
		t.Fatalf("run's group signal after the release was not refused: %v", observation.Err)
	}
	if observation.OwnedProcessGroupCleanup {
		t.Fatalf("cleanup proved after a foreign reap: %+v", observation)
	}
	if n := releases.Load(); n != 1 {
		t.Fatalf("group released %d times, want once", n)
	}
}

// V1-2319: a positive TerminateGrace gives a slow SIGTERM handler that long
// before SIGKILL, so a handler delayed past the 100 ms default still completes;
// a negative grace is refused before any process starts. The shell waits in
// short foreground sleeps rather than on a background `sleep 30`: a SIGTERM that
// reached that child between its fork and its exec ran the inherited trap
// instead of ending it, so a third of loaded Linux runs kept a live member
// until the 10 s grace's SIGKILL (V1-2388).
func TestTerminateGraceLetsSlowTermHandlerFinish(t *testing.T) {
	fixture := t.TempDir()
	ready, handled := filepath.Join(fixture, "ready"), filepath.Join(fixture, "handled")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		waitForProcessFile(t, ready)
		cancel()
	}()
	observation := Run(ctx, Spec{
		Argv: []string{"/bin/sh", "-c", `trap 'sleep 0.5; echo >"$1/handled"; exit 0' TERM; echo >"$1/ready"; while :; do sleep 0.05; done`, "sh", fixture},
		Dir:  fixture, Env: []string{"PATH=/usr/bin:/bin"}, Timeout: time.Minute, ShutdownTimeout: 20 * time.Second, TerminateGrace: 10 * time.Second,
	})
	if _, err := os.Stat(handled); err != nil || !observation.Cancelled || observation.StopSignal != "SIGTERM" {
		t.Fatalf("slow TERM handler did not finish within the grace: err=%v observation=%+v", err, observation)
	}
	if refused := Run(context.Background(), Spec{Argv: []string{"/bin/true"}, Timeout: time.Second, TerminateGrace: -time.Second}); refused.Started || refused.Err == nil {
		t.Fatalf("negative terminate grace accepted: %+v", refused)
	}
}
