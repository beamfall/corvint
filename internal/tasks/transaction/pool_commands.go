package transaction

import (
	"bytes"
	"slices"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PoolFacts are observations gathered outside the writer lock and consumed
// only with the exact durable allocation and unchanged member definition.
type PoolFacts struct {
	AllocationID            wire.Digest
	SweepSelections         []PoolSweepSelection
	SweepOwner              wire.Digest
	SweepLog, SweepResult   []byte
	Tree                    string
	Observation             []byte
	RunnerPID               wire.Count
	RunnerStarted, Revision string
	RunnerGone              bool
}

func (c leaseContext) poolEntry() *snapshot.PoolEntry {
	for _, en := range c.st.pools.Entries {
		if en.MemberID == c.l.Member && string(en.AllocationID) == c.l.Allocation {
			x := en
			return &x
		}
	}
	return nil
}
func (c leaseContext) putPool(en *snapshot.PoolEntry, remove bool, observation []byte) leaseOutcome {
	state := *c.st.pools
	state.Entries = []snapshot.PoolEntry{}
	for _, old := range c.st.pools.Entries {
		if old.MemberID != en.MemberID {
			state.Entries = append(state.Entries, old)
		}
	}
	en.ChangedSeq = c.seq
	if !remove {
		state.Entries = append(state.Entries, *en)
	}
	raw, e := state.Encode()
	if e != nil {
		return c.fail(e)
	}
	posts := map[string][]byte{"pools.json": raw}
	if len(observation) > 0 {
		posts["evidence/"+string(wire.Sum(observation))] = observation
	}
	return leaseOutcome{posts: posts, effect: &leaseEffect{kind: "TRANSITION", outcome: mutation.OutcomeCompleted, codes: []string{}}, detail: en.State + ": " + en.MemberID}
}
func planPoolPrepare(c leaseContext) leaseOutcome {
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "pool preparation blocked by barrier")
	}
	p := c.st.policy.Pool(c.l.Pool)
	if p == nil {
		return c.fail(malformed("unknown pool"))
	}
	found := false
	for _, m := range p.Members {
		found = found || m == c.l.Member
	}
	if !found || p.MemberConfig[c.l.Member].Health == nil {
		return c.fail(malformed("member has no health command"))
	}
	if stage := p.ReservedFor[c.l.Member]; stage != "" && stage != c.l.Stage {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "member reserved for another stage")
	}
	if c.l.ExcludeAuthors != "" {
		// CAL-V0-098: rederive at this snapshot, so a member that became an
		// author after the claim's refusal is never prepared or probed. The
		// request carries the claim's explicit members, so it covers an
		// unrecorded generation (CAL-V0-107) only when the claim's caller
		// asserted that cover; otherwise one raced in refuses before any
		// health command runs.
		x, why := DeriveAuthors(c.st.attempts, c.l.TicketID, c.l.ExcludeAuthors, c.l.Pool, c.l.ExcludeMembers)
		if x == nil {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeIndependenceUnverified, why)
		}
		if slices.Contains(x.Excluded, c.l.Member) {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "member "+c.l.Member+" is an excluded implement author; "+x.AuthorsDetail())
		}
	}
	digest, e := wire.ParseDigest("claim binding", c.l.Evidence)
	if e != nil {
		return c.fail(e)
	}
	for _, en := range c.st.pools.Entries {
		if en.MemberID == c.l.Member || (en.RequestSha256 == digest && en.State == "PREPARING") {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "member or request already occupied")
		}
	}
	f := c.in.LeaseFacts.Pool
	if f.RunnerPID == "" || f.RunnerPID == "0" || f.RunnerStarted == "" {
		return c.fail(malformed("preparation lacks runner identity"))
	}
	if _, e := wire.ParseOID("command revision", f.Revision); e != nil {
		return c.fail(e)
	}
	en := &snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: p.ID, MemberID: c.l.Member, AllocationID: wire.Sum([]byte(c.r.QueueID + ":" + c.r.RequestID + ":" + c.l.Member)), DefinitionSha256: c.st.policy.MemberDefinition(p.ID, c.l.Member), AllocatedSeq: c.seq, ConfigRef: p.MemberConfig[c.l.Member].ConfigRef}, State: "PREPARING", Holder: c.l.Holder, Stage: c.l.Stage, Generation: "0", PolicySha256: c.st.policy.PolicySha256(), RequestSha256: digest, RunnerPID: f.RunnerPID, RunnerStarted: f.RunnerStarted, CommandKind: "health", CommandRevision: f.Revision}
	return c.putPool(en, false, nil)
}
func (c leaseContext) observation(en *snapshot.PoolEntry) (*snapshot.PoolObservation, error) {
	raw := c.in.LeaseFacts.Pool.Observation
	o, e := snapshot.DecodePoolObservation(raw)
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(raw, o.Encode()) || o.AllocationID != en.AllocationID || o.DefinitionSha256 != en.DefinitionSha256 || o.Kind != en.CommandKind || o.Revision != en.CommandRevision {
		return nil, malformed("pool observation binding differs")
	}
	return o, nil
}
func planPoolObserve(c leaseContext) leaseOutcome {
	en := c.poolEntry()
	if en == nil {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation differs")
	}
	if en.State != "PREPARING" && en.State != "CLEANING" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "no pending pool command")
	}
	if en.Sweep != nil {
		return planPoolSweepObserve(c, en)
	}
	o, e := c.observation(en)
	if e != nil {
		return c.fail(e)
	}
	d := wire.Sum(c.in.LeaseFacts.Pool.Observation)
	en.ObservationSha256 = &d
	en.CleanupPassed = o.Kind == "cleanup" && o.Passed
	en.State = "QUARANTINED"
	en.Reason = o.Class + "; operator safe-reuse confirmation required"
	en.RunnerPID = "0"
	en.RunnerStarted = ""
	return c.putPool(en, false, c.in.LeaseFacts.Pool.Observation)
}
func planPoolCleanup(c leaseContext) leaseOutcome {
	en := c.poolEntry()
	if en == nil {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation differs")
	}
	if en.Sweep != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "sweep owns allocation")
	}
	if en.State != "QUARANTINED" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "cleanup requires quarantine")
	}
	if c.st.policy.Pool(en.PoolID).MemberConfig[en.MemberID].Cleanup == nil {
		return c.fail(malformed("no configured cleanup command"))
	}
	f := c.in.LeaseFacts.Pool
	if f.RunnerPID == "" || f.RunnerPID == "0" || f.RunnerStarted == "" {
		return c.fail(malformed("cleanup lacks runner identity"))
	}
	if _, e := wire.ParseOID("command revision", f.Revision); e != nil {
		return c.fail(e)
	}
	en.State = "CLEANING"
	en.RunnerPID = f.RunnerPID
	en.RunnerStarted = f.RunnerStarted
	en.CommandKind = "cleanup"
	en.CommandRevision = f.Revision
	en.CleanupPassed = false
	en.ObservationSha256 = nil
	return c.putPool(en, false, nil)
}
func planPoolRecover(c leaseContext) leaseOutcome {
	en := c.poolEntry()
	if en == nil {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation differs")
	}
	if en.State != "PREPARING" && en.State != "CLEANING" && !(en.State == "QUARANTINED" && en.Sweep != nil) {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "no orphaned pool command")
	}
	if !c.in.LeaseFacts.Pool.RunnerGone {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "runner is live or identity unknown")
	}
	en.State = "QUARANTINED"
	en.Reason = "orphaned command; execution and physical cleanup unknown: " + c.l.Reason
	en.RunnerPID = "0"
	en.RunnerStarted = ""
	en.CleanupPassed = false
	en.Sweep = nil
	return c.putPool(en, false, nil)
}
