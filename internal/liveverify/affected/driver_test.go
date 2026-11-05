package affected_test

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const driverMakefile = `GO ?= go

test:
	$(GO) test ./core/...

lint:
	go vet ./core/...

all: test lint

old:
	echo old
`

const driverWorkflow = `name: ci
on: [push]
env:
  MODE: "1"
jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./core/...
  build:
    needs: unit
    runs-on: ubuntu-latest
    steps:
      - run: make lint
  dyn:
    runs-on: ubuntu-latest
    steps:
      - run: '"$TOOL" --version'
`

// driverGraph builds a repository holding one Go package, a Makefile and a
// workflow with the head contents given, then attaches each base revision.
func driverGraph(t *testing.T, head map[string]string, base map[string]string) *affected.Graph {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                   "module example.test/m\n\ngo 1.27.0\n",
		"core/core.go":             "package core\n\nfunc Value() int { return 1 }\n",
		"core/core_test.go":        "package core\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"Makefile":                 driverMakefile,
		".github/workflows/ci.yml": driverWorkflow,
	}
	for name, body := range head {
		files[name] = body
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
	graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
	if err != nil {
		t.Fatal(err)
	}
	if base == nil {
		return graph
	}
	revisions := map[string]affected.DriverRevision{}
	for name, body := range base {
		revisions[name] = affected.DriverRevision{Base: []byte(body), Head: []byte(files[name]), BaseExists: true, HeadExists: true}
	}
	return graph.WithDriverContent(revisions)
}

// driverSelected maps each driver selection of plan by unit ID, and fails on
// any unknown entry: a driver input is never unowned.
func driverSelected(t *testing.T, plan affected.Plan) map[string]affected.Selection {
	t.Helper()
	if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
		t.Fatalf("scope %s, unknown %v; want BOUNDED with none", plan.Scope, plan.Unknown)
	}
	selected := map[string]affected.Selection{}
	for _, selection := range plan.Selected {
		if strings.HasPrefix(selection.UnitID, affected.DriverNamespace+":") {
			selected[selection.UnitID] = selection
		}
	}
	return selected
}

const (
	makeTest = "driver:make:Makefile#test"
	makeLint = "driver:make:Makefile#lint"
	makeAll  = "driver:make:Makefile#all"
	makeOld  = "driver:make:Makefile#old"
	jobUnit  = "driver:ci:.github/workflows/ci.yml#unit"
	jobBuild = "driver:ci:.github/workflows/ci.yml#build"
	jobDyn   = "driver:ci:.github/workflows/ci.yml#dyn"
)

func wantDriverSelection(t *testing.T, selected map[string]affected.Selection, want, absent []string) {
	t.Helper()
	for _, id := range want {
		if _, hit := selected[id]; !hit {
			t.Errorf("%s not selected; selected %v", id, slices.Sorted(maps.Keys(selected)))
		}
	}
	for _, id := range absent {
		if selection, hit := selected[id]; hit {
			t.Errorf("%s selected by %s; want it left out", id, selection.Witness.Kind)
		}
	}
}

func TestDriverClaimantOwnsDriverInputs_AZU_V0_001(t *testing.T) {
	driver := affected.Driver{}
	for _, path := range []string{"Makefile", "sub/GNUmakefile", "build/rules.mk", ".github/workflows/ci.yml", ".github/actions/setup/action.yml", ".github/actions/setup/run.sh"} {
		if !driver.Owns(path) {
			t.Errorf("Owns(%q) = false", path)
		}
	}
	for _, path := range []string{"docs/make.md", "core/core.go", ".github/CODEOWNERS"} {
		if driver.Owns(path) {
			t.Errorf("Owns(%q) = true", path)
		}
	}
}

// A dirty driver file with no revision to diff selects every target or job of
// that file, with the selection kinds and argv of AZU-V0-001.
func TestDriverFileWithoutRevisionSelectsEveryUnit_AZU_V0_001(t *testing.T) {
	graph := driverGraph(t, nil, nil)
	selected := driverSelected(t, affected.Select(graph, []string{"Makefile", ".github/workflows/ci.yml"}))
	wantDriverSelection(t, selected, []string{makeTest, makeLint, makeAll, makeOld, jobUnit, jobBuild, jobDyn}, nil)
	if got := selected[makeTest]; got.Kind != affected.SelectionKindDriverTarget || !slices.Equal(got.Argv, []string{"make", "test"}) {
		t.Errorf("make test selection %+v", got)
	}
	if got := selected[jobBuild]; got.Kind != affected.SelectionKindCIJob || got.Workflow != ".github/workflows/ci.yml" || got.Job != "build" || len(got.Argv) != 0 {
		t.Errorf("build job selection %+v", got)
	}
}

