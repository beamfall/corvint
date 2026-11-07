// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

// AZU-V0-006: a proven coverage gap is explicit; declared and modelled callers
// are selected before classifying it, and undeclared inbound callers stay unknown.
func TestAZUV0006CoverageGap(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		callers                        []string
		frontier                       []string
		withTest, wantUnknown, wantGap bool
	}{
		{name: "isolated", wantGap: true},
		{name: "modelled caller", withTest: true},
		{name: "declared caller", callers: []string{"caller_test.go"}, withTest: true},
		{name: "unowned declared caller", callers: []string{"unmodelled.mjs"}, wantUnknown: true},
		{name: "undeclared inbound", frontier: []string{"go:hidden-caller"}, wantUnknown: true},
		{name: "inbound with known test", frontier: []string{"go:hidden-caller"}, withTest: true, wantUnknown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			units := []Unit{{ID: "go:unit", Sources: []string{"unit.go"}, Callers: tc.callers, Frontier: tc.frontier}}
			if tc.withTest {
				caller := Unit{ID: "go:caller", Tests: []string{"caller_test.go"}}
				if len(tc.callers) == 0 {
					caller.Imports = []string{"go:unit"}
				}
				units = append(units, caller)
			}
			graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: units})
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{"unit.go"})
			unknown := false
			for _, u := range plan.Unknown {
				if u.Reason == UnknownNoSelectableTest {
					unknown = true
				}
			}
			if unknown != tc.wantUnknown || (len(plan.Uncovered) == 1) != tc.wantGap {
				t.Fatalf("unknown=%v uncovered=%v", plan.Unknown, plan.Uncovered)
			}
			if tc.wantGap && !reflect.DeepEqual(plan.Uncovered, []Unknown{{Reason: UnknownNoSelectableTest, Detail: "go:unit"}}) {
				t.Fatal(plan.Uncovered)
			}
			if tc.withTest && !reflect.DeepEqual(plan.SelectedTests(), []string{"caller_test.go"}) {
				t.Fatal(plan.SelectedTests())
			}
			if !tc.wantUnknown && plan.Scope != ScopeBounded {
				t.Fatalf("scope=%s", plan.Scope)
			}
		})
	}
}

// AZU-V0-006: an owned, nonhidden source can be read without an import edge.
// The reader's callers must be selected before proving no test observes it.
func TestAZUV0006OwnedTestlessReaderCallers(t *testing.T) {
	for _, glob := range []bool{false, true} {
		reader := Unit{ID: "go:helper", Sources: []string{"helper/helper.go"}}
		if glob {
			reader.PathGlobs = []string{"data/*.go"}
		} else {
			reader.PathTokens = []string{"data/data.go"}
		}
		graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
			{ID: "go:data", Sources: []string{"data/data.go"}},
			reader,
			{ID: "go:root", Tests: []string{"root_test.go"}, TestImports: []string{"go:helper"}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		plan := Select(graph, []string{"data/data.go"})
		if plan.Scope != ScopeBounded || len(plan.Unknown) != 0 || len(plan.Uncovered) != 0 || !reflect.DeepEqual(plan.SelectedTests(), []string{"root_test.go"}) {
			t.Fatalf("glob=%v: scope=%s unknown=%v uncovered=%v selected=%v", glob, plan.Scope, plan.Unknown, plan.Uncovered, plan.SelectedTests())
		}
	}
}

// AZU-V0-006: an unresolved caller of a reader, including one behind a
// modelled caller, prevents proving that the changed source has no observer.
func TestAZUV0006ReaderCallerUncertainty(t *testing.T) {
	for _, kind := range []string{"literal", "glob", "scoped", "unbounded"} {
		for _, indirect := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/direct", true: "/caller-closure"}[indirect], func(t *testing.T) {
				reader := Unit{ID: "shell:check.sh", Sources: []string{"check.sh"}}
				switch kind {
				case "literal":
					reader.PathTokens = []string{"data/data.go"}
				case "glob":
					reader.PathGlobs = []string{"data/*.go"}
				case "scoped":
					reader.ReadScoped, reader.ReadScope = true, []string{"data/"}
				case "unbounded":
					reader.UnboundedReads = "check.sh reads a computed path"
				}
				reader.Callers = []string{"Makefile"}
				units := []Unit{reader}
				if indirect {
					units[0].Callers = []string{"caller.sh"}
					units = append(units, Unit{ID: "shell:caller.sh", Sources: []string{"caller.sh"}, Callers: []string{"Makefile"}})
				}
				graph, err := Build(context.Background(), t.TempDir(),
					fake{name: "go", units: []Unit{{ID: "go:data", Sources: []string{"data/data.go"}}}},
					fake{name: "shell", units: units})
				if err != nil {
					t.Fatal(err)
				}
				plan := Select(graph, []string{"data/data.go"})
				if plan.Scope != ScopeUnknown || len(plan.Uncovered) != 0 || !reflect.DeepEqual(plan.Unknown, []Unknown{{Reason: UnknownNoSelectableTest, Detail: "go:data"}}) {
					t.Fatalf("scope=%s unknown=%v uncovered=%v", plan.Scope, plan.Unknown, plan.Uncovered)
				}
			})
		}
	}
}

