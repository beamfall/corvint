package sessiontest

import "testing"

// TestSTRV0548ReasonCodesPinTheProseShapes pins V1-3478: each prose shape
// `corvint test` writes for a Go unit maps to one stable code, the manifest
// unit carries it beside the prose, and an unknown shape is "other".
func TestSTRV0548ReasonCodesPinTheProseShapes(t *testing.T) {
	for reason, want := range map[string]string{
		"":                                    "",
		"selected: the tests of a/a_test.go":  ReasonSelectedChangedTests,
		"selected: the tests reaching a/a.go": ReasonSelectedReachingChange,
		"selected: no test reaches a/a.go; build and vet only": ReasonSelectedBuildOnly,
		"whole: attributed: whole a: x":                        ReasonAttributed,
		"whole: also selected by DEPENDENCY_PATH via b/b.go":   ReasonAlsoSelected,
		"whole: DIRECT_SOURCE_CHANGE via its own a/a_test.go is not separable from the other packages' changed test files": ReasonNotSeparable,
		"whole: DEPENDENCY_PATH via b/b.go (imports example/b)":                                                            "reached-via-dependency-path",
		"whole: UNBOUNDED_READER via AGENTS.md (reads the root)":                                                           "reached-via-unbounded-reader",
		"whole: DIRECT_SOURCE_CHANGE via a/a.go":                                                                           ReasonDirectSource,
		"whole: DIRECT_SOURCE_CHANGE via a/a.go (a/a.go changes more than plain functions)":                                ReasonNonFunctionEdit,
		"whole: DIRECT_SOURCE_CHANGE via a/a.go (F in a/a.go has a method value)":                                          ReasonReferenceSignature,
		"whole: DIRECT_SOURCE_CHANGE via a/a.go (every test function of the package reaches a/a.go)":                       ReasonEveryTestReaches,
		"whole: DIRECT_SOURCE_CHANGE via a/a.go (source slicing plans at most 16 packages)":                                ReasonSourceSliceBound,
		"whole: DIRECT_SOURCE_CHANGE via a/a.go (also selected by DEPENDENCY_PATH via b/b.go)":                             ReasonAlsoSelected,
		"whole: DIRECT_SOURCE_CHANGE via a/a.go (the -run pattern exceeds the bound)":                                      ReasonPatternBound,
		"whole: the -run pattern exceeds the bound":                                                                        ReasonPatternBound,
		"whole: a/a_test.go declares TestMain":                                                                             ReasonTestSideEffect,
		"whole: a/a_test.go declares init":                                                                                 ReasonTestSideEffect,
		"whole: a/a_test.go imports example/x for its side effects":                                                        ReasonTestSideEffect,
		"whole: a/a_test.go dot-imports example/x":                                                                         ReasonTestSideEffect,
		"whole: a/a_test.go initializes v with a call":                                                                     ReasonTestSideEffect,
		"whole: a/a_test.go cannot be read":                                                                                ReasonUnreadable,
		"whole: a/a_test.go does not parse":                                                                                ReasonUnreadable,
		"whole: a/a_test.go cannot be read at base":                                                                        ReasonUnreadable,
		"whole: a/a_test.go is not a file at base":                                                                         ReasonUnreadable,
		"whole: a/a_test.go uses helper declared in a/b_test.go":                                                           ReasonSharedDeclaration,
		"whole: a/a_test.go imports example/y, which no test file of the package imported at base":                         ReasonTestImportsChanged,
		"whole: a/b_test.go imported example/y at base, which no test file of the package imports now":                     ReasonTestImportsChanged,
		"whole: the changed test files declare no test function":                                                           ReasonNoTestFunction,
		"whole: the plan's unit is not in the graph":                                                                       ReasonNotInGraph,
		"whole: no dirty path is one of the unit's test files":                                                             ReasonNoChangedTestFile,
		"whole: a changed test file is not a test file of the unit":                                                        ReasonForeignTestFile,
		moduleReason:               ReasonNestedModule,
		"whole: a new prose shape": ReasonOther,
	} {
		if got := ReasonCode(reason); got != want {
			t.Errorf("ReasonCode(%q) = %q; want %q", reason, got, want)
		}
	}
	slices := map[string]GoSlice{"a": {Pattern: "^TestA$", Reason: "selected: the tests of a/a_test.go"}, "b": {Reason: "whole: a/b_test.go declares TestMain"}}
	if unit := goUnit(slices, "a", 0); unit.ReasonCode != ReasonSelectedChangedTests {
		t.Errorf("selected unit code = %q", unit.ReasonCode)
	}
	if unit := goUnit(slices, "b", 0); unit.ReasonCode != ReasonTestSideEffect || unit.Reason == "" {
		t.Errorf("whole unit = %+v; want the code beside the prose", unit)
	}
}
