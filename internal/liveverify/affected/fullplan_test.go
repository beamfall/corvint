// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

// These reduced replays retain the three offending paths from 39c031980,
// 8e2c13aa9 and cdeed7ee6. Unrelated tests make FULL distinguishable from
// precise selection. The Python test loads its helper dynamically, as in CI.
func TestFullplanReplay_AZU_V0_9920_9921(t *testing.T) {
	files := map[string]string{
		"go.mod":                                     "module example.test/m\n\ngo 1.27.3\n",
		".github/testconfine/scopes.go":              "package testconfine\n",
		".github/testconfine/scopes_test.go":         "package testconfine\nimport \"testing\"\nfunc TestScopes(t *testing.T) {}\n",
		".github/testconfine/cmd/main.go":            "package main\nimport _ \"example.test/m/.github/testconfine\"\nfunc main() {}\n",
		".github/testconfine/cmd/main_test.go":       "package main\nimport \"testing\"\nfunc TestMainCommand(t *testing.T) {}\n",
		".github/macos/select.py":                    "def selection():\n    return []\n",
		".github/macos/select_test.py":               "import importlib.util\nimport unittest\nspec = importlib.util.spec_from_file_location('macselect', 'select.py')\n",
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
			body, err = json.Marshal(declaration)
			if err != nil {
				t.Fatal(err)
			}
		}
		files[name] = string(body)
	}
	disk, immutable := t.TempDir(), fstest.MapFS{}
	writeOwnershipFixture(t, disk, files)
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
				{".github/macos/select.py", ".github/macos/select_test.py", sessiontest.ScopeNarrowed, 1},
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
					manifest := fullplanManifest(t, graph, plan, sessiontest.ProfileLocal)
					if manifest.Scope != tc.scope || len(manifest.Units) != tc.units {
						t.Fatalf("scope=%s units=%v unknown=%v reason=%s; want %s %d", manifest.Scope, manifest.Units, plan.Unknown, manifest.Reason, tc.scope, tc.units)
					}
					if tc.path == ".github/macos/select.py" {
						want := []string{"python3", "-B", ".github/macos/select_test.py"}
						declared := sessiontest.ReadDeclared(repositoryRoot(t))
						invocations := sessiontest.Invocations(manifest, declared, 0, disk, "")
						if len(manifest.NoSuite) != 0 || len(manifest.Incomplete) != 0 || len(invocations) != 1 || !slices.Equal(invocations[0].Argv, want) {
							t.Fatalf("maintained Python command not executable: manifest=%+v invocations=%+v", manifest, invocations)
						}
						ci := fullplanManifest(t, graph, plan, sessiontest.ProfileCI)
						if len(ci.Units) != 0 || len(ci.NoSuite) != 0 || len(ci.Incomplete) != 0 || !slices.Contains(ci.Checks, strings.Join(want, " ")) {
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

// Exercise AFP-V0-028 admission using the fixture graph's exact Go selection.
// The measured repository replay separately uses the structural Go selector.
func fullplanManifest(t *testing.T, graph *affected.Graph, plan affected.Plan, profile string) sessiontest.Manifest {
	t.Helper()
	input := sessiontest.Input{
		Base: strings.Repeat("a", 40), GraphDigest: graph.Digest(), PlanDigest: "sha256:fixture",
		RunnerProfile: profile, ShardCount: 1,
		Source:   sessiontest.SourceIdentity{Head: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)},
		Declared: sessiontest.ReadDeclared(repositoryRoot(t)),
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

// Declared source-shaped output still selects its readers, while a real
// indexed source covered by the declaration keeps its source owner.
func TestFullplanEvidenceReaders_AZU_V0_9921(t *testing.T) {
	root := t.TempDir()
	writeOwnershipFixture(t, root, map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27.3\n",
		affected.NonSourcePathsPath: `{"profile":"corvint-non-source-paths/0","entries":[{"path":".agent-evidence/","reason":"retained output"},{"path":"live/","reason":"output alongside indexed source"}]}`,
		".agent-evidence/env.sh":    "export EXAMPLE=1\n",
		"literal/literal_test.go":   "package literal\nimport \"testing\"\nvar Path = \".agent-evidence/env.sh\"\nfunc TestLiteral(t *testing.T) {}\n",
		"scoped/scoped_test.go":     "package scoped\nimport \"testing\"\nfunc TestScoped(t *testing.T) {}\n",
		affected.ReadScopesPath:     `{"profile":"corvint-test-read-scopes/0","packages":{"scoped":[".agent-evidence/"]}}`,
		"live/live.go":              "package live\n",
		"live/live_test.go":         "package live\nimport \"testing\"\nfunc TestLive(t *testing.T) {}\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), shell.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{".agent-evidence/env.sh"})
	for _, test := range []string{"literal/literal_test.go", "scoped/scoped_test.go"} {
		if !slices.Contains(plan.SelectedTests(), test) {
			t.Fatalf("lost reader %s: %+v", test, plan)
		}
	}
	if len(plan.NonSource) != 1 || plan.NonSource[0].Class != affected.ClassDeclaredNonSource {
		t.Fatalf("output classification: %+v", plan)
	}
	plan = affected.Select(graph, []string{"live/live.go"})
	if len(plan.NonSource) != 0 || !slices.Contains(plan.SelectedTests(), "live/live_test.go") {
		t.Fatalf("indexed source lost ownership: %+v", plan)
	}
}
