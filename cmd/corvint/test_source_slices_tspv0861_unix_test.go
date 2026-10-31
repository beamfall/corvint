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

// TSP-V0-861: a package whose own plain functions and test files both changed
// runs the tests reaching the changed functions plus the test functions of the
// changed test files, each under its own proof; a failed proof on either side,
// or a union holding every test, keeps it whole with the proof named.
func TestTSPV0861DryRunSlicesSourceAndTestChanges(t *testing.T) {
	root, base := sourceSliceFixture(t)
	original, err := os.ReadFile(filepath.Join(root, "p/p.go"))
	if err != nil {
		t.Fatal(err)
	}
	pSource := string(original)
	edit := func(old, replacement string) string {
		t.Helper()
		if !strings.Contains(pSource, old) {
			t.Fatalf("fixture lacks %q", old)
		}
		return strings.Replace(pSource, old, replacement, 1)
	}
	upper := edit("strings.ToUpper(s) +", "strings.ToTitle(s) +")
	plan := func(t *testing.T, edits map[string]string) (string, map[string]sessiontest.Unit) {
		t.Helper()
		affectedGit(t, root, "checkout", "-q", "--", ".")
		affectedGit(t, root, "clean", "-qfd")
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
			Invocations []struct {
				Units []sessiontest.Unit `json:"units"`
			} `json:"invocations"`
		}
		if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
			t.Fatal(err)
		}
		units := make(map[string]sessiontest.Unit)
		for _, invocation := range printed.Invocations {
			for _, unit := range invocation.Units {
				units[unit.Unit] = unit
			}
		}
		return stderr, units
	}
	const internalTest = "package p\n\nimport \"testing\"\n\n"

	sliced := []struct {
		name, id, pattern, reason string
		edits                     map[string]string
	}{
		{"a new test file beside a called function", "go:example.test/m/p", "^(TestExternal|TestUpper|TestUpperAgain)$",
			"selected: the tests reaching Upper changed in p/p.go and the tests of p/upper_test.go",
			map[string]string{"p/p.go": upper, "p/upper_test.go": internalTest + "func TestUpperAgain(t *testing.T) { _ = Upper(\"z\") }\n"}},
		{"an edited external test file", "go:example.test/m/p", "^(TestExternal|TestUpper)$",
			"selected: the tests reaching Upper changed in p/p.go and the tests of p/ext_test.go",
			map[string]string{"p/p.go": upper, "p/ext_test.go": "package p_test\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/p\"\n)\n\nfunc TestExternal(t *testing.T) { _ = p.Lower(\"Y\") }\n"}},
		{"a function no test reached and its new test", "go:example.test/m/r", "^(TestS)$",
			"selected: the tests reaching S changed in r/r.go and the tests of r/s_test.go",
			map[string]string{"r/r.go": "package r\n\nfunc R() int { return 1 }\n\nfunc S() int { return 3 }\n",
				"r/s_test.go": "package r\n\nimport \"testing\"\n\nfunc TestS(t *testing.T) { _ = S() }\n"}},
		{"a string and prose naming the linkname directive", "go:example.test/m/p", "^(TestExternal|TestUpper)$",
			"selected: the tests reaching Upper changed in p/p.go",
			map[string]string{"p/p.go": edit("{ return strings.ToUpper(s) +", "{\n\t// The slicer refuses a go:linkname directive.\n\t_ = \"//go:linkname\"\n\treturn strings.ToTitle(s) +")}},
	}
	for _, tc := range sliced {
		t.Run(tc.name, func(t *testing.T) {
			stderr, units := plan(t, tc.edits)
			got := units[tc.id]
			want := sessiontest.Unit{Adapter: sessiontest.AdapterGo, Flag: "-run", Pattern: tc.pattern, Reason: tc.reason, Slice: sessiontest.SliceSelected, Unit: tc.id}
			if got != want {
				t.Fatalf("unit %+v, want %+v (stderr %q)", got, want, stderr)
			}
			if dependent, ok := units["go:example.test/m/q"]; ok && dependent.Slice != sessiontest.SliceWhole {
				t.Fatalf("dependent %+v, want whole", dependent)
			}
		})
	}

	whole := []struct {
		name, id, reason string
		edits            map[string]string
	}{
		// p_test.go changed, so all its tests run, and with TestExternal that is every test.
		{"every test kept", "go:example.test/m/p", "every test function of the package reaches Upper changed in p/p.go and the tests of p/p_test.go",
			map[string]string{"p/p.go": upper, "p/p_test.go": strings.Replace(string(mustRead(t, filepath.Join(root, "p/p_test.go"))), "Upper(\"a\")", "Upper(\"b\")", 1)}},
		{"a TestMain", "go:example.test/m/p", "(p/upper_test.go declares TestMain)",
			map[string]string{"p/p.go": upper, "p/upper_test.go": internalTest + "func TestMain(m *testing.M) { m.Run() }\n\nfunc TestUpperAgain(t *testing.T) {}\n"}},
		{"an init", "go:example.test/m/p", "(p/upper_test.go declares init)",
			map[string]string{"p/p.go": upper, "p/upper_test.go": internalTest + "func init() {}\n\nfunc TestUpperAgain(t *testing.T) {}\n"}},
		{"a new import", "go:example.test/m/p", "(p/upper_test.go imports strings, which no test file of the package imported at base)",
			map[string]string{"p/p.go": upper, "p/upper_test.go": "package p\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc TestUpperAgain(t *testing.T) { _ = strings.ToUpper(Upper(\"z\")) }\n"}},
		{"a source proof that fails", "go:example.test/m/p", "(Bump reaches the package variable counter)",
			map[string]string{"p/p.go": edit("counter++; return counter", "counter += 2; return counter"), "p/upper_test.go": internalTest + "func TestUpperAgain(t *testing.T) {}\n"}},
		{"a linkname directive", "go:example.test/m/p", "(p/link.go names go:linkname)",
			map[string]string{"p/p.go": upper, "p/link.go": "package p\n\nimport _ \"unsafe\"\n\n  //go:linkname now time.now\nfunc now() (int64, int32, int64)\n"}},
		{"no test function", "go:example.test/m/p", "(the changed test files declare no test function)",
			map[string]string{"p/p.go": upper, "p/upper_test.go": internalTest + "func helper() int { return 1 }\n"}},
	}
	for _, tc := range whole {
		t.Run(tc.name, func(t *testing.T) {
			stderr, units := plan(t, tc.edits)
			got := units[tc.id]
			if got.Slice != sessiontest.SliceWhole || got.Pattern != "" || !strings.HasPrefix(got.Reason, "whole: DIRECT_SOURCE_CHANGE via ") || !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("unit %+v, want whole with reason %q (stderr %q)", got, tc.reason, stderr)
			}
		})
	}
}

