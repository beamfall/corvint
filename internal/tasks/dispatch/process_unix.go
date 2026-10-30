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
	// cpu is the process's cumulative CPU time; cpuKnown is false when
	// the table's value could not be parsed (CAL-V0-218).
	cpu      time.Duration
	cpuKnown bool
}

// lockExclusive takes the program's single-dispatcher lock without waiting.
func lockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// lockShared takes the state directory's start gate shared without waiting.
func lockShared(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
}

// observeProcs reads the live (non-zombie) process table.
func observeProcs() (map[int]proc, error) {
	ps := "/bin/ps"
	if _, err := os.Stat(ps); err != nil {
		ps = "/usr/bin/ps" // Linux without a merged /usr
	}
	raw, err := exec.Command(ps, "-axo", "pid=,ppid=,pgid=,stat=,time=,comm=").Output()
	if err != nil {
		return nil, err
	}
	out := map[int]proc{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) < 6 || strings.HasPrefix(f[3], "Z") {
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
		p.cpu, p.cpuKnown = parseCPUTime(f[4])
		p.comm = filepath.Base(strings.Join(f[5:], " "))
		out[p.pid] = p
	}
	return out, nil
}

// parseCPUTime parses a ps cumulative CPU time, [[dd-]hh:]mm:ss[.ff] as
// Darwin and Linux print it.
func parseCPUTime(s string) (time.Duration, bool) {
	days := 0
	if i := strings.IndexByte(s, '-'); i >= 0 {
		n, err := strconv.Atoi(s[:i])
		if err != nil || n < 0 {
			return 0, false
		}
		days, s = n, s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	last := parts[len(parts)-1]
	if last == "" || last[0] < '0' || last[0] > '9' {
		return 0, false
	}
	// ParseDuration reads the fraction exactly, without float rounding.
	total, err := time.ParseDuration(last + "s")
	if err != nil || total >= time.Minute {
		return 0, false
	}
	for i, unit := range []time.Duration{time.Minute, time.Hour}[:len(parts)-1] {
		n, err := strconv.Atoi(parts[len(parts)-2-i])
		if err != nil || n < 0 {
			return 0, false
		}
		total += time.Duration(n) * unit
	}
	return total + time.Duration(days)*24*time.Hour, true
}

// userHZ is the Linux USER_HZ that /proc reports CPU ticks in; it is 100 on
// every architecture this module builds for.
const userHZ = 100

// parseProcStatCPU reads utime+stime (fields 14 and 15) from a Linux
// /proc/<pid>/stat line. The command name may hold spaces and parentheses,
// so fields are counted after its last closing parenthesis.
func parseProcStatCPU(s string) (time.Duration, bool) {
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return 0, false
	}
	f := strings.Fields(s[i+1:]) // f[0] is field 3, the state
	if len(f) < 13 {
		return 0, false
	}
	user, e1 := strconv.ParseUint(f[11], 10, 32)
	sys, e2 := strconv.ParseUint(f[12], 10, 32)
	if e1 != nil || e2 != nil {
		return 0, false
	}
	return time.Duration(user+sys) * (time.Second / userHZ), true
}

// processIdentity reads the tree's start identities; tests replace it to
// inject an unreadable process table.
var processIdentity = supervisor.ProcessIdentity

// launchGroupKill is syscall.Kill; tests observe the leader at the moment
// launch signals an unidentified worker's group.
var launchGroupKill = syscall.Kill

// launchObserved, when set, receives each launched worker's group leader
// and start identity, so tests can stop and reap every tree they start
// (V1-0948). It is nil outside tests.
var launchObserved func(pid int, identity string)

