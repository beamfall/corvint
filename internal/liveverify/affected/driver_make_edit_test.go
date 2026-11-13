// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// makeEditBase is the make file every NCM-V1-011 and NCM-V1-012 case edits.
const makeEditBase = `FLAGS = -v
OUT = out/$(FLAGS)
TOOL ?= go
.PHONY: all check vet
all: check vet
check:
	$(TOOL) test $(FLAGS) ./tool
vet:
	$(TOOL) vet ./tool
report: | out
	echo $(OUT) > report.txt
out:
	mkdir -p out
`

// TestAttributeMakeEditTargets_NCMV1011 pins what each attributable edit
// selects: a rule's own text (recipe, prerequisites, .PHONY membership,
// target-specific assignment, a new rule), a variable through every rule
// that uses it, and a variable through another variable's assignment. A
// comment or blank line selects nothing.
func TestAttributeMakeEditTargets_NCMV1011(t *testing.T) {
	for _, tc := range []struct {
		name string
		head string
		want []string
	}{
		{"recipe line", strings.Replace(makeEditBase, "\t$(TOOL) vet ./tool", "\t$(TOOL) vet -all ./tool", 1), []string{"vet"}},
		{"prerequisite list", strings.Replace(makeEditBase, "all: check vet", "all: check", 1), []string{"all"}},
		{"phony membership", strings.Replace(makeEditBase, ".PHONY: all check vet", ".PHONY: all check vet report", 1), []string{"report"}},
		{"target-specific assignment", makeEditBase + "check: FLAGS = -race\n", []string{"check"}},
		{"new rule", makeEditBase + "lint:\n\tgo vet ./...\n", []string{"lint"}},
		{"variable in two recipes", strings.Replace(makeEditBase, "TOOL ?= go", "TOOL ?= go1.27", 1), []string{"check", "vet"}},
		{"variable through a variable", strings.Replace(makeEditBase, "FLAGS = -v", "FLAGS = -v -count=1", 1), []string{"check", "report"}},
		{"variable added and used", strings.Replace(makeEditBase, "\tmkdir -p out", "\tmkdir -p $(DIR)", 1) + "DIR = out\n", []string{"out"}},
		{"comment only", "# a comment\n" + strings.Replace(makeEditBase, "check:\n", "check: # runs the tests\n", 1), []string{"check"}},
		{"blank line and comment line", "# heading\n\n" + makeEditBase, nil},
		{"multi-target header", strings.Replace(makeEditBase, "report: | out", "report summary: | out", 1), []string{"report", "summary"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edit := AttributeMakeEdit("Makefile", []byte(makeEditBase), []byte(tc.head))
			if edit.Reason != "" {
				t.Fatalf("not attributed: %s", edit.Reason)
			}
			if !slices.Equal(edit.Targets, tc.want) {
				t.Fatalf("targets %v, want %v", edit.Targets, tc.want)
			}
		})
	}
}

