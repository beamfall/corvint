package store

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0740_NoteWriteCostIndependentOfChainLength pins V1-3591: on the
// writer-checkpoint route a NOTE_SET on a ticket whose note chain is long
// costs what the first note on a fresh ticket costs at the same store head.
// The write reads only the prior event, never the chain, so its cost tracks
// the journal and not the chain (ON-V0-740). The cost is the heap
// allocation count, which, unlike wall time, does not move with host load.
func TestONV0740_NoteWriteCostIndependentOfChainLength(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	repo := writerStore(t, 70)
	now := WallClock()
	long := stagedWrite(writerCreate(repo, "chain-long", now), nil)
	fresh := stagedWrite(writerCreate(repo, "chain-fresh", now), nil)
	if !long.completed() || !fresh.completed() {
		t.Fatalf("create: %+v %v / %+v %v", long.rep, long.err, fresh.rep, fresh.err)
	}
	note := func(id, target string, revision int) []byte {
		s := wire.String
		supersedes := wire.Null()
		if revision > 1 {
			supersedes = s(strconv.Itoa(revision - 1))
		}
		return wire.EncodeFile(historyObject(
			"profile", s(mutation.Profile),
			"requestId", s(id),
			"actor", historyObject("id", s(historyActor.ID), "role", s(historyActor.Role)),
			"queueId", s(fixture.QueueID),
			"targetId", s(target),
			"expectedRevision", wire.Null(),
			"operation", s(mutation.OpNoteSet),
			"payload", historyObject("supersedes", supersedes, "text", s("Revision "+strconv.Itoa(revision)+".")),
			"issuedAt", s("2026-09-07T12:00:00Z"),
		))
	}
	write := func(id, target string, revision int) uint64 {
		t.Helper()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		before := ms.Mallocs
		run := stagedWrite(writerEnvelope(repo, note(id, target, revision), WallClock()), nil)
		runtime.ReadMemStats(&ms)
		if !run.completed() || !run.fast() || run.declined() != "" {
			t.Fatalf("note %s revision %d: %+v %v %v", id, revision, run.rep, run.err, run.stages)
		}
		return ms.Mallocs - before
	}
	const chain = 160
	for i := 1; i <= chain; i++ {
		write("long-"+strconv.Itoa(i), long.rep.Ticket, i)
	}
	longCost := write("long-measured", long.rep.Ticket, chain+1)
	freshCost := write("fresh-measured", fresh.rep.Ticket, 1)
	t.Logf("note allocations at one head: %d-event chain %d, first note %d", chain, longCost, freshCost)
	if longCost > freshCost+freshCost/4 {
		t.Fatalf("a note on a %d-event chain made %d allocations, a first note at the same head %d: the write grows with the chain", chain, longCost, freshCost)
	}
}
