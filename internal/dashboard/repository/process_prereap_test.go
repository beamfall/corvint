//go:build darwin || linux

package repository

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"unsafe"
)

// V1-1047: a leader that exits while a descendant survives is residue, and
// the group is stopped while the exited leader is still unreaped. The old
// Linux path reaped first and then sent SIGTERM/SIGKILL to the numeric group,
// which could by then name a reused group.
func TestResidueCleanupSignalsOnlyWhileLeaderUnreaped(t *testing.T) {
	command := exec.Command("/bin/sh", "-c", "/bin/sleep 30 & exit 0")
	containment, ok := prepareContainment(command)
	if !ok {
		t.Fatal("containment unavailable")
	}
	defer containment.close()
	var mu sync.Mutex
	var sent, afterReap []syscall.Signal
	previous := groupSignal
	t.Cleanup(func() { groupSignal = previous })
	groupSignal = func(pid int, signal syscall.Signal) error {
		mu.Lock()
		sent = append(sent, signal)
		if childReaped(-pid) {
			afterReap = append(afterReap, signal)
		}
		mu.Unlock()
		return previous(pid, signal)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if !containment.attach(command.Process) {
		containment.force()
		_, _ = command.Process.Wait()
		t.Fatal("attach containment")
	}
	outcome := containment.wait(command, context.Background(), nil, func() bool { return true })
	mu.Lock()
	defer mu.Unlock()
	if len(afterReap) != 0 {
		t.Fatalf("group signals %v were sent after the leader was reaped (all: %v)", afterReap, sent)
	}
	if !outcome.residueAfterLeader || !outcome.cleanupProven || len(sent) == 0 {
		t.Fatalf("outcome = %#v, signals %v: want residue stopped and proved", outcome, sent)
	}
	if outcome.waitErr != nil || command.ProcessState == nil {
		t.Fatalf("leader status = %v, %v: want its own exit 0", outcome.waitErr, command.ProcessState)
	}
}

// childReaped reports that pid is no longer this process's unreaped child.
func childReaped(pid int) bool {
	var info [128]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, uintptr(1), uintptr(pid),
		uintptr(unsafe.Pointer(&info[0])), uintptr(syscall.WEXITED|syscall.WNOWAIT|syscall.WNOHANG), 0, 0)
	return errno == syscall.ECHILD
}

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

// V1-1047 review P1 (Linux) and V1-1050 (Darwin): an exit observation that
// fails with ECHILD (another reaper collected the leader) leaves the numeric
// group ID unpinned, so no group signal is sent and cleanup is reported
// unproved.
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
