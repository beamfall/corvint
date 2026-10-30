//go:build linux && (amd64 || arm64)

package main

// Linux step-1 quiet observation tests (stable_process_linux.go).

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestStableLinuxOwnerSupported(t *testing.T) {
	if !stableOwnerSupported() {
		t.Fatal("pidfd, epoll and /proc probes failed on a qualified Linux host")
	}
}

// linuxZombieLeader waits until the unreaped leader shows as a zombie.
func linuxZombieLeader(t *testing.T, pid int) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if e, found, ok := stableProcStat(pid); ok && found && stableProcDead(e.state) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("leader %d never became a zombie", pid)
}

// The defect this route exists for: signal 0 still succeeds on a group whose
// only member is the owner's unreaped zombie leader, while the /proc
// observation proves it quiet and the two-step retirement completes.
func TestStableLinuxZombieOnlyGroupRetires(t *testing.T) {
	cmd := startGroup(t, "exec /bin/sleep 300")
	pgid := cmd.Process.Pid
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	linuxZombieLeader(t, pgid)
	if err := syscall.Kill(-pgid, 0); err != nil {
		t.Fatalf("signal 0 on a zombie-only group: %v (Linux reports success)", err)
	}
	if !stableGroupQuiet(pgid) {
		t.Fatal("zombie-only group not observed quiet")
	}
	if !stableRetire(cmd.Process, pgid, time.Now().Add(stableRetireBound)) {
		t.Fatal("zombie-only group did not retire")
	}
	if err := syscall.Kill(-pgid, 0); err != syscall.ESRCH {
		t.Fatalf("post-reap probe: %v", err)
	}
}

// A dead leader does not make the group quiet while any member still runs:
// the observation reports live, and retirement HOLDs at the bound.
func TestStableLinuxLiveMemberBehindZombieLeaderHolds(t *testing.T) {
	pidFile := s0eTemp(t) + "/member.pid"
	cmd := startGroup(t, "trap '' TERM; /bin/sleep 300 & echo $! > "+pidFile+"; exit 0")
	pgid := cmd.Process.Pid
	defer func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	}()
	linuxZombieLeader(t, pgid)
	for i := 0; i < 400; i++ {
		if b, err := os.ReadFile(pidFile); err == nil && len(b) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if stableGroupQuiet(pgid) {
		t.Fatal("a running member was observed quiet")
	}
	if stableRetire(cmd.Process, pgid, time.Now().Add(75*time.Millisecond)) {
		t.Fatal("a group with a running member must HOLD at the bound")
	}
	// The leader stays unreaped after a HOLD, so the group ID is still pinned.
	if e, found, ok := stableProcStat(pgid); !ok || !found || !stableProcDead(e.state) {
		t.Fatal("HOLD reaped the leader")
	}
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if !stableRetire(cmd.Process, pgid, time.Now().Add(stableRetireBound)) {
		t.Fatal("killed group did not retire")
	}
	pidGone(t, pidFile)
}

// The observation refuses a group whose leader is not this owner's zombie
// child: a live leader, a foreign process and a missing one.
func TestStableLinuxQuietRequiresOwnZombieLeader(t *testing.T) {
	cmd := startGroup(t, "exec /bin/sleep 300")
	pgid := cmd.Process.Pid
	defer func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	}()
	if stableGroupQuiet(pgid) {
		t.Fatal("a live leader was observed quiet")
	}
	if stableGroupQuiet(syscall.Getpgrp()) || stableGroupQuiet(1) {
		t.Fatal("a group not led by this owner's zombie child was observed quiet")
	}
	if stableGroupQuiet(1 << 30) {
		t.Fatal("a missing leader was observed quiet")
	}
}

func TestStableLinuxProcStatParsesHostileCommandName(t *testing.T) {
	dir := s0eTemp(t)
	name := dir + "/a) Z 1 1 ("
	if err := os.Symlink("/bin/sleep", name); err != nil {
		t.Fatal(err)
	}
	cmd := startGroup(t, "exec '"+name+"' 300")
	pgid := cmd.Process.Pid
	defer func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	}()
	for i := 0; i < 400; i++ {
		if b, _, _ := stableProcRead(pgid, "comm", 64); len(b) > 0 && b[0] == 'a' {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	e, found, ok := stableProcStat(pgid)
	if !ok || !found || stableProcDead(e.state) || e.ppid != os.Getpid() || e.pgrp != pgid {
		t.Fatalf("stat of a hostile command name: %+v found %v ok %v", e, found, ok)
	}
}
