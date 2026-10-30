package mutation

import (
	"errors"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ObligationsPayload is one OBLIGATIONS_SEED, OBLIGATIONS_SET or
// OBLIGATIONS_WITNESS payload (TOL-V0-005, TOL-V0-007, TOL-V0-008). Exactly
// one of Seed, Set and Witness is set, matching Op.
type ObligationsPayload struct {
	Op      string
	Seed    *ticket.ObligationSeedPayload
	Set     *ticket.ObligationSetPayload
	Witness *ticket.ObligationWitnessPayload
}

func (p *ObligationsPayload) operation() string { return p.Op }

func (p *ObligationsPayload) value() wire.Value {
	switch {
	case p.Seed != nil:
		return p.Seed.Value()
	case p.Set != nil:
		return p.Set.Value()
	case p.Witness != nil:
		return p.Witness.Value()
	}
	return wire.ObjectValue(wire.NewObject())
}

func readObligations(op string, r *wire.Reader) *ObligationsPayload {
	p := &ObligationsPayload{Op: op}
	switch op {
	case ticket.OpObligationsSeed:
		p.Seed = ticket.ReadObligationSeedPayload(r)
	case ticket.OpObligationsSet:
		p.Set = ticket.ReadObligationSetPayload(r)
	default:
		p.Witness = ticket.ReadObligationWitnessPayload(r)
	}
	return p
}

// ObligationReportCheck is the writer's own recomputation of TOL-V0-010..012
// from a Playwright report (TOL-V0-013). Eligible holds every id the report
// credits: all of its matches passed at retry 0 and are source-bound at
// Commit, with those matches. IDs, when non-nil, is the sorted --ids subset.
// The report itself is never retained (TOL-V0-009).
type ObligationReportCheck struct {
	ReportSha256      wire.Digest
	PlaywrightVersion string
	Commit            string
	IDs               []string
	Eligible          map[string][]ticket.ObligationMatch
	// TestRepository is the declared test repository the recomputation ran
	// in (TOL-V0-065), "" for the store's own repository.
	TestRepository string
	// TestRepositoryPath is the path the policy the CLI read declared for
	// TestRepository. The writer refuses a witness whose name is now
	// declared with another path (TOL-V0-065). A DECLARED witness carries a
	// check holding only these two members.
	TestRepositoryPath string
}

// ExpectedCredits returns the credits the ledger l admits from the check, in
// id order: eligible ids restricted to IDs whose entry is OPEN, DEFECT or
// BLOCKED. WITNESSED and DEFERRED entries are not repeated (TOL-V0-008).
func (c *ObligationReportCheck) ExpectedCredits(l *ticket.ObligationLedger) []ticket.ObligationCredit {
	if c == nil {
		return nil
	}
	var subset map[string]bool
	if c.IDs != nil {
		subset = map[string]bool{}
		for _, id := range c.IDs {
			subset[id] = true
		}
	}
	var out []ticket.ObligationCredit
	for id, ms := range c.Eligible {
		if subset != nil && !subset[id] {
			continue
		}
		e := l.Entry(id)
		if e == nil || e.State == ticket.ObligationWitnessed || e.State == ticket.ObligationDeferred {
			continue
		}
		out = append(out, ticket.ObligationCredit{ID: id, Matches: ms})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ObligationsNameAttempt reports an OBLIGATIONS_WITNESS envelope that names
// an attempt or generation, which routes through the shared attempt audit
// (TOL-V0-015).
func ObligationsNameAttempt(env *Envelope) bool {
	if env == nil || env.Operation != ticket.OpObligationsWitness {
		return false
	}
	p, ok := env.Payload.(*ObligationsPayload)
	return ok && p.Witness != nil && (p.Witness.Attempt != nil || p.Witness.Generation != nil)
}

// obligations applies one OBLIGATIONS_* write (TOL-V0-014): the ledger
// event is derived, folded through the single transition, and the record
// moves by one revision with acceptanceRevision unchanged.
func (c Context) obligations(plan *Plan, env *Envelope) *Plan {
	p, _ := env.Payload.(*ObligationsPayload)
	pre, ok := c.Inventory.Get(env.TargetID.Raw)
	if !ok {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "target %s does not exist in the queue", env.TargetID.Raw))
	}
	plan.Pre = pre
	if env.ExpectedRevision == nil || *env.ExpectedRevision != pre.Revision {
		return plan.refused(refuse(OutcomeRevisionConflict, "", "expectedRevision does not equal the canonical revision %s", pre.Revision))
	}
	if pre.Source.Kind != "NATIVE" || pre.ShadowOverlay || (pre.Status != ticket.StatusOpen && pre.Status != ticket.StatusHeld) {
		return plan.refused(refuse(OutcomeBlocked, wire.CodeTicketState, "%s requires an OPEN or HELD native ticket (is %s %s)", env.Operation, pre.Source.Kind, pre.Status))
	}
	if r := c.obligationAuthority(pre, p); r != nil {
		return plan.refused(r)
	}
	var ledger *ticket.ObligationLedger
	if pre.ObligationsRef != nil {
		l, err := ticket.FoldObligationChain(pre.TicketID, *pre.ObligationsRef, func(d wire.Digest) ([]byte, error) { return c.ObligationEvents[d], nil })
		if err != nil {
			var ce *ticket.ChainError
			if errors.As(err, &ce) {
				return plan.refused(refuse(OutcomeValidationFailed, ce.Code, "%s", ce.Error()))
			}
			return plan.refused(refuseErr(err))
		}
		ledger = l
	}
	if ledger == nil && p.Seed == nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s ticket %s has no obligation ledger; seed it first", ticket.ObligationUnknownDetail, pre.TicketID.Raw))
	}
	if ledger == nil {
		for _, id := range c.Inventory.IDs() {
			other, _ := c.Inventory.Get(id)
			if other != nil && other.TicketID.Raw != pre.TicketID.Raw && other.Status != ticket.StatusArchived && other.Source.Kind == "NATIVE" &&
				other.ObligationsRef != nil && other.ObligationsRef.Prefix == p.Seed.Prefix {
				return plan.refused(refuse(OutcomeValidationFailed, wire.CodeDuplicateID, "obligation prefix %s is already declared by %s", p.Seed.Prefix, other.TicketID.Raw))
			}
		}
	}
	if p.Witness != nil && p.Witness.Source == ticket.ObligationSourceReport && !c.ObligationReplay {
		if r := c.ObligationReport.matches(ledger, p.Witness); r != nil {
			return plan.refused(r)
		}
	}
	if r := screenObligations(p); r != nil {
		return plan.refused(r)
	}
	if r := c.Screen.screenValues(obligationScreenFields(p)); r != nil {
		return plan.refused(r)
	}
	revision := int64(0)
	var previous *wire.Digest
	if pre.ObligationsRef != nil {
		revision = pre.ObligationsRef.Revision.Int()
		head := pre.ObligationsRef.Head
		previous = &head
	}
	if revision >= wire.ObligationsMaxEvents {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "the obligation ledger holds at most %d events", wire.ObligationsMaxEvents))
	}
	ev := ticket.ObligationEvent{TicketID: pre.TicketID, Revision: wire.CountOf(revision + 1), Previous: previous, RequestSha256: env.Sha256(), Request: env.Raw}
	blob, err := ev.Encode()
	if err != nil {
		return plan.refused(refuseErr(err))
	}
	decoded, err := ticket.DecodeObligationEvent(blob)
	if err != nil {
		return plan.refused(refuseErr(err))
	}
	head := wire.Sum(blob)
	next, err := ticket.ApplyObligationEvent(ledger, decoded, head)
	if err != nil {
		return plan.refused(refuseErr(err))
	}
	ref := ticket.ObligationsReference{Prefix: next.Prefix, Revision: wire.CountOf(next.Revision), Head: head, Counts: next.Counts()}
	ref.HighWater = ticket.ObligationHighWater{AcceptanceRevision: pre.AcceptanceRevision, Witnessed: ref.Counts.Witnessed}
	baseline := int64(0)
	if pre.ObligationsRef != nil {
		ref.LastRaise = pre.ObligationsRef.LastRaise
		baseline = pre.ObligationsRef.Counts.Witnessed
		if hw := pre.ObligationsRef.HighWater; hw.AcceptanceRevision == pre.AcceptanceRevision {
			baseline = hw.Witnessed
			if hw.Witnessed > ref.HighWater.Witnessed {
				ref.HighWater.Witnessed = hw.Witnessed
			}
		}
	}
	if w := p.Witness; w != nil && c.Binding.Role == "WORKER" && ref.Counts.Witnessed > baseline {
		ref.LastRaise = &ticket.ObligationRaise{Attempt: *w.Attempt, Generation: *w.Generation, AcceptanceRevision: pre.AcceptanceRevision}
	}
	work, err := clone(pre)
	if err != nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeOf(err), "canonical record is not valid: %v", err))
	}
	work.ObligationsRef = &ref
	if r := c.finalize(pre, work, false); r != nil {
		return plan.refused(r)
	}
	if work.Revision.Int() != pre.Revision.Int()+1 || work.AcceptanceRevision != pre.AcceptanceRevision {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "an obligation write must advance only the ticket revision"))
	}
	plan.DerivedEvent = blob
	return plan.completed(work)
}

