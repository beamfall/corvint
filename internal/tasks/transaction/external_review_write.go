package transaction

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// importExternalReviews keeps ADOPT/import from minting review references:
// an imported record carries exactly the canonical pre map (ERG-V0-004).
func importExternalReviews(post, pre *ticket.Record, where string) error {
	var want map[string]ticket.ExternalReviewRef
	if pre != nil {
		want = pre.ExternalReviews
	}
	if !wire.Equal(ticket.ExternalReviewsValue(want), ticket.ExternalReviewsValue(post.ExternalReviews)) {
		return wire.Errorf(wire.CodeMalformed, where+"/externalReviews", "an imported record cannot add, rewrite or drop external review references")
	}
	return nil
}

func externalRefOf(r ticket.ExternalReviewRef) *snapshot.ExternalReviewRef {
	return &snapshot.ExternalReviewRef{Generation: r.Generation, Revision: r.Revision, Head: r.Head}
}

// externalSubjectCurrent reports whether a verified BUILT subject is still
// the ticket's current author candidate: the durable submission history
// holds no later author-stage submission of the ticket (ERG-V0-006). The
// ticket's one attempt record is not consulted: a later claim of the ticket,
// such as a reviewer's review-stage lease or the author's repair lease, takes
// a new generation without submitting, and that changes neither the subject
// nor its candidate (2026-10-05, ticket V1-0701).
func externalSubjectCurrent(superseded bool) bool {
	return !superseded
}

// ExternalSubmissionHistory is the store's scan of the receipts after a
// review subject up to the audited head, each linked to its predecessor and
// the last to the head digest: every attempt submission they post, inline or
// blob-backed (ERG-V0-006). The writer reads it once per request, so its
// cost is O(receipts after the subject).
type ExternalSubmissionHistory struct {
	Built []ExternalBuilt
	// Artifacts are the artifact links the same receipts post (ERG-V0-002),
	// and Artifact the retained evidence/ bytes of an EVIDENCE candidate
	// (nil when the request names none or they are absent).
	Artifacts []ExternalArtifact
	Artifact  []byte
}

// externalReviewSubject verifies the request's subject against the supplied
// BUILT transition receipt (ERG-V0-005): the receipt hashes to the subject
// digest, is linked to the audited chain by the head digest or its
// successor's prev, posts the subject attempt record inline at that digest,
// and that record is an author-stage BUILT attempt with a candidate tree.
// It returns the candidate the subject proves, or a refusal detail.
func externalReviewSubject(in Input, st inputState, q *snapshot.ExternalReviewRequest, def *intent.ExternalReviewDefinition) (snapshot.ExternalReviewCandidate, string) {
	none := snapshot.ExternalReviewCandidate{}
	s := q.Subject
	if st.head == nil || len(in.ExternalReviewSubject) < 1 || len(in.ExternalReviewSubject) > 2 {
		return none, "subject receipt was not observed"
	}
	raw := in.ExternalReviewSubject[0]
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil || rc.Seq != s.ReceiptSeq || s.ReceiptSeq.Uint64() > st.head.LastSeq.Uint64() || wire.Sum(raw) != s.ReceiptSha256 {
		return none, "subject receipt differs from its sequence or digest"
	}
	if rc.Seq == st.head.LastSeq {
		if len(in.ExternalReviewSubject) != 1 || st.head.LastReceiptSha256 == nil || *st.head.LastReceiptSha256 != s.ReceiptSha256 {
			return none, "subject receipt is not the audited head receipt"
		}
	} else {
		if len(in.ExternalReviewSubject) != 2 {
			return none, "subject receipt successor was not observed"
		}
		next, err := snapshot.DecodeReceipt(in.ExternalReviewSubject[1])
		if err != nil || next.Seq.Uint64() != rc.Seq.Uint64()+1 || next.Prev == nil || *next.Prev != s.ReceiptSha256 {
			return none, "subject receipt is not linked to the audited chain"
		}
	}
	if rc.Kind != "TRANSITION" || (rc.TicketID != nil && rc.TicketID.Raw != q.TicketID) || rc.AttemptID == nil || *rc.AttemptID != s.AttemptID || rc.Generation == nil || *rc.Generation != s.Generation {
		return none, "subject receipt is not a transition of the subject attempt"
	}
	var record []byte
	for _, p := range rc.Post {
		if p.Path == "attempts/"+s.AttemptID+".json" && p.Record != nil && p.Sha256 != nil && *p.Sha256 == s.AttemptSha256 {
			record = wire.EncodeFile(*p.Record)
		}
	}
	if record == nil || wire.Sum(record) != s.AttemptSha256 {
		return none, "subject receipt does not post the subject attempt record"
	}
	a, err := snapshot.DecodeAttempt(record)
	if err != nil || a.TicketID.Raw != q.TicketID || a.Phase != "BUILT" || a.PhaseSinceSeq != s.ReceiptSeq || a.Generation != s.Generation || a.CandidateTreeOid == nil || !externalHas(def.AuthorStages, a.Stage) {
		return none, "subject is not an author-stage BUILT attempt with a candidate tree"
	}
	superseded := in.ExternalReviewLater == nil || ExternalSuperseded(in.ExternalReviewLater.Built, q.TicketID, def.AuthorStages, s.ReceiptSeq)
	if !externalSubjectCurrent(superseded) {
		return none, "subject is no longer the ticket's current author candidate"
	}
	return snapshot.ExternalReviewCandidate{Kind: "TREE", TreeOID: *a.CandidateTreeOid}, ""
}

