package transaction

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// addSeconds is addMinutes at second precision.
func addSeconds(t wire.Timestamp, n int64) wire.Timestamp {
	at, _ := time.Parse(timestampLayout, string(t))
	return wire.Timestamp(at.Add(time.Duration(n) * time.Second).UTC().Format(timestampLayout))
}

// waiterLive is the PSR-V0-133 liveness of one waitlist entry at now: its
// pool is declared, it has not expired, its ticket is OPEN and holds no
// member of the pool, and an attempt-bound entry's attempt is still live at
// the registered generation, unexpired and without an allocation. A dead
// entry never blocks; the next write of pools.json drops it.
func waiterLive(w snapshot.PoolWaiter, policy *intent.Policy, tickets *ticket.Inventory, attempt func(string) *snapshot.Attempt, served map[string]bool, now wire.Timestamp) bool {
	if policy.Pool(w.PoolID) == nil || w.ExpiresAt <= now || served[w.PoolID+"\x00"+w.TicketID] {
		return false
	}
	if rec, _ := tickets.Get(w.TicketID); rec == nil || rec.Status != ticket.StatusOpen {
		return false
	}
	if w.AttemptID == "" {
		return true
	}
	a := attempt(w.AttemptID)
	return a != nil && a.Live() && a.Generation == w.Generation && a.PoolAllocation == nil && !expired(a, now)
}

// servedTickets keys the (pool, ticket) pairs a live attempt already holds a
// member for: such a ticket's entry is consumed (PSR-V0-134).
func servedTickets(state *snapshot.PoolState, attempt func(string) *snapshot.Attempt) map[string]bool {
	out := map[string]bool{}
	for i := range state.Entries {
		en := &state.Entries[i]
		if en.State != "ALLOCATED" {
			continue
		}
		for _, x := range boundAttempts(en) {
			if a := attempt(x.AttemptID); a != nil && a.Live() {
				out[en.PoolID+"\x00"+a.TicketID.Raw] = true
			}
		}
	}
	return out
}

// liveWaiters filters entries to the live ones, in pool then WaiterBefore
// order.
func liveWaiters(entries []snapshot.PoolWaiter, state *snapshot.PoolState, policy *intent.Policy, tickets *ticket.Inventory, attempt func(string) *snapshot.Attempt, now wire.Timestamp) []snapshot.PoolWaiter {
	if len(entries) == 0 {
		return nil
	}
	served := servedTickets(state, attempt)
	out := []snapshot.PoolWaiter{}
	for _, w := range entries {
		if waiterLive(w, policy, tickets, attempt, served, now) {
			out = append(out, w)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PoolID != out[j].PoolID {
			return out[i].PoolID < out[j].PoolID
		}
		return snapshot.WaiterBefore(out[i], out[j])
	})
	return out
}

// PoolWaitlist is the live waitlist of every pool at now, in admission
// order per pool, as plan preview and queue status show it (PSR-V0-135).
func PoolWaitlist(in PlanInput, now wire.Timestamp) []snapshot.PoolWaiter {
	if in.Pools == nil || in.Policy == nil || in.Tickets == nil {
		return nil
	}
	return liveWaiters(in.Pools.Waiters, in.Pools, in.Policy, in.Tickets, func(id string) *snapshot.Attempt { return in.Attempts[id] }, now)
}

// PoolWaiterValue renders one entry with its age at now (PSR-V0-135).
func PoolWaiterValue(w snapshot.PoolWaiter, now wire.Timestamp) wire.Value {
	since, _ := time.Parse(timestampLayout, string(w.Since))
	at, _ := time.Parse(timestampLayout, string(now))
	age := int64(at.Sub(since) / time.Second)
	if age < 0 {
		age = 0
	}
	o := wire.NewObject().Set("poolId", wire.String(w.PoolID)).Set("ticketId", wire.String(w.TicketID)).Set("kind", wire.String(w.Kind)).Set("priority", wire.String(w.Priority)).Set("holder", wire.String(w.Holder)).Set("seq", wire.String(string(w.Seq))).Set("since", wire.String(string(w.Since))).Set("ageSeconds", wire.String(string(wire.CountOf(age)))).Set("expiresAt", wire.String(string(w.ExpiresAt)))
	if w.AttemptID != "" {
		o.Set("attemptId", wire.String(w.AttemptID)).Set("generation", wire.String(string(w.Generation)))
	}
	return wire.ObjectValue(o)
}

