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

// AZU-V0-9700: malformed wrappers in any recipe or step must remain selected,
// including recipes that no other target calls. Graph building must not panic.
func TestDriverMissingWrapperOperand_AZU_V0_9700(t *testing.T) {
	for _, command := range []string{"env -u", "env -C", "env -S", "nice -n", "timeout -k", "xargs -I", "ionice -c", "taskset -c", "env MODE=1 nice -n"} {
		t.Run(command, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                 "check:; " + command + "\nall:; echo all\n",
				".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: " + command + "\n",
			}, nil)
			selected := driverSelected(t, affected.Select(graph, []string{"core/core.go"}))
			want := []string{"driver:make:Makefile#check", jobUnit}
			wantDriverSelection(t, selected, want, []string{makeAll})
			for _, id := range want {
				if selected[id].Witness.Kind != affected.WitnessFrontierSite {
					t.Errorf("%s witness %s, want computed-site fallback", id, selected[id].Witness.Kind)
				}
			}
		})
	}
}

// AZU-V0-9701: package-free Go commands use the recipe or step directory,
// including after flags and before arguments passed to the test binary.
func TestDriverImplicitGoPackage_AZU_V0_9701(t *testing.T) {
	for _, command := range []string{"go test", "go vet", "go build", "go test -race -count 1", "go vet -tags=integration", "go build -o out", "go test -args -custom"} {
		for _, cwd := range []string{".", "core", "unknown"} {
			t.Run(command+"/"+cwd, func(t *testing.T) {
				prefix, directory, dirty := "", "", "root.go"
				if cwd == "core" {
					prefix, directory, dirty = "cd core && ", "        working-directory: core\n", "core/core.go"
				} else if cwd == "unknown" {
					prefix, directory, dirty = "cd $$DIR && ", "        working-directory: ${{ env.DIR }}\n", "core/core.go"
				}
				graph := driverGraph(t, map[string]string{
					"root.go":                  "package root\nfunc Value() int { return 1 }\n",
					"root_test.go":             "package root\nimport \"testing\"\nfunc TestValue(t *testing.T) { _ = Value() }\n",
					"Makefile":                 "check:; " + prefix + command + "\n",
					".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: " + command + "\n" + directory,
				}, nil)
				selected := driverSelected(t, affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", jobUnit}, nil)
			})
		}
	}
}

// AZU-V0-9702: alternatives must not turn ./$(PKG) into ./left right,
// silently losing the package named by make's final assignment.
func TestDriverAlternativeMakeValues_AZU_V0_9702(t *testing.T) {
	for _, tc := range []struct{ name, assignments, recipe, dirty string }{
		{"reassigned", "PKG = left\nPKG = right\n", "go test ./$(PKG)", "right/right.go"},
		{"conditional", "ifeq ($(MODE),left)\nPKG = left\nelse\nPKG = right\nendif\n", "go test ./$(PKG)", "right/right.go"},
		{"suffix", "PKG = left\nPKG = right\n", "go test ./$(PKG)/...", "right/right.go"},
		{"script", "PKG = left\nPKG = right\n", "sh ./$(PKG)/check.sh", "core/core.go"},
		{"same-value", "PKG = right\nPKG = right\n", "go test ./$(PKG)", "right/right.go"},
		{"same-value-append-suffix", "PKG = ./left\nPKG += ./left\n", "go test $(PKG)_suffix", "left/left.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                 tc.assignments + "check:; " + tc.recipe + "\n",
				"left/left.go":             "package left\n",
				"left/left_test.go":        "package left\nimport \"testing\"\nfunc TestLeft(t *testing.T) {}\n",
				"left_suffix/suffix.go":    "package left_suffix\n",
				"right/right.go":           "package right\n",
				"right/right_test.go":      "package right\nimport \"testing\"\nfunc TestRight(t *testing.T) {}\n",
				".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: make check\n",
			}, nil)
			selected := driverSelected(t, affected.Select(graph, []string{tc.dirty}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", jobUnit}, nil)
			unit, found := graph.Unit("driver:make:Makefile#check")
			if !found || !slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation) {
				t.Errorf("ambiguous recipe did not retain computed-site fallback")
			}
		})
	}
}

// AZU-V0-9703: the YAML subset retains flow maps as text. Unsupported step
// structures must conservatively select their job and its dependents.
func TestDriverFlowSteps_AZU_V0_9703(t *testing.T) {
	for _, steps := range []string{
		"    steps: [{run: 'go test ./core/...'}]\n",
		"    steps:\n      - {run: 'go test ./core/...'}\n",
		"    steps: [\n      {run: 'go test ./core/...'}\n      ]\n",
		"    steps: [{run: 'go test ./core/...'}, {run: 'echo done'}]\n",
	} {
		t.Run(steps, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				".github/workflows/ci.yml": "jobs:\n  unit:\n" + steps + "  build:\n    needs: [unit]\n    steps:\n      - run: echo build\n  other:\n    steps:\n      - run: echo other\n",
			}, nil)
			selected := driverSelected(t, affected.Select(graph, []string{"core/core.go"}))
			wantDriverSelection(t, selected, []string{jobUnit, jobBuild}, []string{"driver:ci:.github/workflows/ci.yml#other"})
			if selected[jobUnit].Witness.Kind != affected.WitnessFrontierSite {
				t.Errorf("flow step did not retain computed-site fallback")
			}
		})
	}
}

