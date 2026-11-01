package golang_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
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
// go.work does not list, which requires a listed one, stays a frontier rather
// than a mis-attributed unit: the frontier of a source-less module unit, since
// only its own manifest draws a listed module in (AFP-V0-582).
func TestWorkspaceModulesAreUnitsUnderTheirOwnModulePath(t *testing.T) {
	result, err := golang.New().Units(context.Background(), workspaceRoot(t))
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	units := make(map[string]affected.Unit, len(result.Units))
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	stray := units["go:example.com/workspace/stray"]
	if len(units) != 3 || len(stray.Sources) != 0 || len(stray.Tests) != 0 || !reflect.DeepEqual(stray.Frontier, []string{golang.FrontierNestedModule}) {
		t.Fatalf("units = %v, want exactly %s, %s and the stray module unit", result.Units, coreUnit, appUnit)
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
	wantFrontier := []string{golang.FrontierWorkspaceModuleOutsideRoot}
	if !reflect.DeepEqual(result.Frontier, wantFrontier) {
		t.Errorf("frontier = %v, want %v for ../outside", result.Frontier, wantFrontier)
	}
}

// TestWorkspaceDirtySourceSelectsTheOtherModulesTest is the selection the
// bench's etcd samples need: a dirty source in one workspace module selects
// the test in the other module that imports it.
func TestWorkspaceDirtySourceSelectsTheOtherModulesTest(t *testing.T) {
	graph, err := affected.Build(context.Background(), workspaceRoot(t), golang.New())
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
	// The stray module imports nothing listed, so the change cannot reach it
	// (AFP-V0-582).
	want := []affected.Unknown{
		{Reason: affected.UnknownLanguageFrontier, Detail: golang.FrontierWorkspaceModuleOutsideRoot, Hint: golang.WorkspaceOutsideRootRoute},
	}
	if !reflect.DeepEqual(plan.Unknown, want) {
		t.Errorf("unknown = %+v, want ../outside", plan.Unknown)
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
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Frontier, []string{golang.FrontierWorkspaceModuleOutsideRoot}) {
		t.Fatalf("frontier = %v, want %s", result.Frontier, golang.FrontierWorkspaceModuleOutsideRoot)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if plan := affected.Select(graph, []string{"app/app.go"}); plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope = %s, want %s while a used module is outside the root", plan.Scope, affected.ScopeUnknown)
	}
}

// AFP-V0-761 (V1-2051): the outside-root frontier entry names the supported
// cross-repository route in its Hint, for an explicit sibling change as for a
// root change, while its Detail, scope and selection stay as before.
func TestWorkspaceOutsideRootFrontierNamesTheRoute_AFPV0761(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	writeFiles(t, root, map[string]string{
		"go.work":         "go 1.22\n\nuse (\n\t./app\n\t../sibling\n)\n",
		"app/go.mod":      "module example.test/app\n",
		"app/app.go":      "package app\n\nimport _ \"example.test/sibling\"\n",
		"app/app_test.go": "package app\n",
	})
	writeFiles(t, parent, map[string]string{"sibling/go.mod": "module example.test/sibling\n", "sibling/s.go": "package sibling\n"})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range [][]string{{"app/app.go"}, {"go.work"}} {
		plan := affected.Select(graph, dirty)
		want := []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: golang.FrontierWorkspaceModuleOutsideRoot, Hint: golang.WorkspaceOutsideRootRoute}}
		var frontier []affected.Unknown
		for _, unknown := range plan.Unknown {
			if unknown.Reason == affected.UnknownLanguageFrontier {
				frontier = append(frontier, unknown)
			}
		}
		if !reflect.DeepEqual(frontier, want) || plan.Scope != affected.ScopeUnknown {
			t.Fatalf("dirty %v: frontier = %+v scope %s, want %+v UNKNOWN", dirty, frontier, plan.Scope, want)
		}
	}
	if !strings.Contains(golang.WorkspaceOutsideRootRoute, "corvint breakage") {
		t.Fatalf("route does not name corvint breakage: %s", golang.WorkspaceOutsideRootRoute)
	}
	if route := affected.FrontierRoute(golang.New()).FrontierRoute(golang.FrontierNestedModule); route != "" {
		t.Fatalf("nested-module frontier names a route %q, want none", route)
	}
}

// TestWorkspaceUseSetIsTheStrictReaders_AFPV0190 pins that the go.work use set
// is read by the strict manifest reader (V1-0874) on inputs the earlier
// line-and-fields reader read differently: a quoted directory with a space is
// observed (the earlier reader skipped the line and observed the root alone),
// and a go.work the strict reader refuses (the earlier reader guessed ./app)
// observes the root alone and raises go:workspace-unparsed.
func TestWorkspaceUseSetIsTheStrictReaders_AFPV0190(t *testing.T) {
	refused := []string{golang.FrontierModulePath, golang.FrontierNestedModule, golang.FrontierWorkspaceUnparsed}
	for _, test := range []struct {
		name, work, directory string
		wantUnits             []string
		wantFrontier          []string
	}{
		{"quoted directory with a space", "go 1.26\n\nuse \"./my app\"\n", "my app", []string{"go:example.test/app"}, []string{}},
		{"raw string", "go 1.26\n\nuse `./app`\n", "app", []string{}, refused},
		{"two directories on one use line", "go 1.26\n\nuse ./app ./other\n", "app", []string{}, refused},
		{"unknown directive", "go 1.26\n\nuse ./app\nmodule example.test/root\n", "app", []string{}, refused},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.work":                       test.work,
				test.directory + "/go.mod":      "module example.test/app\n",
				test.directory + "/app.go":      "package app\n",
				test.directory + "/app_test.go": "package app\n",
			})
			result, err := golang.New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			units := []string{}
			for _, unit := range result.Units {
				units = append(units, unit.ID)
			}
			frontier := append([]string{}, result.Frontier...)
			if !reflect.DeepEqual(units, test.wantUnits) || !reflect.DeepEqual(frontier, test.wantFrontier) {
				t.Errorf("units = %v frontier = %v, want %v and %v", units, frontier, test.wantUnits, test.wantFrontier)
			}
		})
	}
}
