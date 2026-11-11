// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"slices"
	"testing"
)

// selfDeclaredRegistry declares the registry itself by exact entry
// (AZU-V0-8910) beside one data entry, the shape of a change that adds an
// entry to this repository's .corvint/runtime-readers.json (V1-5009).
const selfDeclaredRegistry = `{"profile":"corvint-runtime-readers/0","entries":[` +
	`{"path":".corvint/runtime-readers.json","reason":"parsed by the planner through a path-literal constant; its checker declares it in its read scope"},` +
	`{"path":"data/catalog.json","reason":"embedded catalog"}]}`

// V1-5009 criterion 1 (AZU-V0-8910, AFP-V0-602 (2)): an edit to a registry
// that declares itself selects the registry's own readers, the planner unit
// that names it by literal and the checker whose declared read scope holds it,
// together with the ownership audit (AZU-V0-9900); it carries no
// UNOWNED_DIRTY_PATH and no unknown, and a unit that reads neither is excluded.
func TestSelfDeclaredRegistryEditSelectsItsReaders_V1_5009(t *testing.T) {
	graph, err := runtimeReadersGraph(t, selfDeclaredRegistry, func(language *fake) {
		language.units[0].PathTokens = []string{RuntimeReadersPath}
		language.units[3].Sources = []string{"checker/checker.x"}
		language.units[3].Tests = []string{"checker/checker_test.x"}
		language.units[3].ReadScoped = true
		language.units[3].ReadScope = []string{RuntimeReadersPath}
		language.units = append(language.units, Unit{ID: "x:audit", Sources: []string{"audit.x"}, Tests: []string{OwnershipAuditTestPath}})
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{RuntimeReadersPath})
	if plan.Scope != ScopeBounded || len(plan.Unknown) != 0 {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, unknownDetails(plan))
	}
	if !slices.Equal(groupPaths(plan, ClassDeclaredRuntimeReaders), []string{RuntimeReadersPath}) || len(groupPaths(plan, ClassCorvintDeclaration)) != 0 {
		t.Fatalf("nonSource=%+v", plan.NonSource)
	}
	got := witnesses(plan)
	for id, kind := range map[string]string{"x:core": WitnessPathLiteralReader, "x:solo": WitnessDeclaredReadScope} {
		if got[id].Kind != kind || got[id].DirtyPath != RuntimeReadersPath {
			t.Fatalf("%s: witness=%+v selected=%+v", id, got[id], plan.Selected)
		}
	}
	if _, hit := got["x:audit"]; !hit {
		t.Fatalf("ownership audit not selected: %+v", plan.Selected)
	}
	if _, hit := got["y:app"]; hit {
		t.Fatalf("non-reader selected: %+v", plan.Selected)
	}
}

// V1-5009 criterion 2 (AFP-V0-602 (2), (4); AZU-V0-8910; V1-3354): fault
// injection. A reader the declaration does not account for never makes the
// plan certain without selecting it: a literal reader of a registry that does
// not declare itself keeps UNOWNED_DIRTY_PATH; a new .corvint record beside a
// self-declaring registry, which no entry holds, keeps UNOWNED_DIRTY_PATH even
// though its reader names it; and a further reader of a self-declaring
// registry, by literal or by unbounded read, is selected with its witness.
// A reader that opens the registry only by a computed name raises no witness
// in any rule and is the trust AFP-V0-602 states in its failure modes; no
// fixture here can model it.
func TestSelfDeclaredRegistryKeepsUnregisteredReaderUnknown_V1_5009(t *testing.T) {
	unregistered := `{"profile":"corvint-runtime-readers/0","entries":[{"path":"data/catalog.json","reason":"embedded catalog"}]}`
	graph, err := runtimeReadersGraph(t, unregistered, func(language *fake) { language.units[3].PathTokens = []string{RuntimeReadersPath} })
	if err != nil {
		t.Fatal(err)
	}
	if plan := Select(graph, []string{RuntimeReadersPath}); plan.Scope != ScopeUnknown || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+RuntimeReadersPath) || len(plan.NonSource) != 0 {
		t.Fatalf("undeclared registry with a reader: scope=%s unknown=%v nonSource=%+v", plan.Scope, unknownDetails(plan), plan.NonSource)
	}

	added := ".corvint/new-record.json"
	graph, err = runtimeReadersGraph(t, selfDeclaredRegistry, func(language *fake) { language.units[3].PathTokens = []string{added} })
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{added, RuntimeReadersPath})
	if plan.Scope != ScopeUnknown || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+added) || !slices.Equal(groupPaths(plan, ClassDeclaredRuntimeReaders), []string{RuntimeReadersPath}) {
		t.Fatalf("new record: scope=%s unknown=%v nonSource=%+v", plan.Scope, unknownDetails(plan), plan.NonSource)
	}

	for name, tc := range map[string]struct {
		edit    func(*fake)
		witness string
	}{
		"literal reader":   {func(language *fake) { language.units[3].PathTokens = []string{RuntimeReadersPath} }, WitnessPathLiteralReader},
		"unbounded reader": {func(language *fake) { language.units[3].UnboundedReads = "root-locator" }, WitnessUnboundedReader},
	} {
		graph, err := runtimeReadersGraph(t, selfDeclaredRegistry, tc.edit)
		if err != nil {
			t.Fatal(err)
		}
		plan := Select(graph, []string{RuntimeReadersPath})
		if got := witnesses(plan)["x:solo"]; got.Kind != tc.witness || got.DirtyPath != RuntimeReadersPath {
			t.Fatalf("%s: reader not selected: %+v", name, plan.Selected)
		}
	}
}
