package taskman

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// verdictPathsRecord is the Tasks codec's own encoding of a record whose
// effects declare verdictPaths (internal/tasks/ticket
// TestTCBV0035_RecordVerdictPathsKey). The file is a byte-identical copy of
// that package's fixture, because the CI test sandbox refuses reads outside
// the package directory.
func verdictPathsRecord(t *testing.T) wire.Value {
	t.Helper()
	raw, e := os.ReadFile("testdata/verdict-paths-record.json")
	if e != nil {
		t.Fatal(e)
	}
	v, e := document(raw, 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func verdictStrings(xs ...string) wire.Value {
	out := wire.Value{Kind: wire.KindArray}
	for _, x := range xs {
		out.Arr = append(out.Arr, str502(x))
	}
	return out
}

// TestTCBV0035_ReaderAdmitsVerdictPaths: Core's read-only planner decodes the
// bytes the Tasks codec writes for a record with effects.verdictPaths, and the
// same record without it, admitting exactly the shared effects key lists.
func TestTCBV0035_ReaderAdmitsVerdictPaths(t *testing.T) {
	v := verdictPathsRecord(t)
	effects := value(v, "effects")
	if len(effects.Obj.Keys) != len(taskswire.TicketEffectsKeys)+len(taskswire.TicketEffectsOptionalKeys) {
		t.Fatalf("fixture effects keys %v", effects.Obj.Keys)
	}
	got, e := decodeTicket(v)
	if e != nil {
		t.Fatalf("decode: %v", e)
	}
	if strings.Join(got.paths, ",") != "docs/specs/,internal/taskman/,internal/tasks/store/lease_gate.go" {
		t.Fatalf("the planner reserves on touchPaths, got %v", got.paths)
	}
	dropMember(effects, "verdictPaths")
	if _, e := decodeTicket(v); e != nil {
		t.Fatalf("decode without verdictPaths: %v", e)
	}
}

// TestTCBV0036_ReaderRefusesMalformedVerdictPaths: the reader applies the
// codec's rule: a present list is a nonempty, sorted, duplicate-free set of at
// most MaxTouchPaths valid paths, each covered
// by a touchPaths key under CAL-V0-021, and effects stays closed.
func TestTCBV0036_ReaderRefusesMalformedVerdictPaths(t *testing.T) {
	overBound := make([]string, taskswire.MaxTouchPaths+1)
	for i := range overBound {
		overBound[i] = fmt.Sprintf("docs/specs/%04d", i)
	}
	for name, member := range map[string]wire.Value{
		"empty":                        verdictStrings(),
		"null":                         {Kind: wire.KindNull},
		"outside":                      verdictStrings("lib/x.go"),
		"exact key covers only itself": verdictStrings("internal/tasks/store/lease_gate.go/x"),
		"prefix without slash":         verdictStrings("docs/specsx"),
		"invalid path":                 verdictStrings("docs/specs/../x"),
		"unsorted":                     verdictStrings("docs/specs/b", "docs/specs/a"),
		"duplicate":                    verdictStrings("docs/specs/a", "docs/specs/a"),
		"over the touch path bound":    verdictStrings(overBound...),
	} {
		v := verdictPathsRecord(t)
		setMember(value(v, "effects"), "verdictPaths", member)
		if _, e := decodeTicket(v); e == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	v := verdictPathsRecord(t)
	setMember(value(v, "effects"), "verdictPathz", verdictStrings("docs/specs/"))
	if _, e := decodeTicket(v); e == nil {
		t.Fatal("unknown effects member accepted")
	}
	for _, ok := range [][]string{{"docs/specs/"}, {"internal/tasks/store/lease_gate.go"}, {"internal/taskman/deep/x.go"}} {
		v := verdictPathsRecord(t)
		setMember(value(v, "effects"), "verdictPaths", verdictStrings(ok...))
		if _, e := decodeTicket(v); e != nil {
			t.Fatalf("covered %v refused: %v", ok, e)
		}
	}
}
