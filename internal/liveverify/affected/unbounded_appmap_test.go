package affected_test

import (
	json "encoding/json/v2"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestAFPV0180AppmapOnlyChangeSelectsOnlyRecordedUnboundedReaders pins the
// selection for a change confined to internal/appmap on the live repository
// (AFP-V0-180, V1-0999). The appmap package is selected by its own source,
// every other non-reader selection resolves under its rule, and every
// UNBOUNDED_READER selection is exactly a unit the AFP-V0-033 ledger records
// with a reason: a new unbounded reader, or a recorded one losing its reason,
// fails here before it widens every change's selection unexplained.
func TestAFPV0180AppmapOnlyChangeSelectsOnlyRecordedUnboundedReaders(t *testing.T) {
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".corvint", "unbounded-readers.json"))
	if err != nil {
		t.Fatalf("read the unbounded-reader ledger: %v", err)
	}
	var ledger struct {
		Units   []string          `json:"units"`
		Reasons map[string]string `json:"reasons"`
	}
	if err := json.Unmarshal(body, &ledger); err != nil {
		t.Fatalf("decode the unbounded-reader ledger: %v", err)
	}
	graph, _ := buildRepositoryGraph(t)
	const dirty = "internal/appmap/gitread.go"
	plan := affected.Select(graph, []string{dirty})

	readers := []string{}
	others := map[string]bool{}
	for _, selection := range plan.Selected {
		if selection.Witness.Kind != affected.WitnessUnboundedReader {
			if !witnessResolves(graph, selection) {
				t.Errorf("%s has an unresolvable witness %+v", selection.UnitID, selection.Witness)
			}
			others[selection.UnitID] = true
			continue
		}
		directory, ok := unboundedTestDirectory(graph, selection.UnitID)
		if !ok {
			t.Errorf("%s is selected as UNBOUNDED_READER but declares no test", selection.UnitID)
			continue
		}
		if !slices.Contains(ledger.Units, directory) || strings.TrimSpace(ledger.Reasons[directory]) == "" {
			t.Errorf("%s is selected as UNBOUNDED_READER for a change confined to internal/appmap, but .corvint/unbounded-readers.json records no reason for %s (AFP-V0-033)", selection.UnitID, directory)
		}
		if selection.Witness.DirtyPath != dirty || !slices.Equal(selection.Witness.Via, []string{selection.UnitID}) {
			t.Errorf("%s witness = %+v, want dirtyPath %s and via the unit alone", selection.UnitID, selection.Witness, dirty)
		}
		readers = append(readers, directory)
	}
	if !others["go:github.com/Beamfall/corvint/internal/appmap"] {
		t.Errorf("internal/appmap is not selected for its own source change")
	}

	want := []string{}
	for _, id := range graph.UnboundedReaders() {
		if others[id] {
			continue
		}
		if directory, ok := unboundedTestDirectory(graph, id); ok {
			want = append(want, directory)
		}
	}
	slices.Sort(readers)
	slices.Sort(want)
	if !slices.Equal(readers, want) {
		t.Errorf("UNBOUNDED_READER selections = %v, want the recorded unbounded readers not otherwise reached %v", readers, want)
	}
	t.Logf("appmap-only change: selected=%d unbounded=%d excluded=%d", len(plan.Selected), len(readers), len(plan.Excluded))
}

// unboundedTestDirectory names a unit as the AFP-V0-033 ledger does: by the
// directory of its tests, or by its one test file when that file is its
// identity.
func unboundedTestDirectory(graph *affected.Graph, id string) (string, bool) {
	unit, ok := graph.Unit(id)
	switch {
	case !ok || len(unit.Tests) == 0:
		return "", false
	case len(unit.Tests) == 1 && strings.HasSuffix(id, ":"+unit.Tests[0]):
		return unit.Tests[0], true
	default:
		return path.Dir(unit.Tests[0]), true
	}
}
