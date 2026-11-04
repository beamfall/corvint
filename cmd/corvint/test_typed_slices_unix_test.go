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
		"p/p_test.go":               "package p\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/ed\"\n\t\"example.test/m/lib\"\n)\n\nfunc TestA(t *testing.T) { _ = lib.A() }\n\nfunc TestB(t *testing.T) { _ = lib.B() }\n\nfunc TestEmbed(t *testing.T) { _ = ed.Read() }\n",
		"ed/ed.go":                  "package ed\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar data string\n\nfunc Read() string { return data }\n",
		"ed/data.txt":               "one\n",
		"lib/notes.txt":             "read at run time\n",
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
	planFiles := func(t *testing.T, edits map[string]string) map[string]sessiontest.Unit {
		t.Helper()
		affectedGit(t, root, "checkout", "-q", "--", ".")
		for name, body := range edits {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
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
	plan := func(t *testing.T, libSource string) map[string]sessiontest.Unit {
		t.Helper()
		return planFiles(t, map[string]string{"lib/lib.go": libSource})
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
			if got.Slice != sessiontest.SliceSelected || got.Pattern != tc.pattern || !strings.Contains(got.Reason, "(typed reach: 2 of 3 test functions cannot reach it)") || got.ReasonCode != sessiontest.ReasonSelectedReachingChange {
				t.Fatalf("p %+v, want pattern %s", got, tc.pattern)
			}
			// Symbol-level narrowing (ADN-V0-660) leaves o out when only B
			// changes; when o is selected it stays whole.
			if other, ok := units["go:example.test/m/o"]; ok && (other.Slice != sessiontest.SliceWhole || strings.Contains(other.Reason, "typed reach")) {
				t.Fatalf("o off the allow-list %+v, want whole without typed reach", other)
			}
		})
	}
	// AFP-V0-2377: a data file is not Go source, so no declaration shows its
	// change; an embedded one and one beside a package's source keep every
	// selection of a dependent whole, and never slice it.
	for _, tc := range []struct{ name, file, reason string }{
		{"an embedded data file", "ed/data.txt", "may be embedded by the package in ed"},
		{"a data file beside a package", "lib/notes.txt", "is a data file under the Go package directory lib"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edits := map[string]string{tc.file: "two\n"}
			if tc.file == "lib/notes.txt" {
				// Ensure the dependent is selected, even when the data file
				// has no literal reader in this fixture. Mixed code/data edits
				// must preserve the whole-package dependency selection.
				edits["lib/lib.go"] = strings.Replace(lib, "return 1", "return 3", 1)
			}
			units := planFiles(t, edits)
			got, ok := units["go:example.test/m/p"]
			if !ok {
				t.Fatal("p is not selected for the dependency change")
			}
			if got.Slice != sessiontest.SliceWhole || !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("p %+v, want whole naming %q", got, tc.reason)
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
