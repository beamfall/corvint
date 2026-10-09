//go:build linux

package repository

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"
)

// recordGroupSignals replaces groupSignal for one test and returns the
// signals sent so far.
func recordGroupSignals(t *testing.T) func() []syscall.Signal {
	t.Helper()
	var mu sync.Mutex
	var sent []syscall.Signal
	previous := groupSignal
	t.Cleanup(func() { groupSignal = previous })
	groupSignal = func(pid int, signal syscall.Signal) error {
		mu.Lock()
		sent = append(sent, signal)
		mu.Unlock()
		return previous(pid, signal)
	}
	return func() []syscall.Signal {
		mu.Lock()
		defer mu.Unlock()
		return append([]syscall.Signal(nil), sent...)
	}
}

func startContained(t *testing.T, script string) (*exec.Cmd, *childContainment) {
	t.Helper()
	command := exec.Command("/bin/sh", "-c", script)
	containment, ok := prepareContainment(command)
	if !ok {
		t.Fatal("containment unavailable")
	}
	t.Cleanup(containment.close)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if !containment.attach(command.Process) {
		containment.force()
		_, _ = command.Process.Wait()
		t.Fatal("attach containment")
	}
	return command, containment
}

// V1-1047 review P1: an exit observation that fails with ECHILD (another
// reaper collected the leader) leaves the numeric group ID unpinned, so no
// group signal is sent and cleanup is reported unproved.
func TestV11047FailedExitObservationSendsNoGroupSignal(t *testing.T) {
	previous := observeLeaderExit
	t.Cleanup(func() { observeLeaderExit = previous })
	observeLeaderExit = func(pid int) error {
		if err := previous(pid); err != nil {
			return err
		}
		return syscall.ECHILD
	}
	signals := recordGroupSignals(t)
	command, containment := startContained(t, "exit 0")
	outcome := containment.wait(command, context.Background(), nil, func() bool { return true })
	if sent := signals(); len(sent) != 0 {
		t.Fatalf("group signals %v sent after a failed exit observation", sent)
	}
	if !outcome.observationFailed || outcome.cleanupProven {
		t.Fatalf("outcome = %#v: want a failed observation and unproved cleanup", outcome)
	}
}

// V1-1047 review P2: an unreadable process table during cancellation still
// SIGKILLs the pinned group, so a TERM-ignoring leader cannot block the
// wait, and the cleanup stays unproved.
func TestV11047UnreadableProcessTableStillKillsPinnedGroup(t *testing.T) {
	previous := linuxGroupLive
	t.Cleanup(func() { linuxGroupLive = previous })
	linuxGroupLive = func(int) (bool, error) { return true, errors.New("injected /proc failure") }
	signals := recordGroupSignals(t)
	command, containment := startContained(t, "trap '' TERM; while :; do /bin/sleep 1; done")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	result := make(chan containmentOutcome, 1)
	go func() { result <- containment.wait(command, ctx, nil, func() bool { return true }) }()
	select {
	case outcome := <-result:
		if !outcome.timedOutOrCancelled || outcome.cleanupProven {
			t.Fatalf("outcome = %#v: want cancelled with unproved cleanup", outcome)
		}
		sent := signals()
		if len(sent) == 0 || sent[len(sent)-1] != syscall.SIGKILL {
			t.Fatalf("signals %v: want a final group SIGKILL", sent)
		}
	case <-time.After(20 * time.Second):
		containment.force()
		t.Fatal("wait blocked on a TERM-ignoring leader after a /proc failure")
	}
}
