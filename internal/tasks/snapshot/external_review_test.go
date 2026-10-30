package snapshot

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

func issue504Event() ExternalReviewEvent {
	d := wire.Sum([]byte("fixture"))
	v := "PASS"
	q := ExternalReviewRequest{RequestID: "review-1", Action: "RECORD", TicketID: "ticket:acme:main:AT-0001", GateID: "G1", ExpectedGeneration: "0", ExpectedRevision: "0", AcceptanceRevision: "1", DefinitionSha256: d, PolicySha256: d, Subject: ExternalReviewSubject{AttemptID: testAttempt, Generation: "1", ReceiptSeq: "3", ReceiptSha256: d, AttemptSha256: d}, Candidate: ExternalReviewCandidate{Kind: "TREE", TreeOID: strings.Repeat("a", 40)}, Reasons: []ExternalReviewReason{}, Evidence: []GateEvidence{}, Verdict: &v}
	return ExternalReviewEvent{Request: q, ReviewGeneration: "1", EventRevision: "1", ActorID: "owner", ActorRole: "OWNER", TrustSource: "OPERATOR_ATTESTED", RecordedAt: "2026-10-04T00:00:00Z", ReceiptSeq: "4"}
}
func TestIssue504CanonicalRoundTrip(t *testing.T) {
	e := issue504Event()
	raw, err := e.Encode()
	if err != nil {
		t.Fatal("fixture initialization", err)
	}
	back, err := CanonicalExternalReviewEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	again, err := back.Encode()
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatalf("roundtrip: %v", err)
	}
	request, err := e.Request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeExternalReviewRequest(request); err != nil {
		t.Fatal(err)
	}
	ref := ExternalReviewRef{"1", "1", wire.Sum(raw)}
	rr, err := ref.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeExternalReviewRef(rr); err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeGateResult(raw); err == nil {
		t.Fatal("external event became executable GateResult")
	}
	for name, changed := range map[string][]byte{"duplicate": bytes.Replace(request, []byte(`"action":"RECORD"`), []byte(`"action":"RECORD","action":"RECORD"`), 1), "unknown": bytes.Replace(request, []byte(`"action":"RECORD"`), []byte(`"extra":null,"action":"RECORD"`), 1), "missing-null": bytes.Replace(request, []byte(`"authorLease":null,`), nil, 1)} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeExternalReviewRequest(changed); err == nil {
				t.Fatal("closed schema accepted")
			}
		})
	}
	if _, err = CanonicalExternalReviewEvent(bytes.TrimSpace(raw)); err == nil {
		t.Fatal("noncanonical event accepted")
	}
	// Rehash the nested original request: a syntactically valid changed request
	// must still agree with the event's duplicated fields.
	parsed, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := parsed.Obj.Get("request")
	req.Obj.Set("requestId", wire.String("changed"))
	parsed.Obj.Set("requestSha256", wire.String(string(wire.Sum(wire.EncodeFile(req)))))
	parsed.Obj.Set("gateId", wire.String("G2"))
	if _, err = DecodeExternalReviewEvent(wire.EncodeFile(parsed)); err == nil || !strings.Contains(err.Error(), "event and retained request differ") {
		t.Fatalf("semantic request mismatch: %v", err)
	}
}
func TestIssue504BoundsAndUnknown(t *testing.T) {
	fixture := issue504Event()
	if _, err := fixture.Encode(); err != nil {
		t.Fatal("fixture", err)
	}
	for name, spoil := range map[string]func(*ExternalReviewEvent){
		"count": func(e *ExternalReviewEvent) { e.EventRevision = "4097"; d := wire.Sum(nil); e.Previous = &d },
		"reasons-count": func(e *ExternalReviewEvent) {
			e.Request.Reasons = make([]ExternalReviewReason, 17)
			for i := range e.Request.Reasons {
				e.Request.Reasons[i] = ExternalReviewReason{"x", "x"}
			}
		},
		"aggregate": func(e *ExternalReviewEvent) {
			e.Request.Reasons = make([]ExternalReviewReason, 9)
			for i := range e.Request.Reasons {
				e.Request.Reasons[i] = ExternalReviewReason{"x", strings.Repeat("x", 512)}
			}
		},
		"reason-length": func(e *ExternalReviewEvent) {
			e.Request.Reasons = []ExternalReviewReason{{"x", strings.Repeat("x", 513)}}
		},
		"empty-return":   func(e *ExternalReviewEvent) { v := "RETURN"; e.Request.Verdict = &v },
		"missing-author": func(e *ExternalReviewEvent) { e.Request.Action = "RESUBMIT"; e.Request.Verdict = nil },
		"trust":          func(e *ExternalReviewEvent) { e.TrustSource = "VERIFIED" },
	} {
		t.Run(name, func(t *testing.T) {
			e := issue504Event()
			spoil(&e)
			if _, err := e.Encode(); err == nil {
				t.Fatal("bounded shape accepted")
			}
		})
	}
	if _, err := DecodeExternalReviewEvent(bytes.Repeat([]byte("x"), MaxExternalReviewBytes+1)); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("byte cap: %v", err)
	}
	q := fixture.Request
	q.Candidate = ExternalReviewCandidate{Kind: "EVIDENCE", Sha256: wire.Sum(nil), Bytes: "0"}
	if _, err := q.Encode(); err != nil {
		t.Fatalf("evidence union shape %v", err)
	}
}
