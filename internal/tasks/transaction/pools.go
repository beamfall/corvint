package transaction

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"slices"
)

func loadPools(r Request, in Input, st inputState) (*snapshot.PoolState, error) {
	p := &snapshot.PoolState{QueueID: st.queue.QueueID, Entries: []snapshot.PoolEntry{}}
	if len(in.Pools) == 0 {
		if _, ok := in.Inventory.files["pools.json"]; ok {
			return nil, malformed("pool projection missing")
		}
		return p, nil
	}
	var e error
	p, e = snapshot.DecodePools(in.Pools)
	if e != nil {
		return nil, e
	}
	raw, e := p.Encode()
	if e != nil || !bytes.Equal(raw, in.Pools) || p.QueueID.Raw != r.QueueID || !in.Inventory.matches("pools.json", in.Pools) {
		return nil, malformed("pool projection binding")
	}
	for _, entry := range p.Entries {
		if st.head != nil && entry.ChangedSeq.Uint64() > st.head.LastSeq.Uint64() {
			return nil, malformed("pool sequence exceeds head")
		}
		pool := st.policy.Pool(entry.PoolID)
		if pool == nil || st.policy.MemberDefinition(entry.PoolID, entry.MemberID) != entry.DefinitionSha256 {
			return nil, malformed("occupied pool definition changed")
		}
		member := false
		for _, m := range pool.Members {
			member = member || m == entry.MemberID
		}
		if !member {
			return nil, malformed("occupied member removed")
		}
		if r.Operation == Lease && entry.State == "ALLOCATED" {
			a := st.attempts[entry.AttemptID]
			if a == nil || !a.Live() || a.Generation != entry.Generation || a.PoolAllocation == nil || !sameAllocation(a.PoolAllocation, &entry.PoolAllocation) || a.Lease.Holder != entry.Holder || a.Stage != entry.Stage {
				return nil, malformed("pool allocation differs from live attempt")
			}
		}
	}
	if r.Operation == Lease {
		for _, a := range st.attempts {
			if a.Live() && a.PoolAllocation != nil {
				found := false
				for _, en := range p.Entries {
					found = found || en.State == "ALLOCATED" && en.AttemptID == a.AttemptID && en.Generation == a.Generation && sameAllocation(a.PoolAllocation, &en.PoolAllocation) && a.Lease.Holder == en.Holder && a.Stage == en.Stage
				}
				if !found {
					return nil, malformed("live attempt missing pool occupancy")
				}
			}
		}
	}
	return p, nil
}
func (c leaseContext) allocate(a *snapshot.Attempt) (*snapshot.PoolAllocation, error) {
	if c.l.Pool == "" {
		return nil, nil
	}
	if e := CheckPoolExclusions(c.l.Pool, c.l.ExcludeMembers, c.st.policy); e != nil {
		return nil, e
	}
	pool := c.st.policy.Pool(c.l.Pool)
	if pool == nil {
		return nil, malformed("unknown pool")
	}
	if id := c.in.LeaseFacts.Pool.AllocationID; id != "" {
		for _, en := range c.st.pools.Entries {
			if en.AllocationID != id {
				continue
			}
			if en.State != "PREPARING" || en.PoolID != c.l.Pool || en.Holder != c.l.Holder || en.Stage != c.l.Stage || en.RequestSha256 != PoolClaimBinding(c.r.Lease, c.st.queue.QueueID) {
				return nil, malformed("prepared allocation claim differs")
			}
			if !slices.Contains(OrderedPoolMembers(pool, c.l.Stage, c.l.ExcludeMembers), en.MemberID) {
				return nil, wire.Errorf(wire.CodeResourceCollision, "pool", "prepared member is not eligible")
			}
			o, e := c.observation(&en)
			if e != nil {
				return nil, e
			}
			if !o.Passed {
				return nil, malformed("health did not pass")
			}
			a := en.PoolAllocation
			return &a, nil
		}
		return nil, malformed("prepared allocation missing")
	}
	for _, member := range OrderedPoolMembers(pool, c.l.Stage, c.l.ExcludeMembers) {
		occupied := false
		for _, en := range c.st.pools.Entries {
			occupied = occupied || en.MemberID == member
		}
		if occupied {
			continue
		}
		config := pool.MemberConfig[member]
		if config.Health != nil {
			return nil, wire.Errorf(wire.CodeQuiescenceUnproved, "pool", "health preparation required")
		}
		return &snapshot.PoolAllocation{PoolID: pool.ID, MemberID: member, AllocationID: wire.Sum([]byte(c.r.QueueID + ":" + c.r.RequestID + ":" + member)), DefinitionSha256: c.st.policy.MemberDefinition(pool.ID, member), AllocatedSeq: c.seq, ConfigRef: config.ConfigRef}, nil
	}
	return nil, wire.Errorf(wire.CodeResourceCollision, "pool", "no eligible free member; occupied and quarantined members unavailable")
}

