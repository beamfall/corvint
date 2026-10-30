package snapshot

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MaxPoolWaiters bounds the waitlist entries pools.json records across every
// pool (PSR-V0-131).
const MaxPoolWaiters = 512

// Pool waitlist entry kinds (PSR-V0-130, PSR-V0-136).
const (
	WaiterWait  = "WAIT"
	WaiterGrace = "GRACE"
)

// PoolWaiter is one ticket's place on a pool's waitlist (PSR-V0-130..136).
// A WAIT entry is registered by `pool wait`, either for a live attempt at one
// generation (AttemptID and Generation set) or for the ticket alone; a GRACE
// entry is written by a clean HANDOFF release when the pool declares
// handoffGraceSeconds. Neither binds a member, so ending or expiring one
// never quarantines or strands anything.
type PoolWaiter struct {
	PoolID, TicketID, Kind string
	AttemptID              string
	Generation             wire.Size
	Holder, Priority       string
	Seq                    wire.Size
	Since, ExpiresAt       wire.Timestamp
}

// SortPoolWaiters orders entries by pool, then ticket: the canonical
// pools.json order. Admission order is WaiterBefore.
func SortPoolWaiters(entries []PoolWaiter) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.PoolID != b.PoolID {
			return a.PoolID < b.PoolID
		}
		return a.TicketID < b.TicketID
	})
}

// WaiterBefore is the PSR-V0-132 admission order within one pool: GRACE
// entries first, then ticket priority, then arrival sequence, then ticket.
func WaiterBefore(a, b PoolWaiter) bool {
	if (a.Kind == WaiterGrace) != (b.Kind == WaiterGrace) {
		return a.Kind == WaiterGrace
	}
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	if a.Seq.Uint64() != b.Seq.Uint64() {
		return a.Seq.Uint64() < b.Seq.Uint64()
	}
	return a.TicketID < b.TicketID
}

func readPoolWaiters(r *wire.Reader, q wire.QueueID) []PoolWaiter {
	out := []PoolWaiter{}
	seen := map[string]bool{}
	for _, x := range r.Array(MaxPoolWaiters, true) {
		x.Closed(wire.OptionalKeys(x.Value(), []string{"poolId", "ticketId", "kind", "holder", "priority", "seq", "since", "expiresAt"}, "attemptId", "generation")...)
		w := PoolWaiter{PoolID: x.Field("poolId").Label(), TicketID: x.Field("ticketId").TicketID().Raw, Kind: x.Field("kind").Enum(WaiterWait, WaiterGrace), Holder: x.Field("holder").Label(), Priority: x.Field("priority").Enum("P0", "P1", "P2", "P3"), Seq: x.Field("seq").Size(), Since: x.Field("since").Timestamp(), ExpiresAt: x.Field("expiresAt").Timestamp()}
		if wire.Has(x.Value(), "attemptId") != wire.Has(x.Value(), "generation") {
			x.Fail(wire.CodeMalformed, "a waiter names an attempt and its generation together or neither")
		}
		if wire.Has(x.Value(), "attemptId") {
			w.AttemptID, w.Generation = x.Field("attemptId").Identifier(), x.Field("generation").Size()
		}
		if x.Err() != nil {
			return out
		}
		if w.AttemptID != "" {
			if aq, e := AttemptQueue(w.AttemptID); e != nil || aq.Raw != q.Raw {
				x.Fail(wire.CodeMalformed, "invalid waiter attempt")
			}
		}
		if w.Seq.Uint64() == 0 || w.ExpiresAt <= w.Since {
			x.Fail(wire.CodeMalformed, "invalid waiter sequence or expiry")
		}
		key := w.PoolID + "\x00" + w.TicketID
		if seen[key] {
			x.Fail(wire.CodeDuplicateID, "one waiter per ticket and pool")
		}
		seen[key] = true
		out = append(out, w)
	}
	if r.Err() == nil && len(out) == 0 {
		r.Fail(wire.CodeMalformed, "waiters must be non-empty when present")
	}
	return out
}

func poolWaitersValue(entries []PoolWaiter) wire.Value {
	sorted := append([]PoolWaiter{}, entries...)
	SortPoolWaiters(sorted)
	values := []wire.Value{}
	for _, w := range sorted {
		o := wire.NewObject().Set("poolId", wire.String(w.PoolID)).Set("ticketId", wire.String(w.TicketID)).Set("kind", wire.String(w.Kind)).Set("holder", wire.String(w.Holder)).Set("priority", wire.String(w.Priority)).Set("seq", wire.String(string(w.Seq))).Set("since", wire.String(string(w.Since))).Set("expiresAt", wire.String(string(w.ExpiresAt)))
		if w.AttemptID != "" {
			o.Set("attemptId", wire.String(w.AttemptID)).Set("generation", wire.String(string(w.Generation)))
		}
		values = append(values, wire.ObjectValue(o))
	}
	return wire.Array(values...)
}
