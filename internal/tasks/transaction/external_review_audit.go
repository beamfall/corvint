package transaction

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ExternalBuilt is one fresh entry of an attempt into BUILT, read from the
// receipt that posts the attempt record: the durable submission history that
// decides subject currency (ERG-V0-006). It does not depend on the attempt's
// present phase, so a newer submission that later leaves BUILT still
// supersedes an older subject. Generation and Tree are the submitted
// attempt's generation and candidate tree ("" when it has none).
type ExternalBuilt struct {
	TicketID, Stage, AttemptID string
	Seq, Generation            wire.Size
	AttemptSha256              wire.Digest
	Tree                       string
}

// externalPostBytes returns the bytes a post entry names, inline or
// blob-backed, re-hashed against the entry's digest. A blob that is absent
// or differs is an error: currency and binding never skip a post they cannot
// read.
func externalPostBytes(p snapshot.PostEntry, blob ExternalReviewBlob) ([]byte, error) {
	var raw []byte
	switch {
	case p.Record != nil:
		raw = wire.EncodeFile(*p.Record)
	case p.BlobSha256 != nil && blob != nil:
		var ok bool
		if raw, ok = blob(*p.BlobSha256); !ok {
			return nil, wire.Errorf(wire.CodeJournalForked, p.Path, "post bytes evidence/%s are absent", *p.BlobSha256)
		}
	default:
		return nil, wire.Errorf(wire.CodeJournalForked, p.Path, "post bytes are not retained")
	}
	if wire.Sum(raw) != *p.Sha256 {
		return nil, wire.Errorf(wire.CodeJournalForked, p.Path, "post bytes differ from their digest")
	}
	return raw, nil
}

// ExternalBuiltPosts returns the attempt records rc posts, inline or
// blob-backed, whose phase enters BUILT at rc's own sequence. Every attempt
// post is read and fully decoded first: one that cannot be is JOURNAL_FORKED,
// never a skipped entry.
func ExternalBuiltPosts(rc *snapshot.Receipt, blob ExternalReviewBlob) ([]ExternalBuilt, error) {
	var out []ExternalBuilt
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "attempts/") || p.Sha256 == nil {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			return nil, err
		}
		// Every attempt post is decoded before it is filtered, so a
		// hash-consistent record with missing or mistyped fields cannot drop
		// out of the submission history.
		a, err := snapshot.DecodeAttempt(raw)
		if err != nil {
			return nil, wire.Errorf(wire.CodeJournalForked, p.Path, "attempt post is not an attempt record: %v", err)
		}
		if a.Phase != "BUILT" || a.PhaseSinceSeq != rc.Seq {
			continue
		}
		b := ExternalBuilt{TicketID: a.TicketID.Raw, Stage: a.Stage, AttemptID: a.AttemptID, Seq: rc.Seq, Generation: a.Generation, AttemptSha256: *p.Sha256}
		if a.CandidateTreeOid != nil {
			b.Tree = *a.CandidateTreeOid
		}
		out = append(out, b)
	}
	return out, nil
}

// ExternalSuperseded reports whether built names a submission of ticketID in
// one of stages after seq. It scans built once; the receipt fold answers the
// same question from its per-stage index instead.
func ExternalSuperseded(built []ExternalBuilt, ticketID string, stages []string, seq wire.Size) bool {
	for _, b := range built {
		if b.TicketID == ticketID && externalHas(stages, b.Stage) && b.Seq.Uint64() > seq.Uint64() {
			return true
		}
	}
	return false
}

// ExternalReviewReceiptAudit binds every review event to the receipt that
// records it (ERG-V0-009, the operator-notes slot-reuse rule). Receipts are
// folded in sequence order; a ticket post that adds or changes a gate
// reference must be a MUTATION receipt that changes exactly one gate and
// posts exactly that head event. The gate definition and policy digest must
// be those of the policy the history had posted when the event was recorded,
// and the event's subject (attempt, generation, receipt and attempt digests)
// and TREE candidate must be exactly a retained author-stage BUILT
// submission of the ticket that no later submission in one of that
// definition's author stages superseded. The event must then be exactly the
// one the pure transition produces from the audited preceding reference, its
// head event, the retained subject and definition, and this receipt's actor,
// time and sequence: counters, generation, predecessor, prior RETURN, trust
// source and lease holder are all replayed. Lease liveness and lease stages
// are not re-observed. A dropped reference is refused. Each receipt costs
// O(posts + author stages): supersession reads a latest-submission index by
// ticket and stage, never the accumulated history. It is pure: blob returns
// retained evidence bytes by digest.
type ExternalReviewReceiptAudit struct {
	refs     map[string]map[string]ticket.ExternalReviewRef
	receipts map[uint64]externalSubmission
	// latest is the newest BUILT submission sequence by ticket and stage.
	latest map[string]map[string]uint64
	// policy is the newest retained intent/policy.json; policyErr is why it
	// is unavailable (never posted, unreadable or undecodable).
	policy    *intent.Policy
	policyErr string
}

