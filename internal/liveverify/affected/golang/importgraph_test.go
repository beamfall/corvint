package golang_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const importGraphModule = "go:example.test/g/"

// importGraphRepository is a module with two commands: cmd/tool built from
// lib, and cmd/other built from base. guard's test lists cmd/tool's
// dependency closure with all-literal `go list` arguments; slice lists it
// through a local []string extended by append with computed names, as
// internal/swiftimpact does; runner builds cmd/tool and runs it; mixed both
// lists cmd/tool and builds it; wide lists cmd/tool with -json, which
// observes more than the import graph; unrelated lists nothing.
func importGraphRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":            "module example.test/g\n",
		"lib/lib.go":        "package lib\n\nfunc Help() string { return \"help\" }\n",
		"base/base.go":      "package base\n\nfunc Name() string { return \"base\" }\n",
		"cmd/tool/main.go":  "package main\n\nimport \"example.test/g/lib\"\n\nfunc main() { println(lib.Help()) }\n",
		"cmd/other/main.go": "package main\n\nimport \"example.test/g/base\"\n\nfunc main() { println(base.Name()) }\n",
		"guard/guard.go":    "package guard\n",
		"guard/guard_test.go": "package guard\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestClosure(t *testing.T) {\n\tout, err := exec.Command(\"go\", \"list\", \"-deps\", \"-f\", \"{{.ImportPath}} {{join .Imports \\\" \\\"}}\", \"./cmd/tool\").Output()\n\t_, _ = out, err\n}\n",
		"slice/slice.go": "package slice\n",
		"slice/slice_test.go": "package slice\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestClosure(t *testing.T) {\n\tconst module = \"example.test/g/\"\n\targs := []string{\"list\", \"-f\", \"{{.ImportPath}} {{join .Deps \\\" \\\"}}\", module + \"cmd/tool\"}\n" +
			"\tfor _, name := range []string{\"other\"} {\n\t\targs = append(args, module+\"cmd/\"+name)\n\t}\n\t_, _ = exec.Command(\"go\", args...).Output()\n}\n",
		"runner/runner.go": "package runner\n",
		"runner/runner_test.go": "package runner\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestRun(t *testing.T) {\n\t_ = exec.Command(\"go\", \"build\", \"-o\", \"bin\", \"./cmd/tool\").Run()\n}\n",
		"mixed/mixed.go": "package mixed\n",
		"mixed/mixed_test.go": "package mixed\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestBoth(t *testing.T) {\n\t_ = exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\").Run()\n\t_ = exec.Command(\"go\", \"build\", \"./cmd/tool\").Run()\n}\n",
		"wide/wide.go": "package wide\n",
		"wide/wide_test.go": "package wide\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestJSON(t *testing.T) {\n\t_ = exec.Command(\"go\", \"list\", \"-json\", \"./cmd/tool\").Run()\n}\n",
		"dot/dot.go": "package dot\n",
		"dot/dot_test.go": "package dot\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestWhole(t *testing.T) {\n\t_ = exec.Command(\"go\", \"list\", \"-deps\", \"-f\", \"{{.}}\", \"./cmd/tool\").Run()\n}\n",
		"envset/envset.go": "package envset\n",
		"envset/envset_test.go": "package envset\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestEnv(t *testing.T) {\n\tvalue := os.Getenv(\"EXTRA\")\n\tcmd := exec.Command(\"go\", \"list\", \"./cmd/tool\")\n\tcmd.Env = append(os.Environ(), value)\n\t_ = cmd.Run()\n}\n",
		"envok/envok.go": "package envok\n",
		"envok/envok_test.go": "package envok\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestEnv(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\")\n\tcmd.Env = append(os.Environ(), \"GOTOOLCHAIN=local\")\n\t_ = cmd.Run()\n}\n",
		"chain/chain.go": "package chain\n",
		"chain/chain_test.go": "package chain\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestChain(t *testing.T) {\n\tout, _ := exec.Command(\"go\", \"list\", \"./cmd/tool\").Output()\n\t_ = exec.Command(\"go\", \"vet\", string(out)).Run()\n}\n",
		"flags/flags.go": "package flags\n\nconst variable = \"GOFLAGS\"\n",
		"flags/flags_test.go": "package flags\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestFlags(t *testing.T) {\n\t_ = exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\").Run()\n}\n",
		"helper/helper.go": "package helper\n",
		"helper/helper_test.go": "package helper\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func listed() ([]byte, error) {\n\treturn exec.Command(\"go\", \"list\", \"./cmd/tool\").Output()\n}\n\n" +
			"func TestHelper(t *testing.T) {\n\t_, _ = listed()\n}\n",
		"handoff/handoff.go": "package handoff\n",
		"handoff/handoff_test.go": "package handoff\n\nimport (\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func build(t *testing.T, names []string) {}\n\n" +
			"func TestHandoff(t *testing.T) {\n\tout, _ := exec.Command(\"go\", \"list\", \"./cmd/tool\").Output()\n\tnames := strings.Fields(string(out))\n\tbuild(t, names)\n}\n",
		"subtest/subtest.go": "package subtest\n",
		"subtest/subtest_test.go": "package subtest\n\nimport (\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestSub(t *testing.T) {\n\tout, err := exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\").Output()\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n" +
			"\tseen := map[string]bool{}\n\tfor _, line := range strings.Split(string(out), \"\\n\") {\n\t\tseen[line] = true\n\t}\n" +
			"\tt.Run(\"each\", func(t *testing.T) {\n\t\tif !seen[\"x\"] {\n\t\t\tt.Errorf(\"missing %v\", seen)\n\t\t}\n\t})\n}\n",
		"assets/assets.go":            "package assets\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar Data string\n",
		"assets/data.txt":             "data\n",
		"unrelated/unrelated.go":      "package unrelated\n",
		"unrelated/unrelated_test.go": "package unrelated\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n",
	})
	return root
}

