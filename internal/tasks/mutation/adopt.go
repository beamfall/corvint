package mutation

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// AdoptOperation names the reconcile operation in the adoption request
// digest preimage (§3.3 "ADOPT_FILE request digest").
const AdoptOperation = "ADOPT_FILE"

// AdoptProtectedFields are the record fields a diverged intent file may
// never change through ADOPT_FILE (§3.3). A difference in any of them
// refuses VALIDATION_FAILED/ADOPT_UNSUPPORTED_FIELD and leaves the ticket
// diverged. `status` is listed but is derived, never adopted: a difference
// is tolerated only between the live statuses DRAFT, OPEN and HELD, and the
// composed operations (REFINE opening a DRAFT, HOLD/RELEASE_HOLD) decide the
// post status; see adoptStatusCovered. COMPLETED and ARCHIVED can neither
// be entered nor left by adoption.
var AdoptProtectedFields = []string{
	"status", "archivedFrom", "completion", "approvals", "revision", "acceptanceRevision",
	"previousRecordSha256", "source", "shadowOverlay", "supersededBy", "createdAt", "updatedBy",
}

// adoptIgnoredFields are compared neither for difference nor for coverage:
// the adoption receipt sets them.
var adoptIgnoredFields = map[string]bool{
	"updatedAt": true, "updatedBy": true, "revision": true, "acceptanceRevision": true, "previousRecordSha256": true,
}

// AdoptDigest is the TM-V0-006 request digest of an ADOPT_FILE request
// (§3.3 "ADOPT_FILE request digest"): the SHA-256 of the canonical encoding,
// trailing LF included, of the closed object
//
//	{actor:{id,role}, fileSha256, operation:"ADOPT_FILE", queueId, requestId, targetId}
//
// where actor is the trusted Binding (never the file's updatedBy or any
// claim), fileSha256 is the SHA-256 of the exact file bytes offered, queueId
// is the context queue and targetId the canonical record's ticketId. The
// preimage binds the operation, the request, the reconciling principal, the
// queue, the target and the bytes, and nothing that moves between an
// original and its retry (no timestamp, no current revision): an identical
// retry after commit replays, while the same request ID reused by another
// actor or role, for another target or queue, or with other bytes conflicts.
func AdoptDigest(b Binding, queueID wire.QueueID, targetID wire.TicketID, requestID string, file []byte) wire.Digest {
	o := wire.NewObject()
	a := wire.NewObject()
	a.Set("id", wire.String(b.ID))
	a.Set("role", wire.String(b.Role))
	o.Set("actor", wire.ObjectValue(a))
	o.Set("fileSha256", wire.String(string(wire.Sum(file))))
	o.Set("operation", wire.String(AdoptOperation))
	o.Set("queueId", wire.String(queueID.Raw))
	o.Set("requestId", wire.String(requestID))
	o.Set("targetId", wire.String(targetID.Raw))
	return wire.Sum(wire.EncodeFile(wire.ObjectValue(o)))
}