// launch starts argv as a new session leader so it outlives the dispatcher,
// with output appended to the worker's log files.
func launch(argv, env []string, dir, logDir string) (int, string, <-chan exitStatus, error) {
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
	id, err := processIdentity(pid)
	if err == nil && id == "" {
		err = fmt.Errorf("pid %d has no start identity", pid)
	}
	if err != nil {
		// An unidentified tree cannot be supervised safely; stop it now,
		// before Wait can reap the leader: the unreaped leader keeps its
		// session and group ID from naming another group (V1-1036).
		_ = launchGroupKill(-pid, syscall.SIGKILL)
	}
	exit := make(chan exitStatus, 1)
	go func() {
		_ = cmd.Wait()
		st := exitStatus{code: cmd.ProcessState.ExitCode()}
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			st.signal = int(ws.Signal()) // ATR-V0-350
		}
		exit <- st
	}()
	if err != nil {
		return 0, "", nil, &startedError{pid: pid, exit: exit, err: fmt.Errorf("worker start identity unreadable: %w", err)}
	}
	if launchObserved != nil {
		launchObserved(pid, id)
	}
	// The leader's identity stays beside its logs, so a directory whose
	// worker the ledger never recorded is not retired while its tree may run
	// (CAL-V0-144). A failed write leaves the directory to the quiet window.
	_ = os.WriteFile(filepath.Join(logDir, workerLeaderName), []byte(strconv.Itoa(pid)+" "+id+"\n"), 0o600)
	return pid, id, exit, nil
}

