//go:build darwin || linux

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// reaperParentEnv selects the dead-man reaper mode of this test binary; the
// value is "pid:start", the pid and kernel start time of the test binary it
// watches (V1-2305, ATR-V0-310).
const reaperParentEnv = "CORVINT_CLI_TEST_WORKER_REAPER"

// reaperStallEnv makes a reaper start a child in its own group and then
// ignore end of file, as a reaper stuck in a stalled read would; only the
// test of the forced shutdown sets it. Its value is a file the reaper writes
// the child's "pid start" to.
const reaperStallEnv = "CORVINT_CLI_TEST_REAPER_STALL"

// TestRootEnv names the pid of the test binary that armed the reaper. A
// test binary re-executed by a test as a helper inherits it, sees another
// pid, and stays out of the reaping; a test that needs its own armed
// binary clears it for the child.
const TestRootEnv = "CORVINT_CLI_TEST_ROOT"

// ReaperLogEnv names a file the reaper appends "group PID" to when it first
// attributes a worker group, so a test of the reaper can wait until a group
// is attributed before it kills the binary.
const ReaperLogEnv = "CORVINT_CLI_TEST_REAPER_LOG"

// reaperSample is how often the reaper looks for new worker groups. A group
// started within one sample of the binary's death was never attributed and
// is not stopped by the reaper; the per-test and end-of-run checks, which
// need no sampling, are the only cover for it.
const reaperSample = 200 * time.Millisecond

// reaperStopBound bounds the dead-man reaper's normal shutdown, and again
// its forced retirement.
const reaperStopBound = 30 * time.Second

// reaperPID is the dead-man reaper this binary started. It is a child of
// the binary that leads its own group and stays unreaped until the binary
// retires it, so its pid names it for the whole run and it is never counted
// as a leaked worker.
var reaperPID int

// incarnation is one process: a pid with the kernel start time it had. A
// pid alone names whoever holds it now.
type incarnation struct {
	pid   int
	start int64
}

func incarnationOf(p groupreap.Process) incarnation { return incarnation{p.PID, p.Start} }

// pin is a handle on one process incarnation. Once open it names that
// process and never a later holder of its pid, so every signal the reaper
// sends goes through one, and none goes to a numeric pid or group.
type pin interface {
	// pinned reports whether the process still holds its pid, as a live
	// process or an unreaped zombie. While it does, its pid, and any group
	// it leads, cannot name another process or group.
	pinned() (bool, error)
	// signal sends sig to the pinned process only. gone reports that it had
	// already been reaped, the one failure taken as gone; EPERM and every
	// other error are returned.
	signal(sig syscall.Signal) (gone bool, err error)
	close()
}

// errPinUnsupported reports a platform without a process handle that
// survives pid reuse. There the reaper attributes and reports worker groups
// but signals none of them (ATR-V0-310).
var errPinUnsupported = errors.New("this platform has no process handle that survives pid reuse, so ownership of a worker process cannot be proven and nothing is signalled")

// reapHost is the process table and the process handles the reaper uses;
// tests replace it to inject pid reuse and failures.
type reapHost struct {
	table func() (map[int]groupreap.Process, error)
	// open pins the process that holds pid now. gone reports no process.
	open func(pid int) (p pin, gone bool, err error)
}

func platformReapHost() reapHost {
	return reapHost{table: groupreap.ReadProcessTable, open: openPin}
}

// ReaperSignals reports whether this platform's reaper can prove that it owns
// a worker process and so kills leaked worker groups; where it cannot, a leak
// fails the test and the package and is left running.
func ReaperSignals() bool {
	p, gone, err := openPin(os.Getpid())
	if err != nil || gone {
		return false
	}
	p.close()
	return true
}

type ownedProc struct {
	p groupreap.Process // the observation bound to h
	h pin
}

// reapTracker attributes worker processes to the test binary and retires
// them. A process is owned only on an observation bound to its handle: its
// table row is read after the handle is opened, and the handle is found
// still pinned after the read, so the row was the handle's process. A group
// leader is owned when that row names the test binary as its parent while
// the binary is pinned, and a member when its row puts it in the group of
// an owned leader that is still pinned, or under an owned, pinned parent in
// that group.
type reapTracker struct {
	host        reapHost
	exclude     map[int]bool // never attributed: this process, the reaper
	parent      int          // the test binary
	parentStart int64        // its start time, 0 when it is this process
	parentPin   pin          // its handle, nil when it is this process
	keep        map[incarnation]bool
	owned       map[int]*ownedProc
	ended       []pin // handles of owned processes that have been reaped
	groups      map[int]bool
	unproven    map[incarnation]groupreap.Process
	logLeader   func(pid int)
}

