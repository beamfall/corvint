//go:build darwin || linux

package groupreap

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"
	"time"
)

// Escaped-descendant containment (PGO-V0-007). A descendant that leaves the
// leader's process group (a detached browser calling setsid, for example)
// survives the group SIGKILL. RunContained retires such descendants only
// while their ownership is structural: the leader is exited and unreaped, the
// remaining group is stopped, and every retired process descends through
// stopped owned parents from that group, so no retired PID can be reaped and
// reused before its signal. A descendant orphaned before the sweep (its owned
// parent exited first, as when a runner tears down its worker on SIGINT) is
// retired only by the identity it had when a run-time sample proved it owned:
// the same PID with the same kernel start time. That last step has a residual
// window between the table read and the signal that the structural sweep does
// not have; it is documented in PGO-V0-007.
//
// A stopped parent does not hold its child's PID when the parent auto-reaps
// (SIGCHLD ignored or SA_NOCLDWAIT), so every individual signal is also pinned
// to the target's identity (signalPinned): on Linux through a pidfd checked
// against the start time, on Darwin by a fresh identity read just before the
// signal.

// ProcState is a process-table state.
type ProcState int

const (
	StateRunning ProcState = iota
	StateStopped
	StateZombie
)

// Process is one process-table identity. Start is the kernel start time
// (Darwin microseconds since the epoch, Linux clock ticks since boot).
type Process struct {
	PID, PPID, PGID int
	Start           int64
	State           ProcState
}

func (p Process) String() string {
	return fmt.Sprintf("pid=%d ppid=%d pgid=%d start=%d", p.PID, p.PPID, p.PGID, p.Start)
}

func (p Process) same(q Process) bool { return p.PID == q.PID && p.Start == q.Start }

// Containment is the escaped-descendant outcome of RunContained. Only
// Complete() proves that no observed escaped descendant survived.
type Containment struct {
	// Retired are escaped owned descendants signalled under the structural proof.
	Retired []Process
	// Survivors are escaped identities still present (not zombies) when the
	// bounded post-retirement observation ended. They are never signalled
	// unless they were also Retired.
	Survivors []Process
	// Err is why the proof or the observation could not be completed.
	Err error
}

// Complete reports whether containment was proved with no survivor.
func (c Containment) Complete() bool { return c.Err == nil && len(c.Survivors) == 0 }

var errProcessTable = errors.New("groupreap: process table unavailable")

// errIdentityChanged reports that a PID no longer names the identity the
// caller proved owned; nothing was signalled.
var errIdentityChanged = errors.New("groupreap: process identity changed")

// notSignalled reports a target that was gone or replaced before the signal.
func notSignalled(err error) bool {
	return errors.Is(err, syscall.ESRCH) || errors.Is(err, errIdentityChanged)
}

const (
	// sampleInterval separates run-time observations of escaped descendants.
	sampleInterval = 200 * time.Millisecond
	// freezeBound limits stopping the owned tree before retirement.
	freezeBound = 2 * time.Second
	// survivorBound limits the signal-free post-retirement observation.
	survivorBound = 2 * time.Second
	sweepPoll     = 2 * time.Millisecond
	// maxEscaped bounds the retained escaped identities of one run.
	maxEscaped = 4096
)

// Test hooks: the process table, identity-pinned signals and sessions.
// signalProcess(p, group, sig) signals p (its process group when group is
// true) only while p's PID still names p's start time.
var (
	snapshotProcesses = readProcessTable
	// sampleProcesses is the run-time sampler's table read.
	sampleProcesses = func() (map[int]Process, error) { return snapshotProcesses() }
	signalProcess   = signalPinned
	sessionOf       = processSession
)

// RunContained is Run plus bounded retirement of escaped owned descendants.
// It starts command as the leader of a new process group. The returned error
// is the leader's exit status as Wait reports it.
func RunContained(command *exec.Cmd) (Containment, error) {
	return RunContainedRetiring(command, nil)
}

