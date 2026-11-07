package main

import (
	"context"
	"encoding/json"
	"reflect"
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
	t.Parallel()
	graph, err := affected.Build(context.Background(), t.TempDir(), byPathLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"docs/a.md", "docs/b.md"}
	union := affected.Select(graph, paths)
	result := affectedByPath(affectedReceipt{Revision: "r", Plan: union}, graph, paths)
	if len(result.Shared) != 1 || result.Shared[0].UnitID != "go:any" || result.Shared[0].Witness.Kind != affected.WitnessUnboundedReader {
		t.Fatalf("shared=%+v union=%+v", result.Shared, union.Selected)
	}
	if len(result.Paths) != 2 {
		t.Fatalf("paths=%+v", result.Paths)
	}
	for index, want := range []affectedByPathUnit{{UnitID: "go:a", TestCount: 1, Witness: affectedCompactWitness{Kind: affected.WitnessPathLiteralReader}}, {UnitID: "go:b", TestCount: 2, Witness: affectedCompactWitness{Kind: affected.WitnessPathLiteralReader}}} {
		entry := result.Paths[index]
		if entry.Path != paths[index] || entry.Shared != 1 || len(entry.Additions) != 1 || (entry.Additions[0].Kind != want.Kind || !reflect.DeepEqual(entry.Additions[0].Run, want.Run) || entry.Additions[0].UnitID != want.UnitID || entry.Additions[0].TestCount != want.TestCount || entry.Additions[0].Witness.Kind != want.Witness.Kind) {
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
	t.Parallel()
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

// TSP-V0-7311: one object decoder handles full, compact and by-path witnesses.
func TestWitnessFormsShareOneObjectDecoder(t *testing.T) {
	t.Parallel()
	graph, err := affected.Build(context.Background(), t.TempDir(), byPathLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"docs/a.md"})
	compact, _ := compactAffectedSelections(plan.Selected)
	byPath := affectedByPath(affectedReceipt{Plan: plan}, graph, []string{"docs/a.md"})
	for _, object := range []any{plan.Selected[0], compact[0], byPath.Shared[0], byPath.Paths[0].Additions[0]} {
		body, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Witness struct {
				Kind      string   `json:"kind"`
				DirtyPath string   `json:"dirtyPath"`
				ViaLength int      `json:"viaLength"`
				Via       []string `json:"via,omitempty"`
			} `json:"witness"`
		}
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("one decoder refused %s: %v", body, err)
		}
		// Full witnesses retain their frozen via array; compact forms carry
		// viaLength. Recover the same chain length with the shared decoder.
		if via := decoded.Witness.Via; via != nil {
			if decoded.Witness.ViaLength != 0 {
				t.Fatalf("full witness changed the frozen wire: %s", body)
			}
			decoded.Witness.ViaLength = len(via)
		}
		if decoded.Witness.Kind == "" || decoded.Witness.ViaLength < 1 {
			t.Fatalf("incomplete witness: %s", body)
		}
	}
}

// AFP-V0-990 / TSP-V0-7311: sharing preserves a real unbounded witness even
// when the union reaches that unit directly and has no unbounded cause.
func TestByPathSharedWitnessKeepsItsUnboundedCause(t *testing.T) {
	t.Parallel()
	graph, err := affected.Build(context.Background(), t.TempDir(), byPathLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"any/any.go", "docs/a.md"}
	union := affected.Select(graph, paths)
	for _, selection := range union.Selected {
		if selection.UnitID == "go:any" && selection.Witness.Cause != nil {
			t.Fatalf("fixture must reach go:any directly: %+v", selection)
		}
	}
	individual := affected.SelectLinked(graph, paths[1:])
	var want affectedCompactWitness
	for _, selection := range individual.Selected {
		if selection.UnitID == "go:any" {
			want = compactWitness(selection.Witness)
		}
	}
	if want.Kind != affected.WitnessUnboundedReader || want.Cause == nil || want.DirtyPath != "docs/a.md" {
		t.Fatalf("fixture has no unbounded explanation: %+v", want)
	}
	result := affectedByPath(affectedReceipt{Plan: union}, graph, paths)
	if len(result.Shared) != 1 || result.Shared[0].UnitID != "go:any" || !reflect.DeepEqual(result.Shared[0].Witness, want) {
		t.Fatalf("shared=%+v want witness=%+v", result.Shared, want)
	}
	for _, entry := range result.Paths {
		if entry.Shared != 1 {
			t.Fatalf("shared selection lost: %+v", entry)
		}
	}
}
