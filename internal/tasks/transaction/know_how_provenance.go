package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// knowHowAttemptMutation reports a Mutate KNOWHOW_ADD/RECONFIRM request naming an
// attempt or generation, whose provenance is checked against the complete
// audited attempt inventory (KHN-V0-008).
func knowHowAttemptMutation(r Request) bool {
	if r.Operation != Mutate {
		return false
	}
	env, err := mutation.Decode(r.Envelope)
	return err == nil && mutation.KnowHowNamesAttempt(env)
}

// knowHowLedger projects the audited attempt records onto the provenance
// ledger. A nil map (attempts not loaded) stays nil, so the check refuses.
func knowHowLedger(attempts map[string]*snapshot.Attempt) mutation.AttemptLedger {
	if attempts == nil {
		return nil
	}
	out := make(mutation.AttemptLedger, len(attempts))
	for id, a := range attempts {
		p := mutation.AttemptProvenance{TicketID: a.TicketID, Generation: a.Generation, Live: a.Live()}
		for _, g := range a.PriorGenerations {
			p.Prior = append(p.Prior, g.Generation)
		}
		out[id] = p
	}
	return out
}
