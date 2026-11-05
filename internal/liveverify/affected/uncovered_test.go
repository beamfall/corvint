// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"reflect"
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