// TSP-V0-864: a changed file whose residue differs only by comments (directives
// unchanged) or by whole new const and type declarations is sliced to the tests
// reaching its changed functions and new names; a new var, import or method, a
// changed directive or a changed declaration keeps it whole.
func TestTSPV0864DryRunSlicesAdditiveDeclarations(t *testing.T) {
	root, base := sourceSliceFixture(t)
	original, err := os.ReadFile(filepath.Join(root, "r/r.go"))
	if err != nil {
		t.Fatal(err)
	}
	r := string(original)
	plan := func(t *testing.T, edits map[string]string) map[string]sessiontest.Unit {
		t.Helper()
		affectedGit(t, root, "checkout", "-q", "--", ".")
		affectedGit(t, root, "clean", "-qfd")
		for name, body := range edits {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", filepath.Join(t.TempDir(), "m.json"))
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
		units := make(map[string]sessiontest.Unit)
		for _, invocation := range printed.Invocations {
			for _, unit := range invocation.Units {
				units[unit.Unit] = unit
			}
		}
		return units
	}
	const id = "go:example.test/m/r"
	// r has R (reached by TestR) and S (reached by no test); TestLimit is a new test of a new const.
	limitTest := "package r\n\nimport \"testing\"\n\nfunc TestLimit(t *testing.T) { _ = Limit }\n"
	for _, tc := range []struct {
		name, pattern, reason string
		edits                 map[string]string
	}{
		{"a file comment", "^$", "selected: no test reaches comments outside declarations changed in r/r.go; build and vet only",
			map[string]string{"r/r.go": "// Package r holds R and S.\n" + r}},
		{"a new const no test names", "^$", "selected: no test reaches Limit changed in r/r.go; build and vet only",
			map[string]string{"r/r.go": r + "\nconst Limit = 3\n"}},
		{"a new const and a function using it", "^$", "selected: no test reaches Limit, S changed in r/r.go; build and vet only",
			map[string]string{"r/r.go": strings.Replace(r, "return 2", "return Limit", 1) + "\nconst Limit = 2\n"}},
		{"a new type and its new test", "^(TestLimit)$", "selected: the tests reaching Limit, Unit changed in r/r.go and the tests of r/limit_test.go",
			map[string]string{"r/r.go": r + "\ntype Unit int\n\nconst Limit Unit = 3\n", "r/limit_test.go": limitTest}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := plan(t, tc.edits)[id]
			want := sessiontest.Unit{Adapter: sessiontest.AdapterGo, Flag: "-run", Pattern: tc.pattern, Reason: tc.reason, Slice: sessiontest.SliceSelected, Unit: id}
			if got != want {
				t.Fatalf("unit %+v, want %+v", got, want)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		edits map[string]string
	}{
		{"a new var", map[string]string{"r/r.go": r + "\nvar limit = 3\n"}},
		{"a new import", map[string]string{"r/r.go": strings.Replace(r, "package r\n", "package r\n\nimport _ \"strings\"\n", 1)}},
		{"a new method on a new type", map[string]string{"r/r.go": r + "\ntype Unit int\n\nfunc (u Unit) Twice() Unit { return u * 2 }\n"}},
		{"a new directive", map[string]string{"r/r.go": r + "\n//go:generate echo r\n"}},
		{"a changed declaration", map[string]string{"r/r.go": r + "\nconst Limit = 3\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edits := tc.edits
			if tc.name == "a changed declaration" {
				// Base gains the const first, then the change alters its value.
				write := func(body string) {
					if err := os.WriteFile(filepath.Join(root, "r/r.go"), []byte(body), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				affectedGit(t, root, "checkout", "-q", "--", ".")
				write(r + "\nconst Limit = 3\n")
				affectedGit(t, root, "commit", "-qam", "limit")
				defer func() { affectedGit(t, root, "reset", "-q", "--hard", base) }()
				newBase := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
				write(r + "\nconst Limit = 4\n")
				code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", newBase, "--dry-run", "--manifest", filepath.Join(t.TempDir(), "m.json"))
				if code != 0 || !strings.Contains(stdout, "r/r.go changes more than plain functions") {
					t.Fatalf("code %d stdout %q stderr %q", code, stdout, stderr)
				}
				return
			}
			got := plan(t, edits)[id]
			if got.Slice != sessiontest.SliceWhole || !strings.Contains(got.Reason, "changes more than plain functions") {
				t.Fatalf("unit %+v, want whole", got)
			}
		})
	}
}