type externalSubmission struct {
	sum   wire.Digest
	posts []ExternalBuilt
}

func (a *ExternalReviewReceiptAudit) fail(rc *snapshot.Receipt, f string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, "receipts/"+string(rc.Seq), "external review binding: "+f, args...)
}

// Superseded reports whether the folded history holds a submission of
// ticketID in one of stages after seq, in O(len(stages)).
// A nil audit observed no history, so every subject reads as superseded.
func (a *ExternalReviewReceiptAudit) Superseded(ticketID string, stages []string, seq wire.Size) bool {
	if a == nil {
		return true
	}
	for _, stage := range stages {
		if a.latest[ticketID][stage] > seq.Uint64() {
			return true
		}
	}
	return false
}

// Step folds one validated receipt whose bytes hash to sum.
func (a *ExternalReviewReceiptAudit) Step(rc *snapshot.Receipt, sum wire.Digest, blob ExternalReviewBlob) error {
	if a.refs == nil {
		a.refs = map[string]map[string]ticket.ExternalReviewRef{}
		a.receipts = map[uint64]externalSubmission{}
		a.latest = map[string]map[string]uint64{}
		a.policyErr = "no policy was posted"
	}
	events := map[wire.Digest]bool{}
	for _, p := range rc.Post {
		if strings.HasPrefix(p.Path, "evidence/") && p.Sha256 != nil {
			events[*p.Sha256] = true
		}
	}
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "intent/tickets/") {
			continue
		}
		path := p.Path
		if p.Sha256 == nil {
			if len(a.refs[path]) != 0 {
				return a.fail(rc, "%s dropped its external review references", path)
			}
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			return a.fail(rc, "%s post: %v", path, err)
		}
		rec, err := ticket.Decode(raw)
		if err != nil {
			return a.fail(rc, "%s post is not a ticket record: %v", path, err)
		}
		old := a.refs[path]
		changed := []string{}
		for gate, ref := range rec.ExternalReviews {
			if prev, ok := old[gate]; !ok || prev != ref {
				changed = append(changed, gate)
			}
		}
		for gate := range old {
			if _, ok := rec.ExternalReviews[gate]; !ok {
				return a.fail(rc, "%s dropped gate %s", path, gate)
			}
		}
		if len(changed) == 0 {
			a.refs[path] = rec.ExternalReviews
			continue
		}
		if rc.Kind != "MUTATION" || len(changed) != 1 || rc.RequestID == nil {
			return a.fail(rc, "%s changed review references outside one review mutation", path)
		}
		gate := changed[0]
		ref := rec.ExternalReviews[gate]
		if !events[ref.Head] {
			return a.fail(rc, "gate %s head event is not posted by its receipt", gate)
		}
		ev, ok := blob(ref.Head)
		if !ok {
			return a.fail(rc, "gate %s head event is absent", gate)
		}
		e, err := externalRefEvent(externalRefOf(ref), ev)
		if err != nil {
			return a.fail(rc, "gate %s head event: %v", gate, err)
		}
		q := e.Request
		if q.TicketID != rec.TicketID.Raw || q.GateID != gate || q.RequestID != *rc.RequestID || q.AcceptanceRevision != rec.AcceptanceRevision {
			return a.fail(rc, "gate %s head event does not record this receipt's transition", gate)
		}
		if a.policy == nil {
			return a.fail(rc, "gate %s definition cannot be recovered: %s", gate, a.policyErr)
		}
		def := a.policy.ExternalReview(gate)
		if def == nil || def.Sha256 != q.DefinitionSha256 || wire.Sum(a.policy.Raw) != q.PolicySha256 {
			return a.fail(rc, "gate %s definition is not the retained policy in effect", gate)
		}
		built, ok := a.subject(q)
		if !ok || !externalHas(def.AuthorStages, built.Stage) {
			return a.fail(rc, "gate %s subject is not a retained author-stage BUILT submission of the ticket", gate)
		}
		if a.Superseded(q.TicketID, def.AuthorStages, built.Seq) {
			return a.fail(rc, "gate %s subject was superseded before this event", gate)
		}
		var prior *snapshot.ExternalReviewRef
		var priorEvent []byte
		if prev, ok := old[gate]; ok {
			prior = externalRefOf(prev)
			if priorEvent, ok = blob(prev.Head); !ok {
				return a.fail(rc, "gate %s preceding head event is absent", gate)
			}
		}
		binding := ExternalReviewBinding{GateID: gate, TicketID: rec.TicketID.Raw, AcceptanceRevision: rec.AcceptanceRevision, DefinitionSha256: def.Sha256, PolicySha256: q.PolicySha256,
			Subject:   snapshot.ExternalReviewSubject{AttemptID: built.AttemptID, Generation: built.Generation, ReceiptSeq: built.Seq, ReceiptSha256: a.receipts[built.Seq.Uint64()].sum, AttemptSha256: built.AttemptSha256},
			Candidate: snapshot.ExternalReviewCandidate{Kind: "TREE", TreeOID: built.Tree}}
		if err := ValidateExternalReviewRecovery(ev, *externalRefOf(ref), externalReplayObservations(rc, q, binding, def, prior, priorEvent)); err != nil {
			return a.fail(rc, "gate %s head event is not the transition from its preceding reference: %v", gate, err)
		}
		a.refs[path] = rec.ExternalReviews
	}
	posts, err := ExternalBuiltPosts(rc, blob)
	if err != nil {
		return a.fail(rc, "attempt post: %v", err)
	}
	if len(posts) != 0 {
		a.receipts[rc.Seq.Uint64()] = externalSubmission{sum: sum, posts: posts}
		for _, b := range posts {
			if a.latest[b.TicketID] == nil {
				a.latest[b.TicketID] = map[string]uint64{}
			}
			if b.Seq.Uint64() > a.latest[b.TicketID][b.Stage] {
				a.latest[b.TicketID][b.Stage] = b.Seq.Uint64()
			}
		}
	}
	for _, p := range rc.Post {
		if p.Path != "intent/policy.json" {
			continue
		}
		a.policy, a.policyErr = nil, "the retained policy was removed"
		if p.Sha256 == nil {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err == nil {
			a.policy, err = intent.DecodePolicy(raw)
		}
		if err != nil {
			a.policy, a.policyErr = nil, "the retained policy is unreadable: "+err.Error()
		}
	}
	return nil
}