func newReapTracker(host reapHost, parent int, parentStart int64, keep map[incarnation]bool, exclude ...int) *reapTracker {
	r := &reapTracker{host: host, exclude: map[int]bool{}, parent: parent, parentStart: parentStart, keep: keep,
		owned: map[int]*ownedProc{}, groups: map[int]bool{}, unproven: map[incarnation]groupreap.Process{}}
	for _, pid := range exclude {
		r.exclude[pid] = true
	}
	return r
}

func (r *reapTracker) close() {
	for _, o := range r.owned {
		o.h.close()
	}
	for _, h := range r.ended {
		h.close()
	}
	if r.parentPin != nil {
		r.parentPin.close()
	}
	r.owned, r.ended, r.parentPin = map[int]*ownedProc{}, nil, nil
}

func live(p groupreap.Process) bool { return p.State != groupreap.StateZombie }

func (r *reapTracker) parentRowOK(table map[int]groupreap.Process) bool {
	if r.parentStart == 0 {
		return true
	}
	p, ok := table[r.parent]
	return ok && p.Start == r.parentStart
}

func (r *reapTracker) leaderShaped(p groupreap.Process) bool {
	return live(p) && p.PPID == r.parent && p.PGID == p.PID && !r.exclude[p.PID] && !r.keep[incarnationOf(p)]
}

