// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// up climbs one directory. It is built so that this file carries no
// parent-only literal, which would make the package a rule (d) reader.
var up = "." + "./"

// repository writes and indexes a module whose declared internal/cem/verify
// tests reach the root and name the tamper matrix, the V1-1340 shape, and whose
// declared fixtures/f tests name a tracked file without reaching the root.
func repository(t *testing.T, entries, literals string) string {
	t.Helper()
	files := map[string]string{
		"go.mod":                                "module example.test/m\n\ngo 1.27\n",
		"Makefile":                              "gate:\n",
		"protocol/cem-1.0/stable/manifest.json": "{}\n",
		"protocol/cem-1.0/stable/tamper-matrix/case.json": "{}\n",
		"testdata/case.json":                              "{}\n",
		".corvint/change.cem.json":                        "{}\n",
		"internal/cem/verify/testdata/case.json":          "{}\n",
		"internal/cem/verify/v.go":                        "package verify\n",
		"internal/cem/verify/root_test.go":                "package verify\n\nvar top = \"" + strings.Repeat(up, 3) + "\"\n\nvar parent = \"" + strings.TrimSuffix(up, "/") + "\"\n",
		"internal/cem/verify/matrix_test.go":              "package verify\n\nvar matrix = top + \"protocol/cem-1.0/stable/tamper-matrix\"\n\nvar folded = \"makefile\"\n\nvar shadowed = \"testdata/case.json\"\n\nvar evidence = \".corvint/change.cem.json\"\n",
		"fixtures/f/f.go":                                 "package f\n",
		"fixtures/f/f_test.go":                            "package f\n\nvar fixture = \"protocol/cem-1.0/stable/manifest.json\"\n",
		".corvint/test-read-scopes.json":                  `{"profile":"corvint-test-read-scopes/0","packages":{"fixtures/f":[],"internal/cem/verify":[` + entries + `]}}`,
	}
	if literals != "" {
		files[literalsPath] = literals
	}
	return writeRepository(t, files)
}

// writeRepository writes files into a new Git repository and stages them.
func writeRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	return writeRepositoryAt(t, t.TempDir(), files)
}

