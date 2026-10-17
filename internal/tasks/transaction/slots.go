package transaction

import (
	"bytes"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SlotLive returns the slot entries that still count (PSR-V0-032): the
// resource is declared and the entry's attempt is live at the entry's
// generation. Every other entry is dead; it holds and waits for nothing and
// the next slot write prunes it, so an ended or reaped attempt frees its
// slots without operator action.
func SlotLive(policy *intent.Policy, entries []snapshot.SlotEntry, live func(attemptID string) *snapshot.Attempt) []snapshot.SlotEntry {
	out := []snapshot.SlotEntry{}
	for _, en := range entries {
		a := live(en.AttemptID)
		if policy.SlotResource(en.SlotsID) != nil && a != nil && a.Live() && a.Generation == en.Generation {
			out = append(out, en)
		}
	}
	return out
}

// SlotWaitOrder orders waiters by ticket priority, then request sequence,
// then attempt id (PSR-V0-034).
func SlotWaitOrder(entries []snapshot.SlotEntry) []snapshot.SlotEntry {
	out := append([]snapshot.SlotEntry{}, entries...)
	sort.SliceStable(out, func(i, j int) bool { return slotBefore(out[i], out[j]) })
	return out
}

func slotBefore(a, b snapshot.SlotEntry) bool {
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	if a.Seq.Uint64() != b.Seq.Uint64() {
		return a.Seq.Uint64() < b.Seq.Uint64()
	}
	return a.AttemptID < b.AttemptID
}

// SlotUsage is one resource's current held count and per-class held counts.
type SlotUsage struct {
	Held      int64
	Exclusive bool
	ByClass   map[string]int64
}

// SlotUsageOf sums the HELD entries of one resource among live entries.
func SlotUsageOf(slotsID string, live []snapshot.SlotEntry) SlotUsage {
	u := SlotUsage{ByClass: map[string]int64{}}
	for _, en := range live {
		if en.SlotsID == slotsID && en.State == snapshot.SlotHeld {
			u.Held += en.Count.Int()
			u.ByClass[en.Class] += en.Count.Int()
			u.Exclusive = u.Exclusive || en.Exclusive
		}
	}
	return u
}

// unusedReserved is a class's reservation not yet taken by its own holders.
func (u SlotUsage) unusedReserved(res *intent.SlotResource, class string) int64 {
	if class == "" {
		return 0
	}
	return max(0, res.ReservedFor(class)-u.ByClass[class])
}

// SlotAvailable is how many slots a shared request of class may take now:
// capacity minus held minus every other class's unused reservation
// (PSR-V0-035).
func SlotAvailable(res *intent.SlotResource, u SlotUsage, class string) int64 {
	if u.Exclusive {
		return 0
	}
	n := res.Capacity.Int() - u.Held
	for _, k := range res.Classes() {
		if k != class {
			n -= u.unusedReserved(res, k)
		}
	}
	return n
}

// settleSlots grants waiters of one resource in wait order (PSR-V0-033..035,
// PSR-V0-040, PSR-V0-041) in one pass, updating entries through index, and
// reports whether any entry changed. Grants are checked against the current
// policy: a waiter its own acquire would now refuse is neither granted nor
// blocks a later one, and an exclusive grant takes the current capacity. An
// exclusive waiter is eligible only when no reserved-class waiter was
// enqueued before it, and the first eligible one in wait order is granted
// only when nothing is held. A shared waiter is skipped while an exclusive
// waiter was enqueued before it or, when unreserved, is ahead of it in wait
// order; once a shared waiter does not fit, a later one is granted only
// within its own class's unused reservation.
func settleSlots(res *intent.SlotResource, live []snapshot.SlotEntry, index map[string]int, entries []snapshot.SlotEntry) bool {
	u := SlotUsageOf(res.ID, live)
	if u.Exclusive {
		return false
	}
	waiters := []snapshot.SlotEntry{}
	for _, en := range live {
		if en.SlotsID == res.ID && en.State == snapshot.SlotWaiting && slotFits(res, en) {
			waiters = append(waiters, en)
		}
	}
	waiters = SlotWaitOrder(waiters)
	exclusives, eligible := []snapshot.SlotEntry{}, []snapshot.SlotEntry{}
	for _, w := range waiters {
		if w.Exclusive {
			exclusives = append(exclusives, w)
			if !slotReservedBefore(w, waiters) {
				eligible = append(eligible, w)
			}
		}
	}
	changed, blocked := false, false
	grant := func(w snapshot.SlotEntry, n int64) {
		i := index[res.ID+"\x00"+w.AttemptID]
		entries[i].State, entries[i].Count = snapshot.SlotHeld, wire.CountOf(n)
		u.Held += n
		u.ByClass[w.Class] += n
		changed = true
	}
	for _, w := range waiters {
		if w.Exclusive {
			if u.Held == 0 && !blocked && len(eligible) > 0 && eligible[0].AttemptID == w.AttemptID {
				grant(w, res.Capacity.Int())
				return true
			}
			continue
		}
		if slotBehindExclusive(w, exclusives) {
			continue
		}
		n := w.Count.Int()
		if SlotAvailable(res, u, w.Class) >= n && (!blocked || n <= u.unusedReserved(res, w.Class)) {
			grant(w, n)
			continue
		}
		blocked = true
	}
	return changed
}

// slotCeiling is the most a shared request of class can ever hold: capacity
// minus every other class's reservation (PSR-V0-035).
func slotCeiling(res *intent.SlotResource, class string) int64 {
	n := res.Capacity.Int()
	for _, k := range res.Classes() {
		if k != class {
			n -= res.ReservedFor(k)
		}
	}
	return n
}

// slotFits reports whether the current policy could still grant a waiter
// (PSR-V0-040): an exclusive always (it takes the current capacity), a
// shared one while its class is still reserved and its count is within its
// ceiling.
func slotFits(res *intent.SlotResource, w snapshot.SlotEntry) bool {
	if w.Exclusive {
		return true
	}
	if w.Class != "" && res.ReservedFor(w.Class) == 0 {
		return false
	}
	return w.Count.Int() <= slotCeiling(res, w.Class)
}

// slotBehindExclusive reports an exclusive waiter enqueued before w, or, for
// an unreserved w, ahead of it in wait order: new shared grants stop behind
// an exclusive request so it is eventually granted (PSR-V0-033), while a
// reserved-class waiter is not passed by a later exclusive (PSR-V0-041).
func slotBehindExclusive(w snapshot.SlotEntry, exclusives []snapshot.SlotEntry) bool {
	for _, x := range exclusives {
		if x.Seq.Uint64() < w.Seq.Uint64() || w.Class == "" && slotBefore(x, w) {
			return true
		}
	}
	return false
}

// slotReservedBefore reports a reserved-class waiter enqueued before the
// exclusive waiter x, which goes first whatever the priorities, so
// continuous exclusive demand cannot starve a reserved class (PSR-V0-041).
func slotReservedBefore(x snapshot.SlotEntry, waiters []snapshot.SlotEntry) bool {
	for _, w := range waiters {
		if !w.Exclusive && w.Class != "" && w.Seq.Uint64() < x.Seq.Uint64() {
			return true
		}
	}
	return false
}

// slotResource checks the request against the declared resource and returns
// the resource, the entry count and a refusal (PSR-V0-030, PSR-V0-036).
func (c leaseContext) slotResource() (*intent.SlotResource, int64, *leaseOutcome) {
	res := c.st.policy.SlotResource(c.l.Slots)
	if res == nil {
		out := c.fail(malformed("unknown slots resource " + c.l.Slots))
		return nil, 0, &out
	}
	if c.l.SlotClass != "" && res.ReservedFor(c.l.SlotClass) == 0 {
		out := c.fail(malformed("class " + c.l.SlotClass + " is not reserved on slots " + res.ID))
		return nil, 0, &out
	}
	if c.l.SlotExclusive {
		return res, res.Capacity.Int(), nil
	}
	n := int64(1)
	if c.l.SlotCount != "" {
		n = c.l.SlotCount.Int()
	}
	ceiling := slotCeiling(res, c.l.SlotClass)
	if n > ceiling {
		out := c.fail(wire.Errorf(wire.CodeLimitExceeded, "slotCount", "%d slots can never be granted on slots %s; at most %d for this class", n, res.ID, ceiling))
		return nil, 0, &out
	}
	return res, n, nil
}

// slotAttempt fences the named attempt like every attempt verb.
func (c leaseContext) slotAttempt() (*snapshot.Attempt, *leaseOutcome) {
	a, e := c.named()
	if e != nil {
		out := c.fail(e)
		return nil, &out
	}
	if why := c.fenced(a); why != "" {
		out := c.recordFenced(a, why)
		return nil, &out
	}
	if a.Lease == nil {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeTicketState, "slots require a work lease")
		return nil, &out
	}
	return a, nil
}