// AZU-V0-006: the owner's representative seed and a reader's chosen witness
// need not name the same changed file. Inspect every changed path it owns.
func TestAZUV0006MultipleChangedPathsReaderUncertainty(t *testing.T) {
	for _, kind := range []string{"literal", "glob", "scoped", "unbounded"} {
		t.Run(kind, func(t *testing.T) {
			reader := Unit{ID: "shell:check.sh", Sources: []string{"check.sh"}, Callers: []string{"Makefile"}}
			switch kind {
			case "literal":
				reader.PathTokens = []string{"data/b.go"}
			case "glob":
				reader.PathGlobs = []string{"data/b.*"}
			case "scoped":
				reader.ReadScoped, reader.ReadScope = true, []string{"data/b.go"}
			case "unbounded":
				reader.UnboundedReads = "check.sh reads a computed path"
			}
			graph, err := Build(context.Background(), t.TempDir(),
				fake{name: "go", units: []Unit{{ID: "go:data", Sources: []string{"data/a.go", "data/b.go"}}}},
				fake{name: "shell", units: []Unit{reader}})
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{"data/a.go", "data/b.go"})
			if plan.Scope != ScopeUnknown || len(plan.Uncovered) != 0 || !reflect.DeepEqual(plan.Unknown, []Unknown{{Reason: UnknownNoSelectableTest, Detail: "go:data"}}) {
				t.Fatalf("scope=%s unknown=%v uncovered=%v", plan.Scope, plan.Unknown, plan.Uncovered)
			}
		})
	}
}

// AZU-V0-005: newly owning hidden Go source must not remove a testless
// reader's callers, which read or execute it independently of Go imports.
func TestAZUV0005KeepsTestlessReadersCallers(t *testing.T) {
	for _, source := range []string{".github/pkg/source.go", "_private/pkg/source.go"} {
		graph, err := Build(context.Background(), t.TempDir(),
			fake{name: "go", units: []Unit{{ID: "go:source", Sources: []string{source}, Tests: []string{"source_test.go"}}}},
			fake{name: "shell", units: []Unit{
				{ID: "shell:reader", Sources: []string{"reader.sh"}, PathTokens: []string{source}},
				{ID: "shell:test", Tests: []string{"reader_test.sh"}, Imports: []string{"shell:reader"}},
			}})
		if err != nil {
			t.Fatal(err)
		}
		plan := Select(graph, []string{source})
		if !reflect.DeepEqual(plan.SelectedTests(), []string{"reader_test.sh", "source_test.go"}) {
			t.Fatalf("%s: tests=%v", source, plan.SelectedTests())
		}
	}
}

