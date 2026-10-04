package transaction

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func esc502Fixture() (ticket.EscalationRequest, EscalationObservation) {
	d := wire.Sum([]byte("admission"))
	source := ticket.EscalationSource{QueueID: "queue:test:main", TicketID: "ticket:test:main:one", AttemptID: "attempt:test:main:0123456789abcdef0123456789abcdef", Generation: "7", Holder: "worker", AcceptanceRevision: "1", ReceiptSequence: "42", ReceiptSha256: d, PostAttemptSha256: d, TicketRecordSha256: d}
	r := ticket.EscalationRequest{Profile: ticket.EscalationRequestProfile, QueueID: source.QueueID, TicketID: source.TicketID, RequestID: "q1", Actor: "worker", ActorRole: "OPERATOR", Operation: "OPEN", Open: &ticket.EscalationOpen{Source: source, Kind: "decision", Question: "Choose?", Options: []string{"a", "b"}}}
	o := EscalationObservation{Snapshot: EscalationSnapshot{QueueID: source.QueueID, TicketID: source.TicketID, TicketRevision: "5", AcceptanceRevision: "1", Blobs: map[wire.Digest][]byte{}}, Actor: r.Actor, ActorRole: r.ActorRole, PolicyDecision: "ALLOWED", Now: "2026-10-04T00:00:00Z", Admission: &EscalationAdmission{OriginState: "AUDITED", ReceiptOperation: "CLAIM", ReceiptOutcome: "OK", ReceiptSource: source, CurrentSource: source, LeaseState: "ACTIVE", ReservationState: "MATCHED", LeaseExpires: "2026-10-04T01:00:00Z"}, Replay: EscalationReplay{State: "ABSENT"}}
	return r, o
}
func esc502Raw(t testing.TB, r ticket.EscalationRequest) []byte {
	t.Helper()
	b, e := ticket.EncodeEscalationRequest(r)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func esc502Apply(t testing.TB, r ticket.EscalationRequest, o EscalationObservation) EscalationProposal {
	t.Helper()
	p, e := ApplyEscalation(esc502Raw(t, r), o)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func esc502Post(s EscalationSnapshot, p EscalationProposal) EscalationSnapshot {
	b := map[wire.Digest][]byte{}
	for d, v := range s.Blobs {
		b[d] = append([]byte{}, v...)
	}
	for _, v := range p.Events {
		b[v.Sha256] = append([]byte{}, v.Bytes...)
	}
	refs := p.Refs
	refs.Entries = append([]ticket.EscalationRef{}, p.Refs.Entries...)
	return EscalationSnapshot{s.QueueID, s.TicketID, p.TicketRevision, p.AcceptanceRevision, &refs, b}
}
func esc502Answer(o EscalationObservation, id, q string) (ticket.EscalationRequest, EscalationObservation) {
	a := &ticket.EscalationAnswer{Text: "Use a"}
	if q != "" {
		a.RequestID = q
		a.ExpectedRevision = "1"
	}
	r := ticket.EscalationRequest{Profile: ticket.EscalationRequestProfile, QueueID: o.Snapshot.QueueID, TicketID: o.Snapshot.TicketID, RequestID: id, Actor: "owner", ActorRole: "OWNER", Operation: "ANSWER", Answer: a}
	o.Actor = r.Actor
	o.ActorRole = r.ActorRole
	o.Admission = nil
	return r, o
}
func esc502JSON(v any) []byte { b, _ := json.Marshal(v); return b }

// esc502Code requires an exact refusal code, not any error or a substring.
func esc502Code(t *testing.T, e error, want string) *EscalationRefusal {
	t.Helper()
	var refusal *EscalationRefusal
	if !errors.As(e, &refusal) || refusal.Code != want {
		t.Fatalf("want %s, got %v", want, e)
	}
	return refusal
}
func esc502Refuse(t *testing.T, r ticket.EscalationRequest, o EscalationObservation, want string) *EscalationRefusal {
	t.Helper()
	_, e := ApplyEscalation(esc502Raw(t, r), o)
	return esc502Code(t, e, want)
}

func TestIssue502_AdmissionOriginAndStaleGeneration(t *testing.T) {
	r, o := esc502Fixture()
	before := esc502JSON(o)
	p := esc502Apply(t, r, o)
	if p.ActorAuthentication != NotObserved || p.Durability != NotObserved || !bytes.Equal(before, esc502JSON(o)) || o.Snapshot.Refs != nil {
		t.Fatal("purity/authority")
	}
	for name, c := range map[string]struct {
		code   string
		mutate func(*EscalationObservation)
	}{
		"missing": {"MISSING_ADMISSION_CONTEXT", func(o *EscalationObservation) { o.Admission = nil }},
		"receipt": {"STALE_ADMISSION", func(o *EscalationObservation) {
			o.Admission.ReceiptSource.ReceiptSha256 = wire.Sum([]byte("wrong"))
		}},
		"post": {"STALE_ADMISSION", func(o *EscalationObservation) {
			o.Admission.ReceiptSource.PostAttemptSha256 = wire.Sum([]byte("wrong"))
		}},
		"generation":    {"STALE_ADMISSION", func(o *EscalationObservation) { o.Admission.CurrentSource.Generation = "8" }},
		"holder":        {"STALE_ADMISSION", func(o *EscalationObservation) { o.Admission.CurrentSource.Holder = "other" }},
		"reservation":   {"STALE_ADMISSION", func(o *EscalationObservation) { o.Admission.ReservationState = "UNKNOWN" }},
		"expired":       {"EXPIRED_ADMISSION", func(o *EscalationObservation) { o.Admission.LeaseExpires = o.Now }},
		"notClaim":      {"MISSING_ADMISSION_CONTEXT", func(o *EscalationObservation) { o.Admission.ReceiptOperation = "RENEW" }},
		"actor":         {"ACTOR_BINDING", func(o *EscalationObservation) { o.Actor = "other" }},
		"role":          {"ACTOR_BINDING", func(o *EscalationObservation) { o.ActorRole = "OWNER" }},
		"policy":        {"POLICY_NOT_ALLOWED", func(o *EscalationObservation) { o.PolicyDecision = "" }},
		"replayUnknown": {"REPLAY_UNKNOWN", func(o *EscalationObservation) { o.Replay.State = "" }},
		"acceptance":    {"SOURCE_HOLDER_OR_ACCEPTANCE", func(o *EscalationObservation) { o.Snapshot.AcceptanceRevision = "2" }},
	} {
		t.Run(name, func(t *testing.T) {
			r, o := esc502Fixture()
			c.mutate(&o)
			esc502Refuse(t, r, o, c.code)
		})
	}
	r, o = esc502Fixture()
	r.Open.BlockedBy = &ticket.EscalationBlockedBy{TicketID: "ticket:test:main:dependency"}
	r.Open.Kind = "blocked"
	esc502Refuse(t, r, o, "BLOCKED_RELATION_UNKNOWN")
	o.BlockedRelationState = "VALIDATED"
	esc502Apply(t, r, o)
	// Receipt-bound ticket content is provenance, not a requirement that unrelated
	// content has remained unchanged since the original admission.
	r, o = esc502Fixture()
	o.Snapshot.TicketRevision = "20"
	p = esc502Apply(t, r, o)
	if p.Refs.WorkRevision != "20" {
		t.Fatal("initial work seed")
	}
	r, o = esc502Fixture()
	r.Actor = "other"
	o.Actor = "other"
	if _, e := ticket.EncodeEscalationRequest(r); e == nil {
		t.Fatal("source holder mismatch")
	}
}

func TestIssue502_QuestionAnswerCASAndReplay(t *testing.T) {
	r, o := esc502Fixture()
	open := esc502Apply(t, r, o)
	o.Snapshot = esc502Post(o.Snapshot, open)
	answer, ao := esc502Answer(o, "answer-q1", "")
	pre := ao.Snapshot
	answered := esc502Apply(t, answer, ao)
	o.Snapshot = esc502Post(o.Snapshot, answered)
	r.RequestID = "q2"
	second := esc502Apply(t, r, o)
	o.Snapshot = esc502Post(o.Snapshot, second)
	replay := ao
	replay.Snapshot = o.Snapshot
	replay.Now = "2099-01-01T00:00:00Z"
	replay.PolicyDecision = "DENIED"
	replay.Replay = EscalationReplay{State: "FOUND", RequestSha256: wire.Sum(esc502Raw(t, answer)), Before: &pre, RecordedAt: ao.Now, Result: &answered}
	got := esc502Apply(t, answer, replay)
	if !got.Replayed || got.Events[0].Sha256 != answered.Events[0].Sha256 {
		t.Fatal("redirected shorthand replay")
	}
	event, e := ticket.DecodeEscalationEvent(got.Events[0].Bytes)
	if e != nil || event.ResolvedRequestID != "q1" || event.OriginalRequest.Answer.RequestID != "" {
		t.Fatal("derived target entered original request")
	}
	changed := answer
	copyA := *answer.Answer
	changed.Answer = &copyA
	changed.Answer.RequestID = "q2"
	changed.Answer.ExpectedRevision = "1"
	esc502Refuse(t, changed, replay, "REQUEST_ID_CONFLICT")
	corrupt := answered
	corrupt.TicketRevision = "99"
	replay.Replay.Result = &corrupt
	esc502Refuse(t, answer, replay, "REPLAY_MATERIAL_MISMATCH")
	// Two concurrent answers share preimage; once one commits, exact question CAS
	// refuses the second. This models CAS and does not claim a real writer race.
	a1, x := esc502Answer(ao, "race-a", "q1")
	a2 := a1
	a2.RequestID = "race-b"
	win := esc502Apply(t, a1, x)
	x.Snapshot = esc502Post(x.Snapshot, win)
	esc502Refuse(t, a2, x, "STALE_QUESTION_CAS")
	// Multiple open questions are never resolved by a newest-question heuristic.
	r, o = esc502Fixture()
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	r.RequestID = "q2"
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	amb, ambO := esc502Answer(o, "ambiguous", "")
	if named := esc502Refuse(t, amb, ambO, "AMBIGUOUS_OPEN_QUESTIONS"); strings.Join(named.RequestIDs, ",") != "q1,q2" {
		t.Fatalf("ambiguous shorthand must name the open questions: %v", named.RequestIDs)
	}
	exact, exactO := esc502Answer(o, "exact", "q1")
	expected := o.Snapshot.TicketRevision
	exact.ExpectedTicketRevision = &expected
	esc502Apply(t, exact, exactO)
	wrong := wire.CountOf(expected.Int() + 1)
	exact.ExpectedTicketRevision = &wrong
	esc502Refuse(t, exact, exactO, "STALE_TICKET_CAS")
}

func TestIssue502_TypedDispositionAndWorkRevision(t *testing.T) {
	for _, kind := range []string{"decision", "scope", "blocked", "infrastructure"} {
		t.Run(kind, func(t *testing.T) {
			r, o := esc502Fixture()
			r.Open.Kind = kind
			p := esc502Apply(t, r, o)
			s := esc502Post(o.Snapshot, p)
			held, infra, e := EscalationHolds(s)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "infrastructure" {
				if len(held) != 0 || len(infra) != 1 {
					t.Fatal("infra became hold")
				}
			} else if len(held) != 1 || len(infra) != 0 {
				t.Fatal("missing typed hold")
			}
			revision, e := EffectiveEscalationWorkRevision(s)
			if e != nil || revision != "5" {
				t.Fatal("control became work")
			}
			o.Snapshot = s
			a, ao := esc502Answer(o, "answer", "")
			ans := esc502Apply(t, a, ao)
			s = esc502Post(s, ans)
			revision, e = EffectiveEscalationWorkRevision(s)
			if e != nil || revision != "5" {
				t.Fatal("answer became work")
			}
			held, infra, e = EscalationHolds(s)
			if e != nil || len(held)+len(infra) != 0 {
				t.Fatal("answer hold")
			}
			// An ordinary content revision interleaved before another control is work.
			s.TicketRevision = "12"
			revision, e = EffectiveEscalationWorkRevision(s)
			if e != nil || revision != "12" {
				t.Fatal("ordinary edit hidden")
			}
			o.Snapshot = s
			r.RequestID = "next"
			p = esc502Apply(t, r, o)
			if p.Refs.WorkRevision != "12" || p.TicketRevision != "13" {
				t.Fatal("new work seed")
			}
			s = esc502Post(s, p)
			s.AcceptanceRevision = "2"
			held, infra, e = EscalationHolds(s)
			if e != nil || len(held)+len(infra) != 0 {
				t.Fatal("stale scope hold")
			}
		})
	}
	r, o := esc502Fixture()
	o.Snapshot.TicketRevision = wire.CountOf(wire.MaxCountValue)
	esc502Refuse(t, r, o, "TICKET_REVISION_OVERFLOW")
	// Non-answer control requests do not erase another current question's hold.
	r, o = esc502Fixture()
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	r.RequestID = "other"
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	a, ao := esc502Answer(o, "answer", "q1")
	s := esc502Post(o.Snapshot, esc502Apply(t, a, ao))
	held, _, e := EscalationHolds(s)
	if e != nil || len(held) != 1 || held[0] != "other" {
		t.Fatal("unrelated hold cleared")
	}
}

func TestIssue502_ImmutableClaimAnswerSelection(t *testing.T) {
	r, o := esc502Fixture()
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	a, ao := esc502Answer(o, "answer", "")
	p := esc502Apply(t, a, ao)
	pinned := esc502Post(o.Snapshot, p)
	first, e := SelectEscalationAnswers(pinned)
	if e != nil || len(first) != 1 || first[0].Answer != "Use a" || first[0].Source != r.Open.Source {
		t.Fatalf("guidance: %v", e)
	}
	o.Snapshot = pinned
	r.RequestID = "next"
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	a, ao = esc502Answer(o, "answer-next", "next")
	a.Answer.Text = "Use b"
	latest := esc502Post(o.Snapshot, esc502Apply(t, a, ao))
	old, e := SelectEscalationAnswers(pinned)
	if e != nil || !bytes.Equal(esc502JSON(first), esc502JSON(old)) {
		t.Fatal("latest replaced pinned guidance")
	}
	newer, e := SelectEscalationAnswers(latest)
	if e != nil || len(newer) != 2 {
		t.Fatal("next admission guidance")
	}
	first[0].Options[0] = "changed"
	old, e = SelectEscalationAnswers(pinned)
	if e != nil || old[0].Options[0] != "a" {
		t.Fatal("output alias")
	}
	latest.AcceptanceRevision = "2"
	v, e := SelectEscalationAnswers(latest)
	if e != nil || len(v) != 0 {
		t.Fatal("stale guidance")
	}
	// Valid fixture first, then inject the actual missing/corrupt pinned blob.
	missing := esc502Post(pinned, EscalationProposal{TicketRevision: pinned.TicketRevision, AcceptanceRevision: pinned.AcceptanceRevision, Refs: *pinned.Refs})
	delete(missing.Blobs, missing.Refs.Entries[0].HeadSha256)
	_, e = SelectEscalationAnswers(missing)
	esc502Code(t, e, "MISSING_EVIDENCE")
	corrupt := esc502Post(pinned, EscalationProposal{TicketRevision: pinned.TicketRevision, AcceptanceRevision: pinned.AcceptanceRevision, Refs: *pinned.Refs})
	key := corrupt.Refs.Entries[0].HeadSha256
	corrupt.Blobs[key] = []byte("bad")
	_, e = SelectEscalationAnswers(corrupt)
	esc502Code(t, e, "JOURNAL_FORKED")
	// Rehash an otherwise canonical event but cross-bind it to another origin.
	material := esc502Post(pinned, EscalationProposal{TicketRevision: pinned.TicketRevision, AcceptanceRevision: pinned.AcceptanceRevision, Refs: *pinned.Refs})
	head, e := ticket.DecodeEscalationEvent(material.Blobs[key])
	if e != nil {
		t.Fatal(e)
	}
	head.Source.Generation = "99"
	raw, e := ticket.EncodeEscalationEvent(head)
	if e != nil {
		t.Fatal(e)
	}
	newKey := wire.Sum(raw)
	material.Blobs[newKey] = raw
	material.Refs.Entries[0].HeadSha256 = newKey
	_, e = SelectEscalationAnswers(material)
	esc502Code(t, e, "REFERENCE_MATERIAL")
	t.Run("guidanceCapacityBeforePublication", func(t *testing.T) {
		r, o := esc502Fixture()
		r.Open.Question = strings.Repeat("q", 4096)
		refused := false
		for i := 0; i < 40; i++ {
			r.RequestID = fmt.Sprintf("big-%02d", i)
			o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
			a, ao := esc502Answer(o, fmt.Sprintf("answer-big-%02d", i), r.RequestID)
			a.Answer.Text = strings.Repeat("a", 8192)
			before := esc502JSON(o.Snapshot)
			p, e := ApplyEscalation(esc502Raw(t, a), ao)
			if e != nil {
				esc502Code(t, e, "GUIDANCE_CAPACITY")
				if !bytes.Equal(before, esc502JSON(o.Snapshot)) || len(p.Events) != 0 {
					t.Fatal("partial publication")
				}
				refused = true
				break
			}
			o.Snapshot = esc502Post(o.Snapshot, p)
		}
		if !refused {
			t.Fatal("did not reach actual guidance boundary")
		}
	})

}

func TestIssue502_SupersessionAndCapacity(t *testing.T) {
	r, o := esc502Fixture()
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	before := esc502JSON(o.Snapshot)
	r.RequestID = "replacement"
	r.Open.Supersedes = "q1"
	r.Open.ExpectedRevision = "1"
	p := esc502Apply(t, r, o)
	if len(p.Events) != 2 || p.TicketRevision != "7" || p.Refs.Revision != "2" || p.Refs.WorkRevision != "5" || !bytes.Equal(before, esc502JSON(o.Snapshot)) {
		t.Fatal("non-atomic/count/purity proposal")
	}
	old := p.Refs.Entries[0]
	replacement := p.Refs.Entries[1]
	validPair := esc502Post(o.Snapshot, p)
	if _, _, e := EscalationHolds(validPair); e != nil {
		t.Fatal(e)
	}
	// Inject the missing side only after the full two-event fixture succeeds.
	validPair.Refs.Entries = validPair.Refs.Entries[:1]
	_, _, e := EscalationHolds(validPair)
	esc502Code(t, e, "SUPERSESSION_PAIR_MISSING")
	// A rehashed SUPERSEDE head that disagrees with its replacement's OPEN.
	mismatch := esc502Post(o.Snapshot, p)
	head, e := ticket.DecodeEscalationEvent(mismatch.Blobs[old.HeadSha256])
	if e != nil {
		t.Fatal(e)
	}
	head.RecordedAt = "2026-10-04T00:00:01Z"
	raw, e := ticket.EncodeEscalationEvent(head)
	if e != nil {
		t.Fatal(e)
	}
	mismatch.Blobs[wire.Sum(raw)] = raw
	mismatch.Refs.Entries[0].HeadSha256 = wire.Sum(raw)
	_, _, e = EscalationHolds(mismatch)
	esc502Code(t, e, "SUPERSESSION_PAIR_MISMATCH")
	if old.State != "SUPERSEDED" || old.Revision != "2" || replacement.Revision != "1" || old.OriginSha256 != o.Snapshot.Refs.Entries[0].OriginSha256 {
		t.Fatal("supersession identities")
	}
	r.Open.ExpectedRevision = "2"
	esc502Refuse(t, r, o, "STALE_QUESTION_CAS")
	r, o = esc502Fixture()
	for i := 0; i < 16; i++ {
		r.RequestID = fmt.Sprintf("q%02d", i)
		o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	}
	r.RequestID = "overflow"
	esc502Refuse(t, r, o, "CAPACITY_EXCEEDED")
	// Reach the actual last lifetime slot with canonical reachable histories.
	r, o = esc502Fixture()
	for i := 0; i < 62; i++ {
		r.RequestID = fmt.Sprintf("q%02d", i)
		o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
		a, ao := esc502Answer(o, fmt.Sprintf("a%02d", i), r.RequestID)
		o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, a, ao))
	}
	r.RequestID = "q62"
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	r.RequestID = "q63"
	r.Open.Supersedes = "q62"
	r.Open.ExpectedRevision = "1"
	p = esc502Apply(t, r, o)
	if len(p.Refs.Entries) != 64 || len(p.Events) != 2 {
		t.Fatal("last admissible two-event transaction")
	}
	o.Snapshot = esc502Post(o.Snapshot, p)
	r.RequestID = "q64"
	r.Open.Supersedes = "q63"
	esc502Refuse(t, r, o, "CAPACITY_EXCEEDED")
	// The declared 4096 event ceiling is independently enforced; the current
	// one-OPEN/one-terminal lifecycle usually reaches the lifetime cap first.
	refs := ticket.EscalationRefs{Revision: "4096", LastControlTicketRevision: "4097", WorkRevision: "1", Entries: []ticket.EscalationRef{}}
	d := wire.Sum([]byte("capacity"))
	for i := 0; i < 64; i++ {
		refs.Entries = append(refs.Entries, ticket.EscalationRef{RequestID: fmt.Sprintf("q%02d", i), OriginSha256: d, HeadSha256: d, Revision: "64", AcceptanceRevision: "1", Kind: "decision", State: "ANSWERED"})
	}
	if e := refs.Validate(); e != nil {
		t.Fatal(e)
	}
	esc502Code(t, EscalationCapacity(&refs, "1", 1, 2, 0), "CAPACITY_EXCEEDED")
	// The worker route cannot supersede another admission's question.
	r, o = esc502Fixture()
	o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	r.RequestID = "other-source"
	r.Open.Supersedes = "q1"
	r.Open.ExpectedRevision = "1"
	r.Open.Source.Generation = "8"
	o.Admission.ReceiptSource = r.Open.Source
	o.Admission.CurrentSource = r.Open.Source
	esc502Refuse(t, r, o, "SUPERSESSION_SOURCE")
}

