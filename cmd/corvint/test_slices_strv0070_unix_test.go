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

// sliceRebase resets to sliceFixture's commit, commits base over it, applies
// edits on top and returns the planned unit of package b with the dry run's
// stderr; an empty edit deletes the file.
func sliceRebase(t *testing.T, root, fixture string, base, edits map[string]string) (string, sessiontest.Unit) {
	t.Helper()
	affectedGit(t, root, "reset", "-q", "--hard", fixture)
	for name, body := range base {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(base) != 0 {
		affectedGit(t, root, "commit", "-qam", "rebase "+t.Name())
	}
	revision := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for name, body := range edits {
		if body == "" {
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", revision, "--dry-run", "--manifest", manifest)
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
	for _, invocation := range printed.Invocations {
		for _, unit := range invocation.Units {
			if unit.Unit == "go:example.test/m/b" {
				return stderr, unit
			}
		}
	}
	t.Fatalf("package b is not planned: %s", stdout)
	return "", sessiontest.Unit{}
}

func requireWhole(t *testing.T, stderr string, unit sessiontest.Unit, reason string) {
	t.Helper()
	if strings.Contains(stderr, "SLICED") || unit.Slice != sessiontest.SliceWhole || unit.Pattern != "" || unit.Reason != reason {
		t.Fatalf("stderr %q unit %+v, want whole with reason %q", stderr, unit, reason)
	}
}

// STR-V0-070 (V1-2998): an unchanged test function that calls a changed
// test function of the same package runs the edit too, so the package runs
// whole and the reason names both the caller's file and the changed test.
func TestSTRV0070WrapperTestKeepsPackageWhole(t *testing.T) {
	root, fixture := sliceFixture(t)
	wrapper := "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B(); TestBMore(t) }\n"
	edited := "package b\n\nimport \"testing\"\n\nfunc TestBMore(t *testing.T) { _ = B() + 1 }\n\nfunc ExampleB() { _ = B() }\n"
	stderr, unit := sliceRebase(t, root, fixture, map[string]string{"b/b_test.go": wrapper}, map[string]string{"b/b_more_test.go": edited})
	requireWhole(t, stderr, unit, "whole: b/b_test.go uses TestBMore declared in b/b_more_test.go")
}

// STR-V0-071 (V1-3000): an import a changed test file adds or removes
// against every test file of the package, or a blank or dot import now or at
// base, can change the test binary's initialization, so the package runs
// whole with the reason; an import another test file already has slices.
func TestSTRV0071ImportChangesKeepPackageWhole(t *testing.T) {
	root, fixture := sliceFixture(t)
	plain := "package b\n\nimport \"testing\"\n\nfunc TestBMore(t *testing.T) { _ = B() + 1 }\n"
	blank := "package b\n\nimport (\n\t\"testing\"\n\n\t_ \"example.test/m/a\"\n)\n\nfunc TestBMore(t *testing.T) { _ = B() }\n"
	dot := "package b\n\nimport (\n\t\"testing\"\n\n\t. \"example.test/m/a\"\n)\n\nfunc TestBMore(t *testing.T) { _ = A() }\n"
	withStrings := "package b\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc TestBMore(t *testing.T) { _ = strings.ToUpper(\"\") }\n"
	sharedStrings := "package b\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc TestB(t *testing.T) { _ = strings.ToLower(\"\") }\n"
	cases := []struct {
		name        string
		base, edits map[string]string
		reason      string
	}{
		{"a blank import removed", map[string]string{"b/b_more_test.go": blank}, map[string]string{"b/b_more_test.go": plain},
			"whole: b/b_more_test.go imports example.test/m/a for its side effects at base"},
		{"a blank import added", nil, map[string]string{"b/b_more_test.go": blank},
			"whole: b/b_more_test.go imports example.test/m/a for its side effects"},
		{"a dot import added", nil, map[string]string{"b/b_more_test.go": dot},
			"whole: b/b_more_test.go dot-imports example.test/m/a"},
		{"a new import path added", nil, map[string]string{"b/b_more_test.go": withStrings},
			"whole: b/b_more_test.go imports strings, which no test file of the package imported at base"},
		{"an import path removed", map[string]string{"b/b_more_test.go": withStrings}, map[string]string{"b/b_more_test.go": plain},
			"whole: b/b_more_test.go imported strings at base, which no test file of the package imports now"},
		{"a test file with an import deleted", map[string]string{"b/b_more_test.go": withStrings}, map[string]string{"b/b_more_test.go": "", "b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B() + 1 }\n"},
			""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, unit := sliceRebase(t, root, fixture, tc.base, tc.edits)
			if tc.reason == "" {
				if unit.Slice != sessiontest.SliceWhole || unit.Pattern != "" {
					t.Fatalf("stderr %q unit %+v, want whole", stderr, unit)
				}
				return
			}
			requireWhole(t, stderr, unit, tc.reason)
		})
	}
	t.Run("an import another test file has", func(t *testing.T) {
		stderr, unit := sliceRebase(t, root, fixture, map[string]string{"b/b_test.go": sharedStrings}, map[string]string{"b/b_more_test.go": withStrings})
		if unit.Slice != sessiontest.SliceSelected || unit.Pattern != "^(TestBMore)$" {
			t.Fatalf("stderr %q unit %+v, want sliced to TestBMore", stderr, unit)
		}
	})
}