// AZU-V0-006 / TSP-V0-001: when a reader's own files also change, its
// own-files frontier bears on the batch even while proving another owner's gap.
func TestAZUV0006ReaderOwnFilesFrontierInBatch(t *testing.T) {
	graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
		{ID: "go:data", Sources: []string{"data/data.go"}},
		{ID: "go:reader", Sources: []string{"reader/linux.go"}, PathTokens: []string{"data/data.go"}, Frontier: []string{"go:build-constraint-variants"}, FrontierBears: FrontierBearsOnOwnFiles},
	}})
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"data/data.go", "reader/linux.go"})
	want := Unknown{Reason: UnknownNoSelectableTest, Detail: "go:data"}
	for _, unknown := range plan.Unknown {
		if unknown == want {
			return
		}
	}
	t.Fatalf("reader's changed own files must prevent a coverage proof: unknown=%v uncovered=%v", plan.Unknown, plan.Uncovered)
}

// AZU-V0-006 (selbatch fix round 2): a testless helper whose declared read
// scope holds an untested owner's file observes it only through its callers,
// so they are selected before the coverage proof counts them. A scoped reader
// with its own tests stays untraversed (AFP-V0-021), a helper scoped elsewhere
// and an unrelated package stay excluded, and a dirty declaration alone
// carries no caller.
func TestAZUV0006TestlessScopedReaderCallers(t *testing.T) {
	base := []Unit{
		{ID: "go:data", Sources: []string{"data/data.go"}},
		{ID: "go:helper", Sources: []string{"helper/helper.go"}, ReadScoped: true, ReadScope: []string{"data/"}},
		{ID: "go:consumer", Tests: []string{"consumer/consumer_test.go"}, TestImports: []string{"go:helper"}},
		{ID: "go:unrelated", Sources: []string{"unrelated/unrelated.go"}, Tests: []string{"unrelated/unrelated_test.go"}},
	}
	controls := []Unit{
		{ID: "go:caller", Sources: []string{"caller/caller.go"}, Tests: []string{"caller/caller_test.go"}, Imports: []string{"go:helper"}},
		{ID: "go:tested", Sources: []string{"tested/tested.go"}, Tests: []string{"tested/tested_test.go"}, ReadScoped: true, ReadScope: []string{"data/"}},
		{ID: "go:testeduser", Tests: []string{"testeduser/testeduser_test.go"}, TestImports: []string{"go:tested"}},
		{ID: "go:elsewhere", Sources: []string{"elsewhere/elsewhere.go"}, ReadScoped: true, ReadScope: []string{"other/"}},
		{ID: "go:elsewhereuser", Tests: []string{"elsewhereuser/elsewhereuser_test.go"}, TestImports: []string{"go:elsewhere"}},
	}
	for _, tc := range []struct {
		name     string
		units    []Unit
		selected []string
		excluded []string
	}{
		{name: "test-only caller", units: base, selected: []string{"consumer/consumer_test.go"}, excluded: []string{"go:unrelated"}},
		{
			name:     "caller closure and controls",
			units:    append(slices.Clone(base), controls...),
			selected: []string{"caller/caller_test.go", "consumer/consumer_test.go", "tested/tested_test.go"},
			excluded: []string{"go:elsewhereuser", "go:testeduser", "go:unrelated"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: tc.units})
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{"data/data.go"})
			if plan.Scope != ScopeBounded || len(plan.Unknown) != 0 || len(plan.Uncovered) != 0 || !reflect.DeepEqual(plan.SelectedTests(), tc.selected) {
				t.Fatalf("scope=%s unknown=%v uncovered=%v selected=%v, want %v", plan.Scope, plan.Unknown, plan.Uncovered, plan.SelectedTests(), tc.selected)
			}
			var excluded []string
			for _, exclusion := range plan.Excluded {
				excluded = append(excluded, exclusion.UnitID)
			}
			if !reflect.DeepEqual(excluded, tc.excluded) {
				t.Fatalf("excluded=%v, want %v", excluded, tc.excluded)
			}
			for _, selection := range plan.Selected {
				if selection.UnitID == "go:consumer" && (selection.Witness.DirtyPath != "data/data.go" || !reflect.DeepEqual(selection.Witness.Via, []string{"go:helper", "go:consumer"})) {
					t.Fatalf("consumer witness %+v", selection.Witness)
				}
			}
		})
	}
	graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: append(slices.Clone(base), controls...)})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range Select(graph, []string{ReadScopesPath}).SelectedTests() {
		if test == "consumer/consumer_test.go" || test == "caller/caller_test.go" {
			t.Fatalf("a dirty read-scope declaration carried the helper's callers: %s", test)
		}
	}
}