func (c leaseContext) attemptLookup(override *snapshot.Attempt) func(string) *snapshot.Attempt {
	return func(id string) *snapshot.Attempt {
		if override != nil && override.AttemptID == id {
			return override
		}
		return c.st.attempts[id]
	}
}

// planPoolWait records or refreshes one ticket's place on a pool's waitlist
// (PSR-V0-130, PSR-V0-131). It never allocates and never refuses for lack of
// a free member; admission honours the entry (PSR-V0-132).
func planPoolWait(c leaseContext) leaseOutcome {
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "an admission barrier is present")
	}
	pool := c.st.policy.Pool(c.l.Pool)
	if pool == nil {
		return c.fail(malformed("unknown pool"))
	}
	if pool.MaxWaitSeconds == 0 {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, "pool "+pool.ID+" declares no maxWaitSeconds, so it keeps no waitlist")
	}
	if n := c.l.WaitSeconds.Int(); n > pool.MaxWaitSeconds {
		return c.fail(wire.Errorf(wire.CodeLimitExceeded, "waitSeconds", "wait of %d s exceeds pool %s maxWaitSeconds %d", n, pool.ID, pool.MaxWaitSeconds))
	}
	w := snapshot.PoolWaiter{PoolID: pool.ID, Kind: snapshot.WaiterWait, TicketID: c.l.TicketID, Holder: c.l.Holder}
	effectAttempt, effectGeneration := "", wire.Size("")
	if c.l.AttemptID != "" {
		a, e := c.named()
		if e != nil {
			return c.fail(e)
		}
		if why := c.fenced(a); why != "" {
			return c.recordFenced(a, why)
		}
		if a.Lease == nil {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeTicketState, "pool wait requires a work lease")
		}
		if a.PoolAllocation != nil {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "attempt already holds allocation "+string(a.PoolAllocation.AllocationID)+" of pool "+a.PoolAllocation.PoolID)
		}
		w.TicketID, w.AttemptID, w.Generation, w.Holder = a.TicketID.Raw, a.AttemptID, a.Generation, a.Lease.Holder
		effectAttempt, effectGeneration = a.AttemptID, a.Generation
	}
	rec, _ := c.st.tickets.Get(w.TicketID)
	if rec == nil {
		return c.fail(malformed("unknown ticket " + w.TicketID))
	}
	if rec.Status != ticket.StatusOpen {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeTicketState, "ticket "+w.TicketID+" is "+rec.Status+", not OPEN")
	}
	if rec.RequiresPool != "" && rec.RequiresPool != pool.ID {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "ticket requires pool "+rec.RequiresPool)
	}
	now := c.in.RecordedAt
	w.Priority, w.Seq, w.Since, w.ExpiresAt = rec.Priority, c.seq, now, addSeconds(now, c.l.WaitSeconds.Int())
	live := liveWaiters(c.st.pools.Waiters, c.st.pools, c.st.policy, c.st.tickets, c.attemptLookup(nil), now)
	kept := []snapshot.PoolWaiter{}
	for _, en := range live {
		if en.PoolID == w.PoolID && en.TicketID == w.TicketID {
			// A refresh keeps the place and the priority snapshot; a
			// successor waiting inside a GRACE window keeps the grace rank
			// and the later expiry.
			w.Seq, w.Since, w.Priority = en.Seq, en.Since, en.Priority
			if en.Kind == snapshot.WaiterGrace {
				w.Kind = snapshot.WaiterGrace
				if en.ExpiresAt > w.ExpiresAt {
					w.ExpiresAt = en.ExpiresAt
				}
			}
			continue
		}
		kept = append(kept, en)
	}
	if len(kept) >= snapshot.MaxPoolWaiters {
		return c.fail(limit("pool waiters"))
	}
	state := *c.st.pools
	state.Waiters = append(kept, w)
	raw, e := state.Encode()
	if e != nil {
		return c.fail(e)
	}
	eff := &leaseEffect{kind: "TRANSITION", attemptID: effectAttempt, generation: effectGeneration, outcome: mutation.OutcomeCompleted, codes: []string{}}
	if bytes.Equal(raw, c.in.Pools) {
		return leaseOutcome{posts: map[string][]byte{}, effect: eff}
	}
	return leaseOutcome{posts: map[string][]byte{"pools.json": raw}, effect: eff}
}

