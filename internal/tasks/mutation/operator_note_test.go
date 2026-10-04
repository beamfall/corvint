package mutation_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func noteContext(t *testing.T) mutation.OperatorNoteContext {
	t.Helper()
	rec := fixture.Ticket("AT-0001")
	raw := rec.Encode()
	decoded, err := ticket.Decode(raw)
	if err != nil || !bytes.Equal(decoded.Encode(), raw) {
		t.Fatal("canonical ticket fixture", err)
	}
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	return mutation.OperatorNoteContext{Record: decoded, Binding: owner, Allowed: true, QueueID: q, RecordedAt: now}
}

func noteRequest(c *mutation.OperatorNoteContext, id, op, text string, expected, supersedes wire.Value) []byte {
	payload := obj("supersedes", supersedes)
	if op == "NOTE_SET" {
		payload.Obj.Set("text", str(text))
	}
	raw := wire.EncodeFile(obj("profile", str("taskman-mutation/0"), "requestId", str(id), "queueId", str(c.QueueID.Raw), "targetId", str(c.Record.TicketID.Raw), "actor", obj("id", str(c.Binding.ID), "role", str(c.Binding.Role)), "expectedRevision", expected, "operation", str(op), "payload", payload, "issuedAt", str("2026-10-05T00:00:00Z")))
	c.RequestID = id
	c.RequestSha256 = wire.Sum(raw)
	return raw
}

func notePropose(t *testing.T, c mutation.OperatorNoteContext, raw []byte) *mutation.OperatorNoteProposal {
	t.Helper()
	before := c.Record.Encode()
	var prior []byte
	if c.Prior != nil {
		prior = wire.EncodeFile(c.Prior.Value())
	}
	prev := append([]byte(nil), c.PriorEvent...)
	p, err := mutation.ProposeOperatorNote(c, raw)
	if err != nil {
		t.Fatal(err)
	}
	if err = mutation.ValidateOperatorNoteMaterial(c, p.Reference, p.Event); err != nil {
		t.Fatal("canonical material", err)
	}
	if !bytes.Equal(before, c.Record.Encode()) || !bytes.Equal(prev, c.PriorEvent) || (c.Prior != nil && !bytes.Equal(prior, wire.EncodeFile(c.Prior.Value()))) {
		t.Fatal("input mutation")
	}
	return p
}

func noteAdvance(t *testing.T, c *mutation.OperatorNoteContext, p *mutation.OperatorNoteProposal) {
	t.Helper()
	n, err := ticket.DecodeOperatorNoteEvent(p.Event)
	if err != nil {
		t.Fatal(err)
	}
	// This models the next audited pre-record, not a real ticket write/finalizer.
	rec, err := ticket.Decode(c.Record.Encode())
	if err != nil {
		t.Fatal(err)
	}
	previous := wire.Sum(c.Record.Encode())
	rec.PreviousRecordSha256 = &previous
	rec.Revision = n.TicketRevision
	c.Record = rec
	ref := p.Reference
	c.Prior = &ref
	c.PriorEvent = append([]byte(nil), p.Event...)
}

