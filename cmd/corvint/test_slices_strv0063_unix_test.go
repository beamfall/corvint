//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// sliceFixture is a two-package module: package a's helpers_test.go uses a
// helper a_test.go declares; package b's two test files share nothing.
func sliceFixture(t *testing.T) (root, base string) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27\n",
		"a/a.go":                    "package a\n\nfunc A() int { return 1 }\n",
		"a/a_test.go":               "package a\n\nimport \"testing\"\n\nfunc fixtureValue() int { return 1 }\n\nfunc TestA(t *testing.T) { _ = A() }\n",
		"a/helpers_test.go":         "package a\n\nimport \"testing\"\n\nfunc TestHelped(t *testing.T) { _ = fixtureValue() }\n",
		"b/b.go":                    "package b\n\nimport \"example.test/m/a\"\n\nfunc B() int { return a.A() + 1 }\n",
		"b/b_test.go":               "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B() }\n",
		"b/b_more_test.go":          "package b\n\nimport \"testing\"\n\nfunc TestBMore(t *testing.T) { _ = B() }\n\nfunc ExampleB() { _ = B() }\n",
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
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(sessionFakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SESSION_FAKE_LOG", filepath.Join(t.TempDir(), "runners.log"))
	t.Setenv("SESSION_FAKE_MODE", "pass")
	return root, base
}

// STR-V0-063: a change confined to a package's own test files runs only the
// test functions of those files, with the -run pattern and the reason in the
// manifest; every missing proof keeps the package whole with its reason.
func TestSTRV0063DryRunSlicesTestOnlyChanges(t *testing.T) {
	root, base := sliceFixture(t)
	type invocation struct {
		Argv  []string           `json:"argv"`
		Units []sessiontest.Unit `json:"units"`
	}
	plan := func(edits map[string]string) (string, []invocation) {
		t.Helper()
		affectedGit(t, root, "checkout", "--", ".")
		for name, body := range edits {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		manifest := filepath.Join(t.TempDir(), "manifest.json")
		code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", manifest)
		if code != 0 {
			t.Fatalf("code %d stderr %q", code, stderr)
		}
		var printed struct {
			Invocations []invocation `json:"invocations"`
		}
		if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
			t.Fatal(err)
		}
		return stderr, printed.Invocations
	}
	unit := func(invocations []invocation, id string) sessiontest.Unit {
		t.Helper()
		for _, invocation := range invocations {
			for _, unit := range invocation.Units {
				if unit.Unit == id {
					return unit
				}
			}
		}
		t.Fatalf("%s is not planned: %+v", id, invocations)
		return sessiontest.Unit{}
	}
	more := "package b\n\nimport \"testing\"\n\nfunc TestBMore(t *testing.T) { _ = B() + 1 }\n\nfunc ExampleB() { _ = B() }\n"

	stderr, invocations := plan(map[string]string{"b/b_more_test.go": more})
	if stderr != "corvint-test: NARROWED 1 execution units\ncorvint-test: SLICED 1 of them to the test functions of their changed test files\n" {
		t.Fatalf("stderr %q", stderr)
	}
	got := unit(invocations, "go:example.test/m/b")
	want := sessiontest.Unit{Adapter: sessiontest.AdapterGo, Flag: "-run", Pattern: "^(ExampleB|TestBMore)$", Reason: "selected: the tests of b/b_more_test.go", Slice: sessiontest.SliceSelected, Unit: "go:example.test/m/b"}
	if got != want {
		t.Fatalf("sliced unit %+v, want %+v", got, want)
	}
	if len(invocations) != 1 || strings.Join(invocations[0].Argv, " ") != "go test -json -p 1 -count=1 -run ^(ExampleB|TestBMore)$ example.test/m/b" {
		t.Fatalf("argv %+v", invocations)
	}

	wholeCases := []struct {
		name   string
		edits  map[string]string
		id     string
		reason string
	}{
		{"a shared helper", map[string]string{"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc fixtureValue() int { return 2 }\n\nfunc TestA(t *testing.T) { _ = A() }\n"},
			"go:example.test/m/a", "whole: a/helpers_test.go uses fixtureValue declared in a/a_test.go"},
		{"a source change beside the test", map[string]string{"b/b_more_test.go": more, "b/b.go": "package b\n\nimport \"example.test/m/a\"\n\nfunc B() int { return a.A() + 2 }\n"},
			"go:example.test/m/b", "whole: DIRECT_SOURCE_CHANGE via b/b.go"},
		{"a dependency change", map[string]string{"b/b_more_test.go": more, "a/a.go": "package a\n\nfunc A() int { return 3 }\n"},
			"go:example.test/m/b", "a/a.go"},
		{"an init function", map[string]string{"b/b_more_test.go": "package b\n\nimport \"testing\"\n\nfunc init() { _ = B() }\n\nfunc TestBMore(t *testing.T) { _ = B() }\n"},
			"go:example.test/m/b", "whole: b/b_more_test.go declares init"},
		{"a package variable initialized by a call", map[string]string{"b/b_more_test.go": "package b\n\nimport \"testing\"\n\nvar shared = B()\n\nfunc TestBMore(t *testing.T) { _ = shared }\n"},
			"go:example.test/m/b", "whole: b/b_more_test.go initializes shared with a call"},
		{"no test function", map[string]string{"b/b_more_test.go": "package b\n\nfunc unused() {}\n"},
			"go:example.test/m/b", "whole: the changed test files declare no test function"},
		{"a file that does not parse", map[string]string{"b/b_more_test.go": "package b\n\nfunc TestBMore(\n"},
			"go:example.test/m/b", "whole: b/b_more_test.go does not parse"},
	}
	for _, tc := range wholeCases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, invocations := plan(tc.edits)
			if strings.Contains(stderr, "SLICED") {
				t.Fatalf("stderr %q", stderr)
			}
			got := unit(invocations, tc.id)
			if got.Slice != sessiontest.SliceWhole || got.Flag != "" || got.Pattern != "" || !strings.HasPrefix(got.Reason, "whole: ") || !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("unit %+v, want whole with reason %q", got, tc.reason)
			}
		})
	}
}

