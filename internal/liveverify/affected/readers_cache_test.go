package affected

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// namesPathOracle is namesPath before V1-1879, kept apart from pathToken so
// the comparison does not share the matcher it checks.
func namesPathOracle(value string, parts []string) bool {
	for _, candidate := range componentRuns(value) {
		first := 0
		if len(candidate.components) == 1 && !strings.HasPrefix(value, "/") && !strings.Contains(value, "..") {
			first = len(parts) - 1
		}
		for offset := first; offset+len(candidate.components) <= len(parts); offset++ {
			if candidate.matchesAt(parts[offset:]) {
				return true
			}
		}
	}
	return false
}

// namersOracle is namers before the token precompute: componentRuns split
// afresh for every unit, token and dirty path.
func namersOracle(graph *Graph, dirtyPath string) []string {
	parts := strings.Split(dirtyPath, "/")
	ids := make([]string, 0)
	for _, id := range graph.order {
		for _, value := range graph.units[id].PathTokens {
			if namesPathOracle(value, parts) {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids
}

// readersOracle is readers before V1-1879: every unit rescanned for every
// dirty path, in dirty-path order.
func readersOracle(graph *Graph, reached map[string]Witness, dirty []string) map[string]bool {
	narrowed := make(map[string]bool)
	for _, dirtyPath := range dirty {
		for _, id := range namersOracle(graph, dirtyPath) {
			if _, seen := reached[id]; seen {
				continue
			}
			if graph.resolves(id, dirtyPath) {
				reached[id] = Witness{Kind: WitnessPathLiteralReader, DirtyPath: dirtyPath, Via: []string{id}}
				continue
			}
			if dirtyPath == ChangeEvidencePath || len(graph.units[id].Frontier) == 0 {
				continue
			}
			_, owned := graph.owner[dirtyPath]
			narrowed[id] = narrowed[id] || !owned
		}
	}
	return narrowed
}

// The unit-major readers reaches the same units with the same smallest
// witness and narrows the same frontier reasons as the dirty-major scan, for
// anchored, climbing, partial, lone-component and shared tokens held by Go and
// non-Go units, with and without units already reached (V1-1879).
func TestReadersMatchesDirtyMajorScan(t *testing.T) {
	tokens := []string{
		"internal/%03d.go", "../../.corvint", "/etc/config.yaml", "testdata/", "./fixtures/a.json",
		"docs/specs/README.md", "README.md", "specs/REQ", "a//b/./c/../d", "x/y/z.txt",
		".corvint/change.cem.json", "change.cem.json", "z.txt",
	}
	directories := []string{"internal", "a/b", "docs", "x"}
	graph := &Graph{units: map[string]Unit{}}
	for offset := range 30 {
		id := fmt.Sprintf("unit%02d", offset)
		if offset%3 != 0 {
			id = "go:" + id
		}
		unit := Unit{ID: id, Frontier: []string{"frontier-" + id}}
		unit.Sources = []string{directories[offset%len(directories)] + "/f.go"}
		for step := 0; step < 3; step++ {
			unit.PathTokens = append(unit.PathTokens, tokens[(offset*5+step*3)%len(tokens)])
		}
		graph.units[id] = unit
		graph.order = append(graph.order, id)
	}
	for id, unit := range map[string]Unit{
		"go:narrow-file":   {Sources: []string{"x/f.go"}, PathTokens: []string{"README.md"}},
		"go:narrow-later":  {Sources: []string{"docs/f.go"}, PathTokens: []string{"z.txt", "x/y/z.txt"}},
		"go:root":          {Sources: []string{"r/f.go"}, PathTokens: []string{"/", ""}},
		"go:climb":         {Sources: []string{"a/b/f.go"}, PathTokens: []string{".."}},
		"tool:empty-token": {PathTokens: []string{""}},
	} {
		unit.ID, unit.Frontier = id, []string{"frontier-" + id}
		graph.units[id] = unit
		graph.order = append(graph.order, id)
	}
	slices.Sort(graph.order)
	dirty := []string{
		".corvint/change.cem.json", ".corvint/state", "README.md", "a/.corvint/x", "a/b/c/d", "d",
		"docs/specs/README.md", "etc/config.yaml", "fixtures/a.json", "internal/007.go", "internal/sub/007.go", "other/README.md", "q/z.txt",
		"specs/REQUIREMENTS.tsv", "testdata/in.txt", "unrelated/file.go", "x/y/z.txt", "z.txt",
	}
	for _, prior := range []map[string]Witness{{}, {graph.order[1]: {Kind: "PRIOR"}, graph.order[4]: {Kind: "PRIOR"}}} {
		got, want := maps.Clone(prior), maps.Clone(prior)
		gotNarrowed, _ := graph.readers(got, dirty)
		wantNarrowed := readersOracle(graph, want, dirty)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("reached %v, dirty-major %v", got, want)
		}
		if fmt.Sprint(gotNarrowed) != fmt.Sprint(wantNarrowed) {
			t.Fatalf("narrowed %v, dirty-major %v", gotNarrowed, wantNarrowed)
		}
		if len(want) == len(prior) || len(wantNarrowed) == 0 {
			t.Fatalf("fixture reaches %d and narrows %d; the comparison proves too little", len(want)-len(prior), len(wantNarrowed))
		}
	}
	for _, dirtyPath := range dirty {
		if got, want := graph.namers(dirtyPath), namersOracle(graph, dirtyPath); !slices.Equal(got, want) {
			t.Fatalf("%s: namers %v, oracle %v", dirtyPath, got, want)
		}
	}
}
