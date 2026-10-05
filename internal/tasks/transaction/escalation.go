package transaction

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// EscalationSnapshot is an explicitly supplied audited ticket view. Blobs must
// contain its immutable origins and heads. No lookup, lock, or audit occurs here.
type EscalationSnapshot struct {
	QueueID, TicketID                  string
	TicketRevision, AcceptanceRevision wire.Count
	Refs                               *ticket.EscalationRefs
	Blobs                              map[wire.Digest][]byte
}

// EscalationAdmission facts must come from the immutable successful claim
// receipt/POST attempt and fresh writer observations, never CLI holder text.
// This pure model checks consistency, not authenticity of supplied observations.
type EscalationAdmission struct {
	OriginState, ReceiptOperation, ReceiptOutcome string
	ReceiptSource                                 ticket.EscalationSource
	CurrentSource                                 ticket.EscalationSource
	LeaseState, ReservationState                  string
	LeaseExpires                                  wire.Timestamp
}

type EscalationBlob struct {
	Sha256 wire.Digest
	Bytes  []byte
}

// EscalationProposal is hypothetical material, not a receipt or admission grant.
// It only contains the typed fields to apply; the native finalizer is unwired.
type EscalationProposal struct {
	BeforeTicketRevision, TicketRevision, AcceptanceRevision wire.Count
	Refs                                                     ticket.EscalationRefs
	Events                                                   []EscalationBlob
	ActorAuthentication, Durability                          string
	Replayed                                                 bool
}

// FOUND requires the original audited preimage and committed material. Recompute
// it before returning; never re-resolve ticket shorthand against the current view.
type EscalationReplay struct {
	State         string // ABSENT, FOUND, or UNKNOWN; the zero value is unknown.
	RequestSha256 wire.Digest
	Before        *EscalationSnapshot
	RecordedAt    wire.Timestamp
	Result        *EscalationProposal
}

type EscalationObservation struct {
	Snapshot             EscalationSnapshot
	Actor, ActorRole     string
	PolicyDecision       string // ALLOWED is an explicit supplied operation grant.
	PolicyOperation      string // The one request operation the grant covers (ESC-V0-004).
	Now                  wire.Timestamp
	Admission            *EscalationAdmission
	BlockedRelationState string // VALIDATED when the request names a blocked relation.
	Replay               EscalationReplay
}

// EscalationRefusal is a coded refusal. RequestIDs, when present, names the
// sorted current questions the caller must choose between (ESC-V0-004).
type EscalationRefusal struct {
	Code       string
	RequestIDs []string
}

func (e *EscalationRefusal) Error() string {
	if len(e.RequestIDs) == 0 {
		return "escalation: " + e.Code
	}
	return fmt.Sprintf("escalation: %s (%s)", e.Code, strings.Join(e.RequestIDs, ", "))
}

func escalationFailure(code string) error { return &EscalationRefusal{Code: code} }

// escalationView decodes each event blob at most once per call. The memo never
// outlives one exported call, so a blob changed between calls is re-verified.
type escalationView struct {
	EscalationSnapshot
	decoded map[wire.Digest]ticket.EscalationEvent
}

func newEscalationView(s EscalationSnapshot) *escalationView {
	return &escalationView{s, make(map[wire.Digest]ticket.EscalationEvent)}
}

func (s *escalationView) record(d wire.Digest) (ticket.EscalationEvent, error) {
	e, ok := s.decoded[d]
	if !ok {
		raw, ok := s.Blobs[d]
		if !ok {
			return ticket.EscalationEvent{}, escalationFailure("MISSING_EVIDENCE")
		}
		if wire.Sum(raw) != d {
			return ticket.EscalationEvent{}, escalationFailure("JOURNAL_FORKED")
		}
		var err error
		if e, err = ticket.DecodeEscalationEvent(raw); err != nil {
			return e, err
		}
		s.decoded[d] = e
	}
	if e.QueueID != s.QueueID || e.TicketID != s.TicketID {
		return e, escalationFailure("EVENT_IDENTITY")
	}
	return e, nil
}