// Adopt computes the §3.3 ADOPT_FILE composition (R2 F3): the per-field
// difference between a diverged intent file and the canonical record is
// mapped, in the fixed order of the contract table, onto composed
// operations that are each validated by the same step rules as if the
// reconciling actor had issued them, and the whole composition is applied as
// one revision. The result is a Plan whose Post is the single RECONCILE post
// record: revision +1 exactly once, acceptanceRevision +1 iff an
// acceptance-relevant field changed, previousRecordSha256 chained from the
// canonical record and never from the file. Nothing is written; the ticket
// stays diverged until the transaction writer commits the plan.
//
// The reconciling actor is the trusted Binding (OWNER or OPERATOR); the file's
// `updatedBy` and each hold's `actor`/`placedAt` are never adopted. The
// request digest is AdoptDigest, computed from the immutable request identity
// (binding, queue, canonical ticketId, requestId, file bytes) before the
// replay question is asked. A tombstone accepts only RESTORE (§3.2), so an
// ARCHIVED canonical record refuses BLOCKED/TICKET_STATE before anything is
// composed: not even an empty or updatedAt-only adoption may mint a revision
// on it.
func Adopt(ctx Context, requestID string, canonical *ticket.Record, file []byte) *Plan {
	plan := &Plan{Pre: canonical, Composed: []string{}}
	plan.Outcome.RequestID = requestID
	if _, err := ParseRequestID("/requestId", requestID); err != nil {
		return plan.refused(refuseErr(err))
	}
	if r := ctx.Binding.validate(); r != nil {
		return plan.refused(r)
	}
	if ctx.Binding.Role != "OWNER" && ctx.Binding.Role != "OPERATOR" {
		return plan.refused(refuse(OutcomeUnauthorized, "", "ADOPT_FILE is issued by OWNER or OPERATOR, not %s", ctx.Binding.Role))
	}
	if r := ctx.checkInputs(); r != nil {
		return plan.refused(r)
	}
	// Immutable request identity before replay: the target is the canonical
	// record's ticketId and must be the inventory's current record.
	if canonical == nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "no canonical record"))
	}
	if canonical.TicketID.QueueID() != ctx.Queue.QueueID.Raw {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "canonical record %s is outside queue %s", canonical.TicketID.Raw, ctx.Queue.QueueID.Raw))
	}
	if cur, ok := ctx.Inventory.Get(canonical.TicketID.Raw); !ok || cur.FileDigest() != canonical.FileDigest() {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "canonical record %s is not the inventory's current record", canonical.TicketID.Raw))
	}
	plan.MutationSha256 = AdoptDigest(ctx.Binding, ctx.Queue.QueueID, canonical.TicketID, requestID, file)
	if out := replay(ctx.Requests, requestID, plan.MutationSha256); out != nil {
		plan.Outcome = *out
		return plan
	}
	fileRec, err := ticket.Decode(file)
	if err != nil {
		return plan.refused(refuseErr(err))
	}
	if fileRec.TicketID.Raw != canonical.TicketID.Raw {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "file ticketId %s is not the canonical %s", fileRec.TicketID.Raw, canonical.TicketID.Raw))
	}
	// §3.2: a tombstone accepts only RESTORE, which ADOPT_FILE never composes.
	if canonical.Status == ticket.StatusArchived {
		return plan.refused(refuse(OutcomeBlocked, wire.CodeTicketState, "ticket is ARCHIVED; only RESTORE is permitted and ADOPT_FILE composes none"))
	}
	diff := differingFields(canonical, fileRec)
	// Protected fields first, in the contract's order, so the refusal names
	// the first unsupported difference and nothing is composed.
	var protected []string
	for _, f := range AdoptProtectedFields {
		if !diff[f] {
			continue
		}
		if f == "status" && derivableStatus(canonical.Status) && derivableStatus(fileRec.Status) {
			continue
		}
		protected = append(protected, f)
	}
	if len(protected) > 0 {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeAdoptUnsupportedField, "file differs in protected field(s) %s; adopt refused, ticket stays diverged", strings.Join(protected, ", ")))
	}
	ops, r := composeAdopt(canonical, fileRec, diff)
	if r != nil {
		return plan.refused(r)
	}
	work, err := clone(canonical)
	if err != nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeOf(err), "canonical record is not valid: %v", err))
	}
	for _, op := range ops {
		if r := ctx.step(work, op); r != nil {
			return plan.refused(r)
		}
		plan.Composed = append(plan.Composed, op.operation())
	}
	// Every difference must have been covered by a composed operation, and
	// the file's status must be the derived one.
	if r := adoptCovered(canonical, work, fileRec); r != nil {
		return plan.refused(r)
	}
	if r := ctx.finalize(canonical, work, false); r != nil {
		return plan.refused(r)
	}
	return plan.completed(work)
}

// derivableStatus reports whether a status can be reached or left by the
// composed operations: DRAFT opens through REFINE (§3.2), OPEN and HELD
// follow holds. COMPLETED and ARCHIVED are never derivable.
func derivableStatus(status string) bool {
	return status == ticket.StatusDraft || status == ticket.StatusOpen || status == ticket.StatusHeld
}

