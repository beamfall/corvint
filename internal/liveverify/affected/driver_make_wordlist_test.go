// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// makeWordListBase is the make file every NCM-V1-015 case edits: a literal
// word list used in a rule's prerequisites through addprefix and in a
// computed target line, as the repository's GATE_STEPS is.
const makeWordListBase = `STEPS = a b
all: clear $(addprefix ledger/,$(STEPS))
	echo done
$(STEPS) $(addprefix ledger/,$(STEPS)): | clear
ledger/%:
	go run ./ledger $* -- $(MAKE) $*
clear:
	rm -f receipt
a:
	go test ./a
b:
	go test ./b
c:
	go test ./c
`

// TestAttributeMakeEditWordList_NCMV1015 pins what a literal word-list edit
// selects: its changed words as each use evaluates them, the computed line's
// literal prerequisites, and, as Touched, the rules whose prerequisite list
// holds the variable; the words in both lists are not selected.
func TestAttributeMakeEditWordList_NCMV1015(t *testing.T) {
	for _, tc := range []struct {
		name, base, head string
		targets, touched []string
	}{
		{"word added", makeWordListBase, strings.Replace(makeWordListBase, "STEPS = a b", "STEPS = a b c", 1), []string{"c", "clear", "ledger/c"}, []string{"all"}},
		{"word removed", makeWordListBase, strings.Replace(makeWordListBase, "STEPS = a b", "STEPS = a", 1), []string{"b", "clear", "ledger/b"}, []string{"all"}},
		{"immediate assignment", strings.Replace(makeWordListBase, "STEPS = a b", "STEPS := a b", 1), strings.Replace(makeWordListBase, "STEPS = a b", "STEPS := a b c", 1), []string{"c", "clear", "ledger/c"}, []string{"all"}},
		{"continued lines", strings.Replace(makeWordListBase, "STEPS = a b", "STEPS = \\\n  a \\\n  b \\\n  # end", 1), strings.Replace(makeWordListBase, "STEPS = a b", "STEPS = \\\n  a \\\n  b \\\n  c \\\n  # end", 1), []string{"c", "clear", "ledger/c"}, []string{"all"}},
		{"prerequisite list only", "STEPS = a\nall: $(STEPS)\na:\n\tgo test ./a\nc:\n\tgo test ./c\n", "STEPS = a c\nall: $(STEPS)\na:\n\tgo test ./a\nc:\n\tgo test ./c\n", []string{"c"}, []string{"all"}},
		{"computed target line only", "STEPS = a\n$(STEPS): | clear\nclear:\n\trm -f r\na:\n\tgo test ./a\nc:\n\tgo test ./c\n", "STEPS = a c\n$(STEPS): | clear\nclear:\n\trm -f r\na:\n\tgo test ./a\nc:\n\tgo test ./c\n", []string{"c", "clear"}, nil},
		{"addsuffix and patsubst", "STEPS = a\nall: $(addsuffix .ok,$(STEPS)) $(patsubst %,run-%,$(STEPS))\n", "STEPS = a c\nall: $(addsuffix .ok,$(STEPS)) $(patsubst %,run-%,$(STEPS))\n", []string{"c.ok", "run-c"}, []string{"all"}},
		{"with a changed recipe", makeWordListBase, strings.Replace(strings.Replace(makeWordListBase, "STEPS = a b", "STEPS = a b c", 1), "\tgo test ./c", "\tgo test -v ./c", 1), []string{"c", "clear", "ledger/c"}, []string{"all"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edit := AttributeMakeEdit("Makefile", []byte(tc.base), []byte(tc.head))
			if edit.Reason != "" {
				t.Fatalf("not attributed: %s", edit.Reason)
			}
			if !slices.Equal(edit.Targets, tc.targets) || !slices.Equal(edit.Touched, tc.touched) {
				t.Fatalf("targets %v touched %v, want %v and %v", edit.Targets, edit.Touched, tc.targets, tc.touched)
			}
		})
	}
}