// AFP-V0-2200: a literal that is only an argument of a proven `go list` call
// adds an import-graph edge, not a command edge; computed list arguments
// make the package a reader of the whole module; a build, a call that lists
// more than the import graph, or a listing that escapes its function to a
// caller or helper keeps the command edge.
func TestGoListReadersAreImportGraphReaders_AFPV02200(t *testing.T) {
	root := importGraphRepository(t)
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	tool := importGraphModule + "cmd/tool"
	type edges struct {
		execs []string
		reads []string
		all   bool
	}
	got := map[string]edges{}
	for _, unit := range result.Units {
		if unit.Execs != nil || unit.ImportGraphReads != nil || unit.ImportGraphReadsAll {
			got[unit.ID] = edges{unit.Execs, unit.ImportGraphReads, unit.ImportGraphReadsAll}
		}
	}
	want := map[string]edges{
		importGraphModule + "guard":  {reads: []string{tool}},
		importGraphModule + "slice":  {all: true},
		importGraphModule + "runner": {execs: []string{tool}},
		importGraphModule + "mixed":  {execs: []string{tool}, reads: []string{tool}},
		importGraphModule + "wide":   {execs: []string{tool}, all: true},
		// A bare dot prints the whole package, an Env assignment can add
		// GOFLAGS=-json, a listed name may feed another command, and a
		// package naming GOFLAGS can change every call: none is proven.
		importGraphModule + "dot":    {execs: []string{tool}, all: true},
		importGraphModule + "envset": {execs: []string{tool}, all: true},
		// A literal environment without GOFLAGS or GOENV keeps the proof.
		importGraphModule + "envok": {reads: []string{tool}},
		importGraphModule + "chain": {execs: []string{tool}, all: true},
		importGraphModule + "flags": {execs: []string{tool}, all: true},
		// A listing returned by a helper, or passed to a package helper,
		// may become a caller's build or test argument: not proven.
		importGraphModule + "helper":  {execs: []string{tool}, all: true},
		importGraphModule + "handoff": {execs: []string{tool}, all: true},
		// Output kept in locals and read by a subtest stays proven.
		importGraphModule + "subtest": {reads: []string{tool}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("edges = %+v\nwant    %+v", got, want)
	}
}

// AFP-V0-2201 and AFP-V0-2202: a body-only edit to lib, which cmd/tool
// imports, selects the binary runners but neither listing reader (the
// narrowing); an import added to lib, an import change in cmd/other's
// closure, a go.mod edit and an unknown comparison each select the readers
// whose listing can change (the negative controls).
func TestImportGraphReadersFollowImportSetChanges_AFPV02201(t *testing.T) {
	root := importGraphRepository(t)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	guard, slice, runner, mixed, wide := importGraphModule+"guard", importGraphModule+"slice", importGraphModule+"runner", importGraphModule+"mixed", importGraphModule+"wide"
	cases := []struct {
		name     string
		dirty    string
		changes  *affected.ImportSetChanges
		selected map[string]string
		excluded []string
	}{
		{
			name:     "body-only edit in the listed closure",
			dirty:    "lib/lib.go",
			changes:  &affected.ImportSetChanges{Changed: map[string]string{}},
			selected: map[string]string{runner: affected.WitnessBinaryExec, mixed: affected.WitnessBinaryExec, wide: affected.WitnessBinaryExec},
			excluded: []string{guard, slice},
		},
		{
			name:     "import added in the listed closure",
			dirty:    "lib/lib.go",
			changes:  &affected.ImportSetChanges{Changed: map[string]string{importGraphModule + "lib": "lib/lib.go"}},
			selected: map[string]string{guard: affected.WitnessImportGraphReader, slice: affected.WitnessImportGraphReader, runner: affected.WitnessBinaryExec},
		},
		{
			name:     "import change outside the named closure",
			dirty:    "base/base.go",
			changes:  &affected.ImportSetChanges{Changed: map[string]string{importGraphModule + "base": "base/base.go"}},
			selected: map[string]string{slice: affected.WitnessImportGraphReader, wide: affected.WitnessImportGraphReader},
			excluded: []string{guard, runner},
		},
		{
			name:     "go.mod",
			dirty:    "go.mod",
			changes:  &affected.ImportSetChanges{Changed: map[string]string{}, Module: "go.mod"},
			selected: map[string]string{guard: affected.WitnessImportGraphReader, slice: affected.WitnessImportGraphReader},
		},
		{
			name:     "no comparison",
			dirty:    "lib/lib.go",
			selected: map[string]string{guard: affected.WitnessImportGraphReader, slice: affected.WitnessImportGraphReader, runner: affected.WitnessBinaryExec},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			plan := affected.Select(graph.WithImportSetChanges(test.changes), []string{test.dirty})
			kinds := map[string]affected.Witness{}
			for _, selection := range plan.Selected {
				kinds[selection.UnitID] = selection.Witness
			}
			for id, kind := range test.selected {
				if kinds[id].Kind != kind {
					t.Errorf("%s witness = %+v, want %s", id, kinds[id], kind)
				}
			}
			for _, id := range test.excluded {
				if witness, hit := kinds[id]; hit {
					t.Errorf("%s selected by %+v", id, witness)
				}
			}
		})
	}
	plan := affected.Select(graph.WithImportSetChanges(&affected.ImportSetChanges{Changed: map[string]string{importGraphModule + "lib": "lib/lib.go"}}), []string{"lib/lib.go"})
	for _, selection := range plan.Selected {
		if selection.UnitID == guard {
			if want := []string{importGraphModule + "lib", importGraphModule + "cmd/tool", guard}; !reflect.DeepEqual(selection.Witness.Via, want) {
				t.Errorf("guard via = %v, want %v", selection.Witness.Via, want)
			}
		}
	}
}

