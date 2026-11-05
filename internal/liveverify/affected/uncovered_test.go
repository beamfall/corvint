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
