package transaction

import (
	"bytes"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// loadReservations decodes the reservation set and checks that it is the
// canonical encoding bound in the inventory, and that every entry is an
// ACTIVE external-agent claim with no capacity or workers whose attempt
// record exists.
func loadReservations(r Request, in Input) (*snapshot.ReservationSet, error) {
	set, e := snapshot.DecodeReservations(in.Reservations)
	if e != nil {
		return nil, e
	}
	raw, e := set.Encode()
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(raw, in.Reservations) || set.QueueID.Raw != r.QueueID || !in.Inventory.matches("reservations.json", in.Reservations) {
		return nil, malformed("reservation set binding")
	}
	for _, en := range set.Entries {
		if en.State != "ACTIVE" || len(en.CapacityUses) != 0 || en.Workers != "0" {
			return nil, malformed("reservation entry outside the external-agent subset")
		}
		if _, ok := in.Inventory.files[attemptPath(en.AttemptID)]; !ok {
			return nil, malformed("reservation entry without an attempt record")
		}
	}
	return set, nil
}

func decodeAttempt(in Input, raw []byte) (*snapshot.Attempt, error) {
	a, e := snapshot.DecodeAttempt(raw)
	if e != nil {
		return nil, e
	}
	again, e := a.Encode()
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(again, raw) || !in.Inventory.matches(attemptPath(a.AttemptID), raw) {
		return nil, malformed("attempt record binding")
	}
	if a.RuntimeID != snapshot.RuntimeExternalAgent {
		return nil, malformed("attempt runtime outside the external-agent subset")
	}
	return a, nil
}

// loadAttempts decodes every attempt record and checks that the live ones
// are exactly the reservation entries, each with its generation, ticket
// revision and resources.
func loadAttempts(in Input, set *snapshot.ReservationSet) (map[string]*snapshot.Attempt, error) {
	out := map[string]*snapshot.Attempt{}
	for _, raw := range in.Attempts {
		a, e := decodeAttempt(in, raw)
		if e != nil {
			return nil, e
		}
		if out[a.AttemptID] != nil {
			return nil, malformed("duplicate attempt record")
		}
		out[a.AttemptID] = a
	}
	if countPrefix(in.Inventory, "attempts/") != len(out) {
		return nil, malformed("incomplete attempt inventory")
	}
	live := 0
	for _, a := range out {
		if a.Live() {
			live++
		}
	}
	if live != len(set.Entries) {
		return nil, malformed("live attempts differ from reservation entries")
	}
	for _, en := range set.Entries {
		a := out[en.AttemptID]
		if a == nil || !a.Live() || a.Generation != en.Generation || a.TicketID != en.TicketID || a.TicketRevision != en.TicketRevision || !sameResources(a.Scope.Resources, en.Resources) {
			return nil, malformed("reservation entry differs from its attempt")
		}
	}
	return out, nil
}

func countPrefix(inv *Inventory, prefix string) int {
	n := 0
	for path := range inv.files {
		if strings.HasPrefix(path, prefix) {
			n++
		}
	}
	return n
}