// subject finds the request's subject among the folded submissions: the
// retained BUILT receipt at its sequence and digest that posted the same
// attempt, attempt digest, ticket, generation and TREE candidate.
func (a *ExternalReviewReceiptAudit) subject(q snapshot.ExternalReviewRequest) (ExternalBuilt, bool) {
	s := q.Subject
	sub, ok := a.receipts[s.ReceiptSeq.Uint64()]
	if !ok || sub.sum != s.ReceiptSha256 {
		return ExternalBuilt{}, false
	}
	for _, b := range sub.posts {
		if b.AttemptID == s.AttemptID && b.AttemptSha256 == s.AttemptSha256 && b.TicketID == q.TicketID && b.Generation == s.Generation && b.Tree != "" &&
			q.Candidate == (snapshot.ExternalReviewCandidate{Kind: "TREE", TreeOID: b.Tree}) {
			return b, true
		}
	}
	return ExternalBuilt{}, false
}

// externalReplayObservations reconstructs the observations under which the
// pure transition reproduces a retained event: the receipt's actor, time and
// sequence, the audited preceding reference and its head event, the binding
// derived from the retained BUILT submission and policy, and the retained
// definition's recorder roles and reviewer-lease requirement. Lease liveness and lease stages cannot be
// re-observed from history, so the request's leases are taken as live in a
// stage both lease checks admit; the replay checks the material transition.
func externalReplayObservations(rc *snapshot.Receipt, q snapshot.ExternalReviewRequest, binding ExternalReviewBinding, def *intent.ExternalReviewDefinition, prior *snapshot.ExternalReviewRef, priorEvent []byte) ExternalReviewObservations {
	const stage = "historical"
	queue := ""
	if id, err := wire.ParseTicketID("/ticketId", q.TicketID); err == nil {
		queue = id.QueueID()
	}
	digest := wire.Digest("")
	if raw, err := q.Encode(); err == nil {
		digest = wire.Sum(raw)
	}
	lease := func(l *snapshot.ExternalReviewLease) ExternalReviewLeaseObservation {
		if l == nil {
			return ExternalReviewLeaseObservation{State: "ABSENT"}
		}
		return ExternalReviewLeaseObservation{State: "LIVE", TicketID: q.TicketID, Stage: stage, Lease: *l}
	}
	return ExternalReviewObservations{
		Actor:       mutation.Binding{ID: rc.ActorID, Role: rc.ActorRole},
		PolicyState: "VERIFIED", SubjectState: "VERIFIED", CandidateLinkState: "VERIFIED", EvidenceState: "VERIFIED",
		Binding:      binding,
		Policy:       ExternalReviewPolicy{RecorderRoles: def.RecorderRoles, ReviewStages: []string{stage}, AuthorStages: []string{stage}, RequireReviewerLease: def.RequireReviewerLease},
		Reviewer:     lease(q.ReviewerLease),
		Author:       lease(q.AuthorLease),
		Current:      prior,
		CurrentEvent: priorEvent,
		Context:      ExternalReviewContext{QueueID: queue, TicketID: q.TicketID, RequestID: q.RequestID, RequestSha256: digest, Pre: prior},
		ReplayState:  "ABSENT",
		RecordedAt:   rc.RecordedAt,
		ReceiptSeq:   rc.Seq,
	}
}
