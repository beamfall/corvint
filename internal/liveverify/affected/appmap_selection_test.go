package affected_test

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestAppmapOnlyChangeSelectsOnlyJustifiedUnboundedReaders_V1_0999 pins the
// selection for a change confined to internal/appmap on this repository
// (V1-0999). The appmap package is selected directly, every other selection
// carries a witness that resolves under its own rule, and each
// UNBOUNDED_READER selection is a graph-wide unbounded reader with a recorded
// cause: its own unbounded reads, or a dependency whose code locates the
// repository root. A reader that is selected without such a cause, or a unit
// that declares its read scope and is selected as unbounded anyway, fails.
func TestAppmapOnlyChangeSelectsOnlyJustifiedUnboundedReaders_V1_0999(t *testing.T) {
	graph, _ := buildRepositoryGraph(t)
	const changed = "internal/appmap/verify.go"
	owner, found := graph.OwnerOf(changed)
	if !found {
		t.Fatalf("no unit owns %s", changed)
	}
	plan := affected.Select(graph, []string{changed})
	unbounded := graph.UnboundedReaders()
	direct, readers := false, 0
	for _, selection := range plan.Selected {
		if !witnessResolves(graph, selection) {
			t.Fatalf("%s has an unresolvable witness %+v", selection.UnitID, selection.Witness)
		}
		if selection.UnitID == owner && selection.Witness.Kind == affected.WitnessDirectSource {
			direct = true
		}
		if selection.Witness.Kind != affected.WitnessUnboundedReader {
			continue
		}
		readers++
		if !slices.Contains(unbounded, selection.UnitID) {
			t.Fatalf("%s is selected as an unbounded reader but the graph does not list it as one", selection.UnitID)
		}
		unit, _ := graph.Unit(selection.UnitID)
		if unit.ReadScoped {
			t.Fatalf("%s declares its read scope yet is selected as an unbounded reader", selection.UnitID)
		}
		if unit.UnboundedReads == "" && !reachesRootLocator(graph, selection.UnitID) {
			t.Fatalf("%s is selected as an unbounded reader with no recorded cause", selection.UnitID)
		}
	}
	if !direct {
		t.Fatalf("the owning unit %s is not selected DIRECT_SOURCE_CHANGE for %s", owner, changed)
	}
	if readers == 0 || len(plan.Excluded) == 0 {
		t.Fatalf("selected %d unbounded readers and excluded %d units; the case no longer exercises the narrowing", readers, len(plan.Excluded))
	}
	t.Logf("%s: selected=%d unbounded-readers=%d excluded=%d scope=%s", changed, len(plan.Selected), readers, len(plan.Excluded), plan.Scope)
}

// reachesRootLocator reports whether a unit's code or tests depend, through
// any import, test-import, advisory or exec edge, on a unit whose non-test
// code locates the repository root.
func reachesRootLocator(graph *affected.Graph, id string) bool {
	seen := map[string]bool{id: true}
	queue := []string{id}
	for len(queue) != 0 {
		unit, found := graph.Unit(queue[0])
		queue = queue[1:]
		if !found {
			continue
		}
		if unit.LocatesRoot {
			return true
		}
		for _, next := range slices.Concat(unit.Imports, unit.TestImports, unit.AdvisoryImports, unit.Execs) {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}
