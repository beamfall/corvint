package main

import (
	"strings"
	"testing"
)

// V1-3677: the host-adapter wrapper leads its failure with the failing node subtest and its
// diagnostics, not only the exit status.
func TestHostAdapterNodeTestFailuresNamesTheFailingSubtest(t *testing.T) {
	tap := strings.Join([]string{
		"TAP version 13",
		"# Subtest: passes",
		"ok 1 - passes",
		"  ---",
		"  duration_ms: 1.0",
		"  ...",
		"# Subtest: AHI-032 transport stays distinct",
		"not ok 2 - AHI-032 transport stays distinct",
		"  ---",
		"  error: 'Expected values to be strictly equal'",
		"  ...",
		"# Subtest: later",
		"ok 3 - later",
		"1..3",
	}, "\n")
	got := nodeTestFailures([]byte(tap))
	for _, want := range []string{"not ok 2 - AHI-032 transport stays distinct", "Expected values to be strictly equal"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary lacks %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"ok 1 - passes", "ok 3 - later"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("summary carries passing subtest %q:\n%s", unwanted, got)
		}
	}
	if got := nodeTestFailures([]byte("TAP version 13\n")); !strings.Contains(got, "none reported") {
		t.Fatalf("empty summary = %q", got)
	}
}
