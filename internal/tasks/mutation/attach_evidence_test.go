package mutation_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func evidencePayload(reason string, digests ...wire.Digest) wire.Value {
	vs := make([]wire.Value, 0, len(digests))
	for _, d := range digests {
		vs = append(vs, str(string(d)))
	}
	return obj("evidence", wire.Array(vs...), "reason", str(reason))
}

// sameExceptAttachment proves TEA-V0-001's record invariant: post equals pre
// in every field except attachedEvidence and the chain fields finalize always
// rewrites (revision, previousRecordSha256, updatedAt, updatedBy).
func sameExceptAttachment(t *testing.T, pre, post *ticket.Record) {
	t.Helper()
	c := *post
	c.AttachedEvidence = pre.AttachedEvidence
	c.Revision, c.PreviousRecordSha256, c.UpdatedAt, c.UpdatedBy = pre.Revision, pre.PreviousRecordSha256, pre.UpdatedAt, pre.UpdatedBy
	if !bytes.Equal(c.Encode(), pre.Encode()) {
		t.Fatalf("attach-evidence changed the record beyond the attachment:\n%s\n%s", pre.Encode(), post.Encode())
	}
	if post.AcceptanceRevision != pre.AcceptanceRevision || post.Status != pre.Status || fmt.Sprint(post.RequiredGates) != fmt.Sprint(pre.RequiredGates) {
		t.Fatalf("acceptanceRevision/status/gates moved: %s/%s/%v", post.AcceptanceRevision, post.Status, post.RequiredGates)
	}
}

// TestTEAV0001_AttachEvidenceLeavesTheRecordUnchanged: ATTACH_EVIDENCE
// appends one entry composed from the trusted binding and clock, bumps only
// the revision, and keeps acceptanceRevision, status and gates. A live
// attempt does not block it, and a later acceptance revision admits a digest
// already attached at an earlier one.
func TestTEAV0001_AttachEvidenceLeavesTheRecordUnchanged(t *testing.T) {
	pre := fixture.Ticket("AT-01")
	d1, d2 := wire.Sum([]byte("log-1")), wire.Sum([]byte("log-2"))
	if d1 > d2 {
		d1, d2 = d2, d1
	}

	live := attempts{fixture.TicketID("AT-01"): true}
	post := step(t, owner, live, pre, mutation.OpAttachEvidence, evidencePayload("focused test log", d1, d2), "1")
	sameExceptAttachment(t, pre, post)
	if len(post.AttachedEvidence) != 1 {
		t.Fatalf("entries: %+v", post.AttachedEvidence)
	}
	e := post.AttachedEvidence[0]
	if e.AcceptanceRevision != "1" || e.Actor != owner.ID || e.RecordedAt != now || e.Reason != "focused test log" || len(e.Evidence) != 2 || e.Evidence[0] != d1 || e.Evidence[1] != d2 {
		t.Fatalf("entry not composed from the payload and context: %+v", e)
	}
	back, err := ticket.Decode(post.Encode())
	if err != nil || !bytes.Equal(back.Encode(), post.Encode()) {
		t.Fatalf("post record round trip: %v", err)
	}

	// A second attachment appends; the first entry is untouched.
	d3 := wire.Sum([]byte("log-3"))
	post2 := step(t, owner, nil, post, mutation.OpAttachEvidence, evidencePayload("review transcript", d3), "1")
	sameExceptAttachment(t, post, post2)
	if len(post2.AttachedEvidence) != 2 || post2.AttachedEvidence[1].Evidence[0] != d3 {
		t.Fatalf("second attachment: %+v", post2.AttachedEvidence)
	}

	// The same digest at the same acceptance revision is DUPLICATE_ID.
	refused(t, owner, post2, mutation.OpAttachEvidence, evidencePayload("again", d1), mutation.OutcomeValidationFailed, wire.CodeDuplicateID)

	// After REOPEN-style acceptance movement the digest may be attached again.
	moved := *post2
	moved.AcceptanceRevision = "2"
	moved.AttachedEvidence = append([]ticket.AttachedEvidence(nil), post2.AttachedEvidence...)
	post3 := step(t, owner, nil, &moved, mutation.OpAttachEvidence, evidencePayload("rerun after reopen", d1), "2")
	if got := post3.AttachedEvidence[2]; got.AcceptanceRevision != "2" {
		t.Fatalf("entry acceptance revision: %+v", got)
	}
}

