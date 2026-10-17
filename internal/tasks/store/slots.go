package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SlotObservation is the attempt's current place on one capacity-slot
// resource after a slot command (PSR-V0-038): HELD, WAITING with its 1-based
// position in wait order, or NONE. It is a fresh audited read after a
// committed command; a replay carries none (PSR-V0-043).
type SlotObservation struct {
	SlotsID   string
	State     string
	Count     wire.Count
	Exclusive bool
	Class     string
	Position  int
}

// slotObservation reads the audited state after a slot command.
func slotObservation(ctx context.Context, repo *intent.Repository, l transaction.LeaseRequest, report *Report) error {
	proof, err := readLeaseProof(ctx, repo)
	if err != nil {
		return err
	}
	report.Slot = &SlotObservation{SlotsID: l.Slots, State: "NONE"}
	if a, ok := lockedAttempt(proof, l.AttemptID); ok {
		report.Ticket = a.TicketID.Raw
	}
	raw := proof.Records["pools.json"].Raw
	if len(raw) == 0 {
		return nil
	}
	policy, err := intent.DecodePolicy(proof.Records["intent/policy.json"].Raw)
	if err != nil {
		return err
	}
	state, err := snapshot.DecodePools(raw)
	if err != nil {
		return err
	}
	live := transaction.SlotLive(policy, state.Slots, func(id string) *snapshot.Attempt {
		a, _ := lockedAttempt(proof, id)
		return a
	})
	SlotObserve(report.Slot, l.AttemptID, live)
	return nil
}

// SlotObserve fills o from the live entries of o.SlotsID.
func SlotObserve(o *SlotObservation, attemptID string, live []snapshot.SlotEntry) {
	waiters := []snapshot.SlotEntry{}
	for _, en := range live {
		if en.SlotsID == o.SlotsID && en.State == snapshot.SlotWaiting {
			waiters = append(waiters, en)
		}
	}
	waiters = transaction.SlotWaitOrder(waiters)
	for _, en := range live {
		if en.SlotsID != o.SlotsID || en.AttemptID != attemptID {
			continue
		}
		o.State, o.Count, o.Exclusive, o.Class = en.State, en.Count, en.Exclusive, en.Class
		for i, w := range waiters {
			if w.AttemptID == attemptID {
				o.Position = i + 1
			}
		}
	}
}
