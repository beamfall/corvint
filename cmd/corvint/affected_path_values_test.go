package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// AFP-V0-509: one --path takes every following value up to the next option,
// and plans the same bytes as the one-value-per-flag form.
func TestAFPV0509PathTakesSeveralValues(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	receipt, several, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go", "mid/mid.go")
	if code != 0 || !slices.Equal(anyStrings(receipt["path_list"]), []string{"core/core.go", "mid/mid.go"}) {
		t.Fatalf("--path a b: exit %d stderr=%s path_list=%v", code, stderr, receipt["path_list"])
	}
	for _, options := range [][]string{
		{"--path", "core/core.go", "--path", "mid/mid.go"},
		{"--path=core/core.go", "--path", "mid/mid.go"},
		{"--path=mid/mid.go", "core/core.go"},
	} {
		_, stdout, stderr, code := runAffectedCLI(t, root, options...)
		if code != 0 || !bytes.Equal(stdout, several) {
			t.Fatalf("%v: exit %d stderr=%s\n%s\nwant\n%s", options, code, stderr, stdout, several)
		}
	}
	receipt, _, stderr, code = runAffectedCLI(t, root, "--path", "core/core.go", "--full")
	if code != 0 || receipt["profile"] != "affected-plan/0" || !slices.Equal(anyStrings(receipt["path_list"]), []string{"core/core.go"}) {
		t.Fatalf("--path a --full: exit %d stderr=%s receipt=%v", code, stderr, receipt)
	}
	for _, tc := range []struct {
		options []string
		want    string
	}{
		{[]string{"--path", "core/core.go", "-x"}, "unrecognized arguments: -x"},
		{[]string{"--path", "core/core.go", "--full", "mid/mid.go"}, "unrecognized arguments: mid/mid.go"},
		{[]string{"--path", "core/core.go", "/abs"}, "--path must be a repository-relative canonical path"},
		{[]string{"--path", "core/core.go", ""}, "--path must be a repository-relative canonical path"},
	} {
		_, stdout, stderr, code := runAffectedCLI(t, root, tc.options...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, tc.want) {
			t.Fatalf("%v: exit %d stdout=%s stderr=%s", tc.options, code, stdout, stderr)
		}
	}
}