func TestIssue501_NoteTransition(t *testing.T) {
	c := noteContext(t)
	acceptance := c.Record.AcceptanceRevision
	var history [][]byte
	for i, step := range []struct{ op, text string }{{"NOTE_SET", "A"}, {"NOTE_SET", "B"}, {"NOTE_CLEAR", ""}, {"NOTE_SET", "C"}} {
		raw := noteRequest(&c, fmt.Sprintf("note-%d", i), step.op, step.text, wire.Null(), wire.Null())
		p := notePropose(t, c, raw)
		n, err := ticket.DecodeOperatorNoteEvent(p.Event)
		if err != nil {
			t.Fatal(err)
		}
		if n.NoteRevision != wire.CountOf(int64(i+1)) || n.TicketRevision != wire.CountOf(c.Record.Revision.Int()+1) || n.AcceptanceRevision != acceptance || n.RecordedAt != now || n.ActorID != owner.ID {
			t.Fatal("wrong event", n)
		}
		if i == 0 && n.Previous != nil || i > 0 && (n.Previous == nil || *n.Previous != wire.Sum(history[i-1])) {
			t.Fatal("broken predecessor")
		}
		if (step.op == "NOTE_SET") != (p.Reference.Current != nil) {
			t.Fatal("current/tombstone")
		}
		history = append(history, append([]byte(nil), p.Event...))
		noteAdvance(t, &c, p)
	}
	for i, b := range history {
		n, e := ticket.DecodeOperatorNoteEvent(b)
		if e != nil {
			t.Fatal(e)
		}
		q, e := ticket.DecodeOperatorNoteRequest(n.Request)
		if e != nil || q.Text != []string{"A", "B", "", "C"}[i] {
			t.Fatal("immutable history", e)
		}
	}
	// Identical fresh text and repeated fresh CLEAR remain explicit new events.
	for _, op := range []string{"NOTE_SET", "NOTE_CLEAR", "NOTE_CLEAR"} {
		raw := noteRequest(&c, "fresh-"+string(c.Prior.Revision), op, "C", wire.Null(), wire.Null())
		p := notePropose(t, c, raw)
		noteAdvance(t, &c, p)
	}
	if c.Prior.Revision != "7" || c.Prior.Current != nil {
		t.Fatal("fresh clear did not advance")
	}
	first := noteContext(t)
	raw := noteRequest(&first, "initial-clear", "NOTE_CLEAR", "", wire.Null(), str("0"))
	p := notePropose(t, first, raw)
	if p.Reference.Revision != "1" || p.Reference.Current != nil {
		t.Fatal("initial CLEAR")
	}
	// Historical note-write revisions are not compared to a later ticket: after
	// unrelated edits advance the ticket and acceptance revisions, the prior
	// event still resolves and the next note proposes and validates.
	h := noteContext(t)
	p = notePropose(t, h, noteRequest(&h, "hist-1", "NOTE_SET", "A", wire.Null(), str("0")))
	noteAdvance(t, &h, p)
	prior, e := ticket.DecodeOperatorNoteEvent(h.PriorEvent)
	if e != nil {
		t.Fatal(e)
	}
	edited, e := ticket.Decode(h.Record.Encode())
	if e != nil {
		t.Fatal(e)
	}
	edited.Revision = wire.CountOf(edited.Revision.Int() + 3)
	edited.AcceptanceRevision = wire.CountOf(edited.AcceptanceRevision.Int() + 2)
	later := h
	later.Record = edited
	p = notePropose(t, later, noteRequest(&later, "hist-2", "NOTE_SET", "B", str(string(edited.Revision)), str("1")))
	n, e := ticket.DecodeOperatorNoteEvent(p.Event)
	if e != nil || n.TicketRevision != wire.CountOf(edited.Revision.Int()+1) || n.AcceptanceRevision != edited.AcceptanceRevision || prior.AcceptanceRevision == edited.AcceptanceRevision {
		t.Fatal("note after unrelated edits", e)
	}
	// A prior note that postdates the audited ticket is refused for either
	// revision: the hist-2 event names edited.Revision+1 and edited's acceptance.
	stale := later
	stale.Prior = &p.Reference
	stale.PriorEvent = p.Event
	for _, tc := range []struct {
		name string
		edit func(*ticket.Record)
	}{
		{"ticket-revision", func(r *ticket.Record) {}},
		{"acceptance-revision", func(r *ticket.Record) {
			r.Revision = wire.CountOf(r.Revision.Int() + 5)
			r.AcceptanceRevision = h.Record.AcceptanceRevision
		}},
	} {
		t.Run("postdate-"+tc.name, func(t *testing.T) {
			r, e := ticket.Decode(edited.Encode())
			if e != nil {
				t.Fatal(e)
			}
			tc.edit(r)
			bad := stale
			bad.Record = r
			if _, e = mutation.ProposeOperatorNote(bad, noteRequest(&bad, "hist-bad", "NOTE_SET", "C", wire.Null(), str("2"))); e == nil || !strings.Contains(e.Error(), "postdate") {
				t.Fatal("prior note newer than ticket accepted", e)
			}
		})
	}
}