func sortedPIDs(table map[int]groupreap.Process) []int {
	pids := make([]int, 0, len(table))
	for pid := range table {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids
}

func (r *reapTracker) noteLeader(pid int) {
	if r.logLeader != nil {
		r.logLeader(pid)
	}
}

// sample forgets owned processes that have been reaped and attributes new
// ones. Any failure to read the table or to open or check a handle is
// returned: an unobservable process is never taken for a gone one.
func (r *reapTracker) sample() error {
	for pid, o := range r.owned {
		ok, err := o.h.pinned()
		if err != nil {
			return fmt.Errorf("check owned pid %d: %w", pid, err)
		}
		if !ok {
			r.ended = append(r.ended, o.h)
			delete(r.owned, pid)
		}
	}
	before, err := r.host.table()
	if err != nil {
		return fmt.Errorf("read the process table: %w", err)
	}
	parentSeen := r.parentRowOK(before)
	// Candidates: group leaders under the binary, and processes in or under
	// what is owned or a candidate, to a fixed point.
	shaped := map[int]bool{}
	for changed := true; changed; {
		changed = false
		for _, pid := range sortedPIDs(before) {
			p := before[pid]
			if shaped[pid] || !live(p) || r.exclude[pid] || pid == r.parent || r.owned[pid] != nil {
				continue
			}
			leader := parentSeen && r.leaderShaped(p)
			member := r.groups[p.PGID] || (shaped[p.PGID] && before[p.PGID].PGID == p.PGID) || r.owned[p.PPID] != nil || shaped[p.PPID]
			if leader || member {
				shaped[pid], changed = true, true
			}
		}
	}
	cands := map[int]pin{}
	defer func() {
		for _, h := range cands {
			h.close()
		}
	}()
	for _, pid := range sortedPIDs(before) {
		if !shaped[pid] {
			continue
		}
		h, gone, err := r.host.open(pid)
		switch {
		case errors.Is(err, errPinUnsupported):
			if p := before[pid]; parentSeen && r.leaderShaped(p) {
				if _, seen := r.unproven[incarnationOf(p)]; !seen {
					r.unproven[incarnationOf(p)] = p
					r.noteLeader(pid)
				}
			}
			continue
		case err != nil:
			return fmt.Errorf("open pid %d: %w", pid, err)
		case gone:
			continue
		}
		cands[pid] = h
	}
	if len(cands) == 0 {
		return nil
	}
	after, err := r.host.table()
	if err != nil {
		return fmt.Errorf("read the process table: %w", err)
	}
	// Every check from here follows the read of after: a handle still
	// pinned now names the process whose row after holds, since its pid was
	// not released in between.
	pinnedNow := map[int]bool{}
	for pid, h := range cands {
		ok, err := h.pinned()
		if err != nil {
			return fmt.Errorf("check pid %d: %w", pid, err)
		}
		pinnedNow[pid] = ok
	}
	for pid, o := range r.owned {
		ok, err := o.h.pinned()
		if err != nil {
			return fmt.Errorf("check owned pid %d: %w", pid, err)
		}
		pinnedNow[pid] = ok
	}
	parentNow := true
	if r.parentPin != nil {
		if parentNow, err = r.parentPin.pinned(); err != nil {
			return fmt.Errorf("check the test binary: %w", err)
		}
	}
	accept := func(pid int, p groupreap.Process) {
		r.owned[pid] = &ownedProc{p: p, h: cands[pid]}
		delete(cands, pid)
	}
	for _, pid := range sortedPIDs(after) {
		p := after[pid]
		if cands[pid] == nil || !pinnedNow[pid] || !parentNow || !r.parentRowOK(after) || !r.leaderShaped(p) {
			continue
		}
		accept(pid, p)
		r.groups[pid] = true
		r.noteLeader(pid)
	}
	for changed := true; changed; {
		changed = false
		for _, pid := range sortedPIDs(after) {
			p := after[pid]
			if cands[pid] == nil || !pinnedNow[pid] || !live(p) || !r.groups[p.PGID] {
				continue
			}
			leader, parent := r.owned[p.PGID], r.owned[p.PPID]
			byLeader := leader != nil && pinnedNow[p.PGID] && leader.p.PGID == p.PGID
			byParent := parent != nil && pinnedNow[p.PPID] && after[p.PPID].PGID == p.PGID
			if byLeader || byParent {
				accept(pid, p)
				changed = true
			}
		}
	}
	return nil
}

// reapReport is the outcome of a retirement. Only an empty report shows
// that no worker process outlived what was checked.
type reapReport struct {
	killed   []int // owned processes signalled and seen reaped or exited
	stuck    []int // owned processes still running after SIGKILL, within the bound
	unproven []int // processes left running because ownership was not proven
	err      error // the check could not be completed
}

func (rep reapReport) clean() bool {
	return rep.err == nil && len(rep.killed)+len(rep.stuck)+len(rep.unproven) == 0
}

func (rep reapReport) String() string {
	var parts []string
	if len(rep.killed) > 0 {
		parts = append(parts, fmt.Sprintf("killed %v", rep.killed))
	}
	if len(rep.stuck) > 0 {
		parts = append(parts, fmt.Sprintf("still running after SIGKILL %v", rep.stuck))
	}
	if len(rep.unproven) > 0 {
		parts = append(parts, fmt.Sprintf("not signalled, ownership not proven (%v) %v", errPinUnsupported, rep.unproven))
	}
	if rep.err != nil {
		parts = append(parts, fmt.Sprintf("check incomplete, cleanup not proven: %v", rep.err))
	}
	return strings.Join(parts, "; ")
}

// retireBound bounds each step of a retirement.
const retireBound = 250

// retire stops every owned worker process and reports what it found. An
// owned leader is stopped first, so it stays pinned and keeps anchoring its
// group while the members are killed and the group is sampled again for
// members forked meanwhile; the leader is killed last. Every signal goes
// through a pin. Afterwards any live process still in a group an owned
// leader led, and any attributed leader that could not be pinned, is
// reported and never signalled.
func (r *reapTracker) retire() (rep reapReport) {
	defer func() { sort.Ints(rep.killed); sort.Ints(rep.stuck); sort.Ints(rep.unproven) }()
	if rep.err = r.sample(); rep.err != nil {
		return rep
	}
	killed := map[int]*ownedProc{}
	signal := func(o *ownedProc, sig syscall.Signal) error {
		gone, err := o.h.signal(sig)
		if err != nil {
			return fmt.Errorf("signal pid %d: %w", o.p.PID, err)
		}
		if sig == syscall.SIGKILL || gone {
			killed[o.p.PID] = o
		}
		return nil
	}
	isLeader := func(o *ownedProc) bool { return r.groups[o.p.PID] && o.p.PGID == o.p.PID }
	for _, o := range r.owned {
		if isLeader(o) {
			if rep.err = signal(o, syscall.SIGSTOP); rep.err != nil {
				return rep
			}
		}
	}
	for i := 0; ; i++ {
		pending := 0
		for _, o := range r.owned {
			if !isLeader(o) && killed[o.p.PID] == nil {
				if rep.err = signal(o, syscall.SIGKILL); rep.err != nil {
					return rep
				}
			}
		}
		if rep.err = r.sample(); rep.err != nil {
			return rep
		}
		for _, o := range r.owned {
			if !isLeader(o) && killed[o.p.PID] == nil {
				pending++
			}
		}
		if pending == 0 {
			break
		}
		if i == retireBound {
			rep.err = fmt.Errorf("worker groups kept forking members for %d rounds", retireBound)
			return rep
		}
	}
	for _, o := range r.owned {
		if isLeader(o) && killed[o.p.PID] == nil {
			if rep.err = signal(o, syscall.SIGKILL); rep.err != nil {
				return rep
			}
		}
	}
	// A killed process has ended once its pin is released or its row is a
	// zombie with its own start time.
	remaining := killed
	for i := 0; len(remaining) > 0; i++ {
		table, err := r.host.table()
		if err != nil {
			rep.err = fmt.Errorf("read the process table: %w", err)
			return rep
		}
		next := map[int]*ownedProc{}
		for pid, o := range remaining {
			ok, err := o.h.pinned()
			if err != nil {
				rep.err = fmt.Errorf("check killed pid %d: %w", pid, err)
				return rep
			}
			if row, in := table[pid]; ok && in && row.Start == o.p.Start && live(row) {
				next[pid] = o
			}
		}
		if remaining = next; len(remaining) > 0 && i < retireBound {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		break
	}
	for pid := range killed {
		if remaining[pid] != nil {
			rep.stuck = append(rep.stuck, pid)
		} else {
			rep.killed = append(rep.killed, pid)
		}
	}
	table, err := r.host.table()
	if err != nil {
		rep.err = fmt.Errorf("read the process table: %w", err)
		return rep
	}
	for _, pid := range sortedPIDs(table) {
		p := table[pid]
		if !live(p) || killed[pid] != nil && killed[pid].p.Start == p.Start {
			continue
		}
		_, attributed := r.unproven[incarnationOf(p)]
		if r.groups[p.PGID] || attributed {
			rep.unproven = append(rep.unproven, pid)
		}
	}
	return rep
}

// selfExclusions are the processes an in-process check never attributes.
func selfExclusions() []int { return []int{os.Getpid(), reaperPID} }

// retireOwnChildren is the in-process check: it retires every group led by a
// child of this binary that is not in keep, with its members.
func retireOwnChildren(host reapHost, keep map[incarnation]bool) reapReport {
	r := newReapTracker(host, os.Getpid(), 0, keep, selfExclusions()...)
	defer r.close()
	return r.retire()
}

// ownLeaders returns the children of this binary that lead their own group.
func ownLeaders(table map[int]groupreap.Process) map[incarnation]bool {
	r := newReapTracker(reapHost{}, os.Getpid(), 0, nil, selfExclusions()...)
	out := map[incarnation]bool{}
	for _, p := range table {
		if r.leaderShaped(p) {
			out[incarnationOf(p)] = true
		}
	}
	return out
}

// ReapWorkersOnCleanup makes the test own the process groups it starts
// from now on: when it ends, by return, t.Fatal or t.FailNow, every group
// led by a child of the test binary that was not running at this call is
// retired where the platform can prove it owns it, and fails the test either
// way (V1-2305, ATR-V0-310). Call it before the test's own cleanups are
// registered, so it runs after them. A binary that dies instead of ending
// the test is covered by the dead-man reaper. Do not use it in a test that
// runs in parallel with another that starts groups.
func ReapWorkersOnCleanup(t *testing.T) {
	t.Helper()
	table, err := groupreap.ReadProcessTable()
	if err != nil {
		t.Fatalf("V1-2305: the process table cannot be read, so worker groups cannot be checked: %v", err)
	}
	keep := ownLeaders(table)
	t.Cleanup(func() {
		if rep := retireOwnChildren(platformReapHost(), keep); !rep.clean() {
			t.Errorf("V1-2305: worker process groups outlived the test: %s", rep)
		}
	})
}

// startWorkerReaper re-executes this test binary as a reaper leading its own
// process group, owned through its unreaped leader. Its stdin is the read
// end of a pipe whose close-on-exec write end only this binary holds, so the
// kernel closes it when the binary dies by any means, including SIGKILL or
// the package timeout, when no cleanup and no TestMain code after m.Run
// runs. Its stderr is the binary's, so what it finds is seen.
func startWorkerReaper(extraEnv ...string) (*os.File, *groupreap.Owner, int, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, 0, err
	}
	table, err := groupreap.ReadProcessTable()
	if err != nil {
		return nil, nil, 0, err
	}
	self, ok := table[os.Getpid()]
	if !ok {
		return nil, nil, 0, errors.New("this binary is missing from the process table")
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, 0, err
	}
	defer r.Close()
	cmd := exec.Command(exe, "-test.run=^$")
	cmd.Env = append(append(os.Environ(), fmt.Sprintf("%s=%d:%d", reaperParentEnv, self.PID, self.Start)), extraEnv...)
	cmd.Stdin, cmd.Stderr = r, os.Stderr
	owner, err := groupreap.Start(cmd)
	if err != nil {
		w.Close()
		return nil, nil, 0, err
	}
	return w, owner, cmd.Process.Pid, nil
}

// stopWorkerReaper closes the pipe and waits up to bound for the reaper to
// end; one that has not is killed with its whole group while its leader is
// still unreaped. Either way it is reaped before this returns. A reaper that
// ended with a failure, or had to be killed, is an error.
func stopWorkerReaper(w *os.File, owner *groupreap.Owner, bound time.Duration) error {
	w.Close()
	forced := false
	timer := time.NewTimer(bound)
	defer timer.Stop()
	select {
	case <-owner.Exited():
	case <-timer.C:
		forced = true
		owner.Stop()
	}
	deadline := time.Now().Add(bound)
	res := owner.FinishBounded(groupreap.RetirementBound{Expired: func() bool { return !time.Now().Before(deadline) }})
	switch {
	case res.State != groupreap.Released:
		return fmt.Errorf("the dead-man reaper's group was not retired: %v", res.Err)
	case forced:
		return fmt.Errorf("the dead-man reaper did not end within %v and was killed with its group", bound)
	case res.WaitErr != nil:
		return fmt.Errorf("the dead-man reaper failed: %v", res.WaitErr)
	}
	return nil
}

func appendReaperLog(pid int) {
	if p := os.Getenv(ReaperLogEnv); p != "" {
		if f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600); err == nil {
			fmt.Fprintf(f, "group %d\n", pid)
			f.Close()
		}
	}
}