func (c leaseContext) liveAttempt(id string) *snapshot.Attempt {
	return c.st.attempts[id]
}

// planSlotAcquire enqueues the attempt for slots of one resource and grants
// waiters (PSR-V0-032..037). A poll by an attempt that already has the same
// entry only settles; an expired holder or waiter of the resource is
// returned for reaping first, so a crashed holder frees its slots.
func planSlotAcquire(c leaseContext) leaseOutcome {
	a, refusal := c.slotAttempt()
	if refusal != nil {
		return *refusal
	}
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "an admission barrier is present")
	}
	res, n, refusal := c.slotResource()
	if refusal != nil {
		return *refusal
	}
	live := SlotLive(c.st.policy, c.st.pools.Slots, c.liveAttempt)
	reap := []ExpiredLease{}
	for _, en := range live {
		if en.SlotsID == res.ID && en.AttemptID != a.AttemptID && expired(c.st.attempts[en.AttemptID], c.in.RecordedAt) {
			reap = append(reap, ExpiredLease{AttemptID: en.AttemptID, Generation: en.Generation})
		}
	}
	if len(reap) > 0 {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeAttemptLive, "an expired slot holder must be reaped first")
		out.result.Expired = reap
		return out
	}
	want := snapshot.SlotEntry{SlotsID: res.ID, AttemptID: a.AttemptID, Generation: a.Generation, Holder: a.Lease.Holder, Count: wire.CountOf(n), Exclusive: c.l.SlotExclusive, Class: c.l.SlotClass, State: snapshot.SlotWaiting, Seq: c.seq}
	for _, en := range live {
		if en.SlotsID == res.ID && en.AttemptID == a.AttemptID {
			// An exclusive entry's count follows the capacity (PSR-V0-040).
			if en.Exclusive != want.Exclusive || en.Class != want.Class || !want.Exclusive && en.Count != want.Count {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "attempt already "+en.State+" "+string(en.Count)+" slots of "+res.ID+" with other parameters; release first")
			}
			return c.slotWrite(a, live)
		}
	}
	if len(live) >= snapshot.MaxSlotEntries {
		return c.fail(limit("slot entries"))
	}
	rec, _ := c.st.tickets.Get(a.TicketID.Raw)
	if rec == nil {
		return c.fail(malformed("unknown ticket " + a.TicketID.Raw))
	}
	want.Priority = rec.Priority
	return c.slotWrite(a, append(live, want))
}

