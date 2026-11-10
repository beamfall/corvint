// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// TestTSPV08132UnresolvedGoTargetsAreFindings_TSPV08132: a go build or go list whose target the file does
// not spell (a helper parameter, a variable, a call) is a finding of the package unless the package
// declares a whole top-level tree with its reason or records the target as a temporary path; before
// this change such a build recorded nothing and the check passed (V1-5028). A `go list` target
// joined to a range over literals expands as a build target does, a variable `go list` target is
// not taken for the package itself, `go run` arguments after the program and `go test` arguments
// after -args are no targets, and the value of `go list -f` is none either.
func TestTSPV08132UnresolvedGoTargetsAreFindings_TSPV08132(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool unavailable")
	}
	const scopes = `{"profile":"corvint-test-read-scopes/0","packages":{"cmd/reader":[`
	const literals = `{"profile":"corvint-test-read-scope-literals/0","literals":`
	const trees = `{"profile":"corvint-test-read-scope-trees/0","trees":`
	probe := "package reader\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func build(t *testing.T, pkg string) {\n\t_ = exec.Command(\"go\", \"build\", \"-o\", \"./out\", pkg)\n}\n\n" +
		"func TestBuilds(t *testing.T) {\n\tbuild(t, \"./cmd/app\")\n}\n"
	base := map[string]string{
		"go.mod":                   "module example.test/m\n\ngo 1.27\n",
		"cmd/reader/reader.go":     "package reader\n",
		"cmd/app/main.go":          "package main\n\nimport _ \"example.test/m/internal/dep\"\n\nfunc main() {}\n",
		"internal/dep/d.go":        "package dep\n",
		"internal/other/o.go":      "package other\n",
		"internal/other/o_test.go": "package other\n",
	}
	finding := "package cmd/reader: cmd/reader/reader_test.go runs a go command on pkg, which the check cannot resolve to a package path"
	for name, c := range map[string]struct {
		test, entries, extra string
		files                map[string]string
		code                 int
		want, never          string
	}{
		"the V1-5028 probe is a finding": {probe, `"go.mod"`, "", nil, 1,
			finding + " (a variable or helper parameter), so its entries in .corvint/test-read-scopes.json cannot be shown to cover what it builds or lists (TSP-V0-8132); spell the target as a literal package path, or a literal joined to a range over string literals (ASD-V0-611), declare the whole top-level tree it reads with a reason (AFP-V0-3010), or, when the target is a temporary path outside the repository, record \"go-target:pkg\" for the package in .corvint/test-read-scope-literals.json with that reason", ""},
		"a whole-tree entry with a reason admits it": {probe, `"cmd/","go.mod"`, "",
			map[string]string{treesPath: trees + `{"cmd/reader":{"cmd/":"cmd/reader/reader_test.go builds a target computed at run time"}}}`}, 0,
			"test-read-scopes-check: ok", "TSP-V0-8132"},
		"a recorded temporary target is admitted and used": {probe, `"go.mod"`, "",
			map[string]string{literalsPath: literals + `{"cmd/reader":{"go-target:pkg":"a program written under t.TempDir()"}}}`}, 0,
			"test-read-scopes-check: ok", "which no test file of the package names"},
		"a record no test names is stale": {"package reader\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n", `"go.mod"`, "",
			map[string]string{literalsPath: literals + `{"cmd/reader":{"go-target:pkg":"a program written under t.TempDir()"}}}`}, 0,
			".corvint/test-read-scope-literals.json records cmd/reader go-target:pkg, which no test file of the package names; remove it", ""},
		"a variable go list target is a finding, not the package itself": {"package reader\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestLists(t *testing.T) {\n\tdir := t.TempDir()\n\t_ = exec.Command(\"go\", \"list\", \"-deps\", \"-f\", \"{{.Dir}}\", dir)\n}\n", `"go.mod"`, "", nil, 1,
			"runs a go command on dir, which the check cannot resolve", "runs go list -deps"},
		"a ranged go list target expands to its packages": {"package reader\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestLists(t *testing.T) {\n\tfor _, pkg := range []string{\"app\"} {\n\t\t_ = exec.Command(\"go\", \"list\", \"-deps\", \"../../cmd/\"+pkg)\n\t}\n}\n", `"go.mod"`, "", nil, 1,
			"cmd/reader/reader_test.go runs go list -deps, which loads internal/dep, outside its entries", "internal/other"},
		"a ranged go list target with its closure declared passes": {"package reader\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestLists(t *testing.T) {\n\tfor _, pkg := range []string{\"app\"} {\n\t\t_ = exec.Command(\"go\", \"list\", \"-deps\", \"../../cmd/\"+pkg)\n\t}\n}\n", `"cmd/app/","go.mod","internal/dep/"`, "", nil, 0,
			"test-read-scopes-check: ok", ""},
		"go run arguments after the program are no targets": {"package reader\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestRuns(t *testing.T) {\n\targ := t.TempDir()\n\t_ = exec.Command(\"go\", \"run\", \"../../cmd/app\", arg, \"--flag\")\n}\n", `"cmd/app/","go.mod","internal/dep/"`, "", nil, 0,
			"test-read-scopes-check: ok", "TSP-V0-8132"},
		"go test arguments after -args are no targets": {"package reader\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestTests(t *testing.T) {\n\tflag := t.TempDir()\n\t_ = exec.Command(\"go\", \"test\", \"../../internal/other\", \"-args\", flag)\n}\n", `"go.mod","internal/other/"`, "", nil, 0,
			"test-read-scopes-check: ok", "TSP-V0-8132"},
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{"cmd/reader/reader_test.go": c.test, ".corvint/test-read-scopes.json": scopes + c.entries + `]}}`}
			for path, body := range base {
				files[path] = body
			}
			for path, body := range c.files {
				files[path] = body
			}
			var stdout, stderr bytes.Buffer
			code := run(writeRepository(t, files), &stdout, &stderr)
			output := stdout.String() + stderr.String()
			if code != c.code || !strings.Contains(output, c.want) || (c.never != "" && strings.Contains(output, c.never)) {
				t.Errorf("code=%d want %d; output %q lacks %q or reports %q", code, c.code, output, c.want, c.never)
			}
		})
	}
}