// AZU-V0-9850: approximate word selection loses attached prefixes/suffixes.
func TestDriverMakeFunctionsComputed_AZU_V0_9850(t *testing.T) {
	for _, expression := range []string{
		"$(join pkg,_test)", "$(firstword pkg other)_test", "$(lastword other pkg)_test",
		"$(word 2,other pkg)_test", "$(wordlist 2,2,other pkg)_test",
		"$(filter pkg,other pkg)_test", "$(filter-out other,other pkg)_test",
		"$(if yes,pkg,other)_test", "$(or pkg,other)_test", "$(and other,pkg)_test",
		"$(findstring absent,pkg)pkg_test",
	} {
		t.Run(expression, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                 "check:; go test ./" + expression + "\nall: check\n",
				"pkg_test/value.go":        "package pkg_test\n",
				"pkg_test/value_test.go":   "package pkg_test\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n",
				".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: make all\n",
			}, nil)
			selected := driverSelected(t, affected.Select(graph, []string{"pkg_test/value.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			unit, found := graph.Unit("driver:make:Makefile#check")
			if !found || !slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation) {
				t.Error("function did not retain computed-site fallback")
			}
		})
	}
}

// Exact make sort must remove duplicates before a suffix is attached.
func TestDriverMakeSortWords_AZU_V0_9850(t *testing.T) {
	graph := driverGraph(t, map[string]string{
		"Makefile":                 "check:; go test ./$(sort pkg pkg)_test\n",
		"pkg_test/value.go":        "package pkg_test\n",
		"pkg_test/value_test.go":   "package pkg_test\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n",
		".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: make check\n",
	}, nil)
	selected := driverSelected(t, affected.Select(graph, []string{"pkg_test/value.go"}))
	wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", jobUnit}, nil)
}

// AZU-V0-9851: dropping env's cwd hides the services/pkg dependency.
func TestDriverEnvChdirComputed_AZU_V0_9851(t *testing.T) {
	for _, option := range []string{"-C services", "-Cservices", "--chdir=services", "--chdir services", "-C $$DIR", "--chdir=$$DIR"} {
		t.Run(option, func(t *testing.T) {
			command := "env " + option + " go test ./pkg"
			graph := driverGraph(t, map[string]string{
				"Makefile":                   "check:; " + command + "\n",
				"services/pkg/value.go":      "package pkg\n",
				"services/pkg/value_test.go": "package pkg\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n",
				".github/workflows/ci.yml":   "jobs:\n  unit:\n    steps:\n      - run: " + strings.ReplaceAll(command, "$$", "$") + "\n",
			}, nil)
			selected := driverSelected(t, affected.Select(graph, []string{"services/pkg/value.go"}))
			want := []string{"driver:make:Makefile#check", jobUnit}
			wantDriverSelection(t, selected, want, nil)
			for _, id := range want {
				if selected[id].Witness.Kind != affected.WitnessFrontierSite {
					t.Errorf("%s did not retain env directory fallback", id)
				}
			}
		})
	}
}

// AZU-V0-9852: composite actions follow the same flow-step boundary as jobs.
func TestDriverCompositeFlowSteps_AZU_V0_9852(t *testing.T) {
	for _, steps := range []string{
		"  steps: [{run: 'go test ./core/...', shell: bash}]\n",
		"  steps:\n    - {run: 'go test ./core/...', shell: bash}\n",
	} {
		t.Run(steps, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                         "other:; echo other\n",
				".github/actions/check/action.yml": "runs:\n  using: composite\n" + steps,
				".github/workflows/ci.yml":         "jobs:\n  unit:\n    steps:\n      - uses: ./.github/actions/check\n  build:\n    needs: unit\n    steps:\n      - run: echo build\n",
			}, nil)
			selected := driverSelected(t, affected.Select(graph, []string{"core/core.go"}))
			id := "driver:action:.github/actions/check"
			wantDriverSelection(t, selected, []string{jobUnit, jobBuild}, []string{"driver:make:Makefile#other"})
			unit, found := graph.Unit(id)
			if !found || !slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation) {
				t.Error("composite flow steps did not retain computed-site fallback")
			}
		})
	}
}

// AZU-V0-9853: unknown local uses must retain their callers, including nested actions.
func TestDriverUnresolvedLocalUses_AZU_V0_9853(t *testing.T) {
	for _, target := range []string{"./tools/check-action", "./missing-action", "./../outside", "./.github/workflows/missing.yml"} {
		t.Run(target, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                           "other:; echo other\n",
				"tools/check-action/action.yml":      "runs:\n  using: composite\n  steps:\n    - run: go test ./core/...\n      shell: bash\n",
				".github/actions/wrapper/action.yml": "runs:\n  using: composite\n  steps:\n    - uses: " + target + "\n",
				".github/workflows/ci.yml":           "jobs:\n  unit:\n    steps:\n      - uses: " + target + "\n  build:\n    needs: unit\n    steps:\n      - uses: ./.github/actions/wrapper\n",
			}, nil)
			selected := driverSelected(t, affected.Select(graph, []string{"core/core.go"}))
			id := "driver:action:.github/actions/wrapper"
			wantDriverSelection(t, selected, []string{jobUnit, jobBuild}, []string{"driver:make:Makefile#other"})
			for _, site := range []string{id, jobUnit} {
				unit, found := graph.Unit(site)
				if !found || !slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation) {
					t.Errorf("%s silently accepted unresolved local uses", site)
				}
			}
		})
	}
}
