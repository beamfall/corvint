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

// sourceSliceFixture is a module whose package p has plain functions reached
// by different tests, a method, a function used as a value, a package
// variable, an initializer and an external test; q imports p, p imports u,
// and r has one function its only test does not call.
func sourceSliceFixture(t *testing.T) (root, base string) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		"go.mod":      "module example.test/m\n\ngo 1.27\n",
		"u/u.go":      "package u\n\nvar Last string\n\nfunc U() string { return \"u\" }\n",
		"u/u_test.go": "package u\n\nimport \"testing\"\n\nfunc TestU(t *testing.T) { _ = U() }\n",
		"p/p.go": "package p\n\nimport (\n\t\"strings\"\n\n\t\"example.test/m/u\"\n)\n\n" +
			"type T struct{ n int }\n\nfunc (t T) Double() int { return twice(t.n) }\n\n" +
			"func twice(n int) int { return n * 2 }\n\n" +
			"// Upper upper-cases.\nfunc Upper(s string) string { return strings.ToUpper(s) + u.U() }\n\n" +
			"func Lower(s string) string { return strings.ToLower(s) }\n\n" +
			"var table = map[string]func(string) string{\"lower\": Lower}\n\n" +
			"var counter int\n\nfunc Bump() int { counter++; return counter }\n\n" +
			"var greeting = Greet()\n\nfunc Greet() string { return \"hi\" }\n\n" +
			"var state int\n\nfunc Set(p *int) { *p = 1 }\n\n" +
			"func Mark(s string) { u.Last = s }\n\nfunc remember(s string) { u.Last = s }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\n" +
			"func TestUpper(t *testing.T) { _ = Upper(\"a\") }\n\n" +
			"func TestDouble(t *testing.T) { _ = T{n: 1}.Double() }\n\n" +
			"func TestTable(t *testing.T) { _ = table[\"lower\"](\"A\") }\n\n" +
			"func TestBump(t *testing.T) { _ = Bump() }\n\n" +
			"func TestGreeting(t *testing.T) { _ = greeting }\n\n" +
			"func TestCounter(t *testing.T) { _ = counter }\n\n" +
			"func TestSet(t *testing.T) { Set(&state) }\n\n" +
			"func TestState(t *testing.T) { _ = state }\n\n" +
			"func TestMark(t *testing.T) { Mark(\"m\") }\n",
		"p/ext_test.go":             "package p_test\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/p\"\n)\n\nfunc TestExternal(t *testing.T) { _ = p.Lower(\"X\") }\n",
		"q/q.go":                    "package q\n\nimport \"example.test/m/p\"\n\nfunc Q() string { return p.Upper(\"q\") }\n",
		"q/q_test.go":               "package q\n\nimport \"testing\"\n\nfunc TestQ(t *testing.T) { _ = Q() }\n",
		"r/r.go":                    "package r\n\nfunc R() int { return 1 }\n\nfunc S() int { return 2 }\n",
		"r/r_test.go":               "package r\n\nimport \"testing\"\n\nfunc TestR(t *testing.T) { _ = R() }\n",
		"d/d.go":                    "package d\n\nimport . \"strings\"\n\nfunc D(s string) string { return ToUpper(s) }\n\nfunc E() int { return 1 }\n",
		"d/d_test.go":               "package d\n\nimport \"testing\"\n\nfunc TestD(t *testing.T) { _ = D(\"d\") }\n",
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
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(sessionFakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SESSION_FAKE_LOG", filepath.Join(t.TempDir(), "runners.log"))
	t.Setenv("SESSION_FAKE_MODE", "pass")
	return root, base
}