// RunContainedRetiring is RunContained with r also retiring the phase's
// token- or ancestry-owned processes (TRE-V0-030) once the leader exits and
// before it is reaped, ahead of the structural escaped-descendant pass. A nil
// r is RunContained.
func RunContainedRetiring(command *exec.Cmd, r *Retirer) (Containment, error) {
	containLeader(command)
	if err := command.Start(); err != nil {
		return Containment{}, err
	}
	leader := command.Process.Pid
	if r != nil {
		r.Leader(leader)
	}
	seen := map[int]sampled{}
	var sampleErr error
	overflow := false
	stop, sampling := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(sampling)
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			table, err := sampleProcesses()
			if err != nil {
				sampleErr = err
				continue
			}
			if !recordEscaped(seen, table, leader) {
				overflow = true
			}
		}
	}()
	exitErr := leaderUnreaped(leader)
	close(stop)
	<-sampling
	var c Containment
	if exitErr != nil {
		// Without the unreaped leader nothing is provably owned: keep the
		// legacy order and retire no escaped descendant.
		err := command.Wait()
		_ = signalGroup(-leader, syscall.SIGKILL)
		if r != nil {
			r.Retire()
		}
		c.Err = errors.Join(errors.New("groupreap: leader exit observation unavailable"), exitErr)
		return c, err
	}
	var freezeErr error
	if r != nil {
		// The token-owned record is taken first, while the exited leader is
		// unreaped, so the structural pass cannot pre-empt it. The owned tree
		// is frozen and its escaped identities recorded beforehand: a
		// descendant the Retirer cannot prove (no token, and no ancestry from
		// the exited leader) is orphaned out of the structural tree when the
		// Retirer kills its parent, and is then retired by that identity.
		if table, _, err := freezeOwned(leader); err != nil {
			freezeErr = err
		} else if !recordEscaped(seen, table, leader) {
			overflow = true
		}
		r.Retire()
	}
	c.Retired, c.Err = retireEscaped(leader)
	c.Err = errors.Join(freezeErr, c.Err)
	_ = signalGroup(-leader, syscall.SIGKILL)
	// Orphans are retired before Wait: one holding the command's inherited
	// output pipes would otherwise keep Wait's copy goroutines open.
	orphans, orphanErr := retireSampledOrphans(seen, c.Retired)
	c.Retired = append(c.Retired, orphans...)
	if orphanErr != nil {
		c.Err = errors.Join(c.Err, orphanErr)
	}
	err := command.Wait()
	if sampleErr != nil {
		c.Err = errors.Join(c.Err, sampleErr)
	}
	if overflow {
		c.Err = errors.Join(c.Err, fmt.Errorf("groupreap: more than %d escaped identities; some were not tracked", maxEscaped))
	}
	survivors, observeErr := observeSurvivors(c.Retired, seen)
	c.Survivors = survivors
	if observeErr != nil {
		c.Err = errors.Join(c.Err, observeErr)
	}
	return c, err
}

// sampled is an escaped identity a run-time sample proved owned, with its
// depth below the nearest group member at that sample.
type sampled struct {
	Process
	depth int
}

// recordEscaped adds the table's escaped owned identities to seen. It reports
// false when the bound left a new identity untracked.
func recordEscaped(seen map[int]sampled, table map[int]Process, leader int) bool {
	owned := ownedTree(table, leader)
	complete := true
	for _, p := range escapedOf(table, owned) {
		if prev, ok := seen[p.PID]; (ok && prev.same(p)) || len(seen) < maxEscaped {
			seen[p.PID] = sampled{p, owned[p.PID]}
		} else {
			complete = false
		}
	}
	return complete
}

// retireSampledOrphans SIGKILLs, deepest first, each sampled escaped identity
// that was not retired by the structural sweep and still has the same PID and
// start time in a fresh table. A changed start time is a reused PID and is
// never signalled.
func retireSampledOrphans(seen map[int]sampled, retired []Process) ([]Process, error) {
	if len(seen) == 0 {
		return nil, nil
	}
	done := map[int]bool{}
	for _, p := range retired {
		if q, ok := seen[p.PID]; ok && q.same(p) {
			done[p.PID] = true
		}
	}
	table, err := snapshotProcesses()
	if err != nil {
		return nil, err
	}
	var candidates []sampled
	for pid, p := range seen {
		if q, ok := table[pid]; ok && !done[pid] && q.same(p.Process) && q.State != StateZombie {
			candidates = append(candidates, p)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].depth != candidates[j].depth {
			return candidates[i].depth > candidates[j].depth
		}
		return candidates[i].PID < candidates[j].PID
	})
	var out []Process
	var errs error
	for _, p := range candidates {
		if err := signalProcess(p.Process, false, syscall.SIGKILL); err != nil {
			if !notSignalled(err) {
				errs = errors.Join(errs, fmt.Errorf("groupreap: retiring orphaned escaped descendant %s: %w", p.Process, err))
			}
			continue
		}
		out = append(out, p.Process)
	}
	return out, errs
}

// ownedTree returns the processes owned through the leader's group: members
// of group leader plus their descendants by parent link. The caller must
// keep the leader unreaped (or running) so the group ID is the leader's.
// depth is the distance from the nearest group member.
func ownedTree(table map[int]Process, leader int) map[int]int {
	self := os.Getpid()
	children := map[int][]int{}
	owned := map[int]int{}
	var queue []int
	for pid, p := range table {
		if pid <= 1 || pid == self {
			continue
		}
		children[p.PPID] = append(children[p.PPID], pid)
		if p.PGID == leader {
			owned[pid] = 0
			queue = append(queue, pid)
		}
	}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		for _, child := range children[parent] {
			if _, ok := owned[child]; ok {
				continue
			}
			// A child cannot be older than its parent; anything else is a
			// stale or reused parent link and confers no ownership.
			if table[child].Start < table[parent].Start {
				continue
			}
			owned[child] = owned[parent] + 1
			queue = append(queue, child)
		}
	}
	return owned
}