// differingFields compares every record key by canonical bytes.
func differingFields(a, b *ticket.Record) map[string]bool {
	ao := a.Value().Obj
	bo := b.Value().Obj
	out := map[string]bool{}
	for _, k := range ao.SortedKeys() {
		av, _ := ao.Get(k)
		bv, _ := bo.Get(k)
		if !wire.Equal(av, bv) {
			out[k] = true
		}
	}
	return out
}

// composeAdopt maps the differing fields onto operations in the fixed §3.3
// order: REFINE, PRIORITIZE, SET_DEPENDENCIES, SET_GATES, SET_EFFECTS, then
// HOLD per hold only in the file and RELEASE_HOLD per hold only in the
// canonical record.
func composeAdopt(canonical, file *ticket.Record, diff map[string]bool) ([]Payload, *refusal) {
	var ops []Payload
	refine := &RefinePayload{Present: map[string]bool{}}
	for _, k := range RefineFields {
		if !diff[k] {
			continue
		}
		refine.Present[k] = true
		switch k {
		case "title":
			refine.Title = file.Title
		case "body":
			refine.Body = copyString(file.Body)
		case "kind":
			refine.Kind = file.Kind
		case "owner":
			refine.Owner = copyString(file.Owner)
		case "milestone":
			refine.Milestone = copyString(file.Milestone)
		case "labels":
			refine.Labels = copyStrings(file.Labels)
		case "acceptanceCriteria":
			refine.AcceptanceCriteria = copyStrings(file.AcceptanceCriteria)
		case "requirementRefs":
			refine.RequirementRefs = copyStrings(file.RequirementRefs)
		case "dueDate":
			refine.DueDate = copyString(file.DueDate)
		case "estimateMinutes":
			refine.EstimateMinutes = copyCount(file.EstimateMinutes)
		case "supersedes":
			refine.Supersedes = copyTicket(file.Supersedes)
		}
	}
	if len(refine.Present) > 0 {
		ops = append(ops, refine)
	}
	if diff["priority"] || diff["order"] {
		ops = append(ops, &PrioritizePayload{Priority: file.Priority, Order: file.Order})
	}
	if diff["dependencies"] {
		ops = append(ops, &SetDependenciesPayload{Dependencies: copyDependencies(file.Dependencies)})
	}
	if diff["requiredGates"] {
		ops = append(ops, &SetGatesPayload{RequiredGates: copyStrings(file.RequiredGates)})
	}
	if diff["effects"] || diff["capabilities"] || diff["executionClass"] {
		ops = append(ops, &SetEffectsPayload{Effects: copyEffects(file.Effects), Capabilities: copyStrings(file.Capabilities), ExecutionClass: file.ExecutionClass})
	}
	if diff["holds"] {
		holdOps, r := composeHolds(canonical.Holds, file.Holds)
		if r != nil {
			return nil, r
		}
		ops = append(ops, holdOps...)
	}
	return ops, nil
}

