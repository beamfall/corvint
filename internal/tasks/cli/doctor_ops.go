package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Operational detectors (TQD-V0-021..024): queue lock holders and slow
// queue commands from the local process table and this queue's lock files,
// completion stalls from the inventory, and system file-table pressure.
// Every probe only reads; a probe that cannot complete adds a scan warning
// and no finding.

// doctorQueueLocks are this queue's two flock files under the common dir.
var doctorQueueLocks = []string{authority.LockFileName, "taskman.prepare.lock"}

// doctorLongVerbs are corvint-tasks verbs that run long by design and are
// never SLOW_QUEUE_COMMAND; "gate run" is matched with its subcommand.
var doctorLongVerbs = map[string]bool{"run": true, "run-batch": true, "dispatch": true, "service": true, "qualify": true, "doctor": true, "preflight": true, "pool": true}

const (
	doctorCommandText = 256
	doctorUnsupported = "probe not supported on this platform"
)

var errDoctorUnsupported = errors.New(doctorUnsupported)

// doctorCmdProc is one process-table row: pid, elapsed time and argv text.
type doctorCmdProc struct {
	pid     int
	elapsed time.Duration
	command string
}

// doctorCommandTable is the bounded local process walk with elapsed times
// and argv (`/bin/ps -axww -o pid=,etime=,command=`, the TQD-V0-009 bounds).
var doctorCommandTable = func() ([]doctorCmdProc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), doctorPsTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "/bin/ps", "-axww", "-o", "pid=,etime=,command=")
	c.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin"}
	out := &doctorLimited{max: doctorPsBytes}
	c.Stdout = out
	if err := c.Run(); err != nil {
		return nil, err
	}
	if out.over {
		return nil, errors.New("process table larger than the walk bound")
	}
	return parseDoctorCommandTable(out.buf.String())
}

func parseDoctorCommandTable(text string) ([]doctorCmdProc, error) {
	var out []doctorCmdProc
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(out) >= doctorPsProcs || len(f) < 3 {
			return nil, errors.New("process table exceeds the walk bound or is malformed")
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			return nil, errors.New("malformed process table line")
		}
		el, err := parseDoctorElapsed(f[1])
		if err != nil {
			return nil, err
		}
		out = append(out, doctorCmdProc{pid: pid, elapsed: el, command: strings.Join(f[2:], " ")})
	}
	return out, nil
}

// parseDoctorElapsed reads ps etime, [[dd-]hh:]mm:ss.
func parseDoctorElapsed(s string) (time.Duration, error) {
	bad := fmt.Errorf("malformed elapsed time %q", s)
	days := 0
	if i := strings.IndexByte(s, '-'); i >= 0 {
		d, err := strconv.Atoi(s[:i])
		if err != nil || d < 0 {
			return 0, bad
		}
		days, s = d, s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, bad
	}
	secs := 0
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || len(p) == 0 {
			return 0, bad
		}
		secs = secs*60 + n
	}
	return time.Duration(days*86400+secs) * time.Second, nil
}

// doctorQueueCommand reports whether a process is a corvint-tasks command
// and returns its verb text.
func doctorQueueCommand(command string) (string, bool) {
	f := strings.Fields(command)
	if len(f) == 0 || filepath.Base(f[0]) != "corvint-tasks" {
		return "", false
	}
	if len(f) == 1 {
		return "", true
	}
	if len(f) > 2 && f[1] == "gate" && f[2] == "run" {
		return "gate run", true
	}
	return f[1], true
}

func doctorLongRunning(verb string) bool { return verb == "gate run" || doctorLongVerbs[verb] }

// doctorShortCommand bounds a command line for a finding's detail.
func doctorShortCommand(s string) string {
	if len(s) <= doctorCommandText {
		return s
	}
	s = s[:doctorCommandText]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "..."
}

