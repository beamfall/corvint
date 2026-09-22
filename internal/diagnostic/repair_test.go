package diagnostic

import (
	"slices"
	"testing"
)

// TestRepairLoopTerminates checks DRC-V0-009 over a three-round transcript: a seeded input with
// three diagnostics, of which round 1 repairs one and rounds 2 and 3 repair none, stops after
// round 3 with the remaining two reported unresolved; a round reaching a new minimum keeps the
// loop going, and an empty remainder ends it.
func TestRepairLoopTerminates(t *testing.T) {
	outstanding := []string{"pkg/a.txt", "pkg/b.go", "main.go"}
	repairedPerRound := [][]string{{"pkg/a.txt"}, {}, {}, {"pkg/b.go"}}
	counts := []int{len(outstanding)}
	rounds := 0
	for !RepairStop(counts) {
		outstanding = slices.DeleteFunc(outstanding, func(subject string) bool {
			return slices.Contains(repairedPerRound[rounds], subject)
		})
		counts = append(counts, len(outstanding))
		rounds++
	}
	if rounds != 3 || !slices.Equal(outstanding, []string{"pkg/b.go", "main.go"}) {
		t.Fatalf("stopped after %d rounds with unresolved %v", rounds, outstanding)
	}
	for _, test := range []struct {
		counts []int
		stop   bool
	}{
		{[]int{3, 2, 2}, false},
		{[]int{3, 3, 2, 2}, false},
		{[]int{3, 2, 2, 1}, false},
		{[]int{3, 2, 0}, true},
		{[]int{3, 4, 3}, true},
		{nil, false},
	} {
		if got := RepairStop(test.counts); got != test.stop {
			t.Fatalf("RepairStop(%v) = %v, want %v", test.counts, got, test.stop)
		}
	}
}
