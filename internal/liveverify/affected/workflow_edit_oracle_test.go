// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// V1-5184 (AZU-V0-001, AZU-V0-210, AZU-V0-214): a workflow-only edit selects
// the Go units whose literal names the workflow file, the workflow's own
// jobs and the driver sites every plan selects, nothing else, with scope
// BOUNDED. The exact selection is pinned so a frontier cascade on a CI-only
// edit fails this test.
func TestWorkflowOnlyEditSelection_V1_5184(t *testing.T) {
	files := map[string]string{
		".github/workflows/lint.yml":  "name: lint\non: [push]\njobs:\n  check:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go vet ./core/...\n  privilege:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go test ./tools/wfcheck/...\n",
		"tools/wfcheck/check.go":      "package wfcheck\n\nconst Workflow = \".github/workflows/lint.yml\"\n",
		"tools/wfcheck/check_test.go": "package wfcheck\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestWorkflow(t *testing.T) {\n\tif _, err := os.ReadFile(\"../../\" + Workflow); err != nil {\n\t\tt.Skip()\n\t}\n}\n",
		"other/other.go":              "package other\n\nfunc Other() int { return 2 }\n",
		"other/other_test.go":         "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) { _ = Other() }\n",
	}
	graph := driverGraph(t, files, nil)
	plan := affected.Select(graph, []string{".github/workflows/lint.yml"})
	if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
		t.Fatalf("scope %s unknown %+v; want BOUNDED with none", plan.Scope, plan.Unknown)
	}
	got := map[string]string{}
	for _, selection := range plan.Selected {
		got[selection.UnitID] = selection.Witness.Kind
	}
	want := map[string]string{
		"driver:ci:.github/workflows/lint.yml#check":     affected.WitnessDirectSource,
		"driver:ci:.github/workflows/lint.yml#privilege": affected.WitnessDirectSource,
		"go:example.test/m/tools/wfcheck":                affected.WitnessPathLiteralReader,
		// The default fixture's computed-invocation job is selected on every
		// plan with a dirty path (AZU-V0-001, driverSites).
		"driver:ci:.github/workflows/ci.yml#dyn": affected.WitnessFrontierSite,
	}
	for id, kind := range want {
		if got[id] != kind {
			t.Errorf("%s: witness %q, want %s", id, got[id], kind)
		}
	}
	for id := range got {
		if _, expected := want[id]; !expected {
			t.Errorf("unexpected selection %s (%s)", id, got[id])
		}
	}
	if len(got) != len(want) {
		t.Errorf("selected %d units, want %d: %v", len(got), len(want), slices.Sorted(func(yield func(string) bool) {
			for id := range got {
				if !yield(id) {
					return
				}
			}
		}))
	}
}
