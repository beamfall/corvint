// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// sealedCEM is a sealed change-evidence path of the kind every dogfooded
// merge adds: unowned, read by no fixture unit unless a recipe names it.
const sealedCEM = ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"

// boundWorkflow runs one make target per job and is itself bounded, so the
// fixture's only computed sites are the make targets under test.
const boundWorkflow = "jobs:\n  unit:\n    steps:\n      - run: make check\n"

// boundFiles is a repository with two Go tools a recipe can run, one shell
// test, one shell script that reads its operand and one that enumerates every
// tracked file, and the shared bounded workflow.
func boundFiles(makefile string) map[string]string {
	return map[string]string{
		"Makefile":                 makefile,
		".github/workflows/ci.yml": boundWorkflow,
		"tools/ci/main.go":         "package main\n\nfunc main() {}\n",
		"tools/ci/main_test.go":    "package main\n\nimport \"testing\"\n\nfunc TestCI(t *testing.T) {}\n",
		"tools/local/main.go":      "package main\n\nfunc main() {}\n",
		"tools/local/main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestLocal(t *testing.T) {}\n",
		"script/audit.sh":          "#!/bin/sh\nls \"$1\"\n",
		"script/audit_test.sh":     "#!/bin/sh\nscript/audit.sh tools\n",
		"script/lsfiles.sh":        "#!/bin/sh\ngit ls-files | while read -r f; do test -f \"$f\"; done\n",
	}
}

// recipeEvaluated defines EVAL, an evaluated word function (AFP-V0-3122)
// that expands to nothing: a recipe that spells `$(EVAL)` is an evaluated
// target by its own text, which AZU-V0-610 requires for the certification
// hedge. A variable no recipe references marks no target (AZU-V0-610).
const recipeEvaluated = "EVAL = $(strip )\n"

// driverSelections maps each driver selection of plan by unit ID. Unlike
// driverSelected it tolerates unknown entries: a sealed CEM in a fixture is
// an unowned path, which the plan names apart from the selection.
func driverSelections(plan affected.Plan) map[string]affected.Selection {
	selected := map[string]affected.Selection{}
	for _, selection := range plan.Selected {
		if strings.HasPrefix(selection.UnitID, affected.DriverNamespace+":") {
			selected[selection.UnitID] = selection
		}
	}
	return selected
}

func computedSite(t *testing.T, graph *affected.Graph, id string) bool {
	t.Helper()
	unit, found := graph.Unit(id)
	if !found {
		t.Fatalf("%s is not a unit", id)
	}
	return slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation)
}

// TestFrontierSiteFaultInjection_AFP_V0_3120 injects one `$(or ...)`,
// `$(if ...)` or `$(join ...)` into a variable no recipe references. Before
// AFP-V0-3120 the file-level expander marked that ambiguity on every target
// of the file, so `check`, which runs a literal `go test`, became a
// driver:computed-invocation site, and driverSites claimed every dirty path
// for it, including a new sealed CEM; the control never selected it. Until
// AZU-V0-610 the injection also left `enumerate`, which runs an uncertified
// script, an evaluated site that claimed the CEM, while the control did not:
// a variable no text of the file expands changes nothing make runs, so the
// injected graph now carries the control's edges and frontier for both
// targets, and the omitted claim depends on nothing the variable can do.
func TestFrontierSiteFaultInjection_AFP_V0_3120(t *testing.T) {
	recipe := "check:\n\tgo test ./tools/local/...\nenumerate:\n\tscript/lsfiles.sh\n"
	control := driverGraph(t, boundFiles(recipe), nil)
	for _, tc := range []struct{ name, prefix string }{
		{"control", ""},
		{"injected-or", "CACHE = $(or $(XDG_CACHE_HOME),/tmp)/cache\n"},
		{"injected-if", "NICE = $(if $(filter-out 0,$(LEVEL)),nice -n $(LEVEL))\n"},
		{"injected-reassigned", "MODE = fast\nMODE = slow\n"},
		{"injected-join", "PAIR = $(join a b,$(SUFFIXES))\n"},
		{"info-never-expands", "$(info $(or $(XDG_CACHE_HOME),/tmp))\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.prefix+recipe), nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Errorf("check runs only `go test ./tools/local/...` but is a computed site")
			}
			for _, id := range []string{"driver:make:Makefile#check", "driver:make:Makefile#enumerate"} {
				want, _ := control.Unit(id)
				got, _ := graph.Unit(id)
				if fmt.Sprint(got.Imports, got.PathImports, got.BuildPathImports, got.Frontier, got.PathGlobs) != fmt.Sprint(want.Imports, want.PathImports, want.BuildPathImports, want.Frontier, want.PathGlobs) {
					t.Errorf("%s edges differ from the control: %+v vs %+v (AZU-V0-610)", id, got, want)
				}
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", "driver:make:Makefile#enumerate", jobUnit})
			selected = driverSelections(affected.Select(graph, []string{"tools/local/main.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", jobUnit}, nil)
		})
	}
}

// TestFrontierSiteFileStatementKeepsClaim_AFP_V0_3120 keeps every target of a
// file a computed site when a bare reference line can expand to text, which
// make reads again as make file lines the claimant does not model: the old
// file-wide ambiguity covered it, so AFP-V0-3120 must not drop it.
func TestFrontierSiteFileStatementKeepsClaim_AFP_V0_3120(t *testing.T) {
	for _, statement := range []string{
		"$(if $(EXTRA),include extra.mk)\n",
		"$(shell cat rules.txt)\n",
		"$(join a,$(SUFFIX))\n",
	} {
		t.Run(statement, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(statement+"check:\n\tgo test ./tools/local/...\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatalf("a statement that can add make text left check unclaimed")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", jobUnit}, nil)
		})
	}
}

