package cli

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-080: a dependency cycle as large as the queue bound is walked in
// linear work and memory. Blockers, whose cycle detail names the whole
// component, are derived only for the bounded output, never for every
// closure node, so a 10,000-ticket ring stays far below the quadratic
// cost (several GB) of deriving every member's view.
func TestCALV0080_CriticalPathLargeCycleIsBounded(t *testing.T) {
	const n = wire.MaxTicketsPerQueue
	recs := make([]*ticket.Record, n)
	for i := range recs {
		recs[i] = fixture.Ticket(fmt.Sprintf("R%05d", i))
		recs[i].Dependencies = []ticket.Dependency{fixture.Dep(fmt.Sprintf("R%05d", (i+1)%n))}
	}
	queue, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := intent.DecodePolicy(fixture.PolicyBytes())
	if err != nil {
		t.Fatal(err)
	}
	inv, err := ticket.NewInventory(queue.QueueID, recs)
	if err != nil {
		t.Fatal(err)
	}
	in := transaction.PlanInput{Queue: queue, Policy: policy, Tickets: inv}
	derived := 0
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	cp := computeCriticalPath(inv, ticket.Context{}, recs[0].TicketID.Raw)
	cp.blockers = func(rec *ticket.Record) []transaction.ObservedBlocker {
		derived++
		return transaction.ClaimBlockerObservations(in, rec)
	}
	v := cp.value(queue.QueueID.Raw, nil)
	runtime.ReadMemStats(&after)
	if got := v.Obj; got == nil {
		t.Fatal("no item")
	}
	nodesTotal, _ := v.Obj.Get("nodesTotal")
	cycles, _ := v.Obj.Get("cycles")
	truncated, _ := v.Obj.Get("truncated")
	if nodesTotal.Str != fmt.Sprint(n) || len(cycles.Arr) != 1 || !truncated.Bool {
		t.Fatalf("ring: nodesTotal %s cycles %d truncated %v", nodesTotal.Str, len(cycles.Arr), truncated.Bool)
	}
	if derived > maxCriticalPathNodes {
		t.Fatalf("derived blockers for %d nodes, bound %d", derived, maxCriticalPathNodes)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 256<<20 {
		t.Fatalf("allocated %d bytes for a %d-ticket ring", alloc, n)
	}
}
