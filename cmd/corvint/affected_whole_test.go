package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestAFPV0550TestMainChangeMakesThePackageWhole pins V1-3688: a dirty test
// file that adds, changes or removes TestMain makes its unit whole with that
// reason, and an edit beside an unchanged TestMain leaves no verdict.
func TestAFPV0550TestMainChangeMakesThePackageWhole(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const head = "package p\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n"
	const main = "func TestMain(m *testing.M) { os.Exit(m.Run()) }\n\n"
	write("added/a_test.go", head+"var _ = os.Args\n\nfunc TestA(t *testing.T) {}\n")
	write("kept/k_test.go", head+main+"func TestK(t *testing.T) {}\n")
	write("changed/c_test.go", head+main+"func TestC(t *testing.T) {}\n")
	write("removed/r_test.go", head+main+"func TestR(t *testing.T) {}\n")
	materializationGit(t, root, "init", "-q")
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "base")
	base := strings.TrimSpace(materializationGit(t, root, "rev-parse", "HEAD"))

	write("added/a_test.go", head+main+"func TestA(t *testing.T) {}\n")
	write("kept/k_test.go", head+main+"func TestK(t *testing.T) { t.Log(1) }\n")
	write("changed/c_test.go", head+"func TestMain(m *testing.M) { code := m.Run(); os.Exit(code) }\n\nfunc TestC(t *testing.T) {}\n")
	write("removed/r_test.go", head+"var _ = os.Args\n\nfunc TestR(t *testing.T) {}\n")
	plan := affected.Plan{Dirty: []string{"added/a_test.go", "changed/c_test.go", "kept/k_test.go", "removed/r_test.go"}}
	for _, dir := range []string{"added", "changed", "kept", "removed"} {
		plan.Selected = append(plan.Selected, affected.Selection{UnitID: "go:example/" + dir, Tests: []string{dir + "/" + dir[:1] + "_test.go"}})
	}
	advice := withWholePackages(context.Background(), affectedAdvice{}, root, root, base, "", plan)
	got := map[string]string{}
	for _, whole := range advice.WholePackages {
		got[whole.Unit] = whole.Reason
	}
	want := map[string]string{
		"go:example/added":   "added/a_test.go adds TestMain",
		"go:example/changed": "changed/c_test.go changes TestMain",
		"go:example/removed": "removed/r_test.go removes TestMain",
	}
	if len(got) != len(want) {
		t.Fatalf("whole packages = %v; want the three units of %v", got, want)
	}
	for unit, prefix := range want {
		if !strings.HasPrefix(got[unit], prefix) {
			t.Errorf("%s reason = %q; want prefix %q", unit, got[unit], prefix)
		}
	}
	if none := withWholePackages(context.Background(), affectedAdvice{}, root, root, base, "", affected.Plan{Dirty: []string{"kept/k_test.go"}, Selected: plan.Selected[2:3]}); len(none.WholePackages) != 0 {
		t.Errorf("an edit beside an unchanged TestMain made the unit whole: %v", none.WholePackages)
	}
}
