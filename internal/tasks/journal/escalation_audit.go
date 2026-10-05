package journal

import (
	"bytes"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// escalationAudit binds one receipt's typed escalation effects (ESC-V0-010,
// the material branch). step feeds it every post with the canonical latest
// it replaces; bind then requires that a receipt carrying escalation events
// is exactly the completed TRANSITION the native writer commits: the target
// ticket replayed from its audited pre-record with only the escalation
// reference, revision and update stamp changed, and every reference change
// explained by one posted event whose chain, identity, actor and time agree
// with the receipt. Every other receipt must leave each walked ticket's
// reference byte-identical. A pending receipt is folded by the same step, so
// the audit that authorizes redo binds it too.
type escalationAudit struct {
	r       Reader
	st      *chain
	rc      *snapshot.Receipt
	tickets []notePost
	events  [][]byte
	policy  bool
	// grants is the audited pre-policy, read when the receipt is bound.
	grants *intent.Policy
	// digest is the receipt's own digest; admitted and admittedSha are a
	// claim admission's POST attempt and its digest.
	digest      wire.Digest
	admitted    []byte
	admittedSha wire.Digest
	// others counts posts that are neither a ticket, the policy, an
	// escalation event nor a request-index afterimage.
	others int
}

// escalationState is each walked ticket path's encoded escalation reference
// (nil when the record carries none). A checkpoint-resumed read starts empty
// and never reads a receipt before its checkpoint (CAL-V0-061).
type escalationState struct {
	refs map[string][]byte
	// sources is each walked question's immutable source, bound when its
	// OPEN event was audited.
	sources map[string]map[string]ticket.EscalationSource
	// from is the first walked receipt sequence; admissions is each walked
	// successful claim admission by sequence. Receipts are bounded by the
	// store scan, so admissions are too.
	from       uint64
	admissions map[uint64]admission
}

// admission is the source a walked ADMIT receipt proves, as the writer's
// auditedClaim reads it from the POST attempt.
type admission struct {
	source     ticket.EscalationSource
	supervised bool
}

func (r Reader) escalationAudit(st *chain, rc *snapshot.Receipt, digest wire.Digest) *escalationAudit {
	return &escalationAudit{r: r, st: st, rc: rc, digest: digest}
}

// isEscalationEvent reports an evidence post that decodes as a typed
// escalation event. Its semantic coverage is earned only through bind, which
// fails the receipt otherwise.
func isEscalationEvent(p string, raw []byte) bool {
	if !strings.HasPrefix(p, "evidence/") {
		return false
	}
	_, err := ticket.DecodeEscalationEvent(raw)
	return err == nil
}

func (a *escalationAudit) observe(j int, p snapshot.PostEntry, prior latest, raw []byte) {
	if a.rc.Kind == "ADMIT" && a.rc.AttemptID != nil && p.Path == "attempts/"+*a.rc.AttemptID+".json" && p.Sha256 != nil && raw != nil {
		a.admitted, a.admittedSha = raw, *p.Sha256
	}
	switch {
	case strings.HasPrefix(p.Path, "intent/tickets/"):
		a.tickets = append(a.tickets, notePost{path: p.Path, index: j, prior: prior, raw: raw})
	case p.Path == "intent/policy.json":
		a.policy = true
	case raw != nil && isEscalationEvent(p.Path, raw):
		a.events = append(a.events, raw)
	case strings.HasPrefix(p.Path, "requests/"):
	default:
		a.others++
	}
}

func escalationForked(where, format string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, where, "escalation: "+format, args...)
}

func encodedRefs(rec *ticket.Record) ([]byte, error) {
	if rec == nil || rec.Escalations == nil {
		return nil, nil
	}
	return ticket.EncodeEscalationRefs(*rec.Escalations)
}

