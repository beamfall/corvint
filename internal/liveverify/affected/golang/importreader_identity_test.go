// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-9010 / AFP-V0-2200: a builtin spelling is not an identity proof.
// Package helpers can inject flags without spelling their environment key.
// Unknown environment expressions must also retain actual folded-file selection.
func TestReaderBuiltinIdentity_AFP_V0_9010(t *testing.T) {
	const inject = `func append(env []string, extra ...string) []string {
return []string{string([]byte{71,79,70,76,65,71,83,61,45,106,115,111,110})}
}
`
	const command = `c := exec.Command("go", "list", "-f", "{{.Imports}}", "./lib")
`
	for _, test := range []struct {
		name, helper, setup, use string
		unproven                 bool
	}{
		{"builtin control", "", command, `c.Env = append(os.Environ(), "EXTRA=1")`, false},
		{"package append", inject, command, `c.Env = append(os.Environ(), "EXTRA=1")`, true},
		{"local append", "", `append := func(env []string, extra ...string) []string {
return []string{string([]byte{71,79,70,76,65,71,83,61,45,106,115,111,110})}
}
` + command, `c.Env = append(os.Environ(), "EXTRA=1")`, true},
		{"argument append", `func append(args []string, extra ...string) []string { return []string{"list", "-json", "./lib"} }`,
			`args := []string{"list", "-f", "{{.Imports}}"}
args = append(args, "./lib")
c := exec.Command("go", args...)
`, "", true},
		{"shadowed conversion", `func string(data []byte) int { return 0 }`, command, `out, _ := c.Output(); _ = string(out)`, true},
		{"unknown environment form", `func unknownEnvironment() []string { return nil }`, command, `c.Env = unknownEnvironment()`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := importGraphRepository(t)
			writeFiles(t, root, map[string]string{
				"probe/helper.go": "package probe\n" + test.helper,
				"probe/probe_test.go": "package probe\nimport (\"os\"; \"os/exec\"; \"testing\")\n" +
					"func TestList(t *testing.T) {\n_ = os.Environ\n" + test.setup + test.use + "\n_, _ = c.Output()\n}\n",
				"lib/a_test.go": "package lib\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n",
			})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			unit, ok := graph.Unit(importGraphModule + "probe")
			if !ok || unit.ImportGraphReadsFiles != test.unproven || unit.ImportGraphReadsAll != test.unproven {
				t.Fatalf("builtin identity: %+v, want unproven=%v", unit, test.unproven)
			}
			base := fstest.MapFS{
				"go.mod":        {Data: []byte("module example.test/g\n")},
				"lib/lib.go":    {Data: []byte("package lib\n")},
				"lib/a_test.go": {Data: []byte("package lib\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n")},
			}
			head := fstest.MapFS{}
			for name, file := range base {
				head[name] = file
			}
			head["lib/b_test.go"] = &fstest.MapFile{Data: []byte("package lib\nimport \"testing\"\nfunc TestB(t *testing.T) {}\n")}
			dirty := []string{"lib/b_test.go"}
			changes := golang.ImportSetRecords(base, head, graph, dirty)
			if len(changes.Changed) != 0 || changes.Module != "" || changes.Source != dirty[0] {
				t.Fatalf("expected kept import set with source witness: %+v", changes)
			}
			selected := false
			for _, selection := range affected.Select(graph.WithImportSetChanges(changes), dirty).Selected {
				if selection.UnitID == unit.ID && selection.Witness.Kind == affected.WitnessImportGraphReader {
					selected = true
				}
			}
			if selected != test.unproven {
				t.Fatalf("folded test addition selects probe=%v, want %v", selected, test.unproven)
			}
		})
	}
}

// AFP-V0-9010: only an unremapped physical Caller filename is a known origin.
// Repeated Dir calls avoid climbing literals: selection must come from the
// fallback reader edge, including for a new, unsupported filename transform.
func TestCallerFilenameProvenance_AFP_V0_9010(t *testing.T) {
	for _, test := range []struct {
		name, directive, filename string
		unproven                  bool
	}{
		{"physical filename", "", "filename", false},
		{"line comment", "//line /repo/elsewhere/nested/f.go:1\n", "filename", true},
		{"block comment", "/*line /repo/elsewhere/nested/f.go:1*/", "filename", true},
		{"column directive", "/*line /repo/elsewhere/nested/f.go:1:2*/", "filename", true},
		{"unknown filename transform", "", "unknownFilename(filename)", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := importGraphRepository(t)
			writeFiles(t, root, map[string]string{
				"probe/probe_test.go": "package probe\nimport (\"os/exec\"; \"path/filepath\"; \"runtime\"; \"testing\")\n" +
					"func unknownFilename(name string) string { return name }\n" +
					"func TestList(t *testing.T) {\n" + test.directive +
					"_, filename, _, _ := runtime.Caller(0)\n" +
					`c := exec.Command("go", "list", "-f", "{{.Imports}}", "./lib")` + "\n" +
					"c.Dir = filepath.Dir(filepath.Dir(" + test.filename + "))\n_, _ = c.Output()\n}\n",
				"elsewhere/lib/lib.go": "package lib\n",
			})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			unit, ok := graph.Unit(importGraphModule + "probe")
			if !ok || unit.ImportGraphReadsFiles != test.unproven || unit.ImportGraphReadsAll != test.unproven {
				t.Fatalf("filename provenance: %+v, want unproven=%v", unit, test.unproven)
			}
			const dirty = "elsewhere/lib/lib.go"
			base := fstest.MapFS{"go.mod": {Data: []byte("module example.test/g\n")}, dirty: {Data: []byte("package lib\n")}}
			head := fstest.MapFS{"go.mod": base["go.mod"], dirty: {Data: []byte("package lib\nimport " + strconv.Quote("strings") + "\nvar _ = strings.TrimSpace\n")}}
			changes := golang.ImportSetRecords(base, head, graph, []string{dirty})
			if changes.Changed[importGraphModule+"elsewhere/lib"] != dirty {
				t.Fatalf("missing changed import set: %+v", changes)
			}
			selected := false
			for _, selection := range affected.Select(graph.WithImportSetChanges(changes), []string{dirty}).Selected {
				if selection.UnitID == unit.ID && selection.Witness.Kind == affected.WitnessImportGraphReader {
					selected = true
				}
			}
			if selected != test.unproven {
				t.Fatalf("remapped-target import edit selects probe=%v, want %v", selected, test.unproven)
			}
		})
	}
}
