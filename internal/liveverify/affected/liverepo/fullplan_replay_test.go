// SPDX-License-Identifier: Apache-2.0
package liverepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/python"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// fullplanPythonSuite is the CI Python suite AZU-V0-9920 requires the maintained
// test-command declaration to name.
var fullplanPythonSuite = []string{"python3", "-B", ".github/macos/select_test.py"}

// fullplanMaintainedPythonNone is the python entry the maintained declaration holds until the
// .github edit naming the CI Python suite lands (AZU-V0-9920, V1-4730).
const fullplanMaintainedPythonNone = "no make gate step, script/gate-affected.sh, tools/corvint-pr-tests or CI job runs a Python suite; Python files are benchmark and qualification scripts, runner reporters and affected-graph testdata fixtures, and the repository has no pytest or unittest configuration"

// fullplanPythonDifference checks the known difference between the maintained declaration's
// python entry and the suite the replay sets (AZU-V0-9920, V1-4730). It returns "" while the
// maintained entry is the recorded none, and a failure message once the entry names the suite
// (delete the override then) or changes to anything else.
func fullplanPythonDifference(maintained any) string {
	switch {
	case reflect.DeepEqual(maintained, []any{"python3", "-B", ".github/macos/select_test.py"}):
		return sessiontest.DeclarationPath + " now names the CI Python suite; delete the override in fullplanDeclared and this known-difference check (AZU-V0-9920)"
	case !reflect.DeepEqual(maintained, map[string]any{"none": fullplanMaintainedPythonNone}):
		return fmt.Sprintf("%s python entry changed from the recorded known difference to %#v; update fullplanDeclared (AZU-V0-9920)", sessiontest.DeclarationPath, maintained)
	}
	return ""
}

// TestFullplanPythonKnownDifference_AZU_V0_9920 pins the known-difference check: the recorded
// none passes, the CI suite and any other entry fail with their own message.
func TestFullplanPythonKnownDifference_AZU_V0_9920(t *testing.T) {
	if problem := fullplanPythonDifference(map[string]any{"none": fullplanMaintainedPythonNone}); problem != "" {
		t.Fatalf("recorded none: %s", problem)
	}
	if problem := fullplanPythonDifference([]any{"python3", "-B", ".github/macos/select_test.py"}); !strings.Contains(problem, "delete the override") {
		t.Fatalf("suite declared: %q", problem)
	}
	for _, other := range []any{map[string]any{"none": "edited reason"}, []any{"python3", "-m", "pytest"}, nil} {
		if problem := fullplanPythonDifference(other); !strings.Contains(problem, "changed from the recorded known difference") {
			t.Fatalf("%#v: %q", other, problem)
		}
	}
}

// fullplanWrite writes files under root, creating parent directories.
func fullplanWrite(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fullplanDeclared is the maintained .github/cishards/test-commands.json with its
// python entry set to the CI Python suite. The maintained file still declares
// python as none until the claude/ci-1107x .github edit lands; once it names the
// suite, this override changes nothing (AZU-V0-9920, V1-4730).
func fullplanDeclared(t *testing.T) sessiontest.Declared {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(sessiontest.DeclarationPath)))
	if err != nil {
		t.Fatal(err)
	}
	var declaration map[string]any
	if err := json.Unmarshal(raw, &declaration); err != nil {
		t.Fatal(err)
	}
	languages, ok := declaration["languages"].(map[string]any)
	if !ok {
		t.Fatalf("maintained declaration has no languages object")
	}
	if problem := fullplanPythonDifference(languages["python"]); problem != "" {
		t.Fatal(problem)
	}
	languages["python"] = fullplanPythonSuite
	if raw, err = json.Marshal(declaration); err != nil {
		t.Fatal(err)
	}
	declared := sessiontest.ParseDeclared(raw)
	if declared.Unreadable != "" {
		t.Fatal(declared.Unreadable)
	}
	return declared
}

