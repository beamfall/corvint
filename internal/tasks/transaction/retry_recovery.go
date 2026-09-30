package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// openRetryRecovery activates full attempt validation only for the new OPEN
// recovery path. Completed ticket reopen retains its previous input contract.
func openRetryRecovery(r Request, st inputState) bool {
	if r.Operation != Mutate {
		return false
	}
	env, err := mutation.Decode(r.Envelope)
	if err != nil || env.Operation != mutation.OpReopen || env.TargetID == nil {
		return false
	}
	rec, _ := st.tickets.Get(env.TargetID.Raw)
	return rec != nil && rec.Status == ticket.StatusOpen
}

func retryRecovery(st inputState, env *mutation.Envelope) *mutation.RetryRecovery {
	if env.Operation != mutation.OpReopen || env.TargetID == nil {
		return nil
	}
	rec, _ := st.tickets.Get(env.TargetID.Raw)
	if rec == nil || rec.Status != ticket.StatusOpen {
		return nil
	}
	observation := &mutation.RetryRecovery{TicketID: rec.TicketID.Raw, AcceptanceRevision: rec.AcceptanceRevision, State: ticket.Unsatisfied}
	if st.attempts == nil || st.reservations == nil {
		return observation
	}
	for _, en := range st.reservations.Entries {
		if en.TicketID == rec.TicketID {
			return observation
		}
	}
	var last *snapshot.Attempt
	generations := map[uint64]bool{}
	for _, a := range st.attempts {
		if a.TicketID != rec.TicketID {
			continue
		}
		// Any unsafe older attempt still blocks recovery. A newer terminal row
		// cannot hide a live process, pending effect or ambiguous generation.
		generation := a.Generation.Uint64()
		if generations[generation] || a.Live() || len(a.PendingEffects) != 0 {
			return observation
		}
		generations[generation] = true
		switch a.RuntimeID {
		case snapshot.RuntimeExternalAgent:
			if a.Quiescence != "FENCED" {
				return observation
			}
		case snapshot.SupervisedProfile:
			if a.Quiescence != "PROVED" || a.Supervision == nil || a.Supervision.Worker {
				return observation
			}
		default:
			return observation
		}
		if last == nil || generation > last.Generation.Uint64() {
			last = a
		}
	}
	if last == nil || last.TicketRevision != rec.AcceptanceRevision || last.RetryCount.Int() < MaxRetries || (last.Phase != "FAILED" && last.Phase != "CANCELLED") {
		return observation
	}
	observation.State, observation.Phase = ticket.Satisfied, last.Phase
	return observation
}
