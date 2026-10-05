package ticket

import (
	"strings"

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
	// Stage is the claim or plan stage (CAL-V0-099). Execution
	// prerequisites listing it apply; "" (a stageless claim, plan, show or
	// blockers read) fails closed and applies every prerequisite.
	Stage string
	// Loop is the CAL-V0-102 no-progress loop hold of the one ticket this
	// context views, derived by the caller from audited attempt history
	// under an opt-in policy; nil when absent or not evaluated.
	Loop *LoopHold
}

// LoopHold is a derived CAL-V0-102 LOOP_DETECTED hold. Signal is
// NO_PROGRESS or ALTERNATING_RETURNS; Generations are the counted
// generations, oldest first, at AcceptanceRevision; Limit is the policy
// bound they exceed.
type LoopHold struct {
	Signal             string
	AcceptanceRevision wire.Count
	Generations        []string
	Limit              wire.Count
}

// Detail renders the hold for a blocker or refusal.
func (h *LoopHold) Detail() string {
	return "no-progress loop " + h.Signal + " at acceptanceRevision " + string(h.AcceptanceRevision) + ": generations " + strings.Join(h.Generations, ",") + " exceed the policy bound " + string(h.Limit)
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
	// SuggestedEvidence is set only by OfferCompletion (ERG-V0-011); nil
	// leaves the rendered view byte-identical to a view without the offer.
	SuggestedEvidence []wire.Digest
	Untrusted         bool // true when queue prose is embedded (title/body)
}

// NextActionCompleteManual is the ERG-V0-011 read-only completion offer.
const NextActionCompleteManual = "complete-manual"

// OfferCompletion applies the ERG-V0-011 read-only completion offer. evidence
// is the sorted head digests of every required external review gate, each a
// CURRENT PASS (transaction.ExternalReviewCompletionOffer). The offer replaces
// only the admit action of an OPEN ticket with no blocker and nothing
// NOT_OBSERVED; any other view, or empty evidence, is left unchanged. It never
// writes, completes or satisfies a gate: complete-manual stays the operator's
// disposition (ERG-V0-007).
func (v *View) OfferCompletion(evidence []wire.Digest) bool {
	if len(evidence) == 0 || v.Record == nil || v.Record.Status != StatusOpen || v.NextAction != "admit" || len(v.Blockers) != 0 || len(v.Unknowns) != 0 {
		return false
	}
	v.NextAction = NextActionCompleteManual
	v.SuggestedEvidence = append([]wire.Digest{}, evidence...)
	return true
}

