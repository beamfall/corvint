//go:build darwin || linux

package groupreap

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// leaderNotReaped reports, without blocking, whether pid is still a child this
// process has not reaped (running, stopped, or an exited zombie): nil, or the
// waitid error (ECHILD once it has been reaped).
func leaderNotReaped(pid int) error {
	var info [128]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, uintptr(1), uintptr(pid),
		uintptr(unsafe.Pointer(&info[0])), uintptr(syscall.WEXITED|syscall.WNOWAIT|syscall.WNOHANG), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// groupSignals records every group SIGKILL sent to -leader and, for each,
// whether the leader was still unreaped at that instant (nil: unreaped).
type groupSignals struct {
	mu       sync.Mutex
	observed []error
}

func hookGroupSignals(t *testing.T, leader func() int) *groupSignals {
	t.Helper()
	record := &groupSignals{}
	previous := signalGroup
	t.Cleanup(func() { signalGroup = previous })
	signalGroup = func(processID int, signal syscall.Signal) error {
		if processID < 0 && processID == -leader() && signal == syscall.SIGKILL {
			observed := leaderNotReaped(-processID)
			record.mu.Lock()
			record.observed = append(record.observed, observed)
			record.mu.Unlock()
		}
		return previous(processID, signal)
	}
	return record
}

func (g *groupSignals) snapshot() []error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]error(nil), g.observed...)
}

// requirePreReap fails unless at least one group SIGKILL was sent and every
// one of them reached the group while its exited leader was still unreaped.
func requirePreReap(t *testing.T, g *groupSignals) {
	t.Helper()
	observed := g.snapshot()
	if len(observed) == 0 {
		t.Fatal("process group was not signalled")
	}
	for _, err := range observed {
		if err != nil {
			t.Fatalf("group signalled when the leader was not held unreaped: %v", err)
		}
	}
}

func startedPID(command *exec.Cmd) func() int {
	return func() int {
		if command.Process == nil {
			return 0
		}
		return command.Process.Pid
	}
}

