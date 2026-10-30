package taskman

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502Record is the Tasks codec's own encoding of a record carrying every
// shared optional key (internal/tasks/ticket TestIssue502_RecordEscalationsKey).
// The file is a byte-identical copy of that package's fixture, because the CI
// test sandbox refuses reads outside the package directory.
func issue502Record(t *testing.T) wire.Value {
	t.Helper()
	raw, e := os.ReadFile("testdata/issue502-optional-keys-record.json")
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
		{"long pool", "label", func(v wire.Value) { setMember(v, "requiresPool", str502(strings.Repeat("p", 65))) }},
		{"duplicate stage role", "stage roles", func(v wire.Value) {
			setMember(value(v, "requiredRoles"), "review", wire.Value{Kind: wire.KindArray, Arr: []wire.Value{str502("REVIEWER"), str502("REVIEWER")}})
		}},
		{"unsorted stage roles", "stage roles", func(v wire.Value) {
			setMember(value(v, "requiredRoles"), "review", wire.Value{Kind: wire.KindArray, Arr: []wire.Value{str502("VERIFIER"), str502("REVIEWER")}})
		}},
		{"events beyond requests", "capacity", func(v wire.Value) { setMember(esc(v), "revision", str502("100")) }},
		{"seventeen current OPEN", "OPEN request bound", func(v wire.Value) { openEntries(v, 17) }},
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

// openEntries replaces the fixture's questions with n current-acceptance OPEN
// decision questions, one event each.
func openEntries(v wire.Value, n int) {
	esc := value(v, "escalations")
	base := value(esc, "entries").Arr[1]
	var arr []wire.Value
	for i := 0; i < n; i++ {
		x := wire.Value{Kind: wire.KindObject, Obj: &wire.Object{Values: map[string]wire.Value{}}}
		for _, k := range base.Obj.Keys {
			setMember(x, k, base.Obj.Values[k])
		}
		setMember(x, "requestId", str502(fmt.Sprintf("q-%02d", i)))
		arr = append(arr, x)
	}
	setMember(esc, "entries", wire.Value{Kind: wire.KindArray, Arr: arr})
	setMember(esc, "revision", str502(fmt.Sprint(n)))
}

// TestIssue502_ReaderMatchesCodecBounds: Core admits what the Tasks codec
// admits at the boundaries the record alone decides: 16 current OPEN
// questions, and identifiers the native Identifier rule accepts.
func TestIssue502_ReaderMatchesCodecBounds(t *testing.T) {
	v := issue502Record(t)
	openEntries(v, taskswire.EscalationMaxCurrentOpen)
	if _, e := decodeTicket(v); e != nil {
		t.Fatalf("16 current OPEN refused: %v", e)
	}
	v = issue502Record(t)
	setMember(value(value(v, "escalations"), "entries").Arr[1], "requestId", str502("q-2\u200d"))
	if _, e := decodeTicket(v); e != nil {
		t.Fatalf("native identifier refused: %v", e)
	}
}

// TestIssue502_PlannerBlocksUnmodelledConstraints: the planner observes no
// pools, roles or question answers, so a ticket requiring a pool or roles is
// CAPABILITY_UNAVAILABLE and a current OPEN decision, scope or blocked question
// holds it (ESC-V0-006). Infrastructure and stale questions do not hold it.
func TestIssue502_PlannerBlocksUnmodelledConstraints(t *testing.T) {
	fixture := issue502Record(t)
	question := func(kind, acceptance string) func(*captured) {
		return func(c *captured) {
			esc := value(issue502Record(t), "escalations")
			for _, x := range value(esc, "entries").Arr {
				setMember(x, "kind", str502(kind))
				setMember(x, "acceptanceRevision", str502(acceptance))
			}
			setMember(c.tickets[0].raw, "escalations", esc)
		}
	}
	tests := []struct {
		name, state, reason string
		mutate              func(*captured)
	}{
		{"none", "SELECTED", "", func(*captured) {}},
		{"pool", "BLOCKED", "CAPABILITY_UNAVAILABLE", func(c *captured) { setMember(c.tickets[0].raw, "requiresPool", value(fixture, "requiresPool")) }},
		{"roles", "BLOCKED", "CAPABILITY_UNAVAILABLE", func(c *captured) { setMember(c.tickets[0].raw, "requiredRoles", value(fixture, "requiredRoles")) }},
		{"decision", "BLOCKED", "ESCALATION_PENDING", nil},
		{"scope", "BLOCKED", "ESCALATION_PENDING", nil},
		{"blocked", "BLOCKED", "ESCALATION_PENDING", nil},
		{"infrastructure", "SELECTED", "", nil},
		{"stale decision", "SELECTED", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := testCapture(t)
			switch {
			case tc.mutate != nil:
				tc.mutate(&c)
			case tc.name == "stale decision":
				question("decision", "999")(&c)
			default:
				question(tc.name, c.tickets[0].revision)(&c)
			}
			e := testPlan(t, c).Entries[0]
			if e.State != tc.state || tc.reason != "" && e.Reason != tc.reason {
				t.Fatalf("%s/%s; want %s/%s", e.State, e.Reason, tc.state, tc.reason)
			}
		})
	}
}
