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
// with no non-test file. Every dirty .go path is still the comparison's
// Source, which a reader not proven to observe the import graph alone (here
// wide, `go list -json`) is selected by.
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
	}{
		{"new test file, imports in set", head(map[string]string{"lib/more_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestMore(t *testing.T) {}\n"}), "lib/more_test.go", false},
		{"new external test file, imports in set", head(map[string]string{"lib/more_external_test.go": "package lib_test\n\nimport (\n\t\"testing\"\n\n\t\"example.test/g/lib\"\n)\n\nfunc TestMore(t *testing.T) { _ = lib.Help() }\n"}), "lib/more_external_test.go", false},
		{"edited test body", head(map[string]string{"lib/lib_test.go": inPackage + "\nfunc TestOther(t *testing.T) {}\n"}), "lib/lib_test.go", false},
		{"test import dropped that another test file keeps", head(map[string]string{"lib/more_test.go": "package lib\n\nimport \"testing\"\n"}), "lib/more_test.go", false},
		{"removed test file whose imports remain", head(map[string]string{"lib/more_test.go": inPackage}, "lib/lib_test.go"), "lib/lib_test.go", false},
		// Fault injections: each of these must still change lib's import set.
		{"test file adds an import", head(map[string]string{"lib/more_test.go": "package lib\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n"}), "lib/more_test.go", true},
		{"import moves to the external role", head(map[string]string{"lib/more_external_test.go": "package lib_test\n\nimport \"strings\"\n"}), "lib/more_external_test.go", true},
		{"test import set shrinks", head(nil, "lib/lib_test.go"), "lib/lib_test.go", true},
		{"constrained test file", head(map[string]string{"lib/more_test.go": "//go:build linux\n\npackage lib\n\nimport \"testing\"\n"}), "lib/more_test.go", true},
		{"GOOS in the name", head(map[string]string{"lib/more_linux_test.go": "package lib\n\nimport \"testing\"\n"}), "lib/more_linux_test.go", true},
		{"foreign package clause", head(map[string]string{"lib/more_test.go": "package other\n\nimport \"testing\"\n"}), "lib/more_test.go", true},
		{"embed line", head(map[string]string{"lib/more_test.go": "package lib\n\nimport (\n\t_ \"embed\"\n\t\"testing\"\n)\n\n//go:embed lib.go\nvar source string\n"}), "lib/more_test.go", true},
		{"cgo test file", head(map[string]string{"lib/more_test.go": "package lib\n\nimport \"C\"\n"}), "lib/more_test.go", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := golang.ImportSetRecords(base, test.head, graph, []string{test.dirty})
			_, changed := got.Changed[lib]
			if changed != test.changed || got.Source != test.dirty || got.Module != "" {
				t.Fatalf("changes = %+v, want lib changed=%v source=%s", *got, test.changed, test.dirty)
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
	if !reflect.DeepEqual(*kept, affected.ImportSetChanges{Changed: map[string]string{}, Source: "lib/more_test.go"}) {
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
			t.Errorf("proven reader %s selected by a kept import set", id)
		}
	}
	// A body edit of an existing test file selects no proven reader, and
	// still the unproven wide (AFP-V0-9010).
	body := []string{"lib/lib_test.go"}
	bodyOnly := golang.ImportSetRecords(base, head(map[string]string{"lib/lib_test.go": inPackage + "\nfunc TestOther(t *testing.T) {}\n"}), edited, body)
	for _, selection := range affected.Select(edited.WithImportSetChanges(bodyOnly), body).Selected {
		if unit, _ := edited.Unit(selection.UnitID); selection.Witness.Kind == affected.WitnessImportGraphReader && !unit.ImportGraphReadsFiles {
			t.Errorf("a test body edit selects proven import-graph reader %s (changes %+v)", selection.UnitID, *bodyOnly)
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
// set, so the proven reader guard stays out, but names is selected through
// the comparison's Source. The fault injections: the same comparison without
// its Source loses names, and a test file that adds an import selects both
// readers.
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
			if len(changes.Changed) != 0 || changes.Module != "" || changes.Source != test.witness {
				t.Fatalf("changes = %+v, want only the source %s", *changes, test.witness)
			}
			hit := selected(changes, test.dirty)
			witness, found := hit[reader]
			if chain := witness.Chain(); !found || witness.DirtyPath != test.witness || len(chain) == 0 || chain[len(chain)-1] != reader {
				t.Fatalf("names not selected through the source: %+v (hit %v)", witness, hit)
			}
			if _, found := hit[guard]; found {
				t.Errorf("proven reader guard selected by a kept import set")
			}
			// Fault injection: without the Source, names is lost.
			if _, found := selected(&affected.ImportSetChanges{Changed: changes.Changed, Module: changes.Module}, test.dirty)[reader]; found {
				t.Fatal("names selected without the source: the regression proves nothing")
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

// AFP-V0-9010 (the selmin fix-round-2 review's P1): docs builds its target
// path at run time and checks `go list -f '{{.Doc}}'`, an unproven call, so
// the target names no edge and docs imports nothing of the module. An edit
// of lib's package comment alone keeps every header, yet changes what docs
// reads: docs is selected through the Source, with the chain [lib, docs],
// while the proven reader guard stays out. Fault injection: the same
// comparison without its Source loses docs.
func TestPackageDocReaderSelected_AFP_V0_9010(t *testing.T) {
	root := importGraphRepository(t)
	const docs = "package docs\n\nimport (\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
		"func TestDoc(t *testing.T) {\n\ttarget := strings.Join([]string{\"example.test\", \"g\", \"lib\"}, \"/\")\n" +
		"\tout, err := exec.Command(\"go\", \"list\", \"-f\", \"{{.Doc}}\", target).Output()\n" +
		"\tif err != nil || strings.TrimSpace(string(out)) != \"Package lib helps.\" {\n\t\tt.Fatalf(\"%s %v\", out, err)\n\t}\n}\n"
	const before = "// Package lib helps.\npackage lib\n\nfunc Help() string { return \"help\" }\n"
	writeFiles(t, root, map[string]string{"lib/lib.go": before, "docs/docs.go": "package docs\n", "docs/docs_test.go": docs})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	lib, guard, reader := importGraphModule+"lib", importGraphModule+"guard", importGraphModule+"docs"
	if unit, _ := graph.Unit(reader); !unit.ImportGraphReadsAll || !unit.ImportGraphReadsFiles || len(unit.Imports) != 0 {
		t.Fatalf("docs: all=%v files=%v imports=%v, want an unproven whole-module reader importing nothing", unit.ImportGraphReadsAll, unit.ImportGraphReadsFiles, unit.Imports)
	}
	base := fstest.MapFS{"go.mod": {Data: []byte("module example.test/g\n")}, "lib/lib.go": {Data: []byte(before)}}
	head := fstest.MapFS{"go.mod": base["go.mod"], "lib/lib.go": {Data: []byte("// Package lib helps less.\npackage lib\n\nfunc Help() string { return \"help\" }\n")}}
	dirty := []string{"lib/lib.go"}
	changes := golang.ImportSetRecords(base, head, graph, dirty)
	if len(changes.Changed) != 0 || changes.Module != "" || changes.Source != "lib/lib.go" {
		t.Fatalf("changes = %+v, want no import-set change and the source lib/lib.go", *changes)
	}
	hit := map[string]affected.Witness{}
	for _, selection := range affected.Select(graph.WithImportSetChanges(changes), dirty).Selected {
		if selection.Witness.Kind == affected.WitnessImportGraphReader {
			hit[selection.UnitID] = selection.Witness
		}
	}
	if witness, found := hit[reader]; !found || witness.DirtyPath != "lib/lib.go" || !reflect.DeepEqual(witness.Chain(), []string{lib, reader}) {
		t.Fatalf("docs not selected by a package comment edit: %+v (hit %v)", witness, hit)
	}
	if _, found := hit[guard]; found {
		t.Error("proven reader guard selected by a package comment edit")
	}
	// Fault injection: without the Source, docs is lost.
	for _, selection := range affected.Select(graph.WithImportSetChanges(&affected.ImportSetChanges{Changed: changes.Changed}), dirty).Selected {
		if selection.UnitID == reader {
			t.Fatal("docs selected without the source: the regression proves nothing")
		}
	}
}

// AFP-V0-9010 (the orchestrator's rule): a reader is narrowed by headers only
// while every file of its package is proven to observe nothing but its
// proven `go list` output. Each case adds one file to a package whose only
// call is the proven `go list -deps -f '{{.ImportPath}}' ./cmd/tool`; a
// channel the classifier cannot rule out (another command, a go/ parser,
// build information, embedded or read files, a module helper whose code is
// not inspected) makes it an unproven whole-module reader, which a body-only
// edit of lib then selects. The controls (a literal own-testdata read, an
// environment read) stay proven, and the body edit does not select them.
// Every unproven case but the unparsed file is selected only as an
// import-graph reader: no other edge reaches probe.
func TestUnknownReaderFallsBackToRun_AFP_V0_9010(t *testing.T) {
	const list = "package probe\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func TestList(t *testing.T) {\n\t_, _ = exec.Command(\"go\", \"list\", \"-deps\", \"-f\", \"{{.ImportPath}}\", \"./cmd/tool\").Output()\n}\n"
	cases := []struct {
		name     string
		extra    string
		unproven bool
	}{
		{"go build", "package probe\n\nimport \"os/exec\"\n\nfunc build() error { return exec.Command(\"go\", \"build\", \"./cmd/other\").Run() }\n", true},
		{"computed command", "package probe\n\nimport \"os/exec\"\n\nfunc run(name string) error { return exec.Command(name, \"list\").Run() }\n", true},
		{"git reads the tree", "package probe\n\nimport \"os/exec\"\n\nfunc show() error { return exec.Command(\"git\", \"show\", \"HEAD:lib/lib.go\").Run() }\n", true},
		{"go/parser", "package probe\n\nimport \"go/parser\"\n\nvar _ = parser.ParseComments\n", true},
		{"go/build", "package probe\n\nimport \"go/build\"\n\nvar _ = build.Default\n", true},
		{"build info", "package probe\n\nimport \"runtime/debug\"\n\nvar _, _ = debug.ReadBuildInfo()\n", true},
		{"embed", "package probe\n\nimport _ \"embed\"\n\n//go:embed probe.go\nvar source string\n", true},
		{"read outside testdata", "package probe\n\nimport \"os\"\n\nvar _, _ = os.ReadFile(\"../lib/lib.go\")\n", true},
		{"climbing testdata", "package probe\n\nimport \"os\"\n\nvar _, _ = os.ReadFile(\"testdata/../../lib/lib.go\")\n", true},
		{"computed read", "package probe\n\nimport \"os\"\n\nfunc read(name string) ([]byte, error) { return os.ReadFile(name) }\n", true},
		{"read as a value", "package probe\n\nimport \"os\"\n\nvar reader = os.ReadFile\n", true},
		{"walk", "package probe\n\nimport \"path/filepath\"\n\nvar _ = filepath.WalkDir(\"..\", nil)\n", true},
		{"dot import", "package probe\n\nimport . \"os\"\n\nvar _, _ = ReadFile(\"x\")\n", true},
		{"module helper", "package probe\n\nimport \"example.test/g/base\"\n\nvar _ = base.Name()\n", true},
		{"template files", "package probe\n\nimport \"text/template\"\n\nvar _, _ = template.New(\"x\").ParseGlob(\"../lib/*.go\")\n", true},
		{"does not parse", "package probe\n\nfunc {\n", true},
		{"cgo", "package probe\n\n// #include <stdio.h>\nimport \"C\"\n", true},
		{"linkname", "package probe\n\nimport _ \"unsafe\"\n\n//go:linkname read os.ReadFile\nfunc read(string) ([]byte, error)\n", true},
		{"command path swapped", "package probe\n\nimport \"os/exec\"\n\nfunc list() error {\n\tc := exec.Command(\"go\", \"list\", \"-deps\", \"-f\", \"{{.ImportPath}}\", \"./cmd/tool\")\n\tc.Path = \"/usr/bin/git\"\n\treturn c.Run()\n}\n", true},
		{"zip reader", "package probe\n\nimport \"archive/zip\"\n\nvar _, _ = zip.OpenReader(\"../lib.zip\")\n", true},
		{"own testdata", "package probe\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\n\nvar _, _ = os.ReadFile(filepath.Join(\"testdata\", \"fixture.txt\"))\nvar _, _ = os.ReadDir(\"testdata\")\n", false},
		{"environment", "package probe\n\nimport \"os\"\n\nvar _ = os.Getenv(\"HOME\")\n", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := importGraphRepository(t)
			writeFiles(t, root, map[string]string{"probe/probe.go": "package probe\n", "probe/probe_test.go": list, "probe/extra_test.go": test.extra})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			reader := importGraphModule + "probe"
			unit, _ := graph.Unit(reader)
			if unit.ImportGraphReadsFiles != test.unproven || unit.ImportGraphReadsAll != test.unproven || !test.unproven && len(unit.ImportGraphReads) == 0 {
				t.Fatalf("probe: all=%v files=%v reads=%v, want unproven=%v", unit.ImportGraphReadsAll, unit.ImportGraphReadsFiles, unit.ImportGraphReads, test.unproven)
			}
			base := fstest.MapFS{"go.mod": {Data: []byte("module example.test/g\n")}, "lib/lib.go": {Data: []byte("package lib\n\nfunc Help() string { return \"help\" }\n")}}
			head := fstest.MapFS{"go.mod": base["go.mod"], "lib/lib.go": {Data: []byte("package lib\n\nfunc Help() string { return \"more help\" }\n")}}
			dirty := []string{"lib/lib.go"}
			changes := golang.ImportSetRecords(base, head, graph, dirty)
			var witness affected.Witness
			for _, selection := range affected.Select(graph.WithImportSetChanges(changes), dirty).Selected {
				if selection.UnitID == reader {
					witness = selection.Witness
				}
			}
			// An unparsed file also makes the plugin's other edges
			// conservative, so any witness runs probe.
			if selected := witness.Kind != ""; selected != test.unproven {
				t.Fatalf("a body-only edit of lib selected probe by %+v, want selected=%v (changes %+v)", witness, test.unproven, *changes)
			}
		})
	}
}

// AFP-V0-9010: the Source an unproven reader is selected through is the
// least dirty path the go command or a source parser may observe: a .go
// file (testdata included), a file the go command lists by another suffix,
// or any file below an embedding package, whose content its build reads. A
// path of none of these kinds gives no Source, so it selects no unproven
// reader through the import graph.
func TestSourcePath_AFP_V0_9010(t *testing.T) {
	graph, err := affected.Build(context.Background(), importGraphRepository(t), golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		dirty []string
		want  string
	}{
		{[]string{"lib/lib.go"}, "lib/lib.go"},
		{[]string{"lib/testdata/fixture.go"}, "lib/testdata/fixture.go"},
		{[]string{"lib/asm_amd64.s"}, "lib/asm_amd64.s"},
		{[]string{"lib/blob.syso"}, "lib/blob.syso"},
		{[]string{"assets/data.txt"}, "assets/data.txt"},
		{[]string{"z/z.go", "lib/helper.c"}, "lib/helper.c"},
		{[]string{"README.md"}, ""},
		{[]string{"lib/notes.txt"}, ""},
		{nil, ""},
	} {
		if got := graph.SourcePath(test.dirty); got != test.want {
			t.Errorf("SourcePath(%q) = %q, want %q", test.dirty, got, test.want)
		}
	}
	// Both directions through the plan: an assembly file of base (which
	// cmd/tool, the command wide builds, does not import) selects the
	// unproven reader wide, and a README edit selects no reader.
	for _, test := range []struct {
		dirty string
		wide  bool
	}{{"base/asm_amd64.s", true}, {"README.md", false}} {
		dirty := []string{test.dirty}
		changes := golang.ImportSetRecords(fstest.MapFS{}, fstest.MapFS{}, graph, dirty)
		wide := false
		for _, selection := range affected.Select(graph.WithImportSetChanges(changes), dirty).Selected {
			if selection.UnitID == importGraphModule+"wide" && selection.Witness.Kind == affected.WitnessImportGraphReader {
				wide = true
			}
		}
		if wide != test.wide {
			t.Errorf("%s selects wide=%v, want %v (changes %+v)", test.dirty, wide, test.wide, *changes)
		}
	}
}
