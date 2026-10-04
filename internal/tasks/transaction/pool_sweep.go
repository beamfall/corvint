package transaction

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type PoolSweepSelection struct {
	Pool, Member           string
	Allocation, Definition wire.Digest
}

// CheckPoolSweepResultCapacity proves the largest possible aggregate for this
// selection before any ownership or command effects. false is longer than true;
// both digest fields have fixed length. Escaping uses the actual canonical codec.
func CheckPoolSweepResultCapacity(owner wire.Digest, selections []PoolSweepSelection) error {
	rows := make([]wire.Value, 0, len(selections))
	if len(selections) > 256 {
		return limit("sweep selections")
	}
	for _, sel := range selections {
		rows = append(rows, wire.ObjectValue(wire.NewObject().Set("member", wire.String(sel.Member)).Set("allocation", wire.String(string(sel.Allocation))).Set("free", wire.Bool(false)).Set("observation", wire.String(string(owner)))))
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String("taskman-pool-sweep-result/0")).Set("owner", wire.String(string(owner))).Set("members", wire.Array(rows...))))
	if len(raw) > 65536 {
		return limit("sweep result capacity before ownership; select a smaller batch")
	}
	return nil
}

func sweepPosts(c leaseContext, state *snapshot.PoolState, extra map[string][]byte) leaseOutcome {
	raw, e := state.Encode()
	if e != nil {
		return c.fail(e)
	}
	posts := map[string][]byte{"pools.json": raw}
	for p, b := range extra {
		posts[p] = b
	}
	return leaseOutcome{posts: posts, effect: &leaseEffect{kind: "TRANSITION", outcome: mutation.OutcomeCompleted, codes: []string{}}}
}
func planPoolSweep(c leaseContext) leaseOutcome {
	f := c.in.LeaseFacts.Pool
	if f.RunnerPID.Int() <= 0 || f.RunnerStarted == "" || f.Revision == "" {
		return c.fail(wire.Errorf(wire.CodeMissingEvidence, "sweep", "runner/source facts absent"))
	}
	d, e := Digest(c.r)
	if e != nil {
		return c.fail(e)
	}
	if e = CheckPoolSweepResultCapacity(d, f.SweepSelections); e != nil {
		return c.fail(e)
	}
	state := *c.st.pools
	state.Entries = append([]snapshot.PoolEntry{}, state.Entries...)
	if len(f.SweepSelections) > 256 {
		return c.fail(limit("sweep selections"))
	}
	if c.l.Allocation != "" && len(f.SweepSelections) != 1 {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "selected allocation absent")
	}
	seen := map[string]bool{}
	for _, sel := range f.SweepSelections {
		if seen[sel.Member] || (c.l.Member != "" && sel.Member != c.l.Member) {
			return c.fail(malformed("sweep selection"))
		}
		if c.l.Allocation != "" && c.l.Allocation != string(sel.Allocation) {
			return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "selected allocation changed")
		}
		seen[sel.Member] = true
		found := false
		for i := range state.Entries {
			en := &state.Entries[i]
			if en.MemberID != sel.Member {
				continue
			}
			found = true
			p := c.st.policy.Pool(en.PoolID)
			if en.PoolID != sel.Pool || en.AllocationID != sel.Allocation || en.DefinitionSha256 != sel.Definition || p == nil || p.MemberConfig[en.MemberID].SafeReuse == nil || c.st.policy.MemberDefinition(en.PoolID, en.MemberID) != en.DefinitionSha256 {
				return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "selection changed")
			}
			if en.State != "QUARANTINED" || en.Sweep != nil {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "member occupied")
			}
			phase := "reset"
			if p.MemberConfig[en.MemberID].Cleanup != nil {
				phase = "cleanup"
			}
			en.State = "CLEANING"
			en.Sweep = &snapshot.PoolSweepOwner{RequestSha256: d, Phase: phase, Attempt: "1", Tree: f.Tree}
			en.RunnerPID = f.RunnerPID
			en.RunnerStarted = f.RunnerStarted
			en.CommandRevision = f.Revision
			en.CommandKind = "sweep"
			en.ObservationSha256 = nil
			en.CleanupPassed = false
			en.ChangedSeq = c.seq
		}
		if !found {
			return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation absent")
		}
	}
	return sweepPosts(c, &state, nil)
}
func planPoolSweepObserve(c leaseContext, en *snapshot.PoolEntry) leaseOutcome {
	f := c.in.LeaseFacts.Pool
	o, e := snapshot.DecodePoolSweepObservation(f.Observation)
	if e != nil {
		return c.fail(e)
	}
	owner := en.Sweep
	if c.l.Evidence != string(owner.RequestSha256) || o.Owner != owner.RequestSha256 || o.AllocationID != en.AllocationID || o.DefinitionSha256 != en.DefinitionSha256 || o.Revision != en.CommandRevision || o.Tree != owner.Tree || o.Phase != owner.Phase || o.Attempt != owner.Attempt || !bytes.Equal(wire.EncodeFile(sweepPrevious(o.Previous)), wire.EncodeFile(sweepPrevious(owner.Previous))) {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "sweep observation differs")
	}
	if wire.Sum(f.SweepLog) != o.Log {
		return c.fail(malformed("sweep log digest"))
	}
	stdout, stderr, e := snapshot.DecodePoolSweepLog(f.SweepLog)
	if e != nil {
		return c.fail(e)
	}
	if wire.Sum(stdout) != o.Stdout || wire.Sum(stderr) != o.Stderr {
		return c.fail(malformed("sweep stream digests"))
	}
	p := c.st.policy.Pool(en.PoolID)
	if p == nil || c.st.policy.MemberDefinition(en.PoolID, en.MemberID) != en.DefinitionSha256 {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "definition changed")
	}
	safe := p.MemberConfig[en.MemberID].SafeReuse
	if safe == nil {
		return c.fail(malformed("safe reuse absent"))
	}
	d := wire.Sum(f.Observation)
	owner.Previous = &d
	en.ObservationSha256 = &d
	en.ChangedSeq = c.seq
	// Under ALL only an owned successful verify may still finalize; no further
	// cleanup/reset/verify command is authorized, so other outcomes end here.
	all := c.st.barrier != nil && c.st.barrier.Scope == "ALL"
	if o.Passed && (!all || o.Phase == "verify") {
		switch o.Phase {
		case "cleanup":
			en.CleanupPassed = true
			owner.Phase = "reset"
		case "reset":
			owner.Phase = "verify"
		case "verify":
			owner.Phase = "confirm"
		}
	} else if !all && o.GroupClean && (o.Class == "EXIT_NONZERO" || o.Class == "STDOUT_MISMATCH") && owner.Attempt.Int() < safe.MaxAttempts.Int() {
		owner.Attempt = wire.CountOf(owner.Attempt.Int() + 1)
		owner.Phase = "reset"
		if p.MemberConfig[en.MemberID].Cleanup != nil {
			owner.Phase = "cleanup"
			en.CleanupPassed = false
		}
	} else {
		en.State = "QUARANTINED"
		en.Reason = o.Class + "; safe reuse failed"
		if all {
			en.Reason = o.Class + "; ALL barrier ended sweep before next phase"
		}
		if o.GroupClean {
			en.Sweep = nil
			en.RunnerPID = "0"
			en.RunnerStarted = ""
		}
	}
	state := *c.st.pools
	state.Entries = append([]snapshot.PoolEntry{}, state.Entries...)
	for i := range state.Entries {
		if state.Entries[i].MemberID == en.MemberID {
			state.Entries[i] = *en
		}
	}
	return sweepPosts(c, &state, map[string][]byte{"evidence/" + string(d): f.Observation, "evidence/" + string(o.Log): f.SweepLog})
}
func sweepPrevious(d *wire.Digest) wire.Value {
	if d == nil {
		return wire.Null()
	}
	return wire.String(string(*d))
}
func planPoolSweepSafe(c leaseContext, en *snapshot.PoolEntry) leaseOutcome {
	if c.l.Evidence != string(en.Sweep.RequestSha256) || en.Sweep.Phase != "confirm" || en.ObservationSha256 == nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "owned sweep confirmation only")
	}
	raw := c.in.LeaseFacts.Pool.Observation
	o, e := snapshot.DecodePoolSweepObservation(raw)
	if e != nil {
		return c.fail(e)
	}
	if !o.Passed || !o.GroupClean || o.Phase != "verify" || o.Owner != en.Sweep.RequestSha256 || wire.Sum(raw) != *en.ObservationSha256 || en.Sweep.Previous == nil || wire.Sum(raw) != *en.Sweep.Previous || o.AllocationID != en.AllocationID || o.DefinitionSha256 != en.DefinitionSha256 || !c.in.Inventory.matches("evidence/"+string(wire.Sum(raw)), raw) {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "successful owned witness differs")
	}
	if c.st.policy.MemberDefinition(en.PoolID, en.MemberID) != en.DefinitionSha256 {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "definition changed")
	}
	if c.st.policy.Pool(en.PoolID).MemberConfig[en.MemberID].Cleanup != nil && !en.CleanupPassed {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "cleanup required")
	}
	return c.putPool(en, true, nil)
}
func planPoolSweepFinish(c leaseContext) leaseOutcome {
	raw := c.in.LeaseFacts.Pool.SweepResult
	if len(raw) == 0 {
		return c.fail(wire.Errorf(wire.CodeMissingEvidence, "sweep", "pending original request; execution never repeated"))
	}
	if len(raw) > 65536 {
		return c.fail(limit("sweep result"))
	}
	v, e := wire.Parse(raw)
	if e != nil || !bytes.Equal(raw, wire.EncodeFile(v)) {
		return c.fail(malformed("sweep result canonical bytes"))
	}
	r := wire.NewReader(v, "sweep result")
	r.Closed("profile", "owner", "members")
	r.Field("profile").Exact("taskman-pool-sweep-result/0")
	owner := r.Field("owner").Digest()
	if string(owner) != c.l.Evidence {
		return c.fail(malformed("sweep result owner"))
	}
	seen := map[string]bool{}
	for _, row := range r.Field("members").Array(256, false) {
		row.Closed("member", "allocation", "free", "observation")
		member := row.Field("member").Label()
		allocation := row.Field("allocation").Digest()
		free := row.Field("free").Bool()
		observation := row.Field("observation").Digest()
		if seen[member] {
			return c.fail(malformed("duplicate sweep member"))
		}
		seen[member] = true
		witness := false
		for _, f := range c.in.Inventory.Files() {
			if f.Path == "evidence/"+string(observation) && f.Sha256 == observation {
				witness = true
			}
		}
		if !witness && !free && observation == owner {
			// No phase committed: the owner digest is a placeholder, admitted only
			// after explicit recovery released that owner. It is never a witness.
			for _, en := range c.st.pools.Entries {
				if en.MemberID == member && en.AllocationID == allocation && en.Sweep != nil && en.Sweep.RequestSha256 == owner {
					return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "original sweep owner still held")
				}
			}
			witness = true
		}
		if !witness {
			return c.fail(malformed("sweep witness absent"))
		}
		for _, en := range c.st.pools.Entries {
			if en.MemberID == member && en.AllocationID == allocation && free {
				return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "original allocation remains")
			}
		}
	}
	if e := r.Err(); e != nil {
		return c.fail(e)
	}
	// Phase witnesses retain exact journal bytes; the aggregate never grants physical authority.
	return leaseOutcome{posts: map[string][]byte{"evidence/" + string(wire.Sum(raw)): raw}, effect: &leaseEffect{kind: "TRANSITION", outcome: mutation.OutcomeCompleted, codes: []string{}}}
}
