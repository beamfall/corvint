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
	if len(app.Imports) != 0 || !reflect.DeepEqual(app.TestImports, []string{coreUnit}) {
		t.Errorf("app imports = %v testImports = %v, want the test-only cross-module edge to %s", app.Imports, app.TestImports, coreUnit)
	}
	if len(core.Imports) != 0 {
		t.Errorf("core imports = %v, want none", core.Imports)
	}
	wantFrontier := []string{golang.FrontierNestedModule, golang.FrontierWorkspaceModuleOutsideRoot}
	if !reflect.DeepEqual(result.Frontier, wantFrontier) {
		t.Errorf("frontier = %v, want %v for the unlisted stray module and ../outside", result.Frontier, wantFrontier)
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
	want := []affected.Unknown{
		{Reason: affected.UnknownLanguageFrontier, Detail: golang.FrontierNestedModule},
		{Reason: affected.UnknownLanguageFrontier, Detail: golang.FrontierWorkspaceModuleOutsideRoot},
	}
	if !reflect.DeepEqual(plan.Unknown, want) {
		t.Errorf("unknown = %+v, want the unlisted stray module and ../outside", plan.Unknown)
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

// V1-0327: a go.work use directive naming a sibling outside the root cannot be
// observed. Dropping it must raise a frontier, so a plan whose edges into that
// module are missing is UNKNOWN rather than silently bounded.
func TestWorkspaceUseOutsideRootIsAFrontier_AFPV0008(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	writeFiles(t, root, map[string]string{
		"go.work":         "go 1.22\n\nuse (\n\t./app\n\t../sibling\n)\n",
		"app/go.mod":      "module example.test/app\n",
		"app/app.go":      "package app\n\nimport _ \"example.test/sibling\"\n",
		"app/app_test.go": "package app\n",
	})
	writeFiles(t, parent, map[string]string{"sibling/go.mod": "module example.test/sibling\n", "sibling/s.go": "package sibling\n"})
	result, err := golang.New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Frontier, []string{golang.FrontierWorkspaceModuleOutsideRoot}) {
		t.Fatalf("frontier = %v, want %s", result.Frontier, golang.FrontierWorkspaceModuleOutsideRoot)
	}
	graph, err := affected.Build(root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if plan := affected.Select(graph, []string{"app/app.go"}); plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope = %s, want %s while a used module is outside the root", plan.Scope, affected.ScopeUnknown)
	}
}