// doctorLockFindings reports QUEUE_LOCK_HELD (TQD-V0-021) and
// SLOW_QUEUE_COMMAND (TQD-V0-022). Only corvint-tasks processes that have
// one of this queue's lock files open are considered, so commands of other
// queues on the host are never named.
func doctorLockFindings(commonDir string, th doctorThresholds, scan *doctorScan) []doctorFinding {
	warn := func(s string) { scan.warnings = append(scan.warnings, prose("doctor "+s)) }
	procs, err := doctorCommandTable()
	if err != nil {
		warn("process walk incomplete: " + err.Error())
		return nil
	}
	byPid := map[int]doctorCmdProc{}
	var candidates []int
	self := os.Getpid()
	for _, p := range procs {
		if _, ok := doctorQueueCommand(p.command); ok && p.pid != self {
			byPid[p.pid] = p
			candidates = append(candidates, p.pid)
		}
	}
	paths := make([]string, len(doctorQueueLocks))
	for i, n := range doctorQueueLocks {
		paths[i] = filepath.Join(commonDir, n)
	}
	openers := map[string][]int{}
	if len(candidates) > 0 {
		sort.Ints(candidates)
		if openers, err = doctorLockOpeners(paths, candidates); err != nil {
			if !errors.Is(err, errDoctorUnsupported) {
				warn("lock opener probe incomplete: " + err.Error())
			}
			return nil
		}
	}
	var out []doctorFinding
	named := map[int]bool{}
	for i, path := range paths {
		held, holder, err := doctorLockHolder(path)
		if errors.Is(err, errDoctorUnsupported) {
			continue
		}
		if err != nil {
			warn(fmt.Sprintf("lock probe of %s incomplete: %v", doctorQueueLocks[i], err))
			continue
		}
		if !held {
			continue
		}
		others := 0
		if _, ok := byPid[holder]; !ok && holder != 0 {
			warn(fmt.Sprintf("%s is held by pid %d, which is not a corvint-tasks command of this host", doctorQueueLocks[i], holder))
			continue
		}
		if holder == 0 {
			// Without a known holder the oldest corvint-tasks opener is
			// named; it is the holder or a waiter that has waited longest.
			for _, pid := range openers[path] {
				if holder == 0 || byPid[pid].elapsed > byPid[holder].elapsed {
					holder = pid
				}
			}
		}
		if holder == 0 {
			warn(fmt.Sprintf("%s is held by a process that is not a corvint-tasks command of this host", doctorQueueLocks[i]))
			continue
		}
		for _, pid := range openers[path] {
			if pid != holder {
				others++
			}
		}
		p := byPid[holder]
		if p.elapsed < th.lockHeld {
			continue
		}
		named[holder] = true
		out = append(out, doctorFinding{kind: "QUEUE_LOCK_HELD", who: doctorQueueLocks[i], carried: true,
			detail: fmt.Sprintf("held by pid %d running %ds (%s); %d other corvint-tasks process(es) have it open", holder, int64(p.elapsed/time.Second), doctorShortCommand(p.command), others)})
	}
	seen := map[int]bool{}
	for _, path := range paths {
		for _, pid := range openers[path] {
			p, ok := byPid[pid]
			if !ok || seen[pid] || named[pid] {
				continue
			}
			seen[pid] = true
			verb, _ := doctorQueueCommand(p.command)
			if doctorLongRunning(verb) || p.elapsed < th.slowCommand {
				continue
			}
			out = append(out, doctorFinding{kind: "SLOW_QUEUE_COMMAND", who: "pid " + strconv.Itoa(pid), carried: true,
				detail: fmt.Sprintf("corvint-tasks %s running %ds with the queue lock open (%s)", verb, int64(p.elapsed/time.Second), doctorShortCommand(p.command))})
		}
	}
	return out
}

// doctorNoCompletions reports NO_COMPLETIONS (TQD-V0-023): no completion
// recorded within the window while ready tickets exist and a worker is idle.
func doctorNoCompletions(rc *readCtx, in transaction.PlanInput, th doctorThresholds, scan *doctorScan) []doctorFinding {
	idle := scan.lanesFree > 0 || (scan.lanesTotal == 0 && scan.runningSessions == 0)
	if !idle {
		return nil
	}
	var ref time.Time
	refWhat := "the last completion"
	for _, id := range rc.store.Inventory.IDs() {
		rec, ok := rc.store.Inventory.Get(id)
		if !ok || rec.Completion == nil {
			continue
		}
		if at, err := time.Parse(time.RFC3339, string(rec.Completion.RecordedAt)); err == nil && at.After(ref) {
			ref = at
		}
	}
	if ref.IsZero() {
		if len(scan.events) == 0 {
			return nil
		}
		ref, refWhat = scan.events[0].at, "the earliest scanned receipt"
	}
	if scan.observedAt.Sub(ref) < th.noCompletions {
		return nil
	}
	ready := 0
	for _, id := range rc.store.Inventory.IDs() {
		rec, ok := rc.store.Inventory.Get(id)
		if !ok {
			continue
		}
		if v, _ := transaction.RecordedClaimability(in, rec); v.Kind == wire.KindBool && v.Bool {
			ready++
		}
	}
	if ready == 0 {
		return nil
	}
	return []doctorFinding{{kind: "NO_COMPLETIONS", who: rc.store.Queue.QueueID.Raw, firstSeen: ref.Add(th.noCompletions),
		detail: fmt.Sprintf("no completion for %ds since %s (%s) while %d ticket(s) are ready and %d of %d lanes are free with %d running session(s)",
			int64(scan.observedAt.Sub(ref)/time.Second), ref.UTC().Format(time.RFC3339), refWhat, ready, scan.lanesFree, scan.lanesTotal, scan.runningSessions)}}
}

