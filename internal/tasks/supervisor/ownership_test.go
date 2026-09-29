//go:build darwin || linux

package supervisor

import (
	"syscall"
	"testing"
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
