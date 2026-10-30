package affected

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// frontierOfLanguages plans dirty over the given plugins and returns the scope and the
// LANGUAGE_FRONTIER details.
func frontierOfLanguages(t *testing.T, dirty []string, languages ...Language) string {
	t.Helper()
	graph, err := Build(context.Background(), t.TempDir(), languages...)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, dirty)
	frontier := []string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownLanguageFrontier {
			frontier = append(frontier, unknown.Detail)
		}
	}
	return fmt.Sprint(frontier)
}

// AFP-V0-581: a DEPENDENTS unit frontier still bears when the plan follows the
// unit's dependents (it changed, a dependency changed, it is a test-less reader
// of an unowned path, or it locates the root) and when it selects the unit as
// a reader of a dirty path, owned or not, by literal or by glob; a plan that
// selects it only as a test user or as an own-reason unbounded reader no
// longer inherits it.
func TestDependentsFrontierBearsOnlyWhenTheUnitsDependentsAreFollowed_AFP_V0_581(t *testing.T) {
	foreign := func(unit Unit) Unit {
		unit.Frontier, unit.FrontierBears = []string{"x:foreign"}, FrontierBearsOnDependents
		return unit
	}
	y := fake{name: "y", units: []Unit{{ID: "y:app", Sources: []string{"app.y"}, Tests: []string{"app_test.y"}}}}
	cases := []struct {
		name  string
		units []Unit
		dirty []string
		want  string
	}{
		{"dirty script", []Unit{foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}})}, []string{"s.x"}, "[x:foreign]"},
		{"dependency changed", []Unit{
			{ID: "x:lib", Sources: []string{"lib.x"}},
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}, Imports: []string{"x:lib"}}),
		}, []string{"lib.x"}, "[x:foreign]"},
		{"test-less reader of an unowned path", []Unit{
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, PathTokens: []string{"notes.md"}}),
		}, []string{"notes.md"}, "[x:foreign]"},
		{"root locator", []Unit{
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}, LocatesRoot: true, UnboundedReads: "s.x locates the root"}),
		}, []string{"app.y"}, "[x:foreign]"},
		{"reader of an owned path", []Unit{
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}, PathTokens: []string{"app.y"}}),
		}, []string{"app.y"}, "[x:foreign]"},
		{"test-less reader of an owned path", []Unit{
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, PathTokens: []string{"app.y"}}),
		}, []string{"app.y"}, "[x:foreign]"},
		{"glob reader of an owned path", []Unit{
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}, PathGlobs: []string{"*.y"}}),
		}, []string{"app.y"}, "[x:foreign]"},
		{"test user", []Unit{
			{ID: "x:lib", Sources: []string{"lib.x"}},
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}, TestImports: []string{"x:lib"}}),
		}, []string{"lib.x"}, "[]"},
		{"test user that also reads the dirty path by literal", []Unit{
			{ID: "x:lib", Sources: []string{"lib.x"}},
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}, TestImports: []string{"x:lib"}, PathTokens: []string{"lib.x"}}),
		}, []string{"lib.x"}, "[x:foreign]"},
		{"test user that also reads the dirty path by glob", []Unit{
			{ID: "x:lib", Sources: []string{"lib.x"}},
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}, TestImports: []string{"x:lib"}, PathGlobs: []string{"*.x"}}),
		}, []string{"lib.x"}, "[x:foreign]"},
		{"own-reason unbounded reader", []Unit{
			foreign(Unit{ID: "x:s", Sources: []string{"s.x"}, Tests: []string{"s_test.x"}, UnboundedReads: "s.x reads a computed path"}),
		}, []string{"app.y"}, "[]"},
	}
	for _, tc := range cases {
		if got := frontierOfLanguages(t, tc.dirty, fake{name: "x", units: tc.units}, y); got != tc.want {
			t.Errorf("%s: frontier %s, want %s", tc.name, got, tc.want)
		}
	}
}

// AFP-V0-581: only the empty and DEPENDENTS FrontierBears values are valid.
func TestFrontierBearsValuesAreValidated_AFP_V0_581(t *testing.T) {
	for _, bears := range []string{"SOMETIMES", "CHANGED"} {
		unit := Unit{ID: "x:u", Sources: []string{"u.x"}, Frontier: []string{"x:foreign"}, FrontierBears: bears}
		if _, err := Build(context.Background(), t.TempDir(), fake{name: "x", units: []Unit{unit}}); !errors.Is(err, ErrInvalidUnit) {
			t.Errorf("%s: err %v, want ErrInvalidUnit", bears, err)
		}
	}
}
