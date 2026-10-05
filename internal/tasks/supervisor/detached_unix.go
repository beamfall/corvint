//go:build darwin || linux

package supervisor

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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
)

type processRow struct {
	pid, ppid, pgid int
	zombie          bool
}

func processRows() ([]processRow, error) {
	raw, e := exec.Command("/bin/ps", "-axo", "pid=,ppid=,pgid=,stat=").Output()
	if e != nil {
		return nil, e
	}
	var rows []processRow
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 4 {
			return nil, fmt.Errorf("unexpected process row")
		}
		var r processRow
		var e1, e2, e3 error
		r.pid, e1 = strconv.Atoi(f[0])
		r.ppid, e2 = strconv.Atoi(f[1])
		r.pgid, e3 = strconv.Atoi(f[2])
		if e1 != nil || e2 != nil || e3 != nil {
			return nil, fmt.Errorf("unexpected process row")
		}
		r.zombie = strings.HasPrefix(f[3], "Z")
		rows = append(rows, r)
	}
	return rows, nil
}

// escapes are the process groups a detached host started outside the owned
// group: a live process whose parent belongs to the owned group (or to an
// escape already found) but whose own group differs. An escape must lead its
// own group; anything else, or an observation that fails, is uncertainty.
type escapes struct {
	owned     int
	groups    map[int]string
	uncertain bool
	rows      func() ([]processRow, error)
	identity  func(int) (string, error)
	exists    func(int) (bool, error)
}

func newEscapes(owned int) *escapes {
	return &escapes{owned: owned, groups: map[int]string{}, rows: processRows, identity: ProcessIdentity, exists: groupExists}
}

func (x *escapes) known(group int) bool {
	_, ok := x.groups[group]
	return group == x.owned || ok
}

// scan records every escape visible now. It returns whether a member of the
// owned group other than its leader (the host itself) was alive, the
// positive evidence that every escape still has a live, observable parent.
func (x *escapes) scan() bool {
	rows, e := x.rows()
	if e != nil {
		x.uncertain = true
		return false
	}
	groupOf := map[int]int{}
	host := false
	for _, r := range rows {
		groupOf[r.pid] = r.pgid
		if r.pgid == x.owned && r.pid != x.owned && !r.zombie {
			host = true
		}
	}
	for found := true; found; {
		found = false
		for _, r := range rows {
			parent, ok := groupOf[r.ppid]
			if r.zombie || x.known(r.pgid) || !ok || !x.known(parent) {
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
			x.groups[r.pid] = id
			found = true
		}
	}
	return host
}

// drain retires every recorded escape group: children first, then the group
// leader with SIGTERM and, after escapeForceAfter, SIGKILL. It is true only
// when no observation was uncertain and every group is gone.
func (x *escapes) drain() bool {
	clean := !x.uncertain
	for pid, id := range x.groups {
		g := newOwnedGroup(Boot{PID: pid, Started: id})
		g.force = escapeForceAfter
		if !g.drain() {
			clean = false
		}
	}
	return clean
}

// RecoverHost drains a prior run's retained leader group (CAL-V0-077). For a
// detached host it first requires the host itself to be alive, so that every
// escape is still discoverable through its parent, then drains the leader
// group and each escape; otherwise quiescence stays uncertain.
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
	if id, e := ProcessIdentity(boot.PID); e != nil || id != boot.Started {
		return false
	}
	x := newEscapes(boot.PID)
	alive := x.scan()
	owned := Recover(boot)
	return x.drain() && owned && alive
}
