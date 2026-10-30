package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestTEAV0001_ReaderAttachedEvidence: Core's ticket reader admits the
// shared fixture's attachedEvidence member and refuses each malformed or
// record-inconsistent form, so the optional key stays fail-closed.
func TestTEAV0001_ReaderAttachedEvidence(t *testing.T) {
	entry := func(v wire.Value) wire.Value { return value(v, "attachedEvidence").Arr[0] }
	dig := func(s string) wire.Value { return str502(s) }
	a := strings.Repeat("a", 64)
	b := strings.Repeat("b", 64)
	cases := []struct {
		name string
		edit func(wire.Value)
	}{
		{"empty list", func(v wire.Value) { setMember(v, "attachedEvidence", wire.Value{Kind: wire.KindArray}) }},
		{"not an array", func(v wire.Value) { setMember(v, "attachedEvidence", str502("x")) }},
		{"unknown entry key", func(v wire.Value) { setMember(entry(v), "extra", str502("x")) }},
		{"missing reason", func(v wire.Value) { dropMember(entry(v), "reason") }},
		{"future acceptance", func(v wire.Value) { setMember(entry(v), "acceptanceRevision", str502("2")) }},
		{"zero acceptance", func(v wire.Value) { setMember(entry(v), "acceptanceRevision", str502("0")) }},
		{"blank reason", func(v wire.Value) { setMember(entry(v), "reason", str502("  ")) }},
		{"long reason", func(v wire.Value) { setMember(entry(v), "reason", str502(strings.Repeat("r", 513))) }},
		{"long actor", func(v wire.Value) { setMember(entry(v), "actor", str502(strings.Repeat("x", 65))) }},
		{"bad time", func(v wire.Value) { setMember(entry(v), "recordedAt", str502("yesterday")) }},
		{"no digests", func(v wire.Value) { setMember(entry(v), "evidence", wire.Value{Kind: wire.KindArray}) }},
		{"bad digest", func(v wire.Value) {
			setMember(entry(v), "evidence", wire.Value{Kind: wire.KindArray, Arr: []wire.Value{dig("00")}})
		}},
		{"unsorted digests", func(v wire.Value) {
			setMember(entry(v), "evidence", wire.Value{Kind: wire.KindArray, Arr: []wire.Value{dig(b), dig(a)}})
		}},
		{"repeat within acceptance", func(v wire.Value) {
			list := value(v, "attachedEvidence")
			setMember(v, "attachedEvidence", wire.Value{Kind: wire.KindArray, Arr: []wire.Value{list.Arr[0], list.Arr[0]}})
		}},
	}
	if _, e := decodeTicket(issue502Record(t)); e != nil {
		t.Fatalf("fixture: %v", e)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := issue502Record(t)
			c.edit(v)
			if _, e := decodeTicket(v); e == nil {
				t.Fatal("decoded a malformed attachedEvidence member")
			}
		})
	}
}
