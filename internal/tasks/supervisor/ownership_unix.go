//go:build darwin || linux

package supervisor

import (
	"fmt"
	"syscall"
	"time"
)

// ownedGroup never adopts a numeric group without a retained live identity.
// A disappeared anchor plus a nonempty group is uncertainty, not ownership.
type ownedGroup struct {
	group     int
	members   map[int]string
	identity  func(int) (string, error)
	groupOf   func(int) (int, error)
	inventory func(int) (map[int]string, error)
	exists    func(int) (bool, error)
	signal    func(map[int]string, syscall.Signal) error
	// force, when set, also retires the group leader: SIGTERM until force
	// elapses, then SIGKILL. The supervisor's own leader is never forced.
	force time.Duration
}

func groupExists(group int) (bool, error) {
	e := syscall.Kill(-group, 0)
	if e == syscall.ESRCH {
		return false, nil
	}
	if e != nil {
		return true, e
	}
	return true, nil
}
func newOwnedGroup(boot Boot) *ownedGroup {
	return &ownedGroup{group: boot.PID, members: map[int]string{boot.PID: boot.Started}, identity: ProcessIdentity, groupOf: syscall.Getpgid, inventory: groupMembers, exists: groupExists, signal: signalMembers}
}
func (g *ownedGroup) anchor() (int, string, error) {
	pids := []int{g.group}
	for pid := range g.members {
		if pid != g.group {
			pids = append(pids, pid)
		}
	}
	for _, pid := range pids {
		id := g.members[pid]
		now, e := g.identity(pid)
		if e != nil {
			return 0, "", e
		}
		if id != "" && now == id {
			group, e := g.groupOf(pid)
			if e == syscall.ESRCH {
				continue
			}
			if e != nil {
				return 0, "", e
			}
			again, e := g.identity(pid)
			if e != nil {
				return 0, "", e
			}
			if group == g.group && again == id {
				return pid, id, nil
			}
		}
	}
	return 0, "", nil
}
func (g *ownedGroup) observe() error {
	pid, id, e := g.anchor()
	if e != nil {
		return e
	}
	if pid == 0 {
		live, e := g.exists(g.group)
		if e != nil {
			return e
		}
		if live {
			return fmt.Errorf("group remains without retained live identity")
		}
		return nil
	}
	current, e := g.inventory(g.group)
	if e != nil {
		return e
	}
	again, e := g.identity(pid)
	if e != nil {
		return e
	}
	group, groupErr := g.groupOf(pid)
	if again != id || groupErr != nil || group != g.group {
		return fmt.Errorf("ownership anchor changed during observation")
	}
	for p, start := range current {
		if start == "" {
			return fmt.Errorf("member disappeared during group observation")
		}
		g.members[p] = start
	}
	return nil
}
func (g *ownedGroup) drain() bool {
	forceAt := time.Now().Add(10 * time.Second)
	if g.force > 0 {
		forceAt = time.Now().Add(g.force)
	}
	deadline := forceAt.Add(5 * time.Second)
	for time.Now().Before(deadline) {
		live, e := g.exists(g.group)
		if e != nil {
			return false
		}
		if !live {
			return true
		}
		if e = g.observe(); e != nil {
			if pid, _, err := g.anchor(); err != nil || pid == 0 {
				return false
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		current, e := g.inventory(g.group)
		if e != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		children := map[int]string{}
		for pid, id := range current {
			if pid != g.group {
				if id == "" || g.members[pid] != id {
					return false
				}
				children[pid] = id
			}
		}
		if len(children) == 0 && g.force > 0 {
			sig := syscall.SIGTERM
			if time.Now().After(forceAt) {
				sig = syscall.SIGKILL
			}
			if e = g.signal(map[int]string{g.group: g.members[g.group]}, sig); e != nil {
				return false
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if len(children) == 0 {
			if e = g.signal(map[int]string{g.group: g.members[g.group]}, syscall.SIGTERM); e != nil {
				return false
			}
			until := deadline
			for time.Now().Before(until) {
				live, e = g.exists(g.group)
				if e != nil {
					return false
				}
				if !live {
					return true
				}
				time.Sleep(20 * time.Millisecond)
			}
			return false
		}
		// Retain waiting parents long enough to reap children before stopping them.
		newest := 0
		for pid := range children {
			if pid > newest {
				newest = pid
			}
		}
		targets := map[int]string{newest: children[newest]}
		sig := syscall.SIGTERM
		if time.Now().After(forceAt) {
			targets = children
			sig = syscall.SIGKILL
		}
		if e = g.signal(targets, sig); e != nil {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// Recover drains only a retained leader/start binding. Absence is checked at
// the kernel group boundary; a reused or escaped identity remains uncertain.
func Recover(boot Boot) bool {
	if boot.PID <= 0 || boot.Started == "" {
		return false
	}
	return newOwnedGroup(boot).drain()
}
