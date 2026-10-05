package transaction

import (
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// resourcePlanInput builds the issue 584 shape: capacity 16, `live` running
// reservations, `poolTickets` P0 tickets requiring pool "lanes" (members
// m0..m{members-1}, `busy` of them allocated) and `free` P2 lane-free tickets.
// Every ticket touches its own path, so only the window and the pool limit it.
func resourcePlanInput(t *testing.T, live, poolTickets, free, members, busy int) PlanInput {
	t.Helper()
	q, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := intent.DecodePolicy(fixture.PolicyBytes())
	if err != nil {
		t.Fatal(err)
	}
	policy.RequireEnforcedFields = nil
	policy.MaxActiveAttempts = "16"
	names := make([]string, members)
	for i := range names {
		names[i] = fmt.Sprintf("m%d", i)
	}
	policy.Pools = []intent.Pool{{ID: "lanes", Members: names}}
	recs := []*ticket.Record{}
	add := func(id, priority, pool string) {
		rec := fixture.Ticket(id)
		rec.Priority, rec.RequiresPool = priority, pool
		rec.Effects.TouchPaths = []string{id + "/"}
		recs = append(recs, rec)
	}
	for i := 0; i < poolTickets; i++ {
		add(fmt.Sprintf("PL-%02d", i), "P0", "lanes")
	}
	for i := 0; i < free; i++ {
		add(fmt.Sprintf("MX-%02d", i), "P2", "")
	}
	inv, err := ticket.NewInventory(q.QueueID, recs)
	if err != nil {
		t.Fatal(err)
	}
	in := PlanInput{Queue: q, Policy: policy, Tickets: inv, Attempts: map[string]*snapshot.Attempt{}, Reservations: &snapshot.ReservationSet{QueueID: q.QueueID}, Pools: &snapshot.PoolState{QueueID: q.QueueID, Entries: []snapshot.PoolEntry{}}}
	for i := 0; i < live; i++ {
		id, err := wire.ParseTicketID("", fixture.TicketID(fmt.Sprintf("LV-%02d", i)))
		if err != nil {
			t.Fatal(err)
		}
		in.Reservations.Entries = append(in.Reservations.Entries, snapshot.ReservationEntry{TicketID: id, Resources: []ticket.Resource{{Class: "PATH", Key: fmt.Sprintf("live-%02d/", i)}}})
	}
	for i := 0; i < busy; i++ {
		in.Pools.Entries = append(in.Pools.Entries, snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: "lanes", MemberID: names[i], AllocationID: wire.Sum([]byte(names[i]))}, State: "ALLOCATED"})
	}
	return in
}

type planCounts struct{ poolSelected, poolDeferred, freeSelected, limit, other int }

func countPlan(t *testing.T, plan TicketPlan) planCounts {
	t.Helper()
	var c planCounts
	for _, e := range plan.Entries {
		switch {
		case e.Ticket.RequiresPool != "" && e.State == PlanSelected:
			c.poolSelected++
		case e.Ticket.RequiresPool != "" && e.State == PlanDeferred && e.Reason == wire.CodeResourceCollision && len(e.Blockers) == 1 && e.Blockers[0] == e.Ticket.RequiresPool:
			c.poolDeferred++
		case e.Ticket.RequiresPool == "" && e.State == PlanSelected:
			c.freeSelected++
		case e.State == PlanDeferred && e.Reason == wire.CodeLimitExceeded:
			c.limit++
		default:
			c.other++
			t.Errorf("unexpected entry %s %s %s %v", e.Ticket.TicketID.Raw, e.State, e.Reason, e.Blockers)
		}
	}
	return c
}

// CAL-V0-078: the issue 584 fixture. Six higher-priority tickets wait for a
// fully occupied pool; they defer without consuming the window, so the eight
// free slots go to lane-free work.
func TestCALV0078_PoolWaitingTicketsDoNotConsumeWindow(t *testing.T) {
	in := resourcePlanInput(t, 8, 6, 28, 2, 2)
	plan := PriorityFirst(in)
	c := countPlan(t, plan)
	if c != (planCounts{poolDeferred: 6, freeSelected: 8, limit: 20}) {
		t.Fatalf("counts %+v", c)
	}
	if len(plan.Pools) != 1 || plan.Pools[0] != (PoolSelection{PoolID: "lanes", Observed: true, Free: 0, Selected: 0, Deferred: 6}) {
		t.Fatalf("pool summary %+v", plan.Pools)
	}
	if e := plan.ClaimNext(""); e == nil || e.Ticket.RequiresPool != "" {
		t.Fatalf("claim-next %+v", e)
	}
}

