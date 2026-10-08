//go:build darwin || linux

package groupreap

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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

// AHI-048: concurrent starts, releases and one exit kill never signal a group
// whose leader was reaped, and nothing is recorded after the kill. Every
// worker keeps starting until the kill refuses it, so starts and releases
// overlap the kill. Run under -race.
func TestAHI048ConcurrentStartsReleasesAndKill(t *testing.T) {
	var mu sync.Mutex
	reaped := map[int]bool{}
	r := newLiveRegistry(func(group int, signal syscall.Signal) error {
		mu.Lock()
		defer mu.Unlock()
		if reaped[-group] {
			t.Errorf("signalled the group of reaped leader %d", -group)
		}
		return syscall.Kill(group, signal)
	})
	const workers = 8
	var started sync.WaitGroup
	started.Add(workers)
	refused := make(chan bool, workers)
	for range workers {
		go func() {
			first := true
			for {
				command := exec.Command("/bin/sh", "-c", "exit 0")
				command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				err := r.start(command)
				if first {
					first = false
					started.Done()
				}
				if errors.Is(err, ErrExiting) {
					refused <- true
					return
				} else if err != nil {
					t.Error(err)
					refused <- false
					return
				}
				leader := command.Process.Pid
				// The order Wait keeps: release, then reap.
				r.release(leader)
				_ = command.Wait()
				mu.Lock()
				reaped[leader] = true
				mu.Unlock()
			}
		}()
	}
	started.Wait()
	r.kill()
	for range workers {
		if !<-refused {
			t.Fatal("a worker stopped before the kill refused it")
		}
	}
	if len(r.groups) != 0 {
		t.Fatalf("groups remain recorded after the kill: %v", r.groups)
	}
}
