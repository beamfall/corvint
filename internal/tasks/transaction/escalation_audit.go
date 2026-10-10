package transaction

import (
	"bytes"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// EscalationReceiptAudit binds every question reference change to the
// receipt that records it (ESC-V0-010, the material branch). Receipts are
// folded in sequence order; a ticket post whose escalation references
// differ from the folded predecessor must be the one ticket of a completed
// TRANSITION receipt whose request entry is the typed lease request's
// digest, rebuilt from the posted events' original request and this
// receipt's actor, and that receipt posts nothing but its request entry,
// that ticket and the replayed events. The actor's role must hold the
// verb's grant in the policy the history had posted. An OPEN's source must
// be exactly a folded completed ADMIT receipt, and, as the writer observes
// it, the folded attempt must still be that unsupervised generation and
// holder, live, unexpired at this receipt's time and matched by the folded
// reservations. The posted events must then be byte for byte the ones the
// pure reducer computes from the audited predecessor, its retained origins
// and heads, and this receipt's time, and the posted ticket the record the
// writer's finalizer derives from them. A blocked-by relation is not
// re-observed. A dropped reference is refused. An event blob no reference
// change binds installs nothing, since every referenced event is replayed,
// so it stays opaque evidence (a gate may capture identical bytes). It is
// pure: blob returns retained evidence
// bytes by digest. It keeps the latest ticket, attempt and reservation
// posts, so its memory is that of those projections.
type EscalationReceiptAudit struct {
	tickets map[string]escalationTicket
	admits  map[uint64]escalationAdmit
	// attempts is the latest folded attempt per ID, and reservations the
	// latest reservations.json post (Sha256 nil when absent).
	attempts     map[string]escalationAttempt
	reservations snapshot.PostEntry
	// policy is the newest retained intent/policy.json; policyErr is why it
	// is unavailable (never posted, unreadable or undecodable).
	policy    *intent.Policy
	policyErr string
}

type escalationTicket struct {
	post snapshot.PostEntry
	refs []byte // canonical references, nil when the record has none
}

// escalationAttempt is what an OPEN's currency check reads from an attempt.
type escalationAttempt struct {
	source         ticket.EscalationSource // Holder is empty when unleased
	ticketRevision wire.Count
	expires        wire.Timestamp
	live           bool
	supervised     bool
}

type escalationAdmit struct {
	source     ticket.EscalationSource
	supervised bool
}

func (a *EscalationReceiptAudit) fail(rc *snapshot.Receipt, f string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, "receipts/"+string(rc.Seq), "escalation binding: "+f, args...)
}

// Step folds one validated receipt whose bytes hash to sum.
func (a *EscalationReceiptAudit) Step(rc *snapshot.Receipt, sum wire.Digest, blob ExternalReviewBlob) error {
	return a.StepPosts(rc, sum, blob, nil)
}

