package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/lifecycle"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// lifecycles derives the TLC-V0-001 lifecycle of each of ids with at most
// one attempt read and one batched Git observation (TLC-V0-006); in is the
// caller's plan input when it already holds one. It never writes
// (TLC-V0-007). A failed attempt read degrades to NOT_OBSERVED with a
// warning instead of failing the read.
func lifecycles(rc *readCtx, ids []string, in *transaction.PlanInput) (map[string]lifecycle.Lifecycle, []string) {
	inv := rc.store.Inventory
	var warnings []string
	live := false
	for _, id := range ids {
		if s, ok := inv.Summary(id); ok && !ticket.Closed(s.Status) {
			live = true
			break
		}
	}
	observed := false
	var attempts map[string]*snapshot.Attempt
	if live && !rc.journalAbsent {
		if in == nil {
			if p, _, err := planInput(rc); err != nil {
				warnings = append(warnings, "lifecycle attempts NOT_OBSERVED: "+err.Error())
			} else {
				in = &p
			}
		}
		if in != nil {
			observed, attempts = true, liveAttemptsByTicket(*in)
		}
	}
	inputs := make(map[string]*lifecycle.Input, len(ids))
	var queries []store.DeliveryQuery
	var queried []string
	for _, id := range ids {
		s, ok := inv.Summary(id)
		if !ok {
			continue
		}
		x := &lifecycle.Input{Status: s.Status, AttemptsObserved: observed}
		inputs[id] = x
		if ticket.Closed(s.Status) {
			continue
		}
		rec, _ := inv.Get(id)
		x.Progress = rec.EffectiveProgress()
		for _, h := range rec.Holds {
			x.HoldIDs = append(x.HoldIDs, h.HoldID)
		}
		if a := attempts[id]; a != nil {
			x.Attempt = &lifecycle.Attempt{ID: a.AttemptID, Phase: a.Phase, Stage: a.Stage, CandidateTreeOid: a.CandidateTreeOid}
		}
		if s.Status == ticket.StatusOpen && x.Attempt == nil && lifecycle.NeedsGit(x.Progress) {
			branch := ""
			if x.Progress.Branch != nil {
				branch = *x.Progress.Branch
			}
			queries = append(queries, store.DeliveryQuery{Branch: branch, Head: *x.Progress.Head})
			queried = append(queried, id)
		}
	}
	if len(queries) > 0 {
		var obs store.DeliveryObservation
		if rc.repo == nil || rc.bundle != nil {
			obs.Failed = true
		} else {
			obs = store.ObserveDeliveries(rc.repo.PrimaryWorktree, rc.store.Queue.IntentBranch, queries)
		}
		for i, id := range queried {
			g := &lifecycle.Git{Failed: obs.Failed, IntegrationRef: obs.IntegrationRef, IntegrationCommit: obs.IntegrationCommit}
			if !obs.Failed {
				r := obs.Results[i]
				g.Ref, g.RefCommit, g.HeadExists, g.Integrated, g.HeadOnRef = r.Ref, r.RefCommit, r.HeadExists, r.Integrated, r.HeadOnRef
			}
			inputs[id].Git = g
		}
	}
	out := make(map[string]lifecycle.Lifecycle, len(inputs))
	for id, x := range inputs {
		out[id] = lifecycle.Derive(*x)
	}
	return out, warnings
}

// lifecycleStates are the --lifecycle values (TLC-V0-001).
var lifecycleStates = lifecycle.States

// lifecycleCounts counts the lifecycles of ids by state, every state listed
// (TLC-V0-009, TLC-V0-011).
func lifecycleCounts(ls map[string]lifecycle.Lifecycle, ids []string) wire.Value {
	counts := zeroCounts(lifecycle.States)
	for _, id := range ids {
		if l, ok := ls[id]; ok {
			counts[l.State]++
		}
	}
	return countsValue(counts)
}

// lifecycleFlag removes `--lifecycle STATE` from args.
func lifecycleFlag(args []string) ([]string, *string, error) {
	var rest []string
	var state *string
	for i := 0; i < len(args); i++ {
		if args[i] != "--lifecycle" {
			rest = append(rest, args[i])
			continue
		}
		if state != nil || i+1 >= len(args) {
			return nil, nil, wire.Errorf(wire.CodeMalformed, "--lifecycle", "--lifecycle takes one state, given once")
		}
		i++
		if !lifecycle.IsState(args[i]) {
			return nil, nil, wire.Errorf(wire.CodeMalformed, "--lifecycle", "unknown lifecycle state %q", args[i])
		}
		s := args[i]
		state = &s
	}
	return rest, state, nil
}

// releaseLifecycles adds the TLC-V0-011 lifecycle counts of the members of r
// to members, and lifecycleMembers to o when state is given, from one
// inventory read; a failed read counts every member UNKNOWN with a warning.
func releaseLifecycles(env Env, r *release.Record, members wire.Value, state *string, o *wire.Object) []string {
	ids := make([]string, 0, len(r.TicketIDs))
	for _, id := range r.TicketIDs {
		ids = append(ids, id.Raw)
	}
	var lcs map[string]lifecycle.Lifecycle
	var warnings []string
	env.lazyClosed = true
	_, err := withInventoryStore(env, func(rc *readCtx) error {
		lcs, warnings = lifecycles(rc, ids, nil)
		return nil
	})
	if err != nil {
		lcs, warnings = map[string]lifecycle.Lifecycle{}, []string{"lifecycle NOT_OBSERVED: " + err.Error()}
		unknown := lifecycle.Derive(lifecycle.Input{})
		for _, id := range ids {
			lcs[id] = unknown
		}
	}
	present := ids[:0:0]
	for _, id := range ids {
		if _, ok := lcs[id]; ok {
			present = append(present, id)
		}
	}
	members.Obj.Set("byLifecycle", lifecycleCounts(lcs, present))
	if state != nil {
		var in []string
		for _, id := range present {
			if lcs[id].State == *state {
				in = append(in, id)
			}
		}
		o.Set("lifecycleMembers", driftList(in))
	}
	return warnings
}
