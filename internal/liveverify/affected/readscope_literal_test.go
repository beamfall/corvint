package affected

import (
	"slices"
	"testing"
)

// scopedLiteralPlan plans dirty over chain() with x:solo moved to solo/,
// naming tokens, and declared with scope when declared is set.
func scopedLiteralPlan(t *testing.T, tokens []string, declared bool, scope []string, dirty ...string) Plan {
	t.Helper()
	graph, err := documentationGraph(t, "", func(language *fake) {
		solo := &language.units[3]
		solo.Sources = []string{"solo/solo.x"}
		solo.Tests = []string{"solo/solo_test.x"}
		solo.PathTokens = slices.Sorted(slices.Values(tokens))
		solo.ReadScoped = declared
		solo.ReadScope = scope
	})
	if err != nil {
		t.Fatal(err)
	}
	return Select(graph, dirty)
}

// AFP-V0-161 (1), (3); V1-2069: a unit whose declared read scope does not hold
// a Tasks store projection path is not its path-literal reader, so its
// literal no longer keeps the path unknown; undeclared, the same whole-name
// literal still selects the unit and keeps the AFP-V0-088 unknown.
func TestDeclaredReadScopeBoundsPathLiteralReaders_AFPV0161(t *testing.T) {
	ticket := ".taskman/tickets/V1-0001.json"
	tokens := []string{".taskman/tickets/V1-0001.json"}
	undeclared := scopedLiteralPlan(t, tokens, false, nil, ticket)
	if got := witnesses(undeclared)["x:solo"]; got.Kind != WitnessPathLiteralReader || !slices.Contains(unknownDetails(undeclared), "UNOWNED_DIRTY_PATH "+ticket) {
		t.Fatalf("undeclared: selected=%+v unknown=%v", undeclared.Selected, unknownDetails(undeclared))
	}
	for _, scope := range [][]string{nil, {"docs/"}} {
		declared := scopedLiteralPlan(t, tokens, true, scope, ticket)
		if len(declared.Selected) != 0 || len(declared.Unknown) != 0 || declared.Scope != ScopeBounded || !slices.Equal(nonSourcePaths(declared), []string{ticket}) {
			t.Fatalf("declared %v: scope=%s selected=%+v unknown=%v nonSource=%+v", scope, declared.Scope, declared.Selected, unknownDetails(declared), declared.NonSource)
		}
	}
}

// AFP-V0-161 (2): every reader the declaration allows is still selected as a
// path-literal reader: a path in the unit's subtree, a declared entry, a path
// under a declared subtree, an ancestor of an entry, and the declaration
// itself; the selection is the same as without the declaration.
func TestDeclaredReadScopeKeepsEveryInScopeLiteralReader_AFPV0161(t *testing.T) {
	scope := []string{".taskman/policy.json", "fixtures/"}
	for _, dirty := range []string{"solo/testdata/policy.json", ".taskman/policy.json", "fixtures/a/policy.json", ".taskman", ReadScopesPath} {
		tokens := []string{"policy.json", ".taskman", "fixtures", "testdata", "a", ".corvint/test-read-scopes.json"}
		undeclared := scopedLiteralPlan(t, tokens, false, nil, dirty)
		declared := scopedLiteralPlan(t, tokens, true, scope, dirty)
		if witnesses(undeclared)["x:solo"].Kind != WitnessPathLiteralReader || witnesses(declared)["x:solo"].Kind != WitnessPathLiteralReader {
			t.Fatalf("%s: undeclared=%+v declared=%+v", dirty, undeclared.Selected, declared.Selected)
		}
	}
}

// AFP-V0-161 (1); V1-1766: a new testdata/x/policy.json, named only by whole
// segments ("/x", "../x", a lone "policy.json"), selects an undeclared unit
// but not a declared one whose scope is elsewhere, and a declared unit adds no
// unknown entry and removes none.
func TestDeclaredReadScopeDropsOutOfScopeFixtureNamers_AFPV0161(t *testing.T) {
	dirty := "testdata/x/policy.json"
	for _, tokens := range [][]string{{"/x"}, {"../x"}, {"policy.json", "testdata", "x"}} {
		undeclared := scopedLiteralPlan(t, tokens, false, nil, dirty)
		declared := scopedLiteralPlan(t, tokens, true, []string{".taskman/policy.json"}, dirty)
		if witnesses(undeclared)["x:solo"].Kind != WitnessPathLiteralReader {
			t.Fatalf("%v undeclared: selected=%+v", tokens, undeclared.Selected)
		}
		if _, selected := witnesses(declared)["x:solo"]; selected || !slices.Equal(unknownDetails(declared), unknownDetails(undeclared)) {
			t.Fatalf("%v declared: selected=%+v unknown=%v, undeclared unknown=%v", tokens, declared.Selected, unknownDetails(declared), unknownDetails(undeclared))
		}
	}
}
