package transaction

import "github.com/Beamfall/corvint/internal/tasks/ticket"

// NextClaimTicket selects the same conservative candidate as CLAIM_NEXT so
// the store can derive only that ticket's scope outside the pure model.
// The actual claim independently selects again and binds derived facts to
// that ticket before checking collisions.
func NextClaimTicket(queueID string, in Input) (*ticket.Record, error) {
	state, err := validateInput(Request{Operation: Lease, QueueID: queueID}, in)
	if err != nil {
		return nil, err
	}
	plan := PriorityFirst(PlanInput{Queue: state.queue, Policy: state.policy, Tickets: state.tickets, Barrier: state.barrier != nil, Reservations: state.reservations, Attempts: state.attempts})
	chosen := plan.Selected()
	if chosen == nil {
		return nil, nil
	}
	return chosen.Ticket, nil
}
