package ticket

// Obligation mirrors (TOL-V0-120..123, GitHub #1214): an entry declared
// satisfied by an obligation on another ticket. satisfiedBy is an optional
// OBLIGATIONS_SET change member and an optional entry member, so a ledger
// that never records one keeps its exact earlier bytes. The fold records the
// declaration only; the mirrored state is derived at read time.
import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ObligationMirrorSelfDetail prefixes the refusal of a satisfiedBy that
// names the ledger's own ticket (TOL-V0-120).
const ObligationMirrorSelfDetail = "OBLIGATION_MIRROR_SELF:"

// ObligationMirrorRef names the owner obligation of a mirror: obligation ID
// in the ledger of ticket TicketID.
type ObligationMirrorRef struct {
	TicketID wire.TicketID
	ID       string
}

func (m ObligationMirrorRef) Value() wire.Value {
	return noteObject("ticketId", wire.String(m.TicketID.Raw), "id", wire.String(m.ID))
}

func readObligationMirrorRef(r *wire.Reader) *ObligationMirrorRef {
	r.Closed("ticketId", "id")
	m := &ObligationMirrorRef{TicketID: r.Field("ticketId").TicketID(), ID: ReadObligationID(r.Field("id"))}
	if r.Err() != nil {
		return nil
	}
	return m
}

// ObligationMirrorChange is the satisfiedBy member of one OBLIGATIONS_SET
// change: Owner sets the mirror, a nil Owner (the member present as null)
// clears it.
type ObligationMirrorChange struct {
	Owner *ObligationMirrorRef
}

func (c ObligationMirrorChange) Value() wire.Value {
	if c.Owner == nil {
		return wire.Null()
	}
	return c.Owner.Value()
}

func readObligationMirrorChange(r *wire.Reader) *ObligationMirrorChange {
	if r.IsNull() {
		return &ObligationMirrorChange{}
	}
	m := readObligationMirrorRef(r)
	if m == nil {
		return nil
	}
	return &ObligationMirrorChange{Owner: m}
}

// applyMirrorChange records or clears e's mirror; a mirror of the ledger's
// own ticket is refused (TOL-V0-120).
func applyMirrorChange(e *ObligationEntry, c *ObligationMirrorChange, q *ObligationRequest) error {
	if c.Owner == nil {
		e.SatisfiedBy = nil
		return nil
	}
	if c.Owner.TicketID.Raw == q.TicketID.Raw {
		return wire.Errorf(wire.CodeMalformed, "/request/payload/changes", "%s obligation %s cannot be satisfied by its own ticket %s", ObligationMirrorSelfDetail, e.ID, q.TicketID.Raw)
	}
	owner := *c.Owner
	e.SatisfiedBy = &owner
	return nil
}

// countUnblocks is the number of entries BLOCKED in before that are no
// longer BLOCKED in after (TOL-V0-124).
func countUnblocks(before, after *ObligationLedger) int64 {
	if before == nil {
		return 0
	}
	var n int64
	for _, e := range before.Entries {
		if e.State != ObligationBlocked {
			continue
		}
		if x := after.Entry(e.ID); x != nil && x.State != ObligationBlocked {
			n++
		}
	}
	return n
}
