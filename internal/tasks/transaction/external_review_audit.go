package transaction

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ExternalBuilt is one fresh entry of an attempt into BUILT, read from the
// receipt that posts the attempt record inline: the durable submission
// history that decides subject currency (ERG-V0-006). It does not depend on
// the attempt's present phase, so a newer submission that later leaves BUILT
// still supersedes an older subject.
type ExternalBuilt struct {
	TicketID, Stage, AttemptID string
	Seq                        wire.Size
	AttemptSha256              wire.Digest
}

// ExternalBuiltPosts returns the attempt records rc posts inline whose phase
// enters BUILT at rc's own sequence.
func ExternalBuiltPosts(rc *snapshot.Receipt) ([]ExternalBuilt, error) {
	var out []ExternalBuilt
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "attempts/") || p.Record == nil || p.Sha256 == nil || p.Record.Obj == nil {
			continue
		}
		phase, _ := p.Record.Obj.Get("phase")
		since, _ := p.Record.Obj.Get("phaseSinceSeq")
		if phase.Str != "BUILT" || since.Str != string(rc.Seq) {
			continue
		}
		a, err := snapshot.DecodeAttempt(wire.EncodeFile(*p.Record))
		if err != nil {
			return nil, err
		}
		out = append(out, ExternalBuilt{TicketID: a.TicketID.Raw, Stage: a.Stage, AttemptID: a.AttemptID, Seq: rc.Seq, AttemptSha256: *p.Sha256})
	}
	return out, nil
}

// ExternalSuperseded reports whether built names a submission of ticketID in
// one of stages after seq.
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
// posts exactly that head event. The event must name a retained BUILT
// submission of the ticket that no later same-stage submission superseded,
// and it must be exactly the event the pure transition produces from the
// audited preceding reference, its head event and this receipt's actor,
// time and sequence: counters, generation, predecessor, prior RETURN, trust
// source and lease holder are all replayed. Historical policy, lease
// liveness and author stages are not re-observed. A dropped reference is
// refused. It is pure: blob returns retained evidence bytes by digest.
type ExternalReviewReceiptAudit struct {
	refs     map[string]map[string]ticket.ExternalReviewRef
	receipts map[uint64]externalSubmission
	built    []ExternalBuilt
}

type externalSubmission struct {
	sum   wire.Digest
	posts []ExternalBuilt
}

func (a *ExternalReviewReceiptAudit) fail(rc *snapshot.Receipt, f string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, "receipts/"+string(rc.Seq), "external review binding: "+f, args...)
}

// Superseded reports whether the folded history holds a submission of
// ticketID in one of stages after seq.
// A nil audit observed no history, so every subject reads as superseded.
func (a *ExternalReviewReceiptAudit) Superseded(ticketID string, stages []string, seq wire.Size) bool {
	if a == nil {
		return true
	}
	return ExternalSuperseded(a.built, ticketID, stages, seq)
}

// Step folds one validated receipt whose bytes hash to sum.
func (a *ExternalReviewReceiptAudit) Step(rc *snapshot.Receipt, sum wire.Digest, blob ExternalReviewBlob) error {
	if a.refs == nil {
		a.refs = map[string]map[string]ticket.ExternalReviewRef{}
		a.receipts = map[uint64]externalSubmission{}
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
		var raw []byte
		if p.Record != nil {
			raw = wire.EncodeFile(*p.Record)
		} else if p.BlobSha256 != nil {
			var ok bool
			if raw, ok = blob(*p.BlobSha256); !ok {
				return a.fail(rc, "%s post bytes are absent", path)
			}
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
		stage, ok := a.subjectStage(q)
		if !ok {
			return a.fail(rc, "gate %s subject is not a retained BUILT submission of the ticket", gate)
		}
		if a.Superseded(q.TicketID, []string{stage}, q.Subject.ReceiptSeq) {
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
		if err := ValidateExternalReviewRecovery(ev, *externalRefOf(ref), externalReplayObservations(rc, q, prior, priorEvent)); err != nil {
			return a.fail(rc, "gate %s head event is not the transition from its preceding reference: %v", gate, err)
		}
		a.refs[path] = rec.ExternalReviews
	}
	posts, err := ExternalBuiltPosts(rc)
	if err != nil {
		return a.fail(rc, "attempt post: %v", err)
	}
	if len(posts) != 0 {
		a.receipts[rc.Seq.Uint64()] = externalSubmission{sum: sum, posts: posts}
		a.built = append(a.built, posts...)
	}
	return nil
}

// subjectStage finds the request's subject among the folded submissions and
// returns the submitted attempt's stage.
func (a *ExternalReviewReceiptAudit) subjectStage(q snapshot.ExternalReviewRequest) (string, bool) {
	s := q.Subject
	sub, ok := a.receipts[s.ReceiptSeq.Uint64()]
	if !ok || sub.sum != s.ReceiptSha256 {
		return "", false
	}
	for _, b := range sub.posts {
		if b.AttemptID == s.AttemptID && b.AttemptSha256 == s.AttemptSha256 && b.TicketID == q.TicketID {
			return b.Stage, true
		}
	}
	return "", false
}

// externalReplayObservations reconstructs the observations under which the
// pure transition reproduces a retained event: the receipt's actor, time and
// sequence, the audited preceding reference and its head event, and the
// request's own binding, stages and leases. Facts that history cannot
// re-observe (policy, lease liveness, configured stages) are taken as the
// request states them, so the replay checks the material transition only.
func externalReplayObservations(rc *snapshot.Receipt, q snapshot.ExternalReviewRequest, prior *snapshot.ExternalReviewRef, priorEvent []byte) ExternalReviewObservations {
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
		Binding:      ExternalReviewBinding{GateID: q.GateID, TicketID: q.TicketID, AcceptanceRevision: q.AcceptanceRevision, DefinitionSha256: q.DefinitionSha256, PolicySha256: q.PolicySha256, Subject: q.Subject, Candidate: q.Candidate},
		Policy:       ExternalReviewPolicy{RecorderRoles: []string{rc.ActorRole}, ReviewStages: []string{stage}, AuthorStages: []string{stage}},
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
