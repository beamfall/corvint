package transaction

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"reflect"
	"strings"
	"testing"
)

func issue504Fixture(t *testing.T) (snapshot.ExternalReviewRequest, ExternalReviewObservations) {
	t.Helper()
	d := wire.Sum([]byte("fixture"))
	v := "PASS"
	q := snapshot.ExternalReviewRequest{RequestID: "review-1", Action: "RECORD", TicketID: "ticket:acme:main:AT-0001", GateID: "G1", ExpectedGeneration: "0", ExpectedRevision: "0", AcceptanceRevision: "1", DefinitionSha256: d, PolicySha256: d, Subject: snapshot.ExternalReviewSubject{AttemptID: "attempt:acme:main:subject", Generation: "1", ReceiptSeq: "3", ReceiptSha256: d, AttemptSha256: d}, Candidate: snapshot.ExternalReviewCandidate{Kind: "TREE", TreeOID: strings.Repeat("a", 40)}, Reasons: []snapshot.ExternalReviewReason{}, Evidence: []snapshot.GateEvidence{}, Verdict: &v}
	o := ExternalReviewObservations{Actor: mutation.Binding{ID: "owner", Role: "OWNER"}, PolicyState: "VERIFIED", SubjectState: "VERIFIED", CandidateLinkState: "VERIFIED", EvidenceState: "VERIFIED", Binding: ExternalReviewBinding{TicketID: q.TicketID, GateID: q.GateID, AcceptanceRevision: q.AcceptanceRevision, DefinitionSha256: d, PolicySha256: d, Subject: q.Subject, Candidate: q.Candidate}, Policy: ExternalReviewPolicy{RecorderRoles: []string{"OWNER", "OPERATOR", "REVIEWER"}, ReviewStages: []string{"review"}, AuthorStages: []string{"implement"}}, ReplayState: "ABSENT", RecordedAt: "2026-10-04T00:00:00Z", ReceiptSeq: "4"}
	issue504Context(t, q, &o)
	raw, err := q.Encode()
	if err != nil {
		t.Fatal("fixture request", err)
	}
	back, err := snapshot.DecodeExternalReviewRequest(raw)
	if err != nil {
		t.Fatal("fixture decode", err)
	}
	again, err := back.Encode()
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("fixture canonical roundtrip", err)
	}
	if _, err := ApplyExternalReview(q, o); err != nil {
		t.Fatal("fixture transition", err)
	}
	return q, o
}
func issue504Context(t *testing.T, q snapshot.ExternalReviewRequest, o *ExternalReviewObservations) {
	t.Helper()
	raw, err := q.Encode()
	if err != nil {
		t.Fatal(err)
	}
	o.Context = ExternalReviewContext{QueueID: "queue:acme:main", TicketID: q.TicketID, RequestID: q.RequestID, RequestSha256: wire.Sum(raw), Pre: o.Current}
}
func issue504Apply(t *testing.T, q snapshot.ExternalReviewRequest, o ExternalReviewObservations) ExternalReviewTransition {
	t.Helper()
	x, err := ApplyExternalReview(q, o)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func issue504Next(t *testing.T, q *snapshot.ExternalReviewRequest, o *ExternalReviewObservations, x ExternalReviewTransition) {
	t.Helper()
	o.Current = x.Ref
	o.CurrentEvent = x.Event
	q.ExpectedGeneration = x.Ref.Generation
	q.ExpectedRevision = x.Ref.Revision
	q.RequestID += "-next"
	o.ReceiptSeq = wire.SizeOf(o.ReceiptSeq.Uint64() + 1)
	issue504Context(t, *q, o)
}
func TestIssue504TypedRouting(t *testing.T) {
	q, o := issue504Fixture(t)
	q.Reasons = []snapshot.ExternalReviewReason{{Code: "explanation", Text: "No G1 PASS is claimed"}}
	issue504Context(t, q, &o)
	x := issue504Apply(t, q, o)
	v := ExternalReviewCurrent(x.Ref, x.Event, &o.Binding)
	if v.Status != "CURRENT" || v.Verdict == nil || *v.Verdict != "PASS" {
		t.Fatalf("prose overruled PASS: %+v", v)
	}
	issue504Next(t, &q, &o, x)
	ret := "RETURN"
	q.Verdict = &ret
	q.Reasons = []snapshot.ExternalReviewReason{{Code: "repair", Text: "## Author repair\nG1 PASS"}}
	issue504Context(t, q, &o)
	x = issue504Apply(t, q, o)
	v = ExternalReviewCurrent(x.Ref, x.Event, &o.Binding)
	if v.Verdict == nil || *v.Verdict != "RETURN" || v.Resubmitted {
		t.Fatal("prose fabricated repair/PASS")
	}
	q2, o2 := issue504Fixture(t)
	q2.GateID = "G2"
	o2.Binding.GateID = "G2"
	issue504Context(t, q2, &o2)
	g2 := issue504Apply(t, q2, o2)
	if g2.Ref.Generation != "1" || g2.Ref.Revision != "1" {
		t.Fatal("gate counters coupled")
	}
	if _, err := snapshot.DecodeGateResult(x.Event); err == nil {
		t.Fatal("external PASS/RETURN created executable gate")
	}
}
func TestIssue504ResubmitAndSecondReturn(t *testing.T) {
	t.Run("stale-resubmit-fresh-cycle", issue504StaleResubmitFreshCycle)
	q, o := issue504Fixture(t)
	ret := "RETURN"
	q.Verdict = &ret
	q.Reasons = []snapshot.ExternalReviewReason{{Code: "fix", Text: "first return"}}
	issue504Context(t, q, &o)
	first := issue504Apply(t, q, o)
	issue504Next(t, &q, &o, first)
	q.Action = "RESUBMIT"
	q.Verdict = nil
	head := first.Ref.Head
	q.PriorReturn = &head
	q.AuthorLease = &snapshot.ExternalReviewLease{AttemptID: q.Subject.AttemptID, Generation: "2", Holder: "author"}
	o.Actor = mutation.Binding{ID: "author", Role: "WORKER"}
	o.Author = ExternalReviewLeaseObservation{State: "LIVE", TicketID: q.TicketID, Stage: "implement", Lease: *q.AuthorLease}
	q.Subject.Generation = "2"
	o.Binding.Subject = q.Subject
	issue504Context(t, q, &o)
	resub := issue504Apply(t, q, o)
	view := ExternalReviewCurrent(resub.Ref, resub.Event, &o.Binding)
	if view.Status != "CURRENT" || view.Verdict != nil || !view.Resubmitted || resub.Ref.Generation != "2" || resub.Ref.Revision != "2" {
		t.Fatalf("resubmit %+v", view)
	}
	issue504Next(t, &q, &o, resub)
	q.Action = "RECORD"
	q.Verdict = &ret
	q.AuthorLease = nil
	q.PriorReturn = nil
	o.Actor = mutation.Binding{ID: "owner", Role: "OWNER"}
	issue504Context(t, q, &o)
	second := issue504Apply(t, q, o)
	view = ExternalReviewCurrent(second.Ref, second.Event, &o.Binding)
	if view.Status != "CURRENT" || view.Verdict == nil || *view.Verdict != "RETURN" || view.Resubmitted || second.Ref.Generation != "2" || second.Ref.Revision != "3" {
		t.Fatal("second RETURN lost")
	}
	// An old reviewer cannot overwrite the new generation.
	q.ExpectedGeneration = "1"
	issue504Context(t, q, &o)
	if _, err := ApplyExternalReview(q, o); err == nil || !strings.Contains(err.Error(), "CAS") {
		t.Fatalf("old generation: %v", err)
	}
}
func TestIssue504MaterialBindings(t *testing.T) {
	q, o := issue504Fixture(t)
	x := issue504Apply(t, q, o)
	if err := ValidateExternalReviewRecovery(x.Event, *x.Ref, o); err != nil {
		t.Fatal("valid initialized recovery", err)
	}
	for name, spoil := range map[string]func(*snapshot.ExternalReviewRequest, *ExternalReviewObservations){
		"CAS": func(q *snapshot.ExternalReviewRequest, o *ExternalReviewObservations) {
			q.ExpectedGeneration = "1"
			q.ExpectedRevision = "1"
		},
		"context": func(q *snapshot.ExternalReviewRequest, o *ExternalReviewObservations) { o.Context.RequestID = "other" },
		"queue-context": func(q *snapshot.ExternalReviewRequest, o *ExternalReviewObservations) {
			o.Context.QueueID = "queue:other:main"
		},
		"post-reference": func(q *snapshot.ExternalReviewRequest, o *ExternalReviewObservations) {
			ref := *x.Ref
			ref.Head = wire.Sum([]byte("wrong"))
			o.Context.Post = &ref
		},
		"subject receipt": func(q *snapshot.ExternalReviewRequest, o *ExternalReviewObservations) {
			q.Subject.ReceiptSha256 = wire.Sum([]byte("wrong"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			q, o := issue504Fixture(t)
			spoil(&q, &o)
			raw, err := q.Encode()
			if err != nil {
				t.Fatal("fault canonical encoding", err)
			}
			o.Context.RequestSha256 = wire.Sum(raw)
			if name == "CAS" || name == "subject receipt" {
				o.Context.RequestID = q.RequestID
			}
			_, err = ApplyExternalReview(q, o)
			needle := name
			if name == "queue-context" {
				needle = "context"
			}
			if name == "subject receipt" {
				needle = "subject receipt"
			}
			if err == nil || !strings.Contains(err.Error(), needle) {
				t.Fatalf("semantic %s guard not reached: %v", name, err)
			}
		})
	}
	// A valid RETURN and resubmit are initialized before rehashed prior-return
	// corruption. The event's previous link is changed too, so the semantic
	// audited old RETURN comparison, rather than a digest/shape failure, decides.
	rq, ro := issue504Fixture(t)
	ret := "RETURN"
	rq.Verdict = &ret
	rq.Reasons = []snapshot.ExternalReviewReason{{Code: "fix", Text: "return"}}
	issue504Context(t, rq, &ro)
	returned := issue504Apply(t, rq, ro)
	issue504Next(t, &rq, &ro, returned)
	rq.Action = "RESUBMIT"
	rq.Verdict = nil
	head := returned.Ref.Head
	rq.PriorReturn = &head
	rq.AuthorLease = &snapshot.ExternalReviewLease{AttemptID: rq.Subject.AttemptID, Generation: "1", Holder: "author"}
	ro.Actor = mutation.Binding{ID: "author", Role: "WORKER"}
	ro.Author = ExternalReviewLeaseObservation{State: "LIVE", TicketID: rq.TicketID, Stage: "implement", Lease: *rq.AuthorLease}
	issue504Context(t, rq, &ro)
	resub := issue504Apply(t, rq, ro)
	if err := ValidateExternalReviewRecovery(resub.Event, *resub.Ref, ro); err != nil {
		t.Fatal("valid resubmit recovery", err)
	}
	re, err := snapshot.DecodeExternalReviewEvent(resub.Event)
	if err != nil {
		t.Fatal(err)
	}
	wrong := wire.Sum([]byte("wrong RETURN"))
	re.Request.PriorReturn = &wrong
	re.Previous = &wrong
	rehashed, err := re.Encode()
	if err != nil {
		t.Fatal("wrong prior fixture", err)
	}
	rpost := *resub.Ref
	rpost.Head = wire.Sum(rehashed)
	rb, err := re.Request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ro.Context.RequestSha256 = wire.Sum(rb)
	if err = ValidateExternalReviewRecovery(rehashed, rpost, ro); err == nil || !strings.Contains(err.Error(), "prior RETURN") {
		t.Fatalf("prior RETURN semantic guard %v", err)
	}
	for name, change := range map[string]func(*ExternalReviewObservations){"context": func(o *ExternalReviewObservations) { o.Context.RequestID = "wrong" }, "post-reference": func(o *ExternalReviewObservations) {}} {
		t.Run("recovery-"+name, func(t *testing.T) {
			q, o := issue504Fixture(t)
			valid := issue504Apply(t, q, o)
			if err := ValidateExternalReviewRecovery(valid.Event, *valid.Ref, o); err != nil {
				t.Fatal("initialize recovery", err)
			}
			post := *valid.Ref
			change(&o)
			if name == "post-reference" {
				post.Head = wire.Sum([]byte("wrong"))
			}
			if err := ValidateExternalReviewRecovery(valid.Event, post, o); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("recovery %s guard %v", name, err)
			}
		})
	}
	// Rehashed wrong-CAS event/ref remains syntactically valid; recovery must
	// still compare the material expected counters with the audited pre-state.
	e, err := snapshot.DecodeExternalReviewEvent(x.Event)
	if err != nil {
		t.Fatal(err)
	}
	e.Request.ExpectedGeneration = "1"
	e.Request.ExpectedRevision = "1"
	e.ReviewGeneration = "2"
	e.EventRevision = "2"
	previous := wire.Sum([]byte("prior"))
	e.Previous = &previous
	bad, err := e.Encode()
	if err != nil {
		t.Fatal("rehash fixture", err)
	}
	post := *x.Ref
	post.Head = wire.Sum(bad)
	req, err := e.Request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	o.Context.RequestSha256 = wire.Sum(req)
	if err = ValidateExternalReviewRecovery(bad, post, o); err == nil || !strings.Contains(err.Error(), "CAS") {
		t.Fatalf("rehashed wrong-CAS recovery %v", err)
	}
}
func TestIssue504HistoricalPreservation(t *testing.T) {
	q, o := issue504Fixture(t)
	x := issue504Apply(t, q, o)
	b := o.Binding
	b.PolicySha256 = wire.Sum([]byte("later policy, same applicable definition"))
	v := ExternalReviewCurrent(x.Ref, x.Event, &b)
	if v.Status != "CURRENT" {
		t.Fatal("unrelated source policy/content erased history")
	}
	wrongGate := o.Binding
	wrongGate.GateID = "G2"
	if view := ExternalReviewCurrent(x.Ref, x.Event, &wrongGate); view.Status != "UNKNOWN" || view.Verdict != nil {
		t.Fatal("other gate head became actionable")
	}
	for name, spoil := range map[string]func(*ExternalReviewBinding){"acceptance": func(b *ExternalReviewBinding) { b.AcceptanceRevision = "2" }, "definition": func(b *ExternalReviewBinding) { b.DefinitionSha256 = wire.Sum([]byte("new")) }, "subject": func(b *ExternalReviewBinding) { b.Subject.Generation = "2" },
		"candidate": func(b *ExternalReviewBinding) { b.Candidate.TreeOID = strings.Repeat("b", 40) }} {
		t.Run(name, func(t *testing.T) {
			b := o.Binding
			spoil(&b)
			v := ExternalReviewCurrent(x.Ref, x.Event, &b)
			if v.Status != "STALE" || v.Verdict == nil || *v.Verdict != "PASS" {
				t.Fatalf("historical stale %+v", v)
			}
		})
	}
	issue504Next(t, &q, &o, x)
	q.Subject.Generation = "2"
	o.Binding.Subject = q.Subject
	issue504Context(t, q, &o)
	fresh := issue504Apply(t, q, o)
	if fresh.Ref.Generation != "2" {
		t.Fatal("stale PASS revived old generation")
	}
}
func TestIssue504ReplayAndAuthority(t *testing.T) {
	q, o := issue504Fixture(t)
	first := issue504Apply(t, q, o)
	originalQ := q
	issue504Next(t, &q, &o, first)
	ret := "RETURN"
	q.Verdict = &ret
	q.Reasons = []snapshot.ExternalReviewReason{{Code: "fix", Text: "return"}}
	issue504Context(t, q, &o)
	later := issue504Apply(t, q, o)
	replay := o
	replay.Current = later.Ref
	replay.CurrentEvent = later.Event
	replay.PolicyState = "UNKNOWN"
	replay.ReplayState = "FOUND"
	replay.ReplayEvent = first.Event
	x := issue504Apply(t, originalQ, replay)
	if x.Kind != "REPLAY" || !bytes.Equal(x.Event, first.Event) {
		t.Fatal("replay consulted later eligibility")
	}
	originalQ.RequestID = "changed"
	if _, err := ApplyExternalReview(originalQ, replay); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("replay conflict %v", err)
	}
	q, o = issue504Fixture(t)
	q.ReviewerLease = &snapshot.ExternalReviewLease{AttemptID: "attempt:acme:main:reviewer", Generation: "1", Holder: "reviewer"}
	o.Actor = mutation.Binding{ID: "reviewer", Role: "REVIEWER"}
	o.Reviewer = ExternalReviewLeaseObservation{State: "LIVE", TicketID: q.TicketID, Stage: "review", Lease: *q.ReviewerLease}
	issue504Context(t, q, &o)
	x = issue504Apply(t, q, o)
	e, err := snapshot.DecodeExternalReviewEvent(x.Event)
	if err != nil || e.TrustSource != "LEASE_BOUND" || e.Request.Subject.AttemptID == e.Request.ReviewerLease.AttemptID {
		t.Fatal("author/reviewer conflated", err)
	}
	for name, spoil := range map[string]func(*ExternalReviewObservations){"holder": func(o *ExternalReviewObservations) { o.Actor.ID = "forged" }, "expired": func(o *ExternalReviewObservations) { o.Reviewer.State = "EXPIRED" }, "generation": func(o *ExternalReviewObservations) { o.Reviewer.Lease.Generation = "2" }, "role": func(o *ExternalReviewObservations) { o.Actor.Role = "WORKER" }} {
		t.Run(name, func(t *testing.T) {
			bad := o
			spoil(&bad)
			if _, err := ApplyExternalReview(q, bad); err == nil {
				t.Fatal("authority accepted")
			}
		})
	}
	q, o = issue504Fixture(t)
	o.Policy.RequireReviewerLease = true
	if _, err := ApplyExternalReview(q, o); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("owner lease override %v", err)
	}
}
func TestIssue504BoundsAndUnknown(t *testing.T) {
	q, o := issue504Fixture(t)
	for _, field := range []string{"policy", "subject", "candidate", "evidence", "replay"} {
		t.Run(field, func(t *testing.T) {
			bad := o
			switch field {
			case "policy":
				bad.PolicyState = ""
			case "subject":
				bad.SubjectState = "UNKNOWN"
			case "candidate":
				bad.CandidateLinkState = "MISSING"
			case "evidence":
				bad.EvidenceState = "UNKNOWN"
			case "replay":
				bad.ReplayState = ""
			}
			if _, err := ApplyExternalReview(q, bad); err == nil {
				t.Fatal("missing proof accepted")
			}
		})
	}
	before, err := q.Encode()
	if err != nil {
		t.Fatal(err)
	}
	saved := o
	out := issue504Apply(t, q, o)
	after, err := q.Encode()
	if err != nil || !bytes.Equal(before, after) || !reflect.DeepEqual(saved, o) {
		t.Fatal("pure input mutated")
	}
	for _, bad := range []struct {
		ref     *snapshot.ExternalReviewRef
		raw     []byte
		binding *ExternalReviewBinding
	}{{out.Ref, nil, &o.Binding}, {out.Ref, []byte("bad"), &o.Binding}, {out.Ref, out.Event, nil}} {
		v := ExternalReviewCurrent(bad.ref, bad.raw, bad.binding)
		if v.Status != "UNKNOWN" || v.Verdict != nil {
			t.Fatalf("unknown actionable %+v", v)
		}
	}
	if ExternalReviewCurrent(nil, nil, nil).Status != "NONE" {
		t.Fatal("empty absent state")
	}
	// Same CAS sees exactly one pure transition; a second writer against the
	// committed head refuses without changing the supplied head.
	o.Current = out.Ref
	o.CurrentEvent = out.Event
	issue504Context(t, q, &o)
	if _, err := ApplyExternalReview(q, o); err == nil || !strings.Contains(err.Error(), "CAS") {
		t.Fatalf("losing CAS %v", err)
	}
}