// TestFullplanReplay_AZU_V0_9920_9921 replays reduced fixtures of the three
// offending paths of merges 39c031980 (.github/macos), 8e2c13aa9
// (.github/testconfine/cmd/main.go) and cdeed7ee6 (.agent-evidence/pk-tstore-fix1)
// against the maintained .corvint declarations (AZU-V0-9922). Unrelated tests make
// FULL distinguishable from precise selection; the helper paths narrow to their
// named tests, retained evidence adds no unit and no uncertainty, and a new
// unknown kind of source path still goes FULL (AZU-V0-9921, AFP-V0-028). The
// Python test loads its helper dynamically, as in CI.
func TestFullplanReplay_AZU_V0_9920_9921(t *testing.T) {
	files := map[string]string{
		"go.mod":                                     "module example.test/m\n\ngo 1.27.3\n",
		".github/testconfine/scopes.go":              "package testconfine\n",
		".github/testconfine/scopes_test.go":         "package testconfine\nimport \"testing\"\nfunc TestScopes(t *testing.T) {}\n",
		".github/testconfine/cmd/main.go":            "package main\nimport _ \"example.test/m/.github/testconfine\"\nfunc main() {}\n",
		".github/testconfine/cmd/main_test.go":       "package main\nimport \"testing\"\nfunc TestMainCommand(t *testing.T) {}\n",
		".github/macos/darwin_select.py":             "def selection():\n    return []\n",
		".github/macos/select_test.py":               "import importlib.util\nimport unittest\nspec = importlib.util.spec_from_file_location('macselect', 'darwin_select.py')\n",
		"other/other_test.go":                        "package other\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\n",
		".agent-evidence/pk-tstore-fix1/env-base.sh": "export EXAMPLE=1\n",
		".agent-evidence/pk-tstore-fix1/env.sh":      "export EXAMPLE=2\n",
		".agent-evidence/run.py":                     "retained output\n",
		".agent-evidence/run.go":                     "retained output\n",
		".unknown/new.py":                            "print('unindexed')\n",
	}
	// Use the maintained declarations, not a fixture-only version of the rule.
	for _, name := range []string{affected.NonSourcePathsPath, affected.RuntimeEdgesPath} {
		body, err := os.ReadFile(filepath.Join(repositoryRoot(t), name))
		if err != nil {
			t.Fatal(err)
		}
		if name == affected.RuntimeEdgesPath {
			var declaration struct {
				Profile string                 `json:"profile"`
				Edges   []affected.RuntimeEdge `json:"edges"`
			}
			if err := json.Unmarshal(body, &declaration); err != nil {
				t.Fatal(err)
			}
			declaration.Edges = slices.DeleteFunc(declaration.Edges, func(edge affected.RuntimeEdge) bool { return edge.Tests != ".github/macos/select_test.py" })
			if len(declaration.Edges) == 0 {
				t.Fatalf("maintained %s lost the .github/macos/select_test.py edge", name)
			}
			body, err = json.Marshal(declaration)
			if err != nil {
				t.Fatal(err)
			}
		}
		files[name] = string(body)
	}
	declared := fullplanDeclared(t)
	disk, immutable := t.TempDir(), fstest.MapFS{}
	fullplanWrite(t, disk, files)
	for name, body := range files {
		immutable[name] = &fstest.MapFile{Data: []byte(body), Mode: 0o644}
	}
	plugins := []affected.Language{golang.New(), python.New(), shell.New()}
	diskGraph, err := affected.Build(context.Background(), disk, plugins...)
	if err != nil {
		t.Fatal(err)
	}
	immutableGraph, err := affected.BuildFS(immutable, plugins...)
	if err != nil {
		t.Fatal(err)
	}
	for name, graph := range map[string]*affected.Graph{"disk": diskGraph, "immutable": immutableGraph} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				path, test, scope string
				units             int
			}{
				{".github/macos/darwin_select.py", ".github/macos/select_test.py", sessiontest.ScopeNarrowed, 1},
				{".github/testconfine/cmd/main.go", ".github/testconfine/cmd/main_test.go", sessiontest.ScopeNarrowed, 1},
				{".github/testconfine/scopes.go", ".github/testconfine/cmd/main_test.go", sessiontest.ScopeNarrowed, 2},
				{".agent-evidence/pk-tstore-fix1/env-base.sh", "", sessiontest.ScopeNothing, 0},
				{".agent-evidence/pk-tstore-fix1/env.sh", "", sessiontest.ScopeNothing, 0},
				{".agent-evidence/run.py", "", sessiontest.ScopeNothing, 0},
				{".agent-evidence/run.go", "", sessiontest.ScopeNothing, 0},
				{".unknown/new.py", "", sessiontest.ScopeFull, 6},
			} {
				t.Run(tc.path, func(t *testing.T) {
					plan := affected.Select(graph, []string{tc.path})
					if tc.test != "" && !slices.Contains(plan.SelectedTests(), tc.test) {
						t.Errorf("missing %s: %+v", tc.test, plan)
					}
					manifest := fullplanManifest(t, graph, plan, declared, sessiontest.ProfileLocal)
					if manifest.Scope != tc.scope || len(manifest.Units) != tc.units {
						t.Fatalf("scope=%s units=%v unknown=%v reason=%s; want %s %d", manifest.Scope, manifest.Units, plan.Unknown, manifest.Reason, tc.scope, tc.units)
					}
					if tc.path == ".github/macos/darwin_select.py" {
						invocations := sessiontest.Invocations(manifest, declared, 0, disk, "")
						if len(manifest.NoSuite) != 0 || len(manifest.Incomplete) != 0 || len(invocations) != 1 || !slices.Equal(invocations[0].Argv, fullplanPythonSuite) {
							t.Fatalf("Python suite not executable: manifest=%+v invocations=%+v", manifest, invocations)
						}
						ci := fullplanManifest(t, graph, plan, declared, sessiontest.ProfileCI)
						if len(ci.Units) != 0 || len(ci.NoSuite) != 0 || len(ci.Incomplete) != 0 || !slices.Contains(ci.Checks, strings.Join(fullplanPythonSuite, " ")) {
							t.Fatalf("CI lost its required Python check: %+v", ci)
						}
					}
					if tc.units == 0 && (len(plan.Selected) != 0 || len(plan.Unknown) != 0 || len(plan.NonSource) != 1 || plan.NonSource[0].Class != affected.ClassDeclaredNonSource) {
						t.Fatalf("evidence adds units or uncertainty: %+v", plan)
					}
					if tc.scope == sessiontest.ScopeFull && !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
						return u.Reason == affected.UnknownUnindexedSourcePath && u.Detail == tc.path
					}) {
						t.Fatalf("unknown source lost its diagnostic: %+v", plan.Unknown)
					}
				})
			}
		})
	}
}

