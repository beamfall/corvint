package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sharedEntry returns the pool entry a --share-allocation claim names, or nil.
func (c leaseContext) sharedEntry() *snapshot.PoolEntry {
	for i := range c.st.pools.Entries {
		if string(c.st.pools.Entries[i].AllocationID) == c.l.ShareAllocation {
			en := c.st.pools.Entries[i]
			return &en
		}
	}
	return nil
}

// boundAttempts lists the entry's attempt then its shared attempts.
func boundAttempts(en *snapshot.PoolEntry) []snapshot.PoolShare {
	return append([]snapshot.PoolShare{{AttemptID: en.AttemptID, Generation: en.Generation}}, en.Shared...)
}

// shareRefusal decides a --share-allocation claim after the ticket's own
// claim checks (PSR-V0-016, PSR-V0-017). Expired bound leases are returned
// for reaping first, as for any claim (CAL-V0-011); the retry then sees the
// allocation handed over or quarantined.
func (c leaseContext) shareRefusal() *leaseOutcome {
	refuse := func(outcome, code, detail string) *leaseOutcome {
		out := c.refuse(outcome, code, detail)
		return &out
	}
	en := c.sharedEntry()
	if en == nil || en.State != "ALLOCATED" {
		return refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "shared allocation is not current")
	}
	if en.PoolID != c.l.Pool || en.Stage != c.l.Stage {
		out := c.fail(malformed("shared allocation belongs to another pool or stage"))
		return &out
	}
	if en.Holder != c.l.Holder {
		return refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "shared allocation has another holder")
	}
	reap := []ExpiredLease{}
	for _, x := range boundAttempts(en) {
		a := c.st.attempts[x.AttemptID]
		if a == nil || !a.Live() || a.Generation != x.Generation {
			return refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "shared allocation is not current")
		}
		if a.Supervision != nil || a.RuntimeID != snapshot.RuntimeExternalAgent {
			out := c.fail(wire.Errorf(wire.CodeUnsupported, "pool", "a supervised allocation cannot be shared"))
			return &out
		}
		if expired(a, c.in.RecordedAt) {
			reap = append(reap, ExpiredLease{AttemptID: x.AttemptID, Generation: x.Generation})
		}
	}
	if len(reap) != 0 {
		out := refuse(mutation.OutcomeBlocked, wire.CodeAttemptLive, "expired leases block this claim until reaped")
		out.result.Expired = reap
		return out
	}
	if 1+len(en.Shared) >= snapshot.MaxSharedAttempts {
		return refuse(mutation.OutcomeCapacityExhausted, wire.CodeLimitExceeded, "shared allocation already binds "+string(wire.CountOf(snapshot.MaxSharedAttempts))+" attempts")
	}
	return nil
}

// sharesAllocation reports an attempt admitted onto a shared allocation, or
// one whose allocation currently binds further attempts.
func (c leaseContext) sharesAllocation(a *snapshot.Attempt) bool {
	if a.SharedAllocation != nil {
		return true
	}
	if a.PoolAllocation == nil {
		return false
	}
	for _, en := range c.st.pools.Entries {
		if en.AllocationID == a.PoolAllocation.AllocationID && len(en.Shared) != 0 {
			return true
		}
	}
	return false
}

// sharedPost applies one attempt write to an ALLOCATED entry that binds or
// will bind shared attempts (PSR-V0-018). A live attempt not yet bound joins
// the list; an ending shared attempt leaves it; an ending entry attempt hands
// the allocation to the first shared attempt. Only the last bound attempt
// to end reaches the ordinary quarantine, so it happens once.
func (c leaseContext) sharedPost(en *snapshot.PoolEntry, a *snapshot.Attempt) {
	primary := en.AttemptID == a.AttemptID && en.Generation == a.Generation
	at := -1
	for i, x := range en.Shared {
		if x.AttemptID == a.AttemptID && x.Generation == a.Generation {
			at = i
		}
	}
	rest := func(skip int) []snapshot.PoolShare {
		out := []snapshot.PoolShare{}
		for i, x := range en.Shared {
			if i != skip {
				out = append(out, x)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	switch {
	case a.Live() && !primary && at < 0:
		en.Shared = append(append([]snapshot.PoolShare{}, en.Shared...), snapshot.PoolShare{AttemptID: a.AttemptID, Generation: a.Generation})
	case a.Live():
		return
	case at >= 0:
		en.Shared = rest(at)
	case primary:
		en.AttemptID, en.Generation = en.Shared[0].AttemptID, en.Shared[0].Generation
		en.Shared = rest(0)
	default:
		return
	}
	en.ChangedSeq = c.seq
}

// checkBound validates one attempt bound to an ALLOCATED entry.
func checkBound(st inputState, en *snapshot.PoolEntry, x snapshot.PoolShare, shared bool) bool {
	a := st.attempts[x.AttemptID]
	return a != nil && a.Live() && a.Generation == x.Generation && a.PoolAllocation != nil && sameAllocation(a.PoolAllocation, &en.PoolAllocation) && a.Lease != nil && a.Lease.Holder == en.Holder && a.Stage == en.Stage && (!shared || a.SharedAllocation != nil)
}