// A hunk inside one recipe selects that target and its dependents only.
func TestDriverRecipeHunkSelectsItsTarget_AZU_V0_001(t *testing.T) {
	head := strings.Replace(driverMakefile, "go vet ./core/...", "go vet -all ./core/...", 1)
	graph := driverGraph(t, map[string]string{"Makefile": head}, map[string]string{"Makefile": driverMakefile})
	selected := driverSelected(t, affected.Select(graph, []string{"Makefile"}))
	wantDriverSelection(t, selected, []string{makeLint, makeAll}, []string{makeTest, makeOld})
	if selected[makeLint].Witness.Kind != affected.WitnessDirectSource {
		t.Errorf("lint witness %+v", selected[makeLint].Witness)
	}
}

// A hunk outside every recipe (a variable) selects every target of the file.
func TestDriverRemainderHunkSelectsEveryTarget_AZU_V0_001(t *testing.T) {
	head := strings.Replace(driverMakefile, "GO ?= go", "GO ?= go1.27", 1)
	graph := driverGraph(t, map[string]string{"Makefile": head}, map[string]string{"Makefile": driverMakefile})
	selected := driverSelected(t, affected.Select(graph, []string{"Makefile"}))
	wantDriverSelection(t, selected, []string{makeTest, makeLint, makeAll, makeOld}, nil)
}

// Removing a target selects every driver unit: what ran it is unknown.
func TestDriverRemovedTargetSelectsEveryDriverUnit_AZU_V0_001(t *testing.T) {
	head := strings.Replace(driverMakefile, "old:\n\techo old\n", "", 1)
	graph := driverGraph(t, map[string]string{"Makefile": head}, map[string]string{"Makefile": driverMakefile})
	selected := driverSelected(t, affected.Select(graph, []string{"Makefile"}))
	wantDriverSelection(t, selected, []string{makeTest, makeLint, makeAll, jobUnit, jobBuild, jobDyn}, nil)
}

// A step hunk selects its job and the jobs that need it; a top-level env
// hunk selects every job of the workflow.
func TestDriverWorkflowHunks_AZU_V0_001(t *testing.T) {
	head := strings.Replace(driverWorkflow, "make lint", "make all", 1)
	graph := driverGraph(t, map[string]string{".github/workflows/ci.yml": head}, map[string]string{".github/workflows/ci.yml": driverWorkflow})
	selected := driverSelected(t, affected.Select(graph, []string{".github/workflows/ci.yml"}))
	wantDriverSelection(t, selected, []string{jobBuild}, []string{jobUnit})

	head = strings.Replace(driverWorkflow, `MODE: "1"`, `MODE: "2"`, 1)
	graph = driverGraph(t, map[string]string{".github/workflows/ci.yml": head}, map[string]string{".github/workflows/ci.yml": driverWorkflow})
	selected = driverSelected(t, affected.Select(graph, []string{".github/workflows/ci.yml"}))
	wantDriverSelection(t, selected, []string{jobUnit, jobBuild, jobDyn}, nil)
}

// A revision the block parser cannot read selects every job of the file.
func TestDriverUnparsableRevisionSelectsEveryJob_AZU_V0_001(t *testing.T) {
	base := strings.Replace(driverWorkflow, "env:\n  MODE: \"1\"", "env: &shared\n  MODE: \"1\"", 1)
	head := strings.Replace(driverWorkflow, "make lint", "make all", 1)
	graph := driverGraph(t, map[string]string{".github/workflows/ci.yml": head}, map[string]string{".github/workflows/ci.yml": base})
	selected := driverSelected(t, affected.Select(graph, []string{".github/workflows/ci.yml"}))
	wantDriverSelection(t, selected, []string{jobUnit, jobBuild, jobDyn}, nil)
}

// A Go change reaches the target and job that test its package, and the job
// that invokes a computed name is selected on every plan instead of leaving
// a driver:computed-invocation entry unknown.
func TestDriverEdgesAndComputedSite_AZU_V0_001(t *testing.T) {
	graph := driverGraph(t, nil, nil)
	selected := driverSelected(t, affected.Select(graph, []string{"core/core.go"}))
	wantDriverSelection(t, selected, []string{makeTest, makeLint, makeAll, jobUnit, jobBuild, jobDyn}, []string{makeOld})
	if got := selected[jobDyn].Witness.Kind; got != affected.WitnessFrontierSite {
		t.Errorf("dyn witness %s, want %s", got, affected.WitnessFrontierSite)
	}
	if got := selected[jobUnit].Witness.Kind; got != affected.WitnessDependency {
		t.Errorf("unit witness %s, want %s", got, affected.WitnessDependency)
	}
}

