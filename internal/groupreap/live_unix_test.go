//go:build darwin || linux

package groupreap

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func newLiveRegistry(signal func(int, syscall.Signal) error) *liveRegistry {
	return &liveRegistry{groups: map[int]struct{}{}, signal: signal}
}

func (r *liveRegistry) recorded(leader int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.groups[leader]
	return ok
}

// startLiveWithMember starts a Setpgid leader through r that leaves one
// sleeping group member and prints that member's process ID.
func startLiveWithMember(t *testing.T, r *liveRegistry) (*exec.Cmd, int) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $!; wait")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Stdout = write
	err = r.start(command)
	write.Close()
	if err != nil {
		read.Close()
		t.Fatal(err)
	}
	line, err := bufio.NewReader(read).ReadString('\n')
	read.Close()
	if err != nil {
		t.Fatal(err)
	}
	member, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	return command, member
}

// AHI-048: the exit path kills a recorded group, leader and descendant, without
// waiting for any reap, and refuses a start after it.
func TestAHI048KillLiveRetiresRecordedGroups(t *testing.T) {
	r := newLiveRegistry(syscall.Kill)
	command, member := startLiveWithMember(t, r)
	if !r.recorded(command.Process.Pid) {
		t.Fatal("a started Setpgid leader was not recorded")
	}
	r.kill()
	requireGone(t, member)
	if err := command.Wait(); err == nil {
		t.Fatal("the killed leader exited cleanly")
	}
	late := exec.Command("/bin/sh", "-c", "exit 0")
	late.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := r.start(late); !errors.Is(err, ErrExiting) || late.Process != nil {
		t.Fatalf("a start after the exit kill = %v, process %v", err, late.Process)
	}
}

// AHI-048: only a group the command leads is recorded; a child that stays in
// this process's group (an owned worker's Git) or joins another is not.
func TestAHI048StartLiveRecordsOnlyOwnGroups(t *testing.T) {
	r := newLiveRegistry(func(int, syscall.Signal) error { t.Error("nothing recorded should be signalled"); return nil })
	for _, attributes := range []*syscall.SysProcAttr{nil, {}, {Setpgid: true, Pgid: syscall.Getpgrp()}} {
		command := exec.Command("/bin/sh", "-c", "exit 0")
		command.SysProcAttr = attributes
		if err := r.start(command); err != nil {
			t.Fatal(err)
		}
		if r.recorded(command.Process.Pid) {
			t.Fatalf("recorded a child that leads no group of its own: %+v", attributes)
		}
		_ = command.Wait()
	}
	r.kill()
}

// AHI-048: Wait and an Owner release the group while the exited leader is
// still unreaped, so the exit kill never names a reaped leader's group.
func TestAHI048WaitAndOwnerReleaseBeforeReap(t *testing.T) {
	command := exec.Command("/bin/sh", "-c", "exit 0")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := StartLive(command); err != nil {
		t.Fatal(err)
	}
	leader := command.Process.Pid
	if !liveGroups.recorded(leader) {
		t.Fatal("StartLive did not record the group")
	}
	previous := signalGroup
	t.Cleanup(func() { signalGroup = previous })
	recordedAtSignal := false
	signalGroup = func(processID int, signal syscall.Signal) error {
		if processID == -leader {
			recordedAtSignal = liveGroups.recorded(leader)
		}
		return previous(processID, signal)
	}
	// exited runs after the group signal and before the reap.
	releasedBeforeReap := false
	if err := wait(command, func() { releasedBeforeReap = !liveGroups.recorded(leader) }); err != nil {
		t.Fatal(err)
	}
	if !recordedAtSignal {
		t.Fatal("the group was released before the leader exited")
	}
	if !releasedBeforeReap || liveGroups.recorded(leader) {
		t.Fatal("Wait reaped the leader before releasing its group")
	}

	owned, err := Start(exec.Command("/bin/sh", "-c", "exit 0"))
	if err != nil {
		t.Fatal(err)
	}
	if !liveGroups.recorded(owned.leader) {
		t.Fatal("an Owner start did not record its group")
	}
	owned.Finish(bounded(10 * time.Second))
	if liveGroups.recorded(owned.leader) {
		t.Fatal("an Owner reaped its leader without releasing its group")
	}
}

