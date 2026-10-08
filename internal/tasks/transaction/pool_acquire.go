package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// planPoolAcquire allocates one pool member to a live external-agent attempt
// that holds none (CAL-V0-198..200). It is fenced like every attempt verb,
// takes the holder and stage from the attempt, and then admits the member by
// the pooled-claim steps: priority yield (CAL-V0-101), author exclusion
// (CAL-V0-098), health preparation and the free-member scan (CAL-V0-028).
func planPoolAcquire(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if why := c.fenced(a); why != "" {
		return c.recordFenced(a, why)
	}
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "an admission barrier is present")
	}
	if a.Lease == nil || a.RuntimeID != snapshot.RuntimeExternalAgent {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeTicketState, "pool acquire requires an external-agent work lease")
	}
	if a.PoolAllocation != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "attempt already holds allocation "+string(a.PoolAllocation.AllocationID)+" of pool "+a.PoolAllocation.PoolID)
	}
	if a.ReleasedPoolAllocation != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "attempt generation "+string(a.Generation)+" already returned allocation "+string(a.ReleasedPoolAllocation.Allocation.AllocationID)+"; one allocation per generation")
	}
	rec, _ := c.st.tickets.Get(a.TicketID.Raw)
	if rec == nil {
		return c.fail(malformed("unknown ticket " + a.TicketID.Raw))
	}
	if rec.RequiresPool != "" && rec.RequiresPool != c.l.Pool {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "ticket requires pool "+rec.RequiresPool)
	}
	if c.st.policy.Pool(c.l.Pool) == nil {
		return c.fail(malformed("unknown pool"))
	}
	if e := CheckExcludeAuthors(c.l.ExcludeAuthors, c.l.Pool, a.Stage); e != nil {
		return c.fail(e)
	}
	l := *c.l
	l.Holder, l.Stage = a.Lease.Holder, a.Stage
	c.l = &l
	if refusal := c.yieldRefusal(rec); refusal != nil {
		return *refusal
	}
	var refusal *leaseOutcome
	if c.authors, refusal = c.authorExclusion(a.TicketID.Raw); refusal != nil {
		return *refusal
	}
	alloc, e := c.allocate(a)
	if e != nil {
		out := c.fail(e)
		// The store's health preparation probes only members this derivation leaves eligible.
		out.result.AuthorExclusion = c.authors
		return out
	}
	next := *a
	next.PoolAllocation = alloc
	out := c.write(&next, nil, "TRANSITION", false)
	out.authors = c.authors
	return out
}

// planPoolRelease returns the attempt's exact current allocation early and
// quarantines it as an attempt release does; the attempt stays live
// (CAL-V0-201, CAL-V0-202).
func planPoolRelease(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if why := c.fenced(a); why != "" {
		return c.recordFenced(a, why)
	}
	if a.PoolAllocation == nil || string(a.PoolAllocation.AllocationID) != c.l.Allocation {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation is not the attempt's current allocation")
	}
	if c.sharesAllocation(a) {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "a shared allocation is returned only by ending its attempts")
	}
	state := *c.st.pools
	state.Entries = append([]snapshot.PoolEntry{}, state.Entries...)
	found := false
	for i := range state.Entries {
		if sameAllocation(&state.Entries[i].PoolAllocation, a.PoolAllocation) && state.Entries[i].State == "ALLOCATED" {
			state.Entries[i].State = "QUARANTINED"
			state.Entries[i].ChangedSeq = c.seq
			state.Entries[i].Reason = "attempt returned allocation early; physical safe reuse unproved"
			found = true
		}
	}
	if !found {
		return c.fail(malformed("live attempt missing pool occupancy"))
	}
	next := *a
	next.PoolAllocation = nil
	next.ReleasedPoolAllocation = &snapshot.ReleasedPoolAllocation{Allocation: *a.PoolAllocation, ReleasedSeq: c.seq}
	out := c.write(&next, nil, "TRANSITION", false)
	if out.result != nil {
		return out
	}
	if out.posts["pools.json"], e = state.Encode(); e != nil {
		return c.fail(e)
	}
	return out
}
