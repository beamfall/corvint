package transaction

import "github.com/Beamfall/corvint/internal/tasks/ticket"

// NextClaimTicket selects the same conservative candidate as CLAIM_NEXT so
// the store can derive only that ticket's scope outside the pure model.
// The actual claim independently selects again and binds derived facts to
// that ticket before checking collisions.
func NextClaimTicket(queueID string, in Input, choice ...LeaseRequest) (*ticket.Record, error) {
	state, err := validateInput(Request{Operation: Lease, QueueID: queueID}, in)
	if err != nil {
		return nil, err
	}
	l := LeaseRequest{}
	if len(choice) > 0 {
		l = choice[0]
	}
	if err := CheckPoolExclusions(l.Pool, l.ExcludeMembers, state.policy); err != nil {
		return nil, err
	}
	plan := PriorityFirst(claimNextInput(&l, in, state))
	chosen := plan.ClaimNext(l.Pool)
	if chosen == nil {
		return nil, nil
	}
	return chosen.Ticket, nil
}

// claimNextInput is the one plan input both NextClaimTicket and CLAIM_NEXT
// select from, so the store derives scope for the ticket the claim takes.
// Now is the transaction's recorded time, at which a --pool plan honours the
// live waitlist (PSR-V0-132).
func claimNextInput(l *LeaseRequest, in Input, st inputState) PlanInput {
	return PlanInput{Pool: l.Pool, Stage: l.Stage, ExcludeMembers: l.ExcludeMembers, ExcludeAuthors: l.ExcludeAuthors, Pools: st.pools, Prepared: in.LeaseFacts.Pool.AllocationID, Queue: st.queue, Policy: st.policy, Tickets: st.tickets, Barrier: st.barrier != nil, Reservations: st.reservations, Attempts: st.attempts, Now: in.RecordedAt}
}
