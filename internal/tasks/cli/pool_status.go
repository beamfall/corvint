package cli

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const poolNotObserved = "NOT_OBSERVED"

// poolStatus is the read-only per-member pool view (PSR-V0-013..015). It
// shares the TM-V0-008 snapshot read and the journal audit with `queue
// status`: no lock, no probe, no write. Facts the journal does not retain,
// such as wall-clock times, are NOT_OBSERVED rather than inferred.
func poolStatus(env Env, args []string) *wire.Result {
	cmd := []string{"pool", "status"}
	values := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		if (key != "--pool" && key != "--member") || i+1 >= len(args) || values[key] != "" || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
			return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "pool status takes at most one --pool ID and one --member ID"))
		}
		if _, err := wire.ParseLabel(key, args[i+1]); err != nil {
			return failure(cmd, nil, err)
		}
		values[key] = args[i+1]
	}
	poolFilter, memberFilter := values["--pool"], values["--member"]
	var item wire.Value
	rc, err := withStore(env, func(rc *readCtx) error {
		matched := false
		for _, p := range rc.store.Policy.Pools {
			matched = matched || p.ID == poolFilter
		}
		if poolFilter != "" && !matched {
			return wire.Errorf(wire.CodeMalformed, "--pool", "unknown pool %q", poolFilter)
		}
		occupied := map[string]snapshot.PoolEntry{}
		identities := map[string]snapshot.PoolIdentity{}
		operations := map[string]snapshot.PoolOperationReceipt{}
		if len(rc.store.Policy.Pools) > 0 {
			proof, e := auditState(rc, "pools.json")
			if e != nil {
				return e
			}
			// An audited absent pools.json is the empty state: every member FREE.
			if raw := proof.Records["pools.json"].Raw; len(raw) > 0 {
				state, e := snapshot.DecodePools(raw)
				if e != nil {
					return e
				}
				for _, en := range state.Entries {
					occupied[en.MemberID] = en
				}
				for _, x := range state.Operations {
					operations[x.PoolID+"\x00"+x.MemberID+"\x00"+x.Operation] = x
				}
				for _, id := range state.Identities {
					identities[id.PoolID+"\x00"+id.MemberID] = id
				}
			}
		}
		evidence := journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.IntentRoot()}
		pools, found := []wire.Value{}, false
		for _, p := range rc.store.Policy.Pools {
			if poolFilter != "" && p.ID != poolFilter {
				continue
			}
			members := []wire.Value{}
			for _, m := range p.Members {
				if memberFilter != "" && m != memberFilter {
					continue
				}
				found = true
				o := wire.NewObject().Set("memberId", wire.String(m)).Set("reservedFor", stringOrNull(p.ReservedFor[m]))
				en, ok := occupied[m]
				if ok {
					poolMemberOccupied(o, en, evidence)
				} else {
					poolMemberFree(o)
				}
				if p.DefaultSource != nil {
					poolMemberIdentity(o, en, ok, identities[p.ID+"\x00"+m])
				}
				// PSR-V0-162: only a member that declares operations shows
				// them, so every other member's status bytes are unchanged.
				if declared := p.MemberConfig[m].Operations; len(declared) > 0 {
					ops := wire.NewObject()
					for _, op := range intent.PoolOperations {
						if declared[op] == nil {
							continue
						}
						if x, ok := operations[p.ID+"\x00"+m+"\x00"+op]; ok {
							ops.Set(op, x.Value())
						} else {
							ops.Set(op, wire.String(poolNotObserved))
						}
					}
					o.Set("operations", wire.ObjectValue(ops))
				}
				members = append(members, wire.ObjectValue(o))
			}
			if memberFilter == "" || len(members) > 0 {
				po := wire.NewObject().Set("poolId", wire.String(p.ID))
				// PSR-V0-083: only a pool that declares sources shows them, so
				// every other pool's status bytes are unchanged (PSR-V0-086).
				if p.DefaultSource != nil {
					allowed := []wire.Value{}
					for _, x := range p.AllowedSources {
						allowed = append(allowed, snapshot.PoolSourceValue(&x))
					}
					po.Set("defaultSource", snapshot.PoolSourceValue(p.DefaultSource)).Set("allowedSources", wire.Array(allowed...))
				}
				pools = append(pools, wire.ObjectValue(po.Set("members", wire.Array(members...))))
			}
		}
		if memberFilter != "" && !found {
			return wire.Errorf(wire.CodeMalformed, "--member", "unknown pool member %q", memberFilter)
		}
		o := wire.NewObject()
		o.Set("profile", wire.String("taskman-pool-status/0"))
		o.Set("queueId", wire.String(rc.store.Queue.QueueID.Raw))
		o.Set("pool", stringOrNull(poolFilter))
		o.Set("member", stringOrNull(memberFilter))
		o.Set("pools", wire.Array(pools...))
		o.Set("mutationAuthority", wire.Bool(false))
		item = wire.ObjectValue(o)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	return res
}