// Drain sweeps the group only while the exited leader is still unreaped.
func TestDrainSignalsGroupBeforeLeaderIsReaped(t *testing.T) {
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "echo out; exit 0")
	Contain(command)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	signals := hookGroupSignals(t, startedPID(command))
	wait, err := Drain(context.Background(), command, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := wait(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "out\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	requirePreReap(t, signals)
}

// Output a group member writes after the leader exits, within the bound, is
// drained before the sweep, so the capture is complete.
func TestDrainCapturesOutputBeforeSweep(t *testing.T) {
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "(sleep 0.3; echo late) & exit 0")
	Contain(command)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	signals := hookGroupSignals(t, startedPID(command))
	wait, err := Drain(context.Background(), command, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := wait(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "late\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	requirePreReap(t, signals)
}

// A group member that still holds a pipe past the bound makes the capture
// incomplete (exec.ErrWaitDelay) and is killed by the pre-reap sweep.
func TestDrainPipeHolderIsIncompleteCaptureAndKilled(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "holder")
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", `echo early; sleep 60 & echo $! > "$0"; exit 0`, pidFile)
	Contain(command)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	signals := hookGroupSignals(t, startedPID(command))
	wait, err := Drain(context.Background(), command, 300*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = wait()
	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("wait = %v, want exec.ErrWaitDelay", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("wait took %s", elapsed)
	}
	if stdout.String() != "early\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	requirePreReap(t, signals)
	data, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	holder, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if convErr != nil {
		t.Fatal(convErr)
	}
	deadline := time.Now().Add(10 * time.Second)
	for syscall.Kill(holder, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(holder, syscall.SIGKILL)
			t.Fatalf("pipe holder %d survived the group sweep", holder)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Cancellation sweeps a running group at once, before the leader is reaped.
func TestDrainCancellationSignalsGroupPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 60 & sleep 60")
	Contain(command)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	signals := hookGroupSignals(t, startedPID(command))
	wait, err := Drain(ctx, command, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(100*time.Millisecond, cancel)
	started := time.Now()
	if err := wait(); err == nil {
		t.Fatal("cancelled command reported success")
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("cancelled wait took %s", elapsed)
	}
	requirePreReap(t, signals)
}

// A failed exit observation (for example ECHILD after another reaper took the
// leader) sends no group signal from Wait, Drain or RunContained (V1-0652).
func TestFailedExitObservationSendsNoGroupSignal(t *testing.T) {
	previous := observeExit
	t.Cleanup(func() { observeExit = previous })
	observeExit = func(int) error { return syscall.ECHILD }

	t.Run("Wait", func(t *testing.T) {
		command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
		Contain(command)
		signals := hookGroupSignals(t, startedPID(command))
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		if err := Wait(command); err != nil {
			t.Fatal(err)
		}
		if got := signals.snapshot(); len(got) != 0 {
			t.Fatalf("group signalled %d times after a failed observation", len(got))
		}
	})
	t.Run("Drain", func(t *testing.T) {
		command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "echo out; exit 0")
		Contain(command)
		var stdout bytes.Buffer
		command.Stdout = &stdout
		signals := hookGroupSignals(t, startedPID(command))
		wait, err := Drain(context.Background(), command, 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if err := wait(); err != nil {
			t.Fatal(err)
		}
		if stdout.String() != "out\n" {
			t.Fatalf("stdout = %q", stdout.String())
		}
		if got := signals.snapshot(); len(got) != 0 {
			t.Fatalf("group signalled %d times after a failed observation", len(got))
		}
	})
	t.Run("RunContained", func(t *testing.T) {
		command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
		signals := hookGroupSignals(t, startedPID(command))
		containment, err := RunContained(command)
		if err != nil {
			t.Fatal(err)
		}
		if containment.Complete() {
			t.Fatal("containment without an exit observation reported complete")
		}
		if got := signals.snapshot(); len(got) != 0 {
			t.Fatalf("group signalled %d times after a failed observation", len(got))
		}
	})
}

// A leader reaped by a foreign wait4 is not signalled through its group.
func TestWaitAfterForeignReapSendsNoGroupSignal(t *testing.T) {
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
	Contain(command)
	signals := hookGroupSignals(t, startedPID(command))
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(command.Process.Pid, &status, 0, nil); err != nil {
		t.Fatal(err)
	}
	if err := Wait(command); err == nil {
		t.Fatal("Wait after a foreign reap reported success")
	}
	if got := signals.snapshot(); len(got) != 0 {
		t.Fatalf("group signalled %d times after a foreign reap", len(got))
	}
}

// Stop after the reap is refused by os.Process and signals no group.
func TestStopAfterReapSendsNoGroupSignal(t *testing.T) {
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
	Contain(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if err := Wait(command); err != nil {
		t.Fatal(err)
	}
	signals := hookGroupSignals(t, startedPID(command))
	if err := Stop(command); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("Stop after reap = %v, want os.ErrProcessDone", err)
	}
	if got := signals.snapshot(); len(got) != 0 {
		t.Fatalf("Stop signalled the group %d times after the reap", len(got))
	}
}

// Context cancellation through Contain stops the leader; Wait then sweeps the
// rest of the group while the leader is unreaped.
func TestContainCancellationSweepsGroupBeforeReap(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "member")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, "/bin/sh", "-c", `sleep 60 & echo $! > "$0"; wait`, pidFile)
	Contain(command)
	signals := hookGroupSignals(t, startedPID(command))
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	var member int
	for member == 0 {
		if data, err := os.ReadFile(pidFile); err == nil && strings.HasSuffix(string(data), "\n") {
			member, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		if time.Now().After(deadline) {
			t.Fatal("group member did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := Wait(command); err == nil {
		t.Fatal("cancelled command reported success")
	}
	requirePreReap(t, signals)
	for syscall.Kill(member, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(member, syscall.SIGKILL)
			t.Fatalf("group member %d survived cancellation", member)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A stopped or continued leader is not reported as exited: on darwin waitid
// with WEXITED|WNOWAIT also returns for those state changes (golang/go#19314),
// and treating that as an exit would sweep the group while the leader lives.
func TestLeaderUnreapedIgnoresStopAndContinue(t *testing.T) {
	command := exec.CommandContext(context.Background(), "/bin/sleep", "60")
	Contain(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	leader := command.Process.Pid
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	if err := syscall.Kill(leader, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !processStopped(t, leader) {
		if time.Now().After(deadline) {
			t.Fatal("leader did not stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
	returned := make(chan error, 1)
	go func() { returned <- leaderUnreaped(leader) }()
	select {
	case err := <-returned:
		t.Fatalf("leaderUnreaped returned for a stopped leader: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := syscall.Kill(leader, syscall.SIGCONT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		t.Fatalf("leaderUnreaped returned for a continued leader: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := syscall.Kill(leader, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("leaderUnreaped after exit = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("leaderUnreaped did not observe the exit")
	}
}

func processStopped(t *testing.T, pid int) bool {
	t.Helper()
	out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "T")
}