// bind runs after the post loop. target is the decoded receipt-target ticket
// post, reused rather than decoded again.
func (a *escalationAudit) bind(req *snapshot.Request, target *ticket.Record) error {
	if a.st.escalations == nil {
		a.st.escalations = &escalationState{refs: map[string][]byte{}, sources: map[string]map[string]ticket.EscalationSource{}, from: a.rc.Seq.Uint64(), admissions: map[uint64]admission{}}
	}
	a.admit()
	posts := make([][]byte, len(a.tickets))
	for i, t := range a.tickets {
		if t.raw == nil {
			continue
		}
		rec := target
		if rec == nil || t.path != "intent/tickets/"+rec.TicketID.Local+".json" {
			decoded, err := ticket.Decode(t.raw)
			if err != nil {
				return err
			}
			rec = decoded
		}
		enc, err := encodedRefs(rec)
		if err != nil {
			return escalationForked(t.path, "posted reference does not encode: %v", err)
		}
		posts[i] = enc
	}
	if len(a.events) > 0 {
		if err := a.bindEvents(req, target); err != nil {
			return err
		}
	} else {
		for i, t := range a.tickets {
			pre, known := a.st.escalations.refs[t.path]
			if !known && t.prior.seq != "" {
				// A checkpoint-resumed read cannot see a reference posted
				// before its checkpoint; the complete audit binds it.
				continue
			}
			if t.raw != nil && !bytes.Equal(pre, posts[i]) {
				return escalationForked(t.path, "reference changed without its escalation events")
			}
		}
	}
	for i, t := range a.tickets {
		if t.raw == nil {
			delete(a.st.escalations.refs, t.path)
			delete(a.st.escalations.sources, t.path)
			continue
		}
		a.st.escalations.refs[t.path] = posts[i]
	}
	return nil
}

// admit records a completed claim admission (ADMIT is written only by CLAIM
// and CLAIM_NEXT) with the source the writer reads from its POST attempt. An
// attempt that does not decode as the receipt's attempt records nothing, so
// no OPEN can name it.
func (a *escalationAudit) admit() {
	rc := a.rc
	if a.admitted == nil || rc.Outcome != mutation.OutcomeCompleted || rc.Generation == nil {
		return
	}
	at, err := snapshot.DecodeAttempt(a.admitted)
	if err != nil || at.AttemptID != *rc.AttemptID || at.Generation != *rc.Generation || at.Lease == nil {
		return
	}
	a.st.escalations.admissions[rc.Seq.Uint64()] = admission{supervised: at.Supervision != nil, source: ticket.EscalationSource{QueueID: a.r.QueueID.Raw, TicketID: at.TicketID.Raw, AttemptID: at.AttemptID, Generation: at.Generation, Holder: at.Lease.Holder, AcceptanceRevision: at.TicketRevision, ReceiptSequence: rc.Seq, ReceiptSha256: a.digest, PostAttemptSha256: a.admittedSha, TicketRecordSha256: at.TicketRecordSha256}}
}

