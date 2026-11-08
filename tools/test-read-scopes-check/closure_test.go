// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
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

// AFP-V0-3151: in this repository's declaration, cmd/corvint-analyzer-rust
// declares the closures of the two targets its tests list, not internal/, so
// the planner selects it through DECLARED_READ_SCOPE for an edit inside the
// cmd/corvint closure (no missed test) and not for one below
// internal/tasks/journal/, which no read of its tests reaches (the review
// round 1 example).
func TestClosureReaderIsNotSelectedOutsideItsClosure_AFP_V0_3151(t *testing.T) {
	root, err := filepath.Abs(up + up)
	if err != nil {
		t.Fatal(err)
	}
	const reader = "cmd/corvint-analyzer-rust"
	cases := []struct {
		dirty string
		want  bool
	}{
		{"internal/tasks/journal/anchor.go", false},
		{"internal/cem/verify/guards.go", true},
	}
	for _, c := range cases {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(c.dirty))); err != nil {
			t.Fatalf("%s: %v; pick another file of that directory", c.dirty, err)
		}
	}
	graph, err := affected.BuildPartial(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(graph.Frontier(), golang.FrontierReadScopesInvalid) {
		t.Fatalf("%s is invalid", affected.ReadScopesPath)
	}
	// Each edit is a body or comment edit, which changes no import set, as the
	// CLI finds for the guards.go comment edit (AFP-V0-2202); with the import
	// sets unknown, the go list guard is selected one edge past the closure
	// as an IMPORT_GRAPH_READER instead.
	graph = graph.WithImportSetChanges(&affected.ImportSetChanges{})
	// Inside the closure a path-literal read of the same file can win the
	// witness first; either kind is a read-based selection of the reader.
	readBased := map[string]bool{affected.WitnessDeclaredReadScope: true, affected.WitnessPathLiteralReader: true}
	for _, c := range cases {
		plan := affected.Select(graph, []string{c.dirty})
		got := false
		for _, selection := range plan.Selected {
			if len(selection.Tests) > 0 && path.Dir(selection.Tests[0]) == reader && readBased[selection.Witness.Kind] {
				got = true
			}
		}
		if got != c.want {
			t.Errorf("an edit to %s selects %s through a read witness: %t, want %t", c.dirty, reader, got, c.want)
		}
	}
}
