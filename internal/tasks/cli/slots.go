package cli

import (
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// slotObservationValue is the `slot` member of a slots acquire or release
// result (PSR-V0-038).
func slotObservationValue(s *store.SlotObservation) wire.Value {
	o := wire.NewObject().Set("slotsId", wire.String(s.SlotsID)).Set("state", wire.String(s.State))
	if s.State == "NONE" {
		o.Set("count", wire.Null()).Set("exclusive", wire.Bool(false)).Set("class", wire.Null()).Set("position", wire.Null())
		return wire.ObjectValue(o)
	}
	o.Set("count", wire.String(string(s.Count))).Set("exclusive", wire.Bool(s.Exclusive)).Set("class", stringOrNull(s.Class))
	if s.Position > 0 {
		o.Set("position", wire.String(string(wire.CountOf(int64(s.Position)))))
	} else {
		o.Set("position", wire.Null())
	}
	return wire.ObjectValue(o)
}

// slotOccupancy is the queue status `slots` member (PSR-V0-038): for each
// declared resource its capacity, reservations, held and free counts, the
// live holders, and the live waiters in wait order. Entries of ended or
// reaped attempts are not shown; an expired waiter or holder is flagged
// until a slot write or claim reaps it. Pure read.
func slotOccupancy(rc *readCtx, observedAt time.Time) (wire.Value, error) {
	proof, e := auditState(rc, "pools.json")
	if e != nil {
		return wire.Value{}, e
	}
	entries := []snapshot.SlotEntry{}
	if raw := proof.Records["pools.json"].Raw; len(raw) > 0 {
		state, e := snapshot.DecodePools(raw)
		if e != nil {
			return wire.Value{}, e
		}
		entries = state.Slots
	}
	attempts, e := liveAttempts(rc)
	if e != nil {
		return wire.Value{}, e
	}
	byID := map[string]*snapshot.Attempt{}
	for _, a := range attempts {
		byID[a.AttemptID] = a
	}
	live := transaction.SlotLive(rc.store.Policy, entries, func(id string) *snapshot.Attempt { return byID[id] })
	now := wire.Timestamp(observedAt.UTC().Format("2006-01-02T15:04:05Z"))
	out := []wire.Value{}
	for i := range rc.store.Policy.Slots {
		res := &rc.store.Policy.Slots[i]
		u := transaction.SlotUsageOf(res.ID, live)
		reserved := wire.NewObject()
		for _, k := range res.Classes() {
			reserved.Set(k, wire.String(string(res.Reserved[k])))
		}
		holders, waiting := []wire.Value{}, []snapshot.SlotEntry{}
		for _, en := range live {
			if en.SlotsID != res.ID {
				continue
			}
			if en.State == snapshot.SlotWaiting {
				waiting = append(waiting, en)
				continue
			}
			holders = append(holders, wire.ObjectValue(slotEntryObject(en, byID, now)))
		}
		waiters := []wire.Value{}
		for n, en := range transaction.SlotWaitOrder(waiting) {
			o := slotEntryObject(en, byID, now).Set("priority", wire.String(en.Priority)).Set("position", wire.String(string(wire.CountOf(int64(n+1)))))
			waiters = append(waiters, wire.ObjectValue(o))
		}
		o := wire.NewObject().Set("slotsId", wire.String(res.ID)).Set("capacity", wire.String(string(res.Capacity))).Set("reserved", wire.ObjectValue(reserved))
		o.Set("held", wire.String(string(wire.CountOf(u.Held)))).Set("free", wire.String(string(wire.CountOf(res.Capacity.Int()-u.Held)))).Set("exclusiveHeld", wire.Bool(u.Exclusive))
		o.Set("holders", wire.Array(holders...)).Set("waiters", wire.Array(waiters...))
		out = append(out, wire.ObjectValue(o))
	}
	return wire.Array(out...), nil
}

func slotEntryObject(en snapshot.SlotEntry, byID map[string]*snapshot.Attempt, now wire.Timestamp) *wire.Object {
	a := byID[en.AttemptID]
	return wire.NewObject().Set("attemptId", wire.String(en.AttemptID)).Set("generation", wire.String(string(en.Generation))).Set("holder", wire.String(en.Holder)).Set("count", wire.String(string(en.Count))).Set("exclusive", wire.Bool(en.Exclusive)).Set("class", stringOrNull(en.Class)).Set("leaseExpired", wire.Bool(a.Lease.ExpiresAt <= now))
}