// bindEvents proves the one escalation receipt shape the native writer
// commits (ESC-V0-010): a completed TRANSITION on one ticket, no attempt,
// posting exactly the target ticket, one or two events and its one request
// afterimage.
func (a *escalationAudit) bindEvents(req *snapshot.Request, target *ticket.Record) error {
	rc := a.rc
	if rc.Kind != "TRANSITION" || rc.RequestID == nil || req == nil || req.Entry.Outcome.Outcome != mutation.OutcomeCompleted || rc.TicketID == nil || rc.AttemptID != nil || target == nil {
		return escalationForked("receipt", "events require one completed TRANSITION request on a ticket")
	}
	if len(a.events) > 2 || len(a.tickets) != 1 || a.tickets[0].raw == nil || a.tickets[0].path != "intent/tickets/"+rc.TicketID.Local+".json" || a.policy || a.others != 0 || len(rc.Post) != 2+len(a.events) {
		return escalationForked("receipt", "events require exactly the events, target ticket and request posts")
	}
	t := a.tickets[0]
	if t.prior.seq == "" || !equalDigest(rc.Pre[t.index].Sha256, t.prior.digest) {
		return escalationForked(t.path, "pre-ticket is not the audited canonical afterimage")
	}
	preRefs, walked := a.st.escalations.refs[t.path]
	if !walked {
		return errCheckpoint(t.path, "escalation pre-state precedes the checkpoint")
	}
	if !a.st.notes.policyWalked {
		return errCheckpoint(t.path, "escalation pre-policy precedes the checkpoint")
	}
	policyRaw, err := a.r.priorPost(a.st.canonical["intent/policy.json"], "intent/policy.json")
	if err != nil {
		return err
	}
	if a.grants, err = intent.DecodePolicy(policyRaw); err != nil {
		return err
	}
	preRaw, err := a.r.priorPost(t.prior, t.path)
	if err != nil {
		return err
	}
	pre, err := ticket.Decode(preRaw)
	if err != nil {
		return err
	}
	if enc, err := encodedRefs(pre); err != nil || !bytes.Equal(enc, preRefs) {
		return escalationForked(t.path, "pre-ticket reference differs from the walked reference")
	}
	// The writer's record pass changes only the reference and the chain
	// fields (mutation.SetEscalations): replay it from the exact pre-record.
	replayed, err := ticket.Decode(preRaw)
	if err != nil {
		return err
	}
	if target.Escalations == nil || pre.Revision.Int() >= wire.MaxCountValue {
		return escalationForked(t.path, "posted ticket carries no reference")
	}
	refs := *target.Escalations
	refs.Entries = append([]ticket.EscalationRef{}, refs.Entries...)
	replayed.Escalations = &refs
	replayed.Revision = wire.CountOf(pre.Revision.Int() + 1)
	prev := pre.FileDigest()
	replayed.PreviousRecordSha256 = &prev
	replayed.UpdatedAt = rc.RecordedAt
	replayed.UpdatedBy = rc.ActorID
	if !bytes.Equal(replayed.Encode(), t.raw) {
		return escalationForked(t.path, "posted ticket differs from the replayed reference write")
	}
	return a.bindRefs(req, pre, target)
}

// escalationLeaseDigest restates transaction.Digest for the writer's LEASE
// request, whose only lease input is the escalation request digest
// (transaction imports journal, so it cannot be called here). The audit of a
// writer-produced history fails if the two preimages drift.
func escalationLeaseDigest(queueID, requestID, actor, role, verb string, evidence wire.Digest) wire.Digest {
	null := wire.Null()
	lease := wire.NewObject()
	lease.Set("verb", wire.String(verb))
	for _, k := range []string{"ticketId", "holder", "leaseMinutes", "branch", "base", "scope"} {
		lease.Set(k, null)
	}
	lease.Set("wholeRepository", wire.Bool(false))
	for _, k := range []string{"attemptId", "generation", "reason"} {
		lease.Set(k, null)
	}
	lease.Set("evidence", wire.String(string(evidence)))
	who := wire.NewObject()
	who.Set("id", wire.String(actor))
	who.Set("role", wire.String(role))
	o := wire.NewObject()
	o.Set("actor", wire.ObjectValue(who))
	o.Set("operation", wire.String(snapshot.StageLease))
	o.Set("queueId", wire.String(queueID))
	o.Set("requestId", wire.String(requestID))
	o.Set("lease", wire.ObjectValue(lease))
	return wire.Sum(wire.EncodeFile(wire.ObjectValue(o)))
}