// externalArtifactStates observes the candidate link and evidence
// references of q against the artifact links posted after the subject
// (ERG-V0-002). tree is the TREE candidate the subject proves. A TREE
// request binds tree; an EVIDENCE request binds its own candidate only when
// a link of the subject attempt, generation and tree names exactly its
// sha256 and bytes and the retained artifact bytes hash to it, and is
// otherwise UNKNOWN. Every evidence reference needs such a link too.
func externalArtifactStates(later *ExternalSubmissionHistory, q *snapshot.ExternalReviewRequest, tree snapshot.ExternalReviewCandidate) (link, evidence string, bound snapshot.ExternalReviewCandidate) {
	link, evidence, bound = "VERIFIED", "VERIFIED", tree
	var links []ExternalArtifact
	if later != nil {
		links = later.Artifacts
	}
	if q.Candidate.Kind == "EVIDENCE" {
		c := q.Candidate
		if later == nil || !externalLinked(links, q.TicketID, q.Subject, tree.TreeOID, c.Sha256, c.Bytes) ||
			wire.Sum(later.Artifact) != c.Sha256 || wire.SizeOf(uint64(len(later.Artifact))) != c.Bytes {
			link = "UNKNOWN"
		} else {
			bound = c
		}
	}
	for _, e := range q.Evidence {
		if !externalLinked(links, q.TicketID, q.Subject, tree.TreeOID, e.Sha256, e.Bytes) {
			evidence = "UNKNOWN"
		}
	}
	return link, evidence, bound
}

func externalLeaseObservation(st inputState, l *snapshot.ExternalReviewLease, now wire.Timestamp) ExternalReviewLeaseObservation {
	if l == nil {
		return ExternalReviewLeaseObservation{State: "ABSENT"}
	}
	a := st.attempts[l.AttemptID]
	if a == nil || a.Lease == nil {
		return ExternalReviewLeaseObservation{State: "UNKNOWN"}
	}
	o := ExternalReviewLeaseObservation{State: "LIVE", TicketID: a.TicketID.Raw, Stage: a.Stage, Lease: snapshot.ExternalReviewLease{AttemptID: a.AttemptID, Holder: a.Lease.Holder, Generation: a.Generation}}
	if !a.Live() || expired(a, now) {
		o.State = "EXPIRED"
	}
	return o
}

func externalReviewRefusal(outcome, code, detail string) *mutation.ExternalReviewPost {
	return &mutation.ExternalReviewPost{Outcome: outcome, Code: code, Detail: "external review: " + detail}
}

