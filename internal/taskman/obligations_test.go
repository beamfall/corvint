package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestTOLV0001_ReaderAdmitsObligations: Core's ticket reader admits the
// obligations reference the Tasks codec writes, mirrors its bounds, and
// refuses each malformed or inconsistent form fail-closed.
func TestTOLV0001_ReaderAdmitsObligations(t *testing.T) {
	ref := func(t *testing.T) wire.Value {
		v, e := document([]byte(`{"counts":{"coreTotal":"2","coreWitnessed":"1","deferred":"1","total":"3","witnessed":"2"},`+
			`"head":"`+strings.Repeat("e1", 32)+`","highWater":{"acceptanceRevision":"1","witnessed":"2"},`+
			`"lastRaise":{"acceptanceRevision":"1","attempt":"attempt:acme:main:b30a2bde9265683313779c2922bdca29","generation":"1"},"prefix":"AC","revision":"4"}`+"\n"), 1<<16)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	null := wire.Value{Kind: wire.KindNull}
	sub := func(v wire.Value, k string) wire.Value { return value(v, k) }
	record := func(t *testing.T, edit func(wire.Value)) wire.Value {
		v := issue502Record(t)
		o := ref(t)
		if edit != nil {
			edit(o)
		}
		setMember(v, "obligations", o)
		return v
	}
	if _, e := decodeTicket(issue502Record(t)); e != nil {
		t.Fatalf("legacy fixture: %v", e)
	}
	if _, e := decodeTicket(record(t, nil)); e != nil {
		t.Fatalf("reference: %v", e)
	}
	if _, e := decodeTicket(record(t, func(o wire.Value) { setMember(o, "lastRaise", null) })); e != nil {
		t.Fatalf("null lastRaise: %v", e)
	}
	cases := map[string]func(wire.Value){
		"not an object":         func(o wire.Value) { o.Obj.Keys, o.Obj.Values = nil, map[string]wire.Value{} },
		"unknown key":           func(o wire.Value) { setMember(o, "extra", str502("x")) },
		"missing head":          func(o wire.Value) { dropMember(o, "head") },
		"lowercase prefix":      func(o wire.Value) { setMember(o, "prefix", str502("ac")) },
		"zero revision":         func(o wire.Value) { setMember(o, "revision", str502("0")) },
		"revision over bound":   func(o wire.Value) { setMember(o, "revision", str502("1025")) },
		"bad head":              func(o wire.Value) { setMember(o, "head", str502("HEAD")) },
		"witnessed over total":  func(o wire.Value) { setMember(sub(o, "counts"), "witnessed", str502("4")) },
		"over entry bound":      func(o wire.Value) { setMember(sub(o, "counts"), "deferred", str502("254")) },
		"non-canonical count":   func(o wire.Value) { setMember(sub(o, "counts"), "total", str502("03")) },
		"high water below":      func(o wire.Value) { setMember(sub(o, "highWater"), "witnessed", str502("1")) },
		"high water postdates":  func(o wire.Value) { setMember(sub(o, "highWater"), "acceptanceRevision", str502("2")) },
		"raise postdates":       func(o wire.Value) { setMember(sub(o, "lastRaise"), "acceptanceRevision", str502("2")) },
		"raise without attempt": func(o wire.Value) { dropMember(sub(o, "lastRaise"), "attempt") },
		"raise bad generation":  func(o wire.Value) { setMember(sub(o, "lastRaise"), "generation", str502("-1")) },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeTicket(record(t, edit)); e == nil {
				t.Fatal("decoded a malformed obligations member")
			}
		})
	}
}
