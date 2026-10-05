package taskman

import (
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func prereqs502(v wire.Value) wire.Value { return value(v, "executionPrerequisites") }

// TestCALV0099_CoreReaderValidatesExecutionPrerequisites: Core's reader
// admits the CAL-V0-099 member the Tasks codec writes (the shared fixture)
// and refuses each malformed shape the native codec refuses.
func TestCALV0099_CoreReaderValidatesExecutionPrerequisites(t *testing.T) {
	if _, e := decodeTicket(issue502Record(t)); e != nil {
		t.Fatalf("decode: %v", e)
	}
	entry := func(v wire.Value, i int) wire.Value { return prereqs502(v).Arr[i] }
	arr := func(xs ...string) wire.Value {
		out := wire.Value{Kind: wire.KindArray}
		for _, x := range xs {
			out.Arr = append(out.Arr, str502(x))
		}
		return out
	}
	cases := []struct {
		name, detail string
		edit         func(wire.Value)
	}{
		{"empty", "prerequisite count", func(v wire.Value) { setMember(v, "executionPrerequisites", wire.Value{Kind: wire.KindArray}) }},
		{"not an array", "prerequisite count", func(v wire.Value) { setMember(v, "executionPrerequisites", str502("x")) }},
		{"unsorted", "unsorted", func(v wire.Value) {
			a := prereqs502(v).Arr
			a[0], a[1] = a[1], a[0]
		}},
		{"duplicate", "unsorted", func(v wire.Value) {
			p := prereqs502(v)
			p.Arr[1] = p.Arr[0]
			setMember(v, "executionPrerequisites", p)
		}},
		{"open entry", "member count", func(v wire.Value) { setMember(entry(v, 0), "note", str502("x")) }},
		{"unknown obligation", "prerequisite obligation", func(v wire.Value) { setMember(entry(v, 1), "obligation", str502("MERGED")) }},
		{"gate on COMPLETED", "prerequisite gate", func(v wire.Value) { setMember(entry(v, 1), "gateId", str502("zz")) }},
		{"no gate on GATE_PASSED", "prerequisite gate", func(v wire.Value) { setMember(entry(v, 0), "gateId", wire.Value{Kind: wire.KindNull}) }},
		{"bad ticket", "prerequisite ticket", func(v wire.Value) { setMember(entry(v, 1), "ticketId", str502("AT-02")) }},
		{"no stage", "prerequisite stages", func(v wire.Value) { setMember(entry(v, 1), "stages", arr()) }},
		{"unknown stage", "prerequisite stage", func(v wire.Value) { setMember(entry(v, 1), "stages", arr("deploy")) }},
		{"unsorted stages", "unsorted/duplicate prerequisite stages", func(v wire.Value) { setMember(entry(v, 0), "stages", arr("review", "integrate")) }},
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

// TestCALV0099_CorePlannerAppliesEveryPrerequisite: the stageless Core
// planner applies every prerequisite. A GATE_PASSED one is GATE_UNKNOWN (no
// gate evidence), a missing or uncompleted one PREREQUISITE_UNSATISFIED, and
// a COMPLETED prerequisite no longer blocks.
func TestCALV0099_CorePlannerAppliesEveryPrerequisite(t *testing.T) {
	c := testCapture(t)
	base := func() wire.Value {
		v := issue502Record(t)
		dropMember(v, "escalations")
		return v
	}
	x, e := decodeTicket(base())
	if e != nil {
		t.Fatal(e)
	}
	if got := blocker(x, map[string]ticket{x.id: x}, c); got != "GATE_UNKNOWN" {
		t.Fatalf("GATE_PASSED prerequisite: %q", got)
	}
	v := base()
	p := prereqs502(v)
	p.Arr = p.Arr[1:]
	setMember(v, "executionPrerequisites", p)
	if x, e = decodeTicket(v); e != nil {
		t.Fatal(e)
	}
	all := map[string]ticket{x.id: x}
	if got := blocker(x, all, c); got != "PREREQUISITE_UNSATISFIED" {
		t.Fatalf("missing prerequisite: %q", got)
	}
	all["ticket:acme:main:AT-02"] = ticket{id: "ticket:acme:main:AT-02", status: "OPEN", raw: base()}
	if got := blocker(x, all, c); got != "PREREQUISITE_UNSATISFIED" {
		t.Fatalf("open prerequisite: %q", got)
	}
	all["ticket:acme:main:AT-02"] = ticket{id: "ticket:acme:main:AT-02", status: "COMPLETED", raw: base()}
	if got := blocker(x, all, c); got == "PREREQUISITE_UNSATISFIED" || got == "GATE_UNKNOWN" {
		t.Fatalf("completed prerequisite still blocks: %q", got)
	}
}

// TestCALV0099_CoreSharedRefusals: Core's reader refuses every shared case
// the native codec refuses (internal/tasks/ticket TestCALV0099_SharedRefusals
// reads a byte-identical copy): duplicate (ticketId, obligation, gateId)
// edges whose stages differ, self and other-queue prerequisites, and ticket
// IDs that only look like one.
func TestCALV0099_CoreSharedRefusals(t *testing.T) {
	raw, e := os.ReadFile("testdata/cal-v0-099-prerequisite-refusals.json")
	if e != nil {
		t.Fatal(e)
	}
	doc, e := document(raw, 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	cases := value(doc, "cases").Arr
	if len(cases) < 9 {
		t.Fatalf("shared fixture has %d cases", len(cases))
	}
	for _, c := range cases {
		t.Run(stringAt(c, "name"), func(t *testing.T) {
			v := issue502Record(t)
			setMember(v, "executionPrerequisites", value(c, "executionPrerequisites"))
			if _, e := decodeTicket(v); e == nil || !strings.Contains(e.Error(), "executionPrerequisites") {
				t.Fatalf("decoded with %v; want an executionPrerequisites refusal (native %s)", e, stringAt(c, "nativeCode"))
			}
		})
	}
}
