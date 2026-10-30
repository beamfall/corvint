package affected_test

import (
	"context"
	json "encoding/json/v2"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// pr1961Source is the non-.github part of PR #1961's path set
// (07f43929f9e9..0d1329160835): a sealed lane CEM, a build-log entry, the
// governing spec and one Go test file.
var pr1961Source = []string{
	".corvint/changes/ec75c04dacd04534fc29c869504348248e88588a.cem.json",
	"docs/build-log/2026-10-28-v1-3051-tested-tree-record-per-shard-full.md",
	"docs/specs/affected-plan-v0.md",
	"tools/ci-reuse-plan/record_workflow_test.go",
}

// gateRun returns the packages of a gate selector verdict line, failing on any
// other verdict.
func gateRun(t *testing.T, lines []string) []string {
	t.Helper()
	if len(lines) == 0 {
		t.Fatal("the gate selector printed nothing")
	}
	verdict, packages, _ := strings.Cut(lines[len(lines)-1], " ")
	if verdict != "run" {
		t.Fatalf("gate selector verdict %q", lines[len(lines)-1])
	}
	return strings.Fields(packages)
}

// TestDriverSelectionCoversTheShadow_V13067 replays PR #1961's source paths
// through one version of the planner and both gate selector entry points
// (AFP-V0-603). The CI deciding driver runs gateselect.Select (through
// tools/gate-affected-select); the `corvint test` shadow narrows
// gateselect.SelectStructural of the same plan. Both start from the plan's Go
// selection and Select adds only its own rule (d) closure, so at one version
// the driver selects every package the shadow can. benchmarks/runner, one of
// the twelve packages CI reported as omitted, is selected by neither: its
// tokens do not resolve to the sealed CEM, so AFP-V0-152 narrows it out of rule
// (c). The CI omission came from a shadow pin that predates AFP-V0-152.
func TestDriverSelectionCoversTheShadow_V13067(t *testing.T) {
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	module, err := golang.ReadModulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, pr1961Source)
	var packages []string
	for _, selection := range plan.Selected {
		if pkg, ok := strings.CutPrefix(selection.UnitID, "go:"); ok {
			packages = append(packages, pkg)
		}
	}
	encoded, err := json.Marshal(map[string]any{
		"plan":     plan,
		"provider": map[string]any{"go": map[string]any{"packages": packages, "state": "RUNNABLE"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var receipt gateselect.Receipt
	if err := json.Unmarshal(encoded, &receipt); err != nil {
		t.Fatal(err)
	}
	driverLines := gateselect.Select(receipt, module, root)
	driver := gateRun(t, driverLines)
	shadow := gateRun(t, gateselect.SelectStructural(receipt, module, root))
	for _, pkg := range shadow {
		if !slices.Contains(driver, pkg) {
			t.Errorf("the driver omits %s, which the shadow selects", pkg)
		}
	}
	runner := module + "/benchmarks/runner"
	if slices.Contains(driver, runner) || slices.Contains(shadow, runner) {
		t.Errorf("%s is selected; AFP-V0-152 should narrow it out of the sealed CEM's rule (c) readers", runner)
	}
	for _, line := range driverLines {
		if strings.Contains(line, " "+runner+" ") || strings.HasSuffix(line, " "+runner) {
			t.Errorf("gate selector line names %s: %s", runner, line)
		}
	}
	if len(shadow) == 0 || len(driver) < len(shadow) {
		t.Fatalf("driver %d packages, shadow %d", len(driver), len(shadow))
	}
}
