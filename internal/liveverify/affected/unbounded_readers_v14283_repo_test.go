// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// TestRepositoryScriptsAreBoundedSources_V14283 checks this repository's
// scripts and commands against AFP-V0-2951 to AFP-V0-2955 as the review fix
// left them. AFP-V0-3340/3341 now bounds heavy.sh's host-only environment
// source; mutating its realpath guard must restore dynamic-source protection.
// dogfood-finish.sh links its same-valued `$repo` source (AFP-V0-2954);
// qualify-tasks-basic_test.sh is read-scoped and dogfood-cache_test.sh, which
// reaches heavy.sh, keeps its own toolchain floor (AFP-V0-3342); the
// commands whose os.Getwd escapes locate the root for the units that run them
// (AFP-V0-2951); and literals with no output-only evidence keep rule (d)
// (AFP-V0-2952). Reverting any one fix fails the matching assertion.
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
	if unit := byID["shell:script/heavy.sh"]; unit.UnboundedReads != "" || unit.LocatesRoot {
		t.Errorf("heavy.sh: the checked host-only source is unbounded: %q locatesRoot=%v", unit.UnboundedReads, unit.LocatesRoot)
	}
	body, err := os.ReadFile(filepath.Join(root, "script", "heavy.sh"))
	if err != nil {
		t.Fatal(err)
	}
	const environmentSource = `realpath(shift @ARGV)`
	if strings.Count(string(body), environmentSource) != 1 {
		t.Fatalf("heavy.sh: want exactly one environment-named source line %q", environmentSource)
	}
	copied := t.TempDir()
	if err := os.MkdirAll(filepath.Join(copied, "script"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copied, "script", "heavy.sh"), []byte(strings.Replace(string(body), environmentSource, "shift @ARGV", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	copyUnits, err := shell.New().Units(context.Background(), copied)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range copyUnits.Units {
		if unit.ID == "shell:script/heavy.sh" && (unit.UnboundedReads != shell.UnboundedDynamicSource || !unit.LocatesRoot) {
			t.Errorf("heavy.sh without its realpath guard: dynamic source protection lost: %q locatesRoot=%v", unit.UnboundedReads, unit.LocatesRoot)
		}
	}
	if unit := byID["shell:script/dogfood-finish.sh"]; unit.UnboundedReads != "" || unit.LocatesRoot || !slices.Contains(unit.Imports, "shell:script/dogfood-build-corvint.sh") {
		t.Errorf("dogfood-finish.sh: the same-valued $repo source is not linked or a dynamic source is back: %q %v", unit.UnboundedReads, unit.Imports)
	}
	if unit := byID["shell:script/qualify-tasks-basic_test.sh"]; !unit.ReadScoped || unit.UnboundedReads != "" {
		t.Errorf("qualify-tasks-basic_test.sh: not read-scoped: %+v", unit)
	}
	// AFP-V0-3342: heavy.sh is now bounded, but dogfood-cache_test.sh still
	// runs a toolchain and has no read-scope declaration of its own.
	if unit := byID["shell:script/dogfood-cache_test.sh"]; unit.ReadScoped || unit.UnboundedReads != shell.UnboundedRunsToolchain {
		t.Errorf("dogfood-cache_test.sh: its own toolchain floor changed: %+v", unit)
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
		if !unit.LocatesRoot || !strings.Contains(unit.UnboundedReads, "os.Getwd") {
			t.Errorf("%s: a command's escaping os.Getwd no longer reaches the units that run it: locatesRoot=%v (%q)", command, unit.LocatesRoot, unit.UnboundedReads)
		}
	}
	// The Ruby source perfbudget formats with fmt.Appendf and the Markdown
	// table cem-stable-regen builds in a strings.Builder reach no standard
	// stream, so their climbing tokens stay paths (AFP-V0-2952).
	if unit := byID[module+"internal/perfbudget"]; unit.UnboundedReads == "" {
		t.Errorf("perfbudget: formatted text with a climbing token lost rule (d)")
	}
	if unit := byID[module+"tools/cem-stable-regen"]; unit.UnboundedReads == "" || !unit.LocatesRoot {
		t.Errorf("cem-stable-regen: buffered text with a climbing token lost rule (d): %q locatesRoot=%v", unit.UnboundedReads, unit.LocatesRoot)
	}
	if unit := byID[module+"internal/jstestprovider"]; !unit.LocatesRoot {
		t.Errorf("jstestprovider: a library that runs git over a located root must still locate it: %+v", unit.UnboundedReads)
	}
}