// externalReviewPost builds the audited observations for one REVIEW_RECORD or
// REVIEW_RESUBMIT and runs the pure reducer (ERG-V0-009). Every fact comes
// from the audited queue state or the supplied, re-hashed bytes; nothing is
// taken from the request except what the reducer then compares.
func externalReviewPost(r Request, in Input, st inputState, env *mutation.Envelope) *mutation.ExternalReviewPost {
	p, ok := env.Payload.(*mutation.ReviewPayload)
	if !ok || env.TargetID == nil {
		return externalReviewRefusal(mutation.OutcomeValidationFailed, wire.CodeMalformed, "payload is not a review request")
	}
	q, err := snapshot.DecodeExternalReviewRequest(wire.EncodeFile(p.Request))
	if err != nil {
		return externalReviewRefusal(mutation.OutcomeValidationFailed, wire.CodeOf(err), err.Error())
	}
	action := map[string]string{mutation.OpReviewRecord: "RECORD", mutation.OpReviewResubmit: "RESUBMIT"}[env.Operation]
	if q.RequestID != env.RequestID || q.TicketID != env.TargetID.Raw || q.Action != action {
		return externalReviewRefusal(mutation.OutcomeValidationFailed, wire.CodeMalformed, "request identity, target or action differs from its envelope")
	}
	pre, ok := st.tickets.Get(q.TicketID)
	if !ok {
		return externalReviewRefusal(mutation.OutcomeValidationFailed, wire.CodeMalformed, "target does not exist")
	}
	def := st.policy.ExternalReview(q.GateID)
	if def == nil {
		return externalReviewRefusal(mutation.OutcomeValidationFailed, wire.CodeGateUnknown, "policy declares no external review gate "+q.GateID)
	}
	candidate, detail := externalReviewSubject(in, st, q, def)
	if detail != "" {
		return externalReviewRefusal(mutation.OutcomeBlocked, wire.CodeStaleTree, detail)
	}
	link, evidence, bound := externalArtifactStates(in.ExternalReviewLater, q, candidate)
	if link == "VERIFIED" && q.Candidate.Kind == "EVIDENCE" {
		// A VERIFIED EVIDENCE link implies a non-nil history holding the
		// candidate's bytes.
		if detail, blocked := externalAcceptanceCheck(*q, in.ExternalReviewLater.Artifact); blocked {
			return externalReviewRefusal(mutation.OutcomeBlocked, wire.CodeMissingEvidence, detail)
		} else if detail != "" {
			return externalReviewRefusal(mutation.OutcomeValidationFailed, wire.CodeMalformed, detail)
		}
	}
	var current *snapshot.ExternalReviewRef
	if ref, ok := pre.ExternalReviews[q.GateID]; ok {
		current = externalRefOf(ref)
	}
	encoded, err := q.Encode()
	if err != nil {
		return externalReviewRefusal(mutation.OutcomeValidationFailed, wire.CodeOf(err), err.Error())
	}
	o := ExternalReviewObservations{
		Actor:       r.Actor,
		PolicyState: "VERIFIED", SubjectState: "VERIFIED", CandidateLinkState: link, EvidenceState: evidence,
		Binding:      ExternalReviewBinding{GateID: q.GateID, TicketID: q.TicketID, AcceptanceRevision: pre.AcceptanceRevision, DefinitionSha256: def.Sha256, PolicySha256: wire.Sum(st.policy.Raw), Subject: q.Subject, Candidate: bound},
		Policy:       ExternalReviewPolicy{RecorderRoles: def.RecorderRoles, ReviewStages: def.ReviewStages, AuthorStages: def.AuthorStages, RequireReviewerLease: def.RequireReviewerLease},
		Reviewer:     externalLeaseObservation(st, q.ReviewerLease, in.RecordedAt),
		Author:       externalLeaseObservation(st, q.AuthorLease, in.RecordedAt),
		Current:      current,
		CurrentEvent: in.ExternalReviewPriorEvent,
		Context:      ExternalReviewContext{QueueID: env.TargetID.QueueID(), TicketID: q.TicketID, RequestID: q.RequestID, RequestSha256: wire.Sum(encoded), Pre: current},
		ReplayState:  "ABSENT",
		RecordedAt:   in.RecordedAt,
		ReceiptSeq:   wire.SizeOf(st.head.LastSeq.Uint64() + 1),
	}
	t, err := ApplyExternalReview(*q, o)
	if err != nil {
		outcome, code := mutation.OutcomeValidationFailed, wire.CodeMalformed
		msg := err.Error()
		switch {
		case strings.Contains(msg, "CAS"):
			outcome, code = mutation.OutcomeRevisionConflict, ""
		case strings.Contains(msg, "role") || strings.Contains(msg, "lease"):
			outcome, code = mutation.OutcomeUnauthorized, ""
		case strings.Contains(msg, "current acceptance") || strings.Contains(msg, "subject receipt") || strings.Contains(msg, "stale"):
			outcome, code = mutation.OutcomeBlocked, wire.CodeStaleTicket
		case strings.Contains(msg, "capacity"):
			code = wire.CodeLimitExceeded
		case strings.Contains(msg, "required observation"):
			code = wire.CodeMissingEvidence
		}
		return &mutation.ExternalReviewPost{Outcome: outcome, Code: code, Detail: msg}
	}
	if t.Kind != "EVENT" || t.Ref == nil {
		return externalReviewRefusal(mutation.OutcomeValidationFailed, wire.CodeMalformed, "reducer produced no event")
	}
	return &mutation.ExternalReviewPost{GateID: q.GateID, Ref: ticket.ExternalReviewRef{Generation: t.Ref.Generation, Revision: t.Ref.Revision, Head: t.Ref.Head}, Event: t.Event}
}

