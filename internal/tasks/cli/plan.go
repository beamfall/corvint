package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
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
	if len(args) != 0 {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "plan preview takes no argument"))
	}
	var item wire.Value
	rc, err := withStore(env, func(rc *readCtx) error {
		in, digest, err := planInput(rc)
		if err != nil {
			return err
		}
		item, err = planValue(rc, digest, transaction.PriorityFirst(in))
		return err
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	return res
}

// planInput audits the reservation set and every attempt record against the
// outer snapshot. It returns the reservation set's digest for the plan. A
// head generation of zero means no attempt was ever admitted, so the set is
// the empty one init wrote.
func planInput(rc *readCtx) (transaction.PlanInput, wire.Digest, error) {
	in := transaction.PlanInput{Queue: rc.store.Queue, Policy: rc.store.Policy, Tickets: rc.store.Inventory, Barrier: rc.snap.Barrier != nil, Attempts: map[string]*snapshot.Attempt{}}
	if rc.snap.Head.Generation.Uint64() == 0 {
		in.Reservations = &snapshot.ReservationSet{QueueID: rc.snap.Head.QueueID, Entries: []snapshot.ReservationEntry{}}
		raw, err := in.Reservations.Encode()
		return in, wire.Sum(raw), err
	}
	paths, err := store.AttemptPaths(rc.repo)
	if err != nil {
		return in, "", err
	}
	proof, err := auditState(rc, append([]string{"reservations.json"}, paths...)...)
	if err != nil {
		return in, "", err
	}
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
	o.Set("mutationAuthority", wire.Bool(false))
	return wire.ObjectValue(o), nil
}

func planEntryValue(e transaction.PlanEntry) (wire.Value, error) {
	resources, err := snapshot.ResourcesValue("resources", e.Resources)
	if err != nil {
		return wire.Value{}, err
	}
	o := wire.NewObject()
	o.Set("ticketId", wire.String(e.Ticket.TicketID.Raw))
	o.Set("ticketRevision", wire.String(string(e.Ticket.AcceptanceRevision)))
	o.Set("resources", resources)
	o.Set("closureComplete", wire.Bool(e.ClosureComplete))
	o.Set("state", wire.String(e.State))
	o.Set("reason", wire.String(e.Reason))
	o.Set("deferredSinceSeq", wire.Null())
	o.Set("blockers", wire.Strings(e.Blockers))
	return wire.ObjectValue(o), nil
}
