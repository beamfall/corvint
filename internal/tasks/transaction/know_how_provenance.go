package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
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
// Holder is the lease holder while that lease is unexpired at now; as for
// every lease command, a supervised attempt's lease does not expire by time
// (KHN-V0-022).
func knowHowLedger(attempts map[string]*snapshot.Attempt, now wire.Timestamp) mutation.AttemptLedger {
	if attempts == nil {
		return nil
	}
	out := make(mutation.AttemptLedger, len(attempts))
	for id, a := range attempts {
		p := mutation.AttemptProvenance{TicketID: a.TicketID, Generation: a.Generation, Live: a.Live()}
		if a.Lease != nil && !expired(a, now) {
			p.Holder = a.Lease.Holder
		}
		for _, g := range a.PriorGenerations {
			p.Prior = append(p.Prior, g.Generation)
		}
		out[id] = p
	}
	return out
}
