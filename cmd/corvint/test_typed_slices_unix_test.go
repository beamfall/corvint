//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TestAFPV02370DryRunNarrowsAnAllowListedDependent drives `corvint test
// --dry-run` over a module whose package p tests lib through two test
// functions: an edit of lib.A narrows p's DEPENDENCY_PATH selection to TestA,
// the negative control (an edit of lib.B) to TestB, an added init keeps p
// whole with the typed-reach reason, and a package off the allow-list stays
// whole without one (AFP-V0-2370, AFP-V0-2372, AFP-V0-2375).
func TestAFPV02370DryRunNarrowsAnAllowListedDependent(t *testing.T) {
	saved := typedReachPackages
	typedReachPackages = map[string]bool{"p": true}
	t.Cleanup(func() { typedReachPackages = saved })
	root := t.TempDir()
	lib := "package lib\n\nfunc A() int { return 1 }\n\nfunc B() int { return 2 }\n"
	files := map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27\n",
		"lib/lib.go":                lib,
		"p/p.go":                    "package p\n",
		"p/p_test.go":               "package p\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestA(t *testing.T) { _ = lib.A() }\n\nfunc TestB(t *testing.T) { _ = lib.B() }\n",
		"o/o.go":                    "package o\n",
		"o/o_test.go":               "package o\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestO(t *testing.T) { _ = lib.A() }\n",
		sessiontest.DeclarationPath: sessionDeclaration(""),
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
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	bin := t.TempDir()
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(sessionFakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SESSION_FAKE_LOG", filepath.Join(t.TempDir(), "runners.log"))
	t.Setenv("SESSION_FAKE_MODE", "pass")
	plan := func(t *testing.T, libSource string) map[string]sessiontest.Unit {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "lib/lib.go"), []byte(libSource), 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", filepath.Join(t.TempDir(), "manifest.json"))
		if code != 0 {
			t.Fatalf("code %d stderr %q", code, stderr)
		}
		var printed struct {
			Invocations []struct {
				Units []sessiontest.Unit `json:"units"`
			} `json:"invocations"`
		}
		if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
			t.Fatal(err)
		}
		units := map[string]sessiontest.Unit{}
		for _, invocation := range printed.Invocations {
			for _, unit := range invocation.Units {
				units[unit.Unit] = unit
			}
		}
		return units
	}
	if _, ok := plan(t, strings.Replace(lib, "return 1", "return 3", 1))["go:example.test/m/o"]; !ok {
		t.Fatal("o, which calls A, is not selected for an edit of A")
	}
	for _, tc := range []struct{ name, edit, pattern string }{
		{"edit of A", strings.Replace(lib, "return 1", "return 3", 1), "^(TestA)$"},
		{"negative control: edit of B", strings.Replace(lib, "return 2", "return 4", 1), "^(TestB)$"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			units := plan(t, tc.edit)
			got := units["go:example.test/m/p"]
			if got.Slice != sessiontest.SliceSelected || got.Pattern != tc.pattern || !strings.Contains(got.Reason, "(typed reach: 1 of 2 test functions cannot reach it)") || got.ReasonCode != sessiontest.ReasonSelectedReachingChange {
				t.Fatalf("p %+v, want pattern %s", got, tc.pattern)
			}
			// Symbol-level narrowing (ADN-V0-660) leaves o out when only B
			// changes; when o is selected it stays whole.
			if other, ok := units["go:example.test/m/o"]; ok && (other.Slice != sessiontest.SliceWhole || strings.Contains(other.Reason, "typed reach")) {
				t.Fatalf("o off the allow-list %+v, want whole without typed reach", other)
			}
		})
	}
	t.Run("an added init keeps p whole", func(t *testing.T) {
		got := plan(t, lib+"\nvar hits int\n\nfunc init() { hits++ }\n")["go:example.test/m/p"]
		if got.Slice != sessiontest.SliceWhole || !strings.Contains(got.Reason, "(typed reach: ALWAYS_RUN_REACHES_CHANGE: ") {
			t.Fatalf("p %+v, want whole with the always-run reason", got)
		}
	})
}
