package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestKHNV0002_ReaderKnowHow: Core's ticket reader admits the shared
// fixture's knowHow ledger and refuses each malformed or inconsistent form,
// so the optional key stays fail-closed and mirrors the Tasks codec.
func TestKHNV0002_ReaderKnowHow(t *testing.T) {
	entry := func(v wire.Value, i int) wire.Value { return value(v, "knowHow").Arr[i] }
	anchor := func(v wire.Value) wire.Value { return value(entry(v, 0), "anchors").Arr[0] }
	arr := func(vs ...wire.Value) wire.Value { return wire.Value{Kind: wire.KindArray, Arr: vs} }
	cases := []struct {
		name string
		edit func(wire.Value)
	}{
		{"empty list", func(v wire.Value) { setMember(v, "knowHow", arr()) }},
		{"not an array", func(v wire.Value) { setMember(v, "knowHow", str502("x")) }},
		{"unknown entry key", func(v wire.Value) { setMember(entry(v, 0), "extra", str502("x")) }},
		{"unknown operation", func(v wire.Value) { setMember(entry(v, 0), "operation", str502("EDIT")) }},
		{"missing text", func(v wire.Value) { dropMember(entry(v, 0), "text") }},
		{"blank text", func(v wire.Value) { setMember(entry(v, 0), "text", str502("  ")) }},
		{"long text", func(v wire.Value) { setMember(entry(v, 0), "text", str502(strings.Repeat("t", 1025))) }},
		{"seq gap", func(v wire.Value) { setMember(entry(v, 0), "seq", str502("2")) }},
		{"bad commit", func(v wire.Value) { setMember(entry(v, 0), "commit", str502("HEAD")) }},
		{"no anchors", func(v wire.Value) { setMember(entry(v, 0), "anchors", arr()) }},
		{"absolute anchor", func(v wire.Value) { setMember(anchor(v), "path", str502("/etc/passwd")) }},
		{"dotdot anchor", func(v wire.Value) { setMember(anchor(v), "path", str502("../x.go")) }},
		{"directory anchor", func(v wire.Value) { setMember(anchor(v), "path", str502("internal/")) }},
		{"bad blob", func(v wire.Value) { setMember(anchor(v), "blob", str502("00")) }},
		{"duplicate anchors", func(v wire.Value) {
			e := value(entry(v, 1), "anchors")
			setMember(entry(v, 1), "anchors", arr(e.Arr[0], e.Arr[0]))
		}},
		{"unsorted routes", func(v wire.Value) {
			r := value(entry(v, 1), "routes")
			setMember(entry(v, 1), "routes", arr(r.Arr[1], r.Arr[0]))
		}},
		{"supersede without reason", func(v wire.Value) { setMember(entry(v, 1), "reason", wire.Value{Kind: wire.KindNull}) }},
		{"reason without supersede", func(v wire.Value) { setMember(entry(v, 0), "reason", str502("why")) }},
		{"retract of superseded note", func(v wire.Value) { setMember(entry(v, 3), "note", str502("1")) }},
		{"forward target", func(v wire.Value) { setMember(entry(v, 1), "supersedes", str502("4")) }},
		{"worker actor", func(v wire.Value) { setMember(value(entry(v, 0), "actor"), "role", str502("WORKER")) }},
		{"bad time", func(v wire.Value) { setMember(entry(v, 3), "recordedAt", str502("yesterday")) }},
		{"retract text", func(v wire.Value) { setMember(entry(v, 3), "text", str502("x")) }},
	}
	if _, e := decodeTicket(issue502Record(t)); e != nil {
		t.Fatalf("fixture: %v", e)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := issue502Record(t)
			c.edit(v)
			if _, e := decodeTicket(v); e == nil {
				t.Fatal("decoded a malformed knowHow member")
			}
		})
	}
}