// fullplanManifest exercises AFP-V0-028 admission using the fixture graph's exact
// Go selection. The measured repository replay separately uses the structural Go
// selector.
func fullplanManifest(t *testing.T, graph *affected.Graph, plan affected.Plan, declared sessiontest.Declared, profile string) sessiontest.Manifest {
	t.Helper()
	input := sessiontest.Input{
		Base: strings.Repeat("a", 40), GraphDigest: graph.Digest(), PlanDigest: "sha256:fixture",
		RunnerProfile: profile, ShardCount: 1,
		Source:   sessiontest.SourceIdentity{Head: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)},
		Declared: declared,
	}
	for _, id := range graph.UnitIDs() {
		if pkg, ok := strings.CutPrefix(id, "go:"); ok {
			input.Universe = append(input.Universe, pkg)
		}
	}
	var packages []string
	for _, selected := range plan.Selected {
		input.Selected = append(input.Selected, selected.UnitID)
		if pkg, ok := strings.CutPrefix(selected.UnitID, "go:"); ok {
			packages = append(packages, pkg)
		}
	}
	for _, unknown := range plan.Unknown {
		input.Unknown = append(input.Unknown, sessiontest.Unknown{Reason: unknown.Reason, Detail: unknown.Detail})
	}
	input.GoSelection = "NOTHING the fixture selects no Go package"
	if len(packages) != 0 {
		input.GoSelection = "run " + strings.Join(packages, " ")
	}
	manifest, err := sessiontest.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
