package ticket

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Observation is a three-valued fact: unknown facts never become
// satisfied (AGENTS invariant 5).
type Observation string

const (
	Satisfied   Observation = "SATISFIED"
	Unsatisfied Observation = "UNSATISFIED"
	NotObserved Observation = "NOT_OBSERVED"
)

// GateOracle answers whether a GATE_PASSED obligation is satisfied at the
// dependency's current acceptanceRevision (§3.1). The journal-backed
// implementation arrives with TCP-02/TCP-05; TCP-01 has none.
type GateOracle interface {
	GatePassed(dep wire.TicketID, gateID string, acceptanceRevision wire.Count) Observation
}

// AttemptOracle answers whether a ticket has a live attempt (§3.2).
type AttemptOracle interface {
	LiveAttempt(ticketID string) Observation
}

// NoEvidence is the TCP-01 oracle: it observes nothing and therefore never
// satisfies anything.
type NoEvidence struct{}

// GatePassed always reports NOT_OBSERVED.
func (NoEvidence) GatePassed(wire.TicketID, string, wire.Count) Observation { return NotObserved }

// LiveAttempt always reports NOT_OBSERVED.
func (NoEvidence) LiveAttempt(string) Observation { return NotObserved }

// Context carries the queue and policy facts eligibility depends on.
type Context struct {
	CanonicalWriter string // queue.canonicalWriter
	SerialFallback  string // policy.serialFallback
	Gates           GateOracle
	Attempts        AttemptOracle
}

// Blocker is one reason a ticket is not eligible.
type Blocker struct {
	Code     string // §11 code
	TicketID string // related ticket, or ""
	Detail   string // human explanation; never queue prose
}

// Eligibility values of a view. BLOCKED is certain; UNKNOWN means every
// intent-level check passed but a fact this slice cannot observe (attempt
// liveness, gate evidence) is still NOT_OBSERVED, so no ELIGIBLE claim is
// made.
const (
	EligibilityBlocked = "BLOCKED"
	EligibilityUnknown = "UNKNOWN"
)

// View is the derived, read-only description of one ticket (TM-V0-008:
// why it is tracked, eligible or blocked, owner, priority, current attempt,
// gates, holds, next permitted action). Blockers are certain refusals;
// Unknowns are facts the reader could not observe. An unknown never becomes
// a blocker and never becomes satisfied: it keeps Eligibility at UNKNOWN.
type View struct {
	Record         *Record
	Tracked        string // NATIVE | IMPORT | SHADOW
	IntentChecks   string // PASSED | FAILED
	Eligibility    string // BLOCKED | UNKNOWN
	Blockers       []Blocker
	Unknowns       []Blocker
	CurrentAttempt Observation // NOT_OBSERVED in TCP-01
	GateResults    Observation // NOT_OBSERVED in TCP-01
	Publication    Observation // NOT_OBSERVED in TCP-01 (needs the journal post digest)
	NextAction     string
	Untrusted      bool // true when queue prose is embedded (title/body)
}

func (ctx Context) gates() GateOracle {
	if ctx.Gates == nil {
		return NoEvidence{}
	}
	return ctx.Gates
}

func (ctx Context) attempts() AttemptOracle {
	if ctx.Attempts == nil {
		return NoEvidence{}
	}
	return ctx.Attempts
}