// OrderedPoolMembers returns members eligible for stage, preferring an exact
// reservation while retaining unreserved members as fallback capacity. The optional
// exclusion set is applied within each tier (CAL-V0-065).
func OrderedPoolMembers(pool *intent.Pool, stage string, exclusions ...[]string) []string {
	excluded := func(member string) bool {
		return len(exclusions) != 0 && slices.Contains(exclusions[0], member)
	}
	members := make([]string, 0, len(pool.Members))
	if stage != "" {
		for _, member := range pool.Members {
			if pool.ReservedFor[member] == stage && !excluded(member) {
				members = append(members, member)
			}
		}
	}
	for _, member := range pool.Members {
		if pool.ReservedFor[member] == "" && !excluded(member) {
			members = append(members, member)
		}
	}
	return members
}
func (c leaseContext) poolPosts(a *snapshot.Attempt, posts map[string][]byte) error {
	if a.PoolAllocation == nil {
		return nil
	}
	state := *c.st.pools
	state.Entries = append([]snapshot.PoolEntry{}, state.Entries...)
	found := false
	for i, en := range state.Entries {
		if en.AllocationID != a.PoolAllocation.AllocationID {
			continue
		}
		found = true
		if a.Live() && en.State == "PREPARING" {
			state.Entries[i].State = "ALLOCATED"
			state.Entries[i].AttemptID = a.AttemptID
			state.Entries[i].Generation = a.Generation
			state.Entries[i].RunnerPID = "0"
			state.Entries[i].RunnerStarted = ""
			d := wire.Sum(c.in.LeaseFacts.Pool.Observation)
			state.Entries[i].ObservationSha256 = &d
			posts["evidence/"+string(d)] = c.in.LeaseFacts.Pool.Observation
		}
		// Only the fresh validated explicit release may omit this occupancy.
		if !a.Live() && c.l.Verb == LeaseRelease && c.l.LaneUntouched && a.LaneUntouchedAttestation != nil {
			state.Entries = append(state.Entries[:i], state.Entries[i+1:]...)
			break
		}
		if !a.Live() {
			state.Entries[i].State = "QUARANTINED"
			state.Entries[i].ChangedSeq = c.seq
			state.Entries[i].Reason = "logical attempt ended; physical safe reuse unproved"
		}
	}
	if !found {
		if !a.Live() {
			return malformed("terminal attempt lacks pool allocation")
		}
		state.Entries = append(state.Entries, snapshot.PoolEntry{PoolAllocation: *a.PoolAllocation, State: "ALLOCATED", Holder: a.Lease.Holder, Stage: a.Stage, AttemptID: a.AttemptID, Generation: a.Generation, ChangedSeq: c.seq, PolicySha256: a.PolicySha256, RequestSha256: wire.Sum([]byte(c.r.RequestID)), Reason: ""})
	}
	raw, e := state.Encode()
	if e == nil {
		posts["pools.json"] = raw
	}
	return e
}
func planPoolSafe(c leaseContext) leaseOutcome {
	if c.r.Actor.Role != "OWNER" && c.r.Actor.Role != "OPERATOR" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, "safe reuse requires operator attestation")
	}
	state := *c.st.pools
	state.Entries = []snapshot.PoolEntry{}
	var found *snapshot.PoolEntry
	for _, en := range c.st.pools.Entries {
		if en.MemberID == c.l.Member && string(en.AllocationID) == c.l.Allocation {
			copy := en
			found = &copy
		} else {
			state.Entries = append(state.Entries, en)
		}
	}
	if found == nil {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation is not current")
	}
	if found.State != "QUARANTINED" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeAttemptLive, "member is not quarantined")
	}
	if c.st.policy.Pool(found.PoolID).MemberConfig[found.MemberID].Cleanup != nil && !found.CleanupPassed {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "configured cleanup observation required")
	}
	raw, e := state.Encode()
	if e != nil {
		return c.fail(e)
	}
	return leaseOutcome{posts: map[string][]byte{"pools.json": raw}, effect: &leaseEffect{kind: "TRANSITION", attemptID: found.AttemptID, generation: found.Generation, outcome: mutation.OutcomeCompleted, codes: []string{}}}
}
func checkPoolStage(stage string) bool {
	if stage == "" {
		return true
	}
	for _, x := range intent.StageRoles {
		if x == stage {
			return true
		}
	}
	return false
}

func sameAllocation(a, b *snapshot.PoolAllocation) bool {
	return bytes.Equal(wire.EncodeFile(snapshot.PoolAllocationValue(a)), wire.EncodeFile(snapshot.PoolAllocationValue(b)))
}

func PoolClaimBinding(l *LeaseRequest, q wire.QueueID) wire.Digest {
	v, e := leaseValue(l, q)
	if e != nil {
		return ""
	}
	return wire.Sum(wire.EncodeFile(v))
}

// checkExcludedMembers validates only request facts, so replay does not depend
// on today's policy (CAL-V0-065). A nil slice means historical omission.
func checkExcludedMembers(pool string, excluded []string) error {
	if excluded == nil {
		return nil
	}
	if pool == "" {
		return malformed("member exclusions require an explicit pool")
	}
	if len(excluded) == 0 || len(excluded) > intent.MaxPoolMembers {
		return limit("excluded member count")
	}
	for i, member := range excluded {
		if _, e := wire.ParseLabel("excludeMembers", member); e != nil {
			return e
		}
		if i > 0 && excluded[i-1] >= member {
			return malformed("excluded members must be sorted and duplicate-free")
		}
	}
	return nil
}

// CheckPoolExclusions checks current requested-pool membership only for fresh
// selection, health preparation and admission; callers must resolve replay first.
func CheckPoolExclusions(poolID string, excluded []string, policy *intent.Policy) error {
	if e := checkExcludedMembers(poolID, excluded); e != nil {
		return e
	}
	if excluded == nil {
		return nil
	}
	pool := policy.Pool(poolID)
	if pool == nil {
		return malformed("unknown exclusion pool")
	}
	for _, member := range excluded {
		if !slices.Contains(pool.Members, member) {
			return malformed("excluded member does not belong to requested pool")
		}
	}
	return nil
}