// bindRefs checks the posted reference against the pre-reference and the
// events with the reducer's arithmetic: one transaction revision, the
// control and work revisions, unchanged untouched entries, one terminal step
// per ANSWER or SUPERSEDE and one new entry per OPEN, each explained by
// exactly one event.
func (a *escalationAudit) bindRefs(req *snapshot.Request, pre, post *ticket.Record) error {
	rc, path := a.rc, "intent/tickets/"+post.TicketID.Local+".json"
	got := post.Escalations
	want := ticket.EscalationRefs{Revision: "1", LastControlTicketRevision: post.Revision, WorkRevision: pre.Revision}
	if pre.Escalations != nil {
		want.Revision = wire.CountOf(pre.Escalations.Revision.Int() + 1)
		if pre.Escalations.LastControlTicketRevision == pre.Revision {
			want.WorkRevision = pre.Escalations.WorkRevision
		}
	}
	if got.Revision != want.Revision || got.LastControlTicketRevision != want.LastControlTicketRevision || got.WorkRevision != want.WorkRevision || post.AcceptanceRevision != pre.AcceptanceRevision {
		return escalationForked(path, "reference revision arithmetic differs from the reducer")
	}
	events := map[string]ticket.EscalationEvent{}
	digests := map[string]wire.Digest{}
	var request *ticket.EscalationRequest
	var requestSha wire.Digest
	for _, raw := range a.events {
		ev, err := ticket.DecodeEscalationEvent(raw)
		if err != nil {
			return err
		}
		q := ev.OriginalRequest
		if ev.QueueID != a.r.QueueID.Raw || ev.TicketID != rc.TicketID.Raw || ev.Actor != rc.ActorID || ev.ActorRole != rc.ActorRole || ev.RecordedAt != rc.RecordedAt || q.RequestID != *rc.RequestID || q.QueueID != ev.QueueID || q.TicketID != ev.TicketID || q.Actor != ev.Actor || q.ActorRole != ev.ActorRole {
			return escalationForked(path, "event %s identity, actor or time differs from the receipt", ev.EscalationID)
		}
		if ev.ResolvedRequestID != ev.EscalationID || ev.ResolvedPreviousRevision.Int() != ev.Revision.Int()-1 {
			return escalationForked(path, "event %s resolution differs from its revision", ev.EscalationID)
		}
		if request != nil && ev.RequestSha256 != requestSha {
			return escalationForked(path, "events carry different requests")
		}
		if _, dup := events[ev.EscalationID]; dup {
			return escalationForked(path, "two events for request %s", ev.EscalationID)
		}
		events[ev.EscalationID] = ev
		digests[ev.EscalationID] = wire.Sum(raw)
		r := ev.OriginalRequest
		request, requestSha = &r, ev.RequestSha256
	}
	// The events must carry the very request the receipt's request entry
	// retains: its canonical digest is the LEASE evidence input.
	verb := map[string]string{"OPEN": "ESCALATE", "ANSWER": "ANSWER"}[request.Operation]
	if enc, err := ticket.EncodeEscalationRequest(*request); err != nil || wire.Sum(enc) != requestSha || verb == "" || (request.Operation == "OPEN") != (request.Open != nil) || (request.Operation == "ANSWER") != (request.Answer != nil) {
		return escalationForked(path, "events carry a request that does not encode to their digest")
	}
	if escalationLeaseDigest(a.r.QueueID.Raw, *rc.RequestID, rc.ActorID, rc.ActorRole, verb, requestSha) != req.Entry.MutationSha256 {
		return escalationForked(path, "events carry a request other than the retained request")
	}
	// The writer's operation-scoped grant (transaction.escalationGrant): the
	// role's policy row, or its default row when policy names none.
	ops, named := a.grants.Roles[rc.ActorRole]
	if !named {
		ops = intent.DefaultRoleMatrix[rc.ActorRole]
	}
	if !slices.Contains(ops, verb) {
		return escalationForked(path, "audited pre-policy does not grant %s to role %s", verb, rc.ActorRole)
	}
	if request.ExpectedTicketRevision != nil && *request.ExpectedTicketRevision != pre.Revision {
		return escalationForked(path, "request's expected ticket revision differs from the audited pre-ticket")
	}
	sources := a.st.escalations.sources[path]
	opened := map[string]ticket.EscalationSource{}
	expected := 1
	if request.Operation == "OPEN" && request.Open.Supersedes != "" {
		expected = 2
	}
	if len(events) != expected {
		return escalationForked(path, "request %s needs %d event(s), receipt posts %d", request.RequestID, expected, len(events))
	}
	before := map[string]ticket.EscalationRef{}
	if pre.Escalations != nil {
		for _, e := range pre.Escalations.Entries {
			before[e.RequestID] = e
		}
	}
	used := 0
	for _, e := range got.Entries {
		old, existed := before[e.RequestID]
		delete(before, e.RequestID)
		ev, changed := events[e.RequestID]
		d := digests[e.RequestID]
		switch {
		case existed && !changed:
			if e != old {
				return escalationForked(path, "request %s changed without its event", e.RequestID)
			}
			continue
		case existed:
			state := map[string]string{"ANSWER": "ANSWERED", "SUPERSEDE": "SUPERSEDED"}[ev.Operation]
			next := old
			next.HeadSha256, next.Revision, next.State = d, ev.Revision, state
			if state == "" || old.State != "OPEN" || old.AcceptanceRevision != pre.AcceptanceRevision || ev.PreviousSha256 == nil || *ev.PreviousSha256 != old.HeadSha256 || ev.QuestionOriginSha256 == nil || *ev.QuestionOriginSha256 != old.OriginSha256 || ev.Revision.Int() != old.Revision.Int()+1 || e != next {
				return escalationForked(path, "request %s terminal step differs from its event chain", e.RequestID)
			}
			origin, known := sources[e.RequestID]
			if !known {
				return errCheckpoint(path, "escalation origin precedes the checkpoint")
			}
			if ev.Source != origin {
				return escalationForked(path, "request %s terminal step changes its immutable source", e.RequestID)
			}
			var selected string
			var expectedRevision wire.Count
			replacement := ""
			if ev.Operation == "ANSWER" {
				if request.Operation != "ANSWER" {
					return escalationForked(path, "request %s answered by a %s request", e.RequestID, request.Operation)
				}
				selected, expectedRevision = request.Answer.RequestID, request.Answer.ExpectedRevision
			} else {
				if request.Operation != "OPEN" || request.Open.Supersedes != e.RequestID || request.Open.Source != origin {
					return escalationForked(path, "request %s superseded by a request that does not name it", e.RequestID)
				}
				selected, expectedRevision, replacement = request.Open.Supersedes, request.Open.ExpectedRevision, request.RequestID
			}
			if selected == "" {
				// Shorthand answers the sole current OPEN question only.
				open := 0
				if pre.Escalations != nil {
					for _, x := range pre.Escalations.Entries {
						if x.State == "OPEN" && x.AcceptanceRevision == pre.AcceptanceRevision {
							open++
						}
					}
				}
				if ev.Operation != "ANSWER" || open != 1 {
					return escalationForked(path, "request %s shorthand answer without exactly one current open question", e.RequestID)
				}
			} else if selected != e.RequestID || expectedRevision != old.Revision {
				return escalationForked(path, "request %s terminal step differs from its request selector", e.RequestID)
			}
			if ev.ReplacementID != replacement {
				return escalationForked(path, "request %s terminal step differs from its request selector", e.RequestID)
			}
		default:
			if !changed || ev.Operation != "OPEN" || ev.Source.QueueID != a.r.QueueID.Raw || ev.Source.TicketID != rc.TicketID.Raw || ev.Source.AcceptanceRevision != pre.AcceptanceRevision || ev.Source.Holder != ev.Actor || ev.Source.ReceiptSequence.Uint64() >= rc.Seq.Uint64() {
				return escalationForked(path, "request %s appeared without its OPEN event", e.RequestID)
			}
			if request.Operation != "OPEN" || e.RequestID != request.RequestID || ev.Revision != "1" || ev.PreviousSha256 != nil || ev.QuestionOriginSha256 != nil || ev.ReplacementID != "" || ev.Source != request.Open.Source {
				return escalationForked(path, "request %s OPEN event differs from its request", e.RequestID)
			}
			if e != (ticket.EscalationRef{RequestID: e.RequestID, OriginSha256: d, HeadSha256: d, Revision: "1", AcceptanceRevision: pre.AcceptanceRevision, Kind: request.Open.Kind, State: "OPEN"}) {
				return escalationForked(path, "request %s entry differs from its OPEN event", e.RequestID)
			}
			if err := a.admittedSource(path, e.RequestID, ev.Source); err != nil {
				return err
			}
			if err := a.currentClaim(path, e.RequestID, ev.Source, ev.RecordedAt); err != nil {
				return err
			}
			if err := a.blockedRelation(path, e.RequestID, request.Open.BlockedBy); err != nil {
				return err
			}
			opened[e.RequestID] = ev.Source
		}
		used++
	}
	if len(before) != 0 {
		return escalationForked(path, "reference dropped a request")
	}
	if used != len(events) {
		return escalationForked(path, "an event names no reference entry")
	}
	if len(opened) > 0 {
		if sources == nil {
			sources = map[string]ticket.EscalationSource{}
			a.st.escalations.sources[path] = sources
		}
		for id, src := range opened {
			sources[id] = src
		}
	}
	return nil
}

