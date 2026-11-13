// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const makeEditMakefile = `FLAGS = -v
all: check
check:
	go test $(FLAGS) ./tool
gen:
	go run ./gen
docs:
	echo docs
opaque:
	$$RUNNER ./tool
`

// makeEditRepository commits a repository whose Makefile the cases edit:
// mkread names the Makefile (rule (c)), ciread names the workflow that runs
// `make check`, and tool and gen are what check and gen run.
func makeEditRepository(t *testing.T) (string, string) {
	t.Helper()
	test := func(name, body string) string {
		return "package " + name + "\n\nimport \"testing\"\n\n" + body + "func TestX(t *testing.T) {}\n"
	}
	files := map[string]string{
		"go.mod":                   "module " + fixtureModule + "\n\ngo 1.27\n",
		"app/app.go":               "package app\n",
		"app/app_test.go":          test("app", ""),
		"tool/tool.go":             "package tool\n",
		"tool/tool_test.go":        test("tool", ""),
		"gen/gen.go":               "package gen\n",
		"gen/gen_test.go":          test("gen", ""),
		"mkread/mkread_test.go":    test("mkread", "var makefile = \"../Makefile\"\n\n"),
		"ciread/ciread_test.go":    test("ciread", "var workflow = \"../.github/workflows/ci.yml\"\n\n"),                //corvint:fixture-path
		".github/workflows/ci.yml": "jobs:\n  unit:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make check\n", //corvint:fixture-path
		"Makefile":                 makeEditMakefile,
	}
	return baseRepository(t, files, nil)
}

// makeEditPlan is a plan of root's change to the given paths against base.
func makeEditPlan(base string, dirty ...string) Receipt {
	plan := ncmPlan(dirty)
	plan.Range.Base = base
	return plan
}

func writeMakefile(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func modules(names ...string) string {
	var out []string
	for _, name := range names {
		out = append(out, fixtureModule+"/"+name)
	}
	return strings.Join(out, " ")
}

// TestNCMV1010SelectorNarrowsMakeEdit pins NCM-V1-010, NCM-V1-011,
// NCM-V1-013 and NCM-V1-014 in the selector: an attributable Makefile edit
// keeps the Makefile's reader, adds what each changed target (and what it
// reaches) runs, adds the readers of a workflow that runs a changed target,
// and names the targets. Each case is a known-affected unit for the fault
// injections recorded in docs/build-log/2026-11-13-mknarrow1113.md.
func TestNCMV1010SelectorNarrowsMakeEdit(t *testing.T) {
	root, base := makeEditRepository(t)
	for _, tc := range []struct {
		name, head, targets string
		want                []string
		audit               []string
	}{
		{"recipe", strings.Replace(makeEditMakefile, "go run ./gen", "go run ./gen -v", 1), "gen",
			[]string{"gen", "mkread"}, []string{"make " + fixtureModule + "/gen <- Makefile"}},
		{"recipe of a CI target", strings.Replace(makeEditMakefile, "./tool\ngen:", "-count=1 ./tool\ngen:", 1), "check",
			[]string{"ciread", "mkread", "tool"}, []string{"make-ci " + fixtureModule + "/ciread <- .github/workflows/ci.yml", "make " + fixtureModule + "/tool <- Makefile"}}, //corvint:fixture-path
		{"variable", strings.Replace(makeEditMakefile, "FLAGS = -v", "FLAGS = -race", 1), "check",
			[]string{"ciread", "mkread", "tool"}, nil},
		{"prerequisite reach", strings.Replace(makeEditMakefile, "all: check", "all: check gen", 1), "all",
			[]string{"gen", "mkread", "tool"}, nil},
		{"target running nothing", strings.Replace(makeEditMakefile, "echo docs", "echo more docs", 1), "docs",
			[]string{"mkread"}, nil},
		{"comment only", "# The fixture's make file.\n" + makeEditMakefile, "",
			[]string{"mkread"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeMakefile(t, root, tc.head)
			defer writeMakefile(t, root, makeEditMakefile)
			noncode := "noncode make-targets Makefile"
			if tc.targets != "" {
				noncode += " -> " + tc.targets
			}
			requireVerdict(t, makeEditPlan(base, "Makefile"), root, "run "+modules(tc.want...), append([]string{noncode}, tc.audit...)...)
			qualified := SelectQualified(makeEditPlan(base, "Makefile"), fixtureModule, root)
			if !slices.Contains(qualified, noncode) || strings.HasPrefix(qualified[len(qualified)-1], "FALLBACK ") {
				t.Fatalf("SelectQualified did not narrow with %q:\n%s", noncode, strings.Join(qualified, "\n"))
			}
		})
	}
}

// TestNCMV1012SelectorMakeEditFailsClosed pins the selector's full runs for
// a make file edit it cannot attribute or bound, each with its reason.
func TestNCMV1012SelectorMakeEditFailsClosed(t *testing.T) {
	root, base := makeEditRepository(t)
	full := func(p, why string) string { return "FALLBACK never-narrowed-path " + p + ": make edit: " + why }
	edited := strings.Replace(makeEditMakefile, "go run ./gen", "go run ./gen -v", 1)
	t.Run("unknown base", func(t *testing.T) {
		writeMakefile(t, root, edited)
		defer writeMakefile(t, root, makeEditMakefile)
		requireVerdict(t, ncmPlan([]string{"Makefile"}), root, full("Makefile", "the plan's base commit is unknown"))
		snapshot := makeEditPlan(base, "Makefile")
		snapshot.Snapshot = map[string]any{"kind": "snapshot"}
		requireVerdict(t, snapshot, root, full("Makefile", "the plan's base commit is unknown"))
		short := makeEditPlan(base[:12], "Makefile")
		requireVerdict(t, short, root, full("Makefile", "the plan's base commit is unknown"))
	})
	t.Run("deleted", func(t *testing.T) {
		if err := os.Remove(filepath.Join(root, "Makefile")); err != nil {
			t.Fatal(err)
		}
		defer writeMakefile(t, root, makeEditMakefile)
		requireVerdict(t, makeEditPlan(base, "Makefile"), root, full("Makefile", "the make file was deleted"))
		listed := makeEditPlan(base, "Makefile")
		listed.Plan.Deleted = []string{"Makefile"}
		requireVerdict(t, listed, root, full("Makefile", "the make file was deleted"))
	})
	t.Run("not a regular file", func(t *testing.T) {
		name := filepath.Join(root, "Makefile")
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("go.mod", name); err != nil {
			t.Fatal(err)
		}
		defer func() {
			os.Remove(name)
			writeMakefile(t, root, makeEditMakefile)
		}()
		requireVerdict(t, makeEditPlan(base, "Makefile"), root, full("Makefile", "the make file at the head is not a regular file"))
	})
	t.Run("new or renamed", func(t *testing.T) {
		name := filepath.Join(root, "tool", "GNUmakefile")
		if err := os.WriteFile(name, []byte("x:\n\techo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(name)
		requireVerdict(t, makeEditPlan(base, "tool/GNUmakefile"), root, full("tool/GNUmakefile", "the make file is new or renamed at the head"))
	})
	t.Run("unattributable edit", func(t *testing.T) {
		writeMakefile(t, root, makeEditMakefile+"include extra.mk\n")
		defer writeMakefile(t, root, makeEditMakefile)
		requireVerdict(t, makeEditPlan(base, "Makefile"), root, full("Makefile", `include line changed: "include extra.mk"`))
	})
	t.Run("unbounded reach", func(t *testing.T) {
		writeMakefile(t, root, strings.Replace(makeEditMakefile, "$$RUNNER ./tool", "$$RUNNER -v ./tool", 1))
		defer writeMakefile(t, root, makeEditMakefile)
		requireVerdict(t, makeEditPlan(base, "Makefile"), root, full("Makefile", `make target "opaque" runs a command or make goal that does not resolve`))
	})
	t.Run("included make file", func(t *testing.T) {
		requireVerdict(t, makeEditPlan(base, "rules.mk"), root, "FALLBACK never-narrowed-path rules.mk")
	})
}

const makeWordListMakefile = `STEPS = check
gate: clear $(addprefix ledger/,$(STEPS))
	echo gate
