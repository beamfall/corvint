package cli

import (
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// poolWaitlistValue renders the live pool waitlist at observedAt in
// admission order per pool (PSR-V0-135); ok is false when it is empty.
func poolWaitlistValue(in transaction.PlanInput, observedAt time.Time) (wire.Value, bool) {
	now := wire.Timestamp(observedAt.UTC().Format("2006-01-02T15:04:05Z"))
	live := transaction.PoolWaitlist(in, now)
	if len(live) == 0 {
		return wire.Value{}, false
	}
	values := []wire.Value{}
	for _, w := range live {
		values = append(values, transaction.PoolWaiterValue(w, now))
	}
	return wire.Array(values...), true
}

// poolHoldStatus is a HELD entry's window and whether it has lapsed at
// the reader's clock (PSR-V0-272); nothing is written.
func poolHoldStatus(en snapshot.PoolEntry, now time.Time) wire.Value {
	h := snapshot.PoolHoldValue(en.Hold)
	h.Obj.Set("expired", wire.Bool(transaction.PoolHoldExpired(en, now)))
	h.Obj.Set("reason", wire.String(en.Reason))
	return h
}

func poolOccupancy(rc *readCtx, observedAt time.Time) (wire.Value, error) {
	proof, e := auditState(rc, "pools.json")
	if e != nil {
		return wire.Value{}, e
	}
	occupied := map[string]snapshot.PoolEntry{}
	if raw := proof.Records["pools.json"].Raw; len(raw) > 0 {
		state, e := snapshot.DecodePools(raw)
		if e != nil {
			return wire.Value{}, e
		}
		for _, en := range state.Entries {
			occupied[en.MemberID] = en
		}
	}
	pools := []wire.Value{}
	for _, p := range rc.store.Policy.Pools {
		members := []wire.Value{}
		for _, m := range p.Members {
			o := wire.NewObject().Set("memberId", wire.String(m)).Set("state", wire.String("FREE")).Set("reservedFor", stringOrNull(p.ReservedFor[m]))
			if en, ok := occupied[m]; ok {
				o.Set("reason", wire.String(en.Reason)).Set("commandKind", stringOrNull(en.CommandKind)).Set("observationSha256", wire.StringOrNull(func() *string {
					if en.ObservationSha256 == nil {
						return nil
					}
					s := string(*en.ObservationSha256)
					return &s
				}())).Set("state", wire.String(en.State)).Set("allocation", snapshot.PoolAllocationValue(&en.PoolAllocation)).Set("attemptId", stringOrNull(en.AttemptID)).Set("holder", wire.String(en.Holder)).Set("stage", stringOrNull(en.Stage))
				// PSR-V0-272: only a HELD member shows its hold window, so
				// every other member's status bytes are unchanged.
				if en.State == "HELD" {
					o.Set("hold", poolHoldStatus(en, observedAt))
				}
			}
			members = append(members, wire.ObjectValue(o))
		}
		pools = append(pools, wire.ObjectValue(wire.NewObject().Set("poolId", wire.String(p.ID)).Set("members", wire.Array(members...))))
	}
	return wire.Array(pools...), nil
}