func TestIssue501_NoteCASAndBounds(t *testing.T) {
	c := noteContext(t)
	raw := noteRequest(&c, "set0", "NOTE_SET", "A", str(string(c.Record.Revision)), str("0"))
	p := notePropose(t, c, raw)
	t.Log("CANONICAL_NOTE_TRANSITION_READY")
	noteAdvance(t, &c, p)
	raw = noteRequest(&c, "clear1", "NOTE_CLEAR", "", wire.Null(), str("1"))
	p = notePropose(t, c, raw)
	noteAdvance(t, &c, p)
	for _, tc := range []struct {
		name                 string
		expected, supersedes wire.Value
	}{{"supersedes-zero-after-clear", wire.Null(), str("0")}, {"stale-note", wire.Null(), str("1")}, {"stale-content", str("1"), str("2")}} {
		t.Run(tc.name, func(t *testing.T) {
			raw := noteRequest(&c, tc.name, "NOTE_SET", "B", tc.expected, tc.supersedes)
			before := c.Record.Encode()
			prior := append([]byte(nil), c.PriorEvent...)
			p, e := mutation.ProposeOperatorNote(c, raw)
			if e == nil || p != nil || !strings.Contains(e.Error(), "REVISION_CONFLICT") || !bytes.Equal(before, c.Record.Encode()) || !bytes.Equal(prior, c.PriorEvent) {
				t.Fatal("CAS refusal", p, e)
			}
		})
	}
	raw = noteRequest(&c, "after-clear", "NOTE_SET", "B", wire.Null(), str("2"))
	p = notePropose(t, c, raw)
	for _, tc := range []struct {
		name string
		edit func(*mutation.OperatorNoteContext)
	}{
		{"no-grant", func(c *mutation.OperatorNoteContext) { c.Allowed = false }},
		{"binding", func(c *mutation.OperatorNoteContext) { c.Binding = worker }},
		{"archived", func(c *mutation.OperatorNoteContext) {
			c.Record.Status = ticket.StatusArchived
			from := ticket.StatusOpen
			c.Record.ArchivedFrom = &from
		}},
		{"shadow", func(c *mutation.OperatorNoteContext) {
			c.Record.ShadowOverlay = true
			c.Record.Source.Kind = "IMPORT"
			item := "external-1"
			c.Record.Source.SourceItemID = &item
		}},
		{"count-cap", func(c *mutation.OperatorNoteContext) { c.Record.Revision = "2147483647" }},
		{"wrong-prior-revision", func(c *mutation.OperatorNoteContext) { r := *c.Prior; r.Revision = "1"; c.Prior = &r }},
		{"wrong-prior-identity", func(c *mutation.OperatorNoteContext) {
			n, _ := ticket.DecodeOperatorNoteEvent(c.PriorEvent)
			n.TicketID = fixture.Ticket("AT-0002").TicketID
			v, _ := wire.Parse(n.Request)
			v.Obj.Set("targetId", str(n.TicketID.Raw))
			n.Request = wire.EncodeFile(v)
			n.RequestSha256 = wire.Sum(n.Request)
			blob, e := n.Encode()
			if e != nil {
				t.Fatal("prior fault fixture", e)
			}
			if _, e = ticket.DecodeOperatorNoteEvent(blob); e != nil {
				t.Fatal(e)
			}
			r := *c.Prior
			r.Head = wire.Sum(blob)
			c.Prior = &r
			c.PriorEvent = blob
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := c
			rec, _ := ticket.Decode(c.Record.Encode())
			v.Record = rec
			tc.edit(&v)
			before := v.Record.Encode()
			if _, e := ticket.Decode(before); e != nil {
				t.Fatal("context fixture", e)
			}
			out, e := mutation.ProposeOperatorNote(v, raw)
			if e == nil || out != nil || !bytes.Equal(before, v.Record.Encode()) {
				t.Fatal("bad context accepted/mutated", e)
			}
			if want := map[string]string{"archived": "BLOCKED", "shadow": "UNAUTHORIZED"}[tc.name]; want != "" && !strings.HasPrefix(e.Error(), want+" ") {
				t.Fatal("wrong state/ownership outcome", e)
			}
		})
	}
	// Initialize a valid highest-revision event, then test the next transition.
	capctx := noteContext(t)
	capctx.Record.Revision = "5000"
	oldRecord := wire.Sum([]byte("previous-record"))
	capctx.Record.PreviousRecordSha256 = &oldRecord
	capraw := noteRequest(&capctx, "at-cap", "NOTE_CLEAR", "", wire.Null(), wire.Null())
	first := notePropose(t, capctx, capraw)
	n, _ := ticket.DecodeOperatorNoteEvent(first.Event)
	n.NoteRevision = "4096"
	prev := wire.Sum([]byte("earlier-note"))
	n.Previous = &prev
	capped, e := n.Encode()
	if e != nil {
		t.Fatal("cap fixture encoding", e)
	}
	head := wire.Sum(capped)
	capctx.Prior = &ticket.OperatorNoteReference{Revision: "4096", Head: head}
	capctx.PriorEvent = capped
	capctx.Record.Revision = n.TicketRevision
	if _, e = ticket.ResolveOperatorNote(capctx.Record.TicketID, *capctx.Prior, capped); e != nil {
		t.Fatal("cap fixture resolution", e)
	}
	capraw = noteRequest(&capctx, "overflow", "NOTE_SET", "next", wire.Null(), str("4096"))
	if out, e := mutation.ProposeOperatorNote(capctx, capraw); e == nil || out != nil || !strings.Contains(e.Error(), "LIMIT_EXCEEDED") {
		t.Fatal("event 4097 accepted", e)
	}
	// Explicit OPERATOR authorization is data supplied by the policy adapter.
	opctx := noteContext(t)
	opctx.Binding = operator
	opraw := noteRequest(&opctx, "op", "NOTE_SET", "guidance", wire.Null(), wire.Null())
	notePropose(t, opctx, opraw)
	opctx.Allowed = false
	if out, e := mutation.ProposeOperatorNote(opctx, opraw); e == nil || out != nil {
		t.Fatal("missing explicit grant")
	}
}

