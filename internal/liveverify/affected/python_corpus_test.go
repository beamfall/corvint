// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"context"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/python"
)

// TestPythonSelectionNarrowsARealCorpus is the non-Go end-to-end proof
// (AFP-V0-186). It runs over the tracked layered corpus in
// testdata/python-corpus (10 modules, 21 test files): a one-file edit must
// narrow the full test set to exactly the tests whose imports reach the edited
// module, including the paired test of the module that depends on it. The
// 18-module corpus this test first named is not tracked, so it used to skip.
func TestPythonSelectionNarrowsARealCorpus(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "python-corpus"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := affected.BuildPartial(context.Background(), root, python.New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	everyTest := affected.Select(graph, nil)
	total := 0
	for _, exclusion := range everyTest.Excluded {
		unit, _ := graph.Unit(exclusion.UnitID)
		total += len(unit.Tests)
	}
	if total != 21 {
		t.Fatalf("python corpus carries %d tests, want 21", total)
	}
	const edited = "src/corpus/index.py"
	if _, owned := graph.OwnerOf(edited); !owned {
		t.Fatalf("%s is not owned by a unit", edited)
	}
	plan := affected.Select(graph, []string{edited})
	selected := plan.SelectedTests()
	sort.Strings(selected)
	want := []string{"tests/test_cache.py", "tests/test_cache_eviction.py", "tests/test_cli.py", "tests/test_cli_help.py",
		"tests/test_index.py", "tests/test_index_build.py", "tests/test_query.py", "tests/test_query_filters.py"}
	if !slices.Equal(selected, want) {
		t.Fatalf("editing %s selected %v, want %v", edited, selected, want)
	}
	t.Logf("python: editing %s selects %d of %d tests (%.0f%% narrowed)",
		edited, len(selected), total, 100*(1-float64(len(selected))/float64(total)))
}
