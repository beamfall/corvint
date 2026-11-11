// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"slices"
	"testing"
	"testing/fstest"
)

// compsiteCache is a GOCACHE value make evaluates through `or` over an
// environment-only TMPDIR, the root Makefile's CORVINT_GOCACHE shape: a
// recipe that expands it is an evaluated target (AFP-V0-3122).
const compsiteCache = "C ?= $(patsubst %/,%,$(or $(TMPDIR),/tmp))/cache\n"

// compsiteFiles is a repository whose make target `check` runs recipe, with a
// caller target, a CI job that runs the caller and an unrelated target.
func compsiteFiles(makefile string) fstest.MapFS {
	return fstest.MapFS{
		"go.mod":                   {Data: []byte("module example.test/m\n\ngo 1.27.0\n")},
		"Makefile":                 {Data: []byte(makefile + "all: check\nother:\n\techo done\n")},
		".github/workflows/ci.yml": {Data: []byte("jobs:\n  caller:\n    steps:\n      - run: make all\n  unrelated:\n    steps:\n      - run: echo done\n")},
		"tools/check/main.go":      {Data: []byte("package main\n\nfunc main() {}\n")},
		"tools/other/main.go":      {Data: []byte("package main\n\nfunc main() {}\n")},
		"script/a.sh":              {Data: []byte("#!/bin/sh\necho a\n")},
		"x.mk":                     {Data: []byte("T = ./tools/check\n")},
		"lit.sh":                   {Data: []byte("#!/bin/sh\necho lit\n")},
	}
}

// compsiteDirty are dirty paths no recipe names: a new build-log entry, a
// deleted path, an unindexed source, a dot path and a Go source of a tool.
var compsiteDirty = []string{"docs/build-log/2026-11-11-probe.md", "removed/file.txt", "unindexed/new.go", ".corvint/changes/new.cem.json", "tools/other/main.go"}

const (
	compsiteCheck  = "driver:make:Makefile#check"
	compsiteAll    = "driver:make:Makefile#all"
	compsiteCaller = "driver:ci:.github/workflows/ci.yml#caller"
	compsiteOther  = "driver:make:Makefile#other"
)

// AFP-V0-3600: an evaluated target whose make values reach only a GOCACHE
// value, and which reads the whole tree, is no computed site. It keeps its
// file-set edge, so every dirty path still selects it and its callers.
func TestDriverEvaluatedWholeTreeReader_AFP_V0_3600(t *testing.T) {
	for _, makefile := range []string{
		compsiteCache + "check:\n\tGOCACHE=$(C) GOTOOLCHAIN=local go run ./tools/check\n",
		compsiteCache + "check:\n\t@GOCACHE=$(C) GOTOOLCHAIN=local go run ./tools/check >/dev/null\n",
		compsiteCache + "check:\n\tenv GOCACHE=$(C) GOTOOLCHAIN=local go run ./tools/check\n",
		compsiteCache + "check:\n\tGOCACHE=$(C) go run ./tools/check --name $@\n",
		"check:\n\t$(if $(CI),git ls-files,find .)\n",
		"check:\n\t$(if $(CI),go run ./tools/check,go run ./tools/other)\n",
	} {
		t.Run(makefile, func(t *testing.T) {
			files := compsiteFiles(makefile)
			graph, err := BuildFS(files, Driver{})
			if err != nil {
				t.Fatal(err)
			}
			unit, _ := graph.Unit(compsiteCheck)
			if len(unit.Frontier) != 0 || !slices.Contains(unit.PathGlobs, driverTreeReadGlob) {
				t.Fatalf("check is still a computed site or lost its file-set edge: %+v", unit)
			}
			reasons := driverComputedReasons(t, FSSource(files))
			if _, computed := reasons[compsiteCheck]; computed {
				t.Fatalf("check still records computed reasons %v", reasons[compsiteCheck])
			}
			for _, dirty := range compsiteDirty {
				selected := witnesses(Select(graph, []string{dirty}))
				for _, id := range []string{compsiteCheck, compsiteAll, compsiteCaller} {
					if _, found := selected[id]; !found {
						t.Errorf("%s: lost %s: %v", dirty, id, selected)
					}
				}
				if selected[compsiteCheck].Kind == WitnessFrontierSite {
					t.Errorf("%s: check keeps the FRONTIER_SITE witness: %+v", dirty, selected[compsiteCheck])
				}
				if _, found := selected[compsiteOther]; found {
					t.Errorf("%s: selected the unrelated target", dirty)
				}
			}
		})
	}
}

