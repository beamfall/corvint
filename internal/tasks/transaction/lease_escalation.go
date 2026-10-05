package transaction

import (
	"bytes"
	"errors"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Typed escalation verbs (ESC-V0-001, ESC-V0-004, ESC-V0-010). The lease
// evidence is the digest of the typed request; the request ID is the
// question's own ID for ESCALATE and the answer's own ID for ANSWER.
const (
	LeaseEscalate = "ESCALATE"
	LeaseAnswer   = "ANSWER"
)

// EscalationFacts are the writer's observations for ESCALATE and ANSWER: the
// typed request, the claim receipt an OPEN names, and blobs by digest (the
// ticket's question origins and heads, and the claim's POST attempt when it
// was posted as a blob). The planner checks each against the inventory and
// ignores anything it does not need, so a missing fact abstains.
type EscalationFacts struct {
	Request      []byte
	ClaimReceipt []byte
	Blobs        map[wire.Digest][]byte
}

var escalationOperation = map[string]string{LeaseEscalate: "OPEN", LeaseAnswer: "ANSWER"}

// planEscalation reduces one typed request against the audited ticket and
// posts the next ticket record and its events. Attempts, reservations and
// gates are untouched (ESC-V0-010).
func planEscalation(c leaseContext) leaseOutcome {
	f := c.in.LeaseFacts.Escalation
	if f == nil || string(wire.Sum(f.Request)) != c.l.Evidence {
		return c.fail(malformed("escalation request binding"))
	}
	req, e := ticket.DecodeEscalationRequest(f.Request)
	if e != nil {
		return c.fail(e)
	}
	if req.Operation != escalationOperation[c.l.Verb] || req.RequestID != c.r.RequestID || req.QueueID != c.r.QueueID {
		return c.fail(malformed("escalation request names another operation, request or queue"))
	}
	rec, ok := c.st.tickets.Get(req.TicketID)
	if !ok {
		return c.fail(malformed("unknown ticket " + req.TicketID))
	}
	path := "intent/tickets/" + rec.TicketID.Local + ".json"
	if !c.in.Inventory.matches(path, wire.EncodeFile(rec.Value())) {
		return c.fail(malformed("physical projection differs from canonical record"))
	}
	obs := EscalationObservation{
		Snapshot:        c.escalationSnapshot(rec, f.Blobs),
		Actor:           c.r.Actor.ID,
		ActorRole:       c.r.Actor.Role,
		PolicyDecision:  escalationGrant(c.st.policy, c.r.Actor.Role, c.l.Verb),
		PolicyOperation: escalationOperation[c.l.Verb],
		Now:             c.in.RecordedAt,
		Replay:          EscalationReplay{State: "ABSENT"},
	}
	// Admission is audited only once binding and grant hold, so their
	// refusals keep the reducer's order (ESC-V0-001).
	if req.Operation == "OPEN" && req.Actor == c.r.Actor.ID && req.ActorRole == c.r.Actor.Role && obs.PolicyDecision == "ALLOWED" {
		adm, refusal := c.escalationAdmission(req.Open.Source, rec, f)
		if refusal != nil {
			return *refusal
		}
		obs.Admission = adm
		obs.BlockedRelationState = c.blockedRelation(req.Open.BlockedBy)
	}
	prop, e := ApplyEscalation(f.Request, obs)
	if e != nil {
		return c.escalationRefused(e)
	}
	ctx := mutation.Context{Binding: c.r.Actor, Queue: c.st.queue, Policy: c.st.policy, Inventory: c.st.tickets, Attempts: entryOracle{c.st.reservations}, Requests: absentIndex{}, Now: c.in.RecordedAt}
	post, e := mutation.SetEscalations(ctx, rec, prop.Refs)
	if e != nil {
		if wire.CodeOf(e) == wire.CodeLimitExceeded {
			return c.refuse(mutation.OutcomeCapacityExhausted, wire.CodeLimitExceeded, e.Error())
		}
		return c.fail(e)
	}
	if post.Revision != prop.TicketRevision || post.AcceptanceRevision != prop.AcceptanceRevision {
		return c.fail(malformed("escalation post record revision differs from the reduced revision"))
	}
	posts := map[string][]byte{path: wire.EncodeFile(post.Value())}
	ids := make([]string, 0, len(prop.Events))
	for _, ev := range prop.Events {
		posts["evidence/"+string(ev.Sha256)] = bytes.Clone(ev.Bytes)
		decoded, e := ticket.DecodeEscalationEvent(ev.Bytes)
		if e != nil {
			return c.fail(e)
		}
		ids = append(ids, decoded.Operation+" "+decoded.EscalationID)
	}
	eff := &leaseEffect{kind: "TRANSITION", outcome: mutation.OutcomeCompleted, codes: []string{}}
	return leaseOutcome{posts: posts, effect: eff, ticket: &ticketEffect{pre: rec, post: post, kind: "TRANSITION"}, detail: "escalation " + strings.Join(ids, ", ")}
}

// escalationSnapshot is the audited ticket view. Only blobs the inventory
// holds at their evidence path enter it; a missing one is MISSING_EVIDENCE
// when the reducer needs it.
func (c leaseContext) escalationSnapshot(rec *ticket.Record, supplied map[wire.Digest][]byte) EscalationSnapshot {
	s := EscalationSnapshot{QueueID: c.r.QueueID, TicketID: rec.TicketID.Raw, TicketRevision: rec.Revision, AcceptanceRevision: rec.AcceptanceRevision, Refs: rec.Escalations, Blobs: map[wire.Digest][]byte{}}
	if rec.Escalations == nil {
		return s
	}
	for _, en := range rec.Escalations.Entries {
		for _, d := range []wire.Digest{en.OriginSha256, en.HeadSha256} {
			if raw, ok := supplied[d]; ok && c.in.Inventory.matches("evidence/"+string(d), raw) {
				s.Blobs[d] = raw
			}
		}
	}
	return s
}

// escalationGrant is the operation-scoped policy decision: the role's policy
// row, or its default row when policy names none. ESCALATE and ANSWER are
// separate grants, so a source holder's grant is not answer authority.
func escalationGrant(p *intent.Policy, role, op string) string {
	ops, ok := p.Roles[role]
	if !ok {
		ops = intent.DefaultRoleMatrix[role]
	}
	if slices.Contains(ops, op) {
		return "ALLOWED"
	}
	return "NOT_ALLOWED"
}

// escalationAdmission audits an OPEN's origin: the named receipt is in the
// journal with the named digest, is a successful claim admission (ADMIT is
// written only by CLAIM and CLAIM_NEXT) of the named attempt and generation,
// and its POST attempt has the named digest. The receipt source is read from
// that POST attempt, never from the request; the current source is the live
// attempt. An origin that cannot be audited returns nil, which the reducer
// refuses MISSING_ADMISSION_CONTEXT. An attempt supervised at its claim or
// since is UNSUPPORTED until its grant adapter is qualified (ESC-V0-001).
func (c leaseContext) escalationAdmission(src ticket.EscalationSource, rec *ticket.Record, f *EscalationFacts) (*EscalationAdmission, *leaseOutcome) {
	a, receiptSha, postSha := c.auditedClaim(src, f)
	if a == nil {
		return nil, nil
	}
	cur := c.st.attempts[a.AttemptID]
	if a.Supervision != nil || (cur != nil && cur.Supervision != nil) {
		out := c.refuse(mutation.OutcomeUnsupported, wire.CodeUnsupported, "a supervised attempt cannot escalate until its grant adapter is qualified")
		return nil, &out
	}
	origin := ticket.EscalationSource{QueueID: c.r.QueueID, TicketID: a.TicketID.Raw, AttemptID: a.AttemptID, Generation: a.Generation, Holder: a.Lease.Holder, AcceptanceRevision: a.TicketRevision, ReceiptSequence: src.ReceiptSequence, ReceiptSha256: receiptSha, PostAttemptSha256: postSha, TicketRecordSha256: a.TicketRecordSha256}
	adm := &EscalationAdmission{OriginState: "AUDITED", ReceiptOperation: "CLAIM", ReceiptOutcome: "OK", ReceiptSource: origin, LeaseState: "INACTIVE", ReservationState: "UNMATCHED"}
	if cur == nil {
		return adm, nil
	}
	current := origin
	current.TicketID, current.Generation, current.AcceptanceRevision, current.TicketRecordSha256 = cur.TicketID.Raw, cur.Generation, rec.AcceptanceRevision, cur.TicketRecordSha256
	current.Holder = ""
	if cur.Lease != nil {
		current.Holder = cur.Lease.Holder
		adm.LeaseExpires = cur.Lease.ExpiresAt
		if cur.Live() {
			adm.LeaseState = "ACTIVE"
		}
	}
	adm.CurrentSource = current
	for _, en := range c.st.reservations.Entries {
		if en.AttemptID == cur.AttemptID && en.Generation == cur.Generation && en.TicketID.Raw == cur.TicketID.Raw && en.TicketRevision == cur.TicketRevision {
			adm.ReservationState = "MATCHED"
		}
	}
	return adm, nil
}

// auditedClaim returns the POST attempt of the named claim receipt with the
// receipt and POST attempt digests, or nil when any binding fails.
func (c leaseContext) auditedClaim(src ticket.EscalationSource, f *EscalationFacts) (*snapshot.Attempt, wire.Digest, wire.Digest) {
	name, e := snapshot.ReceiptName(src.ReceiptSequence.Uint64())
	if e != nil || src.QueueID != c.r.QueueID || f.ClaimReceipt == nil || !c.in.Inventory.matches("receipts/"+name, f.ClaimReceipt) || wire.Sum(f.ClaimReceipt) != src.ReceiptSha256 {
		return nil, "", ""
	}
	rc, e := snapshot.DecodeReceipt(f.ClaimReceipt)
	if e != nil || rc.Kind != "ADMIT" || rc.Outcome != mutation.OutcomeCompleted || rc.Seq != src.ReceiptSequence || rc.AttemptID == nil || *rc.AttemptID != src.AttemptID || rc.Generation == nil || *rc.Generation != src.Generation {
		return nil, "", ""
	}
	for _, p := range rc.Post {
		if p.Path != attemptPath(src.AttemptID) || p.Sha256 == nil {
			continue
		}
		var raw []byte
		switch {
		case p.Record != nil:
			raw = wire.EncodeFile(*p.Record)
		case p.BlobSha256 != nil && c.in.Inventory.matches("evidence/"+string(*p.BlobSha256), f.Blobs[*p.BlobSha256]):
			raw = f.Blobs[*p.BlobSha256]
		}
		if raw == nil || wire.Sum(raw) != *p.Sha256 {
			return nil, "", ""
		}
		a, e := snapshot.DecodeAttempt(raw)
		if e != nil || a.AttemptID != src.AttemptID || a.Generation != src.Generation || a.Lease == nil {
			return nil, "", ""
		}
		return a, wire.Sum(f.ClaimReceipt), *p.Sha256
	}
	return nil, "", ""
}

// blockedRelation validates an optional same-queue blocked-by relation: the
// ticket exists in this queue and the gate, when named, is a policy gate. It
// creates no dependency (ESC-V0-006).
func (c leaseContext) blockedRelation(b *ticket.EscalationBlockedBy) string {
	if b == nil {
		return ""
	}
	if _, ok := c.st.tickets.Get(b.TicketID); !ok {
		return "UNKNOWN"
	}
	if b.Gate != "" && !c.st.policy.GateIDs()[b.Gate] {
		return "UNKNOWN"
	}
	return "VALIDATED"
}

// escalationRefused maps a typed reducer refusal to the closed outcome and
// code vocabulary. The typed code stays in Detail and Result.Escalation, so
// AMBIGUOUS_OPEN_QUESTIONS keeps its request IDs.
func (c leaseContext) escalationRefused(e error) leaseOutcome {
	var r *EscalationRefusal
	if !errors.As(e, &r) {
		return c.fail(e)
	}
	outcome, code := escalationOutcome(r.Code)
	res := refused(c.r.RequestID, outcome, code, r.Error())
	res.Escalation = r
	return leaseOutcome{result: &res}
}

func escalationOutcome(code string) (string, string) {
	switch code {
	case "ACTOR_BINDING", "POLICY_NOT_ALLOWED":
		return mutation.OutcomeUnauthorized, ""
	case "STALE_TICKET_CAS", "STALE_QUESTION_CAS":
		return mutation.OutcomeRevisionConflict, wire.CodeStaleTicket
	case "STALE_ADMISSION", "EXPIRED_ADMISSION", "SOURCE_HOLDER_OR_ACCEPTANCE":
		return mutation.OutcomeRevisionConflict, wire.CodeFenced
	case "MISSING_ADMISSION_CONTEXT", "MISSING_EVIDENCE":
		return mutation.OutcomeBlocked, wire.CodeMissingEvidence
	case "BLOCKED_RELATION_UNKNOWN":
		return mutation.OutcomeBlocked, wire.CodeDependencyMissing
	case "REQUEST_ID_CONFLICT":
		return mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict
	case "JOURNAL_FORKED":
		return mutation.OutcomeBlocked, wire.CodeJournalForked
	case "CAPACITY_EXCEEDED", "OPEN_CAPACITY", "GUIDANCE_CAPACITY", "REVISION_OVERFLOW", "TICKET_REVISION_OVERFLOW":
		return mutation.OutcomeCapacityExhausted, wire.CodeLimitExceeded
	}
	return mutation.OutcomeValidationFailed, wire.CodeMalformed
}

// EscalationActorRefusal is the actor binding check a writer runs before its
// request-index replay: the request digest covers the invoking actor, so
// without it a committed request retried by another actor would report
// REQUEST_ID_CONFLICT instead of ACTOR_BINDING (ESC-V0-001). An undecodable
// request returns nil; the model refuses it.
func EscalationActorRefusal(requestID string, raw []byte, actor mutation.Binding) *Result {
	req, e := ticket.DecodeEscalationRequest(raw)
	if e != nil || (req.Actor == actor.ID && req.ActorRole == actor.Role) {
		return nil
	}
	c := leaseContext{r: Request{RequestID: requestID}}
	return c.escalationRefused(escalationFailure("ACTOR_BINDING")).result
}
