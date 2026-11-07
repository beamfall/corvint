// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-9010: directory origin is a closed proof, not a list of dangerous
// expressions. Computed bytes reproduce the review; Getenv is an unknown
// origin, and root reassignment/escape must invalidate an otherwise known one.
func TestCommandDirectoryFallsBack_AFP_V0_9010(t *testing.T) {
	const rootDefinition = `_, filename, _, _ := runtime.Caller(0)
root := filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
`
	for _, test := range []struct {
		name, setup, directory, target string
		unproven                       bool
	}{
		{"computed bytes", "", "string([]byte{46,46,47,108,105,98})", ".", true},
		{"unknown environment", "", `os.Getenv("LIST_DIRECTORY")`, ".", true},
		{"relative target with unknown directory", "", `os.Getenv("LIST_DIRECTORY")`, "./lib", true},
		{"module root", rootDefinition, "root", "./lib", false},
		{"module root with dot", rootDefinition, "root", ".", true},
		{"root reassigned", rootDefinition + `root = os.Getenv("LIST_DIRECTORY")` + "\n", "root", "./lib", true},
		{"root escaped", rootDefinition + "_ = &root\n", "root", "./lib", true},
		{"unknown caller depth", "_, filename, _, _ := runtime.Caller(1)\nroot := filepath.Dir(filename)\n", "root", "./lib", true},
		{"unchanged directory", "", `"."`, ".", false},
		{"fresh temporary directory", "", "t.TempDir()", "./lib", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := importGraphRepository(t)
			body := "package probe\nimport (\"os\"; \"os/exec\"; \"path/filepath\"; \"runtime\"; \"testing\")\n" +
				"func TestList(t *testing.T) {\n" + test.setup +
				`c := exec.Command("go", "list", "-f", "{{.Imports}}", ` + strconv.Quote(test.target) + ")\n" +
				"c.Dir = " + test.directory + "\n_, _ = c.Output()\n}\n"
			writeFiles(t, root, map[string]string{"probe/probe.go": "package probe\n", "probe/probe_test.go": body})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			unit, ok := graph.Unit(importGraphModule + "probe")
			if !ok || unit.ImportGraphReadsFiles != test.unproven || unit.ImportGraphReadsAll != test.unproven {
				t.Fatalf("directory %s: all=%v files=%v reads=%v, want unproven=%v", test.directory, unit.ImportGraphReadsAll, unit.ImportGraphReadsFiles, unit.ImportGraphReads, test.unproven)
			}
			if test.unproven {
				assertReaderSelectedForLibEdit(t, graph, false)
				assertReaderSelectedForLibEdit(t, graph, true)
			}
		})
	}
}

// AFP-V0-9010: an existing link in testdata redirects a lexical own-testdata
// path. Both the reported intermediate alias and a linked testdata root must
// retain source-content selection. A regular fixture is the proven control.
func TestExistingTestdataSymlinkFallsBack_AFP_V0_9010(t *testing.T) {
	for _, test := range []struct {
		name, link, target, read string
	}{
		{"intermediate alias", "probe/testdata/alias", "../../lib", "testdata/alias/lib.go"},
		{"testdata root", "probe/testdata", "../lib", "testdata/lib.go"},
		{"file alias", "probe/testdata/alias.go", "../../lib/lib.go", "testdata/alias.go"},
		{"regular fixture", "", "", "testdata/fixture.txt"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := importGraphRepository(t)
			writeFiles(t, root, map[string]string{
				"probe/probe.go": "package probe\n",
				"probe/probe_test.go": "package probe\nimport (\"os\"; \"os/exec\"; \"testing\")\n" +
					"func TestList(t *testing.T) {\n" +
					`_, _ = exec.Command("go", "list", "-f", "{{.Imports}}", ".").Output()` + "\n" +
					"_, _ = os.ReadFile(" + strconv.Quote(test.read) + ")\n}\n",
			})
			if test.link != "" {
				link := filepath.Join(root, filepath.FromSlash(test.link))
				if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(test.target, link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			} else {
				writeFiles(t, root, map[string]string{"probe/testdata/fixture.txt": "fixture"})
			}
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			unit, ok := graph.Unit(importGraphModule + "probe")
			if !ok || unit.ImportGraphReadsFiles != (test.link != "") || unit.ImportGraphReadsAll != (test.link != "") {
				t.Fatalf("testdata: all=%v files=%v reads=%v", unit.ImportGraphReadsAll, unit.ImportGraphReadsFiles, unit.ImportGraphReads)
			}
			if test.link != "" {
				assertReaderSelectedForLibEdit(t, graph, false)
			}
		})
	}
}

func assertReaderSelectedForLibEdit(t *testing.T, graph *affected.Graph, imports bool) {
	t.Helper()
	base := fstest.MapFS{"go.mod": {Data: []byte("module example.test/g\n")}, "lib/lib.go": {Data: []byte("package lib\nfunc Help() string { return \"help\" }\n")}}
	body := "package lib\nfunc Help() string { return \"changed\" }\n"
	if imports {
		body = "package lib\nimport \"strings\"\nfunc Help() string { return strings.ToUpper(\"help\") }\n"
	}
	head := fstest.MapFS{"go.mod": base["go.mod"], "lib/lib.go": {Data: []byte(body)}}
	dirty := []string{"lib/lib.go"}
	changes := golang.ImportSetRecords(base, head, graph, dirty)
	for _, selection := range affected.Select(graph.WithImportSetChanges(changes), dirty).Selected {
		if selection.UnitID == importGraphModule+"probe" && selection.Witness.Kind == affected.WitnessImportGraphReader {
			return
		}
	}
	t.Fatalf("lib edit (imports=%v) lost probe's import-graph reader witness; changes=%+v", imports, changes)
}
