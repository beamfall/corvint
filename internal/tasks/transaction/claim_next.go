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
	plan := PriorityFirst(PlanInput{Pool: l.Pool, Stage: l.Stage, ExcludeMembers: l.ExcludeMembers, Pools: state.pools, Prepared: in.LeaseFacts.Pool.AllocationID, Queue: state.queue, Policy: state.policy, Tickets: state.tickets, Barrier: state.barrier != nil, Reservations: state.reservations, Attempts: state.attempts})
	chosen := plan.ClaimNext(l.Pool)
	if chosen == nil {
		return nil, nil
	}
	return chosen.Ticket, nil
}