// AHI-048 with PGO-V0-008: Drain (which replaced WaitPipes when V1-0373 and
// V1-0734 were integrated) records the group from start through the pipe drain
// and its pre-reap sweep, and releases it only after that sweep and before the
// reap. A member still holding a pipe is reported as incomplete capture and
// killed by the sweep, which WaitPipes left to the caller's post-reap kill.
func TestAHI048DrainKeepsGroupRecordedUntilReap(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", "sleep 300 & echo $! >&3; exit 0")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.ExtraFiles = []*os.File{write}
	var stdout bytes.Buffer
	command.Stdout = &stdout
	wait, err := Drain(context.Background(), command, 300*time.Millisecond)
	write.Close()
	if err != nil {
		read.Close()
		t.Fatal(err)
	}
	leader := command.Process.Pid
	line, err := bufio.NewReader(read).ReadString('\n')
	read.Close()
	if err != nil {
		t.Fatal(err)
	}
	member, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(member, syscall.SIGKILL) })
	if !liveGroups.recorded(leader) {
		t.Fatal("Drain did not record the group")
	}
	previous := signalGroup
	t.Cleanup(func() { signalGroup = previous })
	sweeps, recordedAtSweep := 0, true
	signalGroup = func(processID int, signal syscall.Signal) error {
		if processID == -leader {
			sweeps++
			recordedAtSweep = recordedAtSweep && liveGroups.recorded(leader)
			if leaderNotReaped(leader) != nil {
				t.Error("Drain signalled the group after the leader was reaped")
			}
		}
		return previous(processID, signal)
	}
	if err := wait(); !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("wait = %v, want exec.ErrWaitDelay", err)
	}
	if sweeps == 0 || !recordedAtSweep {
		t.Fatalf("the group was released before the pre-reap sweep (sweeps %d)", sweeps)
	}
	if liveGroups.recorded(leader) {
		t.Fatal("Drain reaped the leader without releasing its group")
	}
	requireGone(t, member)
}

// Contain serves both a context-bound Git spawn (V1-0373: Cancel becomes Stop)
// and a plain exec.Command started through StartLive (V1-0734): the latter
// keeps a nil Cancel, which exec.Cmd.Start requires, and is still recorded.
func TestContainKeepsPlainCommandStartable(t *testing.T) {
	command := exec.Command("/bin/sh", "-c", "exit 0")
	Contain(command)
	if command.Cancel != nil || command.SysProcAttr == nil || !command.SysProcAttr.Setpgid {
		t.Fatalf("Contain on a plain command: Cancel set %v, attributes %+v", command.Cancel != nil, command.SysProcAttr)
	}
	if err := StartLive(command); err != nil {
		t.Fatal(err)
	}
	if !liveGroups.recorded(command.Process.Pid) {
		t.Fatal("a contained plain command was not recorded")
	}
	if err := Wait(command); err != nil {
		t.Fatal(err)
	}
	withContext := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
	Contain(withContext)
	if withContext.Cancel == nil {
		t.Fatal("Contain dropped the context cancellation")
	}
}