// AFP-V0-2202: the comparison names a package whose header changed (an
// import, a build constraint, the package clause, a file added or removed)
// and no package whose only change is a body; a module file or a changed
// directory with no package left changes the module; a testdata Go file is
// no package file.
func TestImportSetRecordsCompareHeaders_AFPV02202(t *testing.T) {
	root := importGraphRepository(t)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	lib := "package lib\n\nfunc Help() string { return \"help\" }\n"
	base := fstest.MapFS{
		"go.mod":                  {Data: []byte("module example.test/g\n")},
		"lib/lib.go":              {Data: []byte(lib)},
		"base/base.go":            {Data: []byte("package base\n")},
		"gone/gone.go":            {Data: []byte("package gone\n")},
		"lib/testdata/fixture.go": {Data: []byte("package fixture\n")},
		"assets/assets.go":        {Data: []byte("package assets\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar Data string\n")},
		"assets/data.txt":         {Data: []byte("data\n")},
	}
	head := func(files map[string]string) fstest.MapFS {
		tree := fstest.MapFS{}
		for name, file := range base {
			tree[name] = file
		}
		delete(tree, "gone/gone.go")
		for name, body := range files {
			tree[name] = &fstest.MapFile{Data: []byte(body)}
		}
		return tree
	}
	libID, assetsID := importGraphModule+"lib", importGraphModule+"assets"
	without := func(tree fstest.MapFS, name string) fstest.MapFS {
		delete(tree, name)
		return tree
	}
	cases := []struct {
		name  string
		head  fstest.MapFS
		dirty []string
		want  affected.ImportSetChanges
	}{
		{"body", head(map[string]string{"lib/lib.go": "package lib\n\nfunc Help() string { return \"other\" }\n"}), []string{"lib/lib.go"}, affected.ImportSetChanges{Changed: map[string]string{}}},
		{"import", head(map[string]string{"lib/lib.go": "package lib\n\nimport \"strings\"\n\nfunc Help() string { return strings.ToUpper(\"help\") }\n"}), []string{"lib/lib.go"}, affected.ImportSetChanges{Changed: map[string]string{libID: "lib/lib.go"}}},
		{"constraint", head(map[string]string{"lib/lib.go": "//go:build linux\n\n" + lib}), []string{"lib/lib.go"}, affected.ImportSetChanges{Changed: map[string]string{libID: "lib/lib.go"}}},
		{"added file", head(map[string]string{"lib/extra.go": "package lib\n"}), []string{"lib/extra.go"}, affected.ImportSetChanges{Changed: map[string]string{libID: "lib/extra.go"}}},
		{"test import", head(map[string]string{"lib/lib_test.go": "package lib\n\nimport \"testing\"\n"}), []string{"lib/lib_test.go"}, affected.ImportSetChanges{Changed: map[string]string{libID: "lib/lib_test.go"}}},
		{"go.mod", head(map[string]string{"go.mod": "module example.test/g\n\ngo 1.27\n"}), []string{"go.mod"}, affected.ImportSetChanges{Changed: map[string]string{}, Module: "go.mod"}},
		{"removed package", head(nil), []string{"gone/gone.go"}, affected.ImportSetChanges{Changed: map[string]string{}, Module: "gone/gone.go"}},
		{"testdata", head(map[string]string{"lib/testdata/fixture.go": "package fixture\n\nimport \"os\"\n"}), []string{"lib/testdata/fixture.go"}, affected.ImportSetChanges{Changed: map[string]string{}}},
		// go list fails on an embed pattern that matches nothing: a new
		// embed line and a removed embedded file change the import set,
		// an edited embedded file does not.
		{"embed line", head(map[string]string{"assets/assets.go": "package assets\n\nimport _ \"embed\"\n\n//go:embed data.txt more.txt\nvar Data string\n"}), []string{"assets/assets.go"}, affected.ImportSetChanges{Changed: map[string]string{assetsID: "assets/assets.go"}}},
		{"embedded file removed", without(head(nil), "assets/data.txt"), []string{"assets/data.txt"}, affected.ImportSetChanges{Changed: map[string]string{assetsID: "assets/data.txt"}}},
		{"embedded file edited", head(map[string]string{"assets/data.txt": "other\n"}), []string{"assets/data.txt"}, affected.ImportSetChanges{Changed: map[string]string{}}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := golang.ImportSetRecords(base, test.head, graph, test.dirty)
			if !reflect.DeepEqual(*got, test.want) {
				t.Fatalf("changes = %+v, want %+v", *got, test.want)
			}
		})
	}
}