func (s *escalationView) validate() error {
	q, e := wire.ParseQueueID("/queueId", s.QueueID)
	if e != nil {
		return e
	}
	t, e := wire.ParseTicketID("/ticketId", s.TicketID)
	if e != nil {
		return e
	}
	if t.QueueID() != q.Raw {
		return escalationFailure("SNAPSHOT_IDENTITY")
	}
	for _, v := range []wire.Count{s.TicketRevision, s.AcceptanceRevision} {
		if _, e := wire.ParseCount("/revision", string(v)); e != nil {
			return e
		}
		if v.Int() < 1 {
			return escalationFailure("SNAPSHOT_REVISION")
		}
	}
	if s.AcceptanceRevision.Int() > s.TicketRevision.Int() {
		return escalationFailure("ACCEPTANCE_REVISION")
	}
	if s.Refs == nil {
		return nil
	}
	if e := s.Refs.Validate(); e != nil {
		return e
	}
	if s.Refs.LastControlTicketRevision.Int() > s.TicketRevision.Int() {
		return escalationFailure("CONTROL_FROM_FUTURE")
	}
	open := 0
	for _, ref := range s.Refs.Entries {
		origin, e := s.record(ref.OriginSha256)
		if e != nil {
			return e
		}
		head, e := s.record(ref.HeadSha256)
		if e != nil {
			return e
		}
		if origin.Operation != "OPEN" || origin.EscalationID != ref.RequestID || head.EscalationID != ref.RequestID || head.Revision != ref.Revision || head.Source != origin.Source || origin.Source.AcceptanceRevision != ref.AcceptanceRevision || origin.OriginalRequest.Open.Kind != ref.Kind {
			return escalationFailure("REFERENCE_MATERIAL")
		}
		if ref.AcceptanceRevision.Int() > s.AcceptanceRevision.Int() {
			return escalationFailure("FUTURE_ACCEPTANCE")
		}
		switch ref.State {
		case "OPEN":
			if head.Operation != "OPEN" || ref.HeadSha256 != ref.OriginSha256 {
				return escalationFailure("OPEN_HEAD")
			}
			if ref.AcceptanceRevision == s.AcceptanceRevision {
				open++
			}
		case "ANSWERED", "SUPERSEDED":
			want := "ANSWER"
			if ref.State == "SUPERSEDED" {
				want = "SUPERSEDE"
			}
			// This profile never reopens or reanswers a terminal question. Its sole
			// terminal event follows OPEN; larger codec caps reserve no such authority.
			if head.Operation != want || head.Revision != "2" || head.PreviousSha256 == nil || *head.PreviousSha256 != ref.OriginSha256 || head.QuestionOriginSha256 == nil || *head.QuestionOriginSha256 != ref.OriginSha256 {
				return escalationFailure("TERMINAL_CHAIN")
			}
		}
	}
	// Readers enforce the writer's current-acceptance open bound too, so an
	// imported or corrupt reference over it is refused rather than served.
	if open > ticket.EscalationMaxCurrentOpen {
		return escalationFailure("OPEN_CAPACITY")
	}
	// Supersession has two immutable sides in one proposed transaction. Check
	// both directions so a missing replacement cannot masquerade as a closed
	// historical request, even when each event's own hash is internally valid.
	byID := make(map[string]ticket.EscalationRef, len(s.Refs.Entries))
	for _, ref := range s.Refs.Entries {
		byID[ref.RequestID] = ref
	}
	for _, ref := range s.Refs.Entries {
		origin, e := s.record(ref.OriginSha256)
		if e != nil {
			return e
		}
		if prior := origin.OriginalRequest.Open.Supersedes; prior != "" {
			old, ok := byID[prior]
			if !ok || old.State != "SUPERSEDED" {
				return escalationFailure("SUPERSESSION_PAIR_MISSING")
			}
			head, e := s.record(old.HeadSha256)
			if e != nil {
				return e
			}
			if head.ReplacementID != ref.RequestID || head.RequestSha256 != origin.RequestSha256 || head.RecordedAt != origin.RecordedAt || head.Source != origin.Source {
				return escalationFailure("SUPERSESSION_PAIR_MISMATCH")
			}
		}
		if ref.State == "SUPERSEDED" {
			head, e := s.record(ref.HeadSha256)
			if e != nil {
				return e
			}
			replacement, ok := byID[head.ReplacementID]
			if !ok {
				return escalationFailure("SUPERSESSION_PAIR_MISSING")
			}
			next, e := s.record(replacement.OriginSha256)
			if e != nil {
				return e
			}
			if next.OriginalRequest.Open.Supersedes != ref.RequestID || next.RequestSha256 != head.RequestSha256 {
				return escalationFailure("SUPERSESSION_PAIR_MISMATCH")
			}
		}
	}
	return nil
}