// TestAttributeMakeEditWordListFailsClosed_NCMV1015 is a table of word-list
// edits NCM-V1-015 does not evaluate. Each falls back to NCM-V1-011, which
// refuses because the computed target line uses the variable.
func TestAttributeMakeEditWordListFailsClosed_NCMV1015(t *testing.T) {
	edit := func(old, updated string) string {
		if !strings.Contains(makeWordListBase, old) {
			t.Fatalf("base lacks %q", old)
		}
		return strings.Replace(makeWordListBase, old, updated, 1)
	}
	added := edit("STEPS = a b", "STEPS = a b c")
	outside := "used outside every explicit rule"
	for _, tc := range []struct {
		name, base, head, reason string
	}{
		{"non-literal value", makeWordListBase, edit("STEPS = a b", "STEPS = a b $(EXTRA)"), outside},
		{"shell function in the value", makeWordListBase, edit("STEPS = a b", "STEPS = a b $(shell echo c)"), "calls $(shell)"},
		{"shell assignment", makeWordListBase, edit("STEPS = a b", "STEPS != echo a b c"), "runs a shell command"},
		{"append assignment", edit("STEPS = a b", "STEPS = a\nSTEPS += b"), edit("STEPS = a b", "STEPS = a\nSTEPS += b c"), outside},
		{"conditional default", edit("STEPS = a b", "STEPS ?= a b"), edit("STEPS = a b", "STEPS ?= a b c"), outside},
		{"word order only", makeWordListBase, edit("STEPS = a b", "STEPS = b a"), outside},
		{"assignment inside a conditional", edit("STEPS = a b", "ifdef CI\nSTEPS = a b\nendif"), edit("STEPS = a b", "ifdef CI\nSTEPS = a b c\nendif"), "changed"},
		{"target-specific assignment too", makeWordListBase + "a: STEPS = x\n", added + "a: STEPS = x\n", outside},
		{"unevaluated function", edit("$(STEPS) $(addprefix", "$(sort $(STEPS)) $(addprefix"), strings.Replace(added, "$(STEPS) $(addprefix", "$(sort $(STEPS)) $(addprefix", 1), outside},
		{"non-literal function argument", edit("$(STEPS) $(addprefix ledger/", "$(STEPS) $(addprefix $(DIR)/"), strings.Replace(added, "$(STEPS) $(addprefix ledger/", "$(STEPS) $(addprefix $(DIR)/", 1), outside},
		{"recipe use site", makeWordListBase + "show:\n\techo $(STEPS)\n", added + "show:\n\techo $(STEPS)\n", outside},
		{"inline recipe use site", makeWordListBase + "show: ; echo $(STEPS)\n", added + "show: ; echo $(STEPS)\n", outside},
		{"automatic variable of the prerequisites", edit("\techo done", "\techo $^"), strings.Replace(added, "\techo done", "\techo $^", 1), outside},
		{"another variable's value", makeWordListBase + "ALL = $(STEPS) z\nshow:\n\techo $(ALL)\n", added + "ALL = $(STEPS) z\nshow:\n\techo $(ALL)\n", outside},
		{"use before the assignment", "$(STEPS): | clear\n" + makeWordListBase, "$(STEPS): | clear\n" + added, outside},
		{"computed line with a recipe", edit("$(STEPS) $(addprefix ledger/,$(STEPS)): | clear", "$(STEPS) $(addprefix ledger/,$(STEPS)): | clear\n\techo step"), strings.Replace(added, "$(STEPS) $(addprefix ledger/,$(STEPS)): | clear", "$(STEPS) $(addprefix ledger/,$(STEPS)): | clear\n\techo step", 1), outside},
		{"computed line with computed prerequisites", edit("): | clear", "): | $(CLEAR)"), strings.Replace(added, "): | clear", "): | $(CLEAR)", 1), outside},
		{"special target use", makeWordListBase + ".PHONY: $(STEPS)\n", added + ".PHONY: $(STEPS)\n", outside},
		{"secondary expansion", ".SECONDEXPANSION:\n" + makeWordListBase, ".SECONDEXPANSION:\n" + added, outside},
		{"conditional use", makeWordListBase + "ifneq ($(filter c,$(STEPS)),)\nendif\n", added + "ifneq ($(filter c,$(STEPS)),)\nendif\n", outside},
		{"use in a conditional rule", strings.Replace(makeWordListBase, "all: clear $(addprefix ledger/,$(STEPS))\n\techo done\n", "ifdef CI\nall: clear $(addprefix ledger/,$(STEPS))\n\techo done\nendif\n", 1), strings.Replace(added, "all: clear $(addprefix ledger/,$(STEPS))\n\techo done\n", "ifdef CI\nall: clear $(addprefix ledger/,$(STEPS))\n\techo done\nendif\n", 1), outside},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AttributeMakeEdit("Makefile", []byte(tc.base), []byte(tc.head))
			if got.Reason == "" || got.Targets != nil {
				t.Fatalf("attributed to %v, want a refusal naming %q", got.Targets, tc.reason)
			}
			if !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("reason %q, want it to name %q", got.Reason, tc.reason)
			}
		})
	}
	// A recipe use with no computed line is no NCM-V1-015 use, so the edit
	// is NCM-V1-011's: every rule that uses the variable, whole.
	base := "STEPS = a\nall: $(STEPS)\nshow:\n\techo $(STEPS)\na:\n\tgo test ./a\n"
	head := strings.Replace(base, "STEPS = a", "STEPS = a c", 1) + "c:\n\tgo test ./c\n"
	if got := AttributeMakeEdit("Makefile", []byte(base), []byte(head)); got.Reason != "" || !slices.Equal(got.Targets, []string{"all", "c", "show"}) || got.Touched != nil {
		t.Fatalf("recipe use: targets %v touched %v reason %q, want all, c and show whole", got.Targets, got.Touched, got.Reason)
	}
}

