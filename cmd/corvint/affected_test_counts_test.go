// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-9451: count runner entries, not files, helpers, benchmarks or TestMain.
func TestRunnableGoTestCounts_AFPV09451(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"several tests", `package p; import "testing"; func TestOne(t *testing.T) {}; func TestTwo(t *testing.T) {}; func Testlower(t *testing.T) {}; func BenchmarkOne(b *testing.B) {}; func TestMain(m *testing.M) {}`, 2},
		{"empty file", "package p", 0},
		{"fuzz seeds", `package p; import "testing"; func FuzzInput(f *testing.F) {}; func Fuzzlower(f *testing.F) {}`, 1},
		{"examples", "package p\nfunc Example() {\n// Output: value\n}\nfunc Example_empty() {\n// Output:\n}\nfunc Example_noOutput() {}\n", 2},
		{"unicode and test main T", `package p; import "testing"; func TestÉ(t *testing.T) {}; func Testé(t *testing.T) {}; func TestMain(t *testing.T) {}`, 2},
		{"methods", `package p; type T int; func (T) TestMethod() {}`, 0},
		{"broken syntax", "package p; func TestBroken(", -1},
		{"invalid test signature", "package p; func TestBroken() {}", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runnableGoTests("p_test.go", []byte(tc.body)); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

// AFP-V0-9451: the actual compact CLI projection carries multiple functions
// per file; the full file list is unchanged. Counts are captured before projection.
func TestAffectedFunctionCounts_AFPV09451(t *testing.T) {
	root := affectedFixtureRepository(t)
	appendFile(t, filepath.Join(root, "leaf", "leaf_test.go"), "\nfunc TestSecond(t *testing.T) {}\nfunc FuzzValue(f *testing.F) {}\n")
	compact, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	selected := compact["plan"].(map[string]any)["selected"].([]any)
	found := false
	for _, raw := range selected {
		item := raw.(map[string]any)
		if item["unitId"] == "go:example.com/fixture/leaf" {
			found = true
			if item["testCount"] != float64(3) {
				t.Fatalf("count=%v", item["testCount"])
			}
		}
	}
	if !found {
		t.Fatalf("leaf absent: %v", selected)
	}
	full, _, stderr, code := runAffectedCLI(t, root, "--full")
	if code != 0 {
		t.Fatalf("full exit %d: %s", code, stderr)
	}
	for _, raw := range full["plan"].(map[string]any)["selected"].([]any) {
		item := raw.(map[string]any)
		if item["unitId"] == "go:example.com/fixture/leaf" && len(item["tests"].([]any)) != 1 {
			t.Fatalf("full test-file list changed: %v", item)
		}
	}
}

// AFP-V0-9451: unavailable files stay explicitly unknown; a legitimate empty
// test file has zero entries. A captured count survives source removal.
func TestAffectedCapturedCounts_AFPV09451(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "p_test.go"), []byte("package p"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := affected.Plan{Selected: []affected.Selection{{UnitID: "go:p", Tests: []string{"p_test.go"}}}}
	counts := affectedTestCounts(root, plan)
	if counts["go:p"] != 0 {
		t.Fatal(counts)
	}
	if err := os.Remove(filepath.Join(root, "p_test.go")); err != nil {
		t.Fatal(err)
	}
	if got := projectedTestCount(counts, plan.Selected[0]); got != 0 {
		t.Fatal(got)
	}
	if got := affectedTestCounts(root, plan)["go:p"]; got != -1 {
		t.Fatal(got)
	}
}

// AFP-V0-9451: by-path shared and per-path rows use the same captured counts.
func TestAffectedByPathFunctionCounts_AFPV09451(t *testing.T) {
	graph, err := affected.Build(context.Background(), t.TempDir(), byPathLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"docs/a.md", "docs/b.md"}
	plan := affected.Select(graph, paths)
	counts := map[string]int{"go:any": 7, "go:a": 3, "go:b": 11}
	result := affectedByPath(affectedReceipt{Plan: plan, TestCounts: counts}, graph, paths)
	if len(result.Shared) != 1 || result.Shared[0].TestCount != 7 {
		t.Fatal(result.Shared)
	}
	for _, entry := range result.Paths {
		for _, added := range entry.Additions {
			if added.TestCount != counts[added.UnitID] {
				t.Fatal(added)
			}
		}
	}
}