// obligationAuthority applies the TOL-V0-015 limits inside a permitted role
// row: DECLARED and core changes are OWNER-only, a WORKER may only witness a
// report under its own live attempt generation, and no other role names an
// attempt.
func (c Context) obligationAuthority(pre *ticket.Record, p *ObligationsPayload) *refusal {
	role := c.Binding.Role
	if p.Set != nil && role != "OWNER" {
		for _, ch := range p.Set.Changes {
			if ch.Core != nil {
				return refuse(OutcomeUnauthorized, "", "changing core is OWNER-only (TOL-V0-015)")
			}
			if ch.Scope != nil && ch.Scope.Action == ticket.ObligationScopeAccept {
				return refuse(OutcomeUnauthorized, "", "accepting a scope decision is OWNER-only (TOL-V0-081)")
			}
		}
	}
	if p.Set != nil && c.Policy.ObligationScopeDecisionRequired() {
		for _, ch := range p.Set.Changes {
			if ch.Core != nil {
				return refuse(OutcomeValidationFailed, wire.CodeApprovalMissing, "%s %s: under obligations.requireScopeDecision core changes only through an accepted scope decision (TOL-V0-082)", ticket.ObligationScopeDecisionRequiredDetail, ch.ID)
			}
		}
	}
	if p.Set != nil && c.Policy.ObligationCitesDeclaredRequired() {
		for _, ch := range p.Set.Changes {
			for _, ct := range ch.Cites {
				if ct.Repository == nil {
					continue
				}
				if _, ok := c.Policy.ObligationTestRepository(*ct.Repository); !ok {
					return refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s %s cites repository %s, which the policy's obligations.testRepositories does not declare (TOL-V0-102)", ticket.ObligationCiteRepositoryDetail, ch.ID, *ct.Repository)
				}
			}
		}
	}
	w := p.Witness
	if w == nil {
		if role == "WORKER" {
			return refuse(OutcomeUnauthorized, "", "a WORKER may only witness a Playwright report")
		}
		return nil
	}
	if w.Source == ticket.ObligationSourceDeclared && role != "OWNER" {
		return refuse(OutcomeUnauthorized, "", "a DECLARED witness is OWNER-only (TOL-V0-015)")
	}
	if w.Source == ticket.ObligationSourceReviewed && role != "OWNER" {
		return refuse(OutcomeUnauthorized, "", "a REVIEWED witness is OWNER-only (TOL-V0-073)")
	}
	if w.TestRepository != nil {
		declared, ok := c.Policy.ObligationTestRepository(*w.TestRepository)
		if !ok {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s /payload/testRepository names %s, which the policy's obligations.testRepositories does not declare", ticket.ObligationTestRepositoryDetail, *w.TestRepository)
		}
		if check := c.ObligationReport; !c.ObligationReplay && (check == nil || check.TestRepository != *w.TestRepository || check.TestRepositoryPath != declared) {
			return refuse(OutcomeRevisionConflict, wire.CodeStalePolicy, "/payload/testRepository %s is declared at write time with a path other than the one the witness resolved; rerun the witness", *w.TestRepository)
		}
	}
	if role != "WORKER" {
		if w.Attempt != nil || w.Generation != nil {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "/payload/attempt: only a WORKER witness names an attempt")
		}
		return nil
	}
	if w.Attempt == nil || w.Generation == nil {
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "/payload/attempt: a WORKER witness must name its live attempt and generation")
	}
	if err := CheckAttemptProvenance(c.AttemptLedgerOf(), pre.TicketID, w.Attempt, w.Generation); err != nil {
		return refuseErr(err)
	}
	a := c.AttemptLedgerOf()[*w.Attempt]
	if !a.Live || a.Holder == "" || a.Generation != *w.Generation {
		return refuse(OutcomeRevisionConflict, wire.CodeFenced, "/payload/attempt and /payload/generation do not name a live attempt generation")
	}
	if a.Holder != c.Binding.ID {
		return refuse(OutcomeUnauthorized, "", "/payload/attempt is not held by the acting binding")
	}
	return nil
}