// waitlistRefusal refuses a member-taking admission that would take capacity
// the pool's live waitlist holds for waiters ahead of it (PSR-V0-132).
func (c leaseContext) waitlistRefusal(rec *ticket.Record) *leaseOutcome {
	if c.l.Pool == "" || len(c.st.pools.Waiters) == 0 {
		return nil
	}
	live := liveWaiters(c.st.pools.Waiters, c.st.pools, c.st.policy, c.st.tickets, c.attemptLookup(nil), c.in.RecordedAt)
	in := PlanInput{Pool: c.l.Pool, Stage: c.l.Stage, ExcludeMembers: c.l.ExcludeMembers, Pools: c.st.pools, Prepared: c.in.LeaseFacts.Pool.AllocationID, Queue: c.st.queue, Policy: c.st.policy, Tickets: c.st.tickets, Reservations: c.st.reservations, Attempts: c.st.attempts}
	detail := waitlistBlock(live, c.l.Pool, rec, poolSlots(in))
	if detail == "" {
		return nil
	}
	out := c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, detail)
	return &out
}

// waitlistAhead is the live entries of pool that a claim of rec yields to,
// in admission order (PSR-V0-132). A claimant with its own entry yields to
// the entries ordered before it; one without yields to every GRACE entry
// and to WAIT entries of the same or a higher priority.
func waitlistAhead(live []snapshot.PoolWaiter, pool string, rec *ticket.Record) []snapshot.PoolWaiter {
	own := waiterOf(live, pool, rec.TicketID.Raw)
	ahead := []snapshot.PoolWaiter{}
	for _, w := range live {
		if w.PoolID != pool || w.TicketID == rec.TicketID.Raw {
			continue
		}
		if own != nil && snapshot.WaiterBefore(w, *own) || own == nil && (w.Kind == snapshot.WaiterGrace || w.Priority <= rec.Priority) {
			ahead = append(ahead, w)
		}
	}
	return ahead
}

func waiterOf(live []snapshot.PoolWaiter, pool, ticketID string) *snapshot.PoolWaiter {
	for i := range live {
		if live[i].PoolID == pool && live[i].TicketID == ticketID {
			return &live[i]
		}
	}
	return nil
}

// waitlistBlock is the refusal detail when the entries ahead of rec number
// at least the pool's free members, or empty. Only free members count, so a
// waiter never strands one.
func waitlistBlock(live []snapshot.PoolWaiter, pool string, rec *ticket.Record, free int) string {
	ahead := waitlistAhead(live, pool, rec)
	if len(ahead) == 0 || free == 0 || len(ahead) < free {
		return ""
	}
	return waitlistDetail(pool, free, ahead)
}

func waitlistDetail(pool string, free int, ahead []snapshot.PoolWaiter) string {
	head := ahead[0]
	return fmt.Sprintf("pool %s waitlist holds its %d free member(s) for %d waiter(s) ahead; head waiter %s (%s %s, seq %s, until %s)", pool, free, len(ahead), head.TicketID, head.Kind, head.Priority, head.Seq, head.ExpiresAt)
}

// waitlistYielders drops from a priority-admission waiting list every
// competitor that the waitlist makes yield to rec (PSR-V0-132): one whose
// own entry is ordered after rec's, or one without an entry when rec's
// entry is GRACE or of the same or a higher priority. Priority admission
// then never makes a claimant yield to a competitor that yields to it.
func waitlistYielders(live []snapshot.PoolWaiter, pool string, rec *ticket.Record, tickets *ticket.Inventory, waiting []string) []string {
	own := waiterOf(live, pool, rec.TicketID.Raw)
	if own == nil {
		return waiting
	}
	out := []string{}
	for _, id := range waiting {
		other, _ := tickets.Get(id)
		if other == nil {
			out = append(out, id)
			continue
		}
		if !slices.ContainsFunc(waitlistAhead(live, pool, other), func(w snapshot.PoolWaiter) bool { return w.TicketID == own.TicketID }) {
			out = append(out, id)
		}
	}
	return out
}