// EscalationCapacity checks the declared counts independently of blob material.
// A two-event supersession consumes two event slots but one transaction slot.
// Only OPEN questions of the current acceptance revision count toward the open
// bound: a stale OPEN question can be neither answered nor superseded, so
// counting it would lock the ticket with no recovery route.
func EscalationCapacity(r *ticket.EscalationRefs, acceptance wire.Count, newRequests, newEvents, openDelta int) error {
	if !((newRequests == 1 && newEvents == 1 && openDelta == 1) || (newRequests == 1 && newEvents == 2 && openDelta == 0) || (newRequests == 0 && newEvents == 1 && openDelta == -1)) {
		return escalationFailure("CAPACITY_DELTA")
	}
	if a, e := wire.ParseCount("/acceptanceRevision", string(acceptance)); e != nil {
		return e
	} else if a.Int() < 1 {
		return escalationFailure("SNAPSHOT_REVISION")
	}
	requests, total, open := 0, 0, 0
	if r != nil {
		if e := r.Validate(); e != nil {
			return e
		}
		requests = len(r.Entries)
		if r.Revision.Int() == wire.MaxCountValue {
			return escalationFailure("REVISION_OVERFLOW")
		}
		for _, x := range r.Entries {
			if x.AcceptanceRevision.Int() > acceptance.Int() {
				return escalationFailure("FUTURE_ACCEPTANCE")
			}
			total += int(x.Revision.Int())
			if x.State == "OPEN" && x.AcceptanceRevision == acceptance {
				open++
			}
		}
	}
	if requests+newRequests > 64 || total+newEvents > 4096 || open+openDelta > ticket.EscalationMaxCurrentOpen || open+openDelta < 0 {
		return escalationFailure("CAPACITY_EXCEEDED")
	}
	return nil
}

// ApplyEscalation checks binding and exact replay before fresh state/expiry/CAS.
// Supplied ALLOWED/audited facts remain unverified observations: every proposal
// retains NOT_OBSERVED authentication/durability, and no native state is touched.
func ApplyEscalation(raw []byte, o EscalationObservation) (EscalationProposal, error) {
	r, e := ticket.DecodeEscalationRequest(raw)
	if e != nil {
		return EscalationProposal{}, e
	}
	if o.Actor != r.Actor || o.ActorRole != r.ActorRole {
		return EscalationProposal{}, escalationFailure("ACTOR_BINDING")
	}
	switch o.Replay.State {
	case "FOUND":
		if o.Replay.RequestSha256 != wire.Sum(raw) {
			return EscalationProposal{}, escalationFailure("REQUEST_ID_CONFLICT")
		}
		if o.Replay.Before == nil || o.Replay.Result == nil {
			return EscalationProposal{}, escalationFailure("MISSING_REPLAY_MATERIAL")
		}
		proposed, e := computeEscalation(r, *o.Replay.Before, o.Replay.RecordedAt)
		if e != nil {
			return EscalationProposal{}, e
		}
		if !sameEscalationProposal(proposed, *o.Replay.Result) {
			return EscalationProposal{}, escalationFailure("REPLAY_MATERIAL_MISMATCH")
		}
		proposed.Replayed = true
		return proposed, nil
	case "ABSENT":
	default:
		return EscalationProposal{}, escalationFailure("REPLAY_UNKNOWN")
	}
	// A grant covers one operation, so a source holder's OPEN grant is never
	// answer authority (ESC-V0-004).
	if o.PolicyDecision != "ALLOWED" || o.PolicyOperation != r.Operation {
		return EscalationProposal{}, escalationFailure("POLICY_NOT_ALLOWED")
	}
	if _, e := wire.ParseTimestamp("/now", string(o.Now)); e != nil {
		return EscalationProposal{}, e
	}
	if r.Operation == "OPEN" {
		a := o.Admission
		if a == nil || a.OriginState != "AUDITED" || (a.ReceiptOperation != "CLAIM" && a.ReceiptOperation != "CLAIM_NEXT") || a.ReceiptOutcome != "OK" {
			return EscalationProposal{}, escalationFailure("MISSING_ADMISSION_CONTEXT")
		}
		if a.ReceiptSource != r.Open.Source || a.CurrentSource != r.Open.Source || a.LeaseState != "ACTIVE" || a.ReservationState != "MATCHED" {
			return EscalationProposal{}, escalationFailure("STALE_ADMISSION")
		}
		if _, e := wire.ParseTimestamp("/leaseExpires", string(a.LeaseExpires)); e != nil {
			return EscalationProposal{}, e
		}
		if a.LeaseExpires <= o.Now {
			return EscalationProposal{}, escalationFailure("EXPIRED_ADMISSION")
		}
		if r.Open.BlockedBy != nil && o.BlockedRelationState != "VALIDATED" {
			return EscalationProposal{}, escalationFailure("BLOCKED_RELATION_UNKNOWN")
		}
	}
	return computeEscalation(r, o.Snapshot, o.Now)
}