// refreshTree recomputes a worker's process tree (CAL-V0-056): recorded
// members whose start identity still matches, then, to a fixed point, their
// children, members of their process groups, and processes in the leader's
// session. Session expansion is disabled once the session ID names a
// different process, so PID reuse cannot pull in an unrelated session. An
// unreadable identity leaves the recorded tree unchanged and returns the
// error: an unobservable member is never taken for a gone one, and neither is
// a member or leader whose identity compares unknown (PSR-V0-227). Processes
// of the verified detached run supervisors in exempt are left out
// (CAL-V0-145).
func refreshTree(w *Worker, procs map[int]proc, exempt map[int]string) error {
	ex, err := newExemption(procs, exempt, w.PID)
	if err != nil {
		return err
	}
	members := map[int]string{}
	for _, m := range w.Members {
		if _, ok := procs[m.PID]; ok {
			if ex.has(m.PID) {
				continue
			}
			id, err := processIdentity(m.PID)
			if err != nil {
				return err
			}
			// The recorded identity is kept, so a Linux btime one second
			// off this read never drifts the record; a wider gap with the
			// same start tick may be the member after a clock step, so it
			// is uncertainty, never a gone member (PSR-V0-227).
			switch supervisor.CompareIdentity(id, m.Identity) {
			case supervisor.IdentitySame:
				members[m.PID] = m.Identity
			case supervisor.IdentityUnknown:
				return fmt.Errorf("process %d identity is uncertain: %s is not %s within one second", m.PID, id, m.Identity)
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
		match := supervisor.CompareIdentity(id, w.LeaderIdentity)
		if match == supervisor.IdentityUnknown {
			return fmt.Errorf("leader %d identity is uncertain: %s is not %s within one second", w.PID, id, w.LeaderIdentity)
		}
		session = id == "" || match == supervisor.IdentitySame
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
			if !in || ex.has(pid) {
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

// exemption decides membership of the detached run supervisors that
// survive their worker (CAL-V0-145): a supervisor whose start identity is
// re-verified against this table, every process in its session, and the
// descendants of either, by parent chain. The chain stops at the worker
// leader and at this dispatcher, and a supervisor that is an ancestor of
// this dispatcher is never one, so a record naming an enclosing session
// leader cannot spare the worker itself.
type exemption struct {
	procs  map[int]proc
	sups   map[int]bool
	memo   map[int]bool
	leader int
}

func newExemption(procs map[int]proc, exempt map[int]string, leader int) (*exemption, error) {
	e := &exemption{procs: procs, sups: map[int]bool{}, memo: map[int]bool{}, leader: leader}
	if len(exempt) == 0 {
		return e, nil
	}
	ancestors := map[int]bool{}
	for pid := os.Getpid(); pid > 1 && !ancestors[pid]; {
		ancestors[pid] = true
		p, ok := procs[pid]
		if !ok {
			break
		}
		pid = p.ppid
	}
	for pid, want := range exempt {
		if _, ok := procs[pid]; !ok || want == "" || pid == leader || ancestors[pid] {
			continue
		}
		id, err := processIdentity(pid)
		if err != nil {
			return nil, err
		}
		if supervisor.SameProcess(id, want) {
			e.sups[pid] = true
		}
	}
	return e, nil
}

func (e *exemption) has(pid int) bool {
	if len(e.sups) == 0 {
		return false
	}
	var chain []int
	seen := map[int]bool{}
	out := false
	for pid > 1 && !seen[pid] && pid != e.leader && pid != os.Getpid() {
		if v, ok := e.memo[pid]; ok {
			out = v
			break
		}
		seen[pid] = true
		chain = append(chain, pid)
		if e.sups[pid] {
			out = true
			break
		}
		if sid, err := getsid(pid); err == nil && e.sups[sid] {
			out = true
			break
		}
		p, ok := e.procs[pid]
		if !ok {
			break
		}
		pid = p.ppid
	}
	for _, p := range chain {
		e.memo[p] = out
	}
	return out
}

// detachedSupervisor reports whether pid is a live detached run supervisor
// with the recorded start identity: a session leader (ATR-V0-008) that is
// neither the worker leader nor this dispatcher.
func detachedSupervisor(pid int, identity string, leader int) bool {
	if pid <= 1 || pid == leader || pid == os.Getpid() || identity == "" {
		return false
	}
	if id, err := processIdentity(pid); err != nil || !supervisor.SameProcess(id, identity) {
		return false
	}
	sid, err := getsid(pid)
	return err == nil && sid == pid
}

// leaderAlive reports whether the recorded leader still runs: a member with
// the leader's PID whose identity names the recorded leader (PSR-V0-227).
func leaderAlive(w *Worker) bool {
	for _, m := range w.Members {
		if m.PID == w.PID && supervisor.SameProcess(m.Identity, w.LeaderIdentity) {
			return true
		}
	}
	return false
}

// signal sends s to m only while m still runs with its recorded identity.
func signal(m Proc, s syscall.Signal) {
	if id, _ := processIdentity(m.PID); supervisor.SameProcess(id, m.Identity) {
		_ = syscall.Kill(m.PID, s)
	}
}

// killTree terminates the whole tree: SIGTERM once per member, re-observing
// so forked processes are caught, then SIGKILL once the grace deadline
// passes. The deadline is kept on the worker, so a later tick or a restarted
// dispatcher continues rather than restarts it. It reports whether the tree
// is empty, or the error that made the tree unobservable.
func killTree(w *Worker, grace time.Duration, exempt map[int]string) (bool, error) {
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
		if err := refreshTree(w, procs, exempt); err != nil {
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

// busyChild reports a non-leader member whose command is not ignored and
// that shows work since the baseline (CAL-V0-218): it is absent from the
// baseline, its CPU time is unreadable or went backwards, or the CPU time
// it used since the baseline reaches 1/idleCPUDivisor of the wall time
// since then. A member that merely exists, sleeping, is not busy.
func busyChild(w *Worker, procs map[int]proc, ignore []string, base childCPU, now time.Time) bool {
	elapsed := now.Sub(base.at)
	for _, m := range w.Members {
		p := procs[m.PID]
		if m.PID == w.PID || contains(ignore, p.comm) {
			continue
		}
		was, ok := base.cpu[childKey(m)]
		if !ok || !p.cpuKnown {
			return true
		}
		// A counter below the baseline belongs to another process.
		if used := p.cpu - was; used < 0 || used > 0 && used*idleCPUDivisor >= elapsed {
			return true
		}
	}
	return false
}

// childKey names one member in a baseline: Linux start identities omit the
// PID, so children started in the same clock tick share one.
func childKey(m Proc) string { return strconv.Itoa(m.PID) + "/" + m.Identity }

// childSample records the CPU time of each non-leader member whose command
// is not ignored, by PID and start identity, as a CAL-V0-218 baseline.
func childSample(w *Worker, procs map[int]proc, ignore []string, now time.Time) childCPU {
	base := childCPU{at: now, cpu: map[string]time.Duration{}}
	for _, m := range w.Members {
		if p, ok := procs[m.PID]; ok && p.cpuKnown && m.PID != w.PID && !contains(ignore, p.comm) {
			base.cpu[childKey(m)] = p.cpu
		}
	}
	return base
}

// openReaderMarker refuses symlinks and cannot block on a raced FIFO.
func openReaderMarker(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// liveTrees reads the process groups and sessions that have a live member,
// for CAL-V0-144. A process that exits between the table read and its
// session read may have forked a child the table does not show, so the read
// is taken once more and both reads are joined: that child is alive at the
// second table read or has itself exited. ok is false when a table or a
// session is unreadable.
func liveTrees() (groups, sessions map[int]bool, ok bool) {
	groups, sessions = map[int]bool{}, map[int]bool{}
	self := os.Getpid()
	for read := 0; read < 2; read++ {
		procs, err := superviseProcs()
		if err != nil {
			return nil, nil, false
		}
		gone := false
		for pid, p := range procs {
			groups[p.pgid] = true
			sid, err := getsid(pid)
			if err == syscall.ESRCH {
				// The table reader itself forks nothing.
				gone = gone || p.ppid != self || p.comm != "ps"
				continue
			}
			if err != nil {
				return nil, nil, false
			}
			sessions[sid] = true
		}
		if !gone {
			break
		}
	}
	return groups, sessions, true
}

// treeMayRun reports whether a worker tree whose leader started as pid with
// start identity id may still run: the leader still has that identity, or a
// live process remains in its process group or session. An unreadable
// identity reports true, and so does one whose start tick matches but whose
// Linux btime is more than a second off: it is unknown, not gone
// (PSR-V0-227).
func treeMayRun(pid int, id string, groups, sessions map[int]bool) bool {
	cur, err := processIdentity(pid)
	if err != nil || supervisor.CompareIdentity(cur, id) != supervisor.IdentityDifferent {
		return true
	}
	return groups[pid] || sessions[pid]
}

// dupForExec duplicates f's descriptor without close-on-exec, so an exec'd
// image inherits the same open file description and with it the flock
// (DSG-V0-012).
// The copy is at least 3, so it never lands on a closed standard stream
// that the new image would reject.
func dupForExec(f *os.File) (int, error) {
	fd, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), syscall.F_DUPFD, 3)
	if errno != 0 {
		return -1, errno
	}
	return int(fd), nil
}

func closeFD(fd int) { _ = syscall.Close(fd) }

// execImage replaces this process image; it returns only on failure.
func execImage(path string, argv, env []string) error {
	return syscall.Exec(path, argv, env)
}

// waitChild observes the exit of pid when it is this process's child: a
// worker an earlier image of this process launched (DSG-V0-013). It returns
// nil when pid is not a child, so adoption stays NOT_OBSERVED.
func waitChild(pid int) <-chan exitStatus {
	if pid <= 0 {
		return nil
	}
	status := func(ws syscall.WaitStatus) exitStatus {
		st := exitStatus{code: ws.ExitStatus()}
		if ws.Signaled() {
			st.code, st.signal = -1, int(ws.Signal())
		}
		return st
	}
	var ws syscall.WaitStatus
	got, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil)
	for err == syscall.EINTR {
		got, err = syscall.Wait4(pid, &ws, syscall.WNOHANG, nil)
	}
	if err != nil {
		return nil
	}
	ch := make(chan exitStatus, 1)
	if got == pid {
		ch <- status(ws)
		return ch
	}
	go func() {
		for {
			var ws syscall.WaitStatus
			got, err := syscall.Wait4(pid, &ws, 0, nil)
			if err == syscall.EINTR {
				continue
			}
			if err == nil && got == pid {
				ch <- status(ws)
			}
			return
		}
	}()
	return ch
}

func markCloseOnExec(fd int) { syscall.CloseOnExec(fd) }