// AFP-V0-3600 fail-closed neighbours: each recipe is an evaluated whole-tree
// reader in which a make value reaches a word other than a GOCACHE value, or
// whose file or text the proof does not model. Each stays a computed site
// selected as FRONTIER_SITE, with its callers, on a path no recipe names.
func TestDriverEvaluatedCacheOnlyFailClosed_AFP_V0_3600(t *testing.T) {
	const run = "GOCACHE=$(C) go run ./tools/check"
	for _, tc := range []struct{ name, makefile string }{
		{"command-word-overridable", compsiteCache + "A ?= $(if $(CI),,nice)\ncheck:\n\tGOCACHE=$(C) $(A) go run ./tools/check\n"},
		{"command-word-single-definition", compsiteCache + "GO = go\ncheck:\n\tGOCACHE=$(C) $(GO) run ./tools/check\n"},
		{"make-command-word", compsiteCache + "check:\n\t" + run + " && $(MAKE) other\n"},
		{"operand-environment", compsiteCache + "check:\n\t" + run + " $(or $(ARG),lit)\n"},
		{"operand-single-definition", compsiteCache + "T = ./tools/check\ncheck:\n\tGOCACHE=$(C) go run $(T)\n"},
		{"operand-recursive-unresolved", compsiteCache + "T = $(U)/check\ncheck:\n\tGOCACHE=$(C) go run ./tools/check $(T)\n"},
		{"operand-conditional", compsiteCache + "ifeq ($(CI),1)\nT = ./tools/other\nelse\nT = ./tools/check\nendif\ncheck:\n\tGOCACHE=$(C) go run $(T)\n"},
		{"operand-reassigned", compsiteCache + "T = ./tools/other\nT = ./tools/check\ncheck:\n\tGOCACHE=$(C) go run $(T)\n"},
		{"operand-define", compsiteCache + "define T\n./tools/check\nendef\ncheck:\n\tGOCACHE=$(C) go run $(T)\n"},
		{"operand-override", compsiteCache + "override T = ./tools/check\ncheck:\n\tGOCACHE=$(C) go run $(T)\n"},
		{"operand-target-specific", compsiteCache + "check: T = ./tools/check\ncheck:\n\tGOCACHE=$(C) go run $(T)\n"},
		{"operand-pattern-specific", compsiteCache + "chec%: T = ./tools/check\ncheck:\n\tGOCACHE=$(C) go run $(T)\n"},
		{"operand-include", compsiteCache + "include x.mk\ncheck:\n\tGOCACHE=$(C) go run $(T)\n"},
		{"operand-call", compsiteCache + "f = $(1)\ncheck:\n\tGOCACHE=$(C) go run $(call f,./tools/check)\n"},
		{"operand-eval", compsiteCache + "check:\n\t$(eval Y = 1)GOCACHE=$(C) go run ./tools/check\n"},
		{"operand-shell", compsiteCache + "check:\n\t" + run + " $(shell echo x)\n"},
		{"discarded-shell", compsiteCache + "check:\n\t$(if $(shell script/a.sh),,)" + run + "\n"},
		{"discarded-pure-shell", compsiteCache + "check:\n\t$(if $(shell echo x),,)" + run + "\n"},
		{"discarded-wildcard", compsiteCache + "check:\n\t$(if $(wildcard lit.sh),,)" + run + "\n"},
		{"discarded-unmodeled-function", compsiteCache + "check:\n\t$(if $(file < lit.sh),,)" + run + "\n"},
		{"operand-automatic", compsiteCache + "check: other\n\t" + run + " $<\n"},
		{"prerequisite", compsiteCache + "DEP ?= other\ncheck: $(DEP)\n\t" + run + "\n"},
		{"interpreter-operand", compsiteCache + "F ?= lit.sh\ncheck:\n\t" + run + " && sh $(F)\n"},
		{"source-operand", compsiteCache + "F ?= lit.sh\ncheck:\n\t" + run + " && . $(F)\n"},
		{"cd-operand", compsiteCache + "D ?= tools\ncheck:\n\tcd $(D) && " + run + "\n"},
		{"pushd-operand", compsiteCache + "D ?= tools\ncheck:\n\tpushd $(D) && " + run + "\n"},
		{"redirect-target-executed", compsiteCache + "OUT ?= gen.sh\ncheck:\n\t" + run + " > $(OUT) && sh gen.sh\n"},
		{"generated-file-executed", compsiteCache + "X ?= true\ncheck:\n\techo $(X) > gen.sh && " + run + " && sh gen.sh\n"},
		{"generated-file-sourced", compsiteCache + "X ?= true\ncheck:\n\tprintf '%s\\n' '$(X)' > gen.sh && " + run + " && . ./gen.sh\n"},
		{"eval-builtin", compsiteCache + "X ?= true\ncheck:\n\t" + run + " && eval '$(X)'\n"},
		{"command-substitution", compsiteCache + "F ?= lit.sh\ncheck:\n\t" + run + " $$(cat $(F))\n"},
		{"inline-script", compsiteCache + "X ?= true\ncheck:\n\t" + run + " && sh -c '$(X)'\n"},
		{"other-assignment", compsiteCache + "X ?= 1\ncheck:\n\tFOO=$(X) " + run + "\n"},
		{"gocache-after-wrapper", compsiteCache + "check:\n\tnice GOCACHE=$(C) go run ./tools/check\n"},
		{"gocache-as-operand", compsiteCache + "check:\n\t" + run + " GOCACHE=$(C)\n"},
		{"read-time-command", compsiteCache + "V != script/a.sh\ncheck:\n\t" + run + "\n"},
		{"heredoc", compsiteCache + "check:\n\t" + run + " <<EOF\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph, err := BuildFS(compsiteFiles(tc.makefile), Driver{})
			if err != nil {
				t.Fatal(err)
			}
			unit, _ := graph.Unit(compsiteCheck)
			if !slices.Contains(unit.Frontier, FrontierDriverComputedInvocation) {
				t.Fatalf("check lost its computed frontier: %+v", unit)
			}
			for _, dirty := range compsiteDirty[:2] {
				selected := witnesses(Select(graph, []string{dirty}))
				for _, id := range []string{compsiteCheck, compsiteAll, compsiteCaller} {
					if _, found := selected[id]; !found {
						t.Errorf("%s: lost %s: %v", dirty, id, selected)
					}
				}
				if selected[compsiteCheck].Kind != WitnessFrontierSite {
					t.Errorf("%s: check witness %+v; want FRONTIER_SITE", dirty, selected[compsiteCheck])
				}
			}
		})
	}
	// A pattern rule's stem is unknown to the proof; the pattern unit keeps
	// its frontier and the concrete caller is selected through it.
	graph, err := BuildFS(compsiteFiles(compsiteCache+"chec%:\n\t"+run+" $*\n"), Driver{})
	if err != nil {
		t.Fatal(err)
	}
	if unit, _ := graph.Unit("driver:make:Makefile#chec%"); !slices.Contains(unit.Frontier, FrontierDriverComputedInvocation) {
		t.Fatalf("pattern-stem: chec%% lost its computed frontier: %+v", unit)
	}
	selected := witnesses(Select(graph, []string{compsiteDirty[0]}))
	for _, id := range []string{compsiteAll, compsiteCaller} {
		if _, found := selected[id]; !found {
			t.Errorf("pattern-stem: lost %s: %v", id, selected)
		}
	}
}