// runWorkerReaper follows the test binary named by spec ("pid:start") until
// its stdin reaches end of file, which the kernel delivers once the binary
// has exited by any means. It attributes the binary's group-leading children
// and their members every sample, then retires them. It returns 1, with
// what it found on out, when it killed anything, left anything unproven, or
// could not observe the process table at any sample.
func runWorkerReaper(host reapHost, spec string, in io.Reader, out io.Writer, every time.Duration) int {
	pidText, startText, _ := strings.Cut(spec, ":")
	parent, e1 := strconv.Atoi(pidText)
	start, e2 := strconv.ParseInt(startText, 10, 64)
	if e1 != nil || e2 != nil || parent <= 0 {
		fmt.Fprintf(out, "FAIL: V1-2305: dead-man reaper: malformed %s %q\n", reaperParentEnv, spec)
		return 1
	}
	r := newReapTracker(host, parent, start, nil, os.Getpid())
	defer r.close()
	r.logLeader = appendReaperLog
	var sampleErr error
	switch h, gone, err := host.open(parent); {
	case errors.Is(err, errPinUnsupported):
		// Attribution still runs, for the report; nothing is signalled.
	case err != nil || gone:
		sampleErr = fmt.Errorf("pin the test binary %d: gone %v, %v", parent, gone, err)
	default:
		r.parentPin = h
	}
	eof := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, in); close(eof) }()
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-eof:
			rep := r.retire()
			code := 0
			if sampleErr != nil {
				fmt.Fprintf(out, "FAIL: V1-2305: dead-man reaper: a sample failed, so attribution is incomplete: %v\n", sampleErr)
				code = 1
			}
			if !rep.clean() {
				fmt.Fprintf(out, "FAIL: V1-2305: dead-man reaper: worker process groups outlived the test binary: %s\n", rep)
				code = 1
			}
			return code
		case <-tick.C:
			if err := r.sample(); err != nil && sampleErr == nil {
				sampleErr = err
			}
		}
	}
}

