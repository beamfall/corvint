//go:build darwin || linux

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// bulkSliceFixture is a module with packages p0..p(count-1), each a source
// file and a test file, a package z beside them and a package a that p0
// imports; it commits that tree, applies edits and returns the root and base.
func bulkSliceFixture(t *testing.T, count int, base, edits map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":      "module example.test/m\n\ngo 1.27\n",
		"a/a.go":      "package a\n\nfunc A() int { return 1 }\n",
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { _ = A() }\n",
		"z/z.go":      "package z\n\nfunc Z() int { return 1 }\n",
		"z/z_test.go": "package z\n\nimport \"testing\"\n\nfunc TestZ(t *testing.T) { _ = Z() }\n",
	}
	for i := range count {
		name := fmt.Sprintf("p%d", i)
		files[name+"/"+name+".go"] = "package " + name + "\n\nfunc P() int { return 1 }\n"
		files[name+"/"+name+"_test.go"] = "package " + name + "\n\nimport \"testing\"\n\nfunc TestP(t *testing.T) { _ = P() }\n"
	}
	if count != 0 {
		files["p0/p0.go"] = "package p0\n\nimport \"example.test/m/a\"\n\nfunc P() int { return a.A() }\n"
	}
	for name, body := range base {
		files[name] = body
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	revision := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for name, body := range edits {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, revision
}

// bulkTestEdits edits the test file of every package p0..p(count-1).
func bulkTestEdits(count int) map[string]string {
	edits := make(map[string]string, count)
	for i := range count {
		name := fmt.Sprintf("p%d", i)
		edits[name+"/"+name+"_test.go"] = "package " + name + "\n\nimport \"testing\"\n\nfunc TestP(t *testing.T) { _ = P() + 1 }\n"
	}
	return edits
}

// legacyReachedElsewhere is the per-package reference STR-V0-072 replaced:
// one Select per DIRECT_TEST_CHANGE Go package over the dirty paths that are
// not its own test files.
func legacyReachedElsewhere(plan affected.Plan, graph *affected.Graph) map[string]string {
	reasons := make(map[string]string)
	for _, selection := range plan.Selected {
		if !strings.HasPrefix(selection.UnitID, "go:") || selection.Witness.Kind != affected.WitnessDirectTest {
			continue
		}
		unit, _ := graph.Unit(selection.UnitID)
		var rest []string
		for _, path := range plan.Dirty {
			if !testFileOf(unit, path) {
				rest = append(rest, path)
			}
		}
		if len(rest) == 0 {
			continue
		}
		for _, other := range affected.Select(graph, rest).Selected {
			if other.UnitID == selection.UnitID {
				reasons[strings.TrimPrefix(selection.UnitID, "go:")] = "whole: also selected by " + other.Witness.Kind + " via " + other.Witness.DirtyPath
			}
		}
	}
	return reasons
}

// bulkSlices plans root against base and slices it, counting the plan walks
// and the Git processes test slicing makes.
func bulkSlices(t *testing.T, root, base string) (affected.Plan, *affected.Graph, map[string]sessiontest.GoSlice, int, int) {
	t.Helper()
	receipt, graph, err := compileAffectedGraph(context.Background(), affectedInvocation{Root: root, Base: base})
	if err != nil {
		t.Fatal(err)
	}
	walks := 0
	select_, reach := testSliceSelect, testSliceReach
	t.Cleanup(func() { testSliceSelect, testSliceReach = select_, reach })
	testSliceSelect = func(graph *affected.Graph, dirty []string) affected.Plan { walks++; return select_(graph, dirty) }
	testSliceReach = func(graph *affected.Graph, dirty []string) map[string]affected.Witness {
		walks++
		return reach(graph, dirty)
	}
	log := filepath.Join(t.TempDir(), "git.log")
	script := sourceGitScript(t, "echo \"$*\" >> "+log+"\nexec /usr/bin/git \"$@\"\n")
	ctx := liftedSourceDeadline(context.WithValue(context.Background(), sourceGitExecutableKey{}, script))
	sliced := testGoSlices(ctx, receipt.Plan, graph, root, base)
	processes := 0
	if raw, err := os.ReadFile(log); err == nil {
		processes = strings.Count(string(raw), "\n")
	}
	return receipt.Plan, graph, sliced, walks, processes
}

// STR-V0-072 (V1-3021): test slicing walks the plan a bounded number of
// times and reads the base in a constant number of Git processes, however
// many packages it slices.
func TestSTRV0072SliceWalksDoNotGrowWithPackages(t *testing.T) {
	for _, count := range []int{1, 2, 4, 8} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			edits := bulkTestEdits(count)
			edits["z/z.go"] = "package z\n\nfunc Z() int { return 2 }\n"
			root, base := bulkSliceFixture(t, count, nil, edits)
			_, _, sliced, walks, processes := bulkSlices(t, root, base)
			for i := range count {
				name := fmt.Sprintf("p%d", i)
				want := sessiontest.GoSlice{Pattern: "^(TestP)$", Reason: "selected: the tests of " + name + "/" + name + "_test.go"}
				if got := sliced["example.test/m/"+name]; got != want {
					t.Fatalf("%s sliced %+v, want %+v", name, got, want)
				}
			}
			if walks > 2 || processes > 2 {
				t.Fatalf("%d packages took %d plan walks and %d Git processes, want at most 2 of each", count, walks, processes)
			}
		})
	}
}

// STR-V0-072 (V1-3021): the bulk walk gives every package the slice reason
// the per-package Select gave it: a dependency change, a reader of another
// package's changed test file, and clean test-only edits.
func TestSTRV0072BulkSliceReasonsMatchPerPackageSelect(t *testing.T) {
	cases := []struct {
		name        string
		count       int
		base, edits map[string]string
		whole       []string
	}{
		{"one package alone", 1, nil, bulkTestEdits(1), nil},
		{"three packages and a source change", 3, nil, bulkMerge(bulkTestEdits(3), map[string]string{"z/z.go": "package z\n\nfunc Z() int { return 2 }\n"}), nil},
		{"a dependency change", 3, nil, bulkMerge(bulkTestEdits(3), map[string]string{"a/a.go": "package a\n\nfunc A() int { return 2 }\n"}), []string{"p0"}},
		{"a reader of another package's test file", 3,
			map[string]string{"p1/p1.go": "package p1\n\nconst fixture = \"../p2/p2_test.go\"\n\nfunc P() int { return len(fixture) }\n"},
			bulkTestEdits(3), []string{"p1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, base := bulkSliceFixture(t, tc.count, tc.base, tc.edits)
			plan, graph, sliced, _, _ := bulkSlices(t, root, base)
			legacy := legacyReachedElsewhere(plan, graph)
			if len(legacy) != len(tc.whole) {
				t.Fatalf("legacy reasons %v, want whole %v", legacy, tc.whole)
			}
			for i := range tc.count {
				pkg := fmt.Sprintf("p%d", i)
				got := sliced["example.test/m/"+pkg]
				if reason, whole := legacy["example.test/m/"+pkg]; whole {
					if got != (sessiontest.GoSlice{Reason: reason}) {
						t.Fatalf("%s sliced %+v, legacy %q", pkg, got, reason)
					}
				} else if got.Pattern != "^(TestP)$" {
					t.Fatalf("%s sliced %+v, legacy sliced it", pkg, got)
				}
			}
		})
	}
}

func bulkMerge(maps ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, m := range maps {
		for key, value := range m {
			merged[key] = value
		}
	}
	return merged
}