// AHI-048: concurrent starts, releases and one exit kill never signal a group
// whose leader was reaped, and nothing is recorded after the kill (V1-1041).
// Workers start through the registry and reap through the production wait,
// so each release precedes its reap exactly as Wait orders them. A barrier
// proves the first starts overlap: every first group is recorded at once. Half
// of those first children stay blocked on stdin, so the kill must signal live
// recorded groups; the other half are released at the kill and keep starting
// and releasing until refused. While the kill holds the gate, its first signal
// waits until a release (the waiter of a recorded group) and a competing start
// are both blocked on the gate, so the race is exercised on every run, even
// with GOMAXPROCS=1. Run under -race.
func TestAHI048ConcurrentStartsReleasesAndKill(t *testing.T) {
	const workers = 8
	var mu sync.Mutex
	reaped := map[int]bool{}
	signalled := map[int]bool{}
	var probing atomic.Bool
	killing, stop := make(chan struct{}), make(chan struct{})
	var releaseBlocked, startBlocked bool
	r := newLiveRegistry(func(group int, signal syscall.Signal) error {
		mu.Lock()
		if reaped[-group] {
			t.Errorf("signalled the group of reaped leader %d", -group)
		}
		signalled[-group] = true
		mu.Unlock()
		err := syscall.Kill(group, signal)
		if probing.CompareAndSwap(true, false) {
			close(killing)
			// A hang detector, not a budget (decision 0082).
			releaseBlocked, startBlocked = awaitGateWaiters(10 * time.Second)
		}
		return err
	})
	previous := liveGroups
	liveGroups = r
	t.Cleanup(func() { liveGroups = previous })

	type pipe struct{ read, write *os.File }
	pipes := make([]pipe, workers)
	for i := range pipes {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		pipes[i] = pipe{read, write}
	}
	proceed := make(chan struct{})
	var proceedOnce sync.Once
	letProceed := func() { proceedOnce.Do(func() { close(proceed) }) }
	var joined sync.WaitGroup
	// Registered before any worker starts: refuse further starts, kill every
	// recorded group, unblock every first child and join every goroutine.
	t.Cleanup(func() {
		probing.Store(false)
		close(stop)
		r.kill()
		letProceed()
		for _, p := range pipes {
			_ = p.read.Close()
			_ = p.write.Close()
		}
		done := make(chan struct{})
		go func() { joined.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(30 * time.Second): // hang detector (decision 0082)
			t.Error("workers did not finish after cleanup")
		}
	})

	reap := func(command *exec.Cmd) {
		_ = wait(command, nil)
		mu.Lock()
		reaped[command.Process.Pid] = true
		mu.Unlock()
	}
	type first struct{ worker, leader int }
	firsts := make(chan first, workers)
	refused := make(chan error, workers)
	joined.Add(workers)
	for i := range workers {
		go func() {
			defer joined.Done()
			command := exec.Command("/bin/sh", "-c", "read _")
			command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			command.Stdin = pipes[i].read
			err := r.start(command)
			_ = pipes[i].read.Close()
			if err != nil {
				refused <- err
				return
			}
			firsts <- first{worker: i, leader: command.Process.Pid}
			<-proceed
			reap(command)
			for {
				command := exec.Command("/bin/sh", "-c", "exit 0")
				command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				if err := r.start(command); err != nil {
					refused <- err
					return
				}
				reap(command)
			}
		}()
	}
	// The competing start: issued only once the kill holds the gate.
	competing := make(chan error, 1)
	joined.Add(1)
	go func() {
		defer joined.Done()
		select {
		case <-killing:
		case <-stop:
			return
		}
		command := exec.Command("/bin/sh", "-c", "exit 0")
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		err := r.start(command)
		if err == nil {
			reap(command)
		}
		competing <- err
	}()

	leaders := make([]int, workers)
	for range workers {
		select {
		case started := <-firsts:
			leaders[started.worker] = started.leader
		case err := <-refused:
			t.Fatalf("a worker failed its first start: %v", err)
		case <-time.After(30 * time.Second): // hang detector (decision 0082)
			t.Fatal("first starts did not complete")
		}
	}
	r.mu.Lock()
	overlapping := len(r.groups)
	r.mu.Unlock()
	if overlapping != workers {
		t.Fatalf("%d groups recorded at the barrier, want all %d first starts to overlap", overlapping, workers)
	}
	letProceed()
	for _, p := range pipes[:workers/2] {
		_ = p.write.Close()
	}
	probing.Store(true)
	r.kill()
	if !releaseBlocked || !startBlocked {
		t.Fatalf("the kill ran without a concurrent release (%v) and start (%v) blocked on the gate", releaseBlocked, startBlocked)
	}
	select {
	case err := <-competing:
		if !errors.Is(err, ErrExiting) {
			t.Fatalf("the start competing with the kill = %v, want ErrExiting", err)
		}
	case <-time.After(30 * time.Second): // hang detector (decision 0082)
		t.Fatal("the competing start did not return")
	}
	for range workers {
		select {
		case err := <-refused:
			if !errors.Is(err, ErrExiting) {
				t.Fatalf("a worker stopped before the kill refused it: %v", err)
			}
		case <-time.After(30 * time.Second): // hang detector (decision 0082)
			t.Fatal("a worker was not refused after the kill")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, leader := range leaders[workers/2:] {
		if !signalled[leader] {
			t.Fatalf("the kill did not signal live recorded group %d", leader)
		}
	}
	if len(r.groups) != 0 {
		t.Fatalf("groups remain recorded after the kill: %v", r.groups)
	}
}

// awaitGateWaiters reports whether, before timeout, goroutine stacks show a
// liveRegistry release and a liveRegistry start both blocked acquiring the
// registry gate.
func awaitGateWaiters(timeout time.Duration) (release, start bool) {
	deadline := time.Now().Add(timeout)
	buffer := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buffer, true)
		if n == len(buffer) {
			buffer = make([]byte, 2*len(buffer))
			continue
		}
		for _, goroutine := range strings.Split(string(buffer[:n]), "\n\n") {
			header, _, _ := strings.Cut(goroutine, "\n")
			if !strings.Contains(header, "[sync.RWMutex.RLock") {
				continue
			}
			release = release || strings.Contains(goroutine, "(*liveRegistry).release(")
			start = start || strings.Contains(goroutine, "(*liveRegistry).start(")
		}
		if (release && start) || time.Now().After(deadline) {
			return release, start
		}
		time.Sleep(time.Millisecond)
	}
}
