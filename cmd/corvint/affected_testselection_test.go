// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// testSelectionFixture is a committed Go module: core, its importer mid, a
// contract package whose test reads docs/contract.json at run time, and a
// smoke package nothing imports. declaration, when not empty, is committed as
// the project's test-selection declaration.
func testSelectionFixture(t *testing.T, declaration string) (root, base string) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		"go.mod":                        "module example.com/owned\n\ngo 1.27.0\n",
		"docs/contract.json":            "{}\n",
		"x/a/core/core.go":              "package core\n\nfunc Value() int { return 1 }\n",
		"x/a/core/core_test.go":         "package core\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"x/b/mid/mid.go":                "package mid\n\nimport \"example.com/owned/x/a/core\"\n\nfunc Value() int { return core.Value() }\n",
		"x/b/mid/mid_test.go":           "package mid\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"x/d/contract/contract.go":      "package contract\n\nfunc Name() string { return \"contract\" }\n",
		"x/d/contract/contract_test.go": "package contract\n\nimport \"testing\"\n\nfunc TestName(t *testing.T) { _ = Name() }\n",
		"x/c/smoke/smoke.go":            "package smoke\n\nfunc Ready() bool { return true }\n",
		"x/c/smoke/smoke_test.go":       "package smoke\n\nimport \"testing\"\n\nfunc TestReady(t *testing.T) { _ = Ready() }\n",
		"x/c/smoke/smoke_extra_test.go": "package smoke\n\nimport \"testing\"\n\nfunc TestExtra(t *testing.T) { _ = Ready() }\n",
		affected.DeclaredSelectionPath:  declaration,
	}
	if declaration == "" {
		delete(files, affected.DeclaredSelectionPath)
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	return root, strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
}

const ownedAndMustRun = `{"profile":"corvint-test-selection/0",` +
	`"owners":[{"paths":"docs/contract.json","tests":["x/d/contract/"]}],` +
	`"mustRun":[{"tests":"x/c/smoke/","reason":"release smoke suite"}]}`

// AFP-V0-2485: a unit a declared owner or a must-run entry selects is tier 0,
// and raises no neighbourhood; a path-literal reader stays tier 2.
func TestMustRunAndOwnerAreTierZero_AFPV02485(t *testing.T) {
	t.Parallel()
	root, _ := testSelectionFixture(t, ownedAndMustRun)
	appendFile(t, filepath.Join(root, "x", "a", "core", "core.go"), "\n// edited\n")
	appendFile(t, filepath.Join(root, "docs", "contract.json"), "\n")
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	tiers, _ := receipt["tiers"].(map[string]any)
	want := map[string]any{
		"go:example.com/owned/x/a/core":     float64(0),
		"go:example.com/owned/x/b/mid":      float64(1),
		"go:example.com/owned/x/d/contract": float64(0),
		"go:example.com/owned/x/c/smoke":    float64(0),
	}
	if !reflect.DeepEqual(tiers["units"], want) {
		t.Fatalf("tiers.units = %v, want %v", tiers["units"], want)
	}
	plan := receipt["plan"].(map[string]any)
	kinds := map[string]string{}
	for _, raw := range plan["selected"].([]any) {
		selection := raw.(map[string]any)
		kinds[selection["unitId"].(string)] = selection["witness"].(map[string]any)["kind"].(string)
	}
	if kinds["go:example.com/owned/x/d/contract"] != affected.WitnessDeclaredTestOwner || kinds["go:example.com/owned/x/c/smoke"] != affected.WitnessMustRun {
		t.Fatalf("witness kinds %v", kinds)
	}
	mustRun, _ := plan["mustRun"].([]any)
	if len(mustRun) != 1 || mustRun[0].(map[string]any)["unitId"] != "go:example.com/owned/x/c/smoke" {
		t.Fatalf("plan.mustRun = %v", plan["mustRun"])
	}
	wantOwners := []any{map[string]any{"unitId": "go:example.com/owned/x/d/contract", "paths": "docs/contract.json", "dirtyPath": "docs/contract.json"}}
	if !reflect.DeepEqual(plan["declaredOwners"], wantOwners) {
		t.Fatalf("plan.declaredOwners = %v", plan["declaredOwners"])
	}

	// Negative control: the same tiering keeps a path reader at tier 2, and a
	// declared unit beside a deep change gains no neighbour.
	got := tierAffectedSelections([]affected.Selection{
		{UnitID: "go:x/a/b/owned", Witness: affected.Witness{Kind: affected.WitnessDeclaredTestOwner, DirtyPath: "x/a/b/c/contract.json"}},
		{UnitID: "go:x/a/b/sibling", Witness: affected.Witness{Kind: affected.WitnessPathLiteralReader, DirtyPath: "x/a/b/c/contract.json"}},
		{UnitID: "go:x/smoke", Witness: affected.Witness{Kind: affected.WitnessMustRun, DirtyPath: "x/a/b/c/contract.json"}},
	})
	if want := map[string]int{"go:x/a/b/owned": 0, "go:x/a/b/sibling": 2, "go:x/smoke": 0}; !reflect.DeepEqual(got.Units, want) {
		t.Fatalf("tiers %v, want %v", got.Units, want)
	}
}

