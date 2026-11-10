// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestFrontierSiteEvaluationScope_AZU_V0_610 holds the evaluated-target
// hedge (AFP-V0-3122) to text whose expansion a target observes. A recipe
// that runs an uncertified repository script is a computed site only when
// its own text, or a parse-time expansion that ran a command, read a file or
// met an unmodeled function, was evaluated. A conditional's condition (every
// branch is an alternative whatever it says), an immediate value whose
// evaluation left no command to run, and a value no text expands mark no
// target: a dirty sealed CEM then selects nothing, while the script's literal
// operand still selects the target through its edge. The dynamic variants,
// where the evaluation decided a parse-time command or the recipe spells the
// evaluated function, keep every claim.
func TestFrontierSiteEvaluationScope_AZU_V0_610(t *testing.T) {
	const recipe = "check:\n\tscript/audit.sh tools/local/main.go\nall: check\n"
	for _, tc := range []struct {
		name, makefile string
		site           bool
	}{
		{"control", recipe, false},
		{"unreferenced-value", "CACHE = $(or $(XDG_CACHE_HOME),/tmp)\n" + recipe, false},
		{"conditional-condition", "ifneq ($(filter gate,$(or $(MAKECMDGOALS),gate)),)\nX = 1\nendif\n" + recipe, false},
		{"immediate-definite-false-branch", "BASE ?= origin/main\nFORWARD := $(if $(filter merge-forward,$(BASE)),$(shell script/lsfiles.sh))\n" + recipe, false},
		{"immediate-value-only", "MODE := $(if $(filter a,b),ci,local)\n" + recipe, false},
		{"prerequisite-of-other-target", "other: $(if $(filter a,b),tools/ci,tools/local)\n\techo ok\n" + recipe, false},
		{"immediate-environment-branch-runs-command", "FORWARD := $(if $(filter merge-forward,$(ENV_BASE)),$(shell script/lsfiles.sh))\n" + recipe, true},
		{"conditional-decides-command", "ifeq ($(shell $(if $(filter a,$(ENV_MODE)),script/lsfiles.sh,true)),)\nX = 1\nendif\n" + recipe, true},
		{"immediate-reads-file", "LIST := $(if $(filter a,a),$(wildcard tools/*/main.go))\n" + recipe, true},
		{"bang-command", "FILES != $(if $(filter a,a),script/lsfiles.sh)\n" + recipe, true},
		{"recipe-spells-function", "check:\n\t$(strip script/audit.sh tools/local/main.go)\nall: check\n", true},
		{"recipe-expands-evaluated-value", "MODE := $(if $(filter a,b),ci,local)\ncheck:\n\tscript/audit.sh tools/$(MODE)/main.go\nall: check\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(tc.makefile), nil)
			if got := computedSite(t, graph, "driver:make:Makefile#check"); got != tc.site {
				t.Fatalf("check computed site = %v, want %v", got, tc.site)
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			if tc.site {
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
				if witness := selected["driver:make:Makefile#check"].Witness; witness.Kind != affected.WitnessFrontierSite || witness.DirtyPath != sealedCEM {
					t.Errorf("check witness %+v; want FRONTIER_SITE on %s", witness, sealedCEM)
				}
			} else {
				wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
				for id, selection := range selected {
					t.Logf("CEM selects %s by %+v", id, selection.Witness)
				}
			}
			// The script's literal operand is the target's edge either way.
			selected = driverSelections(affected.Select(graph, []string{"tools/local/main.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
		})
	}
}

// TestFrontierSiteGoToolchainLocal_AZU_V0_612 certifies the one toolchain
// value whose effect is modeled: `GOTOOLCHAIN=local` forbids a toolchain
// switch and reads nothing, so an evaluated target that runs a bounded Go
// command under it is no site. Every other value, and a value the
// environment supplies, may download and run another toolchain and keeps
// the claim.
func TestFrontierSiteGoToolchainLocal_AZU_V0_612(t *testing.T) {
	for _, tc := range []struct {
		name, assignment string
		site             bool
	}{
		{"local", "GOTOOLCHAIN=local", false},
		{"local-with-cache", "GOCACHE=/tmp/cache GOTOOLCHAIN=local", false},
		{"auto", "GOTOOLCHAIN=auto", true},
		{"path", "GOTOOLCHAIN=path", true},
		{"version", "GOTOOLCHAIN=go1.27.3", true},
		{"local-suffixed", "GOTOOLCHAIN=local+auto", true},
		{"environment", "GOTOOLCHAIN=$(TOOLCHAIN)", true},
		{"empty", "GOTOOLCHAIN=", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(recipeEvaluated+"check:\n\t$(EVAL)"+tc.assignment+" go test ./tools/local\nall: check\n"), nil)
			if got := computedSite(t, graph, "driver:make:Makefile#check"); got != tc.site {
				t.Fatalf("check computed site = %v, want %v", got, tc.site)
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			if tc.site {
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			} else {
				wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
			}
			selected = driverSelections(affected.Select(graph, []string{"tools/local/main.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
		})
	}
}

// TestFrontierSiteLiteralMakeGoals_AZU_V0_612 pins the literal make shapes
// AFP-V0-3122 certifies for an evaluated target, literal goals and a literal
// `-C` directory, against their dynamic variants, which stay computed.
func TestFrontierSiteLiteralMakeGoals_AZU_V0_612(t *testing.T) {
	for _, tc := range []struct {
		name, call string
		site       bool
	}{
		{"literal-goal", "$(MAKE) other", false},
		{"literal-directory", "$(MAKE) -C . other", false},
		{"literal-directory-long", "$(MAKE) --directory=. other", false},
		{"dynamic-goal", "$(MAKE) $(GOAL)", true},
		{"dynamic-directory", "$(MAKE) -C $(DIR) other", true},
		{"variable-override", "$(MAKE) other PKG=ci", true},
		{"shell-variable-goal", "$(MAKE) $$GOAL", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, boundFiles(recipeEvaluated+"check:\n\t$(EVAL)"+tc.call+"\nother:\n\tgo test ./tools/local\nall: check\n"), nil)
			if got := computedSite(t, graph, "driver:make:Makefile#check"); got != tc.site {
				t.Fatalf("check computed site = %v, want %v", got, tc.site)
			}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			if tc.site {
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			} else {
				wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit})
			}
		})
	}
}