// TestAttributeMakeEditFailsClosed_NCMV1012 is a table of every edit that
// must run everything, each with the reason it names.
func TestAttributeMakeEditFailsClosed_NCMV1012(t *testing.T) {
	replace := func(old, updated string) string {
		if !strings.Contains(makeEditBase, old) {
			t.Fatalf("base lacks %q", old)
		}
		return strings.Replace(makeEditBase, old, updated, 1)
	}
	for _, tc := range []struct {
		name, base, head, reason string
	}{
		{"base outside the parsed subset", makeEditBase + "$(eval X = 1)\n", makeEditBase, "at the base is outside the parsed subset"},
		{"head .ONESHELL", makeEditBase, makeEditBase + ".ONESHELL:\n", "at the head is outside the parsed subset"},
		{"head .RECIPEPREFIX", makeEditBase, ".RECIPEPREFIX = >\n" + makeEditBase, "outside the parsed subset"},
		{"unbalanced conditional", makeEditBase, makeEditBase + "ifdef CI\n", "outside the parsed subset"},
		{"include added", makeEditBase, makeEditBase + "include extra.mk\n", "include line changed"},
		{"optional include", makeEditBase + "-include a.mk\n", makeEditBase + "-include b.mk\n", "include line changed"},
		{"conditional added", makeEditBase, makeEditBase + "ifdef CI\nFLAGS = -race\nendif\n", "conditional changed"},
		{"assignment inside a conditional", makeEditBase + "ifdef CI\nX = 1\nendif\n", makeEditBase + "ifdef CI\nX = 2\nendif\n", "changed"},
		{"rule inside a conditional", makeEditBase + "ifdef CI\nci:\n\techo a\nendif\n", makeEditBase + "ifdef CI\nci:\n\techo b\nendif\n", "inside a conditional changed"},
		{"recipe line inside a conditional", makeEditBase + "ci:\nifdef CI\n\techo a\nendif\n", makeEditBase + "ci:\nifdef CI\n\techo b\nendif\n", "inside a conditional changed"},
		{"define block", makeEditBase + "define RUN\necho a\nendef\n", makeEditBase + "define RUN\necho b\nendef\n", "changed"},
		{"export line", makeEditBase, makeEditBase + "export FLAGS\n", "export line changed"},
		{"override assignment", makeEditBase + "override X = 1\n", makeEditBase + "override X = 2\n", "override assignment changed"},
		{"vpath", makeEditBase, makeEditBase + "vpath %.c src\n", "vpath line changed"},
		{"special target", makeEditBase, makeEditBase + ".NOTPARALLEL:\n", "special target changed"},
		{"default goal", makeEditBase, "first:\n\techo first\n" + makeEditBase, "default goal changed"},
		{"computed target", makeEditBase + "$(FLAGS): ; echo\n", replace("FLAGS = -v", "FLAGS = -x") + "$(FLAGS): ; echo\n", "used outside every explicit rule"},
		{"removed rule", makeEditBase, replace("out:\n\tmkdir -p out\n", ""), `make target "out" was removed`},
		{"pattern rule", makeEditBase + "%.o: %.c\n\tcc -c $<\n", makeEditBase + "%.o: %.c\n\tcc -O2 -c $<\n", `pattern rule "%.o" changed`},
		{"double-colon rule", makeEditBase + "clean::\n\trm -f a\n", makeEditBase + "clean::\n\trm -f b\n", `double-colon rule "clean" changed`},
		{"SHELL", makeEditBase, "SHELL = /bin/bash\n" + makeEditBase, `"SHELL" is a global make setting`},
		{".SHELLFLAGS", makeEditBase, ".SHELLFLAGS = -ec\n" + makeEditBase, `".SHELLFLAGS" is a global make setting`},
		{"MAKEFLAGS", makeEditBase, "MAKEFLAGS += -r\n" + makeEditBase, `"MAKEFLAGS" is a global make setting`},
		{".DEFAULT_GOAL", makeEditBase, ".DEFAULT_GOAL := vet\n" + makeEditBase, "default goal changed"},
		{"built-in rule variable", makeEditBase, "CFLAGS = -O2\n" + makeEditBase, "read by make's built-in rules"},
		{"exported variable", "export HELP_BASE\nHELP_BASE = a\n" + makeEditBase + "help:\n\techo $(HELP_BASE)\n", "export HELP_BASE\nHELP_BASE = b\n" + makeEditBase + "help:\n\techo $(HELP_BASE)\n", "is exported to every recipe"},
		{"unmodeled export", "export FLAGS\n" + makeEditBase, "export FLAGS\n" + replace("FLAGS = -v", "FLAGS = -x"), "exports variables it does not model"},
		{"variable used by no rule", makeEditBase, makeEditBase + "UNUSED = 1\n", `"UNUSED" is used by no rule`},
		{"variable used in a conditional", makeEditBase + "ifeq ($(FLAGS),-v)\nendif\n", replace("FLAGS = -v", "FLAGS = -x") + "ifeq ($(FLAGS),-v)\nendif\n", "used outside every explicit rule"},
		{"shell assignment", makeEditBase, replace("TOOL ?= go", "TOOL != which go"), "runs a shell command"},
		{"shell function", makeEditBase, replace("TOOL ?= go", "TOOL := $(shell which go)"), "calls $(shell)"},
		{"call function", makeEditBase, replace("TOOL ?= go", "TOOL = $(call pick,go)"), "calls $(call)"},
		{"computed reference", makeEditBase + "x:\n\techo $($(NAME))\n", replace("TOOL ?= go", "TOOL ?= gofmt") + "x:\n\techo $($(NAME))\n", "computed reference"},
		{"computed value argument", makeEditBase + "x:\n\techo $(value $(NAME))\n", replace("TOOL ?= go", "TOOL ?= gofmt") + "x:\n\techo $(value $(NAME))\n", "computed reference"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edit := AttributeMakeEdit("Makefile", []byte(tc.base), []byte(tc.head))
			if edit.Reason == "" || edit.Targets != nil {
				t.Fatalf("attributed to %v, want a refusal naming %q", edit.Targets, tc.reason)
			}
			if !strings.Contains(edit.Reason, tc.reason) {
				t.Fatalf("reason %q, want it to name %q", edit.Reason, tc.reason)
			}
		})
	}
}

// makeReachRepository writes a repository the driver units are read from.
func makeReachRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/m\n\ngo 1.27\n"
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const makeReachFile = `CACHE = $(or $(XDG_CACHE_HOME),/tmp)/cache
all: check
check:
	go test ./tool
cached:
	test -d $(CACHE)
	go test ./gen
tree:
	go vet ./core
	find . -name '*.go'
opaque:
	$$RUNNER ./core
nested:
	$(MAKE) -C sub x
`