// poolMemberIdentity renders a source-guarded member's requested source and
// its last identity observation (PSR-V0-083): the class, the reported pair,
// the runner's wall clock when the command ended and the receipt that
// recorded it. A member never observed shows NOT_OBSERVED.
func poolMemberIdentity(o *wire.Object, en snapshot.PoolEntry, occupied bool, id snapshot.PoolIdentity) {
	o.Set("source", wire.Null())
	if occupied && en.Source != nil {
		o.Set("source", snapshot.PoolSourceValue(en.Source))
	}
	if id.MemberID == "" {
		o.Set("identity", wire.String(poolNotObserved))
		return
	}
	o.Set("identity", wire.ObjectValue(wire.NewObject().Set("class", wire.String(id.Class)).Set("source", snapshot.PoolSourceValue(id.Source)).Set("allocationId", wire.String(string(id.AllocationID))).Set("outputSha256", wire.String(string(id.OutputSha256))).Set("observedAt", wire.String(string(id.ObservedAt))).Set("observedSeq", wire.String(string(id.ObservedSeq)))))
}

// poolMemberFree renders a member with no audited occupancy. Nothing holds
// it; its earlier health and cleanup outcomes left pool state with the
// allocation and are NOT_OBSERVED here.
func poolMemberFree(o *wire.Object) {
	o.Set("state", wire.String("FREE"))
	for _, key := range []string{"allocationId", "holder", "attemptId", "generation", "stage", "changedSeq", "commandKind", "observationSha256", "quarantine"} {
		o.Set(key, wire.Null())
	}
	o.Set("boundAttempts", wire.Array())
	o.Set("lastHealth", wire.String(poolNotObserved))
	o.Set("lastCleanup", wire.String(poolNotObserved))
}