// admittedSource proves an OPEN's source is a walked successful claim
// admission, exactly as the writer audited it, of an unsupervised attempt. An
// admission before the first walked receipt falls back to the complete audit.
func (a *escalationAudit) admittedSource(path, id string, src ticket.EscalationSource) error {
	seq := src.ReceiptSequence.Uint64()
	adm, ok := a.st.escalations.admissions[seq]
	switch {
	case !ok && seq < a.st.escalations.from:
		return errCheckpoint(path, "escalation admission precedes the checkpoint")
	case !ok || adm.source != src:
		return escalationForked(path, "request %s source is not a recorded claim admission", id)
	case adm.supervised:
		return escalationForked(path, "request %s source is a supervised claim", id)
	}
	return nil
}

// preRecord reads a path's audited pre-state afterimage, or nil when the path
// is absent or deleted. One posted before the first walked receipt falls back
// to the complete audit.
func (a *escalationAudit) preRecord(path string) ([]byte, error) {
	prior := a.st.canonical[path]
	if prior.seq == "" || prior.digest == nil {
		return nil, nil
	}
	if prior.seq.Uint64() < a.st.escalations.from {
		return nil, errCheckpoint(path, "escalation pre-state precedes the checkpoint")
	}
	return a.r.priorPost(prior, path)
}