// AZU-V0-006 (selbatch fix round 2): the coverage proof counts a caller only
// when the plan runs it. A caller the plan leaves out keeps the changed unit
// unknown when nothing that runs observes it; a reader with its own tests is
// still enough; and a unit additive narrowing alone leaves out cannot observe
// the change (ADN-V0-005), so the gap stays a gap.
func TestAZUV0006CoverageCountsOnlyRunCallers(t *testing.T) {
	gap := []Unknown{{Reason: UnknownNoSelectableTest, Detail: "go:data"}}
	for _, tc := range []struct {
		name               string
		units              []Unit
		additive           []GoAdditive
		dirty              string
		selected           []string
		unknown, uncovered []Unknown
		scope              string
	}{
		{
			name: "exec user of a testless reader",
			units: []Unit{
				{ID: "go:data", Sources: []string{"data/data.go"}},
				{ID: "go:cmd/tool", Sources: []string{"cmd/tool/main.go"}, PathTokens: []string{"data/data.go"}},
				{ID: "go:runner", Sources: []string{"runner/runner.go"}, Tests: []string{"runner/runner_test.go"}, Execs: []string{"go:cmd/tool"}},
			},
			dirty: "data/data.go", selected: []string{}, unknown: gap, scope: ScopeUnknown,
		},
		{
			name: "reader with its own tests",
			units: []Unit{
				{ID: "go:data", Sources: []string{"data/data.go"}},
				{ID: "go:reader", Sources: []string{"reader/reader.go"}, Tests: []string{"reader/reader_test.go"}, PathTokens: []string{"data/data.go"}},
				{ID: "go:readeruser", Tests: []string{"readeruser/readeruser_test.go"}, TestImports: []string{"go:reader"}},
			},
			dirty: "data/data.go", selected: []string{"reader/reader_test.go"}, scope: ScopeBounded,
		},
		{
			name: "additive narrowing",
			units: []Unit{
				{ID: "go:data", Sources: []string{"data/data.go", "data/fresh.go"}},
				{ID: "go:unref", Sources: []string{"unref/unref.go"}, Tests: []string{"unref/unref_test.go"}, Imports: []string{"go:data"}},
			},
			additive: []GoAdditive{{Unit: "go:data", State: GoAdditiveNarrowed, Paths: []string{"data/fresh.go"}, Declarations: []string{"Fresh"}, Unreferenced: []string{"go:unref"}}},
			dirty:    "data/fresh.go", selected: []string{}, uncovered: gap, scope: ScopeBounded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: tc.units})
			if err != nil {
				t.Fatal(err)
			}
			if tc.additive != nil {
				graph = graph.WithGoAdditive(tc.additive)
			}
			plan := Select(graph, []string{tc.dirty})
			unknown := tc.unknown
			if unknown == nil {
				unknown = []Unknown{}
			}
			if plan.Scope != tc.scope || !reflect.DeepEqual(plan.Unknown, unknown) || !reflect.DeepEqual(plan.Uncovered, tc.uncovered) || !reflect.DeepEqual(plan.SelectedTests(), tc.selected) {
				t.Fatalf("scope=%s unknown=%v uncovered=%v selected=%v", plan.Scope, plan.Unknown, plan.Uncovered, plan.SelectedTests())
			}
		})
	}
}
