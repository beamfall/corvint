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
	observation := &mutation.RetryRecovery{TicketID: rec.TicketID.Raw, AcceptanceRevision: rec.AcceptanceRevision, State: ticket.Unsatisfied, Reason: "RECOVERY_INVENTORY_NOT_OBSERVED"}
	if st.attempts == nil || st.reservations == nil {
		return observation
	}
	for _, en := range st.reservations.Entries {
		if en.TicketID == rec.TicketID {
			observation.Reason = "RESERVATION_PRESENT"
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
		if generations[generation] {
			observation.Reason = "AMBIGUOUS_GENERATION"
			return observation
		}
		if a.Live() {
			observation.Reason = "ATTEMPT_LIVE"
			return observation
		}
		if len(a.PendingEffects) != 0 {
			observation.Reason = "PENDING_EFFECTS"
			return observation
		}
		generations[generation] = true
		switch a.RuntimeID {
		case snapshot.RuntimeExternalAgent:
			if a.Quiescence != "FENCED" {
				observation.Reason = "QUIESCENCE_UNPROVED"
				return observation
			}
		case snapshot.SupervisedProfile:
			if a.Quiescence != "PROVED" || a.Supervision == nil || a.Supervision.Worker {
				observation.Reason = "QUIESCENCE_UNPROVED"
				return observation
			}
		default:
			observation.Reason = "RUNTIME_UNKNOWN"
			return observation
		}
		if last == nil || generation > last.Generation.Uint64() {
			last = a
		}
	}
	switch {
	case last == nil:
		observation.Reason = "NO_PRIOR_ATTEMPT"
	case last.TicketRevision != rec.AcceptanceRevision:
		observation.Reason = "ACCEPTANCE_REVISION_MISMATCH"
	case !exhaustedAttempt(last, st.policy.AdmissionsPerRevision.Int()) && LoopHoldOf(st.attempts, rec, st.policy) == nil:
		// CAL-V0-103: a LOOP_DETECTED hold is the other automation stop the
		// owner reopen acknowledges.
		observation.Reason = "RETRY_BUDGET_NOT_EXHAUSTED"
	case last.Phase != "FAILED" && last.Phase != "CANCELLED":
		observation.Reason = "LATEST_ATTEMPT_NOT_FAILED_OR_CANCELLED"
	default:
		observation.State, observation.Phase, observation.Reason = ticket.Satisfied, last.Phase, "RECOVERY_ALLOWED"
	}
	return observation
}