// AFP-V0-2489: a unit the plan names in mustRun or declaredOwners is tier 0
// whatever witness reached it first; an undeclared reader stays tier 2, and a
// declared unit reached by its own source still raises its neighbourhood.
func TestDeclaredUnitsAreTierZeroWhateverReachedThem_AFPV02489(t *testing.T) {
	t.Parallel()
	const dirty = "x/a/b/c/contract.json"
	selected := []affected.Selection{
		{UnitID: "go:x/owned", Witness: affected.Witness{Kind: affected.WitnessPathLiteralReader, DirtyPath: dirty}},
		{UnitID: "go:x/smoke", Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: dirty, Via: []string{"go:x/a/b/c", "go:x/m", "go:x/n", "go:x/smoke"}}},
		{UnitID: "go:x/reader", Witness: affected.Witness{Kind: affected.WitnessPathLiteralReader, DirtyPath: dirty}},
		{UnitID: "go:x/a/b/c", Witness: affected.Witness{Kind: affected.WitnessDirectSource, DirtyPath: "x/a/b/c/c.go"}},
		{UnitID: "go:x/a/b/sibling", Witness: affected.Witness{Kind: affected.WitnessPathLiteralReader, DirtyPath: dirty}},
	}
	plan := affected.Plan{
		Selected:       selected,
		MustRun:        []affected.MustRun{{UnitID: "go:x/smoke", Tests: "x/smoke/", Reason: "release smoke suite"}},
		DeclaredOwners: []affected.DeclaredOwner{{UnitID: "go:x/owned", Paths: "x/**/*.json", DirtyPath: dirty}, {UnitID: "go:x/a/b/c", Paths: "x/**/*.json", DirtyPath: dirty}},
	}
	want := map[string]int{"go:x/owned": 0, "go:x/smoke": 0, "go:x/reader": 2, "go:x/a/b/c": 0, "go:x/a/b/sibling": 1}
	if got := tierAffectedPlan(plan); !reflect.DeepEqual(got.Units, want) || got.Summary != "tier0=3 tier1=1 tier2=1" {
		t.Fatalf("tiers %v %q, want %v", got.Units, got.Summary, want)
	}
	// Negative control: the same selection without the declared members
	// ranks by witness alone.
	want = map[string]int{"go:x/owned": 2, "go:x/smoke": 2, "go:x/reader": 2, "go:x/a/b/c": 0, "go:x/a/b/sibling": 1}
	if got := tierAffectedSelections(selected); !reflect.DeepEqual(got.Units, want) {
		t.Fatalf("undeclared tiers %v, want %v", got.Units, want)
	}
}

// AFP-V0-2485: a must-run unit whose test file changed runs whole, naming the
// declared reason, where it would otherwise run only the changed test file's
// functions; its reason code is reached-via-must-run.
func TestMustRunUnitRunsWhole_AFPV02485(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, declaration, reason, code string
	}{
		{"declared", ownedAndMustRun, "whole: MUST_RUN (release smoke suite)", sessiontest.ReasonReachedPrefix + "must-run"},
		// Negative control: without the declaration the change slices.
		{"undeclared", "", "selected: the tests of x/c/smoke/smoke_test.go", sessiontest.ReasonSelectedChangedTests},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root, base := testSelectionFixture(t, test.declaration)
			appendFile(t, filepath.Join(root, "x", "c", "smoke", "smoke_test.go"), "\n// edited\n")
			receipt, graph, err := compileAffectedGraph(context.Background(), affectedInvocation{Root: root, Base: base})
			if err != nil {
				t.Fatal(err)
			}
			slice := testGoSlices(context.Background(), receipt.Plan, graph, root, base)["example.com/owned/x/c/smoke"]
			if slice.Reason != test.reason {
				t.Fatalf("smoke slice %+v, want reason %q", slice, test.reason)
			}
			if test.declaration != "" && slice.Pattern != "" {
				t.Fatalf("a must-run unit was narrowed to -run %q", slice.Pattern)
			}
			if got := sessiontest.ReasonCode(slice.Reason); got != test.code {
				t.Fatalf("ReasonCode(%q) = %q, want %q", slice.Reason, got, test.code)
			}
		})
	}
}
