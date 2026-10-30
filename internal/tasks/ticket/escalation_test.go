package ticket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func issue502Request() EscalationRequest {
	d := wire.Sum([]byte("admission"))
	return EscalationRequest{Profile: EscalationRequestProfile, QueueID: "queue:test:main", TicketID: "ticket:test:main:one", RequestID: "question-1", Actor: "worker-1", ActorRole: "OPERATOR", Operation: "OPEN", Open: &EscalationOpen{Source: EscalationSource{QueueID: "queue:test:main", TicketID: "ticket:test:main:one", AttemptID: "attempt:test:main:0123456789abcdef0123456789abcdef", Generation: "7", Holder: "worker-1", AcceptanceRevision: "1", ReceiptSequence: "42", ReceiptSha256: d, PostAttemptSha256: d, TicketRecordSha256: d}, Kind: "decision", Question: "Which option?", Options: []string{"a", "b"}}}
}
func issue502Event(t *testing.T) EscalationEvent {
	t.Helper()
	r := issue502Request()
	b, e := EncodeEscalationRequest(r)
	if e != nil {
		t.Fatal(e)
	}
	return EscalationEvent{Profile: EscalationEventProfile, QueueID: r.QueueID, TicketID: r.TicketID, EscalationID: r.RequestID, Revision: "1", Operation: "OPEN", Source: r.Open.Source, Actor: r.Actor, ActorRole: r.ActorRole, RecordedAt: "2026-10-04T00:00:00Z", OriginalRequest: r, RequestSha256: wire.Sum(b), ResolvedRequestID: r.RequestID, ResolvedPreviousRevision: "0"}
}

func TestIssue502_EscalationCodecAndBounds(t *testing.T) {
	event := issue502Event(t)
	raw, e := EncodeEscalationEvent(event)
	if e != nil {
		t.Fatal(e)
	}
	decoded, e := DecodeEscalationEvent(raw)
	if e != nil {
		t.Fatal(e)
	}
	again, e := EncodeEscalationEvent(decoded)
	if e != nil || !bytes.Equal(raw, again) {
		t.Fatalf("roundtrip: %v", e)
	}
	if bytes.Contains(raw, []byte("questionOriginSha256")) {
		t.Fatal("OPEN self digest")
	}
	// All mutations begin with a successfully encoded canonical fixture.
	for name, mutate := range map[string]func([]byte) []byte{
		"unknown": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`{"actor":`), []byte(`{"injected":true,"actor":`), 1)
		},
		"nestedUnknown": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"kind":"decision"`), []byte(`"kind":"decision","guess":true`), 1)
		},
		"duplicate": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`{"actor":`), []byte(`{"actor":"x","actor":`), 1)
		},
		"noLF":    func(b []byte) []byte { return bytes.TrimSuffix(b, []byte("\n")) },
		"numeric": func(b []byte) []byte { return bytes.Replace(b, []byte(`"revision":"1"`), []byte(`"revision":1`), 1) },
		"missing": func(b []byte) []byte { return bytes.Replace(b, []byte(`"resolvedPreviousRevision":"0",`), nil, 1) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := DecodeEscalationEvent(mutate(append([]byte{}, raw...))); e == nil {
				t.Fatal("accepted malformed closed event")
			}
		})
	}
	for name, mutate := range map[string]func(*EscalationEvent){
		"actor":      func(e *EscalationEvent) { e.Actor = "other" },
		"source":     func(e *EscalationEvent) { e.Source.Generation = "8" },
		"resolved":   func(e *EscalationEvent) { e.ResolvedRequestID = "different" },
		"selfDigest": func(e *EscalationEvent) { d := wire.Sum([]byte("self")); e.QuestionOriginSha256 = &d },
		"clock":      func(e *EscalationEvent) { e.RecordedAt = "tomorrow" },
	} {
		t.Run(name, func(t *testing.T) {
			v := issue502Event(t)
			mutate(&v)
			b, err := EncodeEscalationRequest(v.OriginalRequest)
			if err != nil {
				t.Fatal(err)
			}
			v.RequestSha256 = wire.Sum(b)
			if _, e := EncodeEscalationEvent(v); e == nil {
				t.Fatal("semantic mutation survived recomputed request hash")
			}
		})
	}
	r := issue502Request()
	r.Open.Question = strings.Repeat("x", 4096)
	if _, e := EncodeEscalationRequest(r); e != nil {
		t.Fatal(e)
	}
	r.Open.Question += "x"
	if _, e := EncodeEscalationRequest(r); e == nil {
		t.Fatal("question bound")
	}
	r = issue502Request()
	r.Open.Options = []string{"same", "same"}
	if _, e := EncodeEscalationRequest(r); e == nil {
		t.Fatal("duplicate options")
	}
	r = issue502Request()
	r.Open.Options = nil
	if _, e := EncodeEscalationRequest(r); e == nil {
		t.Fatal("null options")
	}
	r = issue502Request()
	r.Open.Question = " \n"
	if _, e := EncodeEscalationRequest(r); e == nil {
		t.Fatal("blank")
	}
	r = issue502Request()
	r.Open.Question = string([]byte{0xff})
	if _, e := EncodeEscalationRequest(r); e == nil {
		t.Fatal("invalid UTF8")
	}
	r = issue502Request()
	r.Open.Supersedes = "previous"
	if _, e := EncodeEscalationRequest(r); e == nil {
		t.Fatal("unpaired CAS")
	}
	// Request selectors are different immutable bytes, not resolution aliases.
	a := EscalationRequest{Profile: EscalationRequestProfile, QueueID: r.QueueID, TicketID: r.TicketID, RequestID: "answer", Actor: "owner", ActorRole: "OWNER", Operation: "ANSWER", Answer: &EscalationAnswer{Text: "choose a"}}
	shorthand, e := EncodeEscalationRequest(a)
	if e != nil {
		t.Fatal(e)
	}
	a.Answer.RequestID = "question-1"
	a.Answer.ExpectedRevision = "1"
	exact, e := EncodeEscalationRequest(a)
	if e != nil || bytes.Equal(shorthand, exact) {
		t.Fatal("selector identity")
	}
	// Independent declared wire ceilings; material reducers additionally require
	// reachable OPEN->terminal histories, whose current lifecycle is narrower.
	refs := EscalationRefs{Revision: "4096", LastControlTicketRevision: "4097", WorkRevision: "1", Entries: []EscalationRef{}}
	d := wire.Sum([]byte("ref"))
	for i := 0; i < 64; i++ {
		refs.Entries = append(refs.Entries, EscalationRef{RequestID: fmt.Sprintf("q%02d", i), OriginSha256: d, HeadSha256: d, Revision: "64", AcceptanceRevision: "1", Kind: "decision", State: "ANSWERED"})
	}
	b, e := EncodeEscalationRefs(refs)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := DecodeEscalationRefs(b); e != nil {
		t.Fatal(e)
	}
	refs.Entries[0].Revision = "65"
	if _, e := EncodeEscalationRefs(refs); e == nil {
		t.Fatal("per history cap")
	}
	refs.Entries[0].Revision = "64"
	refs.Entries = append(refs.Entries, EscalationRef{})
	if _, e := EncodeEscalationRefs(refs); e == nil {
		t.Fatal("lifetime bound")
	}
	// Absent optional control state is not introduced into an existing Record.
	var absent *EscalationRefs
	untouched, _ := json.Marshal(struct {
		Refs *EscalationRefs `json:"escalations,omitempty"`
	}{absent})
	if string(untouched) != "{}" {
		t.Fatal("NONE omission")
	}
}