// AZU-V0-001: drivers run changed tests even when ordinary Go importers
// remain isolated from changes confined to another package's tests.
func TestDriverCallersOfContainedTests_AZU_V0_001(t *testing.T) {
	graph := driverGraph(t, map[string]string{
		"Makefile":                  "check:\n\tgo test -race ./core/...\nall: check\n",
		".github/workflows/ci.yml":  "jobs:\n  check:\n    steps:\n      - run: make check\n  after:\n    needs: check\n    steps:\n      - run: echo done\n",
		"consumer/consumer.go":      "package consumer\nimport \"example.test/m/core\"\nvar Value = core.Value\n",
		"consumer/consumer_test.go": "package consumer\nimport \"testing\"\nfunc TestValue(t *testing.T) { _ = Value() }\n",
	}, nil)
	dirty := []string{"core/core_test.go"}
	plan := affected.Select(graph, dirty)
	want := []string{"driver:make:Makefile#check", makeAll, "driver:ci:.github/workflows/ci.yml#check", "driver:ci:.github/workflows/ci.yml#after"}
	wantDriverSelection(t, driverSelected(t, plan), want, nil)
	selected := map[string]bool{}
	for _, selection := range plan.Selected {
		selected[selection.UnitID] = true
	}
	if !selected["go:example.test/m/core"] || selected["go:example.test/m/consumer"] {
		t.Fatalf("test-only importer isolation lost: %v", selected)
	}
	beyond := graph.ReachedBeyondOwnTests(dirty)
	for _, id := range want {
		if _, ok := beyond[id]; !ok {
			t.Errorf("driver %s absent beyond own tests", id)
		}
	}
	if _, ok := beyond["go:example.test/m/core"]; ok {
		t.Fatal("contained own tests included beyond own tests")
	}
	if _, ok := beyond["go:example.test/m/consumer"]; ok {
		t.Fatal("ordinary importer reached beyond own tests")
	}
}

// AZU-V0-001: omitted package arguments mean the package in the command's
// working directory, including commands whose only arguments are flags.
func TestDriverImplicitGoPackage_AZU_V0_001(t *testing.T) {
	for _, command := range []string{
		"cd core && go test", "cd core && go test -race",
		"cd core && go test -count 1", "cd core && go test -args -custom",
		"go -C core test -race", "cd core && go vet", "cd core && go build",
	} {
		t.Run(command, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                 "check:\n\t" + command + "\nall: check\n",
				".github/workflows/ci.yml": "jobs:\n  check:\n    steps:\n      - run: make check\n  after:\n    needs: check\n    steps:\n      - run: echo done\n",
			}, nil)
			selected := driverSelected(t, affected.Select(graph, []string{"core/core.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, "driver:ci:.github/workflows/ci.yml#check", "driver:ci:.github/workflows/ci.yml#after"}, nil)
		})
	}
}

// AZU-V0-001: unsupported flow mappings cannot silently hide a run step.
func TestDriverFlowMappingKeepsUncertainty_AZU_V0_001(t *testing.T) {
	for _, steps := range []string{
		"\n      - {run: 'go test -race ./core/...'}",
		"\n      - {run: 'go test -race ./core/...', name: race}",
		" [{run: 'go test -race ./core/...'}]",
	} {
		t.Run(steps, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:" + steps + "\n",
			}, nil)
			plan := affected.Select(graph, []string{"core/core.go"})
			if plan.Scope == affected.ScopeUnknown && len(plan.Unknown) > 0 {
				return
			}
			if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == jobUnit }) {
				t.Fatalf("flow step silently dropped: %+v", plan)
			}
		})
	}
}

// AZU-V0-001, AFP-V0-772: a project proof excludes unbounded readers of
// declared data, but neither an ordinary code change nor a literal reader.
func TestDriverUnparsableReaderExclusions_AZU_V0_001(t *testing.T) {
	files := map[string]string{
		"data/evidence.json":        "{}",
		affected.RuntimeReadersPath: `{"profile":"corvint-runtime-readers/0","entries":[{"path":"data/evidence.json","reason":"read only by literal readers"}]}`,
		".github/workflows/ci.yml":  "jobs:\n  unit:\n    steps:\n      - {run: 'go test -race ./core/...'}\n",
	}
	plan := affected.Select(driverGraph(t, files, nil), []string{"data/evidence.json"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("unproven data reader lost uncertainty: %+v", plan)
	}
	files[affected.UnboundedReaderExclusionsPath] = `{"profile":"corvint-unbounded-reader-exclusions/0","paths":["data/evidence.json"]}`
	graph := driverGraph(t, files, nil)
	plan = affected.Select(graph, []string{"data/evidence.json"})
	if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
		t.Fatalf("project reader proof ignored: %+v", plan)
	}
	plan = affected.Select(graph, []string{"data/evidence.json", "core/core.go"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("data proof hid code change: %+v", plan)
	}
	files[".github/workflows/ci.yml"] = "jobs:\n  unit:\n    steps:\n      - {run: 'cat data/evidence.json'}\n"
	plan = affected.Select(driverGraph(t, files, nil), []string{"data/evidence.json"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("data proof hid literal reader: %+v", plan)
	}
}
