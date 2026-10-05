package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"slices"
	"strings"
)

// planCommand dispatches `plan preview`; `plan record` stays NOT_RUN.
func planCommand(env Env, args []string) *wire.Result {
	if len(args) >= 1 && args[0] == "preview" {
		return planPreview(env, args[1:])
	}
	if len(args) >= 1 && args[0] == "record" {
		return notRun([]string{"plan", "record"})
	}
	return usage([]string{"plan"}, "plan needs the verb preview")
}

// planPreview is the CAL-V0-014 taskman-priority-first/0 plan over the
// intent inventory, the live reservations and the attempt history, as a
// pure read. No plan is pinned, so deferredSinceSeq is null.
func planPreview(env Env, args []string) *wire.Result {
	cmd := []string{"plan", "preview"}
	pool, stage := "", ""
	var excluded []string
	selectedOnly := false
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		if seen[args[i]] && args[i] != "--exclude-member" {
			return usage(cmd, "duplicate plan flag")
		}
		seen[args[i]] = true
		switch args[i] {
		case "--exclude-member":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return usage(cmd, "--exclude-member needs one nonempty member")
			}
			i++
			excluded = append(excluded, args[i])
		case "--selected-only":
			selectedOnly = true
		case "--pool":
			i++
			if i >= len(args) {
				return usage(cmd, "missing pool or stage value")
			}
			pool = args[i]
			if _, err := wire.ParseLabel("pool", pool); err != nil {
				return usage(cmd, "invalid pool label")
			}
		case "--stage":
			i++
			if i >= len(args) {
				return usage(cmd, "missing pool or stage value")
			}
			stage = args[i]
			if !slices.Contains(intent.StageRoles, stage) {
				return usage(cmd, "unknown pool stage")
			}
		default:
			return usage(cmd, "unknown plan flag")
		}
	}
	var item wire.Value
	rc, err := withStore(env, func(rc *readCtx) error {
		in, digest, err := planInput(rc)
		if err != nil {
			return err
		}
		in.Pool, in.Stage, in.ExcludeMembers = pool, stage, scopePaths(excluded)
		if err := transaction.CheckPoolExclusions(pool, in.ExcludeMembers, in.Policy); err != nil {
			return err
		}
		plan := transaction.PriorityFirst(in)
		if selectedOnly {
			item = selectedPlanValue(rc, plan)
			return nil
		}
		item, err = planValue(rc, digest, plan)
		return err
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	return res
}

// selectedPlanValue renders the complete selected roster without the detailed
// blocker records. It is a projection of the same plan, never a second plan.
func selectedPlanValue(rc *readCtx, plan transaction.TicketPlan) wire.Value {
	selected := make([]string, 0)
	for _, entry := range plan.Entries {
		if entry.State == "SELECTED" {
			selected = append(selected, entry.Ticket.TicketID.Raw)
		}
	}
	o := wire.NewObject()
	o.Set("profile", wire.String("taskman-plan-selected/0"))
	o.Set("planningProfile", wire.String("taskman-priority-first/0"))
	o.Set("queueId", wire.String(rc.store.Queue.QueueID.Raw))
	o.Set("selectedTicketIds", wire.Strings(selected))
	o.Set("selectedTotal", wire.String(string(wire.CountOf(int64(len(selected))))))
	o.Set("complete", wire.Bool(true))
	o.Set("mutationAuthority", wire.Bool(false))
	return wire.ObjectValue(o)
}

// planInput audits the reservation set and every attempt record against the
// outer snapshot. It returns the reservation set's digest for the plan. A
// head generation of zero means no attempt was ever admitted, so the set is
// the empty one init wrote.
func planInput(rc *readCtx) (transaction.PlanInput, wire.Digest, error) {
	in := transaction.PlanInput{Queue: rc.store.Queue, Policy: rc.store.Policy, Tickets: rc.store.Inventory, Barrier: rc.snap.Barrier != nil, Attempts: map[string]*snapshot.Attempt{}}
	if len(rc.store.Policy.Pools) > 0 {
		proofPools, e := auditState(rc, "pools.json")
		if e != nil {
			return in, "", e
		}
		// The audited state proves pools.json absent when no member was
		// ever occupied; that is the empty pool state a claim reads, so
		// the default plan observes every member free (CAL-V0-078).
		in.Pools = &snapshot.PoolState{QueueID: rc.snap.Head.QueueID, Entries: []snapshot.PoolEntry{}}
		if raw := proofPools.Records["pools.json"].Raw; len(raw) > 0 {
			in.Pools, e = snapshot.DecodePools(raw)
			if e != nil {
				return in, "", e
			}
		}
	}
	if rc.snap.Head.Generation.Uint64() == 0 {
		in.Reservations = &snapshot.ReservationSet{QueueID: rc.snap.Head.QueueID, Entries: []snapshot.ReservationEntry{}}
		raw, err := in.Reservations.Encode()
		return in, wire.Sum(raw), err
	}
	proof, err := auditState(rc, "reservations.json")
	if err != nil {
		return in, "", err
	}
	paths := attemptPaths(proof)
	raw := proof.Records["reservations.json"].Raw
	if in.Reservations, err = snapshot.DecodeReservations(raw); err != nil {
		return in, "", err
	}
	for _, p := range paths {
		a, err := snapshot.DecodeAttempt(proof.Records[p].Raw)
		if err != nil {
			return in, "", err
		}
		in.Attempts[a.AttemptID] = a
	}
	return in, wire.Sum(raw), nil
}

// planValue renders a taskman-plan/0 object (TCP-00 §4.3).
func planValue(rc *readCtx, reservations wire.Digest, plan transaction.TicketPlan) (wire.Value, error) {
	entries := make([]wire.Value, 0, len(plan.Entries))
	for _, e := range plan.Entries {
		v, err := planEntryValue(e)
		if err != nil {
			return wire.Value{}, err
		}
		entries = append(entries, v)
	}
	capacity := wire.NewObject()
	capacity.Set("maxActiveAttempts", wire.String(string(plan.MaxActiveAttempts)))
	capacity.Set("availableWorkers", wire.String(string(plan.AvailableWorkers)))
	o := wire.NewObject()
	o.Set("profile", wire.String("taskman-plan/0"))
	o.Set("planningProfile", wire.String("taskman-priority-first/0"))
	o.Set("queueId", wire.String(rc.store.Queue.QueueID.Raw))
	o.Set("policySha256", wire.String(string(rc.store.Policy.PolicySha256())))
	o.Set("headSeq", wire.String(string(rc.snap.Head.LastSeq)))
	o.Set("intentTreeSha256", wire.String(string(rc.snap.IntentTree)))
	o.Set("reservationSetSha256", wire.String(string(reservations)))
	o.Set("capacity", wire.ObjectValue(capacity))
	o.Set("entries", wire.Array(entries...))
	if plan.Pools != nil {
		o.Set("resourceDeferred", resourceDeferredValue(plan.Pools))
	}
	o.Set("mutationAuthority", wire.Bool(false))
	return wire.ObjectValue(o), nil
}

// resourceDeferredValue is the default plan's additive per-pool summary
// (CAL-V0-078), present only when the policy declares pools and no --pool
// was requested: free eligible members (null when member state is
// NOT_OBSERVED), and the entries requiring each pool that were selected or
// deferred for want of a free member.
func resourceDeferredValue(pools []transaction.PoolSelection) wire.Value {
	rows := make([]wire.Value, 0, len(pools))
	for _, p := range pools {
		availability, free := "OBSERVED", wire.String(string(wire.CountOf(int64(p.Free))))
		if !p.Observed {
			availability, free = "NOT_OBSERVED", wire.Null()
		}
		o := wire.NewObject()
		o.Set("poolId", wire.String(p.PoolID))
		o.Set("availability", wire.String(availability))
		o.Set("freeEligibleMembers", free)
		o.Set("selected", wire.String(string(wire.CountOf(int64(p.Selected)))))
		o.Set("deferred", wire.String(string(wire.CountOf(int64(p.Deferred)))))
		rows = append(rows, wire.ObjectValue(o))
	}
	return wire.Array(rows...)
}

func planEntryValue(e transaction.PlanEntry) (wire.Value, error) {
	resources, err := snapshot.ResourcesValue("resources", e.Resources)
	if err != nil {
		return wire.Value{}, err
	}
	o := wire.NewObject()
	o.Set("ticketId", wire.String(e.Ticket.TicketID.Raw))
	o.Set("ticketRevision", wire.String(string(e.Ticket.AcceptanceRevision)))
	o.Set("retries", e.Retries)
	o.Set("resources", resources)
	o.Set("closureComplete", wire.Bool(e.ClosureComplete))
	o.Set("state", wire.String(e.State))
	o.Set("reason", wire.String(e.Reason))
	o.Set("deferredSinceSeq", wire.Null())
	o.Set("blockers", wire.Strings(e.Blockers))
	return wire.ObjectValue(o), nil
}
