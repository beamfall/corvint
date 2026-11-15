// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// gateselectImporterFiles mirrors the repository shape V1-4362 observed:
// cmd/corvint imports internal/gateselect directly and through
// internal/sessiontest; internal/analyzerpython's modularity test lists
// cmd/corvint's dependency closure with `go list -deps`, as it did before
// it moved to internal/closureguard; tools/corvint-pr-tests builds and runs
// cmd/corvint; internal/unrelated reaches none of it.
func gateselectImporterFiles() map[string]string {
	return map[string]string{
		"go.mod":                        "module example.test/c\n\ngo 1.27\n",
		"internal/gateselect/select.go": gateselectBase,
		"internal/sessiontest/plan.go":  "package sessiontest\n\nimport \"example.test/c/internal/gateselect\"\n\nfunc Plan() string { return gateselect.Select(\" plan \") }\n",
		"cmd/corvint/main.go":           "package main\n\nimport (\n\t\"example.test/c/internal/gateselect\"\n\t\"example.test/c/internal/sessiontest\"\n)\n\nfunc main() { println(gateselect.Select(sessiontest.Plan())) }\n",
		"internal/analyzerpython/a.go":  "package analyzerpython\n",
		"internal/analyzerpython/modularity_test.go": "package analyzerpython\n\nimport (\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestCoreOmitsAnalyzer(t *testing.T) {\n\tout, err := exec.Command(\"go\", \"list\", \"-deps\", \"-f\", \"{{.ImportPath}}\", \"./cmd/corvint\").Output()\n" +
			"\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif strings.Contains(string(out), \"internal/analyzerpython\") {\n\t\tt.Fatal(\"cmd/corvint imports internal/analyzerpython\")\n\t}\n}\n",
		"tools/corvint-pr-tests/run.go": "package prtests\n",
		"tools/corvint-pr-tests/run_test.go": "package prtests\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestBinary(t *testing.T) {\n\tif err := exec.Command(\"go\", \"build\", \"-o\", \"corvint\", \"./cmd/corvint\").Run(); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n",
		"internal/unrelated/u.go":      "package unrelated\n",
		"internal/unrelated/u_test.go": "package unrelated\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n",
	}
}

const gateselectBase = "package gateselect\n\nimport \"strings\"\n\nfunc Select(s string) string { return strings.TrimSpace(s) }\n"

// AFP-V0-2201 and AFP-V0-2202, V1-4362 AC2: on the gateselect ->
// cmd/corvint -> analyzerpython shape, a body-only edit to
// internal/gateselect selects neither analyzerpython (no IMPORT_GRAPH_READER
// and no BINARY_EXEC through cmd/corvint) nor anything unrelated, while the
// unit that builds and runs cmd/corvint keeps its BINARY_EXEC selection. An
// added import and a removed import each select analyzerpython through
// gateselect -> cmd/corvint, and with no measured comparison the reader is
// selected (fail-closed).
func TestGateselectBodyEditSkipsAnalyzerpython_AFPV02201(t *testing.T) {
	files := gateselectImporterFiles()
	root := t.TempDir()
	writeFiles(t, root, files)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	const module = "go:example.test/c/"
	gateselect, corvint, analyzer := module+"internal/gateselect", module+"cmd/corvint", module+"internal/analyzerpython"
	runner, unrelated := module+"tools/corvint-pr-tests", module+"internal/unrelated"
	unit, known := graph.Unit(analyzer)
	if !known {
		t.Fatalf("%s is not a unit", analyzer)
	}
	if unit.ImportGraphReadsAll || !reflect.DeepEqual(unit.ImportGraphReads, []string{corvint}) {
		t.Fatalf("analyzerpython reads all=%v %v, want only %s", unit.ImportGraphReadsAll, unit.ImportGraphReads, corvint)
	}
	base := fstest.MapFS{}
	for name, body := range files {
		base[name] = &fstest.MapFile{Data: []byte(body)}
	}
	head := func(source string) fstest.MapFS {
		tree := fstest.MapFS{}
		for name, file := range base {
			tree[name] = file
		}
		tree["internal/gateselect/select.go"] = &fstest.MapFile{Data: []byte(source)}
		return tree
	}
	const dirty = "internal/gateselect/select.go"
	measure := func(source string) *affected.ImportSetChanges {
		return golang.ImportSetRecords(base, head(source), graph, []string{dirty})
	}
	cases := []struct {
		name     string
		changes  *affected.ImportSetChanges
		selected map[string]string
		excluded []string
	}{
		{
			name:     "body-only edit",
			changes:  measure("package gateselect\n\nimport \"strings\"\n\nfunc Select(s string) string { return strings.ToLower(strings.TrimSpace(s)) }\n"),
			selected: map[string]string{runner: affected.WitnessBinaryExec},
			excluded: []string{analyzer, unrelated},
		},
		{
			name:     "import added",
			changes:  measure("package gateselect\n\nimport (\n\t\"sort\"\n\t\"strings\"\n)\n\nfunc Select(s string) string { f := strings.Fields(s); sort.Strings(f); return strings.Join(f, \" \") }\n"),
			selected: map[string]string{analyzer: affected.WitnessImportGraphReader, runner: affected.WitnessBinaryExec},
			excluded: []string{unrelated},
		},
		{
			name:     "import removed",
			changes:  measure("package gateselect\n\nfunc Select(s string) string { return s }\n"),
			selected: map[string]string{analyzer: affected.WitnessImportGraphReader, runner: affected.WitnessBinaryExec},
			excluded: []string{unrelated},
		},
		{
			name:     "no comparison",
			selected: map[string]string{analyzer: affected.WitnessImportGraphReader, runner: affected.WitnessBinaryExec},
			excluded: []string{unrelated},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			plan := affected.Select(graph.WithImportSetChanges(test.changes), []string{dirty})
			witnesses := map[string]affected.Witness{}
			for _, selection := range plan.Selected {
				witnesses[selection.UnitID] = selection.Witness
			}
			for id, kind := range test.selected {
				if witnesses[id].Kind != kind {
					t.Errorf("%s witness = %+v, want %s", id, witnesses[id], kind)
				}
			}
			for _, id := range test.excluded {
				if witness, hit := witnesses[id]; hit {
					t.Errorf("%s selected by %+v", id, witness)
				}
			}
			if test.name == "import added" {
				if want := []string{gateselect, corvint, analyzer}; !reflect.DeepEqual(witnesses[analyzer].Via, want) {
					t.Errorf("analyzerpython via = %v, want %v", witnesses[analyzer].Via, want)
				}
			}
		})
	}
	if got := measure(gateselectBase + "\n// comment only\n"); len(got.Changed) != 0 {
		t.Errorf("comment-only edit changed import sets %v", got.Changed)
	}
}