// TestFrontierSiteBoundedAlternativesKeepTheirEdges_AFP_V0_3121 evaluates a
// computed invocation whose every alternative is a literal command the graph
// bounds: the site is resolved into the edges of its alternatives, so a
// change that any alternative can run or name still selects it, and its
// dependents.
func TestFrontierSiteBoundedAlternativesKeepTheirEdges_AFP_V0_3121(t *testing.T) {
	for _, tc := range []struct{ name, makefile, dirty string }{
		{"if-branch", "RUN = $(if $(CI),go test ./tools/ci,go vet ./tools/local)\ncheck:\n\t$(RUN) -count=1\n", "tools/local/main.go"},
		{"if-other-branch", "RUN = $(if $(CI),go test ./tools/ci,go vet ./tools/local)\ncheck:\n\t$(RUN) -count=1\n", "tools/ci/main.go"},
		{"or-first", "RUN = $(or $(TOOLS),go test ./tools/local)\nTOOLS = go test ./tools/ci\ncheck:\n\t$(RUN)\n", "tools/ci/main.go"},
		{"or-and-default", "RUN = $(or $(and $(CI),$(EMPTY)),go test ./tools/local)\nEMPTY =\ncheck:\n\t$(RUN)\n", "tools/local/main.go"},
		{"word-selection", "TOOLS = ci local\ncheck:\n\tgo test ./tools/$(lastword $(TOOLS)) ./tools/$(word 1,$(TOOLS))\n", "tools/ci/main.go"},
		{"reassigned", "TOOL = ./tools/ci\nTOOL = ./tools/local\ncheck:\n\tgo build $(TOOL)\n", "tools/local/main.go"},
		{"target-specific", "TOOL = ./tools/ci\ncheck: TOOL = ./tools/local\ncheck:\n\tgo build $(TOOL)\n", "tools/ci/main.go"},
		{"conditional-else", "ifeq ($(MODE),ci)\nTOOL = ./tools/ci\nelse\nTOOL = ./tools/local\nendif\ncheck:\n\tgo build $(TOOL)\n", "tools/local/main.go"},
		{"conditional-no-else", "TOOL = ./tools/ci\nifdef LOCAL\nTOOL = ./tools/local\nendif\ncheck:\n\tgo build $(TOOL)\n", "tools/ci/main.go"},
		{"append", "TOOLS = ./tools/ci\nTOOLS += ./tools/local\ncheck:\n\tgo test $(TOOLS)\n", "tools/local/main.go"},
		{"foreach", "TOOLS = ci local\ncheck:\n\t$(foreach tool,$(TOOLS),go vet ./tools/$(tool);)\n", "tools/local/main.go"},
		{"filter", "TOOLS = ci local docs\ncheck:\n\tgo test $(addprefix ./tools/,$(filter-out docs,$(TOOLS)))\n", "tools/local/main.go"},
		{"wildcard", "check:\n\tgo test $(wildcard ./tools/local)\n", "tools/local/main.go"},
		{"shell-test", "TEST = $(if $(CI),script/audit_test.sh,sh script/audit_test.sh)\ncheck:\n\t$(TEST)\n", "script/audit_test.sh"},
		{"names-cem-directory", "LIST = $(if $(VERBOSE),cat,head -n 1)\ncheck:\n\t$(LIST) .corvint/changes/*.cem.json\n", sealedCEM},
		{"names-cem-file", "PICK = $(if $(SUMMARY),grep -c profile,grep -l profile)\ncheck:\n\t$(PICK) " + sealedCEM + "\n", sealedCEM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := boundFiles(tc.makefile + "all: check\n")
			files[sealedCEM] = "{}\n"
			graph := driverGraph(t, files, nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Errorf("every alternative is a literal command the graph bounds, but check stays a computed site")
			}
			selected := driverSelections(affected.Select(graph, []string{tc.dirty}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if witness := selected["driver:make:Makefile#check"].Witness.Kind; witness == affected.WitnessFrontierSite {
				t.Errorf("check selected by %s; want the edge of the alternative that names %s", witness, tc.dirty)
			}
		})
	}
}

// TestFrontierSiteBoundedSetExcludesPath_AFP_V0_3122 is the same evaluated
// sites with a dirty path no alternative can run, name or read: the claim is
// dropped, for the site and for the target that depends on it.
func TestFrontierSiteBoundedSetExcludesPath_AFP_V0_3122(t *testing.T) {
	for _, makefile := range []string{
		"RUN = $(if $(CI),go test ./tools/ci,go vet ./tools/local)\ncheck:\n\t$(RUN) -count=1\n",
		"TOOL = ./tools/ci\nTOOL = ./tools/local\ncheck:\n\tgo build $(TOOL)\n",
		"ifeq ($(MODE),ci)\nTOOL = ./tools/ci\nelse\nTOOL = ./tools/local\nendif\ncheck:\n\tgo build $(TOOL)\n",
		"NICE = $(if $(filter-out 0,$(LEVEL)),$(if $(shell command -v nice),nice -n $(LEVEL)))\nLEVEL ?= 10\ncheck:\n\t$(NICE) env GOCACHE=$(or $(GOCACHE),/tmp/cache) go test ./tools/local\n",
		"CACHE = $(or $(XDG_CACHE_HOME),/tmp)/cache\ncheck:\n\tGOCACHE=$(CACHE) go test ./tools/local && test -d tools/local\n",
		"TEST = $(if $(CI),script/audit_test.sh,bash script/audit_test.sh)\ncheck:\n\t$(TEST) && go vet ./tools/local\n",
	} {
		t.Run(strings.SplitN(makefile, "\n", 2)[0], func(t *testing.T) {
			graph := driverGraph(t, boundFiles(makefile+"all: check\n"), nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Errorf("every alternative is a literal command the graph bounds, but check stays a computed site")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
			selected = driverSelections(affected.Select(graph, []string{"tools/local/main.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
		})
	}
}

// TestFrontierSiteUnboundedKeepsClaim_AFP_V0_3123 keeps every site whose
// alternatives cannot be bounded: a command word read from the environment,
// a make variable the environment can supply, a `$(shell ...)` command word,
// an unevaluated function over an unknown value, alternatives past the
// variant bound, and an evaluated site that runs a program whose reads the
// graph does not bound (a repository script, `go run`, `git ls-files`, an
// interpreter). Each still claims every dirty path with a FRONTIER_SITE
// witness, and the target depending on it follows.
func TestFrontierSiteUnboundedKeepsClaim_AFP_V0_3123(t *testing.T) {
	many := strings.Repeat("$(if $(A),a,b)", 9)
	for _, tc := range []struct{ name, makefile string }{
		{"shell-variable", "check:\n\t$$TOOL --verify\n"},
		{"environment-default", "RUN = $(or $(RUNNER),go test ./tools/local)\ncheck:\n\t$(RUN)\n"},
		{"undefined", "check:\n\t$(RUNNER) ./tools/local\n"},
		{"shell-function", "check:\n\t$(shell cat tool.txt) --verify\n"},
		{"unknown-word-mix", "check:\n\tgo test ./tools/$(firstword $(TOOLS) local)\n"},
		{"past-variant-bound", "check:\n\tgo test ./tools/" + many + "\n"},
		{"eval", "check:\n\teval \"$$(cat tool.txt)\"\n"},
		{"evaluated-repository-script", "AUDIT = $(if $(CI),script/audit.sh,sh script/audit.sh)\ncheck:\n\t$(AUDIT) tools\n"},
		{"evaluated-go-run", "RUN = $(if $(CI),go run ./tools/ci,go run ./tools/local)\ncheck:\n\t$(RUN)\n"},
		{"evaluated-enumerator", "LIST = $(if $(CI),script/lsfiles.sh,sh script/lsfiles.sh)\ncheck:\n\t$(LIST)\n"},
		{"evaluated-git", "CACHE = $(or $(XDG_CACHE_HOME),/tmp)\ncheck:\n\tgit ls-files -z > $(CACHE)/files\n"},
		{"evaluated-interpreter", "PY = $(if $(CI),python3,python)\ncheck:\n\t$(PY) tools/check.py\n"},
		{"evaluated-recursive-grep", "CACHE = $(or $(XDG_CACHE_HOME),/tmp)\ncheck:\n\tgrep -rl profile . > $(CACHE)/hits\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.makefile+"all: check\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatalf("check's alternatives or reads cannot be bounded, but it is not a computed site")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if witness := selected["driver:make:Makefile#check"].Witness; witness.Kind != affected.WitnessFrontierSite || witness.DirtyPath != sealedCEM {
				t.Errorf("check witness %+v; want FRONTIER_SITE on %s", witness, sealedCEM)
			}
			if got := slices.Sorted(maps.Keys(selected)); len(got) != 3 {
				t.Errorf("selected %v; want check, all and the job", got)
			}
		})
	}
}

// TestFrontierSiteShellTestParity_AFP_V0_3122 holds the driver's shell-test
// certification to the shell plugin's own rule in the direction soundness
// needs: an evaluated target running a script the driver certifies as a test
// runs one the shell plugin indexes as a test, which it selects for itself.
// A shell test the driver does not certify only keeps a claim.
func TestFrontierSiteShellTestParity_AFP_V0_3122(t *testing.T) {
	for _, name := range []string{
		"script/a_test.sh", "script/a-test.sh", "script/test_a.sh", "script/test-a.sh", "script/a.bats",
		"script/a_test.bash", "script/a.sh", "script/atest.sh", "script/testa.sh", "script/a_test.py",
		"tests/a.sh", "script/a_test",
	} {
		t.Run(name, func(t *testing.T) {
			files := boundFiles(recipeEvaluated + "check:\n\t$(EVAL)" + name + "\n")
			files[name] = "#!/bin/sh\necho ok\n"
			files["go.mod"] = "module example.test/m\n\ngo 1.27.0\n"
			root := t.TempDir()
			for file, body := range files {
				full := filepath.Join(root, filepath.FromSlash(file))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			graph, err := affected.Build(t.Context(), root, golang.New(), shell.New(), affected.Driver{})
			if err != nil {
				t.Fatal(err)
			}
			if computedSite(t, graph, "driver:make:Makefile#check") {
				if strings.HasSuffix(name, "_test.sh") || strings.HasSuffix(name, ".bats") {
					t.Errorf("the driver does not certify the shell test %s", name)
				}
				return
			}
			unit, found := graph.Unit("shell:" + name)
			if !found || len(unit.Tests) == 0 {
				t.Errorf("the driver certifies %s as a shell test, but the shell plugin indexes no test for it", name)
			}
		})
	}
}

// TestFrontierSiteImmediateValueKeepsCapturedEdge_AFP_V0_3125 holds an
// immediate assignment to the value make captures where it reads it: with
// `PKG = ci`, `RUN := go test ./tools/$(PKG)` and a later `PKG = local`, make
// runs the ci package, so a ci change selects check by its edge, and a local
// change, which no run of check reads, does not. A prerequisite, which make
// also expands where it reads it, keeps the same captured edge.
func TestFrontierSiteImmediateValueKeepsCapturedEdge_AFP_V0_3125(t *testing.T) {
	for _, tc := range []struct{ name, makefile, dirty, unread string }{
		{"immediate", "PKG = ci\nRUN := go test ./tools/$(PKG)\nPKG = local\ncheck:\n\t$(RUN)\n", "tools/ci/main.go", "tools/local/main.go"},
		{"double-colon", "PKG = ci\nRUN ::= go test ./tools/$(PKG)\nPKG = local\ncheck:\n\t$(RUN)\n", "tools/ci/main.go", "tools/local/main.go"},
		{"append-to-simple", "PKG = local\nRUN := go test\nRUN += ./tools/$(PKG)\nPKG = ci\ncheck:\n\t$(RUN)\n", "tools/local/main.go", "tools/ci/main.go"},
		{"recursive-control", "PKG = ci\nRUN = go test ./tools/$(PKG)\nPKG = local\ncheck:\n\t$(RUN)\n", "tools/local/main.go", "tools/ci/main.go"},
		{"define-immediate", "PKG = ci\ndefine RUN :=\ngo test ./tools/$(PKG)\nendef\nPKG = local\ncheck:\n\t$(RUN)\n", "tools/ci/main.go", "tools/local/main.go"},
		{"prerequisite", "PKG = ci\ncheck: tools/$(PKG)/main.go\n\tgo vet ./tools/...\nPKG = local\n", "tools/ci/main.go", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.makefile+"all: check\n"), nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatalf("every value make captures is a literal go command, but check stays a computed site")
			}
			selected := driverSelections(affected.Select(graph, []string{tc.dirty}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if tc.unread != "" {
				selected = driverSelections(affected.Select(graph, []string{tc.unread}))
				wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check"})
			}
			selected = driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check"})
		})
	}
}

// TestFrontierSiteTargetSpecificSurvivesGlobal_AFP_V0_3126 keeps a
// target-specific value after a later global assignment: with `check: PKG =
// ci` and then `PKG = local`, make builds check with ci, so a ci change
// selects it; the global value stays an alternative for other targets.
func TestFrontierSiteTargetSpecificSurvivesGlobal_AFP_V0_3126(t *testing.T) {
	for _, tc := range []struct{ name, makefile, dirty string }{
		{"scoped-then-global", "check: PKG = ci\nPKG = local\ncheck:\n\tgo test ./tools/$(PKG)\n", "tools/ci/main.go"},
		{"scoped-then-global-other", "check: PKG = ci\nPKG = local\ncheck:\n\tgo test ./tools/$(PKG)\n", "tools/local/main.go"},
		{"scoped-append-after-global", "check: PKG += ./tools/ci\nPKG = ./tools/local\ncheck:\n\tgo test $(PKG)\n", "tools/ci/main.go"},
		{"scoped-chain", "check: PKG = ./tools/ci\ncheck: PKG += ./tools/local\nPKG = ./tools/x\ncheck:\n\tgo test $(PKG)\n", "tools/local/main.go"},
		{"scoped-immediate", "TOOL = ci\ncheck: PKG := ./tools/$(TOOL)\nTOOL = local\nPKG = ./tools/x\ncheck:\n\tgo test $(PKG)\n", "tools/ci/main.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.makefile+"all: check\n"), nil)
			selected := driverSelections(affected.Select(graph, []string{tc.dirty}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
		})
	}
}

// TestFrontierSiteParseTimeCommandKeepsClaim_AFP_V0_3127 keeps every target
// a computed site when make runs a command it cannot certify while reading
// the file, before it builds any target: an immediate `$(shell ...)` whose
// value no recipe uses, a `!=` assignment, a conditional line, an include and
// a prerequisite, and a SHELL or .SHELLFLAGS that runs a program other than
// a plain shell. A `$(shell ...)` in a recursive value no recipe expands
// never runs, so it alone keeps no claim, and neither does a bash SHELL.
func TestFrontierSiteParseTimeCommandKeepsClaim_AFP_V0_3127(t *testing.T) {
	const evaluated = "CACHE = $(or $(XDG_CACHE_HOME),/tmp)/cache\n"
	const recipe = "check:\n\tGOCACHE=$(CACHE) go test ./tools/local\n"
	for _, tc := range []struct {
		name, makefile string
		claimed        bool
	}{
		{"unused-immediate-shell", "AUDIT := $(shell $(if $(CI),script/lsfiles.sh,script/lsfiles.sh))\n" + recipe, true},
		{"unused-immediate-literal-shell", evaluated + "AUDIT := $(shell script/lsfiles.sh)\n" + recipe, true},
		{"bang-assignment", evaluated + "FILES != script/lsfiles.sh\n" + recipe, true},
		{"conditional-line", evaluated + "ifeq ($(shell script/lsfiles.sh),)\nX = 1\nendif\n" + recipe, true},
		{"include-line", evaluated + "-include $(shell script/lsfiles.sh)\n" + recipe, true},
		{"prerequisite-of-other-target", evaluated + "other: $(shell script/lsfiles.sh)\n" + recipe, true},
		{"immediate-through-recursive", evaluated + "LIST = $(shell script/lsfiles.sh)\nAUDIT := $(LIST)\n" + recipe, true},
		{"unused-recursive-shell", evaluated + "LIST = $(shell script/lsfiles.sh)\n" + recipe, false},
		{"recipe-shell-program", evaluated + "SHELL = script/lsfiles.sh\n" + recipe, true},
		{"recipe-shell-flags-operand", evaluated + ".SHELLFLAGS = script/lsfiles.sh -c\n" + recipe, true},
		{"recipe-shell-bash", evaluated + "SHELL := /bin/bash\n.SHELLFLAGS := -eu -o pipefail -c\n" + recipe, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.makefile+"all: check\n"), nil)
			if got := computedSite(t, graph, "driver:make:Makefile#check"); got != tc.claimed {
				t.Fatalf("check computed site = %v, want %v", got, tc.claimed)
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			if tc.claimed {
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
				if witness := selected["driver:make:Makefile#check"].Witness; witness.Kind != affected.WitnessFrontierSite {
					t.Errorf("check witness %+v; want FRONTIER_SITE", witness)
				}
			} else {
				wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
			}
			selected = driverSelections(affected.Select(graph, []string{"tools/local/main.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
		})
	}
}

// TestFrontierSiteUnboundedOperandKeepsClaim_AFP_V0_3128 keeps an evaluated
// target a computed site when a command it runs reads more than its literal
// operands: an operand, redirection or test path the environment supplies,
// operands xargs reads from input, a relative operand after a change to an
// unknown directory, a checksum manifest's listed files, a list of file
// names, a jq module, an awk or sed program, a go operand that can split
// into flags, and a make call that overrides a variable.
func TestFrontierSiteUnboundedOperandKeepsClaim_AFP_V0_3128(t *testing.T) {
	// The recipe's own `$(EVAL)` makes check an evaluated target
	// (AZU-V0-610); CACHE stays an environment-supplied directory.
	const evaluated = "CACHE = $(or $(XDG_CACHE_HOME),/tmp)\n" + recipeEvaluated
	for _, tc := range []struct{ name, makefile string }{
		{"environment-operand", "INPUT = $(or $(FILE),known.txt)\ncheck:\n\tcat $(INPUT)\n"},
		{"environment-redirection", "INPUT = $(or $(FILE),known.txt)\ncheck:\n\twc -l < $(INPUT)\n"},
		{"environment-test-path", "INPUT = $(or $(FILE),known.txt)\ncheck:\n\ttest -f $(INPUT)\n"},
		{"shell-variable-operand", evaluated + "check:\n\t$(EVAL)for f in a b; do cat \"$$f\"; done\n"},
		{"xargs", evaluated + "check:\n\t$(EVAL)xargs cat < known.txt\n"},
		{"unknown-directory", evaluated + "check:\n\tcd $(CACHE) && cat known.txt\n"},
		{"checksum-manifest", evaluated + "check:\n\t$(EVAL)sha256sum -c checksums.txt\n"},
		{"checksum-manifest-long", evaluated + "check:\n\t$(EVAL)shasum --check --quiet checksums.txt\n"},
		{"checksum-manifest-cluster", evaluated + "check:\n\t$(EVAL)md5sum -sc checksums.txt\n"},
		{"files0-from", evaluated + "check:\n\t$(EVAL)wc -l --files0-from=list.txt\n"},
		{"jq-module", evaluated + "check:\n\t$(EVAL)jq -n 'import \"lib\" as lib; lib::x'\n"},
		{"jq-library-path", evaluated + "check:\n\t$(EVAL)jq -L lib -n '1'\n"},
		{"awk", evaluated + "check:\n\t$(EVAL)awk '{ while ((getline line < $$1) > 0) print line }' known.txt\n"},
		{"sed", evaluated + "check:\n\t$(EVAL)sed -n '1r other.txt' known.txt\n"},
		{"go-environment-operand", "PKG = $(or $(PKGS),./tools/local)\ncheck:\n\tgo test $(PKG)\n"},
		{"go-environment-flags", evaluated + "check:\n\t$(EVAL)go test $(GOFLAGS_EXTRA) ./tools/local\n"},
		{"make-override", evaluated + "check:\n\t$(EVAL)$(MAKE) other PKG=ci\nother:\n\tgo test ./tools/local\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.makefile+"all: check\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatalf("check reads more than its literal operands, but it is not a computed site")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if witness := selected["driver:make:Makefile#check"].Witness; witness.Kind != affected.WitnessFrontierSite || witness.DirtyPath != sealedCEM {
				t.Errorf("check witness %+v; want FRONTIER_SITE on %s", witness, sealedCEM)
			}
		})
	}
}

// TestFrontierSiteBoundedOperandDropsClaim_AFP_V0_3128 is the control: the
// same commands in modes that read only their literal operands, with a
// bounded Go output value, leave the evaluated target no site.
func TestFrontierSiteBoundedOperandDropsClaim_AFP_V0_3128(t *testing.T) {
	const evaluated = "CACHE = $(or $(XDG_CACHE_HOME),/tmp)\n" + recipeEvaluated
	for _, tc := range []struct{ name, makefile string }{
		{"checksum-compute", evaluated + "check:\n\tsha256sum known.txt > $(CACHE)/sums\n"},
		{"literal-redirection", evaluated + "check:\n\t$(EVAL)wc -l < known.txt\n"},
		{"literal-test", evaluated + "check:\n\ttest -f known.txt && test -n \"$(CACHE)\"\n"},
		{"jq-literal", evaluated + "check:\n\t$(EVAL)jq -r .name known.json\n"},
		{"go-output", evaluated + "CACHE = /tmp\ncheck:\n\tgo build -o $(CACHE)/bin ./tools/local\n"},
		{"make-literal", evaluated + "check:\n\t$(EVAL)$(MAKE) other\nother:\n\tgo test ./tools/local\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := boundFiles(tc.makefile + "all: check\n")
			files["known.txt"] = "x\n"
			files["known.json"] = "{}\n"
			graph := driverGraph(t, files, nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatalf("check reads only literal operands, but it is a computed site")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check"})
		})
	}
}

// TestFrontierSiteRootMakefileRunsNoReadTimeCommand_AFP_V0_3127 builds the
// driver graph of this repository's root Makefile. Its WINDOWS_VET_GOCACHE
// is recursive, so the `$(shell stat ...)` probe runs only where windows-vet
// expands it, and windows-vet alone keeps that claim. The same file with
// that one assignment made immediate, as it was at 538e823f4, runs the probe
// while make reads the file, on a directory the environment supplies, so
// every target of the file is a computed site and claims every dirty path,
// including each merge's sealed CEM.
func TestFrontierSiteRootMakefileRunsNoReadTimeCommand_AFP_V0_3127(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repositoryRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	const recursive, immediate = "\nWINDOWS_VET_GOCACHE = $(if ", "\nWINDOWS_VET_GOCACHE := $(if "
	if strings.Count(string(body), recursive) != 1 {
		t.Fatalf("the root Makefile no longer assigns WINDOWS_VET_GOCACHE recursively once; an immediate `$(shell ...)` makes every root target a computed site (AFP-V0-3127)")
	}
	// AFP-V0-9051: script operands must exist as plain files before a
	// command can certify. Their contents are irrelevant to this driver-only
	// comparison; the fixture does not execute them.
	files := map[string]string{}
	for _, word := range strings.Fields(string(body)) {
		word = strings.TrimLeft(word, "@+-") // Make recipe prefixes are not part of the path.
		if strings.HasSuffix(word, ".sh") && affected.ValidRelativePath(word) {
			files[word] = "#!/bin/sh\n"
		}
	}
	computed := func(makefile string) (sites []string, targets int) {
		files["Makefile"] = makefile
		graph := driverGraph(t, files, nil)
		for _, id := range graph.UnitIDs() {
			if !strings.HasPrefix(id, "driver:make:Makefile#") {
				continue
			}
			targets++
			if computedSite(t, graph, id) {
				sites = append(sites, id)
			}
		}
		return sites, targets
	}
	asIs, targets := computed(string(body))
	reverted, revertedTargets := computed(strings.Replace(string(body), recursive, immediate, 1))
	t.Logf("root Makefile computed sites: %d of %d recursive, %d of %d immediate", len(asIs), targets, len(reverted), revertedTargets)
	if len(reverted) != revertedTargets {
		t.Errorf("with an immediate WINDOWS_VET_GOCACHE %d of %d root targets are computed sites; want every one", len(reverted), revertedTargets)
	}
	if !slices.Contains(asIs, "driver:make:Makefile#windows-vet") {
		t.Errorf("windows-vet expands the stat probe on an environment directory, but it is not a computed site")
	}
	if dropped := revertedTargets - len(asIs); dropped < targets/4 {
		t.Errorf("a recursive WINDOWS_VET_GOCACHE drops only %d of %d root computed sites; want at least a quarter", dropped, targets)
	}
}

// TestFrontierSiteUnsupportedAssignmentKeepsClaim_AFP_V0_3125 refuses
// immediate-recursive and unknown flavors rather than treating them as simple.
// In the first case GNU make appends recursively and runs tools/local.
func TestFrontierSiteUnsupportedAssignmentKeepsClaim_AFP_V0_3125(t *testing.T) {
	for _, assignment := range []string{
		"RUN :::= go test\nRUN += ./tools/$(PKG)",
		"define RUN :::=\ngo test\nendef\nRUN += ./tools/$(PKG)",
		"check: RUN :::= go test\ncheck: RUN += ./tools/$(PKG)",
		"RUN ::::= go test ./tools/$(PKG)",
		"UNUSED :::= ignored\nRUN = go test ./tools/local",
		"$(NAME) :::= ignored\nRUN = go test ./tools/local",
	} {
		t.Run(assignment, func(t *testing.T) {
			makefile := "PKG = ci\n" + assignment + "\nPKG = local\ncheck:\n\t$(RUN)\nall: check\n"
			graph := driverGraph(t, boundFiles(makefile), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unsupported assignment flavor certified check")
			}
			for _, dirty := range []string{"tools/local/main.go", sealedCEM} {
				selected := driverSelections(affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			}
		})
	}
}

// TestFrontierSiteGoFlagAllowlist_AFP_V0_3128 checks both the reported
// executable-flag injection and other unrecognized or unbounded flag forms.
func TestFrontierSiteGoFlagAllowlist_AFP_V0_3128(t *testing.T) {
	for _, recipe := range []string{
		"go build -o $(OUTPUT) ./tools/local",
		"go build -o=$(OUTPUT) ./tools/local",
		"go build -o \"$(OUTPUT)\" ./tools/local",
		"go build -o $(CACHE)/bin ./tools/local",
		"go test -run $(OUTPUT) ./tools/local",
		"go build -unrecognized=literal ./tools/local",
		"go build -unrecognized literal ./tools/local",
		"go build -toolexec=script/audit.sh ./tools/local",
		"go test -race=$(OUTPUT) ./tools/local",
		"go build -o",
		"go build -o * ./tools/local",
	} {
		t.Run(recipe, func(t *testing.T) {
			graph := driverGraph(t, boundFiles("OUTPUT = $(or $(OUT),/tmp/bin)\nCACHE = $(or $(XDG_CACHE_HOME),/tmp)\n"+recipeEvaluated+"check:\n\t$(EVAL)"+recipe+"\nall: check\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unknown Go flag or value certified check")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
		})
	}
}

// TestFrontierSiteCommandOptionAllowlist_AFP_V0_3128 refuses every
// checksum-check abbreviation and unknown option; literal options retain
// certification only when their exact spelling and arity are recognized.
func TestFrontierSiteCommandOptionAllowlist_AFP_V0_3128(t *testing.T) {
	for _, command := range []string{
		"sha256sum --chec checksums.txt",
		"sha256sum --c checksums.txt",
		"sha256sum --check=checksums.txt",
		"sha1sum --che checksums.txt",
		"md5sum --che checksums.txt",
		"shasum --chec checksums.txt",
		"cksum --chec checksums.txt",
		"sha256sum --unrecognized checksums.txt",
		"cat --unrecognized known.txt",
		"cat -Z known.txt",
		"head -n",
		"head -n * known.txt",
		"sha256sum *",
		"wc --files0-f=list.txt",
		"grep --rec known.txt tools",
	} {
		t.Run(command, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(recipeEvaluated+"check:\n\t$(EVAL)"+command+"\nall: check\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unknown command option certified check")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if witness := selected["driver:make:Makefile#check"].Witness.Kind; witness != affected.WitnessFrontierSite {
				t.Fatalf("got %s; want run-all FRONTIER_SITE", witness)
			}
		})
	}
}

// AFP-V0-3121: a filesystem function's argument is not its result. In
// particular, an absent wildcard selects the false branch in GNU make.
func TestFrontierSiteFilesystemCondition_AFP_V0_3121(t *testing.T) {
	for _, function := range []string{"wildcard", "realpath"} {
		for _, immutable := range []bool{false, true} {
			name := function + "/disk"
			if immutable {
				name = function + "/immutable"
			}
			t.Run(name, func(t *testing.T) {
				files := boundFiles("check:; go test ./$(if $(" + function + " missing),left,right)_pkg\nall: check\n")
				files["left_pkg/main.go"] = "package left\n"
				files["right_pkg/main.go"] = "package right\n"
				var graph *affected.Graph
				if immutable {
					files["go.mod"] = "module example.test/m\n\ngo 1.27.0\n"
					fsys := fstest.MapFS{}
					for name, body := range files {
						fsys[name] = &fstest.MapFile{Data: []byte(body)}
					}
					var err error
					graph, err = affected.BuildFS(fsys, golang.New(), affected.Driver{})
					if err != nil {
						t.Fatal(err)
					}
				} else {
					graph = driverGraph(t, files, nil)
				}
				selected := driverSelections(affected.Select(graph, []string{"right_pkg/main.go"}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			})
		}
	}
}

// AFP-V0-3128: dropping a claim requires an edge for every literal file
// operand, even when its spelling has neither a slash nor an extension.
func TestFrontierSiteExtensionlessOperand_AFP_V0_3128(t *testing.T) {
	for _, recipe := range []string{
		"$(if yes,cat,cat) LICENSE",
		"$(if yes,cat,cat) 'LICENSE COPY'",
		"$(if yes,test,test) -f LICENSE",
		"$(if yes,wc,wc) -l < LICENSE",
		"cd data && $(if yes,cat,cat) LICENSE",
	} {
		t.Run(recipe, func(t *testing.T) {
			files := boundFiles("check:; " + recipe + "\nall: check\n")
			operand := "LICENSE"
			if strings.Contains(recipe, "COPY") {
				operand = "LICENSE COPY"
			}
			if strings.HasPrefix(recipe, "cd ") {
				operand = "data/" + operand
			}
			files[operand] = "license text\n"
			graph := driverGraph(t, files, nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("literal file operand should be represented by an edge")
			}
			selected := driverSelections(affected.Select(graph, []string{operand}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			selected = driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
		})
	}
}

// AFP-V0-3128: a deferred trap can read a path supplied at runtime, after
// recipe scanning. It must retain the frontier claim.
func TestFrontierSiteDynamicTrap_AFP_V0_3128(t *testing.T) {
	graph := driverGraph(t, boundFiles("check:; $(if yes,trap,trap) 'cat \"$$INPUT\"' EXIT\nall: check\n"), nil)
	if !computedSite(t, graph, "driver:make:Makefile#check") {
		t.Fatal("dynamic trap body lost its frontier claim")
	}
	selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
	wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
	if witness := selected["driver:make:Makefile#check"].Witness; witness.Kind != affected.WitnessFrontierSite {
		t.Errorf("check witness %+v; want FRONTIER_SITE", witness)
	}
}

// AFP-V0-3127: a repository executable named bash is not a certified plain
// shell. Its policy reads are outside the recipe's explicit operands.
func TestFrontierSiteRepositoryShellWrapper_AFP_V0_3127(t *testing.T) {
	files := boundFiles("SHELL = tools/bash\ncheck:; $(if yes,echo,echo) ok\nall: check\n")
	files["tools/bash"] = "#!/bin/sh\ncat policy.json\nexec /bin/bash \"$@\"\n"
	files["policy.json"] = "{}\n"
	graph := driverGraph(t, files, nil)
	if !computedSite(t, graph, "driver:make:Makefile#check") {
		t.Fatal("repository shell wrapper lost its frontier claim")
	}
	selected := driverSelections(affected.Select(graph, []string{"policy.json"}))
	wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
	if witness := selected["driver:make:Makefile#check"].Witness; witness.Kind != affected.WitnessFrontierSite {
		t.Errorf("check witness %+v; want FRONTIER_SITE", witness)
	}
}

// AFP-V0-3121: strip normalizes expanded whitespace before if tests it.
func TestFrontierSiteStripCondition_AFP_V0_3121(t *testing.T) {
	for _, condition := range []string{
		"$(strip $(subst x, ,x))",
		"$(strip $(subst x, \t,x))",
	} {
		t.Run(condition, func(t *testing.T) {
			files := boundFiles("check:; go test ./$(if " + condition + ",ci,local)\nall: check\n")
			files["ci/ci.go"] = "package ci\n"
			files["local/local.go"] = "package local\n"
			graph := driverGraph(t, files, nil)
			selected := driverSelections(affected.Select(graph, []string{"local/local.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
		})
	}
}

// AFP-V0-3128: attached file values cannot be certified while the operand
// scanner cannot represent them, even when their names have no extension.
func TestFrontierSiteAttachedFileOptionKeepsClaim_AFP_V0_3128(t *testing.T) {
	for _, recipe := range []string{
		"$(if $(CI),grep,grep) --file=LICENSE known.txt",
		"$(if $(CI),grep,grep) --file='LICENSE COPY' known.txt",
		"cd data && $(if $(CI),grep,grep) --file=LICENSE known.txt",
	} {
		t.Run(recipe, func(t *testing.T) {
			files := boundFiles("check:; " + recipe + "\nall: check\n")
			operand := "LICENSE"
			if strings.Contains(recipe, "COPY") {
				operand = "LICENSE COPY"
			}
			if strings.HasPrefix(recipe, "cd ") {
				operand = "data/" + operand
			}
			files[operand] = "pattern\n"
			files["known.txt"] = "pattern\n"
			graph := driverGraph(t, files, nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unrecorded attached file value certified check")
			}
			for _, dirty := range []string{operand, sealedCEM} {
				selected := driverSelections(affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			}
		})
	}
}

// AFP-V0-3122/3128: make options that evaluate text or load code can read
// paths absent from the literal goals' modeled dependencies.
func TestFrontierSiteMakeOptionAllowlist_AFP_V0_3128(t *testing.T) {
	for _, option := range []string{
		"--eval='X := $$(shell script/audit.sh)'",
		"--eval 'X := $$(shell script/audit.sh)'",
		"-E 'X := $$(shell script/audit.sh)'",
		"--load=script/plugin.so",
		"--load script/plugin.so",
		"-l script/plugin.so",
		"--file=other.mk",
		"-f other.mk",
		"--include-dir=other",
		"--unrecognized",
		"--directory=",
		"-- -unmodeled",
	} {
		t.Run(option, func(t *testing.T) {
			graph := driverGraph(t, boundFiles("check:; $(if $(CI),make,make) "+option+" other\nother:; echo ok\nall: check\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unsupported make option certified check")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if witness := selected["driver:make:Makefile#check"].Witness.Kind; witness != affected.WitnessFrontierSite {
				t.Fatalf("got %s; want FRONTIER_SITE", witness)
			}
		})
	}
	for _, options := range []string{"", "-s", "--no-print-directory", "-j2", "--jobs=2", "-C . -j 2"} {
		t.Run("bounded/"+options, func(t *testing.T) {
			graph := driverGraph(t, boundFiles("check:; $(if $(CI),make,make) "+options+" other\nother:; echo ok\nall: check\n"), nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("modeled make options should preserve certification")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
		})
	}
}

// AFP-V0-3128: explicit GOFLAGS is part of the command's read boundary,
// including when env or another shell wraps the Go invocation.
func TestFrontierSiteGoEnvironmentKeepsClaim_AFP_V0_3128(t *testing.T) {
	for _, recipe := range []string{
		"env GOFLAGS=-toolexec=script/audit.sh $(if $(CI),go,go) test ./tools/local",
		"GOFLAGS=-toolexec=script/audit.sh $(if $(CI),go,go) test ./tools/local",
		"env GOFLAGS='-toolexec=script/audit.sh' $(if $(CI),go,go) test ./tools/local",
		"env GOFLAGS=$(FLAGS) $(if $(CI),go,go) test ./tools/local",
		"env GOFLAGS=\"$$FLAGS\" $(if $(CI),go,go) test ./tools/local",
		"GOFLAGS=-toolexec=script/audit.sh $(if $(CI),sh,sh) -c 'go test ./tools/local'",
	} {
		t.Run(recipe, func(t *testing.T) {
			graph := driverGraph(t, boundFiles("check:; "+recipe+"\nall: check\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unmodeled GOFLAGS certified check")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if witness := selected["driver:make:Makefile#check"].Witness.Kind; witness != affected.WitnessFrontierSite {
				t.Fatalf("got %s; want FRONTIER_SITE", witness)
			}
		})
	}
}

// AFP-V0-3128: certification admits literal operands only, at every position.
func TestFrontierSiteGlobOperandsKeepClaim_AFP_V0_3128(t *testing.T) {
	for _, operand := range []string{"LICENSE*", "LICENSE?new", "LICENSE[.]new", "LICENSE[[:future:]]", "< LICENSE*", "-- LICENSE*", "-- LICENSE?new", "--unrecognized LICENSE", "data/LICENSE*", "data/**", "data/*[.]new", "-- -dir/*"} {
		t.Run(operand, func(t *testing.T) {
			files := boundFiles("check:; $(if $(CI),grep,grep) -q approved " + operand + "\nall: check\n")
			files["LICENSE.new"] = "approved\n"
			graph := driverGraph(t, files, nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unmodeled operand certified check")
			}
			for _, dirty := range []string{"LICENSE.new", sealedCEM} {
				selected := driverSelections(affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			}
		})
	}
}

// AFP-V0-3128: export effects outlive a simple command and make assignments.
func TestFrontierSiteExportEffectsKeepClaim_AFP_V0_3128(t *testing.T) {
	for _, tc := range []struct{ name, prefix, recipe string }{
		{"shell-export", "", "export GOFLAGS=-toolexec=script/audit.sh; "},
		{"shell-assignment-then-export", "", "GOFLAGS=-toolexec=script/audit.sh; export GOFLAGS; "},
		{"shell-unknown-export", "", "export FUTURE_TOOL_SETTING=script/audit.sh; "},
		{"shell-export-option", "", "export --unrecognized GOFLAGS; "},
		{"shell-declare", "", "declare -x GOFLAGS=-toolexec=script/audit.sh; "},
		{"make-export", "export GOFLAGS=-toolexec=script/audit.sh\n", ""},
		{"make-export-name", "GOFLAGS=-toolexec=script/audit.sh\nexport GOFLAGS\n", ""},
		{"make-export-all", "GOFLAGS=-toolexec=script/audit.sh\nexport\n", ""},
		{"make-export-all-special", "GOFLAGS=-toolexec=script/audit.sh\n.EXPORT_ALL_VARIABLES:\n", ""},
		{"make-target-export", "check: export GOFLAGS=-toolexec=script/audit.sh\n", ""},
		{"make-target-override-export", "check: override export GOFLAGS=-toolexec=script/audit.sh\n", ""},
		{"make-tab-export", "export\tGOFLAGS=-toolexec=script/audit.sh\n", ""},
		{"make-override-export", "override export GOFLAGS=-toolexec=script/audit.sh\n", ""},
		{"make-unknown-export", "export FUTURE_TOOL_SETTING=script/audit.sh\n", ""},
		{"make-export-effectful-value", "export CORVINT_REQUIRE_NODE=$(file <AUDIT_INPUT)\n", ""},
		{"make-computed-export", "SETTING=GOFLAGS\nexport $(SETTING)=-toolexec=script/audit.sh\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := boundFiles(tc.prefix + "check:; " + tc.recipe + "$(if $(CI),go,go) test ./tools/local\nall: check\n")
			files["AUDIT_INPUT"] = "approved\n"
			graph := driverGraph(t, files, nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unmodeled export effects certified check")
			}
			for _, dirty := range []string{"AUDIT_INPUT", sealedCEM} {
				selected := driverSelections(affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
				if witness := selected["driver:make:Makefile#check"].Witness.Kind; witness != affected.WitnessFrontierSite {
					t.Fatalf("got %s; want FRONTIER_SITE", witness)
				}
			}
		})
	}
}

// AFP-V0-3121/3122: enumerating outcomes does not model condition reads.
func TestFrontierSiteConditionEffectsKeepClaim_AFP_V0_3121(t *testing.T) {
	for _, condition := range []string{
		"$(file <MODE)",
		"$(future-function MODE)",
		"$(call file,<MODE)",
		"$($(FUNCTION) MODE)",
		strings.Repeat("$(strip ", 12) + "$(file <MODE)" + strings.Repeat(")", 12),
		"$(info $(file <MODE))",
		"$(origin $(file <MODE))",
		"$(abspath $(file <MODE))",
		"$(if $(file <MODE),yes,no)",
	} {
		for _, immediate := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/immediate=%v", condition, immediate), func(t *testing.T) {
				prefix := ""
				if immediate {
					prefix = "MODE_VALUE := " + condition + "\n"
					condition = "$(MODE_VALUE)"
				}
				files := boundFiles(prefix + "check:; $(if " + condition + ",go test ./tools/ci,go test ./tools/local)\nall: check\n")
				files["MODE"] = "ci\n"
				graph := driverGraph(t, files, nil)
				if !computedSite(t, graph, "driver:make:Makefile#check") {
					t.Fatal("unmodeled condition effects certified check")
				}
				for _, dirty := range []string{"MODE", sealedCEM} {
					selected := driverSelections(affected.Select(graph, []string{dirty}))
					wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
				}
			})
		}
	}
}

// AFP-V0-3128: the admitted directory glob represents its whole read set,
// including newly added extensionless files and a changed working directory.
func TestFrontierSiteDirectoryGlobEdges_AFP_V0_3128(t *testing.T) {
	for _, recipe := range []string{
		"$(if $(CI),cat,cat) data/*",
		"cd work && $(if $(CI),cat,cat) data/*",
	} {
		t.Run(recipe, func(t *testing.T) {
			files := boundFiles("check:; " + recipe + "\nall: check\n")
			prefix := "data/"
			if strings.HasPrefix(recipe, "cd ") {
				prefix = "work/data/"
			}
			files[prefix+"OLD"] = "approved\n"
			graph := driverGraph(t, files, nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("explicit directory edge should bound the admitted glob")
			}
			for _, dirty := range []string{prefix + "OLD", prefix + "NEW"} {
				selected := driverSelections(affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
				if witness := selected["driver:make:Makefile#check"].Witness.Kind; witness == affected.WitnessFrontierSite {
					t.Fatal("directory read must have an edge witness")
				}
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
		})
	}
}

// AFP-V0-3128: environment effects retain claims without make expansion.
func TestFrontierSiteLiteralExportEffects_AFP_V0_3128(t *testing.T) {
	for _, tc := range []struct{ name, prefix, recipe string }{
		{"shell-export", "", "export GOFLAGS=-toolexec=script/audit.sh; "},
		{"shell-unknown-name", "", "export FUTURE_TOOL_SETTING=script/audit.sh; "},
		{"shell-unknown-option", "", "export --future-option GOFLAGS; "},
		{"shell-assignment-export", "", "GOFLAGS=-toolexec=script/audit.sh; export GOFLAGS; "},
		{"shell-command-local", "", "GOFLAGS=-toolexec=script/audit.sh "},
		{"shell-env", "", "env GOFLAGS=-toolexec=script/audit.sh "},
		{"make-export", "export GOFLAGS=-toolexec=script/audit.sh\n", ""},
		{"make-unknown-name", "export FUTURE_TOOL_SETTING=script/audit.sh\n", ""},
		{"make-export-all", "GOFLAGS=-toolexec=script/audit.sh\nexport\n", ""},
		{"make-target-export", "check: export GOFLAGS=-toolexec=script/audit.sh\n", ""},
		{"make-export-effectful-control", "export LANE=$(file <AUDIT_INPUT)\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := boundFiles(tc.prefix + "check:; " + tc.recipe + "go test ./tools/local\nall: check\n")
			files["AUDIT_INPUT"] = "approved\n"
			files["script/audit.sh"] = "#!/bin/sh\ncat AUDIT_INPUT\n"
			graph := driverGraph(t, files, nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("literal recipe lost unmodeled environment effects")
			}
			for _, dirty := range []string{"AUDIT_INPUT", sealedCEM} {
				selected := driverSelections(affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
				if witness := selected["driver:make:Makefile#check"].Witness.Kind; witness != affected.WitnessFrontierSite {
					t.Fatalf("got %s; want FRONTIER_SITE", witness)
				}
			}
		})
	}
}

// AFP-V0-3121/3122: value alternatives cannot discharge filesystem effects.
func TestFrontierSiteFilesystemConditionEffects_AFP_V0_3121(t *testing.T) {
	for _, function := range []string{"realpath MODE", "wildcard MODE", "wildcard MODE*", "future-filesystem-function MODE"} {
		for _, present := range []bool{false, true} {
			for _, immediate := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/present=%v/immediate=%v", function, present, immediate), func(t *testing.T) {
					condition := "$(" + function + ")"
					prefix := ""
					if immediate {
						prefix = "MODE_VALUE := " + condition + "\n"
						condition = "$(MODE_VALUE)"
					}
					files := boundFiles(prefix + "check:; $(if " + condition + ",go test ./tools/ci,go test ./tools/local)\nall: check\n")
					if present {
						files["MODE"] = "ci\n"
					}
					graph := driverGraph(t, files, nil)
					bounded := function == "wildcard MODE" && present
					if computedSite(t, graph, "driver:make:Makefile#check") == bounded {
						t.Fatal("filesystem effect certification disagrees with literal read allowlist")
					}
					for _, dirty := range []string{"MODE", sealedCEM} {
						selected := driverSelections(affected.Select(graph, []string{dirty}))
						if bounded && dirty == sealedCEM {
							wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
							continue
						}
						wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
						if witness := selected["driver:make:Makefile#check"].Witness.Kind; (witness == affected.WitnessFrontierSite) == bounded {
							t.Fatalf("got %s with bounded=%v", witness, bounded)
						}
					}
				})
			}
		}
	}
}

// AFP-V0-3121/3122: imported targets do not model included evaluation effects.
func TestFrontierSiteIncludeEffects_AFP_V0_3121(t *testing.T) {
	for _, contents := range []string{
		"RUN = sh script/lsfiles.sh\n",
		"RUN := $(future-include-function MODE)\n",
	} {
		t.Run(contents, func(t *testing.T) {
			files := boundFiles("RUN = $(if $(CI),go test ./tools/ci,go test ./tools/local)\ninclude config.mk\ncheck:; $(RUN)\nall: check\n")
			files["config.mk"] = contents // No targets to import.
			graph := driverGraph(t, files, nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("included effects were certified without an evaluator")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if selected["driver:make:Makefile#check"].Witness.Kind != affected.WitnessFrontierSite {
				t.Fatal("included effects lost FRONTIER_SITE")
			}
		})
	}
}

// AFP-V0-3128: command-local environment can run hooks outside the operands.
func TestFrontierSiteCommandEnvironmentEffects_AFP_V0_3128(t *testing.T) {
	for _, prefix := range []string{
		"BASH_ENV=./hook.sh ",
		"env BASH_ENV=./hook.sh ",
		"FUTURE_TOOL_SETTING=./hook.sh ",
		"env FUTURE_TOOL_SETTING=./hook.sh ",
	} {
		t.Run(prefix, func(t *testing.T) {
			files := boundFiles("check:; " + prefix + "$(if $(CI),bash,bash) -c 'go test ./tools/local'\nall: check\n")
			files["hook.sh"] = "#!/bin/sh\ncat HIDDEN_INPUT\n"
			files["HIDDEN_INPUT"] = "hook input\n"
			graph := driverGraph(t, files, nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unmodeled command-local environment was certified")
			}
			for _, dirty := range []string{"HIDDEN_INPUT", sealedCEM} {
				selected := driverSelections(affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
				if selected["driver:make:Makefile#check"].Witness.Kind != affected.WitnessFrontierSite {
					t.Fatal("command-local environment lost FRONTIER_SITE")
				}
			}
		})
	}
}

// AFP-V0-3121: GNU make removes through the last slash without cleaning paths.
func TestFrontierSiteNotdirCondition_AFP_V0_3121(t *testing.T) {
	for _, tc := range []struct {
		condition string
		truth     bool
	}{
		{"$(notdir cache/)", false},
		{"$(notdir cache//)", false},
		{"$(notdir cache/../)", false},
		{"$(notdir /)", false},
		// Two empty results retain their separator, which is nonempty.
		{"$(notdir cache/ other/)", true},
		{"$(future-path-function cache/)", false},
	} {
		t.Run(tc.condition, func(t *testing.T) {
			first, second := "go test ./tools/local", "sh script/lsfiles.sh"
			if tc.truth {
				first, second = second, first
			}
			graph := driverGraph(t, boundFiles("check:; $(if "+tc.condition+","+first+","+second+")\nall: check\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("condition hid the enumerating branch")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
		})
	}
}

// AFP-V0-3121: an unvisited foreach body cannot certify discarded effects.
func TestFrontierSiteForeachLimitEffects_AFP_V0_3121(t *testing.T) {
	for _, tc := range []struct{ name, list, body string }{
		{"over-limit-shell", strings.Repeat("item ", 257), "$(shell script/lsfiles.sh)"},
		{"unknown-list", "$(UNKNOWN_ITEMS)", "$(shell script/lsfiles.sh)"},
		{"unknown-body", "item", "$(future-body-function MODE)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			condition := "$(foreach item," + tc.list + "," + tc.body + ")"
			graph := driverGraph(t, boundFiles("check:; $(if "+condition+",go test ./tools/ci,go test ./tools/local)\nall: check\n"), nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("discarded condition certified unknown loop effects")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if selected["driver:make:Makefile#check"].Witness.Kind != affected.WitnessFrontierSite {
				t.Fatal("unknown loop effects lost FRONTIER_SITE")
			}
		})
	}
}

// AFP-V0-2956 and AFP-V0-3125: output captured while make reads an immediate
// assignment retains execution edges at that point, before later reassignment.
func TestFrontierSiteParseTimeListingKeepsExecution_AFP_V0_3125(t *testing.T) {
	for _, command := range []string{"go list", "go vet"} {
		t.Run(command, func(t *testing.T) {
			graph := driverGraph(t, boundFiles("PKG = ./tools/local\nOUT := $(shell "+command+" $(PKG))\nPKG = ./tools/ci\ncheck:\n\techo $(if $(OUT),yes,no)\n"), nil)
			unit, found := graph.Unit("driver:make:Makefile#check")
			if !found {
				t.Fatal("check target missing")
			}
			if !slices.Contains(unit.PathImports, "tools/local") {
				t.Fatalf("captured command lost execution path: paths=%v buildPaths=%v", unit.PathImports, unit.BuildPathImports)
			}
		})
	}
}

// TestFrontierSiteMakeDirSpelling_AFP_V0_9050 is Greptile's PR #142 case:
// GNU make keeps the literal spelling `./a/` that dir returns, so the filter
// matches and `check` tests ./ci. path.Dir would clean it to `a/` and the
// planner would follow only ./local.
func TestFrontierSiteMakeDirSpelling_AFP_V0_9050(t *testing.T) {
	files := boundFiles("check:; go test ./$(if $(filter ./a/,$(dir ./a/file)),ci,local)\nall: check\n")
	files["ci/ci.go"] = "package ci\n"
	files["local/local.go"] = "package local\n"
	graph := driverGraph(t, files, nil)
	selected := driverSelections(affected.Select(graph, []string{"ci/ci.go"}))
	wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
}

// TestFrontierSiteMakeFunctionSpelling_AFP_V0_9050 places the enumerating
// branch on each side of a condition. An exact condition visits only the
// branch GNU make takes; a retained one (truth -1) visits both.
func TestFrontierSiteMakeFunctionSpelling_AFP_V0_9050(t *testing.T) {
	for _, tc := range []struct {
		prelude, condition string
		truth              int
	}{
		{"", "$(filter ./a/,$(dir ./a/file))", 1},
		{"", "$(filter a/,$(dir ./a/file))", 0},
		{"", "$(filter ./,$(dir file))", 1},
		{"", "$(filter a//,$(dir a//b))", 1},
		// suffix emits nothing for a word without one.
		{"", "$(suffix a b)", 0},
		// An empty search string matches once, at the end of the text.
		{"", "$(subst ,x,)", 1},
		// Blanks after a function name are not part of its first argument.
		{"", "$(subst  ,x,)", 1},
		// A substitution reference expands its search and replacement.
		{"V = a.c\nX = .c\n", "$(filter a.o,$(V:$(X)=.o))", 1},
		// Partly unknown words may expand to another spelling.
		{"", "$(notdir $(CI)/)", -1},
		{"", "$(basename $(CI).)", -1},
		// GNU make splits words at ASCII whitespace only.
		{"", "$(filter a, a)", -1},
		// Numbers are decimal digits; GNU make rejects a sign.
		{"", "$(word +1,a)", -1},
		// A backslash may quote a pattern's percent sign.
		{"", "$(filter a\\%,a%)", -1},
		{"", "$(patsubst a\\%,b,a%)", -1},
		// Wildcard returns a literal name as spelled, so only clean,
		// distinct, non-symlink spellings are exact.
		{"", "$(wildcard script//audit.sh)", -1},
		{"", "$(wildcard script/audit.sh script/audit.sh)", -1},
		{"", "$(wildcard script/)", -1},
	} {
		for _, onTrue := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/enumerate-on-%t", tc.condition, onTrue), func(t *testing.T) {
				first, second := "go test ./tools/local", "sh script/lsfiles.sh"
				if onTrue {
					first, second = second, first
				}
				graph := driverGraph(t, boundFiles(tc.prelude+"check:; $(if "+tc.condition+","+first+","+second+")\nall: check\n"), nil)
				want := tc.truth < 0 || onTrue == (tc.truth == 1)
				if got := computedSite(t, graph, "driver:make:Makefile#check"); got != want {
					t.Fatalf("computed site %t; want %t", got, want)
				}
				if want {
					selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
					wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
				}
			})
		}
	}
}

// TestFrontierSiteUnrecordableOperand_AFP_V0_9051 starts from Greptile's
// PR #142 case: neither pathOperand nor driverPathTokens records an absolute
// operand, so a certified `cat /work/project/INPUT` would read a file no
// edge names. Every certification path retains an operand it cannot record.
func TestFrontierSiteUnrecordableOperand_AFP_V0_9051(t *testing.T) {
	for _, recipe := range []string{
		"$(if yes,cat,cat) /work/project/INPUT",
		"$(if yes,cat,cat) ~/INPUT",
		"$(if yes,cat,cat) ../INPUT",
		"$(if yes,cat,cat) -- /work/project/INPUT",
		"$(if yes,cat,cat) -- -INPUT",
		"$(if yes,wc,wc) -l < /work/project/INPUT",
		"$(if yes,wc,wc) -l < ~/INPUT",
		"$(if yes,test,test) -f /work/project/INPUT",
		"$(if yes,grep,grep) -f /work/project/PATTERNS known.txt",
		"$(if yes,grep,grep) -e x /work/project/INPUT",
		"$(if yes,sort,sort) -o ~/out known.txt",
		"$(if yes,sh,sh) script/audit_test.sh /work/project/INPUT",
		"$(if yes,script/audit_test.sh,script/audit_test.sh) /work/project/INPUT",
		"$(if yes,~/audit_test.sh,~/audit_test.sh)",
		"$(if yes,make,make) /work/project/INPUT",
		"$(if yes,make,make) -C /work/project other",
		"$(if yes,go,go) test /work/project/tools/local",
		"$(if yes,go,go) -C /work/project test ./tools/local",
		"$(if yes,go,go) test ./tools/local -args /work/project/INPUT",
	} {
		t.Run(recipe, func(t *testing.T) {
			files := boundFiles("check:; " + recipe + "\nother:\n\tgo test ./tools/local\nall: check\n")
			files["known.txt"] = "x\n"
			files["-INPUT"] = "x\n"
			graph := driverGraph(t, files, nil)
			if !computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("unrecordable operand certified check")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if witness := selected["driver:make:Makefile#check"].Witness; witness.Kind != affected.WitnessFrontierSite {
				t.Errorf("check witness %+v; want FRONTIER_SITE", witness)
			}
		})
	}
}

// AFP-V0-9051: a recorded alias does not cover the file a symlink reads,
// whether the link is the operand itself or an intermediate directory.
func TestFrontierSiteLinkedOperand_AFP_V0_9051(t *testing.T) {
	for _, tc := range []struct {
		name, operand, linkTarget string
	}{
		{"file", "alias", "data/INPUT"},
		{"directory", "alias/INPUT", "data"},
		{"plain", "data/INPUT", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			files := boundFiles("check:; $(if yes,cat,cat) " + tc.operand + "\nall: check\n")
			files["data/INPUT"] = "x\n"
			for name, body := range files {
				file := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.linkTarget != "" {
				if err := os.Symlink(tc.linkTarget, filepath.Join(root, "alias")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			graph, err := affected.Build(t.Context(), root, affected.Driver{})
			if err != nil {
				t.Fatal(err)
			}
			linked := tc.linkTarget != ""
			if got := computedSite(t, graph, "driver:make:Makefile#check"); got != linked {
				t.Fatalf("computed site %t; want %t", got, linked)
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			units := []string{"driver:make:Makefile#check", makeAll, jobUnit}
			if linked {
				wantDriverSelection(t, selected, units, nil)
			} else {
				wantDriverSelection(t, selected, nil, units)
			}
			selected = driverSelections(affected.Select(graph, []string{"data/INPUT"}))
			wantDriverSelection(t, selected, units, nil)
		})
	}
}

// TestFrontierSiteRecordedOperandDropsClaim_AFP_V0_9051 is the control:
// patterns, translation sets and /dev/null are not file operands, and
// relative operands are recorded, so these evaluated targets keep no site.
func TestFrontierSiteRecordedOperandDropsClaim_AFP_V0_9051(t *testing.T) {
	for _, recipe := range []string{
		"$(if yes,cat,cat) known.txt",
		"$(if yes,grep,grep) ^/work known.txt",
		"$(if yes,tr,tr) / _ < known.txt",
		"$(if yes,wc,wc) -l < /dev/null",
		"$(if yes,test,test) -f known.txt",
	} {
		t.Run(recipe, func(t *testing.T) {
			files := boundFiles("check:; " + recipe + "\nall: check\n")
			files["known.txt"] = "x\n"
			graph := driverGraph(t, files, nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("recorded operands left check a computed site")
			}
		})
	}
}