// ExternalReviewCurrentBindings derives each declared gate's current binding
// for the read side (ERG-V0-009 workState): acceptance revision and
// definition from the ticket and policy, and the head event's subject only
// while it is still the current author candidate; a superseded subject's
// binding cannot equal the event's, so the view is STALE. An undeclared gate
// has no binding and reads UNKNOWN. superseded answers from the durable
// submission history (ExternalReviewReceiptAudit). A TREE or EVIDENCE
// candidate is taken from the head event: the receipt fold that every
// caller runs first admitted it only against the subject's BUILT tree or a
// journal-backed artifact link of that subject, and both are immutable
// history, so the candidate changes only with the subject.
func ExternalReviewCurrentBindings(rec *ticket.Record, policy *intent.Policy, blob ExternalReviewBlob, superseded func(ticketID string, stages []string, seq wire.Size) bool) map[string]*ExternalReviewBinding {
	out := map[string]*ExternalReviewBinding{}
	if rec == nil || policy == nil {
		return out
	}
	for gate, ref := range rec.ExternalReviews {
		def := policy.ExternalReview(gate)
		raw, ok := blob(ref.Head)
		if def == nil || !ok {
			continue
		}
		e, err := snapshot.CanonicalExternalReviewEvent(raw)
		if err != nil {
			continue
		}
		b := &ExternalReviewBinding{GateID: gate, TicketID: rec.TicketID.Raw, AcceptanceRevision: rec.AcceptanceRevision, DefinitionSha256: def.Sha256, PolicySha256: wire.Sum(policy.Raw), Subject: e.Request.Subject, Candidate: e.Request.Candidate}
		if superseded == nil || !externalSubjectCurrent(superseded(rec.TicketID.Raw, def.AuthorStages, e.Request.Subject.ReceiptSeq)) {
			b.Subject.ReceiptSha256 = ""
		}
		out[gate] = b
	}
	return out
}

// reviewMutation reports a Mutate REVIEW_* request, whose leases and subject
// are judged against the complete audited attempt inventory.
func reviewMutation(r Request) bool {
	if r.Operation != Mutate {
		return false
	}
	env, err := mutation.Decode(r.Envelope)
	return err == nil && mutation.IsReviewOperation(env.Operation)
}