// doctorFDPressure reports FD_PRESSURE (TQD-V0-024) from the system file
// table.
func doctorFDPressure(th doctorThresholds, scan *doctorScan) []doctorFinding {
	used, max, err := doctorFileTable()
	if errors.Is(err, errDoctorUnsupported) {
		return nil
	}
	if err != nil {
		scan.warnings = append(scan.warnings, prose("doctor file-table probe incomplete: "+err.Error()))
		return nil
	}
	if max == 0 || used*100 < uint64(th.fdPercent)*max {
		return nil
	}
	return []doctorFinding{{kind: "FD_PRESSURE", who: "system", carried: true,
		detail: fmt.Sprintf("%d of %d system file handles in use (%d%%, threshold %d%%)", used, max, used*100/max, th.fdPercent)}}
}

// parseDoctorProcLocks returns the pids holding a FLOCK on the file with
// device major:minor and inode ino in Linux /proc/locks text; blocked
// waiters ("->" lines) are not holders.
func parseDoctorProcLocks(text string, major, minor, ino uint64) []int {
	want := fmt.Sprintf("%02x:%02x:%d", major, minor, ino)
	var out []int
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 || f[1] != "FLOCK" {
			continue
		}
		if f[5] != want {
			continue
		}
		if pid, err := strconv.Atoi(f[4]); err == nil && pid > 0 {
			out = append(out, pid)
		}
	}
	return out
}

// parseDoctorLsof maps each inode in lsof -F pi output to the pids that
// have it open.
func parseDoctorLsof(text string) map[uint64][]int {
	out := map[uint64][]int{}
	pid := 0
	for _, line := range strings.Split(text, "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'i':
			if ino, err := strconv.ParseUint(line[1:], 10, 64); err == nil && pid > 0 && !containsInt(out[ino], pid) {
				out[ino] = append(out[ino], pid)
			}
		}
	}
	return out
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// parseDoctorFileNr reads Linux /proc/sys/fs/file-nr: allocated, unused
// and maximum handles; it returns the handles in use and the maximum.
func parseDoctorFileNr(s string) (uint64, uint64, error) {
	f := strings.Fields(s)
	if len(f) != 3 {
		return 0, 0, errors.New("malformed file-nr")
	}
	var n [3]uint64
	for i := range f {
		v, err := strconv.ParseUint(f[i], 10, 64)
		if err != nil {
			return 0, 0, errors.New("malformed file-nr")
		}
		n[i] = v
	}
	if n[1] > n[0] {
		return 0, 0, errors.New("malformed file-nr")
	}
	return n[0] - n[1], n[2], nil
}

