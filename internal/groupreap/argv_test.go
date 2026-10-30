//go:build darwin || linux

package groupreap

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// WQO-V0-052: a group leader is found by an exact argument and its group is
// stopped and killed under identity pinning; a marked non-leader is not found.
func TestGroupLeadersWithArgumentAndStopKillGroup(t *testing.T) {
	const marker = "corvint.groupreaptest=7f3a9c"
	start := func(leader bool) (*exec.Cmd, chan error) {
		command := exec.Command("/bin/sh", "-c", "sleep 600; :", marker)
		if leader {
			command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		t.Cleanup(func() { _ = command.Process.Kill() })
		return command, done
	}
	leader, leaderDone := start(true)
	follower, _ := start(false)
	// The marker is readable once exec completes; the deadline is a hang detector.
	deadline := time.Now().Add(time.Minute)
	var found []Process
	for {
		var err error
		if found, err = GroupLeadersWithArgument(marker); err != nil {
			t.Fatal(err)
		}
		if len(found) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(found) != 1 || found[0].PID != leader.Process.Pid {
		t.Fatalf("leaders %v, want only %d (follower %d)", found, leader.Process.Pid, follower.Process.Pid)
	}
	if err := StopKillGroup(found[0]); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-leaderDone:
		if status, ok := err.(*exec.ExitError); !ok || status.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
			t.Fatalf("leader ended with %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("killed group leader did not exit")
	}
	if err := syscall.Kill(follower.Process.Pid, 0); err != nil {
		t.Fatalf("unmarked-group process was signalled: %v", err)
	}
}