func sameEscalationProposal(a, b EscalationProposal) bool {
	if a.BeforeTicketRevision != b.BeforeTicketRevision || a.TicketRevision != b.TicketRevision || a.AcceptanceRevision != b.AcceptanceRevision || a.ActorAuthentication != b.ActorAuthentication || a.Durability != b.Durability || b.Replayed || len(a.Events) != len(b.Events) {
		return false
	}
	ar, e := ticket.EncodeEscalationRefs(a.Refs)
	if e != nil {
		return false
	}
	br, e := ticket.EncodeEscalationRefs(b.Refs)
	if e != nil || !bytes.Equal(ar, br) {
		return false
	}
	for i := range a.Events {
		if a.Events[i].Sha256 != b.Events[i].Sha256 || !bytes.Equal(a.Events[i].Bytes, b.Events[i].Bytes) {
			return false
		}
	}
	return true
}

func computeEscalation(r ticket.EscalationRequest, s EscalationSnapshot, now wire.Timestamp) (EscalationProposal, error) {
	fail := func(e error) (EscalationProposal, error) { return EscalationProposal{}, e }
	v := newEscalationView(s)
	if e := v.validate(); e != nil {
		return fail(e)
	}
	if _, e := wire.ParseTimestamp("/recordedAt", string(now)); e != nil {
		return fail(e)
	}
	if r.QueueID != s.QueueID || r.TicketID != s.TicketID {
		return fail(escalationFailure("TARGET_MISMATCH"))
	}
	if r.ExpectedTicketRevision != nil && *r.ExpectedTicketRevision != s.TicketRevision {
		return fail(escalationFailure("STALE_TICKET_CAS"))
	}
	if s.TicketRevision.Int() == wire.MaxCountValue {
		return fail(escalationFailure("TICKET_REVISION_OVERFLOW"))
	}
	refs := ticket.EscalationRefs{Revision: "1", LastControlTicketRevision: wire.CountOf(s.TicketRevision.Int() + 1), WorkRevision: s.TicketRevision, Entries: []ticket.EscalationRef{}}
	if s.Refs != nil {
		refs.Revision = wire.CountOf(s.Refs.Revision.Int() + 1)
		refs.Entries = append(refs.Entries, s.Refs.Entries...)
		if s.Refs.LastControlTicketRevision == s.TicketRevision {
			refs.WorkRevision = s.Refs.WorkRevision
		}
	}
	result := EscalationProposal{BeforeTicketRevision: s.TicketRevision, TicketRevision: refs.LastControlTicketRevision, AcceptanceRevision: s.AcceptanceRevision, ActorAuthentication: NotObserved, Durability: NotObserved, Events: []EscalationBlob{}}
	requestBytes, e := ticket.EncodeEscalationRequest(r)
	if e != nil {
		return fail(e)
	}
	event := func(id string, rev wire.Count, op string, source ticket.EscalationSource) ticket.EscalationEvent {
		return ticket.EscalationEvent{Profile: ticket.EscalationEventProfile, QueueID: r.QueueID, TicketID: r.TicketID, EscalationID: id, Revision: rev, Operation: op, Source: source, Actor: r.Actor, ActorRole: r.ActorRole, RecordedAt: now, OriginalRequest: r, RequestSha256: wire.Sum(requestBytes), ResolvedRequestID: id, ResolvedPreviousRevision: wire.CountOf(rev.Int() - 1)}
	}
	emit := func(ev ticket.EscalationEvent) (wire.Digest, error) {
		b, e := ticket.EncodeEscalationEvent(ev)
		if e != nil {
			return "", e
		}
		d := wire.Sum(b)
		result.Events = append(result.Events, EscalationBlob{d, b})
		return d, nil
	}
	find := func(id string) int {
		for i, x := range refs.Entries {
			if x.RequestID == id {
				return i
			}
		}
		return -1
	}
	terminal := func(i int, op string) error {
		ref := &refs.Entries[i]
		origin, e := v.record(ref.OriginSha256)
		if e != nil {
			return e
		}
		ev := event(ref.RequestID, wire.CountOf(ref.Revision.Int()+1), op, origin.Source)
		prev, orig := ref.HeadSha256, ref.OriginSha256
		ev.PreviousSha256 = &prev
		ev.QuestionOriginSha256 = &orig
		if op == "SUPERSEDE" {
			ev.ReplacementID = r.RequestID
		}
		d, e := emit(ev)
		if e != nil {
			return e
		}
		ref.HeadSha256 = d
		ref.Revision = ev.Revision
		ref.State = "ANSWERED"
		if op == "SUPERSEDE" {
			ref.State = "SUPERSEDED"
		}
		return nil
	}
	switch r.Operation {
	case "OPEN":
		if r.Actor != r.Open.Source.Holder || r.Open.Source.AcceptanceRevision != s.AcceptanceRevision {
			return fail(escalationFailure("SOURCE_HOLDER_OR_ACCEPTANCE"))
		}
		if find(r.RequestID) >= 0 {
			return fail(escalationFailure("QUESTION_ID_EXISTS"))
		}
		events, delta := 1, 1
		if r.Open.Supersedes != "" {
			events = 2
			delta = 0
		}
		if e := EscalationCapacity(s.Refs, s.AcceptanceRevision, 1, events, delta); e != nil {
			return fail(e)
		}
		if r.Open.Supersedes != "" {
			i := find(r.Open.Supersedes)
			if i < 0 {
				return fail(escalationFailure("QUESTION_NOT_FOUND"))
			}
			ref := refs.Entries[i]
			if ref.State != "OPEN" || ref.AcceptanceRevision != s.AcceptanceRevision || ref.Revision != r.Open.ExpectedRevision {
				return fail(escalationFailure("STALE_QUESTION_CAS"))
			}
			origin, e := v.record(ref.OriginSha256)
			if e != nil {
				return fail(e)
			}
			// The administrative cross-holder supersession route is intentionally
			// unsupported; this pure worker route must retain exact admission provenance.
			if origin.Source != r.Open.Source {
				return fail(escalationFailure("SUPERSESSION_SOURCE"))
			}
			if e := terminal(i, "SUPERSEDE"); e != nil {
				return fail(e)
			}
		}
		d, e := emit(event(r.RequestID, "1", "OPEN", r.Open.Source))
		if e != nil {
			return fail(e)
		}
		refs.Entries = append(refs.Entries, ticket.EscalationRef{RequestID: r.RequestID, OriginSha256: d, HeadSha256: d, Revision: "1", AcceptanceRevision: s.AcceptanceRevision, Kind: r.Open.Kind, State: "OPEN"})
	case "ANSWER":
		i := -1
		if r.Answer.RequestID != "" {
			i = find(r.Answer.RequestID)
		} else {
			// Shorthand names every current open question when it cannot pick
			// exactly one; it never answers all of them or the newest. The IDs
			// are sorted because validate requires strictly increasing RequestID.
			open := []string{}
			for j, x := range refs.Entries {
				if x.State == "OPEN" && x.AcceptanceRevision == s.AcceptanceRevision {
					open = append(open, x.RequestID)
					i = j
				}
			}
			if len(open) > 1 {
				return fail(&EscalationRefusal{Code: "AMBIGUOUS_OPEN_QUESTIONS", RequestIDs: open})
			}
			if len(open) == 0 {
				return fail(escalationFailure("NO_OPEN_QUESTION"))
			}
		}
		if i < 0 {
			return fail(escalationFailure("QUESTION_NOT_FOUND"))
		}
		ref := refs.Entries[i]
		if ref.State != "OPEN" || ref.AcceptanceRevision != s.AcceptanceRevision || (r.Answer.RequestID != "" && ref.Revision != r.Answer.ExpectedRevision) {
			return fail(escalationFailure("STALE_QUESTION_CAS"))
		}
		if e := EscalationCapacity(s.Refs, s.AcceptanceRevision, 0, 1, -1); e != nil {
			return fail(e)
		}
		if e := terminal(i, "ANSWER"); e != nil {
			return fail(e)
		}
	}
	sort.Slice(refs.Entries, func(i, j int) bool { return refs.Entries[i].RequestID < refs.Entries[j].RequestID })
	if e := refs.Validate(); e != nil {
		return fail(e)
	}
	result.Refs = refs
	post := EscalationSnapshot{QueueID: s.QueueID, TicketID: s.TicketID, TicketRevision: result.TicketRevision, AcceptanceRevision: s.AcceptanceRevision, Refs: &result.Refs, Blobs: make(map[wire.Digest][]byte, len(s.Blobs)+2)}
	for d, b := range s.Blobs {
		post.Blobs[d] = b
	}
	for _, b := range result.Events {
		post.Blobs[b.Sha256] = b.Bytes
	}
	// The post blobs extend the pre blobs byte for byte, so v's memo stays valid.
	if _, e := (&escalationView{post, v.decoded}).answers(); e != nil {
		return fail(e)
	}
	return result, nil
}

