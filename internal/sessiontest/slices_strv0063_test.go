package sessiontest

import (
	"reflect"
	"strings"
	"testing"
)

// STR-V0-063: a selected Go package with a proven slice is placed as one
// `selected` unit carrying -run, the anchored pattern and the reason; one
// without a pattern stays whole with its reason; the manifest binds the
// pattern, the argv carries it, and only the whole units share an argv.
func TestSTRV0063SelectedSlicePlacement(t *testing.T) {
	input := fixtureInput()
	input.GoSelection = "run example.test/m/a example.test/m/b"
	input.GoSlices = map[string]GoSlice{
		"example.test/m/a": {Pattern: "^(TestA|TestA2)$", Reason: "selected: the tests of a/a_test.go"},
		"example.test/m/b": {Reason: "whole: DIRECT_SOURCE_CHANGE via b/b.go"},
	}
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []Unit{
		{Adapter: AdapterGo, Flag: "-run", Pattern: "^(TestA|TestA2)$", Reason: "selected: the tests of a/a_test.go", Slice: SliceSelected, Unit: "go:example.test/m/a"},
		{Adapter: AdapterGo, Reason: "whole: DIRECT_SOURCE_CHANGE via b/b.go", Slice: SliceWhole, Unit: "go:example.test/m/b"},
	}
	if m.Scope != ScopeNarrowed || !reflect.DeepEqual(m.Units, want) || m.SlicedUnits() != 1 {
		t.Fatalf("units %+v scope %s sliced %d", m.Units, m.Scope, m.SlicedUnits())
	}
	argv := []string{}
	for _, planned := range Invocations(m, input.Declared, 0, "", "") {
		argv = append(argv, strings.Join(planned.Argv, " "))
	}
	if len(argv) != 2 || !strings.HasSuffix(argv[0], " example.test/m/b") || strings.Contains(argv[0], "-run") ||
		!strings.HasSuffix(argv[1], " -run ^(TestA|TestA2)$ example.test/m/a") {
		t.Fatalf("argv %q", argv)
	}
	first, _ := Digest(m)
	input.GoSlices["example.test/m/a"] = GoSlice{Pattern: "^(TestA)$", Reason: "selected: the tests of a/a_test.go"}
	again, _ := Build(input)
	second, _ := Digest(again)
	if first == second {
		t.Fatal("the manifest does not bind the -run pattern")
	}
	// A FULL run places every package whole and carries no slice.
	input.GoSelection = "FALLBACK go.mod changed"
	full, _ := Build(input)
	for _, unit := range full.Units {
		if unit.Slice != SliceWhole {
			t.Fatalf("FULL placed %+v", unit)
		}
	}
	if full.SlicedUnits() != 0 {
		t.Fatal("FULL counts sliced units")
	}
}
