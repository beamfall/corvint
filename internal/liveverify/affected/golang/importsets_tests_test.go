// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"os"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-9010 (V1-5029): a new or edited test file whose imports stay in its
// package's test import set (its role's: the in-package test files' or the
// external _test package's) changes no import set, so it selects no
// import-graph reader outside its package. A test file that adds an import
// path to its role's set, carries a build constraint or a GOOS/GOARCH name,
// declares another package clause, adds an embed line or imports "C" still
// changes the set (the fault injections), as does a test file of a directory
// with no non-test file. A kept-set test file that is added, removed or
// renamed still changes its package's file set, which a reader not proven to
// observe the import graph alone (here wide, `go list -json`) is selected by.
func TestTestFileImportEdges_AFP_V0_9010(t *testing.T) {
	root := importGraphRepository(t)
	const (
		inPackage = "package lib\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc TestHelp(t *testing.T) { _ = strings.ToUpper(Help()) }\n"
		external  = "package lib_test\n\nimport (\n\t\"testing\"\n\n\t\"example.test/g/lib\"\n)\n\nfunc TestExternal(t *testing.T) { _ = lib.Help() }\n"
	)
	writeFiles(t, root, map[string]string{"lib/lib_test.go": inPackage, "lib/external_test.go": external})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	base := fstest.MapFS{}
	for _, name := range []string{"go.mod", "lib/lib.go", "lib/lib_test.go", "lib/external_test.go"} {
		body, err := os.ReadFile(root + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		base[name] = &fstest.MapFile{Data: body}
	}
	head := func(files map[string]string, removed ...string) fstest.MapFS {
		tree := fstest.MapFS{}
		for name, file := range base {
			tree[name] = file
		}
		for _, name := range removed {
			delete(tree, name)
		}
		for name, body := range files {
			tree[name] = &fstest.MapFile{Data: []byte(body)}
		}
		return tree
	}
	lib := importGraphModule + "lib"
	cases := []struct {
		name    string
		head    fstest.MapFS
		dirty   string
		changed bool
		files   bool
	}{
		{"new test file, imports in set", head(map[string]string{"lib/more_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestMore(t *testing.T) {}\n"}), "lib/more_test.go", false, true},
		{"new external test file, imports in set", head(map[string]string{"lib/more_external_test.go": "package lib_test\n\nimport (\n\t\"testing\"\n\n\t\"example.test/g/lib\"\n)\n\nfunc TestMore(t *testing.T) { _ = lib.Help() }\n"}), "lib/more_external_test.go", false, true},
		{"edited test body", head(map[string]string{"lib/lib_test.go": inPackage + "\nfunc TestOther(t *testing.T) {}\n"}), "lib/lib_test.go", false, false},
		{"test import dropped that another test file keeps", head(map[string]string{"lib/more_test.go": "package lib\n\nimport \"testing\"\n"}), "lib/more_test.go", false, true},
		{"removed test file whose imports remain", head(map[string]string{"lib/more_test.go": inPackage}, "lib/lib_test.go"), "lib/lib_test.go", false, true},
		// Fault injections: each of these must still change lib's import set.
		{"test file adds an import", head(map[string]string{"lib/more_test.go": "package lib\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n"}), "lib/more_test.go", true, false},
		{"import moves to the external role", head(map[string]string{"lib/more_external_test.go": "package lib_test\n\nimport \"strings\"\n"}), "lib/more_external_test.go", true, false},
		{"test import set shrinks", head(nil, "lib/lib_test.go"), "lib/lib_test.go", true, false},
		{"constrained test file", head(map[string]string{"lib/more_test.go": "//go:build linux\n\npackage lib\n\nimport \"testing\"\n"}), "lib/more_test.go", true, false},
		{"GOOS in the name", head(map[string]string{"lib/more_linux_test.go": "package lib\n\nimport \"testing\"\n"}), "lib/more_linux_test.go", true, false},
		{"foreign package clause", head(map[string]string{"lib/more_test.go": "package other\n\nimport \"testing\"\n"}), "lib/more_test.go", true, false},
		{"embed line", head(map[string]string{"lib/more_test.go": "package lib\n\nimport (\n\t_ \"embed\"\n\t\"testing\"\n)\n\n//go:embed lib.go\nvar source string\n"}), "lib/more_test.go", true, false},
		{"cgo test file", head(map[string]string{"lib/more_test.go": "package lib\n\nimport \"C\"\n"}), "lib/more_test.go", true, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := golang.ImportSetRecords(base, test.head, graph, []string{test.dirty})
			_, changed := got.Changed[lib]
			_, files := got.Files[lib]
			if changed != test.changed || files != test.files || got.Module != "" {
				t.Fatalf("changes = %+v, want lib changed=%v files=%v", *got, test.changed, test.files)
			}
		})
	}
	// A test file of a directory without a non-test file keeps its own
	// header line, so a new one changes the directory.
	only := fstest.MapFS{"only/a_test.go": {Data: []byte("package only\n\nimport \"testing\"\n")}}
	onlyHead := fstest.MapFS{"only/a_test.go": only["only/a_test.go"], "only/b_test.go": {Data: []byte("package only\n\nimport \"testing\"\n")}}
	if got := golang.ImportSetRecords(only, onlyHead, graph, []string{"only/b_test.go"}); len(got.Changed) == 0 && got.Module == "" {
		t.Fatalf("a new test file of a test-only directory changed nothing: %+v", *got)
	}

	// The plan: a new test file keeping the sets selects no proven reader
	// through the import graph, but still every unproven one (wide); one
	// adding an import selects the readers of lib's closure (guard lists
	// cmd/tool, built from lib) and of the module.
	guard, slice, wide := importGraphModule+"guard", importGraphModule+"slice", importGraphModule+"wide"
	writeFiles(t, root, map[string]string{"lib/more_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestMore(t *testing.T) {}\n"})
	edited, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"lib/more_test.go"}
	kept := golang.ImportSetRecords(base, os.DirFS(root), edited, dirty)
	if !reflect.DeepEqual(*kept, affected.ImportSetChanges{Changed: map[string]string{}, Files: map[string]string{lib: "lib/more_test.go"}}) {
		t.Fatalf("kept changes = %+v", *kept)
	}
	readers := func(changes *affected.ImportSetChanges) map[string]bool {
		hit := map[string]bool{}
		for _, selection := range affected.Select(edited.WithImportSetChanges(changes), dirty).Selected {
			if selection.Witness.Kind == affected.WitnessImportGraphReader {
				hit[selection.UnitID] = true
			}
		}
		return hit
	}
	hit := readers(kept)
	if hit[guard] || hit[slice] || !hit[wide] {
		t.Errorf("a new test file that keeps the import sets selects import-graph readers %v, want wide and neither guard nor slice", hit)
	}
	for id := range hit {
		if unit, _ := edited.Unit(id); !unit.ImportGraphReadsFiles {
			t.Errorf("proven reader %s selected by a file-set change alone", id)
		}
	}
	// A body edit of an existing test file keeps the file set too, so it
	// selects no reader at all.
	body := []string{"lib/lib_test.go"}
	bodyOnly := golang.ImportSetRecords(base, head(map[string]string{"lib/lib_test.go": inPackage + "\nfunc TestOther(t *testing.T) {}\n"}), edited, body)
	for _, selection := range affected.Select(edited.WithImportSetChanges(bodyOnly), body).Selected {
		if selection.Witness.Kind == affected.WitnessImportGraphReader {
			t.Errorf("a test body edit selects import-graph reader %s (changes %+v)", selection.UnitID, *bodyOnly)
		}
	}
	writeFiles(t, root, map[string]string{"lib/more_test.go": "package lib\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMore(t *testing.T) { _ = os.Args }\n"})
	added := golang.ImportSetRecords(base, os.DirFS(root), edited, dirty)
	if hit := readers(added); !hit[guard] || !hit[slice] || !hit[wide] {
		t.Errorf("a test file that adds an import selects import-graph readers %v, want guard, slice and wide (changes %+v)", hit, *added)
	}
}