// TSP-V0-006: a package selected only by a change to its own plain functions
// runs just the test functions that can reach them (all external tests kept),
// or builds and vets only when none can; every missing proof keeps it whole
// with the proof named, and its dependents stay whole.
func TestTSPV0006DryRunSlicesSourceChanges(t *testing.T) {
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

	slicedCases := []struct {
		name, id, pattern, reason string
		edits                     map[string]string
	}{
		{"a called function", "go:example.test/m/p", "^(TestExternal|TestUpper)$", "selected: the tests reaching Upper changed in p/p.go",
			map[string]string{"p/p.go": edit("strings.ToUpper(s) +", "strings.ToTitle(s) +")}},
		{"a doc comment", "go:example.test/m/p", "^(TestExternal|TestUpper)$", "selected: the tests reaching Upper changed in p/p.go",
			map[string]string{"p/p.go": edit("// Upper upper-cases.", "// Upper returns s upper-cased.")}},
		{"a function a method calls", "go:example.test/m/p", "^(TestDouble|TestExternal)$", "selected: the tests reaching twice changed in p/p.go",
			map[string]string{"p/p.go": edit("return n * 2", "return n + n")}},
		{"an added function", "go:example.test/m/p", "^(TestExternal)$", "selected: the tests reaching Extra changed in p/p.go",
			map[string]string{"p/p.go": pSource + "\nfunc Extra() int { return 1 }\n"}},
		// TSP-V0-864: a comment outside functions is observed by no test.
		{"a comment outside functions", "go:example.test/m/p", "^$", "selected: no test reaches comments outside declarations changed in p/p.go; build and vet only",
			map[string]string{"p/p.go": edit("type T struct", "// T holds n.\ntype T struct")}},
		{"no test reaches it", "go:example.test/m/r", "^$", "selected: no test reaches S changed in r/r.go; build and vet only",
			map[string]string{"r/r.go": "package r\n\nfunc R() int { return 1 }\n\nfunc S() int { return 3 }\n"}},
	}
	for _, tc := range slicedCases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, units := plan(t, tc.edits)
			got := units[tc.id]
			want := sessiontest.Unit{Adapter: sessiontest.AdapterGo, Flag: "-run", Pattern: tc.pattern, Reason: tc.reason, Slice: sessiontest.SliceSelected, Unit: tc.id}
			if got != want {
				t.Fatalf("unit %+v, want %+v (stderr %q)", got, want, stderr)
			}
			if !strings.Contains(stderr, "corvint-test: SLICED 1 of them to the test functions their changed functions reach (TSP-V0-006)\n") || strings.Contains(stderr, "changed test files") {
				t.Fatalf("stderr %q", stderr)
			}
			if dependent, ok := units["go:example.test/m/q"]; ok && dependent.Slice != sessiontest.SliceWhole {
				t.Fatalf("dependent %+v, want whole", dependent)
			}
		})
	}
	if _, units := plan(t, slicedCases[0].edits); units["go:example.test/m/q"].Slice != sessiontest.SliceWhole {
		t.Fatalf("the importer q is not planned whole: %+v", units)
	}

	wholeCases := []struct {
		name, id, reason string
		edits            map[string]string
	}{
		{"every test reaches it", "go:example.test/m/r", "whole: DIRECT_SOURCE_CHANGE via r/r.go",
			map[string]string{"r/r.go": "package r\n\nfunc R() int { return 4 }\n\nfunc S() int { return 2 }\n"}},
		// TSP-V0-861: the changed test file's tests run, with TestExternal every test.
		{"its test file changed too", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (every test function of the package reaches Upper changed in p/p.go and the tests of p/p_test.go)",
			map[string]string{"p/p.go": edit("strings.ToUpper(s) +", "strings.ToTitle(s) +"), "p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestUpper(t *testing.T) { _ = Upper(\"b\") }\n"}},
		{"a function used as a value", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (p/p.go uses Lower other than in a call)",
			map[string]string{"p/p.go": edit("strings.ToLower(s)", "strings.ToLower(s + \"\")")}},
		{"a function reaching a package variable", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (Bump reaches the package variable counter)",
			map[string]string{"p/p.go": edit("counter++; return counter", "counter += 2; return counter")}},
		// The base body counts too: TestCounter observes what TestBump's Bump did at base.
		{"a function that stopped reaching a package variable", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (Bump reaches the package variable counter)",
			map[string]string{"p/p.go": edit("counter++; return counter", "return 1")}},
		// TestState observes what TestSet's Set writes through its pointer argument.
		{"a function with a pointer parameter", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (Set in p/p.go has a parameter or result of a type that may share memory)",
			map[string]string{"p/p.go": edit("*p = 1", "*p = 2")}},
		{"a write to another package", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (Mark writes through u.Last (not provably local))",
			map[string]string{"p/p.go": edit("func Mark(s string) { u.Last = s }", "func Mark(s string) { u.Last = s + \"!\" }")}},
		{"a write only the base body made", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (Mark writes through u.Last (not provably local))",
			map[string]string{"p/p.go": edit("func Mark(s string) { u.Last = s }", "func Mark(s string) {}")}},
		{"a function reaching a write", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (Upper reaches remember, which writes through u.Last (not provably local))",
			map[string]string{"p/p.go": edit("{ return strings.ToUpper(s) +", "{ remember(s); return strings.ToUpper(s) +")}},
		{"a dot import", "go:example.test/m/d", "whole: DIRECT_SOURCE_CHANGE via d/d.go (d/d.go dot-imports \"strings\")",
			map[string]string{"d/d.go": "package d\n\nimport . \"strings\"\n\nfunc D(s string) string { return ToUpper(s) }\n\nfunc E() int { return 2 }\n"}},
		{"a function package initialization calls", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (package initialization in p/p.go reaches Greet)",
			map[string]string{"p/p.go": edit("return \"hi\"", "return \"hello\"")}},
		{"a method", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (p/p.go changes more than plain functions)",
			map[string]string{"p/p.go": edit("return twice(t.n)", "return twice(t.n) + 0")}},
		{"an import", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (p/p.go changes more than plain functions)",
			map[string]string{"p/p.go": edit("\t\"strings\"\n", "\t\"strings\"\n\t_ \"unicode\"\n")}},
		{"a package variable", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (p/p.go changes more than plain functions)",
			map[string]string{"p/p.go": edit("var counter int", "var counter = 1")}},
		{"a dependency also changed", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/p.go (also selected by DEPENDENCY_PATH via u/u.go)",
			map[string]string{"p/p.go": edit("strings.ToUpper(s) +", "strings.ToTitle(s) +"), "u/u.go": "package u\n\nfunc U() string { return \"v\" }\n"}},
		{"a new file", "go:example.test/m/p", "whole: DIRECT_SOURCE_CHANGE via p/extra.go (p/extra.go is new)",
			map[string]string{"p/extra.go": "package p\n\nfunc Extra() int { return 1 }\n"}},
		{"an assembly file", "go:example.test/m/p", "(also selected by ENCLOSING_PACKAGE via p/p_amd64.s)",
			map[string]string{"p/p.go": edit("strings.ToUpper(s) +", "strings.ToTitle(s) +"), "p/p_amd64.s": "// assembly\n"}},
		{"a linkname directive", "go:example.test/m/p", "(p/link.go names go:linkname)",
			map[string]string{"p/p.go": edit("strings.ToUpper(s) +", "strings.ToTitle(s) +"), "p/link.go": "package p\n\nimport _ \"unsafe\"\n\n//go:linkname now time.now\nfunc now() (int64, int32, int64)\n"}},
		// cgo is a module-level Go frontier, so the plan falls back before slicing.
		{"a cgo file", "go:example.test/m/p", "",
			map[string]string{"p/p.go": edit("strings.ToUpper(s) +", "strings.ToTitle(s) +"), "p/cgo.go": "package p\n\nimport \"C\"\n"}},
	}
	for _, tc := range wholeCases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, units := plan(t, tc.edits)
			got := units[tc.id]
			if got.Slice != sessiontest.SliceWhole || got.Flag != "" || got.Pattern != "" || !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("unit %+v, want whole with reason %q (stderr %q)", got, tc.reason, stderr)
			}
			if strings.Contains(stderr, "TSP-V0-006") {
				t.Fatalf("stderr %q", stderr)
			}
		})
	}
}
