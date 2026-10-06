//go:build darwin || linux

package supervisor

import (
	"syscall"
	"time"
)

// Detached-host bounds (CAL-V0-077). escapeForceAfter is how long a drained
// escaped group receives SIGTERM before SIGKILL; detachedQuiesce bounds the
// wait, after every observed group is drained, for end of file on the host
// output pipes. Tests shorten both.
var (
	escapeForceAfter = 5 * time.Second
	detachedQuiesce  = 5 * time.Second
	escapeScanEvery  = 200 * time.Millisecond
	// hostExitPollMin is the first host exit capsule poll interval; the
	// backoff doubles it up to escapeScanEvery (CAL-V0-137).
	hostExitPollMin = 10 * time.Millisecond
)

type processRow struct {
	pid, ppid, pgid int
	zombie          bool
}

// escapes are the process groups a detached host started outside the owned
// group: a live process whose parent belongs to the owned group (or to an
// escape already found) but whose own group differs. An escape must lead its
// own group; anything else, or an observation that fails, is uncertainty.
//
// Numeric group membership is never trusted on its own (CAL-V0-077): a group
// is expanded only through its leader identity, recorded before the process
// snapshot and still live, still leading that group, after it. A candidate is
// recorded only when a later snapshot still shows it, with the identity read
// before that snapshot, as a child of such an anchored group.
type escapes struct {
	owned     Boot
	groups    map[int]string
	uncertain bool
	rows      func() ([]processRow, error)
	identity  func(int) (string, error)
	groupOf   func(int) (int, error)
	exists    func(int) (bool, error)
	newGroup  func(Boot) *ownedGroup
}

// maxEscapeRounds bounds the snapshots of one scan; each recorded escape
// level needs two (candidate, then confirmation).
const maxEscapeRounds = 16

func newEscapes(owned Boot) *escapes {
	return &escapes{owned: owned, groups: map[int]string{}, rows: processRows, identity: ProcessIdentity, groupOf: syscall.Getpgid, exists: groupExists, newGroup: newOwnedGroup}
}

func (x *escapes) known(group int) bool {
	_, ok := x.groups[group]
	return group == x.owned.PID || ok
}

// anchor reports whether group is still led by the identity recorded for it.
// A leader PID now held by another live process proves the recorded group
// ended, since a PID is not reused while its process group exists; a missing
// leader with a live group is uncertainty.
func (x *escapes) anchor(group int, id string) (anchored, gone bool) {
	now, e := x.identity(group)
	if e != nil {
		x.uncertain = true
		return false, false
	}
	if id != "" && now == id {
		pg, e := x.groupOf(group)
		again, e2 := x.identity(group)
		if e == nil && e2 == nil && pg == group && again == id {
			return true, false
		}
		x.uncertain = true
		return false, false
	}
	if now != "" {
		return false, true
	}
	live, e := x.exists(group)
	if e != nil || live {
		x.uncertain = true
		return false, false
	}
	return false, true
}

// scan records every escape it can prove, taking snapshots until one shows
// nothing new.
func (x *escapes) scan() {
	pending := map[int]string{}
	for round := 0; ; round++ {
		if round == maxEscapeRounds {
			x.uncertain = true
			return
		}
		rows, e := x.rows()
		if e != nil {
			x.uncertain = true
			return
		}
		anchored := map[int]bool{}
		if ok, _ := x.anchor(x.owned.PID, x.owned.Started); ok {
			anchored[x.owned.PID] = true
		}
		for g, id := range x.groups {
			ok, gone := x.anchor(g, id)
			anchored[g] = ok
			if gone {
				delete(x.groups, g)
			}
		}
		groupOf := map[int]int{}
		for _, r := range rows {
			groupOf[r.pid] = r.pgid
		}
		seen := map[int]bool{}
		found := false
		for _, r := range rows {
			parent, ok := groupOf[r.ppid]
			if r.zombie || x.known(r.pgid) || !ok || !anchored[parent] {
				continue
			}
			if r.pgid != r.pid {
				x.uncertain = true
				continue
			}
			id, e := x.identity(r.pid)
			if e != nil {
				x.uncertain = true
				continue
			}
			if id == "" {
				// Exited between observations: harmless only if its group
				// is gone with it.
				if live, e := x.exists(r.pid); e != nil || live {
					x.uncertain = true
				}
				continue
			}
			seen[r.pid] = true
			found = true
			if pending[r.pid] == id {
				x.groups[r.pid] = id
				delete(pending, r.pid)
			} else {
				pending[r.pid] = id
			}
		}
		for pid, id := range pending {
			if seen[pid] {
				continue
			}
			delete(pending, pid)
			// No longer a child of an anchored group: unprovable while it,
			// or a group under its PID, may still live.
			now, e := x.identity(pid)
			if e != nil || now == id {
				x.uncertain = true
			} else if now == "" {
				if live, e := x.exists(pid); e != nil || live {
					x.uncertain = true
				}
			}
		}
		if !found {
			return
		}
	}
}

// drain retires every recorded escape group: children first, then the group
// leader with SIGTERM and, after escapeForceAfter, SIGKILL. Each drain
// revalidates the leader identity before it signals. It is true only when no
// observation was uncertain and every group is gone.
func (x *escapes) drain() bool {
	clean := !x.uncertain
	for pid, id := range x.groups {
		g := x.newGroup(Boot{PID: pid, Started: id})
		g.force = escapeForceAfter
		if !g.drain() {
			clean = false
		}
	}
	return clean
}

// afterRecoveryScan runs between discovery and drain in RecoverHost; tests
// use it to start an escape in that window.
var afterRecoveryScan = func() {}

// RecoverHost drains a prior run's retained leader group (CAL-V0-077). A
// detached host's recovery is never proved: an escape started after the
// discovery snapshot, or orphaned before it, has no link to the retained
// group, and no witness that survives the supervisor's crash (the host output
// pipes are gone with it) proves its absence. Recovery still drains every
// escape it can observe, then reports quiescence uncertain.
func RecoverHost(dir string, boot Boot) bool {
	if boot.PID <= 0 || boot.Started == "" {
		return false
	}
	raw, e := ReadBounded(dir+"/capsule", MaxCapsule)
	if e != nil {
		return false
	}
	var c Capsule
	if decode(raw, &c) != nil {
		return false
	}
	host, ok := HostVocabulary(c.Host)
	if !ok {
		return false
	}
	if !host.Detached {
		return Recover(boot)
	}
	if id, e := ProcessIdentity(boot.PID); e == nil && id == boot.Started {
		x := newEscapes(boot)
		x.scan()
		afterRecoveryScan()
		Recover(boot)
		x.drain()
	}
	return false
}

// nextHostExitPoll doubles the host exit capsule poll interval up to
// escapeScanEvery (CAL-V0-137), so a detached host keeps its scan cadence.
func nextHostExitPoll(poll time.Duration) time.Duration {
	return max(min(2*poll, escapeScanEvery), hostExitPollMin)
}
