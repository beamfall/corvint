// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"slices"
	"strings"
	"testing"
)

// makeFixtureTest is a test file of package name with body before its test.
func makeFixtureTest(name, body string) string {
	return "package " + name + "\n\nimport \"testing\"\n\n" + body + "func TestX(t *testing.T) {}\n"
}

// requireQualified fails unless SelectQualified ends with want.
func requireQualified(t *testing.T, plan Receipt, root, want string) {
	t.Helper()
	got := SelectQualified(plan, fixtureModule, root)
	if got[len(got)-1] != want {
		t.Fatalf("SelectQualified ends with %q, want %q:\n%s", got[len(got)-1], want, strings.Join(got, "\n"))
	}
}

const makeRootMakefile = `all:
	go test ./...
here:
	go test .
`

// TestNCMV1013SelectorRootPatterns pins the driver's root encodings in the
// selector (NCM-V1-013): a recipe running `go test ./...` or `go test .` at
// the root selects every package, the root package among them, through the
// final qualified set as well; runPackages maps each driver encoding.
func TestNCMV1013SelectorRootPatterns(t *testing.T) {
	root, base := baseRepository(t, map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"root.go":               "package fixture\n",
		"root_test.go":          makeFixtureTest("fixture", ""),
		"tool/tool.go":          "package tool\n",
		"tool/tool_test.go":     makeFixtureTest("tool", ""),
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
		"Makefile":              makeRootMakefile,
	}, nil)
	every := "run " + fixtureModule + " " + modules("mkread", "tool")
	for _, tc := range []struct {
		name, from, to, targets, want string
	}{
		{"whole tree", "go test ./...", "go test -v ./...", "all", every},
		{"root package", "go test .\n", "go test -v .\n", "here", every},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeMakefile(t, root, strings.Replace(makeRootMakefile, tc.from, tc.to, 1))
			defer writeMakefile(t, root, makeRootMakefile)
			requireVerdict(t, makeEditPlan(base, "Makefile"), root, tc.want, "noncode make-targets Makefile -> "+tc.targets)
			requireQualified(t, makeEditPlan(base, "Makefile"), root, tc.want)
		})
	}
	index := &repositoryIndex{packages: map[string]*goPackage{"": {}, "tool": {}, "tool/sub": {}}}
	for _, tc := range []struct {
		paths []string
		want  []string
	}{
		{[]string{"..."}, []string{"", "tool", "tool/sub"}},
		{[]string{"./..."}, []string{"", "tool", "tool/sub"}},
		{[]string{"."}, []string{""}},
		{[]string{""}, []string{""}},
		{[]string{"root.go"}, []string{""}},
		{[]string{"tool/..."}, []string{"tool", "tool/sub"}},
		{[]string{"tool/x.go"}, []string{"tool"}},
		{[]string{"script/check.sh"}, []string{}},
	} {
		if got := index.runPackages(tc.paths); !slices.Equal(got, tc.want) {
			t.Errorf("runPackages(%q) = %q, want %q", tc.paths, got, tc.want)
		}
	}
}

// TestNCMV1018SelectorOrderMove pins the review's case in the selector:
// moving `RUN := go test $(PKG)` from between `PKG = ./a` and `PKG = ./b`
// to after both makes t run ./b instead of ./a while every per-variable
// assignment list and rule block stays the same, so it runs everything.
func TestNCMV1018SelectorOrderMove(t *testing.T) {
	before := "all:\n\techo all\nPKG = ./a\nRUN := go test $(PKG)\nPKG = ./b\nt:\n\t$(RUN)\n"
	root, base := baseRepository(t, map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"a/a.go":                "package a\n",
		"a/a_test.go":           makeFixtureTest("a", ""),
		"b/b.go":                "package b\n",
		"b/b_test.go":           makeFixtureTest("b", ""),
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
		"Makefile":              before,
	}, nil)
	writeMakefile(t, root, "all:\n\techo all\nPKG = ./a\nPKG = ./b\nRUN := go test $(PKG)\nt:\n\t$(RUN)\n")
	want := `FALLBACK never-narrowed-path Makefile: make edit: the order of make file statements changed at "assign PKG"`
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, want)
	requireQualified(t, makeEditPlan(base, "Makefile"), root, want)
}

// TestNCMV1019SelectorUnfollowedScript pins the review's case in the
// selector: check's recipe runs a script that runs `go test ./tool`, which
// no Go package names; changing its arguments runs everything although the
// Makefile's own reader would keep the selection nonempty.
func TestNCMV1019SelectorUnfollowedScript(t *testing.T) {
	before := "check:\n\tscript/check.sh\n"
	root, base := baseRepository(t, map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"tool/tool.go":          "package tool\n",
		"tool/tool_test.go":     makeFixtureTest("tool", ""),
		"script/check.sh":       "#!/bin/sh\ngo test ./tool\n",
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
		"Makefile":              before,
	}, nil)
	writeMakefile(t, root, "check:\n\tscript/check.sh --race\n")
	want := `FALLBACK never-narrowed-path Makefile: make edit: make target "check" runs a command or make goal that does not resolve`
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, want)
	requireQualified(t, makeEditPlan(base, "Makefile"), root, want)
}
