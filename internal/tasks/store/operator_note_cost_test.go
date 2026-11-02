package store_test

import (
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
)

// TestONV0885_NoteWriteCostIsBoundedByChainLength (ON-V0-885, V1-3591):
// a NOTE_SET reads only the head event of the ticket's note chain, so a
// write deep in a chain costs what a ticket's first note write costs in the
// same store. The test builds a chain of noteCostChain events through the
// native writer, then interleaves noteCostSample deep writes on that chain
// with first note writes on fresh tickets, so both samples see the same
// store size and host load, and compares their medians. The bound is loose
// (2x plus 50 ms); a per-write walk of the chain would grow past it.
func TestONV0885_NoteWriteCostIsBoundedByChainLength(t *testing.T) {
	if testing.Short() {
		t.Skip("times native note writes")
	}
	const noteCostChain, noteCostSample = 40, 8
	repo, _ := initialized(t)
	note := func(target string, revision int, id string) time.Duration {
		n := strconv.Itoa(revision)
		env := noteEnvelope("req-note-"+id, mutation.OpNoteSet, target, "", "Revision "+n+".", strconv.Itoa(revision-1))
		start := time.Now()
		r := mutate(t, repo, env)
		cost := time.Since(start)
		if r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("note %s = %s %s", id, r.Outcome.Outcome, r.Detail)
		}
		return cost
	}
	chain := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Note cost"))).Ticket
	for revision := 1; revision <= noteCostChain; revision++ {
		note(chain, revision, "chain-"+strconv.Itoa(revision))
	}
	var deep, first []time.Duration
	for i := 1; i <= noteCostSample; i++ {
		n := strconv.Itoa(i)
		deep = append(deep, note(chain, noteCostChain+i, "deep-"+n))
		fresh := mutate(t, repo, envelope("req-create-"+n, mutation.OpCreate, "", "", createPayload("Fresh "+n))).Ticket
		first = append(first, note(fresh, 1, "first-"+n))
	}
	median := func(d []time.Duration) time.Duration {
		s := append([]time.Duration(nil), d...)
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		return s[len(s)/2]
	}
	deepMedian, firstMedian := median(deep), median(first)
	t.Logf("note write median: revisions %d-%d %s, first notes %s", noteCostChain+1, noteCostChain+noteCostSample, deepMedian, firstMedian)
	if deepMedian > 2*firstMedian+50*time.Millisecond {
		t.Fatalf("note write cost grew with the chain: deep median %s, first-note median %s", deepMedian, firstMedian)
	}
}