// TestTEAV0001_AttachEvidenceRefusals: OPEN native tickets only, closed
// payload, bounded entries, OWNER by default and OPERATOR only by explicit
// policy row, and the CAS on expectedRevision.
func TestTEAV0001_AttachEvidenceRefusals(t *testing.T) {
	d := wire.Sum([]byte("log"))
	ok := evidencePayload("why", d)

	open := fixture.Ticket("AT-01")
	draft := fixture.Ticket("AT-01")
	draft.Status = ticket.StatusDraft
	nonOpen := map[string]*ticket.Record{
		"DRAFT":     draft,
		"HELD":      step(t, owner, nil, open, mutation.OpHold, obj("holdId", str("h"), "reason", str("wait")), "1"),
		"ARCHIVED":  step(t, owner, nil, open, mutation.OpArchive, obj("reason", str("gone")), "1"),
		"COMPLETED": step(t, owner, nil, open, mutation.OpCompleteManual, obj("evidence", wire.Strings([]string{string(d)}), "reason", str("done")), "2"),
	}
	for name, rec := range nonOpen {
		t.Run("status "+name, func(t *testing.T) {
			if rec.Status != name {
				t.Fatalf("fixture status %s", rec.Status)
			}
			refused(t, owner, rec, mutation.OpAttachEvidence, ok, mutation.OutcomeBlocked, wire.CodeTicketState)
		})
	}

	imported := fixture.Ticket("AT-01")
	src := "X-1"
	imported.Source = ticket.Source{Kind: "IMPORT", SourceQueueID: "queue:ext:src", SourceItemID: &src}
	refused(t, owner, imported, mutation.OpAttachEvidence, ok, mutation.OutcomeBlocked, wire.CodeTicketState)

	at01 := fixture.Ticket("AT-01")
	rev := string(at01.Revision)
	payloadCases := map[string]wire.Value{
		"empty evidence":    evidencePayload("why"),
		"duplicate digests": evidencePayload("why", d, d),
		"blank reason":      evidencePayload("   ", d),
		"empty reason":      evidencePayload("", d),
		"unknown key":       obj("evidence", wire.Array(str(string(d))), "extra", str("x"), "reason", str("why")),
		"bad digest":        obj("evidence", wire.Array(str("00")), "reason", str("why")),
	}
	for name, p := range payloadCases {
		t.Run(name, func(t *testing.T) {
			if _, err := mutation.Decode(envelope("p", owner, "AT-01", rev, mutation.OpAttachEvidence, p)); err == nil {
				t.Fatalf("payload decoded: %s", wire.Encode(p))
			}
		})
	}
	var many []wire.Digest
	for i := 0; i <= wire.AttachedEvidenceMaxDigests; i++ {
		many = append(many, wire.Sum([]byte(fmt.Sprint("d", i))))
	}
	slices.Sort(many)
	if _, err := mutation.Decode(envelope("p", owner, "AT-01", rev, mutation.OpAttachEvidence, evidencePayload("why", many...))); err == nil {
		t.Fatal("seventeen digests decoded")
	}

	// The entry bound.
	full := fixture.Ticket("AT-01")
	for i := 0; i < wire.AttachedEvidenceMaxEntries; i++ {
		full.AttachedEvidence = append(full.AttachedEvidence, ticket.AttachedEvidence{AcceptanceRevision: "1", Actor: "russell", Evidence: []wire.Digest{wire.Sum([]byte(fmt.Sprint("e", i)))}, Reason: "r", RecordedAt: now})
	}
	if _, err := ticket.Decode(full.Encode()); err != nil {
		t.Fatalf("thirty-two entries: %v", err)
	}
	refused(t, owner, full, mutation.OpAttachEvidence, ok, mutation.OutcomeValidationFailed, wire.CodeLimitExceeded)

	// Roles.
	for _, b := range []mutation.Binding{operator, worker, reviewer, importer, system} {
		refused(t, b, at01, mutation.OpAttachEvidence, ok, mutation.OutcomeUnauthorized, "")
	}
	pv := fixture.PolicyValue()
	pv.Obj.Set("roles", obj("OPERATOR", wire.Strings([]string{"ATTACH_EVIDENCE"})))
	ctx := ctxWithPolicy(t, operator, nil, wire.EncodeFile(pv), at01)
	plan := apply(t, ctx, envelope("op1", operator, "AT-01", rev, mutation.OpAttachEvidence, ok))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.AttachedEvidence[0].Actor != operator.ID {
		t.Fatalf("operator entry actor: %+v", plan.Post.AttachedEvidence)
	}
	pv.Obj.Set("roles", obj("WORKER", wire.Strings([]string{"ATTACH_EVIDENCE"})))
	if _, err := intent.DecodePolicy(wire.EncodeFile(pv)); err == nil {
		t.Fatal("a WORKER row naming ATTACH_EVIDENCE was admitted")
	}

	// CAS: a stale expectedRevision is a conflict.
	ctx = newCtx(t, owner, nil, at01)
	want(t, apply(t, ctx, envelope("cas", owner, "AT-01", "7", mutation.OpAttachEvidence, ok)), mutation.OutcomeRevisionConflict, "")
}

