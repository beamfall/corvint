package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func poolOccupancy(rc *readCtx) (wire.Value, error) {
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
			}
			members = append(members, wire.ObjectValue(o))
		}
		pools = append(pools, wire.ObjectValue(wire.NewObject().Set("poolId", wire.String(p.ID)).Set("members", wire.Array(members...))))
	}
	return wire.Array(pools...), nil
}