func poolMemberOccupied(o *wire.Object, en snapshot.PoolEntry, evidence journal.Native) {
	o.Set("state", wire.String(en.State))
	o.Set("allocationId", wire.String(string(en.AllocationID)))
	o.Set("holder", wire.String(en.Holder))
	o.Set("stage", stringOrNull(en.Stage))
	// A PREPARING allocation has no attempt yet; its generation is not one.
	if en.AttemptID == "" {
		o.Set("attemptId", wire.Null()).Set("generation", wire.Null())
	} else {
		o.Set("attemptId", wire.String(en.AttemptID)).Set("generation", wire.String(string(en.Generation)))
	}
	o.Set("changedSeq", wire.String(string(en.ChangedSeq)))
	o.Set("boundAttempts", boundAttemptsValue(en))
	// The command kind outlives its command; it is pending only while a
	// health (PREPARING) or cleanup/sweep (CLEANING) runner owns the member.
	o.Set("commandKind", wire.Null())
	if en.State == "PREPARING" || en.State == "CLEANING" {
		o.Set("commandKind", stringOrNull(en.CommandKind))
	}
	o.Set("quarantine", wire.Null())
	if en.State == "QUARANTINED" {
		// The journal records sequence numbers, not wall-clock time.
		o.Set("quarantine", wire.ObjectValue(wire.NewObject().Set("reason", wire.String(en.Reason)).Set("changedSeq", wire.String(string(en.ChangedSeq))).Set("since", wire.String(poolNotObserved))))
	}
	o.Set("observationSha256", wire.Null())
	o.Set("lastHealth", wire.String(poolNotObserved))
	o.Set("lastCleanup", wire.String(poolNotObserved))
	if en.ObservationSha256 == nil {
		return
	}
	digest := *en.ObservationSha256
	o.Set("observationSha256", wire.String(string(digest)))
	raw, err := evidence.Read("evidence/"+string(digest), snapshot.MaxPoolObservationBytes)
	if err != nil || wire.Sum(raw) != digest {
		return
	}
	// A sweep phase observation has another profile; it stays NOT_OBSERVED.
	obs, err := snapshot.DecodePoolObservation(raw)
	if err != nil || obs.AllocationID != en.AllocationID {
		return
	}
	outcome := wire.NewObject().Set("class", wire.String(obs.Class)).Set("passed", wire.Bool(obs.Passed)).Set("groupClean", wire.Bool(obs.GroupClean)).Set("observationSha256", wire.String(string(digest))).Set("observedAt", wire.String(poolNotObserved))
	key := "lastHealth"
	if obs.Kind == "cleanup" {
		key = "lastCleanup"
	}
	o.Set(key, wire.ObjectValue(outcome))
}

// boundAttemptsValue lists the attempts an allocation binds: the entry's
// attempt as PRIMARY, then each PSR-V0-016 shared attempt as SHARED in
// binding order. Only an ALLOCATED member binds attempts.
func boundAttemptsValue(en snapshot.PoolEntry) wire.Value {
	out := []wire.Value{}
	if en.State != "ALLOCATED" || en.AttemptID == "" {
		return wire.Array()
	}
	add := func(id string, generation wire.Size, role string) {
		out = append(out, wire.ObjectValue(wire.NewObject().Set("attemptId", wire.String(id)).Set("generation", wire.String(string(generation))).Set("role", wire.String(role))))
	}
	add(en.AttemptID, en.Generation, "PRIMARY")
	for _, x := range en.Shared {
		add(x.AttemptID, x.Generation, "SHARED")
	}
	return wire.Array(out...)
}

// poolBindingValue is the derived `attempt show` view of the attempt's
// allocation in audited pools.json (PSR-V0-020): its member state, this
// attempt's role and every bound attempt. An allocation pools.json no
// longer holds is FREE with no role.
func poolBindingValue(raw []byte, a *snapshot.Attempt) (wire.Value, error) {
	o := wire.NewObject().Set("allocationId", wire.String(string(a.PoolAllocation.AllocationID))).Set("state", wire.String("FREE")).Set("role", wire.Null()).Set("boundAttempts", wire.Array())
	if len(raw) == 0 {
		return wire.ObjectValue(o), nil
	}
	state, err := snapshot.DecodePools(raw)
	if err != nil {
		return wire.Value{}, err
	}
	for _, en := range state.Entries {
		if en.AllocationID != a.PoolAllocation.AllocationID {
			continue
		}
		o.Set("state", wire.String(en.State)).Set("boundAttempts", boundAttemptsValue(en))
		if en.State != "ALLOCATED" {
			break
		}
		if en.AttemptID == a.AttemptID && en.Generation == a.Generation {
			o.Set("role", wire.String("PRIMARY"))
		}
		for _, x := range en.Shared {
			if x.AttemptID == a.AttemptID && x.Generation == a.Generation {
				o.Set("role", wire.String("SHARED"))
			}
		}
	}
	return wire.ObjectValue(o), nil
}