// TestReachMakeTargets_NCMV1013 pins what a changed target can run: its own
// recipe's paths and its prerequisites' (all reaches check's tool), a
// target whose followed commands read what the graph does not bound (an
// evaluated test operand) stays bounded, and one whose command does not
// resolve, find's whole-tree read included (NCM-V1-019), or that sits in a
// file outside the parsed subset, does not.
func TestReachMakeTargets_NCMV1013(t *testing.T) {
	root := makeReachRepository(t, map[string]string{"Makefile": makeReachFile, "sub/Makefile": "$(eval Y = 1)\nx:\n\techo\n"})
	ctx := context.Background()
	for _, tc := range []struct {
		file    string
		targets []string
		paths   []string
		reason  string
	}{
		{"Makefile", []string{"check"}, []string{"tool"}, ""},
		{"Makefile", []string{"all"}, []string{"tool"}, ""},
		{"Makefile", []string{"cached"}, []string{"gen"}, ""},
		{"Makefile", []string{"tree"}, nil, `make target "tree" runs a command or make goal that does not resolve: shell:unfollowed-command:find`},
		{"Makefile", []string{"opaque"}, nil, `make target "opaque" runs a command or make goal that does not resolve: shell:computed-command-word, shell:unfollowed-command:$?`},
		{"Makefile", []string{"check", "opaque"}, nil, "does not resolve"},
		{"Makefile", []string{"nested"}, nil, `make unit "driver:make:sub/Makefile#*" is in a file outside the parsed subset`},
		{"Makefile", []string{"missing"}, nil, "is not in the driver units"},
	} {
		t.Run(tc.file+":"+strings.Join(tc.targets, "+"), func(t *testing.T) {
			reach := ReachMakeTargets(ctx, root, tc.file, tc.targets)
			if tc.reason != "" {
				if !strings.Contains(reach.Reason, tc.reason) {
					t.Fatalf("reason %q, want %q", reach.Reason, tc.reason)
				}
				return
			}
			if reach.Reason != "" {
				t.Fatalf("not bounded: %s", reach.Reason)
			}
			for _, want := range tc.paths {
				if !slices.Contains(reach.Paths, want) {
					t.Fatalf("paths %v lack %q", reach.Paths, want)
				}
			}
		})
	}
	// The cases above must really be the frontier sites the planner keeps
	// (an evaluated, uncertified site, bounded in the reach, and a
	// whole-tree read, open in it), or the case proves nothing.
	units, err := Driver{}.Units(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range units.Units {
		switch unit.ID {
		case "driver:make:Makefile#cached":
			if !slices.Contains(unit.Frontier, FrontierDriverComputedInvocation) {
				t.Errorf("cached frontier %v, want %s", unit.Frontier, FrontierDriverComputedInvocation)
			}
		case "driver:make:Makefile#tree":
			if len(unit.PathGlobs) == 0 {
				t.Errorf("tree has no whole-tree read")
			}
		}
	}
}

// TestReachMakeTargetsCI_NCMV1014 pins the workflows that invoke a changed
// target directly, and the refusals for a local action that invokes one and
// for a workflow outside the parsed subset.
func TestReachMakeTargetsCI_NCMV1014(t *testing.T) {
	ctx := context.Background()
	workflow := "jobs:\n  unit:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make check\n" //corvint:fixture-path
	root := makeReachRepository(t, map[string]string{"Makefile": makeReachFile, ".github/workflows/ci.yml": workflow})
	if reach := ReachMakeTargets(ctx, root, "Makefile", []string{"check"}); reach.Reason != "" || !slices.Equal(reach.Workflows, []string{".github/workflows/ci.yml"}) {
		t.Fatalf("check: workflows %v reason %q, want the CI workflow", reach.Workflows, reach.Reason)
	}
	if reach := ReachMakeTargets(ctx, root, "Makefile", []string{"all"}); reach.Reason != "" || len(reach.Workflows) != 0 {
		t.Fatalf("all, which no job invokes: workflows %v reason %q", reach.Workflows, reach.Reason)
	}
	unparsed := makeReachRepository(t, map[string]string{"Makefile": makeReachFile, ".github/workflows/ci.yml": workflow, ".github/workflows/odd.yml": "name: odd\n"})
	if reach := ReachMakeTargets(ctx, unparsed, "Makefile", []string{"all"}); !strings.Contains(reach.Reason, "is outside the parsed subset") {
		t.Fatalf("an unparsable workflow: reason %q", reach.Reason)
	}
	action := "runs:\n  using: composite\n  steps:\n    - run: make check\n      shell: bash\n"
	acted := memorySource(t, map[string]string{"go.mod": "module example.com/m\n\ngo 1.27\n", "Makefile": makeReachFile, ".github/actions/setup/action.yml": action})
	if reach := reachMakeTargets(acted, "Makefile", []string{"check"}, nil); !strings.Contains(reach.Reason, "local action") {
		t.Fatalf("a local action invoking check: reason %q", reach.Reason)
	}
}