// composeHolds diffs holds by holdId. An entry present on both sides must be
// byte-identical (actor, reason, placedAt): the contract composes only
// HOLD and RELEASE_HOLD, so an edit to an existing hold has no composed
// operation and is refused rather than silently ignored. The relative order
// of the common entries must also be unchanged (holds are a semantic array).
func composeHolds(canonical, file []ticket.Hold) ([]Payload, *refusal) {
	canonBy := map[string]ticket.Hold{}
	for _, h := range canonical {
		canonBy[h.HoldID] = h
	}
	fileBy := map[string]ticket.Hold{}
	for _, h := range file {
		fileBy[h.HoldID] = h
	}
	var canonCommon, fileCommon []string
	for _, h := range canonical {
		if f, ok := fileBy[h.HoldID]; ok {
			if !wire.Equal(holdValue(h), holdValue(f)) {
				return nil, refuse(OutcomeValidationFailed, wire.CodeAdoptUnsupportedField, "file edits existing hold %q (actor, reason or placedAt); no composed operation covers that", h.HoldID)
			}
			canonCommon = append(canonCommon, h.HoldID)
		}
	}
	for _, h := range file {
		if _, ok := canonBy[h.HoldID]; ok {
			fileCommon = append(fileCommon, h.HoldID)
		}
	}
	if strings.Join(canonCommon, "\x00") != strings.Join(fileCommon, "\x00") {
		return nil, refuse(OutcomeValidationFailed, wire.CodeAdoptUnsupportedField, "file reorders existing holds; no composed operation covers that")
	}
	var ops []Payload
	for _, h := range file {
		if _, ok := canonBy[h.HoldID]; !ok {
			// The file's actor and placedAt are ignored; the reconciling
			// context supplies them in step.
			ops = append(ops, &HoldPayload{HoldID: h.HoldID, Reason: h.Reason})
		}
	}
	for _, h := range canonical {
		if _, ok := fileBy[h.HoldID]; !ok {
			ops = append(ops, &ReleaseHoldPayload{HoldID: h.HoldID})
		}
	}
	return ops, nil
}

func holdValue(h ticket.Hold) wire.Value {
	o := wire.NewObject()
	o.Set("holdId", wire.String(h.HoldID))
	o.Set("actor", wire.String(h.Actor))
	o.Set("reason", wire.String(h.Reason))
	o.Set("placedAt", wire.String(string(h.PlacedAt)))
	return wire.ObjectValue(o)
}

// adoptCovered proves that the composed operations reproduce the file on
// every field the adoption does not set itself. Holds are compared as the
// sequence of (holdId, reason) because new entries take actor/placedAt from
// the context, not from the file. Status is compared by adoptStatusCovered.
func adoptCovered(canonical, work, file *ticket.Record) *refusal {
	wo := work.Value().Obj
	fo := file.Value().Obj
	for _, k := range wo.SortedKeys() {
		if adoptIgnoredFields[k] || k == "holds" || k == "status" {
			continue
		}
		wv, _ := wo.Get(k)
		fv, _ := fo.Get(k)
		if !wire.Equal(wv, fv) {
			return refuse(OutcomeValidationFailed, wire.CodeAdoptUnsupportedField, "difference in %q is not covered by the composition rule", k)
		}
	}
	if r := adoptStatusCovered(canonical, work, file); r != nil {
		return r
	}
	if len(work.Holds) != len(file.Holds) {
		return refuse(OutcomeValidationFailed, wire.CodeAdoptUnsupportedField, "holds difference is not covered by the composition rule")
	}
	for i := range work.Holds {
		if work.Holds[i].HoldID != file.Holds[i].HoldID || work.Holds[i].Reason != file.Holds[i].Reason {
			return refuse(OutcomeValidationFailed, wire.CodeAdoptUnsupportedField, "holds order in the file is not covered by the composition rule")
		}
	}
	return nil
}

// adoptStatusCovered accepts the file's status only when it is the status the
// composed operations derived, with one carve-out: a file that adds
// acceptance criteria to a DRAFT without rewriting `status` still says DRAFT
// while the composed REFINE opened the ticket (§3.2 DRAFT→OPEN); that file is
// adopted as OPEN because the derivation, not the file, decides status. Every
// other mismatch (a hand-opened DRAFT whose criteria stay empty, a DRAFT
// written over an OPEN record, HELD without the holds that derive it) is
// refused ADOPT_UNSUPPORTED_FIELD.
func adoptStatusCovered(canonical, work, file *ticket.Record) *refusal {
	if work.Status == file.Status {
		return nil
	}
	if canonical.Status == ticket.StatusDraft && file.Status == ticket.StatusDraft && work.Status == ticket.StatusOpen {
		return nil
	}
	return refuse(OutcomeValidationFailed, wire.CodeAdoptUnsupportedField, "file status %s is not the status %s derived by the composition rule", file.Status, work.Status)
}
