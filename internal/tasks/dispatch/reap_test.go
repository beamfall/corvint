//go:build darwin || linux

package dispatch

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// launchedGroup is one worker process group a test started: launch makes
// every worker a session and group leader, so pid is also the group ID.
type launchedGroup struct {
	pid int
	id  string
}

var launchedGroups struct {
	sync.Mutex
	all []launchedGroup
}

func init() {
	launchObserved = func(pid int, id string) {
		launchedGroups.Lock()
		launchedGroups.all = append(launchedGroups.all, launchedGroup{pid, id})
		launchedGroups.Unlock()
		deadMan.Lock()
		if deadMan.w != nil {
			_, _ = fmt.Fprintf(deadMan.w, "%d %s\n", pid, id)
		}
		deadMan.Unlock()
	}
}

// deadManEnv selects the dead-man reaper mode of this test binary.
const deadManEnv = "CORVINT_DISPATCH_TEST_DEAD_MAN"

// deadMan is the write end of the reaper's stdin pipe. Only this test
// binary holds it (os.Pipe sets close-on-exec, so no worker inherits it), so
// the kernel closes it when the binary dies, even by SIGKILL, when neither
// t.Cleanup nor TestMain runs (V1-2189, ATR-V0-213).
var deadMan struct {
	sync.Mutex
	w   *os.File
	cmd *exec.Cmd
}

// startDeadMan re-executes this test binary as a reaper in its own session,
// reading launched groups from a pipe until end of file.
func startDeadMan() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	defer r.Close()
	cmd := exec.Command(exe, "-test.run=^$")
	cmd.Env = append(os.Environ(), deadManEnv+"=1")
	cmd.Stdin = r
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		w.Close()
		return err
	}
	deadMan.Lock()
	deadMan.w, deadMan.cmd = w, cmd
	deadMan.Unlock()
	return nil
}

// stopDeadMan closes the pipe after a normal run, whose own leak check has
// already reaped every group, and waits, bounded, for the reaper.
func stopDeadMan() {
	deadMan.Lock()
	w, cmd := deadMan.w, deadMan.cmd
	deadMan.w = nil
	deadMan.Unlock()
	if w == nil {
		return
	}
	w.Close()
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
	}
}

