//go:build darwin || linux

package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// reaperParentEnv selects the dead-man reaper mode of this test binary; the
// value is the pid of the test binary it watches (V1-2305, ATR-V0-310).
const reaperParentEnv = "CORVINT_CLI_TEST_WORKER_REAPER"

// TestRootEnv names the pid of the test binary that armed the reaper. A
// test binary re-executed by a test as a helper inherits it, sees another
// pid, and stays out of the reaping; a test that needs its own armed
// binary clears it for the child.
const TestRootEnv = "CORVINT_CLI_TEST_ROOT"

// ReaperLogEnv names a file the reaper appends "group PID" to when it first
// records a worker group, so a test of the reaper can wait until a group is
// attributed before it kills the binary.
const ReaperLogEnv = "CORVINT_CLI_TEST_REAPER_LOG"

// reaperSample is how often the reaper looks for new worker groups. A group
// started within one sample of the binary's death was never attributed and
// is not stopped by the reaper; the per-test and end-of-run checks, which
// need no sampling, are the only cover for it.
const reaperSample = 200 * time.Millisecond

// reaperPID is the dead-man reaper this binary started, a session leader
// that is a child of the binary and must not be counted as a leaked worker.
var reaperPID int

type procRow struct{ pid, ppid, pgid int }

// processRows reads the process table: every live process's pid, parent and
// process group. Zombies are left out, as an exited child that the owning
// goroutine has not yet waited for is not running.
func processRows() ([]procRow, error) {
	if runtime.GOOS == "linux" {
		return procfsRows()
	}
	ps := "/bin/ps"
	if _, err := os.Stat(ps); err != nil {
		ps = "/usr/bin/ps"
	}
	raw, err := exec.Command(ps, "-axo", "pid=,ppid=,pgid=,stat=").Output()
	if err != nil {
		return nil, err
	}
	var rows []procRow
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) < 4 || strings.HasPrefix(f[3], "Z") {
			continue
		}
		p, e1 := strconv.Atoi(f[0])
		pp, e2 := strconv.Atoi(f[1])
		pg, e3 := strconv.Atoi(f[2])
		if e1 != nil || e2 != nil || e3 != nil {
			return nil, fmt.Errorf("unparsable process row %q", line)
		}
		rows = append(rows, procRow{p, pp, pg})
	}
	return rows, nil
}

// procfsRows is processRows from /proc, which the reaper samples every
// reaperSample without starting a process. A process that ends while the
// table is read is skipped.
func procfsRows() ([]procRow, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var rows []procRow
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		// pid (comm) state ppid pgrp ...; comm may hold spaces and parentheses.
		text := string(raw)
		f := strings.Fields(text[strings.LastIndexByte(text, ')')+1:])
		if len(f) < 3 || f[0] == "Z" {
			continue
		}
		pp, e1 := strconv.Atoi(f[1])
		pg, e2 := strconv.Atoi(f[2])
		if e1 != nil || e2 != nil {
			continue
		}
		rows = append(rows, procRow{pid, pp, pg})
	}
	return rows, nil
}

// childLeaders returns the live children of parent that lead their own
// process group, which is how the dispatcher (a session leader per worker)
// and the detached runners start the trees that outlive a command.
func childLeaders(parent int) ([]int, error) {
	rows, err := processRows()
	if err != nil {
		return nil, err
	}
	var out []int
	for _, r := range rows {
		if r.ppid == parent && r.pgid == r.pid && r.pid != reaperPID {
			out = append(out, r.pid)
		}
	}
	return out, nil
}

// groupPresent reports whether a live, non-zombie process is in the group. A
// zombie keeps kill(-pgid, 0) succeeding until its parent or init reaps it,
// which not every host's init does.
func groupPresent(pgid int) bool {
	rows, err := processRows()
	if err != nil {
		return syscall.Kill(-pgid, 0) == nil
	}
	for _, r := range rows {
		if r.pgid == pgid {
			return true
		}
	}
	return false
}

