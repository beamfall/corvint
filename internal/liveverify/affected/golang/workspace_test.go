package golang_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const (
	coreUnit = "go:example.com/workspace/core/lib"
	appUnit  = "go:example.com/workspace/app/cmd"
)

// TestWorkspaceModulesAreUnitsUnderTheirOwnModulePath covers a go.work tree:
// every listed module is walked under its own module path, a test in one
// module importing a package in another is a reverse edge, and the module
// go.work does not list stays a frontier rather than a mis-attributed unit.
func TestWorkspaceModulesAreUnitsUnderTheirOwnModulePath(t *testing.T) {
	result, err := golang.New().Units(workspaceRoot(t))
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	units := make(map[string]affected.Unit, len(result.Units))
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	if len(units) != 2 {
		t.Fatalf("units = %v, want exactly %s and %s", result.Units, coreUnit, appUnit)
	}
	core := units[coreUnit]
	if !reflect.DeepEqual(core.Sources, []string{"core/lib/lib.go"}) || !reflect.DeepEqual(core.Tests, []string{"core/lib/lib_test.go"}) {
		t.Errorf("core unit = %+v", core)
	}
	app := units[appUnit]
	if !reflect.DeepEqual(app.Sources, []string{"app/cmd/cmd.go"}) || !reflect.DeepEqual(app.Tests, []string{"app/cmd/cmd_test.go"}) {
		t.Errorf("app unit = %+v", app)
	}
	if !reflect.DeepEqual(app.Imports, []string{coreUnit}) {
		t.Errorf("app imports = %v, want the cross-module edge to %s", app.Imports, coreUnit)
	}
	if len(core.Imports) != 0 {
		t.Errorf("core imports = %v, want none", core.Imports)
	}
	if !reflect.DeepEqual(result.Frontier, []string{golang.FrontierNestedModule}) {
		t.Errorf("frontier = %v, want only %s for the unlisted stray module", result.Frontier, golang.FrontierNestedModule)
	}
}

// TestWorkspaceDirtySourceSelectsTheOtherModulesTest is the selection the
// bench's etcd samples need: a dirty source in one workspace module selects
// the test in the other module that imports it.
func TestWorkspaceDirtySourceSelectsTheOtherModulesTest(t *testing.T) {
	graph, err := affected.Build(workspaceRoot(t), golang.New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	plan := affected.Select(graph, []string{"core/lib/lib.go"})
	selected := make([]string, 0, len(plan.Selected))
	for _, selection := range plan.Selected {
		selected = append(selected, selection.UnitID)
	}
	if !reflect.DeepEqual(selected, []string{coreUnit, appUnit}) {
		t.Fatalf("selected = %v, want %s (direct) before %s (dependent)", selected, coreUnit, appUnit)
	}
	if !reflect.DeepEqual(plan.SelectedTests(), []string{"app/cmd/cmd_test.go", "core/lib/lib_test.go"}) {
		t.Errorf("selected tests = %v", plan.SelectedTests())
	}
	if plan.Selected[0].Witness.DirtyPath != "core/lib/lib.go" {
		t.Errorf("cross-module witness = %+v", plan.Selected[0].Witness)
	}
	want := []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: golang.FrontierNestedModule}}
	if !reflect.DeepEqual(plan.Unknown, want) {
		t.Errorf("unknown = %+v, want only the unlisted stray module", plan.Unknown)
	}
	if plan.Scope != affected.ScopeUnknown {
		t.Errorf("scope = %s, want %s while a module is unobserved", plan.Scope, affected.ScopeUnknown)
	}
}

func workspaceRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