// STR-V0-063: an initialization effect the change removes is still a proof
// gap. Deleting init, TestMain or a call-initialized package variable from a
// changed test file, or deleting a test file that had one, changes what every
// other test of the package observed at base, so the package runs whole.
func TestSTRV0063RemovedInitializationKeepsPackageWhole(t *testing.T) {
	root, _ := sliceFixture(t)
	withInit := "package b\n\nimport \"testing\"\n\nfunc init() { _ = B() }\n\nfunc TestBMore(t *testing.T) { _ = B() }\n"
	withVar := "package b\n\nimport \"testing\"\n\nvar seeded = B()\n\nfunc TestBMore(t *testing.T) { _ = seeded }\n"
	edited := "package b\n\nimport \"testing\"\n\nfunc TestBMore(t *testing.T) { _ = B() + 1 }\n"
	cases := []struct {
		name, base, reason string
		remove             bool
	}{
		{"an init function removed", withInit, "whole: b/b_more_test.go declares init at base", false},
		{"a call-initialized variable removed", withVar, "whole: b/b_more_test.go initializes seeded with a call at base", false},
		{"a test file with init deleted", withInit, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			affectedGit(t, root, "checkout", "-q", "--", ".")
			if err := os.WriteFile(filepath.Join(root, "b/b_more_test.go"), []byte(tc.base), 0o644); err != nil {
				t.Fatal(err)
			}
			affectedGit(t, root, "commit", "-qam", tc.name)
			base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
			if tc.remove {
				if err := os.Remove(filepath.Join(root, "b/b_more_test.go")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "b/b_test.go"), []byte("package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B() + 1 }\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(root, "b/b_more_test.go"), []byte(edited), 0o644); err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(t.TempDir(), "manifest.json")
			code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", manifest)
			if code != 0 {
				t.Fatalf("code %d stderr %q", code, stderr)
			}
			if strings.Contains(stderr, "SLICED") {
				t.Fatalf("stderr %q stdout %s", stderr, stdout)
			}
			var printed struct {
				Invocations []struct {
					Units []sessiontest.Unit `json:"units"`
				} `json:"invocations"`
			}
			if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
				t.Fatal(err)
			}
			for _, invocation := range printed.Invocations {
				for _, unit := range invocation.Units {
					if unit.Unit == "go:example.test/m/b" {
						if unit.Slice != sessiontest.SliceWhole || unit.Pattern != "" || !strings.Contains(unit.Reason, tc.reason) {
							t.Fatalf("unit %+v, want whole with reason %q", unit, tc.reason)
						}
						return
					}
				}
			}
			t.Fatalf("package b is not planned: %s", stdout)
		})
	}
}