// planWaitlist is the live waitlist a --pool plan honours at in.Now, or nil.
func planWaitlist(in PlanInput) []snapshot.PoolWaiter {
	if in.Now == "" || in.Pool == "" || in.Pools == nil || len(in.Pools.Waiters) == 0 {
		return nil
	}
	return liveWaiters(in.Pools.Waiters, in.Pools, in.Policy, in.Tickets, func(id string) *snapshot.Attempt { return in.Attempts[id] }, in.Now)
}

// graceWaiter is the PSR-V0-136 GRACE entry a clean HANDOFF release of an attempt
// writes, or nil: the attempt held a member of, or a live WAIT entry on, a
// pool that declares handoffGraceSeconds.
func (c leaseContext) graceWaiter(a *snapshot.Attempt) *snapshot.PoolWaiter {
	if c.l.Verb != LeaseRelease || c.l.Reason != wire.CodeHandoff || a.Live() || a.Lease == nil {
		return nil
	}
	poolID := ""
	if a.PoolAllocation != nil {
		poolID = a.PoolAllocation.PoolID
	} else {
		for _, w := range liveWaiters(c.st.pools.Waiters, c.st.pools, c.st.policy, c.st.tickets, c.attemptLookup(nil), c.in.RecordedAt) {
			if w.AttemptID == a.AttemptID {
				poolID = w.PoolID
			}
		}
	}
	pool := c.st.policy.Pool(poolID)
	if pool == nil || pool.HandoffGraceSeconds == 0 {
		return nil
	}
	rec, _ := c.st.tickets.Get(a.TicketID.Raw)
	if rec == nil {
		return nil
	}
	now := c.in.RecordedAt
	return &snapshot.PoolWaiter{PoolID: pool.ID, TicketID: a.TicketID.Raw, Kind: snapshot.WaiterGrace, Holder: a.Lease.Holder, Priority: rec.Priority, Seq: c.seq, Since: now, ExpiresAt: addSeconds(now, pool.HandoffGraceSeconds)}
}

// waitlistPosts settles the waitlist after an attempt write: dead and
// consumed entries are dropped and a clean HANDOFF's GRACE entry is added
// (PSR-V0-134, PSR-V0-136); a full waitlist refuses that HANDOFF release
// with LIMIT_EXCEEDED rather than drop its GRACE entry. pools.json is posted only when the waitlist
// changes, so a queue without waiters keeps its receipts byte-identical.
func (c leaseContext) waitlistPosts(a *snapshot.Attempt, posts map[string][]byte) error {
	grace := c.graceWaiter(a)
	if len(c.st.pools.Waiters) == 0 && grace == nil {
		return nil
	}
	state := *c.st.pools
	if raw, ok := posts["pools.json"]; ok {
		decoded, e := snapshot.DecodePools(raw)
		if e != nil {
			return e
		}
		state = *decoded
	}
	next := liveWaiters(state.Waiters, &state, c.st.policy, c.st.tickets, c.attemptLookup(a), c.in.RecordedAt)
	if grace != nil {
		kept := []snapshot.PoolWaiter{}
		for _, w := range next {
			if w.PoolID != grace.PoolID || w.TicketID != grace.TicketID {
				kept = append(kept, w)
			}
		}
		if len(kept) >= snapshot.MaxPoolWaiters {
			return limit(fmt.Sprintf("pool %s waitlist holds %d entries; no room for the HANDOFF GRACE entry of %s", grace.PoolID, snapshot.MaxPoolWaiters, grace.TicketID))
		}
		next = append(kept, *grace)
	}
	if len(next) == 0 && len(state.Waiters) == 0 || reflect.DeepEqual(sortedWaiters(next), sortedWaiters(state.Waiters)) {
		return nil
	}
	state.Waiters = next
	raw, e := state.Encode()
	if e == nil {
		posts["pools.json"] = raw
	}
	return e
}

func sortedWaiters(in []snapshot.PoolWaiter) []snapshot.PoolWaiter {
	out := append([]snapshot.PoolWaiter{}, in...)
	snapshot.SortPoolWaiters(out)
	return out
}

// pruneWaiters drops the dead entries before a pool sweep posts pools.json
// (PSR-V0-134).
func (c leaseContext) pruneWaiters(state *snapshot.PoolState) {
	if len(state.Waiters) > 0 {
		state.Waiters = liveWaiters(state.Waiters, state, c.st.policy, c.st.tickets, c.attemptLookup(nil), c.in.RecordedAt)
	}
}