// View derives the §3.2 eligibility for one ticket. It never writes.
func (inv *Inventory) View(id string, ctx Context) (View, bool) {
	rec, ok := inv.byID[id]
	if !ok {
		return View{}, false
	}
	v := View{Record: rec, CurrentAttempt: NotObserved, GateResults: NotObserved, Publication: NotObserved, Untrusted: true}
	switch {
	case rec.ShadowOverlay:
		v.Tracked = "SHADOW"
	case rec.Source.Kind == "IMPORT":
		v.Tracked = "IMPORT"
	default:
		v.Tracked = "NATIVE"
	}
	var blockers, unknowns []Blocker
	add := func(code, tid, detail string) {
		blockers = append(blockers, Blocker{Code: code, TicketID: tid, Detail: detail})
	}
	unknown := func(code, tid, detail string) {
		unknowns = append(unknowns, Blocker{Code: code, TicketID: tid, Detail: detail})
	}
	// §3.2: OPEN and not held.
	switch rec.Status {
	case StatusOpen:
	case StatusHeld:
		add(wire.CodeTicketHeld, "", "ticket is HELD by "+holdIDs(rec))
	default:
		add(wire.CodeTicketState, "", "status "+rec.Status+" is not OPEN")
	}
	// TM-V0-005 structure: missing dependencies and cycles.
	for _, p := range inv.Problems(id) {
		add(p.Code, p.TicketID, p.Detail)
	}
	// §3.1 dependency obligations at the dependency's current acceptanceRevision.
	for _, d := range rec.Dependencies {
		dep, ok := inv.byID[d.TicketID.Raw]
		if !ok {
			continue // already DEPENDENCY_MISSING
		}
		switch d.Obligation {
		case "COMPLETED":
			if !(dep.Status == StatusCompleted || (dep.Status == StatusArchived && dep.ArchivedFrom != nil && *dep.ArchivedFrom == StatusCompleted)) {
				add(wire.CodeDependencyUnsatisfied, dep.TicketID.Raw, "dependency "+dep.TicketID.Raw+" is "+dep.Status+", obligation COMPLETED")
			}
		case "GATE_PASSED":
			gate := ""
			if d.GateID != nil {
				gate = *d.GateID
			}
			switch ctx.gates().GatePassed(dep.TicketID, gate, dep.AcceptanceRevision) {
			case Satisfied:
			case Unsatisfied:
				add(wire.CodeDependencyUnsatisfied, dep.TicketID.Raw, "gate "+gate+" of "+dep.TicketID.Raw+" has no PASSED result at acceptanceRevision "+string(dep.AcceptanceRevision))
			default:
				unknown(wire.CodeDependencyUnsatisfied, dep.TicketID.Raw, "gate "+gate+" of "+dep.TicketID.Raw+": result NOT_OBSERVED (no journal evidence available to this reader)")
			}
		}
	}
	// §3.2 execution class and approvals at the current acceptanceRevision.
	switch rec.ExecutionClass {
	case "AUTONOMOUS":
	case "APPROVAL_REQUIRED":
		granted := false
		revoked := false
		for _, a := range rec.Approvals {
			if a.Operation == "RUN" && a.TargetRevision == rec.AcceptanceRevision {
				if a.Revoked {
					revoked = true
				} else {
					granted = true
				}
			}
		}
		if !granted {
			if revoked {
				add(wire.CodeApprovalRevoked, "", "the RUN grant at acceptanceRevision "+string(rec.AcceptanceRevision)+" is revoked")
			} else {
				add(wire.CodeApprovalMissing, "", "no RUN grant at acceptanceRevision "+string(rec.AcceptanceRevision))
			}
		}
	default:
		add(wire.CodeTicketState, "", "executionClass "+rec.ExecutionClass+" is never autonomously eligible")
	}
	// §3.2 effects.
	if rec.Effects.ExternalUnbounded {
		add(wire.CodeExternalUnbounded, "", "effects.externalUnbounded is true")
	}
	if rec.Effects.Coverage != "QUALIFIED" && ctx.SerialFallback != "WHOLE_REPOSITORY" {
		add(wire.CodeCoverageUnknown, "", "effects.coverage is "+rec.Effects.Coverage+" and policy serialFallback is not WHOLE_REPOSITORY")
	}
	// §3.2 imported records only under a NATIVE writer.
	if rec.Source.Kind == "IMPORT" && ctx.CanonicalWriter != "NATIVE" {
		add(wire.CodeCutoverMissing, "", "imported record while canonicalWriter is "+ctx.CanonicalWriter)
	}
	// §3.2 no live attempt.
	switch ctx.attempts().LiveAttempt(id) {
	case Satisfied: // a live attempt exists
		add(wire.CodeAttemptLive, "", "an attempt is live")
		v.CurrentAttempt = Satisfied
	case Unsatisfied:
		v.CurrentAttempt = Unsatisfied
	default:
		unknown(wire.CodeAttemptLive, "", "attempt liveness NOT_OBSERVED (no journal available to this reader)")
	}
	v.Blockers = blockers
	v.Unknowns = unknowns
	if len(blockers) == 0 {
		// Every intent-level check passed. Whatever remains is NOT_OBSERVED,
		// so the honest answer is UNKNOWN, never ELIGIBLE (invariant 5).
		v.IntentChecks = "PASSED"
		v.Eligibility = EligibilityUnknown
	} else {
		v.IntentChecks = "FAILED"
		v.Eligibility = EligibilityBlocked
	}
	v.NextAction = nextAction(rec, blockers)
	return v, true
}

