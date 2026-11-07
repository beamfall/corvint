// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// testScopeRepository is a committed Go module whose checks/chk tests read a
// declared docs path and import lib, which embeds a data file.
func testScopeRepository(t *testing.T, declaration string) string {
	t.Helper()
	root := fixtureRepository(t)
	files := map[string]string{
		"go.mod":                         "module example.com/fixture\n\ngo 1.27\n",
		"lib/lib.go":                     "package lib\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar Data string\n",
		"lib/data.txt":                   "data\n",
		"checks/chk/chk_test.go":         "package chk\n\nimport (\n\t\"testing\"\n\n\t_ \"example.com/fixture/lib\"\n)\n\nfunc TestRead(t *testing.T) {}\n",
		"checks/chk/testdata/case.txt":   "case\n",
		"checks/undeclared/u_test.go":    "package undeclared\n\nimport \"testing\"\n\nfunc TestU(t *testing.T) {}\n",
		"other/other.go":                 "package other\n",
		"docs/read.md":                   "read\n",
		"docs/build-log/entry.md":        "log\n",
		".corvint/test-read-scopes.json": declaration,
	}
	for file, body := range files {
		writeAll(t, root, file, body)
	}
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "packages")
	return root
}

func writeAll(t *testing.T, root, file, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(file))), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, root, file, body)
}

func withScope(t *testing.T, name string, scope []string) {
	t.Helper()
	previous, had := scopes[name]
	scopes[name] = scope
	t.Cleanup(func() {
		if had {
			scopes[name] = previous
		} else {
			delete(scopes, name)
		}
	})
}

// TestRunStepKeysTestScopeOnPackageClosure replays GL-V0-775: an `@test:DIR`
// step hits after a CEM-only change, a build-log edit and an unrelated
// package's edit, and reruns after an edit to its own subtree, a dependency's
// source or embedded file, a declared entry or the declaration. The keyed set
// is a superset of what the slow path (the whole-tree key) would rerun on for
// each such edit, so every hit is on inputs the step reads unchanged.
func TestRunStepKeysTestScopeOnPackageClosure(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := testScopeRepository(t, `{"profile":"corvint-test-read-scopes/0","packages":{"checks/chk":["docs/read.md"]}}`+"\n")
	write(t, root, ".gitignore", "COUNT\n")
	withScope(t, "fixture-check", []string{testScopePrefix + "checks/chk"})
	step := append([]string{"run", "fixture-check+aaa"}, counting...)
	if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "RECORD fixture-check+aaa") {
		t.Fatalf("first run: code %d, %q", code, out)
	}
	hits := []struct{ file, body string }{
		{".corvint/changes/0123.cem.json", "{}\n"},
		{"docs/build-log/entry.md", "log, edited\n"},
		{"other/other.go", "package other\n\nvar X = 1\n"},
	}
	for _, edit := range hits {
		writeAll(t, root, edit.file, edit.body)
		if _, out := ledgerRun(t, step...); !strings.Contains(out, "HIT fixture-check+aaa") {
			t.Fatalf("an edit to %s outside the closure missed: %q", edit.file, out)
		}
	}
	misses := []struct{ file, body string }{
		{"checks/chk/testdata/case.txt", "case, edited\n"},
		{"lib/lib.go", "package lib\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar Data string\n\nvar Y = 2\n"},
		{"lib/data.txt", "data, edited\n"},
		{"docs/read.md", "read, edited\n"},
		{".corvint/test-read-scopes.json", `{"profile":"corvint-test-read-scopes/0","packages":{"checks/chk":["docs/"]}}` + "\n"},
	}
	for _, edit := range misses {
		writeAll(t, root, edit.file, edit.body)
		if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN fixture-check+aaa: no recorded pass") {
			t.Fatalf("an edit to %s inside the closure hit: %q", edit.file, out)
		}
	}
	if got := runs(t, root); got != 1+len(misses) {
		t.Fatalf("the step ran %d times, want %d", got, 1+len(misses))
	}
}

// TestRunStepTestScopeFallsBackToTheTree replays GL-V0-775's doubt rule: a
// package the declaration does not bound, and an unreadable declaration, key
// on the whole tree with the reason printed, so a CEM-only change reruns them.
func TestRunStepTestScopeFallsBackToTheTree(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	for name, tc := range map[string]struct{ declaration, dir, reason string }{
		"undeclared package": {`{"profile":"corvint-test-read-scopes/0","packages":{"checks/chk":[]}}` + "\n", "checks/undeclared", "declares no read scope for checks/undeclared"},
		"unknown member":     {`{"profile":"corvint-test-read-scopes/0","packages":{},"extra":1}` + "\n", "checks/chk", "unknown field"},
		"wrong profile":      {`{"profile":"other/0","packages":{"checks/chk":[]}}` + "\n", "checks/chk", "profile must be"},
		"dotdot entry":       {`{"profile":"corvint-test-read-scopes/0","packages":{"checks/chk":["../x"]}}` + "\n", "checks/chk", `entry "../x"`},
	} {
		t.Run(name, func(t *testing.T) {
			root := testScopeRepository(t, tc.declaration)
			write(t, root, ".gitignore", "COUNT\n")
			withScope(t, "fixture-check", []string{testScopePrefix + tc.dir})
			step := append([]string{"run", "fixture-check+aaa"}, counting...)
			if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "SCOPE @test:"+tc.dir+" unavailable") || !strings.Contains(out, tc.reason) || !strings.Contains(out, "keyed on the whole tree") || !strings.Contains(out, "RECORD fixture-check+aaa") {
				t.Fatalf("first run: code %d, %q", code, out)
			}
			writeAll(t, root, ".corvint/changes/0123.cem.json", "{}\n")
			if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN fixture-check+aaa: no recorded pass") {
				t.Fatalf("a tree-keyed fallback hit after a tree change: %q", out)
			}
		})
	}
}

