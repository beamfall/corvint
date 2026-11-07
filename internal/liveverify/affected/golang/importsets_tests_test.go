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
// with no non-test file.
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
			if changed != test.changed || got.Module != "" {
				t.Fatalf("changes = %+v, want lib changed=%v", *got, test.changed)
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

	// The plan: a test-only edit keeping the sets selects no reader through
	// the import graph; one adding an import selects the readers of lib's
	// closure (guard lists cmd/tool, built from lib) and of the module.
	guard, slice, wide := importGraphModule+"guard", importGraphModule+"slice", importGraphModule+"wide"
	writeFiles(t, root, map[string]string{"lib/more_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestMore(t *testing.T) {}\n"})
	edited, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"lib/more_test.go"}
	kept := golang.ImportSetRecords(base, os.DirFS(root), edited, dirty)
	if !reflect.DeepEqual(*kept, affected.ImportSetChanges{Changed: map[string]string{}}) {
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
	if hit := readers(kept); len(hit) != 0 {
		t.Errorf("a test file that keeps the import sets selects import-graph readers %v", hit)
	}
	writeFiles(t, root, map[string]string{"lib/more_test.go": "package lib\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMore(t *testing.T) { _ = os.Args }\n"})
	added := golang.ImportSetRecords(base, os.DirFS(root), edited, dirty)
	if hit := readers(added); !hit[guard] || !hit[slice] || !hit[wide] {
		t.Errorf("a test file that adds an import selects import-graph readers %v, want guard, slice and wide (changes %+v)", hit, *added)
	}
}
