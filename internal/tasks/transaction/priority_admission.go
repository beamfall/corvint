package transaction

import (
	"strconv"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// priorityWaiting lists, in plan order, the tickets that compete with rec
// for pool under priority-yield admission (CAL-V0-101): OPEN tickets ahead
// of rec by (priority, order, ticketId) that require pool and have no claim
// blocker, which includes a live attempt. A competitor whose only blockers
// are unknown is listed in unobserved instead and never counts as waiting.
// The read is pure: it consults the plan input and runs no probe.
func priorityWaiting(in PlanInput, rec *ticket.Record, pool string) (waiting, unobserved []string) {
	for _, t := range planTickets(in.Tickets) {
		if !planLess(t, rec) {
			break
		}
		if t.Status != ticket.StatusOpen || t.RequiresPool != pool {
			continue
		}
		_, known, unknown := claimBlockerObservations(in, t)
		switch {
		case len(known) > 0:
		case len(unknown) > 0:
			unobserved = append(unobserved, t.TicketID.Raw)
		default:
			waiting = append(waiting, t.TicketID.Raw)
		}
	}
	return waiting, unobserved
}

// priorityYield is the ticket a claim on pool yields to, or "" when it need
// not yield: the pool opts into priority admission, it has a free eligible
// member, and the waiting competitors are at least as many as those free
// members. It names the first, highest-priority competitor.
func priorityYield(policy *intent.Policy, pool string, free int, waiting []string) string {
	p := policy.Pool(pool)
	if p == nil || !p.PriorityAdmission || free <= 0 || len(waiting) < free {
		return ""
	}
	return waiting[0]
}

// priorityYieldDetail is the refusal detail of a claim yielded to to.
func priorityYieldDetail(pool string, free int, waiting []string, to string) string {
	return "pool " + pool + " priority admission: " + strconv.Itoa(len(waiting)) + " higher-priority waiting ticket(s) for " + strconv.Itoa(free) + " free eligible member(s); yields to " + to
}

// yieldRefusal refuses an explicit pooled claim that would take a member a
// higher-priority waiting ticket needs (CAL-V0-101). It runs after the
// live-attempt checks and before the collision, capacity, health and
// allocation steps, so a yielded claim prepares no member. Without
// --pool, or on a pool that does not opt in, it never refuses.
func (c leaseContext) yieldRefusal(rec *ticket.Record) *leaseOutcome {
	if c.l.Pool == "" {
		return nil
	}
	if p := c.st.policy.Pool(c.l.Pool); p == nil || !p.PriorityAdmission {
		return nil
	}
	in := PlanInput{Pool: c.l.Pool, Stage: c.l.Stage, ExcludeMembers: c.l.ExcludeMembers, Pools: c.st.pools, Prepared: c.in.LeaseFacts.Pool.AllocationID, Queue: c.st.queue, Policy: c.st.policy, Tickets: c.st.tickets, Reservations: c.st.reservations, Attempts: c.st.attempts}
	free := poolSlots(in)
	waiting, _ := priorityWaiting(in, rec, c.l.Pool)
	to := priorityYield(c.st.policy, c.l.Pool, free, waiting)
	if to == "" {
		return nil
	}
	out := c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, priorityYieldDetail(c.l.Pool, free, waiting, to))
	return &out
}
