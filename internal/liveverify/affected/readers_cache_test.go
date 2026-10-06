package affected

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// namersOracle is namers before the run cache: componentRuns split afresh
// for every unit, token and dirty path.
func namersOracle(graph *Graph, dirtyPath string, parts []string) []string {
	ids := make([]string, 0)
	for _, id := range graph.order {
		for _, value := range graph.units[id].PathTokens {
			if namesPath(value, parts) {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids
}

// One run cache shared across every dirty path of a selection names exactly
// the units the uncached match names, for anchored, climbing, partial,
// lone-component and shared tokens, in either dirty-path order.
func TestNamersWithSharedRunCacheMatchesUncached(t *testing.T) {
	tokens := []string{
		"internal/%03d.go", "../../.corvint", "/etc/config.yaml", "testdata/", "./fixtures/a.json",
		"docs/specs/README.md", "README.md", "specs/REQ", "a//b/./c/../d", "", "/", "..", "x/y/z.txt",
	}
	graph := &Graph{units: map[string]Unit{}}
	for offset := range 24 {
		id := fmt.Sprintf("unit%02d", offset)
		unit := Unit{ID: id}
		for step := 0; step < 3; step++ {
			unit.PathTokens = append(unit.PathTokens, tokens[(offset*5+step*3)%len(tokens)])
		}
		graph.units[id] = unit
		graph.order = append(graph.order, id)
	}
	dirty := []string{
		"internal/007.go", "internal/sub/007.go", ".corvint/state", "a/.corvint/x", "etc/config.yaml",
		"testdata/in.txt", "fixtures/a.json", "docs/specs/README.md", "README.md", "specs/REQUIREMENTS.tsv",
		"a/b/c/d", "d", "x/y/z.txt", "z.txt", "unrelated/file.go",
	}
	matched := 0
	backward := slices.Clone(dirty)
	slices.Reverse(backward)
	for _, order := range [][]string{dirty, backward} {
		runs := runCache{}
		for _, dirtyPath := range order {
			parts := strings.Split(dirtyPath, "/")
			got, want := graph.namersWith(dirtyPath, runs), namersOracle(graph, dirtyPath, parts)
			if !slices.Equal(got, want) {
				t.Fatalf("%s: cached %v, uncached %v", dirtyPath, got, want)
			}
			if !slices.Equal(graph.namers(dirtyPath), want) {
				t.Fatalf("%s: namers differs from uncached", dirtyPath)
			}
			matched += len(want)
		}
	}
	if matched == 0 {
		t.Fatal("fixture names no unit; the comparison proves nothing")
	}
}