// runDeadMan reads "pid identity" lines until end of file, which the kernel
// delivers once the test binary has exited by any means, then sends SIGKILL
// to each recorded group whose leader still carries its recorded start
// identity. A group whose leader is gone or unreadable is not attributed and
// is never signalled, as in reapLaunched.
func runDeadMan(in *os.File) {
	var groups []launchedGroup
	scan := bufio.NewScanner(in)
	for scan.Scan() {
		pid, id, ok := strings.Cut(scan.Text(), " ")
		if n, err := strconv.Atoi(pid); ok && err == nil && n > 0 && id != "" {
			groups = append(groups, launchedGroup{n, id})
		}
	}
	var killed []int
	for _, g := range groups {
		if groupOwned(g) {
			killed = append(killed, g.pid)
			_ = syscall.Kill(-g.pid, syscall.SIGKILL)
		}
	}
	// A child forked while the group signal is delivered can miss it. While
	// a killed group stays present its ID cannot name another group, so it is
	// signalled again until it empties, within a bound.
	for i := 0; i < 500 && len(killed) > 0; i++ {
		var present []int
		for _, pid := range killed {
			if syscall.Kill(-pid, syscall.SIGKILL) == nil {
				present = append(present, pid)
			}
		}
		if killed = present; len(killed) > 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func launchedSince(from int) []launchedGroup {
	launchedGroups.Lock()
	defer launchedGroups.Unlock()
	return append([]launchedGroup(nil), launchedGroups.all[from:]...)
}

func launchedCount() int {
	launchedGroups.Lock()
	defer launchedGroups.Unlock()
	return len(launchedGroups.all)
}

// groupOwned reports whether the recorded leader still runs with its
// recorded start identity. Only that positive match proves the process
// group is the one the test started: an absent leader or an unreadable
// identity proves nothing, because the pid may since name another group.
func groupOwned(g launchedGroup) bool {
	id, err := identityOf(g.pid)
	return err == nil && id != "" && id == g.id
}

func groupPresent(pid int) bool { return syscall.Kill(-pid, 0) == nil }

// reapLaunched stops every worker group recorded at or after index from
// whose leader is positively matched, with SIGKILL to the group, and waits,
// bounded, until each is empty (V1-0948, ATR-V0-183). It returns the groups
// it killed, those still present after the wait, and the unverified ones:
// a group still present after the same wait under a recorded pid whose
// leader no longer matches, which may be a leak or an unrelated group and
// is never signalled.
func reapLaunched(from int) (killed, survivors, unverified []int) {
	for _, g := range launchedSince(from) {
		switch {
		case groupOwned(g):
			killed = append(killed, g.pid)
			_ = syscall.Kill(-g.pid, syscall.SIGKILL)
		case groupPresent(g.pid):
			unverified = append(unverified, g.pid)
		}
	}
	return killed, drained(killed), drained(unverified)
}

// drained waits, bounded, for each group to empty and returns those still
// present. A group a test's own cleanup has just signalled can take a
// moment to empty, so only one that outlasts the wait is reported.
func drained(pids []int) (present []int) {
	for _, pid := range pids {
		gone := false
		for i := 0; i < 250 && !gone; i++ {
			if gone = !groupPresent(pid); !gone {
				time.Sleep(20 * time.Millisecond)
			}
		}
		if !gone {
			present = append(present, pid)
		}
	}
	return present
}

// reapOnCleanup registers a cleanup that stops and reaps every worker group
// the test launches from now on, including after t.Fatal. testConfig calls
// it, so it runs after the test's own cleanups such as Dispatcher.Close.
func reapOnCleanup(t *testing.T) {
	t.Helper()
	from := launchedCount()
	t.Cleanup(func() {
		_, left, unverified := reapLaunched(from)
		if len(left) > 0 {
			t.Errorf("V1-0948: worker process groups %v survived SIGKILL", left)
		}
		if len(unverified) > 0 {
			t.Errorf("V1-0948: worker process groups %v outlived their leaders; possible leak, not signalled", unverified)
		}
	})
}

// TestMain is the maintained leak check (ATR-V0-183): after every test has
// run its cleanups, no worker group any test launched may still run under
// its matched leader; such a group is killed and fails the package. A group
// present under a recorded pid whose leader no longer matches cannot be
// attributed, so it is reported as a possible leak and never signalled.
//
// Cleanups and TestMain never run when the binary is killed, so a dead-man
// reaper also stops the groups then (ATR-V0-213).
func TestMain(m *testing.M) {
	if os.Getenv(deadManEnv) == "1" {
		runDeadMan(os.Stdin)
		os.Exit(0)
	}
	if err := startDeadMan(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: V1-2189: start the dead-man reaper: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	killed, left, unverified := reapLaunched(0)
	stopDeadMan()
	if len(killed) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: V1-0948: dispatch tests left worker process groups alive: %v (killed; still present: %v)\n", killed, left)
		code = 1
	}
	if len(unverified) > 0 {
		fmt.Fprintf(os.Stderr, "V1-0948: process groups %v remain under recorded leader pids whose identity no longer matches; possible leak, cannot verify, not signalled\n", unverified)
	}
	os.Exit(code)
}

func TestCALV0353_CleanupReapsTheWholeWorkerGroup(t *testing.T) {
	from := launchedCount()
	pid, _, exit, err := launch([]string{"/bin/sh", "-c", "/bin/sleep 300 & /bin/sleep 300; wait"}, nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reapLaunched(from) })
	members := func() int {
		procs, err := observeProcs()
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for p, row := range procs {
			if row.pgid == pid && p != pid {
				n++
			}
		}
		return n
	}
	for i := 0; i < 250 && members() < 2; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if n := members(); n < 2 {
		t.Fatalf("worker group has %d descendants, want 2 sleeps", n)
	}
	alive, left, unverified := reapLaunched(from)
	if len(alive) != 1 || alive[0] != pid || len(unverified) != 0 {
		t.Fatalf("leak check killed %v (unverified %v), want [%d]: the TestMain check would miss this leak", alive, unverified, pid)
	}
	if len(left) > 0 || members() != 0 {
		t.Fatalf("group %d not reaped: survivors %v, %d descendants left", pid, left, members())
	}
	select {
	case <-exit:
	case <-time.After(10 * time.Second):
		t.Fatal("killed leader was not reaped")
	}
	if alive, _, unverified := reapLaunched(from); len(alive)+len(unverified) != 0 {
		t.Fatalf("leak check still sees %v (unverified %v) after the reap", alive, unverified)
	}
}

// ATR-V0-183: once the leader is gone, a group under its pid cannot be
// attributed, so the reaper reports it and never signals it.
func TestCALV0353_UnmatchedLeaderIsReportedNotSignalled(t *testing.T) {
	from := launchedCount()
	dir := t.TempDir()
	pid, _, exit, err := launch([]string{"/bin/sh", "-c", "/bin/sleep 300 & echo $! > child; exit 0"}, nil, dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-exit:
	case <-time.After(10 * time.Second):
		t.Fatal("leader did not exit")
	}
	raw, err := os.ReadFile(dir + "/child")
	if err != nil {
		t.Fatal(err)
	}
	var child int
	if _, err := fmt.Sscan(string(raw), &child); err != nil || child <= 0 {
		t.Fatalf("child pid %q: %v", raw, err)
	}
	childID, err := identityOf(child)
	if err != nil || childID == "" {
		t.Fatalf("child identity: %q %v", childID, err)
	}
	t.Cleanup(func() {
		if id, _ := identityOf(child); id == childID {
			_ = syscall.Kill(child, syscall.SIGKILL)
		}
	})
	killed, left, unverified := reapLaunched(from)
	if len(killed)+len(left) != 0 || len(unverified) != 1 || unverified[0] != pid {
		t.Fatalf("killed %v, survivors %v, unverified %v; want only %d unverified", killed, left, unverified, pid)
	}
	if id, _ := identityOf(child); id != childID {
		t.Fatal("the reaper signalled a group it could not attribute")
	}
	_ = syscall.Kill(child, syscall.SIGKILL)
	if !gone(child) {
		t.Fatal("child not reaped")
	}
	if _, _, unverified := reapLaunched(from); len(unverified) != 0 {
		t.Fatalf("unverified %v after the child exited", unverified)
	}
}