// Stale OPEN questions stay visible but release the current open bound: they
// can be neither answered nor superseded, so counting them would lock the ticket.
func TestIssue502_StaleOpenReleasesCapacity(t *testing.T) {
	r, o := esc502Fixture()
	for i := 0; i < 16; i++ {
		r.RequestID = fmt.Sprintf("old%02d", i)
		o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	}
	r.RequestID = "overflow"
	esc502Refuse(t, r, o, "CAPACITY_EXCEEDED")
	// An acceptance-changing edit: the ticket revision and acceptance advance.
	o.Snapshot.TicketRevision = wire.CountOf(o.Snapshot.TicketRevision.Int() + 1)
	o.Snapshot.AcceptanceRevision = "2"
	r.Open.Source.AcceptanceRevision = "2"
	o.Admission.ReceiptSource = r.Open.Source
	o.Admission.CurrentSource = r.Open.Source
	for i := 0; i < 16; i++ {
		r.RequestID = fmt.Sprintf("new%02d", i)
		o.Snapshot = esc502Post(o.Snapshot, esc502Apply(t, r, o))
	}
	stale := 0
	for _, x := range o.Snapshot.Refs.Entries {
		if x.State == "OPEN" && x.AcceptanceRevision == "1" {
			stale++
		}
	}
	if stale != 16 {
		t.Fatalf("stale questions must stay visible as OPEN history, got %d", stale)
	}
	held, _, e := EscalationHolds(o.Snapshot)
	if e != nil || len(held) != 16 || held[0] != "new00" {
		t.Fatalf("only current questions hold: %v %v", held, e)
	}
	r.RequestID = "overflow"
	esc502Refuse(t, r, o, "CAPACITY_EXCEEDED")
	// A stale question is not answerable, so it cannot free its own slot.
	a, ao := esc502Answer(o, "answer-old", "old00")
	esc502Refuse(t, a, ao, "STALE_QUESTION_CAS")
}

// BenchmarkIssue502_ApplyNearCapacity measures one reducer call over a snapshot
// at 63 answered questions, the work a native writer would do under its lock.
func BenchmarkIssue502_ApplyNearCapacity(b *testing.B) {
	r, o := esc502Fixture()
	for i := 0; i < 63; i++ {
		r.RequestID = fmt.Sprintf("q%02d", i)
		o.Snapshot = esc502Post(o.Snapshot, esc502Apply(b, r, o))
		a, ao := esc502Answer(o, fmt.Sprintf("a%02d", i), r.RequestID)
		o.Snapshot = esc502Post(o.Snapshot, esc502Apply(b, a, ao))
	}
	r.RequestID = "q63"
	raw := esc502Raw(b, r)
	b.ResetTimer()
	for range b.N {
		if _, e := ApplyEscalation(raw, o); e != nil {
			b.Fatal(e)
		}
	}
}