// matches refuses a report witness whose credits differ from the writer's
// recomputation (TOL-V0-013).
func (c *ObligationReportCheck) matches(l *ticket.ObligationLedger, w *ticket.ObligationWitnessPayload) *refusal {
	mismatch := func(what string) *refusal {
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s %s", ticket.ObligationCreditMismatchDetail, what)
	}
	if c == nil {
		return mismatch("no report was supplied for the writer to recompute the credits")
	}
	if w.ReportSha256 == nil || *w.ReportSha256 != c.ReportSha256 || w.PlaywrightVersion == nil || *w.PlaywrightVersion != c.PlaywrightVersion || w.Commit != c.Commit {
		return mismatch("the payload report digest, version or commit differs from the supplied report")
	}
	if named := w.TestRepository; (named == nil) != (c.TestRepository == "") || (named != nil && *named != c.TestRepository) {
		return mismatch("the payload test repository differs from the repository the credits were recomputed in")
	}
	want := c.ExpectedCredits(l)
	if len(want) != len(w.Credits) {
		return mismatch("the payload credits differ from the recomputed credits")
	}
	for i := range want {
		if want[i].ID != w.Credits[i].ID || !wire.Equal(ticket.MatchesValue(want[i].Matches), ticket.MatchesValue(w.Credits[i].Matches)) {
			return mismatch("the payload credits differ from the recomputed credits at " + want[i].ID)
		}
	}
	return nil
}