// hiddenImportFiles is a module whose command cmd/tool imports the hidden
// package .github/lib, which `./...` skips but the Go plugin indexes
// (AZU-V0-005). observer's test lists cmd/tool's dependencies with an
// all-literal `go list -deps`, so an import added to .github/lib changes its
// result; .github/lib has a test of its own; a hidden testdata fixture and a
// declared non-source dump are no package files.
func hiddenImportFiles() map[string]string {
	return map[string]string{
		"go.mod": "module example.test/h\n\ngo 1.27\n",
		".corvint/non-source-paths.json": `{"profile": "corvint-non-source-paths/0", "entries": [` +
			`{"path": ".agent-evidence/", "reason": "retained evidence output, never built"}]}` + "\n",
		".github/lib/lib.go":              "package lib\n\nfunc Help() string { return \"help\" }\n",
		".github/lib/lib_test.go":         "package lib\n\nimport \"testing\"\n\nfunc TestHelp(t *testing.T) {\n\tif Help() != \"help\" {\n\t\tt.Fatal(Help())\n\t}\n}\n",
		".github/lib/testdata/fixture.go": "package fixture\n",
		".agent-evidence/run/dump.go":     "package dump\n",
		"cmd/tool/main.go":                "package main\n\nimport \"example.test/h/.github/lib\"\n\nfunc main() { println(lib.Help()) }\n",
		"observer/observer.go":            "package observer\n",
		"observer/observer_test.go": "package observer\n\nimport (\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestToolDeps(t *testing.T) {\n\tout, err := exec.Command(\"go\", \"list\", \"-deps\", \"-f\", \"{{.ImportPath}} {{join .Imports \\\" \\\"}}\", \"example.test/h/cmd/tool\").Output()\n" +
			"\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tfor _, line := range strings.Split(string(out), \"\\n\") {\n\t\tif strings.HasPrefix(line, \"strings \") {\n\t\t\tt.Fatalf(\"cmd/tool depends on strings: %s\", line)\n\t\t}\n\t}\n}\n",
		"unrelated/unrelated.go":      "package unrelated\n",
		"unrelated/unrelated_test.go": "package unrelated\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n",
	}
}