// EscalationAnswerView is immutable guidance selected from a pinned snapshot.
// It does not assert a worker read or acknowledge the answer, or close a hold.
type EscalationAnswerView struct {
	RequestID     string                  `json:"requestId"`
	OriginSha256  wire.Digest             `json:"originSha256"`
	HeadSha256    wire.Digest             `json:"headSha256"`
	Question      string                  `json:"question"`
	Options       []string                `json:"options"`
	Kind          string                  `json:"kind"`
	Source        ticket.EscalationSource `json:"source"`
	Answer        string                  `json:"answer"`
	Actor         string                  `json:"actor"`
	ActorRole     string                  `json:"actorRole"`
	RecordedAt    wire.Timestamp          `json:"recordedAt"`
	EventRevision wire.Count              `json:"eventRevision"`
}

func SelectEscalationAnswers(s EscalationSnapshot) ([]EscalationAnswerView, error) {
	return newEscalationView(s).answers()
}

func (s *escalationView) answers() ([]EscalationAnswerView, error) {
	if e := s.validate(); e != nil {
		return nil, e
	}
	var pinned []snapshot.EscalationAnswerRef
	if s.Refs != nil {
		for _, ref := range s.Refs.Entries {
			if ref.State == "ANSWERED" && ref.AcceptanceRevision == s.AcceptanceRevision {
				pinned = append(pinned, snapshot.EscalationAnswerRef{RequestID: ref.RequestID, OriginSha256: ref.OriginSha256, HeadSha256: ref.HeadSha256})
			}
		}
	}
	out, _, e := s.guidance(pinned)
	return out, e
}

