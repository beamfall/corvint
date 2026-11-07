// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
// for it, including a new sealed CEM; the control never selected it. A target
// that runs a script enumerating every tracked file keeps the claim under the
// same injection, since its read set includes the CEM (AFP-V0-3122).
func TestFrontierSiteFaultInjection_AFP_V0_3120(t *testing.T) {
	recipe := "check:\n\tgo test ./tools/local/...\nenumerate:\n\tscript/lsfiles.sh\n"
	for _, tc := range []struct {
		name, prefix string
		evaluated    bool
	}{
		{"control", "", false},
		{"injected-or", "CACHE = $(or $(XDG_CACHE_HOME),/tmp)/cache\n", true},
		{"injected-if", "NICE = $(if $(filter-out 0,$(LEVEL)),nice -n $(LEVEL))\n", true},
		{"injected-reassigned", "MODE = fast\nMODE = slow\n", false},
		{"injected-join", "PAIR = $(join a b,$(SUFFIXES))\n", true},
		{"info-never-expands", "$(info $(or $(XDG_CACHE_HOME),/tmp))\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.prefix+recipe), nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Errorf("check runs only `go test ./tools/local/...` but is a computed site")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", jobUnit})
			if _, claimed := selected["driver:make:Makefile#enumerate"]; claimed != tc.evaluated {
				t.Errorf("the enumerating target claims the CEM = %v, want %v (the base claimed it exactly when the file was ambiguous)", claimed, tc.evaluated)
			}
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
			graph := driverGraph(t, boundFiles(tc.makefile+"all: check\n"), nil)
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
			files := boundFiles("CACHE = $(or $(XDG_CACHE_HOME),/tmp)\ncheck:\n\t" + name + "\n")
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
	const evaluated = "CACHE = $(or $(XDG_CACHE_HOME),/tmp)\n"
	for _, tc := range []struct{ name, makefile string }{
		{"environment-operand", "INPUT = $(or $(FILE),known.txt)\ncheck:\n\tcat $(INPUT)\n"},
		{"environment-redirection", "INPUT = $(or $(FILE),known.txt)\ncheck:\n\twc -l < $(INPUT)\n"},
		{"environment-test-path", "INPUT = $(or $(FILE),known.txt)\ncheck:\n\ttest -f $(INPUT)\n"},
		{"shell-variable-operand", evaluated + "check:\n\tfor f in a b; do cat \"$$f\"; done\n"},
		{"xargs", evaluated + "check:\n\txargs cat < known.txt\n"},
		{"unknown-directory", evaluated + "check:\n\tcd $(CACHE) && cat known.txt\n"},
		{"checksum-manifest", evaluated + "check:\n\tsha256sum -c checksums.txt\n"},
		{"checksum-manifest-long", evaluated + "check:\n\tshasum --check --quiet checksums.txt\n"},
		{"checksum-manifest-cluster", evaluated + "check:\n\tmd5sum -sc checksums.txt\n"},
		{"files0-from", evaluated + "check:\n\twc -l --files0-from=list.txt\n"},
		{"jq-module", evaluated + "check:\n\tjq -n 'import \"lib\" as lib; lib::x'\n"},
		{"jq-library-path", evaluated + "check:\n\tjq -L lib -n '1'\n"},
		{"awk", evaluated + "check:\n\tawk '{ while ((getline line < $$1) > 0) print line }' known.txt\n"},
		{"sed", evaluated + "check:\n\tsed -n '1r other.txt' known.txt\n"},
		{"go-environment-operand", "PKG = $(or $(PKGS),./tools/local)\ncheck:\n\tgo test $(PKG)\n"},
		{"go-environment-flags", evaluated + "check:\n\tgo test $(GOFLAGS_EXTRA) ./tools/local\n"},
		{"make-override", evaluated + "check:\n\t$(MAKE) other PKG=ci\nother:\n\tgo test ./tools/local\n"},
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
// same commands in modes that read only their literal operands, an unknown
// value only where it names an output, leave the evaluated target no site.
func TestFrontierSiteBoundedOperandDropsClaim_AFP_V0_3128(t *testing.T) {
	const evaluated = "CACHE = $(or $(XDG_CACHE_HOME),/tmp)\n"
	for _, tc := range []struct{ name, makefile string }{
		{"checksum-compute", evaluated + "check:\n\tsha256sum known.txt > $(CACHE)/sums\n"},
		{"literal-redirection", evaluated + "check:\n\twc -l < known.txt\n"},
		{"literal-test", evaluated + "check:\n\ttest -f known.txt && test -n \"$(CACHE)\"\n"},
		{"jq-literal", evaluated + "check:\n\tjq -r .name known.json\n"},
		{"go-output", evaluated + "check:\n\tgo build -o $(CACHE)/bin ./tools/local\n"},
		{"make-literal", evaluated + "check:\n\t$(MAKE) other\nother:\n\tgo test ./tools/local\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.makefile+"all: check\n"), nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatalf("check reads only literal operands, but it is a computed site")
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check"})
		})
	}
}
