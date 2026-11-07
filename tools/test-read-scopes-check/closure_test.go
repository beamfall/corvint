// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// AFP-V0-3150: a declaration that lists the import closure of a literal target
// in place of the whole tree cannot fall below that closure silently. The
// fixture reader lists and builds ./cmd/app from the root, as
// cmd/corvint-analyzer-rust and cmd/corvint-corpus-mcp do; dropping one closure
// entry, or the target gaining an import, fails the check naming the
// directory, and a directory outside the closure is never reported.
func TestClosureDeclarationCannotFallBelowItsClosure_AFP_V0_3150(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool unavailable")
	}
	test := "package main\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n\n\t\"example.test/m/internal/procgroup\"\n)\n\n" +
		"func TestClosure(t *testing.T) {\n\tgoPath, _ := exec.LookPath(\"go\")\n" +
		"\t_, _ = exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/app\").Output()\n" +
		"\t_ = procgroup.Spec{Argv: []string{goPath, \"build\", \"-o\", \"./out\", \"./cmd/app\"}}\n}\n"
	const complete = `"cmd/app/","go.mod","internal/a/","internal/b/","internal/procgroup/"`
	app := "package main\n\nimport (\n\t_ \"example.test/m/internal/a\"\n\t_ \"example.test/m/internal/b\"\n)\n\nfunc main() {}\n"
	grown := strings.Replace(app, "\t_ \"example.test/m/internal/b\"\n", "\t_ \"example.test/m/internal/b\"\n\t_ \"example.test/m/internal/c\"\n", 1)
	for name, c := range map[string]struct {
		entries, app string
		code         int
		want         []string
	}{
		"complete closure": {complete, app, 0, []string{"test-read-scopes-check: ok"}},
		"closure entry dropped": {`"cmd/app/","go.mod","internal/a/","internal/procgroup/"`, app, 1, []string{
			"package cmd/reader: cmd/reader/reader_test.go runs go list -deps, which loads internal/b, outside its entries in .corvint/test-read-scopes.json",
			"package cmd/reader: cmd/reader/reader_test.go builds repository source with go, which reads internal/b, outside its entries",
		}},
		"target gains an import": {complete, grown, 1, []string{
			"runs go list -deps, which loads internal/c, outside its entries",
			"builds repository source with go, which reads internal/c, outside its entries",
		}},
	} {
		files := map[string]string{
			"go.mod":                         "module example.test/m\n\ngo 1.27\n",
			"internal/procgroup/p.go":        "package procgroup\n\ntype Spec struct{ Argv []string }\n",
			"internal/a/a.go":                "package a\n",
			"internal/b/b.go":                "package b\n",
			"internal/c/c.go":                "package c\n",
			"internal/journal/j.go":          "package journal\n",
			"cmd/app/main.go":                c.app,
			"cmd/reader/main.go":             "package main\n\nfunc main() {}\n",
			"cmd/reader/reader_test.go":      test,
			".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"cmd/reader":[` + c.entries + `]}}`,
		}
		var stdout, stderr bytes.Buffer
		code := run(writeRepository(t, files), &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != c.code {
			t.Errorf("%s: code=%d want %d; output %q", name, code, c.code, output)
		}
		for _, want := range c.want {
			if !strings.Contains(output, want) {
				t.Errorf("%s: output %q lacks %q", name, output, want)
			}
		}
		if strings.Contains(output, "internal/journal") {
			t.Errorf("%s: output %q reports internal/journal, which no closure loads", name, output)
		}
	}
}
