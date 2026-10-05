package wire

import (
	"fmt"
	"slices"
	"testing"
)

// TestIssue502_EscalationPendingPredicate: the shared ESC-V0-006 predicate
// holds only current OPEN decision, scope and blocked questions, names them
// sorted and deduplicated, and bounds the list to EscalationMaxCurrentOpen.
func TestIssue502_EscalationPendingPredicate(t *testing.T) {
	e := func(id, acceptance, kind, state string) EscalationHoldEntry {
		return EscalationHoldEntry{RequestID: id, AcceptanceRevision: acceptance, Kind: kind, State: state}
	}
	cases := []struct {
		name    string
		entries []EscalationHoldEntry
		want    []string
	}{
		{"none", nil, nil},
		{"decision", []EscalationHoldEntry{e("q-a", "2", "decision", "OPEN")}, []string{"q-a"}},
		{"scope", []EscalationHoldEntry{e("q-a", "2", "scope", "OPEN")}, []string{"q-a"}},
		{"blocked", []EscalationHoldEntry{e("q-a", "2", "blocked", "OPEN")}, []string{"q-a"}},
		{"infrastructure", []EscalationHoldEntry{e("q-a", "2", "infrastructure", "OPEN")}, nil},
		{"stale", []EscalationHoldEntry{e("q-a", "1", "decision", "OPEN")}, nil},
		{"answered", []EscalationHoldEntry{e("q-a", "2", "decision", "ANSWERED")}, nil},
		{"superseded", []EscalationHoldEntry{e("q-a", "2", "scope", "SUPERSEDED")}, nil},
		{"unknown kind", []EscalationHoldEntry{e("q-a", "2", "other", "OPEN")}, nil},
		{"noncanonical revision", []EscalationHoldEntry{e("q-a", "02", "decision", "OPEN")}, nil},
		{"unsorted and duplicate", []EscalationHoldEntry{e("q-c", "2", "blocked", "OPEN"), e("q-a", "2", "decision", "OPEN"), e("q-b", "2", "infrastructure", "OPEN"), e("q-a", "2", "decision", "OPEN")}, []string{"q-a", "q-c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, total := EscalationPending("2", tc.entries)
			if !slices.Equal(got, tc.want) || total != len(tc.want) {
				t.Fatalf("got %v (%d), want %v", got, total, tc.want)
			}
		})
	}
	t.Run("bound", func(t *testing.T) {
		var many []EscalationHoldEntry
		for i := 20; i > 0; i-- {
			many = append(many, e(fmt.Sprintf("q-%02d", i), "2", "decision", "OPEN"))
		}
		got, total := EscalationPending("2", many)
		if total != 20 || len(got) != EscalationMaxCurrentOpen || got[0] != "q-01" || got[len(got)-1] != "q-16" || !slices.IsSorted(got) {
			t.Fatalf("bound: %v (%d)", got, total)
		}
		got, total = EscalationPending("2", many[4:])
		if total != EscalationMaxCurrentOpen || len(got) != EscalationMaxCurrentOpen {
			t.Fatalf("exactly the bound: %v (%d)", got, total)
		}
	})
}