// TestDeclaredTestScopesNameOnePackage replays GL-V0-776 against this
// repository's Makefile: every testScopes step's recipe is exactly one go test
// -run of the `@test:` package, so the package closure bounds what it reads.
// Whether the declaration bounds that package is decided at run time, where a
// missing declaration keys the step on the whole tree (GL-V0-775).
func TestDeclaredTestScopesNameOnePackage(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	for name, scope := range testScopes {
		dir, ok := strings.CutPrefix(scope[0], testScopePrefix)
		if !ok {
			dir, ok = strings.CutPrefix(scope[0], depsScopePrefix)
		}
		if !ok || (len(scope) != 1 && !strings.HasPrefix(scope[0], depsScopePrefix)) {
			t.Fatalf("%s: scope %q does not start with its one @test: or @deps: package", name, scope)
		}
		_, recipe, ok := strings.Cut(string(makefile), "\n"+name+":\n\t")
		if !ok {
			t.Fatalf("%s: no Makefile target", name)
		}
		recipe, _, _ = strings.Cut(recipe, "\n")
		run, light := strings.CutPrefix(recipe, "$(DOC_GATE_LIGHT_GO_TEST) -run ")
		if !light {
			run, _ = strings.CutPrefix(recipe, "$(DOC_GATE_GO_TEST) -run ")
		}
		if run == recipe || !strings.HasSuffix(recipe, " ./"+dir) || strings.Count(recipe, " ./") != 1 {
			t.Errorf("%s: recipe %q is not one go test -run of ./%s", name, recipe, dir)
		}
	}
}

// TestRunStepKeysDepsAndAllScope_GLV03720 replays GL-V0-3720 and GL-V0-788 on the
// shell-foreign-callers-check scope shape: a CEM-only change, a build-log edit
// and a new .tsv hit, since the step reads none of them; an edit to a shell
// script, to a file that names it, to the .corvint declaration, a new caller
// file, and an embedded .json of the Go test closure (excluded from `@all`, put
// back by `@deps:`) each rerun it.
func TestRunStepKeysDepsAndAllScope_GLV03720(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := testScopeRepository(t, `{"profile":"corvint-test-read-scopes/0","packages":{}}`+"\n")
	write(t, root, ".gitignore", "COUNT\n")
	writeAll(t, root, "lib/lib.go", "package lib\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar Data string\n\n//go:embed table.json\nvar Table string\n")
	writeAll(t, root, "lib/table.json", "{}\n")
	writeAll(t, root, "jobs/run.sh", "#!/bin/sh\necho run\n")
	writeAll(t, root, "other/caller.go", "package other\n\nconst job = \"jobs/run.sh\"\n")
	writeAll(t, root, ".corvint/shell-foreign-callers.json", `{"scripts":{}}`+"\n") //corvint:fixture-path
	withScope(t, "fixture-callers", []string{depsScopePrefix + "checks/chk", allScope, "!*.md", "!*.json", "!*.jsonl", "!*.tsv", ".corvint/*.json"})
	step := append([]string{"run", "fixture-callers+aaa"}, counting...)
	if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "RECORD fixture-callers+aaa") || strings.Contains(out, "unavailable") {
		t.Fatalf("first run: code %d, %q", code, out)
	}
	for _, edit := range []struct{ file, body string }{
		{".corvint/changes/0123.cem.json", "{}\n"},
		{"docs/build-log/entry.md", "log, edited\n"},
		{"docs/specs/REQUIREMENTS.tsv", "id\tpath\n"}, //corvint:fixture-path
	} {
		writeAll(t, root, edit.file, edit.body)
		if _, out := ledgerRun(t, step...); !strings.Contains(out, "HIT fixture-callers+aaa") {
			t.Fatalf("an edit to %s, which the step does not read, missed: %q", edit.file, out)
		}
	}
	misses := []struct{ file, body string }{
		{"jobs/run.sh", "#!/bin/sh\necho changed\n"},
		{"other/caller.go", "package other\n\nconst job = \"jobs/run.sh --now\"\n"},
		{".corvint/shell-foreign-callers.json", `{"scripts":{"jobs/run.sh":{}}}` + "\n"}, //corvint:fixture-path
		{"tools2/new.py", "import subprocess\nsubprocess.run(['jobs/run.sh'])\n"},
		{"lib/table.json", "{\"edited\":true}\n"},
		// An excluded suffix with a name the source walk refuses still reruns it.
		{`notes/bad\name.md`, "walk-invalid\n"},   //corvint:fixture-path
		{"notes/with space.md", "unusual name\n"}, //corvint:fixture-path
	}
	for _, edit := range misses {
		writeAll(t, root, edit.file, edit.body)
		if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN fixture-callers+aaa: no recorded pass") {
			t.Fatalf("an edit to %s, which the step reads, hit: %q", edit.file, out)
		}
	}
	if got := runs(t, root); got != 1+len(misses) {
		t.Fatalf("the step ran %d times, want %d", got, 1+len(misses))
	}
	// A symlink with an excluded suffix stays in the key: a walk may follow it.
	if err := os.Symlink("../docs/read.md", filepath.Join(root, "notes", "link.md")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN fixture-callers+aaa: no recorded pass") {
		t.Fatalf("a new symlink with an excluded suffix hit: %q", out)
	}
}
