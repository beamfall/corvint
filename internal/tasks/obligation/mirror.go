package obligation

// Obligation mirror resolution (TOL-V0-121..123, GitHub #1214). A mirror is
// an entry whose satisfiedBy names an owner obligation on another ticket.
// Resolution is pure and one hop: the caller folds the owner ledgers and
// this reads each owner entry's own recorded state, never the owner's own
// mirrors, so a cycle cannot loop and a chain stays visible.
import (
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Mirror reasons: why a mirror whose own state is pending is not decided.
const (
	MirrorOwnerTicketMissing     = "OWNER_TICKET_MISSING"
	MirrorOwnerLedgerMissing     = "OWNER_LEDGER_MISSING"
	MirrorOwnerLedgerUnknown     = "OWNER_LEDGER_UNKNOWN"
	MirrorOwnerObligationMissing = "OWNER_OBLIGATION_MISSING"
	MirrorOwnerUndecided         = "OWNER_UNDECIDED"

	// Finding codes of the worklist and queue status (TOL-V0-122).
	MirrorFindingDecided    = "MIRROR-DECIDED-ELSEWHERE"
	MirrorFindingUnresolved = "MIRROR-OWNER-UNRESOLVED"
)

// MirrorOwner is one folded owner ticket as the caller observed it: a nil
// Ledger carries the Missing reason (OWNER_TICKET_MISSING,
// OWNER_LEDGER_MISSING or OWNER_LEDGER_UNKNOWN).
type MirrorOwner struct {
	Ledger  *ticket.ObligationLedger
	Missing string
}

// MirrorLookup returns the owner ticket named by a mirror.
type MirrorLookup func(ticketID string) MirrorOwner

// MirrorResolution is one mirror entry of a ledger. Decided is true when
// the entry's own state is pending and the owner entry's recorded state is
// terminal (WITNESSED or DEFERRED); State is then the owner's state and
// Evidence the owner's witness event digest (nil for DEFERRED). Otherwise
// State is the entry's own state, and Reason says why a pending mirror is
// still open ("" when the entry's own state is already terminal).
type MirrorResolution struct {
	ID         string
	Owner      ticket.ObligationMirrorRef
	LocalState string
	OwnerState string
	Decided    bool
	State      string
	Evidence   *wire.Digest
	Reason     string
}

// ResolveMirrors resolves every mirror entry of l in entry order; nil when l
// has none.
func ResolveMirrors(l *ticket.ObligationLedger, lookup MirrorLookup) []MirrorResolution {
	var out []MirrorResolution
	if l == nil {
		return nil
	}
	for _, e := range l.Entries {
		if e.SatisfiedBy == nil {
			continue
		}
		r := MirrorResolution{ID: e.ID, Owner: *e.SatisfiedBy, LocalState: e.State, State: e.State}
		owner := lookup(e.SatisfiedBy.TicketID.Raw)
		var oe *ticket.ObligationEntry
		if owner.Ledger != nil {
			oe = owner.Ledger.Entry(e.SatisfiedBy.ID)
		}
		if oe != nil {
			r.OwnerState = oe.State
		}
		switch {
		case !ticket.ObligationPending(e.State):
		case owner.Ledger == nil:
			r.Reason = owner.Missing
		case oe == nil:
			r.Reason = MirrorOwnerObligationMissing
		case ticket.ObligationPending(oe.State):
			r.Reason = MirrorOwnerUndecided
		default:
			r.Decided, r.State = true, oe.State
			if oe.Evidence != nil {
				d := oe.Evidence.EventSha256
				r.Evidence = &d
			}
		}
		out = append(out, r)
	}
	return out
}

// MirrorsCompletable reports a ledger with at least one decided mirror whose
// every pending entry is a decided mirror (TOL-V0-123).
func MirrorsCompletable(l *ticket.ObligationLedger, rs []MirrorResolution) bool {
	decided := map[string]bool{}
	for _, r := range rs {
		if r.Decided {
			decided[r.ID] = true
		}
	}
	if l == nil || len(decided) == 0 {
		return false
	}
	for _, e := range l.Entries {
		if ticket.ObligationPending(e.State) && !decided[e.ID] {
			return false
		}
	}
	return true
}

// MirrorValue is one resolution as the reads show it: {id, satisfiedBy,
// localState, state, derived, source?, reason?}. source, present only when
// derived, is {ticketId, id, state, eventSha256}; a derived state carries
// no review of its own.
func MirrorValue(r MirrorResolution) wire.Value {
	o := wire.NewObject().Set("id", wire.String(r.ID)).Set("satisfiedBy", r.Owner.Value()).
		Set("localState", wire.String(r.LocalState)).Set("state", wire.String(r.State)).Set("derived", wire.Bool(r.Decided))
	if r.Decided {
		ev := wire.Null()
		if r.Evidence != nil {
			ev = wire.String(string(*r.Evidence))
		}
		o.Set("source", wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(r.Owner.TicketID.Raw)).Set("id", wire.String(r.Owner.ID)).
			Set("state", wire.String(r.OwnerState)).Set("eventSha256", ev)))
	}
	if r.Reason != "" {
		o.Set("reason", wire.String(r.Reason))
	}
	return wire.ObjectValue(o)
}

// MirrorFindings are a ticket's mirror findings: one MIRROR-DECIDED-ELSEWHERE
// listing its decided mirrors and one MIRROR-OWNER-UNRESOLVED listing its
// pending mirrors whose owner ticket, ledger or obligation is missing or
// unreadable. An undecided owner is no finding.
func MirrorFindings(ticketID string, rs []MirrorResolution, completable bool) []wire.Value {
	var decided, unresolved []wire.Value
	for _, r := range rs {
		switch {
		case r.Decided:
			decided = append(decided, MirrorValue(r))
		case r.Reason != "" && r.Reason != MirrorOwnerUndecided:
			unresolved = append(unresolved, MirrorValue(r))
		}
	}
	var out []wire.Value
	if len(decided) > 0 {
		out = append(out, wire.ObjectValue(wire.NewObject().Set("code", wire.String(MirrorFindingDecided)).Set("ticketId", wire.String(ticketID)).
			Set("completable", wire.Bool(completable)).Set("items", wire.Array(decided...))))
	}
	if len(unresolved) > 0 {
		out = append(out, wire.ObjectValue(wire.NewObject().Set("code", wire.String(MirrorFindingUnresolved)).Set("ticketId", wire.String(ticketID)).
			Set("items", wire.Array(unresolved...))))
	}
	return out
}
