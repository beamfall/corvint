// SPDX-License-Identifier: Apache-2.0

package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestTLFV0880_ReaderProgress: Core's ticket reader admits the shared
// fixture's progress member and refuses each malformed form, so the
// optional key stays fail-closed.
func TestTLFV0880_ReaderProgress(t *testing.T) {
	entry := func(v wire.Value) wire.Value { return value(v, "progress").Arr[0] }
	cases := []struct {
		name string
		edit func(wire.Value)
	}{
		{"empty list", func(v wire.Value) { setMember(v, "progress", wire.Value{Kind: wire.KindArray}) }},
		{"not an array", func(v wire.Value) { setMember(v, "progress", str502("x")) }},
		{"unknown entry key", func(v wire.Value) { setMember(entry(v), "extra", str502("x")) }},
		{"missing pr", func(v wire.Value) { dropMember(entry(v), "pr") }},
		{"unknown phase", func(v wire.Value) { setMember(entry(v), "phase", str502("DONE")) }},
		{"blocked without reason", func(v wire.Value) { setMember(entry(v), "phase", str502("BLOCKED")) }},
		{"review without pr", func(v wire.Value) { setMember(entry(v), "phase", str502("IN_REVIEW")) }},
		{"cleared with lane", func(v wire.Value) { setMember(entry(v), "phase", str502("CLEARED")) }},
		{"bad head", func(v wire.Value) { setMember(entry(v), "head", str502("abc")) }},
		{"bad pr", func(v wire.Value) { setMember(entry(v), "pr", str502("-1")) }},
		{"long reason", func(v wire.Value) { setMember(entry(v), "reason", str502(strings.Repeat("r", 513))) }},
		{"blank reason", func(v wire.Value) { setMember(entry(v), "reason", str502("  ")) }},
		{"bad time", func(v wire.Value) { setMember(entry(v), "recordedAt", str502("yesterday")) }},
	}
	if _, e := decodeTicket(issue502Record(t)); e != nil {
		t.Fatalf("fixture: %v", e)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := issue502Record(t)
			c.edit(v)
			if _, e := decodeTicket(v); e == nil {
				t.Fatal("decoded a malformed progress member")
			}
		})
	}
}
