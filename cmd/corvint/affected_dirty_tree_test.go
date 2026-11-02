package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// V1-4003: consecutive affected calls against the same base follow uncommitted
// edits made between them. A same-size edit that only swaps which package a
// file imports changes the next plan, and undoing it changes it back, so a
// reused graph cache entry never hides a dirty-tree change.
func TestAffectedFollowsDirtyEditsBetweenCalls(t *testing.T) {
	root, _ := selfBinaryRepository(t)
	importing := func(dependency string) string {
		return "package a\n\nimport _ \"example.test/self/" + dependency + "\"\n"
	}
	commitSelfBinaryFiles(t, root, map[string]string{
		"a/a.go": importing("c"), "a/a_test.go": "package a\n",
		"b/b.go": "package b\n", "b/b_test.go": "package b\n",
		"c/c.go": "package c\n", "c/c_test.go": "package c\n",
	})
	packages := func(step string) []string {
		t.Helper()
		receipt, _, stderr, code := runAffectedCLI(t, root, "--base", "HEAD")
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", step, code, stderr)
		}
		var names []string
		for _, item := range receipt["provider"].(map[string]any)["go"].(map[string]any)["packages"].([]any) {
			names = append(names, item.(string))
		}
		return names
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("b/b.go", "package b\n\nvar X = 1\n")
	if got := packages("b dirty"); !slices.Equal(got, []string{"example.test/self/b"}) {
		t.Fatalf("b dirty: packages %v, want b alone", got)
	}
	write("a/a.go", importing("b"))
	if got := packages("a imports b"); !slices.Equal(got, []string{"example.test/self/a", "example.test/self/b"}) {
		t.Fatalf("a now imports dirty b: packages %v, want a and b", got)
	}
	write("a/a.go", importing("c"))
	if got := packages("a restored"); !slices.Equal(got, []string{"example.test/self/b"}) {
		t.Fatalf("a restored: packages %v, want b alone", got)
	}
}