// StepPosts is Step reading ticket posts through memo, which a fold shares
// with its review step; nil reads them afresh.
func (a *EscalationReceiptAudit) StepPosts(rc *snapshot.Receipt, sum wire.Digest, blob ExternalReviewBlob, memo *TicketPosts) error {
	if a.tickets == nil {
		a.tickets = map[string]escalationTicket{}
		a.admits = map[uint64]escalationAdmit{}
		a.attempts = map[string]escalationAttempt{}
		a.policyErr = "no policy was posted"
	}
	tickets := 0
	for _, p := range rc.Post {
		if strings.HasPrefix(p.Path, "intent/tickets/") {
			tickets++
		}
	}
	for i, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "intent/tickets/") {
			continue
		}
		prior, known := a.tickets[p.Path]
		if p.Sha256 == nil {
			if prior.refs != nil {
				return a.fail(rc, "%s dropped its question references", p.Path)
			}
			delete(a.tickets, p.Path)
			continue
		}
		post := memo.post(rc, i, blob)
		if post.rawErr != nil {
			return a.fail(rc, "%s post: %v", p.Path, post.rawErr)
		}
		if post.decErr != nil {
			return a.fail(rc, "%s post is not a ticket record: %v", p.Path, post.decErr)
		}
		raw, rec := post.raw, post.rec
		var err error
		var refs []byte
		if rec.Escalations != nil {
			if refs, err = ticket.EncodeEscalationRefs(*rec.Escalations); err != nil {
				return a.fail(rc, "%s question references: %v", p.Path, err)
			}
		}
		if !bytes.Equal(refs, prior.refs) {
			if !known || rc.Kind != "TRANSITION" || rc.Outcome != mutation.OutcomeCompleted || rc.RequestID == nil || tickets != 1 {
				return a.fail(rc, "%s changed question references outside one escalation transition", p.Path)
			}
			if err := a.bind(rc, prior.post, p.Path, rec, raw, blob); err != nil {
				return err
			}
		}
		a.tickets[p.Path] = escalationTicket{post: p, refs: refs}
	}
	if err := a.admit(rc, sum, blob); err != nil {
		return err
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

// admit retains the source a completed ADMIT receipt grants, read from its
// POST attempt exactly as the writer's admission audit reads it, and folds
// the latest attempt and reservation posts.
func (a *EscalationReceiptAudit) admit(rc *snapshot.Receipt, sum wire.Digest, blob ExternalReviewBlob) error {
	admitted := rc.Kind == "ADMIT" && rc.Outcome == mutation.OutcomeCompleted && rc.AttemptID != nil && rc.Generation != nil
	for _, p := range rc.Post {
		if p.Path == "reservations.json" {
			a.reservations = p
		}
		if !strings.HasPrefix(p.Path, "attempts/") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(p.Path, "attempts/"), ".json")
		delete(a.attempts, id)
		if p.Sha256 == nil {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			return a.fail(rc, "attempt post %s: %v", p.Path, err)
		}
		at, err := snapshot.DecodeAttempt(raw)
		if err != nil {
			continue
		}
		cur := escalationAttempt{ticketRevision: at.TicketRevision, live: at.Live(), supervised: at.Supervision != nil,
			source: ticket.EscalationSource{TicketID: at.TicketID.Raw, AttemptID: at.AttemptID, Generation: at.Generation, TicketRecordSha256: at.TicketRecordSha256}}
		if at.Lease != nil {
			cur.source.Holder, cur.expires = at.Lease.Holder, at.Lease.ExpiresAt
		}
		a.attempts[id] = cur
		if !admitted || p.Path != attemptPath(*rc.AttemptID) || at.AttemptID != *rc.AttemptID || at.Generation != *rc.Generation || at.Lease == nil {
			continue
		}
		a.admits[rc.Seq.Uint64()] = escalationAdmit{supervised: at.Supervision != nil, source: ticket.EscalationSource{
			QueueID: at.TicketID.QueueID(), TicketID: at.TicketID.Raw, AttemptID: at.AttemptID, Generation: at.Generation, Holder: at.Lease.Holder,
			AcceptanceRevision: at.TicketRevision, ReceiptSequence: rc.Seq, ReceiptSha256: sum, PostAttemptSha256: *p.Sha256, TicketRecordSha256: at.TicketRecordSha256}}
	}
	return nil
}

// bind replays one escalation transition: pre is the folded predecessor post
// of rec's path and raw is rec's posted bytes.
func (a *EscalationReceiptAudit) bind(rc *snapshot.Receipt, prePost snapshot.PostEntry, path string, rec *ticket.Record, raw []byte, blob ExternalReviewBlob) error {
	preRaw, err := externalPostBytes(prePost, blob)
	if err != nil {
		return a.fail(rc, "preceding ticket post: %v", err)
	}
	pre, err := ticket.Decode(preRaw)
	if err != nil {
		return a.fail(rc, "preceding ticket post is not a ticket record: %v", err)
	}
	posted := map[wire.Digest][]byte{}
	var first []byte
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "evidence/") || p.Sha256 == nil {
			continue
		}
		ev, ok := blob(*p.Sha256)
		if !ok || wire.Sum(ev) != *p.Sha256 || p.Path != "evidence/"+string(*p.Sha256) {
			return a.fail(rc, "event %s is absent or differs from its digest", *p.Sha256)
		}
		posted[*p.Sha256] = ev
		if first == nil {
			first = ev
		}
	}
	if first == nil {
		return a.fail(rc, "the transition posts no escalation event")
	}
	ev, err := ticket.DecodeEscalationEvent(first)
	if err != nil {
		return a.fail(rc, "posted event: %v", err)
	}
	r := ev.OriginalRequest
	queue := rec.TicketID.QueueID()
	if r.RequestID != *rc.RequestID || r.Actor != rc.ActorID || r.ActorRole != rc.ActorRole || r.QueueID != queue || r.TicketID != rec.TicketID.Raw {
		return a.fail(rc, "the event's request does not name this receipt's request, actor or ticket")
	}
	request, err := ticket.EncodeEscalationRequest(r)
	if err != nil {
		return a.fail(rc, "the event's request: %v", err)
	}
	verb := LeaseAnswer
	if r.Operation == "OPEN" {
		verb = LeaseEscalate
	}
	actor := mutation.Binding{ID: rc.ActorID, Role: rc.ActorRole}
	d, err := Digest(Request{Operation: Lease, QueueID: queue, RequestID: r.RequestID, Actor: actor, Lease: &LeaseRequest{Verb: verb, Evidence: string(wire.Sum(request))}})
	if err != nil {
		return a.fail(rc, "lease request digest: %v", err)
	}
	if err := a.requestEntry(rc, r.RequestID, d, blob); err != nil {
		return err
	}
	// The request path is valid: requestEntry found its entry.
	requestPath, _ := snapshot.RequestPath(r.RequestID)
	for _, p := range rc.Post {
		if p.Path != path && p.Path != requestPath && !strings.HasPrefix(p.Path, "evidence/") {
			return a.fail(rc, "the transition posts %s", p.Path)
		}
	}
	if a.policy == nil {
		return a.fail(rc, "the grant cannot be recovered: %s", a.policyErr)
	}
	if escalationGrant(a.policy, rc.ActorRole, verb) != "ALLOWED" {
		return a.fail(rc, "role %s holds no %s grant in the retained policy", rc.ActorRole, verb)
	}
	if r.Operation == "OPEN" {
		adm, ok := a.admits[r.Open.Source.ReceiptSequence.Uint64()]
		if !ok || adm.source != r.Open.Source {
			return a.fail(rc, "the question's source is not a retained claim admission")
		}
		cur, ok := a.attempts[adm.source.AttemptID]
		if adm.supervised || cur.supervised {
			return a.fail(rc, "the question's source attempt was supervised")
		}
		if err := a.current(rc, adm.source, cur, ok, pre.AcceptanceRevision, blob); err != nil {
			return err
		}
	}
	s := EscalationSnapshot{QueueID: queue, TicketID: pre.TicketID.Raw, TicketRevision: pre.Revision, AcceptanceRevision: pre.AcceptanceRevision, Refs: pre.Escalations, Blobs: map[wire.Digest][]byte{}}
	if pre.Escalations != nil {
		for _, en := range pre.Escalations.Entries {
			for _, h := range []wire.Digest{en.OriginSha256, en.HeadSha256} {
				if b, ok := blob(h); ok {
					s.Blobs[h] = b
				}
			}
		}
	}
	prop, err := computeEscalation(r, s, rc.RecordedAt)
	if err != nil {
		return a.fail(rc, "the transition does not replay: %v", err)
	}
	if len(prop.Events) != len(posted) {
		return a.fail(rc, "the posted events are not the replayed events")
	}
	for _, e := range prop.Events {
		if !bytes.Equal(posted[e.Sha256], e.Bytes) {
			return a.fail(rc, "the posted events are not the replayed events")
		}
	}
	inv, err := ticket.NewInventory(mustQueue(queue), []*ticket.Record{pre})
	if err != nil {
		return a.fail(rc, "replay inventory: %v", err)
	}
	ctx := mutation.Context{Binding: actor, Queue: &intent.Queue{QueueID: mustQueue(queue)}, Policy: a.policy, Inventory: inv, Requests: absentIndex{}, Now: rc.RecordedAt}
	post, err := mutation.SetEscalations(ctx, pre, prop.Refs)
	if err != nil {
		return a.fail(rc, "the ticket does not replay: %v", err)
	}
	if !bytes.Equal(wire.EncodeFile(post.Value()), raw) {
		return a.fail(rc, "the posted ticket is not the replayed record")
	}
	return nil
}