func blockerValues(bs []Blocker) wire.Value {
	out := make([]wire.Value, 0, len(bs))
	for _, b := range bs {
		bo := wire.NewObject()
		bo.Set("code", wire.String(b.Code))
		if b.TicketID == "" {
			bo.Set("ticketId", wire.Null())
		} else {
			bo.Set("ticketId", wire.String(b.TicketID))
		}
		bo.Set("detail", wire.String(b.Detail))
		out = append(out, wire.ObjectValue(bo))
	}
	return wire.Array(out...)
}

func holdIDs(rec *Record) string {
	out := ""
	for i, h := range rec.Holds {
		if i > 0 {
			out += ", "
		}
		out += h.HoldID
	}
	return out
}

// nextAction names the next permitted operation as a literal verb label.
func nextAction(rec *Record, blockers []Blocker) string {
	switch rec.Status {
	case StatusDraft:
		return "refine"
	case StatusHeld:
		return "release-hold"
	case StatusCompleted:
		return "reopen"
	case StatusArchived:
		return "restore"
	}
	if len(blockers) == 0 {
		// Intent checks passed; admission itself needs the journal (TCP-02)
		// to observe attempts and gates, so the next action is that check.
		return "admit"
	}
	switch blockers[0].Code {
	case wire.CodeDependencyMissing, wire.CodeCycle:
		return "set-dependencies"
	case wire.CodeDependencyUnsatisfied:
		return "wait-dependency"
	case wire.CodeApprovalMissing, wire.CodeApprovalRevoked:
		return "grant-approval"
	case wire.CodeCoverageUnknown, wire.CodeExternalUnbounded:
		return "set-effects"
	case wire.CodeCutoverMissing:
		return "cutover"
	case wire.CodeAttemptLive:
		return "wait-attempt"
	}
	return "refine"
}

// Value renders the view as a read item. The record is embedded when
// includeRecord is set (`ticket show`); the compact form (`ticket list`)
// carries the derived facts and the record's identity, priority and owner.
// Title prose is included only as an escaped JSON string; it is queue data.
func (v View) Value(includeRecord bool) wire.Value {
	rec := v.Record
	o := wire.NewObject()
	o.Set("ticketId", wire.String(rec.TicketID.Raw))
	o.Set("revision", wire.String(string(rec.Revision)))
	o.Set("acceptanceRevision", wire.String(string(rec.AcceptanceRevision)))
	o.Set("status", wire.String(rec.Status))
	o.Set("archivedFrom", wire.StringOrNull(rec.ArchivedFrom))
	o.Set("kind", wire.String(rec.Kind))
	o.Set("priority", wire.String(rec.Priority))
	o.Set("order", wire.String(string(rec.Order)))
	o.Set("owner", wire.StringOrNull(rec.Owner))
	o.Set("milestone", wire.StringOrNull(rec.Milestone))
	o.Set("title", wire.String(rec.Title))
	o.Set("executionClass", wire.String(rec.ExecutionClass))
	o.Set("tracked", wire.String(v.Tracked))
	o.Set("intentChecks", wire.String(v.IntentChecks))
	o.Set("eligibility", wire.String(v.Eligibility))
	o.Set("blockers", blockerValues(v.Blockers))
	o.Set("unknowns", blockerValues(v.Unknowns))
	holds := make([]string, len(rec.Holds))
	for i, h := range rec.Holds {
		holds[i] = h.HoldID
	}
	o.Set("holds", wire.Strings(holds))
	o.Set("requiredGates", wire.Strings(rec.RequiredGates))
	o.Set("gateResults", wire.String(string(v.GateResults)))
	o.Set("currentAttempt", wire.String(string(v.CurrentAttempt)))
	o.Set("publication", wire.String(string(v.Publication)))
	if rec.Completion == nil {
		o.Set("completion", wire.Null())
	} else {
		o.Set("completion", wire.String(rec.Completion.Kind))
	}
	o.Set("nextAction", wire.String(v.NextAction))
	if includeRecord {
		o.Set("record", rec.Value())
	} else {
		o.Set("record", wire.Null())
	}
	return wire.ObjectValue(o)
}
