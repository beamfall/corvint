package affected

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"testing"
)

// deepChain is a comb of depth units shaped like the perf-budget Ruby
// fixture: model n imports model n-1, a test-less service imports each model,
// and a test unit's tests import each model, so every chain is the model
// chain plus at most one unit (V1-2979).
func deepChain(t *testing.T, depth int) *Graph {
	t.Helper()
	units := make([]Unit, 0, 3*depth)
	for n := 1; n <= depth; n++ {
		model := fmt.Sprintf("x:m%05d", n)
		unit := Unit{ID: model, Sources: []string{fmt.Sprintf("m%05d.x", n)}, Tests: []string{fmt.Sprintf("m%05d_test.x", n)}, Imports: []string{}}
		if n > 1 {
			unit.Imports = []string{fmt.Sprintf("x:m%05d", n-1)}
		}
		units = append(units, unit,
			Unit{ID: fmt.Sprintf("x:s%05d", n), Sources: []string{fmt.Sprintf("s%05d.x", n)}, Imports: []string{model}},
			Unit{ID: fmt.Sprintf("x:t%05d", n), Tests: []string{fmt.Sprintf("t%05d_test.x", n)}, Imports: []string{}, TestImports: []string{model}})
	}
	graph, err := Build(context.Background(), t.TempDir(), fake{name: "x", units: units})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

// TestAFPV0641LinkedPlanMaterializesToSelect: AFP-V0-641 (V1-2979).
// SelectLinked holds each witness chain by reference, and MaterializeVia
// turns it into exactly Select's plan, whose chains are the model chain from
// the dirty unit to the selected one.
func TestAFPV0641LinkedPlanMaterializesToSelect(t *testing.T) {
	t.Parallel()
	for _, graph := range []*Graph{deepChain(t, 60), mustBuild(t, chain())} {
		for _, dirty := range [][]string{{"m00001.x"}, {"m00030.x", "t00002_test.x"}, {"core.x"}, {"mid.x", "solo_test.x"}} {
			full := Select(graph, dirty)
			linked := SelectLinked(graph, dirty)
			for index, selection := range linked.Selected {
				if selection.Witness.link != nil && selection.Witness.Via != nil {
					t.Fatalf("%v: %s carries both a link and a copy", dirty, selection.UnitID)
				}
				want := full.Selected[index].Witness
				if selection.Witness.ViaLength() != len(want.Via) || !slices.Equal(selection.Witness.Chain(), want.Via) {
					t.Fatalf("%v: %s chain %v, want %v", dirty, selection.UnitID, selection.Witness.Chain(), want.Via)
				}
			}
			for _, selection := range full.Selected {
				if selection.Witness.link != nil {
					t.Fatalf("%v: Select left %s held by reference", dirty, selection.UnitID)
				}
			}
			linked.MaterializeVia()
			if !reflect.DeepEqual(linked, full) {
				t.Fatalf("%v: materialized linked plan differs from Select", dirty)
			}
			encodedFull, err := full.Canonical()
			if err != nil {
				t.Fatal(err)
			}
			encodedLinked, err := linked.Canonical()
			if err != nil {
				t.Fatal(err)
			}
			if string(encodedFull) != string(encodedLinked) {
				t.Fatalf("%v: canonical plans differ", dirty)
			}
		}
	}
	plan := Select(deepChain(t, 5), []string{"m00001.x"})
	for _, selection := range plan.Selected {
		if selection.UnitID == "x:t00005" && !slices.Equal(selection.Witness.Via, []string{"x:m00001", "x:m00002", "x:m00003", "x:m00004", "x:m00005", "x:t00005"}) {
			t.Fatalf("test user chain %v", selection.Witness.Via)
		}
	}
}

func mustBuild(t *testing.T, language fake) *Graph {
	t.Helper()
	graph, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

// TestAFPV0641SharedPrefixSteps: AFP-V0-641. A chain step compares prefixes
// by content whether the chain is a copy or a shared reference, and walks a
// chain back to its first unit.
func TestAFPV0641SharedPrefixSteps(t *testing.T) {
	t.Parallel()
	copied := Witness{Via: []string{"a", "b", "c"}}
	shared := Witness{link: Witness{Via: []string{"a", "b"}}.extended("c")}
	other := Witness{Via: []string{"z", "b", "c"}}
	last, _ := copied.LastStep()
	linked, _ := shared.LastStep()
	foreign, _ := other.LastStep()
	if !last.SamePrefix(linked) || !linked.SamePrefix(last) || last.SamePrefix(foreign) || linked.SamePrefix(foreign) {
		t.Fatal("prefix comparison must follow content")
	}
	if previous, _ := linked.Previous(); previous.Unit() != "b" || previous.Index() != 1 || previous.SamePrefix(last) {
		t.Fatalf("previous step %s at %d", previous.Unit(), previous.Index())
	}
	if shared.viaHead() != "a" || copied.viaHead() != "a" || (Witness{}).viaHead() != "" {
		t.Fatal("chain head")
	}
	if _, ok := (Witness{}).LastStep(); ok {
		t.Fatal("an empty chain has no step")
	}
	if !slices.Equal(shared.Chain(), copied.Via) || shared.ViaLength() != 3 {
		t.Fatalf("shared chain %v", shared.Chain())
	}
}

// TestAFPV0641LinkedSelectionMemoryIsLinear: AFP-V0-641 (V1-2979). Selecting
// a comb four times as deep allocates about four times the memory with chains
// held by reference; copied chains allocate about sixteen times.
func TestAFPV0641LinkedSelectionMemoryIsLinear(t *testing.T) {
	allocated := func(graph *Graph) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		plan := SelectLinked(graph, []string{"m00001.x"})
		runtime.ReadMemStats(&after)
		if len(plan.Selected) == 0 {
			t.Fatal("nothing selected")
		}
		return after.TotalAlloc - before.TotalAlloc
	}
	small, large := allocated(deepChain(t, 1000)), allocated(deepChain(t, 4000))
	if ratio := float64(large) / float64(small); ratio > 7 {
		t.Fatalf("linked selection allocates %.1fx for 4x the depth (%d -> %d bytes), want linear", ratio, small, large)
	}
}