// current requires the folded attempt to be the source as the writer
// observes it: the same generation, holder, ticket record and acceptance,
// live, unexpired at this receipt's time, and matched by the folded
// reservations.
func (a *EscalationReceiptAudit) current(rc *snapshot.Receipt, src ticket.EscalationSource, cur escalationAttempt, ok bool, acceptance wire.Count, blob ExternalReviewBlob) error {
	want := cur.source
	want.QueueID, want.AcceptanceRevision, want.ReceiptSequence, want.ReceiptSha256, want.PostAttemptSha256 = src.QueueID, acceptance, src.ReceiptSequence, src.ReceiptSha256, src.PostAttemptSha256
	if !ok || !cur.live || want != src {
		return a.fail(rc, "the question's source is not the current admission")
	}
	if cur.expires <= rc.RecordedAt {
		return a.fail(rc, "the question's source lease had expired")
	}
	if a.reservations.Sha256 != nil {
		raw, err := externalPostBytes(a.reservations, blob)
		if err != nil {
			return a.fail(rc, "reservations post: %v", err)
		}
		set, err := snapshot.DecodeReservations(raw)
		if err != nil {
			return a.fail(rc, "reservations post: %v", err)
		}
		for _, en := range set.Entries {
			if en.AttemptID == src.AttemptID && en.Generation == src.Generation && en.TicketID.Raw == src.TicketID && en.TicketRevision == cur.ticketRevision {
				return nil
			}
		}
	}
	return a.fail(rc, "the question's source holds no matching reservation")
}

// requestEntry requires the receipt's request entry to bind d at its own
// sequence.
func (a *EscalationReceiptAudit) requestEntry(rc *snapshot.Receipt, id string, d wire.Digest, blob ExternalReviewBlob) error {
	path, err := snapshot.RequestPath(id)
	if err != nil {
		return a.fail(rc, "request path: %v", err)
	}
	for _, p := range rc.Post {
		if p.Path != path || p.Sha256 == nil {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			return a.fail(rc, "request entry: %v", err)
		}
		entry, err := snapshot.DecodeRequest(raw)
		if err != nil || entry.Seq != rc.Seq || entry.Entry.MutationSha256 != d {
			return a.fail(rc, "the request entry does not bind the typed request")
		}
		return nil
	}
	return a.fail(rc, "the transition posts no request entry")
}