// currentClaim restates the writer's fresh-OPEN fence against the audited
// pre-state: the source attempt is live and unsupervised under the source's
// generation, holder and ticket record, its reservation matches, and its
// lease outlives the event.
func (a *escalationAudit) currentClaim(path, id string, src ticket.EscalationSource, at wire.Timestamp) error {
	raw, err := a.preRecord("attempts/" + src.AttemptID + ".json")
	if err != nil {
		return err
	}
	if raw == nil {
		return escalationForked(path, "request %s source attempt is not current", id)
	}
	cur, err := snapshot.DecodeAttempt(raw)
	if err != nil {
		return err
	}
	if cur.Supervision != nil || !cur.Live() || cur.Lease == nil || cur.TicketID.Raw != src.TicketID || cur.Generation != src.Generation || cur.Lease.Holder != src.Holder || cur.TicketRecordSha256 != src.TicketRecordSha256 {
		return escalationForked(path, "request %s source is not the current claim", id)
	}
	if cur.Lease.ExpiresAt <= at {
		return escalationForked(path, "request %s opened after its lease expired", id)
	}
	if raw, err = a.preRecord("reservations.json"); err != nil {
		return err
	}
	if raw != nil {
		set, err := snapshot.DecodeReservations(raw)
		if err != nil {
			return err
		}
		for _, en := range set.Entries {
			if en.AttemptID == cur.AttemptID && en.Generation == cur.Generation && en.TicketID.Raw == cur.TicketID.Raw && en.TicketRevision == cur.TicketRevision {
				return nil
			}
		}
	}
	return escalationForked(path, "request %s source has no matching reservation", id)
}

// blockedRelation restates the writer's BLOCKED_RELATION_UNKNOWN check: a
// named blocked-by ticket exists in the audited pre-state and a named gate is
// a gate of the audited pre-policy.
func (a *escalationAudit) blockedRelation(path, id string, b *ticket.EscalationBlockedBy) error {
	if b == nil {
		return nil
	}
	tid, err := wire.ParseTicketID("/blockedBy/ticketId", b.TicketID)
	if err != nil {
		return err
	}
	if prior := a.st.canonical["intent/tickets/"+tid.Local+".json"]; prior.seq == "" || prior.digest == nil || (b.Gate != "" && !a.grants.GateIDs()[b.Gate]) {
		return escalationForked(path, "request %s names an unknown blocked relation", id)
	}
	return nil
}
