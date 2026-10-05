//go:build darwin || linux

package supervisor

import (
	"syscall"
	"testing"
	"time"
)

func TestSupervisorOwnershipRejectsReusedAndEscapedAnchors(t *testing.T) {
	for _, mode := range []string{"reused", "escaped", "late-child"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			identity := "owned"
			group := 12
			g := &ownedGroup{group: 12, members: map[int]string{12: "owned"}, identity: func(int) (string, error) { return identity, nil }, groupOf: func(int) (int, error) { return group, nil }, exists: func(int) (bool, error) { return true, nil }, signal: func(map[int]string, syscall.Signal) error { t.Fatal("unowned signal"); return nil }}
			g.inventory = func(int) (map[int]string, error) { calls++; identity = ""; return map[int]string{}, nil }
			switch mode {
			case "reused":
				identity = "new"
			case "escaped":
				group = 99
			}
			if err := g.observe(); err == nil {
				t.Fatal("uncertain ownership admitted")
			}
			if mode != "late-child" && calls != 0 {
				t.Fatal("adopted unrelated inventory")
			}
			if len(g.members) != 1 {
				t.Fatal("adopted new member")
			}
		})
	}
}

// TestCALV0086_DrainWaitsOutUnprovableGroupProbe pins the V1-0772 drain
// bound: an EPERM group probe (Darwin's answer for a group of unreaped
// zombies) is re-probed until the group is proved gone, never signalled or
// reported as survivors at once, and never itself counted as gone.
func TestCALV0086_DrainWaitsOutUnprovableGroupProbe(t *testing.T) {
	g := &ownedGroup{group: 12, members: map[int]string{12: "owned"}, identity: func(int) (string, error) { return "owned", nil }, groupOf: func(int) (int, error) { return 12, nil }, inventory: func(int) (map[int]string, error) { return map[int]string{12: "owned"}, nil }, signal: func(map[int]string, syscall.Signal) error { t.Fatal("signalled an unobservable group"); return nil }}
	probes := 0
	g.exists = func(int) (bool, error) {
		probes++
		if probes <= 3 {
			return true, syscall.EPERM
		}
		return false, nil
	}
	if !g.drain() || probes != 4 {
		t.Fatalf("a group proved gone after EPERM probes was not clean (%d probes)", probes)
	}
	g.force = time.Millisecond
	g.exists = func(int) (bool, error) { return true, syscall.EPERM }
	if g.drain() {
		t.Fatal("a group that never stops answering EPERM was counted gone")
	}
}