// AFP-V0-2202 with AZU-V0-005 (selbatch fix round 1): an import added to the
// indexed hidden package .github/lib changes the dependency listing of
// cmd/tool, its ordinary importer, that observer's test checks; both the
// measured comparison and the assumed one select observer, with the witness
// chain .github/lib -> cmd/tool -> observer. A body-only edit keeps the
// narrowing; a hidden testdata fixture and declared non-source output change
// no import set; a removed hidden package changes the module's.
func TestHiddenPackageImportSetSelectsObserver_AFPV02202(t *testing.T) {
	files := hiddenImportFiles()
	root := t.TempDir()
	writeFiles(t, root, files)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	const module = "go:example.test/h/"
	lib, tool, observer, unrelated := module+".github/lib", module+"cmd/tool", module+"observer", module+"unrelated"
	if _, known := graph.Unit(lib); !known {
		t.Fatalf("hidden package %s is not a unit", lib)
	}
	if unit, _ := graph.Unit(observer); unit.ImportGraphReadsAll || !reflect.DeepEqual(unit.ImportGraphReads, []string{tool}) {
		t.Fatalf("observer reads all=%v %v, want only %s", unit.ImportGraphReadsAll, unit.ImportGraphReads, tool)
	}
	base := fstest.MapFS{".github/gone/gone.go": {Data: []byte("package gone\n")}}
	for name, body := range files {
		base[name] = &fstest.MapFile{Data: []byte(body)}
	}
	head := func(changed map[string]string) fstest.MapFS {
		tree := fstest.MapFS{}
		for name, body := range files {
			tree[name] = &fstest.MapFile{Data: []byte(body)}
		}
		for name, body := range changed {
			tree[name] = &fstest.MapFile{Data: []byte(body)}
		}
		return tree
	}
	const libFile = ".github/lib/lib.go"
	importAdded := "package lib\n\nimport \"strings\"\n\nfunc Help() string { return strings.ToLower(\"help\") }\n"
	bodyOnly := "package lib\n\nfunc Help() string { return \"other\" }\n"
	cases := []struct {
		name  string
		head  fstest.MapFS
		dirty string
		want  affected.ImportSetChanges
	}{
		{"hidden import", head(map[string]string{libFile: importAdded}), libFile, affected.ImportSetChanges{Changed: map[string]string{lib: libFile}}},
		{"hidden file added", head(map[string]string{".github/lib/extra.go": "package lib\n\nimport \"os\"\n\nvar _ = os.Args\n"}), ".github/lib/extra.go", affected.ImportSetChanges{Changed: map[string]string{lib: ".github/lib/extra.go"}}},
		{"hidden body", head(map[string]string{libFile: bodyOnly}), libFile, affected.ImportSetChanges{Changed: map[string]string{}}},
		{"hidden package removed", head(nil), ".github/gone/gone.go", affected.ImportSetChanges{Changed: map[string]string{}, Module: ".github/gone/gone.go"}},
		{"hidden testdata", head(map[string]string{".github/lib/testdata/fixture.go": "package fixture\n\nimport \"os\"\n"}), ".github/lib/testdata/fixture.go", affected.ImportSetChanges{Changed: map[string]string{}}},
		{"declared output", head(map[string]string{".agent-evidence/run/dump.go": "package dump\n\nimport \"os\"\n"}), ".agent-evidence/run/dump.go", affected.ImportSetChanges{Changed: map[string]string{}}},
	}
	for _, test := range cases {
		t.Run("measured "+test.name, func(t *testing.T) {
			if got := golang.ImportSetRecords(base, test.head, graph, []string{test.dirty}); !reflect.DeepEqual(*got, test.want) {
				t.Fatalf("changes = %+v, want %+v", *got, test.want)
			}
		})
	}
	selected := func(changes *affected.ImportSetChanges, dirty string) map[string]affected.Witness {
		plan := affected.Select(graph.WithImportSetChanges(changes), []string{dirty})
		witnesses := map[string]affected.Witness{}
		for _, selection := range plan.Selected {
			witnesses[selection.UnitID] = selection.Witness
		}
		return witnesses
	}
	// The fault: an import added to the hidden dependency. Measured and
	// assumed comparisons alike must select the observer.
	measured := golang.ImportSetRecords(base, head(map[string]string{libFile: importAdded}), graph, []string{libFile})
	for name, changes := range map[string]*affected.ImportSetChanges{"measured": measured, "assumed": nil} {
		witnesses := selected(changes, libFile)
		if witnesses[observer].Kind != affected.WitnessImportGraphReader {
			t.Errorf("%s: observer witness = %+v, want %s", name, witnesses[observer], affected.WitnessImportGraphReader)
		}
		if _, hit := witnesses[lib]; !hit {
			t.Errorf("%s: hidden package %s not selected", name, lib)
		}
		if witness, hit := witnesses[unrelated]; hit {
			t.Errorf("%s: unrelated selected by %+v", name, witness)
		}
		if name == "measured" {
			if want := []string{lib, tool, observer}; !reflect.DeepEqual(witnesses[observer].Via, want) {
				t.Errorf("observer via = %v, want %v", witnesses[observer].Via, want)
			}
		}
	}
	// A body-only edit changes no import set, and neither declared output
	// nor a testdata fixture is a package file, measured or assumed.
	for _, control := range []struct {
		dirty   string
		changes *affected.ImportSetChanges
	}{
		{libFile, golang.ImportSetRecords(base, head(map[string]string{libFile: bodyOnly}), graph, []string{libFile})},
		{".agent-evidence/run/dump.go", nil},
		{".github/lib/testdata/fixture.go", nil},
	} {
		if witness, hit := selected(control.changes, control.dirty)[observer]; hit {
			t.Errorf("%s (measured=%v) selected observer by %+v", control.dirty, control.changes != nil, witness)
		}
	}
	// The toolchain agrees that the observer's listing changes.
	t.Run("go list observes the change", func(t *testing.T) {
		if _, err := exec.LookPath("go"); err != nil {
			t.Skip("go toolchain is unavailable")
		}
		module := t.TempDir()
		writeFiles(t, module, map[string]string{"go.mod": files["go.mod"], libFile: files[libFile], "cmd/tool/main.go": files["cmd/tool/main.go"]})
		list := func() []byte {
			command := exec.Command("go", "list", "-deps", "./cmd/tool")
			command.Dir = module
			command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-buildvcs=false", "GO111MODULE=on")
			out, err := command.Output()
			if err != nil {
				t.Skipf("go list failed: %v", err)
			}
			return out
		}
		before := list()
		if err := os.WriteFile(filepath.Join(module, filepath.FromSlash(libFile)), []byte(importAdded), 0o600); err != nil {
			t.Fatal(err)
		}
		if after := list(); bytes.Equal(before, after) {
			t.Fatalf("go list -deps ./cmd/tool unchanged by the hidden import:\n%s", after)
		}
	})
}
