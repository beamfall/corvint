//go:build darwin || linux

package groupreap

import (
	"bufio"
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

// AHI-048: WaitPipes releases the group once the leader exits, before the
// reap, and leaves a member that holds no pipe of Wait's to the caller.
func TestAHI048WaitPipesReleasesWithoutKillingTheGroup(t *testing.T) {
	r := liveGroups
	command, member := startLiveWithMember(t, r)
	t.Cleanup(func() { _ = syscall.Kill(member, syscall.SIGKILL) })
	leader := command.Process.Pid
	if !r.recorded(leader) {
		t.Fatal("StartLive did not record the group")
	}
	if err := syscall.Kill(leader, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if err := WaitPipes(command); err == nil {
		t.Fatal("the killed leader exited cleanly")
	}
	if r.recorded(leader) {
		t.Fatal("WaitPipes reaped the leader without releasing its group")
	}
	if syscall.Kill(member, 0) != nil {
		t.Fatal("WaitPipes killed the rest of the group")
	}
}

// AHI-048: concurrent starts, releases and one exit kill never signal a group
// whose leader was reaped, and nothing is recorded after the kill. Run under
// -race.
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
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 10 {
				command := exec.Command("/bin/sh", "-c", "exit 0")
				command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				if err := r.start(command); errors.Is(err, ErrExiting) {
					return
				} else if err != nil {
					t.Error(err)
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
	time.Sleep(20 * time.Millisecond)
	r.kill()
	workers.Wait()
	if len(r.groups) != 0 {
		t.Fatalf("groups remain recorded after the kill: %v", r.groups)
	}
}
