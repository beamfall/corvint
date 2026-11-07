// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// TestRepositoryScriptsAreBoundedSources_V14283 checks this repository's
// scripts and commands against the rules of AFP-V0-2951 to AFP-V0-2955: the
// two shell scripts whose computed sources once made every unit that ran them
// an unbounded reader are bounded, the two toolchain tests the lane declared
// are read-scoped, and the commands whose os.Getwd once reached their runners
// keep their own reason without locating the root for anyone else. Reverting
// any one fix (the heavy.sh variable renames, the lexer's test-expression
// split, the constructed-name short-circuit, the environment-named source,
// the same-valued reassignment, or the command guard) fails the matching
// assertion.
func TestRepositoryScriptsAreBoundedSources_V14283(t *testing.T) {
	root := repositoryRoot(t)
	shellUnits, err := shell.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(shellUnits.Frontier, func(reason string) bool { return strings.HasPrefix(reason, shell.FrontierShellReadScopesInvalid) }) {
		t.Fatalf("the plugin refused %s: frontier %v", affected.ShellReadScopesPath, shellUnits.Frontier)
	}
	byID := map[string]affected.Unit{}
	for _, unit := range shellUnits.Units {
		byID[unit.ID] = unit
	}
	for _, script := range []string{"script/heavy.sh", "script/dogfood-finish.sh"} {
		unit, ok := byID["shell:"+script]
		if !ok {
			t.Fatalf("%s: no shell unit", script)
		}
		if unit.UnboundedReads != "" || unit.LocatesRoot {
			t.Errorf("%s: a dynamic source is back: %q locatesRoot=%v", script, unit.UnboundedReads, unit.LocatesRoot)
		}
	}
	if unit := byID["shell:script/dogfood-finish.sh"]; !slices.Contains(unit.Imports, "shell:script/dogfood-build-corvint.sh") {
		t.Errorf("dogfood-finish.sh: the same-valued $repo source is not linked: %v", unit.Imports)
	}
	for _, test := range []string{"script/dogfood-cache_test.sh", "script/qualify-tasks-basic_test.sh"} {
		if unit := byID["shell:"+test]; !unit.ReadScoped || unit.UnboundedReads != "" {
			t.Errorf("%s: not read-scoped: %+v", test, unit)
		}
	}
	// lane-scripts_test.sh stays on the floor with its recorded reason: it
	// sources a sed-filtered copy of heavy.sh from its temporary directory.
	if unit := byID["shell:script/lane-scripts_test.sh"]; unit.UnboundedReads != shell.UnboundedRunsToolchain || unit.ReadScoped {
		t.Errorf("lane-scripts_test.sh: want the recorded runs-toolchain floor, got %+v", unit)
	}

	goUnits, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range goUnits.Units {
		byID[unit.ID] = unit
	}
	const module = "go:github.com/Beamfall/corvint/"
	for _, command := range []string{"cmd/corvint", "benchmarks/runner", "cmd/corvint-tasks", "tools/gate-ledger", "tools/test-commands-run"} {
		unit, ok := byID[module+command]
		if !ok {
			t.Fatalf("%s: no Go unit", command)
		}
		if !unit.Command || unit.LocatesRoot {
			t.Errorf("%s: a command locates the root for its runners: command=%v locatesRoot=%v (%q)", command, unit.Command, unit.LocatesRoot, unit.UnboundedReads)
		}
		if !strings.Contains(unit.UnboundedReads, "os.Getwd") {
			t.Errorf("%s: the command's own reason was dropped: %q", command, unit.UnboundedReads)
		}
	}
	// The Markdown table main.go prints and the Ruby source perfbudget writes
	// are text (AFP-V0-2952); cem-stable-regen keeps a single-line climbing
	// path of its own (pointers.go) but, as a command, locates the root for
	// no runner.
	if unit := byID[module+"internal/perfbudget"]; strings.Contains(unit.UnboundedReads, "climbs from the working directory") || unit.LocatesRoot {
		t.Errorf("perfbudget: a line-broken text literal reads as a climbing path again: %q", unit.UnboundedReads)
	}
	if unit := byID[module+"tools/cem-stable-regen"]; strings.Contains(unit.UnboundedReads, "main.go carries the literal") || unit.LocatesRoot || !unit.Command {
		t.Errorf("cem-stable-regen: the printed table reads as a climbing path again, or the command locates the root: %q locatesRoot=%v", unit.UnboundedReads, unit.LocatesRoot)
	}
	if unit := byID[module+"internal/jstestprovider"]; !unit.LocatesRoot {
		t.Errorf("jstestprovider: a library that runs git over a located root must still locate it: %+v", unit.UnboundedReads)
	}
}