// DigestsValue renders digests as a JSON string array.
func DigestsValue(ds []wire.Digest) wire.Value {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = string(d)
	}
	return wire.Strings(out)
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
	// ESC-V0-006: current OPEN decision, scope or blocked questions hold.
	if ids := rec.EscalationPending(); len(ids) != 0 {
		add(wire.CodeEscalationPending, "", "escalation questions pending: "+strings.Join(ids, ","))
	}
	// CAL-V0-102: the caller's derived no-progress loop hold.
	if ctx.Loop != nil && ctx.Loop.AcceptanceRevision == rec.AcceptanceRevision {
		add(wire.CodeLoopDetected, "", ctx.Loop.Detail())
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
	// CAL-V0-099 stage-scoped execution prerequisites, after dependencies and
	// with the same three-valued gate reading; never part of Problems.
	for _, p := range rec.ExecutionPrerequisites {
		if !PrerequisiteApplies(p, ctx.Stage) {
			continue
		}
		scope := " (execution prerequisite for stages " + strings.Join(p.Stages, ",") + ")"
		pre, ok := inv.byID[p.TicketID.Raw]
		if !ok {
			add(wire.CodePrerequisiteUnsatisfied, p.TicketID.Raw, "prerequisite "+p.TicketID.Raw+" does not exist in the queue"+scope)
			continue
		}
		switch p.Obligation {
		case "COMPLETED":
			if !(pre.Status == StatusCompleted || (pre.Status == StatusArchived && pre.ArchivedFrom != nil && *pre.ArchivedFrom == StatusCompleted)) {
				add(wire.CodePrerequisiteUnsatisfied, pre.TicketID.Raw, "prerequisite "+pre.TicketID.Raw+" is "+pre.Status+", obligation COMPLETED"+scope)
			}
		case "GATE_PASSED":
			gate := ""
			if p.GateID != nil {
				gate = *p.GateID
			}
			switch ctx.gates().GatePassed(pre.TicketID, gate, pre.AcceptanceRevision) {
			case Satisfied:
			case Unsatisfied:
				add(wire.CodePrerequisiteUnsatisfied, pre.TicketID.Raw, "prerequisite gate "+gate+" of "+pre.TicketID.Raw+" has no PASSED result at acceptanceRevision "+string(pre.AcceptanceRevision)+scope)
			default:
				unknown(wire.CodePrerequisiteUnsatisfied, pre.TicketID.Raw, "prerequisite gate "+gate+" of "+pre.TicketID.Raw+": result NOT_OBSERVED (no journal evidence available to this reader)"+scope)
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
	v.NextAction = nextAction(rec, blockers, unknowns)
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
// An unknown execution prerequisite (CAL-V0-099: a GATE_PASSED obligation
// whose gate is NOT_OBSERVED) refuses admission like a blocker, so it waits
// on the prerequisite rather than recommending an admission that must fail.
func nextAction(rec *Record, blockers, unknowns []Blocker) string {
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
		for _, u := range unknowns {
			if u.Code == wire.CodePrerequisiteUnsatisfied {
				return "wait-dependency"
			}
		}
		// Intent checks passed; admission itself needs the journal (TCP-02)
		// to observe attempts and gates, so the next action is that check.
		return "admit"
	}
	switch blockers[0].Code {
	case wire.CodeDependencyMissing, wire.CodeCycle:
		return "set-dependencies"
	case wire.CodeDependencyUnsatisfied, wire.CodePrerequisiteUnsatisfied:
		return "wait-dependency"
	case wire.CodeApprovalMissing, wire.CodeApprovalRevoked:
		return "grant-approval"
	case wire.CodeCoverageUnknown, wire.CodeExternalUnbounded:
		return "set-effects"
	case wire.CodeCutoverMissing:
		return "cutover"
	case wire.CodeAttemptLive:
		return "wait-attempt"
	case wire.CodeEscalationPending:
		return "answer"
	case wire.CodeLoopDetected:
		return "reopen"
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
	if rec.RequiredRoles != nil {
		o.Set("requiredRoles", StageRolesValue(rec.RequiredRoles))
	}
	if len(rec.ExecutionPrerequisites) > 0 {
		o.Set("executionPrerequisites", PrerequisitesValue(rec.ExecutionPrerequisites))
	}
	o.Set("gateResults", wire.String(string(v.GateResults)))
	o.Set("currentAttempt", wire.String(string(v.CurrentAttempt)))
	o.Set("publication", wire.String(string(v.Publication)))
	if rec.Completion == nil {
		o.Set("completion", wire.Null())
	} else {
		o.Set("completion", wire.String(rec.Completion.Kind))
	}
	o.Set("nextAction", wire.String(v.NextAction))
	if v.SuggestedEvidence != nil {
		o.Set("suggestedEvidence", DigestsValue(v.SuggestedEvidence))
	}
	if includeRecord {
		o.Set("record", rec.Value())
	} else {
		o.Set("record", wire.Null())
	}
	return wire.ObjectValue(o)
}

// EscalationPending returns the sorted, bounded request IDs of the record's
// ESCALATION_PENDING derived hold (ESC-V0-006), or nil when nothing holds.
// It reads only the tool-owned `escalations` reference through the predicate
// Core's planner shares, and never writes the hold.
func (rec *Record) EscalationPending() []string {
	if rec.Escalations == nil {
		return nil
	}
	entries := make([]wire.EscalationHoldEntry, 0, len(rec.Escalations.Entries))
	for _, e := range rec.Escalations.Entries {
		entries = append(entries, wire.EscalationHoldEntry{RequestID: e.RequestID, AcceptanceRevision: string(e.AcceptanceRevision), Kind: e.Kind, State: e.State})
	}
	ids, _ := wire.EscalationPending(string(rec.AcceptanceRevision), entries)
	return ids
}