// ResolveEscalationAnswers rebuilds the guidance a claim pinned at admission
// (ESC-V0-005) from the immutable event blobs. It checks each origin and head
// binding but not the current ticket, so later answers never leak into a
// replay. The value is the delivered array, at most 256 KiB.
func ResolveEscalationAnswers(queueID, ticketID string, pinned []snapshot.EscalationAnswerRef, blobs map[wire.Digest][]byte) ([]EscalationAnswerView, wire.Value, error) {
	return newEscalationView(EscalationSnapshot{QueueID: queueID, TicketID: ticketID, Blobs: blobs}).guidance(pinned)
}

func (s *escalationView) guidance(pinned []snapshot.EscalationAnswerRef) ([]EscalationAnswerView, wire.Value, error) {
	out := []EscalationAnswerView{}
	encoded := []wire.Value{}
	for _, ref := range pinned {
		origin, e := s.record(ref.OriginSha256)
		if e != nil {
			return nil, wire.Value{}, e
		}
		head, e := s.record(ref.HeadSha256)
		if e != nil {
			return nil, wire.Value{}, e
		}
		if origin.Operation != "OPEN" || origin.EscalationID != ref.RequestID || head.Operation != "ANSWER" || head.EscalationID != ref.RequestID || head.QuestionOriginSha256 == nil || *head.QuestionOriginSha256 != ref.OriginSha256 || head.Source != origin.Source || origin.OriginalRequest.Open == nil || head.OriginalRequest.Answer == nil {
			return nil, wire.Value{}, escalationFailure("ANSWER_BINDING")
		}
		// The same terminal-chain rule validate applies: a claim pin resolves
		// without the ticket's references, so it must not admit a rehashed answer.
		if head.Revision != "2" || head.PreviousSha256 == nil || *head.PreviousSha256 != ref.OriginSha256 {
			return nil, wire.Value{}, escalationFailure("TERMINAL_CHAIN")
		}
		v := EscalationAnswerView{ref.RequestID, ref.OriginSha256, ref.HeadSha256, origin.OriginalRequest.Open.Question, append([]string{}, origin.OriginalRequest.Open.Options...), origin.OriginalRequest.Open.Kind, origin.Source, head.OriginalRequest.Answer.Text, head.Actor, head.ActorRole, head.RecordedAt, head.Revision}
		out = append(out, v)

		original, err := wire.Parse(s.Blobs[ref.OriginSha256])
		if err != nil {
			return nil, wire.Value{}, err
		}
		obj := wire.NewObject().Set("requestId", wire.String(v.RequestID)).Set("originSha256", wire.String(string(v.OriginSha256))).Set("headSha256", wire.String(string(v.HeadSha256))).Set("question", wire.String(v.Question)).Set("options", wire.Strings(v.Options)).Set("kind", wire.String(v.Kind)).Set("source", original.Obj.Vals["source"]).Set("answer", wire.String(v.Answer)).Set("actor", wire.String(v.Actor)).Set("actorRole", wire.String(v.ActorRole)).Set("recordedAt", wire.String(string(v.RecordedAt))).Set("eventRevision", wire.String(string(v.EventRevision)))
		encoded = append(encoded, wire.ObjectValue(obj))
	}
	value := wire.Array(encoded...)
	if len(wire.EncodeFile(value)) > MaxEscalationGuidanceBytes {
		return nil, wire.Value{}, escalationFailure("GUIDANCE_CAPACITY")
	}
	return out, value, nil
}

