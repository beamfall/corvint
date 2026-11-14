// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// TestNCMV1019SelectorRecipeShell pins V1-5551 in the selector: a recipe
// make hands to a SHELL the scan does not recognize runs whatever that
// program does, here `go test ./tool`, so changing the recipe's `echo` runs
// everything with the recipe-shell reason (NCM-V1-019), from Select and
// SelectQualified alike, although the Makefile's own reader would keep the
// selection nonempty.
func TestNCMV1019SelectorRecipeShell(t *testing.T) {
	before := "SHELL = ./tools/test-shell\ncheck:\n\techo hi\n"
	root, base := baseRepository(t, map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"tool/tool.go":          "package tool\n",
		"tool/tool_test.go":     makeFixtureTest("tool", ""),
		"tools/test-shell":      "#!/bin/sh\ngo test ./tool\n",
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
		"Makefile":              before,
	}, nil)
	writeMakefile(t, root, strings.Replace(before, "echo hi", "echo hello", 1))
	want := `FALLBACK never-narrowed-path Makefile: make edit: make target "check" runs a command or make goal that does not resolve: make:recipe-shell-uncertified`
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, want)
	requireQualified(t, makeEditPlan(base, "Makefile"), root, want)
}

// makeChdirMakefile has one target per `cd` form of V1-5552 and one that
// names a package directory that does not exist.
const makeChdirMakefile = `dashes:
	cd -- tool && go test ./b
logical:
	cd -L tool && go test ./b
physical:
	cd -P tool && go test ./b
both:
	cd -LP -- tool && go test ./b
other:
	cd -e tool && go test ./b
home:
	cd && go test ./b
previous:
	cd - && go test ./b
missing:
	go test ./nope
`

// TestNCMV1019SelectorChdirOptions pins V1-5552 in the selector: cd with
// `--`, `-L` or `-P` changes to its directory operand, so a change after it
// selects that directory's package; any other option, no operand, or `-`
// leaves cd unfollowed (NCM-V1-019); and a go package selector that names
// no package in the repository runs everything rather than dropping out of
// the selection (NCM-V1-013).
func TestNCMV1019SelectorChdirOptions(t *testing.T) {
	root, base := baseRepository(t, map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"tool/b/b.go":           "package b\n",
		"tool/b/b_test.go":      makeFixtureTest("b", ""),
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
		"Makefile":              makeChdirMakefile,
	}, nil)
	defer writeMakefile(t, root, makeChdirMakefile)
	edit := func(target string) string {
		return strings.Replace(makeChdirMakefile, target+":\n\t", target+":\n\t: ; ", 1)
	}
	for _, target := range []string{"dashes", "logical", "physical", "both"} {
		writeMakefile(t, root, edit(target))
		want := []string{"mkread", "tool/b"}
		requireVerdict(t, makeEditPlan(base, "Makefile"), root, "run "+modules(want...), "noncode make-targets Makefile -> "+target)
		requireQualified(t, planSelecting(makeEditPlan(base, "Makefile"), want...), root, "run "+modules(want...))
	}
	for _, target := range []string{"other", "home", "previous"} {
		writeMakefile(t, root, edit(target))
		want := `FALLBACK never-narrowed-path Makefile: make edit: make target "` + target + `" runs a command or make goal that does not resolve: shell:unfollowed-command:cd`
		requireVerdict(t, makeEditPlan(base, "Makefile"), root, want)
		requireQualified(t, makeEditPlan(base, "Makefile"), root, want)
	}
	writeMakefile(t, root, edit("missing"))
	want := `FALLBACK never-narrowed-path Makefile: make edit: a changed make target runs Go package "nope", which names no package of the repository`
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, want)
	requireQualified(t, makeEditPlan(base, "Makefile"), root, want)
}