// stallReaper is the reaper of the forced-shutdown test: it starts a child in
// its own group, records it, and never returns.
func stallReaper(record string) {
	child := exec.Command("/bin/sleep", "600")
	if err := child.Start(); err == nil {
		if table, err := groupreap.ReadProcessTable(); err == nil {
			p := table[child.Process.Pid]
			_ = os.WriteFile(record+".tmp", []byte(fmt.Sprintf("%d %d\n", p.PID, p.Start)), 0o600)
			_ = os.Rename(record+".tmp", record)
		}
	}
	for {
		time.Sleep(time.Hour)
	}
}

// testMain arms the dead-man reaper for the binary and, after the run, joins
// the detached children the tests launched and fails the package if any test
// left a process group running under a child of the binary. Cleanups and
// the code after m.Run never run when the binary is killed, so the reaper
// stops the groups then (V1-2305, ATR-V0-310). It returns the exit code.
func testMain(m *testing.M) int {
	if v := os.Getenv(reaperParentEnv); v != "" {
		if record := os.Getenv(reaperStallEnv); record != "" {
			stallReaper(record)
		}
		os.Exit(runWorkerReaper(platformReapHost(), v, os.Stdin, os.Stderr, reaperSample))
	}
	if root := os.Getenv(TestRootEnv); root != "" && root != strconv.Itoa(os.Getpid()) {
		return m.Run() // a helper re-execution of this binary
	}
	if err := os.Setenv(TestRootEnv, strconv.Itoa(os.Getpid())); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: V1-2305: %v\n", err)
		return 1
	}
	w, owner, pid, err := startWorkerReaper()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: V1-2305: start the dead-man reaper: %v\n", err)
		return 1
	}
	reaperPID = pid
	code := m.Run()
	if left := joinDetached("", detachJoinBound); left > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: ATR-V0-310: %d detached children were still running %v after the last test\n", left, detachJoinBound)
		code = 1
	}
	if rep := retireOwnChildren(platformReapHost(), nil); !rep.clean() {
		fmt.Fprintf(os.Stderr, "FAIL: V1-2305: tests left worker process groups running: %s\n", rep)
		code = 1
	}
	if err := stopWorkerReaper(w, owner, reaperStopBound); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: V1-2305: %v\n", err)
		code = 1
	}
	return code
}