// writeRepositoryAt writes files into a new Git repository at root.
func writeRepositoryAt(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	for name, content := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func TestUndeclaredRepositoryLiteralFailsNamingPackageAndPath_V1_1351(t *testing.T) {
	const matrix = `"protocol/cem-1.0/stable/tamper-matrix/"`
	// The root testdata/case.json is named by a literal that also names the
	// package's own testdata/case.json; every case but one declares it.
	const shadow = `,"testdata/case.json"`
	const head = `{"profile":"corvint-test-read-scope-literals/0","literals":`
	recorded := head + `{"internal/cem/verify":{"protocol/cem-1.0/stable/tamper-matrix":"fixture"}}}`
	for name, c := range map[string]struct {
		entries, literals string
		code              int
		want              string
	}{
		"declared entry":      {matrix + shadow, "", 0, "test-read-scopes-check: ok"},
		"declared parent":     {`"protocol/cem-1.0/stable/"` + shadow, "", 0, "test-read-scopes-check: ok"},
		"declared file below": {`"protocol/cem-1.0/stable/tamper-matrix/case.json"` + shadow, "", 0, "test-read-scopes-check: ok"},
		"entry removed":       {`"testdata/case.json"`, "", 1, "package internal/cem/verify: internal/cem/verify/matrix_test.go names protocol/cem-1.0/stable/tamper-matrix, outside its entries"},
		"sibling only":        {`"protocol/cem-1.0/stable/manifest.json"` + shadow, "", 1, "names protocol/cem-1.0/stable/tamper-matrix"},
		"root shadowed":       {matrix, "", 1, "internal/cem/verify/matrix_test.go names testdata/case.json, outside its entries"},
		"recorded literal":    {`"testdata/case.json"`, recorded, 0, "test-read-scopes-check: ok"},
		"stale record":        {matrix + shadow, recorded, 0, "records internal/cem/verify protocol/cem-1.0/stable/tamper-matrix, which no test file of the package names"},
		"empty reason":        {"", head + `{"internal/cem/verify":{"protocol/cem-1.0/stable/tamper-matrix":" "}}}`, 2, "want exactly"},
		"empty package":       {"", head + `{"internal/cem/verify":{}}}`, 2, "want exactly"},
		"unknown member":      {"", head + `{},"extra":1}`, 2, "want exactly"},
		"invalid scopes":      {`"/abs"`, "", 2, "is invalid"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(repository(t, c.entries, c.literals), &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != c.code || !strings.Contains(output, c.want) {
			t.Errorf("%s: code=%d want %d; output %q lacks %q", name, code, c.code, output, c.want)
		}
		// fixtures/f does not reach the root, the climb to internal/cem is the
		// root lookup, a case-folded literal names no tracked path, and the
		// change-evidence map is tracked only until its seal.
		for _, never := range []string{"package fixtures/f", "names internal/cem,", "names Makefile", "names .corvint/change.cem.json"} {
			if strings.Contains(output, never) {
				t.Errorf("%s: output %q reports %q", name, output, never)
			}
		}
	}
}

// V1-2117: a test that builds a tracked path from adjacent filepath.Join string
// segments, as cmd/corvint-cem-candidate/next_action_test.go did at d114cc62e,
// is reported when the path is outside its scope; the same segments joined onto
// a fixture root are not resolved from the repository root.
func TestJoinedSegmentsOutsideScopeFail_V1_2117(t *testing.T) {
	const recipe = "protocol/cem-1.0/stable/neutral-criterion/repository.recipe.json"
	test := "package main\n\nimport (\n\t\"path/filepath\"\n\t\"runtime\"\n)\n\n" +
		"func fixtureDirectory() string {\n\t_, file, _, _ := runtime.Caller(0)\n" +
		"\treturn filepath.Join(filepath.Dir(file), \"..\", \"..\", \"protocol\", \"cem-1.0\", \"stable\", \"neutral-criterion\")\n}\n\n" +
		"func fixtureRoot(root string) string { return filepath.Join(root, \"internal\", \"tasks\") }\n"
	for name, c := range map[string]struct {
		entries string
		code    int
		want    string
	}{
		"undeclared": {"", 1, "package cmd/corvint-cem-candidate: cmd/corvint-cem-candidate/next_action_test.go names protocol/cem-1.0/stable/neutral-criterion, outside its entries"},
		"declared":   {`"protocol/cem-1.0/stable/neutral-criterion/"`, 0, "test-read-scopes-check: ok"},
	} {
		root := writeRepository(t, map[string]string{
			"go.mod":                            "module example.test/m\n\ngo 1.27\n",
			recipe:                              "{}\n",
			"internal/tasks/t.go":               "package tasks\n",
			"cmd/corvint-cem-candidate/main.go": "package main\n\nfunc main() {}\n",
			"cmd/corvint-cem-candidate/next_action_test.go": test,
			".corvint/test-read-scopes.json":                `{"profile":"corvint-test-read-scopes/0","packages":{"cmd/corvint-cem-candidate":[` + c.entries + `]}}`,
		})
		var stdout, stderr bytes.Buffer
		code := run(root, &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != c.code || !strings.Contains(output, c.want) {
			t.Errorf("%s: code=%d want %d; output %q lacks %q", name, code, c.code, output, c.want)
		}
		if strings.Contains(output, "names internal/tasks") {
			t.Errorf("%s: a run joined onto a fixture root resolved from the repository root: %q", name, output)
		}
	}
}

// V1-2037: a declared package whose test runs go list -deps fails the check
// when a module package it transitively imports lies outside its scope, the
// #1288 shape (internal/gitnotes -> internal/cem/verify -> internal/cem/sim).
func TestGoListDependencyOutsideScopeFails_V1_2037(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool unavailable")
	}
	test := "package gitnotes\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func TestFetchesNothing(t *testing.T) {\n\tgoTool, _ := exec.LookPath(\"go\")\n" +
		"\t_, _ = exec.Command(goTool, \"list\", \"-deps\", \".\").Output()\n}\n"
	for name, c := range map[string]struct {
		entries string
		code    int
		want    string
	}{
		"transitive omitted": {`"go.mod","internal/cem/verify/"`, 1, "package internal/gitnotes: internal/gitnotes/gitnotes_test.go runs go list -deps, which loads internal/cem/sim, outside its entries"},
		"parent entry only":  {`"go.mod","internal/cem/sim/x.json","internal/cem/verify/"`, 1, "loads internal/cem/sim, outside"},
		"declared":           {`"go.mod","internal/cem/sim/","internal/cem/verify/"`, 0, "test-read-scopes-check: ok"},
	} {
		// A root with a space must not split go list's directory lines.
		for _, root := range []string{t.TempDir(), filepath.Join(t.TempDir(), "review work", "repo")} {
			root = writeRepositoryAt(t, root, map[string]string{
				"go.mod":                             "module example.test/m\n\ngo 1.27\n",
				"internal/cem/sim/s.go":              "package sim\n",
				"internal/cem/sim/x.json":            "{}\n",
				"internal/cem/verify/v.go":           "package verify\n\nimport _ \"example.test/m/internal/cem/sim\"\n",
				"internal/gitnotes/g.go":             "package gitnotes\n\nimport _ \"example.test/m/internal/cem/verify\"\n",
				"internal/gitnotes/gitnotes_test.go": test,
				".corvint/test-read-scopes.json":     `{"profile":"corvint-test-read-scopes/0","packages":{"internal/gitnotes":[` + c.entries + `]}}`,
			})
			var stdout, stderr bytes.Buffer
			code := run(root, &stdout, &stderr)
			output := stdout.String() + stderr.String()
			if code != c.code || !strings.Contains(output, c.want) {
				t.Errorf("%s under %q: code=%d want %d; output %q lacks %q", name, root, code, c.code, output, c.want)
			}
		}
	}
}

// V1-2952: a package whose tests launch a nested sandbox, recorded in
// .corvint/test-nested-sandboxes.json, fails the check when it is declared, the
// #1937 shape (internal/liveverify/mutate under the Landlock wrapper); the
// message names the package and the unbounded-readers units alternative.
func TestDeclaredNestedSandboxPackageFails_V1_2952(t *testing.T) {
	const mutate = "internal/liveverify/mutate"
	const sandboxes = `{"profile":"corvint-test-nested-sandboxes/0","packages":{"` + mutate + `":"bubblewrap (FPK-V0-142)"}}`
	for name, c := range map[string]struct {
		scopes, sandboxes string
		code              int
		want              string
	}{
		"declared":        {`{"` + mutate + `":[]}`, sandboxes, 1, "package internal/liveverify/mutate is declared in .corvint/test-read-scopes.json, but .corvint/test-nested-sandboxes.json records that its tests launch a nested sandbox (bubblewrap (FPK-V0-142)), which the confined CI shard refuses; remove its entry and add it to units of .corvint/unbounded-readers.json"},
		"undeclared":      {`{"internal/other":[]}`, sandboxes, 0, "test-read-scopes-check: ok"},
		"no record":       {`{"` + mutate + `":[]}`, "", 0, "test-read-scopes-check: ok"},
		"blank reason":    {`{}`, `{"profile":"corvint-test-nested-sandboxes/0","packages":{"` + mutate + `":" "}}`, 2, "want exactly"},
		"unclean package": {`{}`, `{"profile":"corvint-test-nested-sandboxes/0","packages":{"./x":"r"}}`, 2, "want exactly"},
		"unknown member":  {`{}`, `{"profile":"corvint-test-nested-sandboxes/0","packages":{},"extra":1}`, 2, "want exactly"},
	} {
		files := map[string]string{
			"go.mod":                         "module example.test/m\n\ngo 1.27\n",
			mutate + "/m.go":                 "package mutate\n",
			mutate + "/m_test.go":            "package mutate\n",
			"internal/other/o.go":            "package other\n",
			"internal/other/o_test.go":       "package other\n",
			".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":` + c.scopes + `}`,
		}
		if c.sandboxes != "" {
			files[sandboxesPath] = c.sandboxes
		}
		var stdout, stderr bytes.Buffer
		code := run(writeRepository(t, files), &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != c.code || !strings.Contains(output, c.want) {
			t.Errorf("%s: code=%d want %d; output %q lacks %q", name, code, c.code, output, c.want)
		}
	}
}

// V1-3210: a declared package whose test builds repository source with go, the
// tools/compat-trial shape at d92b0a2a5 (procgroup.Spec{Argv: []string{goPath,
// "build", ...}} and "./fixtures/"+variant importing internal/archivepath),
// fails the check when a module package the build loads lies outside its
// scope, and passes once that directory is declared. A string slice that is
// no procgroup.Spec Argv is data (resolving its ./absent target would exit 2),
// and a recorded path keeps a build quiet.
func TestGoBuildDependencyOutsideScopeFails_V1_3210(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool unavailable")
	}
	test := "package main\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n\n\t\"example.test/m/internal/procgroup\"\n)\n\n" +
		"func TestBuilds(t *testing.T) {\n\tgoPath, _ := exec.LookPath(\"go\")\n\tvariant := \"a\"\n" +
		"\t_ = procgroup.Spec{Argv: []string{goPath, \"build\", \"-o\", \"./out\", \".\"}}\n" +
		"\t_ = procgroup.Spec{Argv: []string{goPath, \"build\", \"./fixtures/\" + variant}}\n" +
		"\t_ = []string{\"go\", \"test\", \"./absent\"}\n}\n"
	const head = `{"profile":"corvint-test-read-scope-literals/0","literals":`
	for name, c := range map[string]struct {
		entries, literals string
		code              int
		want, never       string
	}{
		"undeclared":         {`"go.mod","internal/hidden/","internal/procgroup/"`, "", 1, "package tools/trial: tools/trial/runner_test.go builds repository source with go, which reads internal/archivepath, outside its entries in .corvint/test-read-scopes.json; declare the directory there (AFP-V0-023)", "reads internal/hidden"},
		"fixture undeclared": {`"go.mod","internal/archivepath/","internal/procgroup/"`, "", 1, "which reads internal/hidden, outside", "reads internal/archivepath"},
		"declared":           {`"go.mod","internal/archivepath/","internal/hidden/","internal/procgroup/"`, "", 0, "test-read-scopes-check: ok", "builds repository source"},
		"recorded":           {`"go.mod","internal/hidden/","internal/procgroup/"`, head + `{"tools/trial":{"internal/archivepath":"a build the confined shard never runs"}}}`, 0, "test-read-scopes-check: ok", "which no test file of the package names"},
		"parent entry only":  {`"go.mod","internal/archivepath/x.json","internal/hidden/","internal/procgroup/"`, "", 1, "which reads internal/archivepath, outside", "reads internal/hidden"},
	} {
		files := map[string]string{
			"go.mod":                         "module example.test/m\n\ngo 1.27\n",
			"internal/procgroup/p.go":        "package procgroup\n\ntype Spec struct{ Argv []string }\n",
			"internal/archivepath/a.go":      "package archivepath\n",
			"internal/archivepath/x.json":    "{}\n",
			"internal/hidden/h.go":           "package hidden\n",
			"tools/trial/main.go":            "package main\n\nimport _ \"example.test/m/internal/archivepath\"\n\nfunc main() {}\n",
			"tools/trial/runner_test.go":     test,
			"tools/trial/fixtures/a/main.go": "package main\n\nimport _ \"example.test/m/internal/hidden\"\n\nfunc main() {}\n",
			".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"tools/trial":[` + c.entries + `]}}`,
		}
		if c.literals != "" {
			files[literalsPath] = c.literals
		}
		var stdout, stderr bytes.Buffer
		code := run(writeRepository(t, files), &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != c.code || !strings.Contains(output, c.want) || strings.Contains(output, c.never) {
			t.Errorf("%s: code=%d want %d; output %q lacks %q or reports %q", name, code, c.code, output, c.want, c.never)
		}
	}
}

// V1-3210 review: `go test` and `go vet` also load the imports of the target's
// test files, and the separate value of a flag such as -tags is no target, so
// a flags-only `go build -tags x` still builds the package itself.
func TestGoTestDependencyAndFlagValues_V1_3210(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool unavailable")
	}
	test := "package main\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func TestBuilds(t *testing.T) {\n" +
		"\t_ = exec.Command(\"go\", \"build\", \"-tags\", \"feature\")\n" +
		"\t_ = exec.Command(\"go\", \"test\", \"-count\", \"1\", \"./fixture\")\n}\n"
	for name, c := range map[string]struct {
		entries     string
		code        int
		want, never string
	}{
		"test-only import undeclared": {`"go.mod","internal/built/"`, 1, "which reads internal/testonly, outside", "reads internal/built"},
		"flags-only build undeclared": {`"go.mod","internal/testonly/"`, 1, "which reads internal/built, outside", "reads internal/testonly"},
		"declared":                    {`"go.mod","internal/built/","internal/testonly/"`, 0, "test-read-scopes-check: ok", "builds repository source"},
	} {
		files := map[string]string{
			"go.mod":                         "module example.test/m\n\ngo 1.27\n",
			"internal/built/b.go":            "package built\n",
			"internal/testonly/o.go":         "package testonly\n",
			"tools/trial/main.go":            "package main\n\nimport _ \"example.test/m/internal/built\"\n\nfunc main() {}\n",
			"tools/trial/runner_test.go":     test,
			"tools/trial/fixture/f.go":       "package fixture\n",
			"tools/trial/fixture/f_test.go":  "package fixture\n\nimport _ \"example.test/m/internal/testonly\"\n",
			".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"tools/trial":[` + c.entries + `]}}`,
		}
		var stdout, stderr bytes.Buffer
		code := run(writeRepository(t, files), &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != c.code || !strings.Contains(output, c.want) || strings.Contains(output, c.never) {
			t.Errorf("%s: code=%d want %d; output %q lacks %q or reports %q", name, code, c.code, output, c.want, c.never)
		}
	}
}