func issue504StaleResubmitFreshCycle(t *testing.T) {
	for _, drift := range []string{"acceptance", "subject", "definition", "candidate"} {
		t.Run(drift, func(t *testing.T) {
			q, o := issue504Fixture(t)
			ret := "RETURN"
			q.Verdict = &ret
			q.Reasons = []snapshot.ExternalReviewReason{{Code: "fix", Text: "first RETURN"}}
			issue504Context(t, q, &o)
			returned := issue504Apply(t, q, o)
			issue504Next(t, &q, &o, returned)
			q.Action = "RESUBMIT"
			q.Verdict = nil
			head := returned.Ref.Head
			q.PriorReturn = &head
			q.AuthorLease = &snapshot.ExternalReviewLease{AttemptID: q.Subject.AttemptID, Generation: "2", Holder: "author"}
			o.Actor = mutation.Binding{ID: "author", Role: "WORKER"}
			o.Author = ExternalReviewLeaseObservation{State: "LIVE", TicketID: q.TicketID, Stage: "implement", Lease: *q.AuthorLease}
			q.Subject.Generation = "2"
			o.Binding.Subject = q.Subject
			issue504Context(t, q, &o)
			resubmitted := issue504Apply(t, q, o)
			if err := ValidateExternalReviewRecovery(resubmitted.Event, *resubmitted.Ref, o); err != nil {
				t.Fatal("valid canonical RESUBMIT fixture", err)
			}
			returnedBytes := append([]byte(nil), returned.Event...)
			resubmitBytes := append([]byte(nil), resubmitted.Event...)
			issue504Next(t, &q, &o, resubmitted)
			switch drift {
			case "acceptance":
				q.AcceptanceRevision = "2"
				o.Binding.AcceptanceRevision = q.AcceptanceRevision
			case "subject":
				q.Subject.Generation = "3"
				q.Subject.ReceiptSeq = "7"
				q.Subject.ReceiptSha256 = wire.Sum([]byte("new SUBMIT"))
				o.Binding.Subject = q.Subject
			case "definition":
				q.DefinitionSha256 = wire.Sum([]byte("new definition"))
				o.Binding.DefinitionSha256 = q.DefinitionSha256
			case "candidate":
				q.Candidate.TreeOID = strings.Repeat("b", 40)
				o.Binding.Candidate = q.Candidate
			}
			if view := ExternalReviewCurrent(o.Current, o.CurrentEvent, &o.Binding); view.Status != "STALE" || view.Verdict != nil || !view.Resubmitted {
				t.Fatalf("fixture failed to reach stale RESUBMIT %+v", view)
			}
			q.Action = "RECORD"
			pass := "PASS"
			q.Verdict = &pass
			q.AuthorLease = nil
			q.PriorReturn = nil
			q.Reasons = []snapshot.ExternalReviewReason{}
			q.ReviewerLease = &snapshot.ExternalReviewLease{AttemptID: "attempt:acme:main:reviewer", Generation: "4", Holder: "reviewer"}
			o.Actor = mutation.Binding{ID: "reviewer", Role: "REVIEWER"}
			o.Policy.RequireReviewerLease = true
			o.Reviewer = ExternalReviewLeaseObservation{State: "LIVE", TicketID: q.TicketID, Stage: "review", Lease: *q.ReviewerLease}
			issue504Context(t, q, &o)
			pendingRequest := q
			pendingObs := o
			for name, spoil := range map[string]func(*ExternalReviewObservations){"evidence": func(o *ExternalReviewObservations) { o.EvidenceState = "UNKNOWN" }, "subject": func(o *ExternalReviewObservations) { o.SubjectState = "UNKNOWN" }, "lease": func(o *ExternalReviewObservations) { o.Reviewer.State = "EXPIRED" }, "malformed-head": func(o *ExternalReviewObservations) { o.CurrentEvent = []byte("bad") }} {
				t.Run("still-refuses-"+name, func(t *testing.T) {
					bad := o
					spoil(&bad)
					if _, err := ApplyExternalReview(q, bad); err == nil {
						t.Fatal("stale RESUBMIT bypassed proof")
					}
				})
			}
			fresh := issue504Apply(t, q, o)
			event, err := snapshot.DecodeExternalReviewEvent(fresh.Event)
			if err != nil {
				t.Fatal(err)
			}
			if fresh.Ref.Generation != "3" || fresh.Ref.Revision != "3" || event.Previous == nil || *event.Previous != resubmitted.Ref.Head {
				t.Fatal("fresh cycle counters/link")
			}
			if err = ValidateExternalReviewRecovery(fresh.Event, *fresh.Ref, o); err != nil {
				t.Fatal("fresh cycle recovery", err)
			}
			view := ExternalReviewCurrent(fresh.Ref, fresh.Event, &o.Binding)
			if view.Status != "CURRENT" || view.Resubmitted || view.Verdict == nil || *view.Verdict != "PASS" {
				t.Fatalf("fresh review state %+v", view)
			}
			if !bytes.Equal(returnedBytes, returned.Event) || !bytes.Equal(resubmitBytes, resubmitted.Event) {
				t.Fatal("historical events changed")
			}
			if _, err = snapshot.DecodeGateResult(fresh.Event); err == nil {
				t.Fatal("fresh external event created executable gate")
			}
			oldReviewer := pendingObs
			oldReviewer.Current = fresh.Ref
			oldReviewer.CurrentEvent = fresh.Event
			issue504Context(t, pendingRequest, &oldReviewer)
			if _, err = ApplyExternalReview(pendingRequest, oldReviewer); err == nil || !strings.Contains(err.Error(), "CAS") {
				t.Fatalf("old reviewer CAS %v", err)
			}
			// Replay after a later current head returns exactly the prior event/time,
			// regardless of current eligibility; changed original bytes conflict.
			replay := oldReviewer
			replay.ReplayState = "FOUND"
			replay.ReplayEvent = fresh.Event
			replay.PolicyState = "UNKNOWN"
			same := issue504Apply(t, pendingRequest, replay)
			if same.Kind != "REPLAY" || !bytes.Equal(same.Event, fresh.Event) {
				t.Fatal("fresh record replay changed bytes")
			}
			pendingRequest.Reasons = []snapshot.ExternalReviewReason{{Code: "changed", Text: "different"}}
			if _, err = ApplyExternalReview(pendingRequest, replay); err == nil || !strings.Contains(err.Error(), "conflict") {
				t.Fatalf("replay changed identity %v", err)
			}
			// A stale RETURN is not a supersedable resubmission.
			staleReturn := pendingObs
			staleReturn.Current = returned.Ref
			staleReturn.CurrentEvent = returned.Event
			pendingRequest.Reasons = []snapshot.ExternalReviewReason{}
			pendingRequest.ExpectedGeneration = returned.Ref.Generation
			pendingRequest.ExpectedRevision = returned.Ref.Revision
			issue504Context(t, pendingRequest, &staleReturn)
			if _, err = ApplyExternalReview(pendingRequest, staleReturn); err == nil || !strings.Contains(err.Error(), "stale RETURN") {
				t.Fatalf("stale RETURN shortcut %v", err)
			}
		})
	}
}
