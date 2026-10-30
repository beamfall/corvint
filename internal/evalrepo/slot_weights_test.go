package evalrepo

import "testing"

func TestLTAV0010SlotDeltaClassification(t *testing.T) {
	base := slotArm{cases: []slotCaseScore{{criticalMisses: 1}, {}, {}, {}}}
	cases := []struct {
		name  string
		arm   slotArm
		label string
	}{
		{"two cases gain top five", slotArm{cases: []slotCaseScore{{criticalMisses: 1}, {top5Hit: true}, {top5Hit: true}, {}}}, SlotDeltaImproved},
		{"one case is not distinguished", slotArm{cases: []slotCaseScore{{criticalMisses: 1}, {top5Hit: true}, {}, {}}}, SlotDeltaNotDistinguished},
		{"identical arm", base, SlotDeltaNotDistinguished},
		{"critical miss rises despite gains", slotArm{cases: []slotCaseScore{{criticalMisses: 1}, {criticalMisses: 1, top5Hit: true}, {mustHit: 1}, {mustHit: 1}}}, SlotDeltaRegressed},
		{"critical miss recovered and one gain", slotArm{cases: []slotCaseScore{{}, {mustHit: 1}, {}, {}}}, SlotDeltaImproved},
	}
	for _, item := range cases {
		delta := slotDelta(base, item.arm)
		if delta["classification"] != item.label {
			t.Fatalf("%s: delta = %v", item.name, delta)
		}
	}
}

func TestLTAV0010SelectorPathsKeepOnlyPathBearingSelectors(t *testing.T) {
	got := selectorPaths([]string{"symbol:cache/demux.go:Split", "file:docs/a.md", "feature:reveal", "scenario:x", "symbol:nopath"})
	if len(got) != 2 || got[0] != "cache/demux.go" || got[1] != "docs/a.md" {
		t.Fatalf("paths = %v", got)
	}
}