// TestNCMV1013SelectorNestedMakefileSelectors pins that the selector check
// of V1-5552 reads a nested make file's selectors per scan directory: the
// driver scans its recipes both from its own directory and from the root,
// and a selector that names a package from either one is not missing.
func TestNCMV1013SelectorNestedMakefileSelectors(t *testing.T) {
	before := "check:\n\tgo test ./x\n"
	root, base := baseRepository(t, map[string]string{
		"go.mod":                  "module " + fixtureModule + "\n\ngo 1.27\n",
		"sub/x/x.go":              "package x\n",
		"sub/x/x_test.go":         makeFixtureTest("x", ""),
		"subread/subread_test.go": makeFixtureTest("subread", "var makefile = \"../sub/Makefile\"\n\n"),
		"sub/Makefile":            before,
		"other/other.go":          "package other\n",
		"other/other_test.go":     makeFixtureTest("other", ""),
	}, nil)
	if err := os.WriteFile(filepath.Join(root, "sub", "Makefile"), []byte("check:\n\tgo test -v ./x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{"sub/x", "subread"}
	requireVerdict(t, makeEditPlan(base, "sub/Makefile"), root, "run "+modules(want...), "noncode make-targets sub/Makefile -> check")
}

// TestNCMV1019SelectorMakeCommandLine pins the recursive make invocation's
// command line (V1-5551 class): a variable assignment on it, SHELL or one a
// recipe reads, and an option outside the closed list, such as --eval, leave
// the invocation unfollowed, so a change to the target that runs it selects
// everything (NCM-V1-019); an option on that list keeps it narrowed.
func TestNCMV1019SelectorMakeCommandLine(t *testing.T) {
	files := map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"tool/tool.go":          "package tool\n",
		"tool/tool_test.go":     makeFixtureTest("tool", ""),
		"tools/test-shell":      "#!/bin/sh\ngo test ./tool\n",
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
	}
	for _, tc := range []struct {
		name, makefile, reason string
	}{
		{"shell", "top:\n\t$(MAKE) SHELL=./tools/test-shell check\ncheck:\n\techo hi\n", "make:command-line-assignment:SHELL"},
		{"variable", "top:\n\t$(MAKE) PKG=./tool check\ncheck:\n\tgo test $(PKG)\n", "make:command-line-assignment:PKG"},
		{"eval", "top:\n\t$(MAKE) '--eval=x: ; go test ./tool' x\ncheck:\n\techo hi\n", "make:unfollowed-option:--eval"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files["Makefile"] = tc.makefile
			root, base := baseRepository(t, files, nil)
			writeMakefile(t, root, strings.Replace(tc.makefile, "top:\n\t", "top:\n\t: ; ", 1))
			want := `FALLBACK never-narrowed-path Makefile: make edit: make target "top" runs a command or make goal that does not resolve: ` + tc.reason
			requireVerdict(t, makeEditPlan(base, "Makefile"), root, want)
			requireQualified(t, makeEditPlan(base, "Makefile"), root, want)
		})
	}
	files["Makefile"] = "top:\n\t$(MAKE) -s --no-print-directory -k -j 4 check\ncheck:\n\tgo test ./tool\n"
	root, base := baseRepository(t, files, nil)
	writeMakefile(t, root, strings.Replace(files["Makefile"], "top:\n\t", "top:\n\t: ; ", 1))
	want := []string{"mkread", "tool"}
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, "run "+modules(want...), "noncode make-targets Makefile -> top")
}

// TestNCMV1019SelectorSubshellChdir pins that a cd inside a subshell changes
// the directory of that subshell alone: the go selector after the closing
// parenthesis reads the make file's directory again, while a `{ ...; }`
// group runs in the shell itself and keeps the cd (NCM-V1-019).
func TestNCMV1019SelectorSubshellChdir(t *testing.T) {
	files := map[string]string{
		"go.mod":                  "module " + fixtureModule + "\n\ngo 1.27\n",
		"subread/subread_test.go": makeFixtureTest("subread", "var makefile = \"../sub/Makefile\"\n\n"),
	}
	for _, directory := range []string{"sub/c", "sub/tool", "sub/tool/c"} {
		name := path.Base(directory)
		files[directory+"/"+name+".go"] = "package " + name + "\n"
		files[directory+"/"+name+"_test.go"] = makeFixtureTest(name, "")
	}
	for _, tc := range []struct {
		name, recipe string
		want         []string
	}{
		{"subshell", "(cd tool && go vet .); go test ./c", []string{"sub/c", "sub/tool", "subread"}},
		{"group", "{ cd tool && go vet .; }; go test ./c", []string{"sub/tool", "sub/tool/c", "subread"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := "t:\n\t" + tc.recipe + "\n"
			files["sub/Makefile"] = before
			root, base := baseRepository(t, files, nil)
			if err := os.WriteFile(filepath.Join(root, "sub", "Makefile"), []byte("t:\n\t: ; "+tc.recipe+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			requireVerdict(t, makeEditPlan(base, "sub/Makefile"), root, "run "+modules(tc.want...), "noncode make-targets sub/Makefile -> t")
		})
	}
}

// TestNCMV1019SelectorSubshellPrecision pins that a cd scoped to a subshell
// no longer leaks into the selector after it: `go test ./other` names the
// root's other package, not tool/other, and the edit narrows.
func TestNCMV1019SelectorSubshellPrecision(t *testing.T) {
	before := "t:\n\t(cd tool && go vet .); go test ./other\n"
	root, base := baseRepository(t, map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"tool/tool.go":          "package tool\n",
		"tool/tool_test.go":     makeFixtureTest("tool", ""),
		"other/other.go":        "package other\n",
		"other/other_test.go":   makeFixtureTest("other", ""),
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
		"Makefile":              before,
	}, nil)
	writeMakefile(t, root, strings.Replace(before, "t:\n\t", "t:\n\t: ; ", 1))
	want := []string{"mkread", "other", "tool"}
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, "run "+modules(want...), "noncode make-targets Makefile -> t")
}

// TestNCMV1013SelectorNestedModule pins the reason for a go selector inside
// a nested module, whose packages the repository index skips: the edit still
// selects everything, and says why (NCM-V1-013).
func TestNCMV1013SelectorNestedModule(t *testing.T) {
	before := "t:\n\tcd interop/x && go vet ./...\n"
	root, base := baseRepository(t, map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"interop/x/go.mod":      "module example.com/x\n\ngo 1.27\n",
		"interop/x/x.go":        "package x\n",
		"interop/x/x_test.go":   makeFixtureTest("x", ""),
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
		"Makefile":              before,
	}, nil)
	writeMakefile(t, root, strings.Replace(before, "t:\n\t", "t:\n\t: ; ", 1))
	want := `FALLBACK never-narrowed-path Makefile: make edit: a changed make target runs Go selector "interop/x/..." inside a nested module`
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, want)
	requireQualified(t, makeEditPlan(base, "Makefile"), root, want)
}