$(STEPS) $(addprefix ledger/,$(STEPS)): | clear
ledger/%:
	go run ./steptool $* -- $(MAKE) $*
clear:
	echo clear
check:
	go test ./tool
gen:
	go run ./gen
`

// TestNCMV1015SelectorNarrowsWordListEdit pins NCM-V1-015 and NCM-V1-016 in
// the selector: adding gen to a literal step list selects what gen runs,
// what ledger/gen runs through the ledger/% instance (steptool), the
// workflow that invokes the touched gate, and the Makefile's reader, but not
// check's tool, which both lists hold. A recipe use of the list, or a
// `$(shell)` in its value, keeps the full run.
func TestNCMV1015SelectorNarrowsWordListEdit(t *testing.T) {
	test := func(name, body string) string {
		return "package " + name + "\n\nimport \"testing\"\n\n" + body + "func TestX(t *testing.T) {}\n"
	}
	root, base := baseRepository(t, map[string]string{
		"go.mod":                    "module " + fixtureModule + "\n\ngo 1.27\n",
		"tool/tool.go":              "package tool\n",
		"tool/tool_test.go":         test("tool", ""),
		"gen/gen.go":                "package gen\n",
		"gen/gen_test.go":           test("gen", ""),
		"steptool/steptool.go":      "package steptool\n",
		"steptool/steptool_test.go": test("steptool", ""),
		"mkread/mkread_test.go":     test("mkread", "var makefile = \"../Makefile\"\n\n"),
		"ciread/ciread_test.go":     test("ciread", "var workflow = \"../.github/workflows/ci.yml\"\n\n"),               //corvint:fixture-path
		".github/workflows/ci.yml":  "jobs:\n  unit:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make gate\n", //corvint:fixture-path
		"Makefile":                  makeWordListMakefile,
	}, nil)
	added := strings.Replace(makeWordListMakefile, "STEPS = check", "STEPS = check gen", 1)
	writeMakefile(t, root, added)
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, "run "+modules("ciread", "gen", "mkread", "steptool"),
		"noncode make-targets Makefile -> clear gen ledger/gen",
		"make-ci "+fixtureModule+"/ciread <- .github/workflows/ci.yml") //corvint:fixture-path
	full := func(why string) string { return "FALLBACK never-narrowed-path Makefile: make edit: " + why }
	writeMakefile(t, root, added+"show:\n\techo $(STEPS)\n")
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, full(`variable "STEPS" is used outside every explicit rule: "$(STEPS) $(addprefix ledger/,$(STEPS)): | clear"`))
	writeMakefile(t, root, strings.Replace(makeWordListMakefile, "STEPS = check", "STEPS = check $(shell echo gen)", 1))
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, full(`variable "STEPS" calls $(shell)`))
}