// killGroup sends SIGKILL to the group until it is empty, within a bound.
// A child that forks while the signal is delivered can miss it, so the
// signal repeats while the group stays present; a present group's ID cannot
// name another group, so this never signals a stranger.
func killGroup(pgid int) (gone bool) {
	for i := 0; i < 500; i++ {
		if syscall.Kill(-pgid, syscall.SIGKILL) != nil || !groupPresent(pgid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !groupPresent(pgid)
}

// reapChildGroups kills every group led by a child of this binary that is
// not in keep, and returns the pids it found and those it could not empty.
func reapChildGroups(keep map[int]bool) (found, stuck []int) {
	leaders, err := childLeaders(os.Getpid())
	if err != nil {
		fmt.Fprintf(os.Stderr, "V1-2305: the process table cannot be read, so worker groups are not checked: %v\n", err)
		return nil, nil
	}
	for _, pid := range leaders {
		if keep[pid] {
			continue
		}
		found = append(found, pid)
		if !killGroup(pid) {
			stuck = append(stuck, pid)
		}
	}
	return found, stuck
}

// ReapWorkersOnCleanup makes the test own the process groups it starts
// from now on: when it ends, by return, t.Fatal or t.FailNow, every group
// led by a child of the test binary that was not running at this call is
// killed and fails the test (V1-2305, ATR-V0-310). Call it before the
// test's own cleanups are registered, so it runs after them. A binary that
// dies instead of ending the test is covered by the dead-man reaper. Do not
// use it in a test that runs in parallel with another that starts groups.
func ReapWorkersOnCleanup(t *testing.T) {
	t.Helper()
	keep := map[int]bool{}
	if leaders, err := childLeaders(os.Getpid()); err == nil {
		for _, pid := range leaders {
			keep[pid] = true
		}
	}
	t.Cleanup(func() {
		found, stuck := reapChildGroups(keep)
		if len(found) > 0 {
			t.Errorf("V1-2305: worker process groups %v outlived the test; killed (still present: %v)", found, stuck)
		}
	})
}

// startWorkerReaper re-executes this test binary as a reaper in its own
// session. Its stdin is the read end of a pipe whose close-on-exec write
// end only this binary holds, so the kernel closes it when the binary dies
// by any means, including SIGKILL or the package timeout, when no cleanup
// and no TestMain code after m.Run runs.
func startWorkerReaper() (*os.File, *exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	cmd := exec.Command(exe, "-test.run=^$")
	cmd.Env = append(os.Environ(), reaperParentEnv+"="+strconv.Itoa(os.Getpid()))
	cmd.Stdin = r
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		w.Close()
		return nil, nil, err
	}
	reaperPID = cmd.Process.Pid
	return w, cmd, nil
}

// stopWorkerReaper closes the pipe after a normal run, whose own leak check
// has already reaped every group, and waits, bounded, for the reaper.
func stopWorkerReaper(w *os.File, cmd *exec.Cmd) {
	w.Close()
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
	}
}

// runWorkerReaper follows the children of parent until its stdin reaches end
// of file, which the kernel delivers once parent has exited by any means. It
// records each child that leads a process group with its start identity, and
// the members of those groups, then kills every recorded group whose leader
// still carries its recorded identity and every recorded member that still
// does. A leader or member that is gone or unreadable proves nothing, since
// its pid may since name another process, and is never signalled.
func runWorkerReaper(parent int, in io.Reader) int {
	self := os.Getpid()
	leaders := map[int]string{}
	members := map[int]string{}
	eof := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, in); close(eof) }()
	identity := func(pid int) string {
		id, _ := supervisor.ProcessIdentity(pid)
		return id
	}
	sample := func() {
		rows, err := processRows()
		if err != nil {
			return
		}
		for _, r := range rows {
			if _, ok := leaders[r.pid]; ok || r.ppid != parent || r.pgid != r.pid || r.pid == self {
				continue
			}
			if id := identity(r.pid); id != "" {
				leaders[r.pid] = id
				if p := os.Getenv(ReaperLogEnv); p != "" {
					if f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600); err == nil {
						fmt.Fprintf(f, "group %d\n", r.pid)
						f.Close()
					}
				}
			}
		}
		for _, r := range rows {
			id, tracked := leaders[r.pgid]
			if _, ok := members[r.pid]; !tracked || ok || r.pid == r.pgid || identity(r.pgid) != id {
				continue
			}
			if m := identity(r.pid); m != "" {
				members[r.pid] = m
			}
		}
	}
	for {
		select {
		case <-eof:
			var killed []int
			for pid, id := range leaders {
				if identity(pid) == id {
					killed = append(killed, pid)
					_ = syscall.Kill(-pid, syscall.SIGKILL)
				}
			}
			for pid, id := range members {
				if identity(pid) == id {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			}
			for _, pid := range killed {
				killGroup(pid)
			}
			return 0
		case <-time.After(reaperSample):
			sample()
		}
	}
}

// testMain arms the dead-man reaper for the binary and, after the run, fails
// the package if any test left a process group running under a child of the
// binary. Cleanups and the code after m.Run never run when the binary is
// killed, so the reaper stops the groups then (V1-2305, ATR-V0-310; the same
// ownership rule as the dispatch package's V1-2189). It returns the exit code.
func testMain(m *testing.M) int {
	if v := os.Getenv(reaperParentEnv); v != "" {
		parent, _ := strconv.Atoi(v)
		os.Exit(runWorkerReaper(parent, os.Stdin))
	}
	if root := os.Getenv(TestRootEnv); root != "" && root != strconv.Itoa(os.Getpid()) {
		return m.Run() // a helper re-execution of this binary
	}
	if err := os.Setenv(TestRootEnv, strconv.Itoa(os.Getpid())); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: V1-2305: %v\n", err)
		return 1
	}
	w, cmd, err := startWorkerReaper()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: V1-2305: start the dead-man reaper: %v\n", err)
		return 1
	}
	code := m.Run()
	found, stuck := reapChildGroups(nil)
	stopWorkerReaper(w, cmd)
	if len(found) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: V1-2305: tests left worker process groups running: %v (killed; still present: %v)\n", found, stuck)
		code = 1
	}
	return code
}