// planSlotRelease drops the attempt's entry for one resource, held or
// waiting, and grants waiters; no member, cleanup or quarantine is involved
// (PSR-V0-036).
func planSlotRelease(c leaseContext) leaseOutcome {
	a, refusal := c.slotAttempt()
	if refusal != nil {
		return *refusal
	}
	live := SlotLive(c.st.policy, c.st.pools.Slots, c.liveAttempt)
	kept := []snapshot.SlotEntry{}
	found := false
	for _, en := range live {
		if en.SlotsID == c.l.Slots && en.AttemptID == a.AttemptID {
			found = true
			continue
		}
		kept = append(kept, en)
	}
	if !found {
		return c.slotNoop(a)
	}
	return c.slotWrite(a, kept)
}

// slotNoop records a slot command that changes nothing (PSR-V0-042): the
// receipt posts only the request entry, so a retry of its request id replays
// it instead of acting on a later state.
func (c leaseContext) slotNoop(a *snapshot.Attempt) leaseOutcome {
	return leaseOutcome{posts: map[string][]byte{}, effect: &leaseEffect{kind: "TRANSITION", attemptID: a.AttemptID, generation: a.Generation, outcome: mutation.OutcomeCompleted, codes: []string{}}}
}

// holdsSlots reports a live slot entry of the attempt, held or waiting.
func (c leaseContext) holdsSlots(a *snapshot.Attempt) bool {
	for _, en := range SlotLive(c.st.policy, c.st.pools.Slots, c.liveAttempt) {
		if en.AttemptID == a.AttemptID {
			return true
		}
	}
	return false
}

// slotWrite settles every declared resource over the live entries and posts
// pools.json when its bytes change; dead entries are pruned in the same
// write.
func (c leaseContext) slotWrite(a *snapshot.Attempt, live []snapshot.SlotEntry) leaseOutcome {
	entries := append([]snapshot.SlotEntry{}, live...)
	index := map[string]int{}
	for i, en := range entries {
		index[en.SlotsID+"\x00"+en.AttemptID] = i
	}
	for i := range c.st.policy.Slots {
		res := &c.st.policy.Slots[i]
		settleSlots(res, append([]snapshot.SlotEntry{}, entries...), index, entries)
	}
	state := *c.st.pools
	state.Slots = entries
	raw, e := state.Encode()
	if e != nil {
		return c.fail(e)
	}
	if bytes.Equal(raw, c.in.Pools) || len(c.in.Pools) == 0 && len(entries) == 0 && len(state.Entries) == 0 {
		return c.slotNoop(a)
	}
	return leaseOutcome{posts: map[string][]byte{"pools.json": raw}, effect: &leaseEffect{kind: "TRANSITION", attemptID: a.AttemptID, generation: a.Generation, outcome: mutation.OutcomeCompleted, codes: []string{}}}
}
