//go:build darwin || linux

package dispatch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

const platformSupported = true

// proc is one row of a process table observation.
type proc struct {
	pid, ppid, pgid int
	comm            string
}

// lockExclusive takes the program's single-dispatcher lock without waiting.
func lockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// observeProcs reads the live (non-zombie) process table.
func observeProcs() (map[int]proc, error) {
	ps := "/bin/ps"
	if _, err := os.Stat(ps); err != nil {
		ps = "/usr/bin/ps" // Linux without a merged /usr
	}
	raw, err := exec.Command(ps, "-axo", "pid=,ppid=,pgid=,stat=,comm=").Output()
	if err != nil {
		return nil, err
	}
	out := map[int]proc{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || strings.HasPrefix(f[3], "Z") {
			continue
		}
		var p proc
		var e1, e2, e3 error
		p.pid, e1 = strconv.Atoi(f[0])
		p.ppid, e2 = strconv.Atoi(f[1])
		p.pgid, e3 = strconv.Atoi(f[2])
		if e1 != nil || e2 != nil || e3 != nil {
			return nil, fmt.Errorf("unparsable process row %q", line)
		}
		p.comm = filepath.Base(strings.Join(f[4:], " "))
		out[p.pid] = p
	}
	return out, nil
}

// processIdentity reads the tree's start identities; tests replace it to
// inject an unreadable process table.
var processIdentity = supervisor.ProcessIdentity

// launch starts argv as a new session leader so it outlives the dispatcher,
// with output appended to the worker's log files.
func launch(argv, env []string, dir, logDir string) (int, string, <-chan int, error) {
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return 0, "", nil, err
	}
	open := func(name string) (*os.File, error) {
		return os.OpenFile(filepath.Join(logDir, name), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	}
	stdout, err := open("stdout.log")
	if err != nil {
		return 0, "", nil, err
	}
	defer stdout.Close()
	stderr, err := open("stderr.log")
	if err != nil {
		return 0, "", nil, err
	}
	defer stderr.Close()
	null, err := os.Open(os.DevNull)
	if err != nil {
		return 0, "", nil, err
	}
	defer null.Close()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, stdout, stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, "", nil, err
	}
	pid := cmd.Process.Pid
	// Read the identity before anything can reap the leader: until Wait
	// runs, an exited leader stays a zombie whose identity is still readable.
	id, err := supervisor.ProcessIdentity(pid)
	if err == nil && id == "" {
		err = fmt.Errorf("pid %d has no start identity", pid)
	}
	exit := make(chan int, 1)
	go func() {
		_ = cmd.Wait()
		exit <- cmd.ProcessState.ExitCode()
	}()
	if err != nil {
		// An unidentified tree cannot be supervised safely; stop it now.
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		return 0, "", nil, fmt.Errorf("worker start identity unreadable: %w", err)
	}
	return pid, id, exit, nil
}

// refreshTree recomputes a worker's process tree (CAL-V0-052): recorded
// members whose start identity still matches, then, to a fixed point, their
// children, members of their process groups, and processes in the leader's
// session. Session expansion is disabled once the session ID names a
// different process, so PID reuse cannot pull in an unrelated session. An
// unreadable identity leaves the recorded tree unchanged and returns the
// error: an unobservable member is never taken for a gone one.
func refreshTree(w *Worker, procs map[int]proc) error {
	members := map[int]string{}
	for _, m := range w.Members {
		if _, ok := procs[m.PID]; ok {
			id, err := processIdentity(m.PID)
			if err != nil {
				return err
			}
			if id != "" && id == m.Identity {
				members[m.PID] = id
			}
		}
	}
	session := w.LeaderIdentity != ""
	if session {
		// Checked live rather than from the snapshot, so a leader PID reused
		// after the snapshot also disables session expansion.
		id, err := processIdentity(w.PID)
		if err != nil {
			return err
		}
		session = id == "" || id == w.LeaderIdentity
	}
	pids := make([]int, 0, len(procs))
	for pid := range procs {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	for changed := true; changed; {
		changed = false
		groups := map[int]bool{}
		for pid := range members {
			groups[procs[pid].pgid] = true
		}
		for _, pid := range pids {
			p := procs[pid]
			if _, ok := members[pid]; ok || pid == os.Getpid() {
				continue
			}
			_, parent := members[p.ppid]
			in := parent || groups[p.pgid]
			if !in && session {
				sid, err := getsid(pid)
				in = err == nil && sid == w.PID
			}
			if !in {
				continue
			}
			id, err := processIdentity(pid)
			if err != nil {
				return err
			}
			if id != "" {
				members[pid] = id
				changed = true
			}
		}
	}
	w.Members = w.Members[:0]
	for pid, id := range members {
		w.Members = append(w.Members, Proc{PID: pid, Identity: id})
	}
	sort.Slice(w.Members, func(i, j int) bool { return w.Members[i].PID < w.Members[j].PID })
	return nil
}

// leaderAlive reports whether the recorded leader still runs.
func leaderAlive(w *Worker) bool {
	for _, m := range w.Members {
		if m.PID == w.PID && m.Identity == w.LeaderIdentity {
			return true
		}
	}
	return false
}

// signal sends s to m only while m still runs with its recorded identity.
func signal(m Proc, s syscall.Signal) {
	if id, _ := supervisor.ProcessIdentity(m.PID); id != "" && id == m.Identity {
		_ = syscall.Kill(m.PID, s)
	}
}

// killTree terminates the whole tree: SIGTERM once per member, re-observing
// so forked processes are caught, then SIGKILL once the grace deadline
// passes. The deadline is kept on the worker, so a later tick or a restarted
// dispatcher continues rather than restarts it. It reports whether the tree
// is empty, or the error that made the tree unobservable.
func killTree(w *Worker, grace time.Duration) (bool, error) {
	if w.KillDeadline.IsZero() {
		w.KillDeadline = time.Now().Add(grace)
	}
	stop := w.KillDeadline
	if now := time.Now(); now.After(stop) {
		stop = now
	}
	stop = stop.Add(2 * time.Second)
	termed := map[Proc]bool{}
	for {
		procs, err := observeProcs()
		if err != nil {
			return false, err
		}
		if err := refreshTree(w, procs); err != nil {
			return false, err
		}
		if len(w.Members) == 0 {
			return true, nil
		}
		now := time.Now()
		if now.After(stop) {
			return false, nil
		}
		for _, m := range w.Members {
			if now.After(w.KillDeadline) {
				signal(m, syscall.SIGKILL)
			} else if !termed[m] {
				signal(m, syscall.SIGTERM)
				termed[m] = true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// busyChild reports a running non-leader member whose command is not ignored.
func busyChild(w *Worker, procs map[int]proc, ignore []string) bool {
	for _, m := range w.Members {
		if m.PID != w.PID && !contains(ignore, procs[m.PID].comm) {
			return true
		}
	}
	return false
}