func TestIssue501_NoteMaterialBindings(t *testing.T) {
	c := noteContext(t)
	raw := noteRequest(&c, "material", "NOTE_SET", "A", str(string(c.Record.Revision)), str("0"))
	valid := notePropose(t, c, raw)
	t.Log("CANONICAL_NOTE_MATERIAL_READY")
	for _, tc := range []struct {
		name, where string
		edit        func(wire.Value, *ticket.OperatorNoteEvent)
	}{
		{"wrong-pre-content", "expectedRevision", func(v wire.Value, n *ticket.OperatorNoteEvent) {
			v.Obj.Set("expectedRevision", str(string(n.TicketRevision)))
		}},
		{"wrong-supersedes", "supersedes", func(v wire.Value, n *ticket.OperatorNoteEvent) {
			p, _ := v.Obj.Get("payload")
			p.Obj.Set("supersedes", str("1"))
		}},
		{"wrong-request", "requestId", func(v wire.Value, n *ticket.OperatorNoteEvent) { v.Obj.Set("requestId", str("different")) }},
		{"wrong-queue", "queueId", func(v wire.Value, n *ticket.OperatorNoteEvent) {
			v.Obj.Set("queueId", str("queue:other:main"))
			v.Obj.Set("targetId", str("ticket:other:main:AT-0001"))
			n.TicketID, _ = wire.ParseTicketID("", "ticket:other:main:AT-0001")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, _ := ticket.DecodeOperatorNoteEvent(valid.Event)
			v, _ := wire.Parse(n.Request)
			tc.edit(v, n)
			n.Request = wire.EncodeFile(v)
			n.RequestSha256 = wire.Sum(n.Request)
			bad, e := n.Encode()
			if e != nil {
				t.Fatal("fault fixture failed canonical encoding", e)
			}
			if _, e = ticket.DecodeOperatorNoteEvent(bad); e != nil {
				t.Fatal("fault fixture failed canonical decode", e)
			}
			head := wire.Sum(bad)
			ref := valid.Reference
			ref.Head = head
			ref.Current = &head
			ctx := c
			ctx.RequestSha256 = n.RequestSha256 // prevent digest mismatch masking semantic guard
			before := ctx.Record.Encode()
			e = mutation.ValidateOperatorNoteMaterial(ctx, ref, bad)
			if e == nil || !strings.Contains(e.Error(), tc.where) || strings.Contains(e.Error(), "digest mismatch") || !bytes.Equal(before, ctx.Record.Encode()) {
				t.Fatal("semantic guard not reached", e)
			}
			t.Log("SEMANTIC_GUARD_REACHED", tc.where)
		})
	}
	for _, field := range []string{"time", "actor", "post-revision", "acceptance", "reference"} {
		t.Run(field, func(t *testing.T) {
			n, _ := ticket.DecodeOperatorNoteEvent(valid.Event)
			ctx := c
			ref := valid.Reference
			switch field {
			case "time":
				n.RecordedAt = "2026-10-06T00:00:00Z"
			case "actor":
				ctx.Binding = operator
			case "post-revision":
				n.TicketRevision = "4"
			case "acceptance":
				n.AcceptanceRevision = "2"
			case "reference":
				ref.Current = nil
			}
			bad, e := n.Encode()
			if e != nil {
				t.Fatal("fixture encoding", e)
			}
			if e = mutation.ValidateOperatorNoteMaterial(ctx, ref, bad); e == nil {
				t.Fatal("material mismatch accepted")
			}
		})
	}
	// Same semantic supersedes guard remains active after a real CLEAR reference.
	noteAdvance(t, &c, valid)
	r := noteRequest(&c, "clear", "NOTE_CLEAR", "", wire.Null(), str("1"))
	clear := notePropose(t, c, r)
	noteAdvance(t, &c, clear)
	r = noteRequest(&c, "set-after-clear", "NOTE_SET", "B", wire.Null(), str("2"))
	next := notePropose(t, c, r)
	n, _ := ticket.DecodeOperatorNoteEvent(next.Event)
	v, _ := wire.Parse(n.Request)
	payload, _ := v.Obj.Get("payload")
	payload.Obj.Set("supersedes", str("0"))
	n.Request = wire.EncodeFile(v)
	n.RequestSha256 = wire.Sum(n.Request)
	bad, e := n.Encode()
	if e != nil {
		t.Fatal(e)
	}
	ctx := c
	ctx.RequestSha256 = n.RequestSha256
	head := wire.Sum(bad)
	ref := next.Reference
	ref.Head = head
	ref.Current = &head
	before := append([]byte(nil), c.PriorEvent...)
	e = mutation.ValidateOperatorNoteMaterial(ctx, ref, bad)
	if e == nil || !strings.Contains(e.Error(), "supersedes") || !reflect.DeepEqual(before, c.PriorEvent) {
		t.Fatal("CLEAR CAS guard not reached", e)
	}
}