func escapedOf(table map[int]Process, owned map[int]int) []Process {
	var out []Process
	for pid, depth := range owned {
		if depth > 0 && table[pid].State != StateZombie {
			out = append(out, table[pid])
		}
	}
	return out
}

// retireEscaped stops the leader's group and every owned descendant, then
// SIGKILLs escaped descendants deepest first. A process is stopped or killed
// individually only while its parent is an owned process observed stopped,
// so its PID cannot be reaped and reused before the signal; a descendant that
// leads its own session is retired with one group signal, because every
// member of that session's groups descends from it.
func retireEscaped(leader int) ([]Process, error) {
	table, owned, err := freezeOwned(leader)
	if err != nil {
		return nil, err
	}
	escaped := escapedOf(table, owned)
	sort.Slice(escaped, func(i, j int) bool {
		if owned[escaped[i].PID] != owned[escaped[j].PID] {
			return owned[escaped[i].PID] > owned[escaped[j].PID]
		}
		return escaped[i].PID < escaped[j].PID
	})
	var retired []Process
	var errs error
	for _, p := range escaped {
		if !stoppedOwnedParent(table, owned, p) {
			errs = errors.Join(errs, fmt.Errorf("groupreap: escaped descendant %s lost its stopped owned parent", p))
			continue
		}
		group := false
		if p.PGID == p.PID {
			if sid, err := sessionOf(p.PID); err == nil && sid == p.PID {
				group = true
			}
		}
		if err := signalProcess(p, group, syscall.SIGKILL); err != nil {
			if !notSignalled(err) {
				errs = errors.Join(errs, fmt.Errorf("groupreap: retiring escaped descendant %s: %w", p, err))
			}
			continue
		}
		retired = append(retired, p)
	}
	return retired, errs
}

// freezeOwned stops the leader's group and every owned descendant within the
// bound and returns the stopped table and owned tree. A descendant is stopped
// individually only while its owned parent is observed stopped.
func freezeOwned(leader int) (map[int]Process, map[int]int, error) {
	deadline := time.Now().Add(freezeBound)
	var table map[int]Process
	var owned map[int]int
	for {
		// The exited leader is unreaped, so its group ID is still ours.
		if err := signalGroup(-leader, syscall.SIGSTOP); err != nil && err != syscall.ESRCH && err != syscall.EPERM {
			return nil, nil, fmt.Errorf("groupreap: stopping the owned group: %w", err)
		}
		var err error
		if table, err = snapshotProcesses(); err != nil {
			return nil, nil, err
		}
		owned = ownedTree(table, leader)
		running := false
		for pid, depth := range owned {
			p := table[pid]
			if p.State != StateRunning {
				continue
			}
			running = true
			if depth > 0 && stoppedOwnedParent(table, owned, p) {
				if err := signalProcess(p, false, syscall.SIGSTOP); err != nil && !notSignalled(err) {
					return nil, nil, fmt.Errorf("groupreap: stopping escaped descendant %s: %w", p, err)
				}
			}
		}
		if !running {
			return table, owned, nil
		}
		if time.Now().After(deadline) {
			return nil, nil, errors.New("groupreap: owned descendants did not stop within the bound")
		}
		time.Sleep(sweepPoll)
	}
}

func stoppedOwnedParent(table map[int]Process, owned map[int]int, p Process) bool {
	parent, ok := table[p.PPID]
	if _, isOwned := owned[p.PPID]; !ok || !isOwned {
		return false
	}
	return parent.State == StateStopped && parent.Start <= p.Start
}

// observeSurvivors polls the process table, without signalling, until no
// retired or sampled escaped identity remains alive or the bound ends.
func observeSurvivors(retired []Process, seen map[int]sampled) ([]Process, error) {
	watch := map[int]Process{}
	for _, p := range seen {
		watch[p.PID] = p.Process
	}
	for _, p := range retired {
		watch[p.PID] = p
	}
	if len(watch) == 0 {
		return nil, nil
	}
	deadline := time.Now().Add(survivorBound)
	for {
		table, err := snapshotProcesses()
		if err != nil {
			return nil, err
		}
		var alive []Process
		for _, p := range watch {
			if q, ok := table[p.PID]; ok && q.same(p) && q.State != StateZombie {
				alive = append(alive, p)
			}
		}
		if len(alive) == 0 {
			return nil, nil
		}
		if time.Now().After(deadline) {
			sort.Slice(alive, func(i, j int) bool { return alive[i].PID < alive[j].PID })
			return alive, nil
		}
		time.Sleep(sweepPoll * 5)
	}
}