// TestTEAV0001_RecordCodecRefusals: the optional record member is omitted
// when empty, stays in 1..acceptanceRevision and append order, and never
// repeats a digest within one acceptance revision.
func TestTEAV0001_RecordCodecRefusals(t *testing.T) {
	legacy := fixture.Ticket("AT-01")
	if bytes.Contains(legacy.Encode(), []byte(`"attachedEvidence"`)) {
		t.Fatal("a record without attachments gained the key")
	}
	d := wire.Sum([]byte("x"))
	entry := func(acc string, ds ...wire.Digest) ticket.AttachedEvidence {
		return ticket.AttachedEvidence{AcceptanceRevision: wire.Count(acc), Actor: "russell", Evidence: ds, Reason: "r", RecordedAt: now}
	}
	atRevision2 := func() *ticket.Record {
		rec := fixture.Ticket("AT-01")
		rec.Revision, rec.AcceptanceRevision, rec.PreviousRecordSha256 = "2", "2", &d
		return rec
	}
	cases := map[string][]ticket.AttachedEvidence{
		"future acceptance":  {entry("2", d)},
		"zero acceptance":    {entry("0", d)},
		"out of order":       {entry("2", d), entry("1", wire.Sum([]byte("y")))},
		"repeat at revision": {entry("1", d), entry("1", d)},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			rec := atRevision2()
			if name == "future acceptance" {
				rec.AcceptanceRevision = "1"
			}
			rec.AttachedEvidence = entries
			if _, err := ticket.Decode(rec.Encode()); err == nil || !strings.Contains(err.Error(), "/attachedEvidence/") {
				t.Fatalf("decoded %s with %v; want an attachedEvidence refusal", rec.Encode(), err)
			}
		})
	}
	rec := atRevision2()
	rec.AttachedEvidence = []ticket.AttachedEvidence{entry("1", d), entry("2", d)}
	if back, err := ticket.Decode(rec.Encode()); err != nil || !bytes.Equal(back.Encode(), rec.Encode()) {
		t.Fatalf("same digest at two acceptance revisions: %v", err)
	}
	empty := fixture.Ticket("AT-01").Value()
	empty.Obj.Set("attachedEvidence", wire.Array())
	if _, err := ticket.Decode(wire.EncodeFile(empty)); err == nil {
		t.Fatal("an empty attachedEvidence array decoded")
	}
}

// TestTEAV0001_AdoptFileRefusesAttachedEvidence: a hand-edited intent file
// cannot add, change or drop attachments; ADOPT_FILE refuses the field and
// composes nothing.
func TestTEAV0001_AdoptFileRefusesAttachedEvidence(t *testing.T) {
	d := wire.Sum([]byte("log"))
	entry := ticket.AttachedEvidence{AcceptanceRevision: "1", Actor: "russell", Evidence: []wire.Digest{d}, Reason: "r", RecordedAt: now}
	added := fixture.Ticket("AT-01")
	attached := step(t, owner, nil, fixture.Ticket("AT-01"), mutation.OpAttachEvidence, evidencePayload("r", d), "1")
	cases := map[string]struct {
		canonical *ticket.Record
		edit      func(*ticket.Record)
	}{
		"add":     {added, func(r *ticket.Record) { r.AttachedEvidence = []ticket.AttachedEvidence{entry} }},
		"drop":    {attached, func(r *ticket.Record) { r.AttachedEvidence = nil }},
		"rewrite": {attached, func(r *ticket.Record) { r.AttachedEvidence[0].Reason = "edited" }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			before := string(c.canonical.Encode())
			ctx := newCtx(t, owner, nil, c.canonical)
			plan := adopt(t, ctx, c.canonical, fileOf(t, c.canonical, c.edit))
			want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
			if !strings.Contains(plan.Detail, "protected field(s) attachedEvidence") || len(plan.Composed) != 0 {
				t.Fatalf("not refused as a protected field before composition: %q %v", plan.Detail, plan.Composed)
			}
			stillDiverged(t, plan, c.canonical, before)
		})
	}
}
