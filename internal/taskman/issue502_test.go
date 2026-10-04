package taskman

import (
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502Record is the Tasks codec's own encoding of a record carrying every
// shared optional key (internal/tasks/ticket TestIssue502_RecordEscalationsKey).
func issue502Record(t *testing.T) wire.Value {
	t.Helper()
	raw, e := os.ReadFile("../tasks/ticket/testdata/issue502-optional-keys-record.json")
	if e != nil {
		t.Fatal(e)
	}
	v, e := document(raw, 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func setMember(v wire.Value, k string, x wire.Value) {
	if _, ok := v.Obj.Values[k]; !ok {
		v.Obj.Keys = append(v.Obj.Keys, k)
	}
	v.Obj.Values[k] = x
}

func dropMember(v wire.Value, k string) {
	delete(v.Obj.Values, k)
	for i, x := range v.Obj.Keys {
		if x == k {
			v.Obj.Keys = append(v.Obj.Keys[:i], v.Obj.Keys[i+1:]...)
			return
		}
	}
}

func str502(s string) wire.Value { return wire.Value{Kind: wire.KindString, Str: s} }

// TestIssue502_ReaderAdmitsSharedOptionalKeys: Core's read-only planner
// decodes the bytes the Tasks codec writes for a record with requiresPool,
// requiredRoles and escalations (V1-0754, ESC-V0-002), and has a validator for
// every key the codec may omit.
func TestIssue502_ReaderAdmitsSharedOptionalKeys(t *testing.T) {
	for _, k := range taskswire.TicketRecordOptionalKeys {
		if optionalTicketMembers[k] == nil {
			t.Fatalf("no validator for shared optional key %s", k)
		}
	}
	v := issue502Record(t)
	for _, k := range taskswire.TicketRecordOptionalKeys {
		if _, ok := v.Obj.Values[k]; !ok {
			t.Fatalf("fixture lacks %s", k)
		}
	}
	if _, e := decodeTicket(v); e != nil {
		t.Fatalf("decode: %v", e)
	}
	for _, k := range taskswire.TicketRecordOptionalKeys {
		x := issue502Record(t)
		dropMember(x, k)
		if _, e := decodeTicket(x); e != nil {
			t.Fatalf("decode without %s: %v", k, e)
		}
	}
}

// TestIssue502_ReaderRefusesMalformedOptionalKeys: the reader stays closed. An
// unknown key, a missing base key, and a malformed or record-inconsistent
// optional member are each refused.
func TestIssue502_ReaderRefusesMalformedOptionalKeys(t *testing.T) {
	esc := func(v wire.Value) wire.Value { return value(v, "escalations") }
	entry := func(v wire.Value, i int) wire.Value { return value(esc(v), "entries").Arr[i] }
	cases := []struct {
		name, detail string
		edit         func(wire.Value)
	}{
		{"unknown key", "member count", func(v wire.Value) { setMember(v, "escalation", esc(v)) }},
		{"base key swapped for optional", "missing member title", func(v wire.Value) {
			dropMember(v, "title")
			setMember(v, "escalation", esc(v))
		}},
		{"pool not text", "requiresPool", func(v wire.Value) { setMember(v, "requiresPool", wire.Value{Kind: wire.KindBool}) }},
		{"stage without role", "stage roles", func(v wire.Value) {
			setMember(value(v, "requiredRoles"), "review", wire.Value{Kind: wire.KindArray})
		}},
		{"unknown role", "stage role", func(v wire.Value) {
			setMember(value(v, "requiredRoles"), "review", wire.Value{Kind: wire.KindArray, Arr: []wire.Value{str502("OWNER")}})
		}},
		{"refs missing member", "missing member workRevision", func(v wire.Value) {
			dropMember(esc(v), "workRevision")
			setMember(esc(v), "work", str502("1"))
		}},
		{"control after record revision", "work/control revision", func(v wire.Value) { setMember(esc(v), "lastControlTicketRevision", str502("4")) }},
		{"work not before control", "work/control revision", func(v wire.Value) { setMember(esc(v), "workRevision", str502("3")) }},
		{"empty entries", "request count", func(v wire.Value) { setMember(esc(v), "entries", wire.Value{Kind: wire.KindArray}) }},
		{"unsorted entries", "unsorted", func(v wire.Value) {
			a := value(esc(v), "entries").Arr
			a[0], a[1] = a[1], a[0]
		}},
		{"future acceptance", "acceptance revision", func(v wire.Value) { setMember(entry(v, 1), "acceptanceRevision", str502("2")) }},
		{"bad digest", "request digest", func(v wire.Value) { setMember(entry(v, 0), "headSha256", str502("00")) }},
		{"history cap", "history cap", func(v wire.Value) { setMember(entry(v, 0), "revision", str502("65")) }},
		{"unknown state", "request enum", func(v wire.Value) { setMember(entry(v, 0), "state", str502("CLOSED")) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := issue502Record(t)
			c.edit(v)
			if _, e := decodeTicket(v); e == nil || !strings.Contains(e.Error(), c.detail) {
				t.Fatalf("decoded with %v; want a refusal naming %q", e, c.detail)
			}
		})
	}
}
