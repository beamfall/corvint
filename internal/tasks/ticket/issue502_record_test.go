package ticket_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502RecordFixture is shared with Core's reader test
// (internal/taskman), so both readers decode the bytes this codec writes.
const issue502RecordFixture = "testdata/issue502-optional-keys-record.json"

func issue502Ref(id, state, acceptance string) ticket.EscalationRef {
	return ticket.EscalationRef{RequestID: id, OriginSha256: wire.Sum([]byte(id + "-origin")), HeadSha256: wire.Sum([]byte(id + "-" + state)), Revision: "1", AcceptanceRevision: wire.Count(acceptance), Kind: "decision", State: state}
}

// issue502Carrier is a revision-3 record whose escalation reference holds
// one answered and one open question.
func issue502Carrier() *ticket.Record {
	rec := fixture.Ticket("AT-01")
	rec.Revision = "3"
	rec.PreviousRecordSha256 = digest("revision 2")
	answered := issue502Ref("q-1", "ANSWERED", "1")
	answered.Revision = "2"
	rec.Escalations = &ticket.EscalationRefs{Revision: "2", LastControlTicketRevision: "3", WorkRevision: "1", Entries: []ticket.EscalationRef{answered, issue502Ref("q-2", "OPEN", "1")}}
	return rec
}

// issue502Questions replaces the reference with one fresh question per
// entry, each its own transaction.
func issue502Questions(rec *ticket.Record, entries []ticket.EscalationRef) {
	n := wire.CountOf(int64(len(entries)))
	rec.Revision = wire.CountOf(int64(len(entries)) + 1)
	rec.Escalations = &ticket.EscalationRefs{Revision: n, LastControlTicketRevision: rec.Revision, WorkRevision: "1", Entries: entries}
}

// TestIssue502_RecordEscalationsKey proves the optional `escalations` record
// key (ESC-V0-002): a legacy record keeps its exact bytes, a carrier round
// trips byte-identically, and the member is the reference codec's encoding.
func TestIssue502_RecordEscalationsKey(t *testing.T) {
	legacy := fixture.Ticket("AT-01")
	raw := legacy.Encode()
	if bytes.Contains(raw, []byte(`"escalations"`)) {
		t.Fatalf("a record without questions gained the key: %s", raw)
	}
	back, err := ticket.Decode(raw)
	if err != nil || back.Escalations != nil || !bytes.Equal(back.Encode(), raw) {
		t.Fatalf("legacy round trip: %v", err)
	}

	rec := issue502Carrier()
	raw = rec.Encode()
	back, err = ticket.Decode(raw)
	if err != nil {
		t.Fatalf("decode carrier: %v", err)
	}
	if !bytes.Equal(back.Encode(), raw) || back.FileDigest() != wire.Sum(raw) {
		t.Fatalf("carrier round trip differs:\n%s\n%s", raw, back.Encode())
	}
	v, _ := wire.Parse(raw)
	member, ok := v.Obj.Get("escalations")
	if !ok {
		t.Fatal("carrier lost the key")
	}
	want, err := ticket.EncodeEscalationRefs(*rec.Escalations)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire.EncodeFile(member), want) {
		t.Fatalf("record member is not the reference codec's encoding:\n%s\n%s", wire.EncodeFile(member), want)
	}

	// The fixture Core's reader decodes is exactly what this codec writes,
	// with every shared optional key present.
	rec.RequiresPool = "gpu"
	rec.RequiredRoles = map[string][]string{"implement": {"BUILDER"}, "review": {"REVIEWER"}, "integrate": {"VERIFIER"}}
	note := wire.Sum([]byte("note"))
	rec.OperatorNote = &ticket.OperatorNoteReference{Revision: "1", Current: &note, Head: note}
	rec.ExternalReviews = map[string]ticket.ExternalReviewRef{"codex-review": {Generation: "1", Revision: "1", Head: wire.Sum([]byte("review"))}}
	file, err := os.ReadFile(issue502RecordFixture)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(file, rec.Encode()) {
		t.Fatalf("%s is not this codec's encoding:\n%s", issue502RecordFixture, rec.Encode())
	}
	fv, _ := wire.Parse(file)
	for _, k := range wire.TicketRecordOptionalKeys {
		if _, ok := fv.Obj.Get(k); !ok {
			t.Fatalf("fixture lacks optional key %s", k)
		}
	}
}

// TestIssue502_RecordEscalationsRefusals: the record refuses a malformed
// reference and one inconsistent with the record that carries it, and the
// open bound counts only the current acceptance revision.
func TestIssue502_RecordEscalationsRefusals(t *testing.T) {
	open := func(acceptance string, n int) []ticket.EscalationRef {
		var out []ticket.EscalationRef
		for i := 0; i < n; i++ {
			out = append(out, issue502Ref("q-"+acceptance+string(rune('a'+i)), "OPEN", acceptance))
		}
		return out
	}
	cases := []struct {
		name, detail string
		edit         func(*ticket.Record)
	}{
		{"unsorted entries", "unsorted", func(r *ticket.Record) {
			r.Escalations.Entries[0], r.Escalations.Entries[1] = r.Escalations.Entries[1], r.Escalations.Entries[0]
		}},
		{"empty entries", "request count", func(r *ticket.Record) { r.Escalations.Entries = nil }},
		{"control after record revision", "exceeds record revision", func(r *ticket.Record) { r.Escalations.LastControlTicketRevision = "4" }},
		{"future acceptance", "exceeds record acceptance revision", func(r *ticket.Record) { r.Escalations.Entries[1].AcceptanceRevision = "2" }},
		{"seventeen current open", "OPEN questions", func(r *ticket.Record) { issue502Questions(r, open("1", 17)) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := issue502Carrier()
			c.edit(rec)
			_, err := ticket.Decode(rec.Encode())
			if code(err) != wire.CodeMalformed || !strings.Contains(err.Error(), c.detail) {
				t.Fatalf("decoded with %v; want %s naming %q", err, wire.CodeMalformed, c.detail)
			}
		})
	}

	// Sixteen current OPEN questions plus a stale one are within the bound.
	rec := issue502Carrier()
	rec.AcceptanceRevision = "2"
	issue502Questions(rec, append(open("1", 1), open("2", 16)...))
	if _, err := ticket.Decode(rec.Encode()); err != nil {
		t.Fatalf("stale OPEN counted toward the bound: %v", err)
	}

	// The key name is exact: a near miss is an unknown key.
	raw := strings.Replace(string(issue502Carrier().Encode()), `"escalations"`, `"escalation"`, 1)
	if _, err := ticket.Decode([]byte(raw)); err == nil {
		t.Fatal("unknown key escalation decoded")
	}
}
