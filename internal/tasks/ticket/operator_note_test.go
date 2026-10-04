package ticket_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func noteObj(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}
func noteStr(s string) wire.Value { return wire.String(s) }

func TestIssue501_NoteCodec(t *testing.T) {
	rec := fixture.Ticket("AT-0001")
	before := rec.Encode()
	if got, err := ticket.Decode(before); err != nil || !bytes.Equal(got.Encode(), before) {
		t.Fatal("canonical ticket fixture", err)
	}
	req := noteObj("profile", noteStr("taskman-mutation/0"), "requestId", noteStr("note-1"), "queueId", noteStr(fixture.QueueID), "targetId", noteStr(rec.TicketID.Raw), "actor", noteObj("id", noteStr("owner"), "role", noteStr("OWNER")), "expectedRevision", wire.Null(), "operation", noteStr("NOTE_SET"), "payload", noteObj("text", noteStr("A: advisory guidance"), "supersedes", noteStr("0")), "issuedAt", noteStr("2026-10-05T00:00:00Z"))
	raw := wire.EncodeFile(req)
	q, err := ticket.DecodeOperatorNoteRequest(raw)
	if err != nil {
		t.Fatal("canonical request fixture", err)
	}
	event := ticket.OperatorNoteEvent{TicketID: rec.TicketID, NoteRevision: "1", TicketRevision: "2", AcceptanceRevision: rec.AcceptanceRevision, Operation: "SET", ActorID: "owner", ActorRole: "OWNER", RecordedAt: "2026-10-04T00:00:00Z", RequestSha256: wire.Sum(raw), Request: raw}
	blob, err := event.Encode()
	if err != nil {
		t.Fatal("canonical event fixture", err)
	}
	decoded, err := ticket.DecodeOperatorNoteEvent(blob)
	if err != nil {
		t.Fatal(err)
	}
	again, err := decoded.Encode()
	if err != nil || !bytes.Equal(blob, again) {
		t.Fatal("canonical event round trip", err)
	}
	head := wire.Sum(blob)
	ref := ticket.OperatorNoteReference{Revision: "1", Head: head, Current: &head}
	refRaw := wire.EncodeFile(ref.Value())
	rv, err := wire.Parse(refRaw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ticket.OperatorNoteReferenceFromValue(rv); err != nil {
		t.Fatal(err)
	}
	if _, err = ticket.ResolveOperatorNote(rec.TicketID, ref, blob); err != nil {
		t.Fatal(err)
	}
	t.Log("CANONICAL_NOTE_FIXTURE_READY")
	if decoded.RecordedAt == q.IssuedAt {
		t.Fatal("caller time replaced queue clock")
	}
	// Request decoder owns its bytes, and legacy ticket bytes remain untouched.
	raw[0] = 'X'
	if bytes.Equal(raw, q.Raw) {
		t.Fatal("request aliases caller buffer")
	}
	if !bytes.Equal(before, rec.Encode()) {
		t.Fatal("legacy ticket changed")
	}
	for _, tt := range []struct {
		name string
		edit func(wire.Value)
	}{
		{"unknown", func(v wire.Value) { v.Obj.Set("other", wire.Null()) }},
		{"whole-null", func(v wire.Value) { v.Kind = wire.KindNull; v.Obj = nil }},
		{"zero", func(v wire.Value) { v.Obj.Set("revision", noteStr("0")) }},
		{"bound", func(v wire.Value) { v.Obj.Set("revision", noteStr("4097")) }},
		{"count-overflow", func(v wire.Value) { v.Obj.Set("revision", noteStr("2147483648")) }},
		{"uppercase-digest", func(v wire.Value) { v.Obj.Set("head", noteStr(strings.ToUpper(string(head)))) }},
		{"current-mismatch", func(v wire.Value) { v.Obj.Set("current", noteStr(string(wire.Sum([]byte("other"))))) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := wire.Parse(refRaw)
			if tt.name == "whole-null" {
				v = wire.Null()
			} else {
				tt.edit(v)
			}
			if _, e := ticket.OperatorNoteReferenceFromValue(v); e == nil {
				t.Fatal("invalid reference accepted")
			}
		})
	}
	for _, tt := range []struct {
		name string
		edit func(*ticket.OperatorNoteEvent)
	}{
		{"request-digest", func(n *ticket.OperatorNoteEvent) { n.RequestSha256 = wire.Sum([]byte("wrong")) }},
		{"actor", func(n *ticket.OperatorNoteEvent) { n.ActorID = "other" }},
		{"target", func(n *ticket.OperatorNoteEvent) { n.TicketID = fixture.Ticket("AT-0002").TicketID }},
		{"operation", func(n *ticket.OperatorNoteEvent) { n.Operation = "CLEAR" }},
		{"previous-first", func(n *ticket.OperatorNoteEvent) { n.Previous = &head }},
		{"previous-missing", func(n *ticket.OperatorNoteEvent) { n.NoteRevision = "2" }},
		{"acceptance", func(n *ticket.OperatorNoteEvent) { n.AcceptanceRevision = "3" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := *decoded
			tt.edit(&n)
			if _, e := n.Encode(); e == nil {
				t.Fatal("invalid event accepted")
			}
		})
	}
	for _, key := range []string{"recordedAt", "actor", "history", "current", "head"} {
		t.Run("payload-"+key, func(t *testing.T) {
			v, _ := wire.Parse(decoded.Request)
			p, _ := v.Obj.Get("payload")
			p.Obj.Set(key, wire.Null())
			if _, e := ticket.DecodeOperatorNoteRequest(wire.EncodeFile(v)); e == nil {
				t.Fatal("payload injection accepted")
			}
		})
	}
	for _, text := range []string{"", " \t\n", string([]byte{255}), strings.Repeat("x", 8193)} {
		v, _ := wire.Parse(decoded.Request)
		p, _ := v.Obj.Get("payload")
		p.Obj.Set("text", noteStr(text))
		if _, e := ticket.DecodeOperatorNoteRequest(wire.EncodeFile(v)); e == nil {
			t.Fatal("invalid text accepted")
		}
	}
	// Maximum admissible escaped prose is still bounded after canonical encoding.
	v, _ := wire.Parse(decoded.Request)
	p, _ := v.Obj.Get("payload")
	p.Obj.Set("text", noteStr("x"+strings.Repeat("\t", 8191)))
	largest := *decoded
	largest.Request = wire.EncodeFile(v)
	largest.RequestSha256 = wire.Sum(largest.Request)
	maxBlob, err := largest.Encode()
	if err != nil || len(maxBlob) > ticket.MaxOperatorNoteBytes {
		t.Fatal("escaped bound", len(maxBlob), err)
	}
	t.Logf("max-text escaped event bytes=%d", len(maxBlob))
	if _, err = ticket.DecodeOperatorNoteEvent(append(blob, bytes.Repeat([]byte(" "), ticket.MaxOperatorNoteBytes)...)); err == nil {
		t.Fatal("event byte bound bypassed")
	}
	if _, err = ticket.DecodeOperatorNoteEvent(bytes.TrimSuffix(blob, []byte("\n"))); err == nil {
		t.Fatal("noncanonical framing accepted")
	}
	wrong := ref
	wrong.Revision = "2"
	if _, err = ticket.ResolveOperatorNote(rec.TicketID, wrong, blob); err == nil {
		t.Fatal("reference revision mismatch accepted")
	}
	wrong = ref
	wrong.Current = nil
	if _, err = ticket.ResolveOperatorNote(rec.TicketID, wrong, blob); err == nil {
		t.Fatal("SET resolved as CLEAR")
	}
	if _, err = ticket.ResolveOperatorNote(fixture.Ticket("AT-0002").TicketID, ref, blob); err == nil {
		t.Fatal("cross-ticket resolution accepted")
	}
	// Closed event and request shapes: an extra top-level event key and a CLEAR
	// payload carrying prose both refuse, as do wrong-typed reference members.
	ev, _ := wire.Parse(blob)
	ev.Obj.Set("other", wire.Null())
	if _, err = ticket.DecodeOperatorNoteEvent(wire.EncodeFile(ev)); err == nil {
		t.Fatal("unknown event key accepted")
	}
	clear, _ := wire.Parse(decoded.Request)
	clear.Obj.Set("operation", noteStr("NOTE_CLEAR"))
	if _, err = ticket.DecodeOperatorNoteRequest(wire.EncodeFile(clear)); err == nil {
		t.Fatal("CLEAR payload with text accepted")
	}
	for _, key := range []string{"revision", "current", "head"} {
		v, _ := wire.Parse(refRaw)
		v.Obj.Set(key, noteObj())
		if _, e := ticket.OperatorNoteReferenceFromValue(v); e == nil {
			t.Fatal("wrong-typed reference member accepted", key)
		}
	}
	dup := bytes.Replace(refRaw, []byte(`"revision":"1"`), []byte(`"revision":"1","revision":"1"`), 1)
	if bytes.Equal(dup, refRaw) {
		t.Fatal("duplicate-key fixture not built")
	}
	if dv, e := wire.Parse(dup); e == nil {
		if _, e = ticket.OperatorNoteReferenceFromValue(dv); e == nil {
			t.Fatal("duplicate reference key accepted")
		}
	}
}
