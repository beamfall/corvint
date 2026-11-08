// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const frozenProfilePath = "cmd/corvint/core_freeze_test.go"
const frozenProfileName = "TestCoreVerbsEmitTheFrozenProfiles"

// Read the actual repository policy: a copied declaration would not detect
// accidental removal of the guard. Other repository owners do not belong to
// this disposable module and are deliberately not copied.
func frozenProfileDeclaration(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("../..", affected.DeclaredSelectionPath))
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Profile string                  `json:"profile"`
		MustRun []affected.MustRunEntry `json:"mustRun"`
	}
	if err := json.Unmarshal(body, &policy); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(policy.MustRun, func(entry affected.MustRunEntry) bool {
		return entry.Tests == frozenProfilePath && strings.Contains(entry.Reason, frozenProfileName)
	}) {
		t.Fatal("AFP-V0-9320: repository policy must name the frozen profile test and its reason")
	}
	body, err = json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func frozenProfileSelectionFixture(t *testing.T, declaration string) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                    "module example.test/frozen\n\ngo 1.27.0\n",
		frozenProfilePath:           "package main\nimport \"testing\"\nfunc " + frozenProfileName + "(t *testing.T) {}\n",
		"cmd/corvint/other_test.go": "package main\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\n",
		"engine/graph.go":           "package engine\nfunc Value() int { return 1 }\n",
		"engine/graph_test.go":      "package engine\nimport \"testing\"\nfunc TestGraph(t *testing.T) { _ = Value() }\n",
	} {
		cemWrite(t, root, name, body)
	}
	if declaration != "" {
		cemWrite(t, root, affected.DeclaredSelectionPath, declaration)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.test"}, {"config", "gc.auto", "0"}, {"config", "maintenance.auto", "false"}, {"add", "."}, {"commit", "-qm", "base"}} {
		affectedGit(t, root, args...)
	}
	return root, strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
}

// AFP-V0-9320, AFP-V0-9321: both graph-changing and exclusion-only diffs
// keep the frozen test. The undeclared controls expose an actual omission,
// without relying on unrelated repository-wide unbounded readers.
func TestAffectedDigestChangeKeepsFrozenProfiles_AFPV09320(t *testing.T) {
	t.Parallel()
	declaration := frozenProfileDeclaration(t)
	for _, mutation := range []string{"graph", "excluded"} {
		for _, state := range []string{"worktree", "staged", "committed"} {
			for _, declared := range []bool{false, true} {
				name := mutation + "/" + state + "/undeclared"
				policy := ""
				if declared {
					name = mutation + "/" + state + "/declared"
					policy = declaration
				}
				t.Run(name, func(t *testing.T) {
					root, base := frozenProfileSelectionFixture(t, policy)
					before, err := compileAffected(context.Background(), affectedInvocation{Root: root, Base: base, NoCache: true})
					if err != nil {
						t.Fatal(err)
					}
					if len(before.Plan.Selected) != 0 {
						t.Fatalf("clean plan selected tests: %+v", before.Plan.Selected)
					}
					if mutation == "graph" {
						cemWrite(t, root, "engine/added.go", "package engine\nfunc Added() int { return 2 }\n")
					} else {
						cemWrite(t, root, "engine/graph.go", "package engine\nfunc Value() int { return 2 }\n")
					}
					if state != "worktree" {
						affectedGit(t, root, "add", ".")
					}
					if state == "committed" {
						affectedGit(t, root, "commit", "-qm", "change")
					}
					after, graph, err := compileAffectedGraph(context.Background(), affectedInvocation{Root: root, Base: base, NoCache: true})
					if err != nil {
						t.Fatal(err)
					}
					if changed := before.Plan.GraphDigest != after.Plan.GraphDigest; changed != (mutation == "graph") {
						t.Fatalf("graph digest changed=%v for %s", changed, mutation)
					}
					oldExcluded, err := summarizeAffectedExclusions(before.Plan.Excluded)
					if err != nil {
						t.Fatal(err)
					}
					newExcluded, err := summarizeAffectedExclusions(after.Plan.Excluded)
					if err != nil || oldExcluded.Digest == newExcluded.Digest {
						t.Fatalf("exclusion digest did not change: %v", err)
					}
					selected := slices.ContainsFunc(after.Plan.Selected, func(s affected.Selection) bool {
						return slices.Contains(s.Tests, frozenProfilePath)
					})
					if selected != declared {
						t.Fatalf("frozen test selected=%v declared=%v: %+v", selected, declared, after.Plan)
					}
					if !declared {
						return
					}
					// AFP-V0-9322: the generated lane verification command and
					// downstream slice include the test, not just the raw plan.
					const pkg = "example.test/frozen/cmd/corvint"
					if after.Provider.Go.State != providerStateRunnable || !slices.Contains(after.Provider.Go.Packages, pkg) {
						t.Fatalf("frozen package missing from provider: %+v", after.Provider.Go)
					}
					if !slices.ContainsFunc(after.Advice.Checks, func(c affectedCheck) bool {
						return c.Kind == adviceKindAdvisory && strings.Contains(c.Command, pkg) && !strings.Contains(c.Command, "-run")
					}) {
						t.Fatalf("lane verify command omits frozen test: %+v", after.Advice.Checks)
					}
					got, ok := testGoSlices(context.Background(), after.Plan, graph, root, base, typedReachExecutor{environ: os.Environ()})[pkg]
					if !ok || got.Pattern != "" || !strings.Contains(got.Reason, frozenProfileName) {
						t.Fatalf("frozen test sliced away: %+v, present=%v", got, ok)
					}
				})
			}
		}
	}
}

// AFP-V0-9322: a direct edit to another test must not narrow away the
// unchanged frozen test. A removed guard file must refuse the graph build.
func TestAffectedFrozenProfilesGuardFailureModes_AFPV09322(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"other-test", "missing-guard"} {
		t.Run(change, func(t *testing.T) {
			root, base := frozenProfileSelectionFixture(t, frozenProfileDeclaration(t))
			if change == "missing-guard" {
				if err := os.Remove(filepath.Join(root, frozenProfilePath)); err != nil {
					t.Fatal(err)
				}
			} else {
				appendFile(t, filepath.Join(root, "cmd/corvint/other_test.go"), "\n// changed\n")
			}
			receipt, graph, err := compileAffectedGraph(context.Background(), affectedInvocation{Root: root, Base: base, NoCache: true})
			if change == "missing-guard" {
				if err == nil || !strings.Contains(err.Error(), affected.DeclaredSelectionPath) || !strings.Contains(err.Error(), frozenProfilePath) || !strings.Contains(err.Error(), "holds no test file") {
					t.Fatalf("missing guard must refuse with the contradicted declaration: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, ok := testGoSlices(context.Background(), receipt.Plan, graph, root, base, typedReachExecutor{environ: os.Environ()})["example.test/frozen/cmd/corvint"]
			if !ok || got.Pattern != "" || !strings.Contains(got.Reason, frozenProfileName) {
				t.Fatalf("other test edit drops frozen test: %+v, present=%v", got, ok)
			}
		})
	}
}
