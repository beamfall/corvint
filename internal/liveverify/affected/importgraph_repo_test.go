package affected_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestRepositoryImportGraphReaders_V14409_V14362 pins the repository's own
// `go list` guards (AFP-V0-2200, AFP-V0-2201): internal/closureguard lists the
// closures of cmd/* binaries it names by computed strings, so it reads the
// whole module's import graph and an import-set change in any of them, such
// as dropping cmd/corvint-mcp's link to swiftimpact, selects it (V1-4409).
// V1-5071 moved that guard from swiftimpact to closureguard; the
// all-literal guard in internal/liveverify/provider/interlock lists
// cmd/corvint's closure, so its literal is an import-graph edge and no
// longer a command edge that every body edit in that closure follows
// (V1-4362).
func TestRepositoryImportGraphReaders_V14409_V14362(t *testing.T) {
	testsupport.RequireLiveTier(t)
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	const module = "go:github.com/Beamfall/corvint/"
	guard, ok := graph.Unit(module + "internal/closureguard")
	if !ok || !guard.ImportGraphReadsAll || slices.Contains(guard.Execs, module+"cmd/corvint-tasks") {
		t.Fatalf("internal/closureguard: all=%v execs=%v", guard.ImportGraphReadsAll, guard.Execs)
	}
	interlock, ok := graph.Unit(module + "internal/liveverify/provider/interlock")
	if !ok || !slices.Equal(interlock.ImportGraphReads, []string{module + "cmd/corvint"}) || slices.Contains(interlock.Execs, module+"cmd/corvint") {
		t.Fatalf("interlock: reads=%v execs=%v", interlock.ImportGraphReads, interlock.Execs)
	}
	mcp := "cmd/corvint-mcp/swift_impact.go" //corvint:fixture-path
	changed := graph.WithImportSetChanges(&affected.ImportSetChanges{Changed: map[string]string{module + "cmd/corvint-mcp": mcp}})
	selected := false
	for _, selection := range affected.Select(changed, []string{mcp}).Selected {
		selected = selected || selection.UnitID == guard.ID && selection.Witness.Kind == affected.WitnessImportGraphReader
	}
	if !selected {
		t.Error("an import-set change to cmd/corvint-mcp does not select internal/closureguard as an import-graph reader")
	}
	body := graph.WithImportSetChanges(&affected.ImportSetChanges{Changed: map[string]string{}})
	for _, selection := range affected.Select(body, []string{"internal/necessity/necessity.go"}).Selected { //corvint:fixture-path
		if selection.Witness.Kind == affected.WitnessBinaryExec && selection.UnitID == interlock.ID {
			t.Errorf("a body-only edit selects interlock through %+v", selection.Witness)
		}
	}
}