// makeInstanceFile has a pattern rule a word-list edit reaches through
// ledger/<word>, as the repository's ledger/% is.
const makeInstanceFile = `STEPS = check
all: $(addprefix ledger/,$(STEPS))
ledger/%:
	go run ./ledger $* -- $(MAKE) $*
check:
	go test ./tool
lint:
	go vet ./core
twice/%:
	go run ./ledger $*
twice/%.x:
	go run ./core $*
needs/%: %.in
	go run ./ledger $*
`

// TestReachMakeEditInstance_NCMV1016 pins the reach through a pattern rule
// instance: ledger/check runs the ledger tool and, through its stem, check's
// tool; a target two pattern rules match, one whose pattern rule has
// prerequisites, or one nothing matches, is not in the driver units; and a
// workflow that invokes a Touched rule is recorded (NCM-V1-014).
func TestReachMakeEditInstance_NCMV1016(t *testing.T) {
	ctx := context.Background()
	workflow := "jobs:\n  unit:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make all\n" //corvint:fixture-path
	root := makeReachRepository(t, map[string]string{"Makefile": makeInstanceFile, ".github/workflows/ci.yml": workflow})
	reach := ReachMakeEdit(ctx, root, "Makefile", MakeEdit{Targets: []string{"ledger/lint"}, Touched: []string{"all"}})
	if reach.Reason != "" {
		t.Fatalf("ledger/lint not bounded: %s", reach.Reason)
	}
	for _, want := range []string{"ledger", "core"} {
		if !slices.Contains(reach.Paths, want) {
			t.Fatalf("paths %v lack %q", reach.Paths, want)
		}
	}
	if slices.Contains(reach.Paths, "tool") {
		t.Fatalf("paths %v hold check's tool, which ledger/lint does not run", reach.Paths)
	}
	if !slices.Equal(reach.Workflows, []string{".github/workflows/ci.yml"}) {
		t.Fatalf("workflows %v, want the one that invokes the touched all", reach.Workflows)
	}
	if reach := ReachMakeEdit(ctx, root, "Makefile", MakeEdit{Targets: []string{"ledger/lint"}}); reach.Reason != "" || len(reach.Workflows) != 0 {
		t.Fatalf("no touched rule: workflows %v reason %q", reach.Workflows, reach.Reason)
	}
	for _, tc := range []struct{ target, why string }{
		{"twice/a.x", "two pattern rules match"},
		{"needs/a", "its pattern rule has prerequisites"},
		{"nothing/a", "no rule matches"},
	} {
		if reach := ReachMakeEdit(ctx, root, "Makefile", MakeEdit{Targets: []string{tc.target}}); !strings.Contains(reach.Reason, "is not in the driver units") {
			t.Fatalf("%s (%s): reason %q, want it not in the driver units", tc.target, tc.why, reach.Reason)
		}
	}
	if reach := ReachMakeEdit(ctx, root, "Makefile", MakeEdit{Targets: []string{"twice/a"}}); reach.Reason != "" || !slices.Contains(reach.Paths, "ledger") {
		t.Fatalf("twice/a, which one pattern rule matches: paths %v reason %q", reach.Paths, reach.Reason)
	}
	// The pattern rule's own unit, which every word reaches without the
	// instance, imports every target: the generic ledger/% makes no bound.
	units, err := Driver{}.Units(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range units.Units {
		if unit.ID == "driver:make:Makefile#ledger/lint" {
			t.Fatalf("the planner's units hold the reach's instance %s", unit.ID)
		}
	}
}

// TestDriverEnvPrefix_NCMV1017 pins which environment assignments in front
// of a command word leave its run set resolved: a literal value of a name
// that does not choose or load the program. Every case stays computed for
// the planner (AFP-V0-3128).
func TestDriverEnvPrefix_NCMV1017(t *testing.T) {
	for _, tc := range []struct {
		command string
		open    bool
	}{
		{"HEAVY_WHOLE_REASON='make x of y' script/wrap.sh go test ./tool", true}, // NCM-V1-019: a script is not followed.
		{`A=1 B="two words" go test ./tool`, false},
		{"A= go test ./tool", false},
		{"A=$(date) go test ./tool", true},
		{"A=\"$(date)\" go test ./tool", true},
		{"A=`date` go test ./tool", true},
		{"A=$HOME go test ./tool", true},
		{"A=" + driverComputed + " go test ./tool", true},
		{"PATH=/opt/bin go test ./tool", true},
		{"path=/opt/bin go test ./tool", true},
		{"LD_PRELOAD=x.so go test ./tool", true},
		{"BASH_ENV=hook.sh script/run.sh", true},
		{"GOFLAGS=-toolexec=x go test ./tool", true},
		{"NODE_OPTIONS=--require=x node a.js", true},
		{"HOME=/tmp/h git status", true},
		{"env A=1 go test ./tool", true},
		{"A=1 nice go test ./tool", true},
		{"nice A=1 go test ./tool", true},
		{"A=1 $RUNNER ./tool", true},
	} {
		t.Run(tc.command, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(tc.command, ".", "example.test/m")
			if !scan.computed {
				t.Fatal("not computed: the planner must still keep the site")
			}
			if scan.runOpen != tc.open {
				t.Fatalf("run set open %v, want %v (reasons %v)", scan.runOpen, tc.open, scan.reasons)
			}
		})
	}
	ctx := context.Background()
	file := "check:\n\tHEAVY_WHOLE_REASON='x' go test ./tool\nsubst:\n\tA=$$(date) go test ./tool\n"
	root := makeReachRepository(t, map[string]string{"Makefile": file})
	if reach := ReachMakeTargets(ctx, root, "Makefile", []string{"check"}); reach.Reason != "" || !slices.Contains(reach.Paths, "tool") {
		t.Fatalf("check: paths %v reason %q, want it bounded to tool", reach.Paths, reach.Reason)
	}
	if reach := ReachMakeTargets(ctx, root, "Makefile", []string{"subst"}); !strings.Contains(reach.Reason, "does not resolve") {
		t.Fatalf("subst: reason %q, want it unresolved", reach.Reason)
	}
	units, err := Driver{}.Units(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range units.Units {
		if unit.ID == "driver:make:Makefile#check" && !slices.Contains(unit.Frontier, FrontierDriverComputedInvocation) {
			t.Fatalf("check frontier %v: the planner lost the computed site", unit.Frontier)
		}
	}
}