// MaxEscalationGuidanceBytes bounds the delivered answer array (ESC-V0-005).
const MaxEscalationGuidanceBytes = wire.EscalationMaxGuidanceBytes

// EscalationHolds derives typed request IDs only, never ticket status or prose.
// Infrastructure IDs are observations for a later qualified dispatcher adapter.
func EscalationHolds(s EscalationSnapshot) (held, infrastructure []string, err error) {
	if e := newEscalationView(s).validate(); e != nil {
		return nil, nil, e
	}
	held = []string{}
	infrastructure = []string{}
	if s.Refs == nil {
		return held, infrastructure, nil
	}
	for _, x := range s.Refs.Entries {
		if x.State != "OPEN" || x.AcceptanceRevision != s.AcceptanceRevision {
			continue
		}
		if x.Kind == "infrastructure" {
			infrastructure = append(infrastructure, x.RequestID)
		} else {
			held = append(held, x.RequestID)
		}
	}
	return held, infrastructure, nil
}

func EffectiveEscalationWorkRevision(s EscalationSnapshot) (wire.Count, error) {
	if e := newEscalationView(s).validate(); e != nil {
		return "", e
	}
	if s.Refs != nil && s.Refs.LastControlTicketRevision == s.TicketRevision {
		return s.Refs.WorkRevision, nil
	}
	return s.TicketRevision, nil
}