// obligationFields is a payload's stored prose, paths and test titles.
func obligationFields(p *ObligationsPayload) []string {
	var fields []string
	switch {
	case p.Seed != nil:
		for _, o := range p.Seed.Obligations {
			fields = append(fields, o.Title, o.Alias)
		}
	case p.Set != nil:
		for _, ch := range p.Set.Changes {
			fields = append(fields, ch.Reason)
			for _, ct := range ch.Cites {
				fields = append(fields, ct.Path)
			}
			if ch.Scope != nil {
				fields = append(fields, ch.Scope.Evidence)
			}
			if ch.Candidate != nil {
				fields = append(fields, ch.Candidate.Path)
			}
			for _, src := range ch.Sources {
				fields = append(fields, src.Ref, src.Query)
				fields = append(fields, src.Searched...)
			}
		}
	case p.Witness != nil:
		if d := p.Witness.Declaration; d != nil {
			fields = append(fields, d.TestID, d.Reason)
		}
		fields = append(fields, p.Witness.Provenance.Strings()...)
		if rv := p.Witness.Review; rv != nil {
			fields = append(fields, rv.Subject.Path)
			for _, v := range rv.Verdicts {
				fields = append(fields, v.Reviewer, v.Reason)
			}
		}
		for _, cr := range p.Witness.Credits {
			for _, m := range cr.Matches {
				fields = append(fields, m.Path, m.TestID)
				fields = append(fields, m.TitlePath...)
				fields = append(fields, m.StepPath...)
			}
		}
	}
	return fields
}

// screenObligations refuses a payload whose stored prose, paths or test
// titles match the shared secret screen (TOL-V0-009). The detail names the
// field, never the value.
func screenObligations(p *ObligationsPayload) *refusal {
	fields := obligationFields(p)
	for _, s := range fields {
		if matchesSecretScreen(s) {
			return refuse(OutcomeValidationFailed, wire.CodeSecretDetected, "/payload: a title, alias, reason, path, test id, review source or witness provenance matches the secret screen; remove the secret and retry with a new request ID")
		}
	}
	return nil
}

// obligationScreenFields names an obligation write's stored text for the
// KHN-V0-030 value screen; the detail stays at /payload, as TOL-V0-009's.
func obligationScreenFields(p *ObligationsPayload) []screenField {
	var out []screenField
	for _, s := range obligationFields(p) {
		out = append(out, screenField{"/payload", s})
	}
	return out
}

// ReplayObligation re-derives one committed OBLIGATIONS_* write from audited
// historical pre-state through the ordinary Apply path, as ReplayOperatorNote
// does for notes. events holds the prior chain by digest; attempts is the
// attempt ledger at the receipt's prior state, consulted for a WORKER
// witness. The report check is skipped (the report is not retained,
// TOL-V0-009); the caller audits source presence separately (TOL-V0-013).
func ReplayObligation(queue wire.QueueID, policy *intent.Policy, inventory []*ticket.Record, events map[wire.Digest][]byte, attempts AttemptLedger, binding Binding, request []byte, recordedAt wire.Timestamp) (post, event []byte, err error) {
	env, err := Decode(request)
	if err != nil {
		return nil, nil, err
	}
	if !ticket.IsObligationOperation(env.Operation) {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "/operation", "%s is not an obligation operation", env.Operation)
	}
	inv, err := ticket.NewInventory(queue, inventory)
	if err != nil {
		return nil, nil, err
	}
	ctx := Context{Binding: binding, Queue: &intent.Queue{QueueID: queue}, Policy: policy, Inventory: inv, Requests: noRequests{}, Now: recordedAt,
		ObligationEvents: events, ObligationReplay: true}.WithAttemptLedger(attempts)
	plan := Apply(ctx, env)
	if !plan.Planned() || plan.DerivedEvent == nil {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "/operation", "obligation transition does not replay: %s %v %s", plan.Outcome.Outcome, plan.Outcome.Codes, plan.Detail)
	}
	return plan.Post.Encode(), plan.DerivedEvent, nil
}