// doctorAdmission reports RUNNABLE_HELD_IDLE (TQD-V0-043) and
// ROLE_QUEUE_DEEP (TQD-V0-044) from one configured dispatcher's ATR-V0-150
// queue observation. A missing observation reports nothing; an unreadable
// one, or one older than three of its ticks, adds a scan warning and no
// finding. A deep queue is reported only at CALM pressure, so an UNKNOWN or
// unconfigured pressure level abstains, and a run-only hold only beside a
// free member, which the dispatcher already requires.
func doctorAdmission(d doctorDispatcher, th doctorThresholds, scan *doctorScan, warn func(string, ...any)) []doctorFinding {
	program := filepath.Base(d.programDir)
	// As doctorLoadLedger: no final symlink is followed and a FIFO is
	// refused, not waited on.
	raw, err := readBounded(filepath.Join(d.programDir, dispatch.AdmissionFile), dispatch.MaxAdmissionBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	var rec *dispatch.AdmissionRecord
	if err == nil {
		rec, err = dispatch.DecodeAdmission(raw, program)
	}
	if err != nil {
		warn("queue observation unreadable: %v", err)
		return nil
	}
	if age := scan.observedAt.Sub(rec.At); age > 3*time.Duration(rec.TickSeconds)*time.Second {
		warn("queue observation is stale: written %s, %d tick(s) of %ds ago", rec.At.UTC().Format(time.RFC3339), int64(age/(time.Duration(rec.TickSeconds)*time.Second)), rec.TickSeconds)
		return nil
	}
	var out []doctorFinding
	for _, h := range rec.Held {
		if h.Ticks < th.heldIdleTicks || h.Free < 1 {
			continue
		}
		fence := h.Fence
		if h.Detail != "" {
			fence += " (" + h.Detail + ")"
		}
		members := strings.Join(h.FreeMembers, ", ")
		if len(h.FreeMembers) < h.Free {
			members += ", ..."
		}
		out = append(out, doctorFinding{kind: "RUNNABLE_HELD_IDLE", who: program + "/" + h.Role + "/" + h.Ticket, firstSeen: h.Since,
			detail: fmt.Sprintf("run-only ticket %s of role %s held by %s for %d tick(s) since %s while pool %s had %d free member(s): %s",
				h.Ticket, h.Role, fence, h.Ticks, h.Since.UTC().Format(time.RFC3339), h.Pool, h.Free, members)})
	}
	// DSG-V0-005: a work state no role admits and no hold declares, and a
	// dispatcher idle beside free capacity for its idleAlertMinutes or more.
	for _, u := range rec.UnmatchedStates {
		out = append(out, doctorFinding{kind: "WORK_STATE_NO_ROLE_MATCH", who: program + "/" + u.State, firstSeen: rec.At,
			detail: fmt.Sprintf("work state %q of %d ticket(s) (for example %s) matches no enabled role and is no declared hold, so those tickets are deferred NO_ROLE_MATCH", u.State, u.Tickets, u.Example)})
	}
	if rec.IdleSince != nil && rec.IdleAlertSeconds > 0 && rec.At.Sub(*rec.IdleSince) >= time.Duration(rec.IdleAlertSeconds)*time.Second {
		parts := make([]string, 0, len(rec.IdleDeferred))
		for _, c := range rec.IdleDeferred {
			parts = append(parts, fmt.Sprintf("%s=%d", c.Reason, c.Tickets))
		}
		out = append(out, doctorFinding{kind: "IDLE_WITH_CAPACITY", who: program, firstSeen: *rec.IdleSince,
			detail: fmt.Sprintf("no launch for %d tick(s) since %s while %d of %d sessions ran; planned tickets not running, by reason: %s",
				rec.IdleTicks, rec.IdleSince.UTC().Format(time.RFC3339), rec.Running, rec.GlobalCap, strings.Join(parts, ", "))})
	}
	if rec.Pressure != dispatch.AdmissionPressureCalm {
		return out
	}
	room := rec.GlobalCap - rec.Running
	for _, q := range rec.Roles {
		if q.DeepTicks < th.queueDeepTicks || q.DeepSince == nil {
			continue
		}
		oldest := "none observed"
		if q.OldestWaitSince != nil {
			oldest = q.OldestWaitTicket + " since " + q.OldestWaitSince.UTC().Format(time.RFC3339)
		}
		capText := fmt.Sprintf("effective cap %d (cap %d", q.EffectiveCap, q.Cap)
		if q.CapMax > 0 {
			capText += fmt.Sprintf(", capMax %d", q.CapMax)
		}
		headroom := fmt.Sprintf("the global cap allows %d more session(s) (%d of %d running) and calm pressure allows a raise", room, rec.Running, rec.GlobalCap)
		if room < 1 {
			headroom = fmt.Sprintf("the global cap is full (%d of %d running), so a raise also needs a higher globalCap; calm pressure allows it", rec.Running, rec.GlobalCap)
		}
		out = append(out, doctorFinding{kind: "ROLE_QUEUE_DEEP", who: program + "/" + q.Role, firstSeen: *q.DeepSince,
			detail: fmt.Sprintf("role %s queue depth %d (%d running, %d waiting) above %s) for %d tick(s) since %s at calm pressure; oldest wait %s; headroom: %s",
				q.Role, q.Running+q.Waiting, q.Running, q.Waiting, capText, q.DeepTicks, q.DeepSince.UTC().Format(time.RFC3339), oldest, headroom)})
	}
	return out
}

// parseDoctorProcLocksByInode lists the pids of every FLOCK line for ino on
// any device. A btrfs subvolume's stat device differs from the superblock
// device /proc/locks prints, so the exact match finds nothing there; the
// caller then keeps only a pid with a descriptor on the same file (TQD-V0-821).
func parseDoctorProcLocksByInode(text string, ino uint64) []int {
	suffix := ":" + strconv.FormatUint(ino, 10)
	var out []int
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 || f[1] != "FLOCK" || !strings.HasSuffix(f[5], suffix) || strings.Count(f[5], ":") != 2 {
			continue
		}
		if pid, err := strconv.Atoi(f[4]); err == nil && pid > 0 && !containsInt(out, pid) {
			out = append(out, pid)
		}
	}
	return out
}