// awaitGroupExit waits, bounded, until no live process is in group pgid, so
// a group the test killed has exited before ReapWorkersOnCleanup judges it
// (V1-3832). A group still live at the deadline is left to that check.
func awaitGroupExit(t *testing.T, pgid int, within time.Duration) {
	t.Helper()
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		table, err := groupreap.ReadProcessTable()
		if err != nil {
			return
		}
		if !groupLive(table, pgid) {
			return
		}
	}
}

// groupLive reports a live (non-zombie) member of group pgid in table.
func groupLive(table map[int]groupreap.Process, pgid int) bool {
	for _, p := range table {
		if p.PGID == pgid && live(p) {
			return true
		}
	}
	return false
}

// TestAwaitGroupExitWaitsForKilledMembers (V1-3832): a group with a leader
// and a member, killed with SIGKILL, has no live member once awaitGroupExit
// returns, so ReapWorkersOnCleanup finds nothing to report.
func TestAwaitGroupExitWaitsForKilledMembers(t *testing.T) {
	ReapWorkersOnCleanup(t)
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & exec sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(waited) }()
	pgid := cmd.Process.Pid
	for i := 0; i < 250; i++ {
		table, err := groupreap.ReadProcessTable()
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, p := range table {
			if p.PGID == pgid && live(p) {
				n++
			}
		}
		if n >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	syscall.Kill(-pgid, syscall.SIGKILL)
	awaitGroupExit(t, pgid, 30*time.Second)
	table, err := groupreap.ReadProcessTable()
	if err != nil {
		t.Fatal(err)
	}
	if groupLive(table, pgid) {
		t.Fatalf("group %d has a live member after awaitGroupExit", pgid)
	}
	<-waited
}