// CAL-V0-078: pool-needing selections are capped at the free eligible members;
// the excess defers with the pool as blocker and the window stays open.
func TestCALV0078_CapAtFreeEligibleMembers(t *testing.T) {
	in := resourcePlanInput(t, 8, 6, 28, 3, 1)
	plan := PriorityFirst(in)
	c := countPlan(t, plan)
	if c != (planCounts{poolSelected: 2, poolDeferred: 4, freeSelected: 6, limit: 22}) {
		t.Fatalf("counts %+v", c)
	}
	if plan.Pools[0] != (PoolSelection{PoolID: "lanes", Observed: true, Free: 2, Selected: 2, Deferred: 4}) {
		t.Fatalf("pool summary %+v", plan.Pools)
	}
	// CAL-V0-008/029: claim-next without --pool skips the pool selections.
	if e := plan.ClaimNext(""); e == nil || e.Ticket.RequiresPool != "" || e.Ticket.TicketID.Raw != fixture.TicketID("MX-00") {
		t.Fatalf("claim-next without pool %+v", e)
	}
	if e := plan.ClaimNext("lanes"); e == nil || e != plan.Selected() {
		t.Fatalf("claim-next with pool %+v", e)
	}
	rec := mustGet(t, in, "PL-00")
	if got, reason := RecordedClaimability(in, rec); got.Kind != wire.KindBool || !got.Bool || reason != PlanSelected {
		t.Fatalf("pool ticket claimability %s %s", wire.Encode(got), reason)
	}
}

// CAL-V0-078: unobservable member state defers and is never a selection.
func TestCALV0078_UnobservedPoolStateDefers(t *testing.T) {
	in := resourcePlanInput(t, 0, 2, 1, 2, 0)
	in.Pools = nil
	plan := PriorityFirst(in)
	if c := countPlan(t, plan); c != (planCounts{poolDeferred: 2, freeSelected: 1}) {
		t.Fatalf("counts %+v", c)
	}
	if plan.Pools[0] != (PoolSelection{PoolID: "lanes", Observed: false, Deferred: 2}) {
		t.Fatalf("pool summary %+v", plan.Pools)
	}
	got, reason := RecordedClaimability(in, mustGet(t, in, "PL-00"))
	if got.Kind != wire.KindNull || reason != string(ticket.NotObserved) {
		t.Fatalf("claimability %s %s", wire.Encode(got), reason)
	}
}

// CAL-V0-078: an undeclared pool still blocks; every refusal stays definite.
func TestCALV0078_UndeclaredPoolBlocks(t *testing.T) {
	in := resourcePlanInput(t, 0, 1, 1, 1, 0)
	in.Policy.Pools = nil
	plan := PriorityFirst(in)
	if plan.Pools != nil {
		t.Fatalf("summary without pools %+v", plan.Pools)
	}
	var e *PlanEntry
	for i := range plan.Entries {
		if plan.Entries[i].Ticket.RequiresPool != "" {
			e = &plan.Entries[i]
		}
	}
	if e == nil || e.State != PlanBlocked || e.Reason != wire.CodeResourceCollision {
		t.Fatalf("undeclared pool %+v", e)
	}
}

// CAL-V0-078 and CAL-V0-034: the --pool plan is unchanged. It caps all
// selections at the requested pool's free members, blocks tickets requiring
// another pool, and reports no default-plan summary.
func TestCALV0078_PoolPlanUnchanged(t *testing.T) {
	in := resourcePlanInput(t, 0, 2, 3, 3, 1)
	in.Policy.Pools = append(in.Policy.Pools, intent.Pool{ID: "other", Members: []string{"o"}})
	in.Pool = "lanes"
	plan := PriorityFirst(in)
	selected := 0
	for _, e := range plan.Entries {
		if e.State == PlanSelected {
			selected++
		}
	}
	if selected != 2 || plan.Pools != nil {
		t.Fatalf("--pool plan selected %d summary %+v", selected, plan.Pools)
	}
	in.Pool = "other"
	for _, e := range PriorityFirst(in).Entries {
		if e.Ticket.RequiresPool == "lanes" && (e.State != PlanBlocked || e.Reason != wire.CodeResourceCollision) {
			t.Fatalf("foreign pool ticket %+v", e)
		}
	}
}

func mustGet(t *testing.T, in PlanInput, local string) *ticket.Record {
	t.Helper()
	rec, ok := in.Tickets.Get(fixture.TicketID(local))
	if !ok {
		t.Fatal(local)
	}
	return rec
}
