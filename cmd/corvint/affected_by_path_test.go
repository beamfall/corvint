package main

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// byPathLanguage is a fixture plugin: go:any reads unbounded paths, and
// go:a and go:b each read one literal documentation path.
type byPathLanguage struct{}

func (byPathLanguage) Name() string          { return "go" }
func (byPathLanguage) Owns(path string) bool { return strings.HasSuffix(path, ".go") }
func (byPathLanguage) ReadsAnyPath() bool    { return true }
func (byPathLanguage) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{Units: []affected.Unit{
		{ID: "go:any", Sources: []string{"any/any.go"}, Tests: []string{"any/any_test.go"}, UnboundedReads: "any/any_test.go calls os.Getwd"},
		{ID: "go:a", Sources: []string{"a/a.go"}, Tests: []string{"a/a_test.go"}, PathTokens: []string{"docs/a.md"}, PathTokensBounded: true},
		{ID: "go:b", Sources: []string{"b/b.go"}, Tests: []string{"b/b_test.go", "b/c_test.go"}, PathTokens: []string{"docs/b.md"}, PathTokensBounded: true},
	}}, nil
}

// AFP-V0-990 (V1-3648): --by-path lists the units every named path selects
// through an unbounded reader once, then only each path's own additions.
func TestAFPV0990ByPathSeparatesSharedUnboundedReaders(t *testing.T) {
	graph, err := affected.Build(context.Background(), t.TempDir(), byPathLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"docs/a.md", "docs/b.md"}
	union := affected.Select(graph, paths)
	result := affectedByPath(affectedReceipt{Revision: "r", Plan: union}, graph, paths)
	if len(result.Shared) != 1 || result.Shared[0].UnitID != "go:any" || result.Shared[0].Witness != affected.WitnessUnboundedReader {
		t.Fatalf("shared=%+v union=%+v", result.Shared, union.Selected)
	}
	if len(result.Paths) != 2 {
		t.Fatalf("paths=%+v", result.Paths)
	}
	for index, want := range []affectedByPathUnit{{UnitID: "go:a", TestCount: 1, Witness: affected.WitnessPathLiteralReader}, {UnitID: "go:b", TestCount: 2, Witness: affected.WitnessPathLiteralReader}} {
		entry := result.Paths[index]
		if entry.Path != paths[index] || entry.Shared != 1 || len(entry.Additions) != 1 || entry.Additions[0] != want {
			t.Errorf("path %d = %+v, want shared 1 and addition %+v", index, entry, want)
		}
	}
	text := renderAffectedByPathText(result)
	for _, line := range []string{"affected by-path scope=", "shared go:any tests=1 UNBOUNDED_READER\n", "path docs/a.md scope=", " shared=1/1 additions=1\n", "  added go:b tests=2 PATH_LITERAL_READER\n"} {
		if !strings.Contains(text, line) {
			t.Errorf("text lacks %q:\n%s", line, text)
		}
	}
	if strings.Count(text, "go:any") != 1 {
		t.Errorf("the shared unit is listed more than once:\n%s", text)
	}
}

// AFP-V0-990: --by-path needs a path list and is not the --full wire.
func TestAFPV0990ByPathArguments(t *testing.T) {
	for _, arguments := range [][]string{{"--by-path"}, {"--by-path", "--full", "--path", "a.go"}, {"--by-path", "--by-path", "--path", "a.go"}, {"--by-path=1", "--path", "a.go"}} {
		if _, err := parseAffectedOptions(arguments); err == nil {
			t.Errorf("%q accepted", arguments)
		}
	}
	invocation, err := parseAffectedOptions([]string{"--by-path", "--path", "a.go", "b.go", "--format", "text"})
	if err != nil || !invocation.ByPath || len(invocation.PathList) != 2 {
		t.Fatalf("%+v %v", invocation, err)
	}
}