// AFP-V0-9010 (the selmin review's P1): the folded header only narrows
// readers proven to observe the import graph alone. lib/a_test.go imports
// testing; names lists lib's TestGoFiles, an unproven call that observes
// file names. A new lib/b_test.go importing only testing keeps lib's import
// set, so the proven reader guard stays out, but it changes lib's file set,
// so names is selected. The fault injections: the same comparison without
// its file-set changes (the reviewed code) loses names, and a test file that
// adds an import selects both readers.
func TestFoldedTestFilesKeepUnprovenReaders_AFP_V0_9010(t *testing.T) {
	root := importGraphRepository(t)
	const (
		existing = "package lib\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n"
		names    = "package names\n\nimport (\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestNames(t *testing.T) {\n\tout, err := exec.Command(\"go\", \"list\", \"-f\", \"{{.TestGoFiles}}\", \"example.test/g/lib\").Output()\n" +
			"\tif err != nil || strings.TrimSpace(string(out)) != \"[a_test.go]\" {\n\t\tt.Fatalf(\"%s %v\", out, err)\n\t}\n}\n"
	)
	writeFiles(t, root, map[string]string{"lib/a_test.go": existing, "names/names.go": "package names\n", "names/names_test.go": names})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	lib, guard, reader := importGraphModule+"lib", importGraphModule+"guard", importGraphModule+"names"
	if unit, _ := graph.Unit(reader); !unit.ImportGraphReadsAll || !unit.ImportGraphReadsFiles {
		t.Fatalf("names: all=%v files=%v, want an unproven whole-module reader", unit.ImportGraphReadsAll, unit.ImportGraphReadsFiles)
	}
	if unit, _ := graph.Unit(guard); unit.ImportGraphReadsFiles {
		t.Fatal("guard, a proven reader, is marked as observing file names")
	}
	base := fstest.MapFS{
		"go.mod":        {Data: []byte("module example.test/g\n")},
		"lib/lib.go":    {Data: []byte("package lib\n\nfunc Help() string { return \"help\" }\n")},
		"lib/a_test.go": {Data: []byte(existing)},
	}
	with := func(files map[string]string, removed ...string) fstest.MapFS {
		tree := fstest.MapFS{}
		for name, file := range base {
			tree[name] = file
		}
		for _, name := range removed {
			delete(tree, name)
		}
		for name, body := range files {
			tree[name] = &fstest.MapFile{Data: []byte(body)}
		}
		return tree
	}
	selected := func(changes *affected.ImportSetChanges, dirty []string) map[string]affected.Witness {
		hit := map[string]affected.Witness{}
		for _, selection := range affected.Select(graph.WithImportSetChanges(changes), dirty).Selected {
			if selection.Witness.Kind == affected.WitnessImportGraphReader {
				hit[selection.UnitID] = selection.Witness
			}
		}
		return hit
	}

	for _, test := range []struct {
		name    string
		head    fstest.MapFS
		dirty   []string
		witness string
	}{
		{"duplicate-import test file added", with(map[string]string{"lib/b_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n"}), []string{"lib/b_test.go"}, "lib/b_test.go"},
		{"test file renamed", with(map[string]string{"lib/c_test.go": existing}, "lib/a_test.go"), []string{"lib/a_test.go", "lib/c_test.go"}, "lib/a_test.go"},
	} {
		t.Run(test.name, func(t *testing.T) {
			changes := golang.ImportSetRecords(base, test.head, graph, test.dirty)
			if len(changes.Changed) != 0 || changes.Module != "" || changes.Files[lib] != test.witness {
				t.Fatalf("changes = %+v, want only lib's file set changed by %s", *changes, test.witness)
			}
			hit := selected(changes, test.dirty)
			witness, found := hit[reader]
			if !found || witness.DirtyPath != test.witness || !reflect.DeepEqual(witness.Chain(), []string{lib, reader}) {
				t.Fatalf("names not selected through lib's file set: %+v (hit %v)", witness, hit)
			}
			if _, found := hit[guard]; found {
				t.Errorf("proven reader guard selected by a file-set change alone")
			}
			// Fault injection: without the file-set changes, as the
			// reviewed code compared them, names is lost.
			if _, found := selected(&affected.ImportSetChanges{Changed: changes.Changed, Module: changes.Module}, test.dirty)[reader]; found {
				t.Fatal("names selected without the file-set change: the regression proves nothing")
			}
		})
	}

	// Fault injection: a test file that adds an import changes lib's import
	// set, so the proven reader guard is selected as well.
	dirty := []string{"lib/b_test.go"}
	changes := golang.ImportSetRecords(base, with(map[string]string{"lib/b_test.go": "package lib\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n"}), graph, dirty)
	if changes.Changed[lib] != "lib/b_test.go" {
		t.Fatalf("changes = %+v, want lib's import set changed", *changes)
	}
	if hit := selected(changes, dirty); len(hit) == 0 || hit[reader].Kind == "" || hit[guard].Kind == "" {
		t.Fatalf("an added test import selects %v, want names and guard", hit)
	}
}
