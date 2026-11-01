package verify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

const importerModule = "module example.test/m\n\ngo 1.22\n"

// p has no tests of its own; q imports it and its test exercises Double
// through q's own helper, so only an importer test can guard p.
const importerSource = "package p\n\n" + // 1-2
	"func add(v int) int {\n\treturn v + 1\n}\n\n" + // 3-6
	"func Double(v int) int {\n\treturn add(v) * 2\n}\n" // 7-9

const importerQ = "package q\n\nimport \"example.test/m/p\"\n\nfunc Wrap(v int) int { return p.Double(v) }\n"

const importerQTest = "package q\n\nimport \"testing\"\n\n" +
	"func TestWrap(t *testing.T) {\n\tif Wrap(1) != 4 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n\n" +
	"func TestUnrelated(t *testing.T) {}\n"

// TestTCQV0761GuardInAnImportingPackageIsFound (V1-1450): a hunk whose own
// directory has no candidate test names the tests of its direct importer that
// reach an exported name it changes, and the revert-and-run mutation proves
// the guard: with the hunk reverted the importer's test fails, so the row is
// guarded and cited at the importer's base header.
func TestTCQV0761GuardInAnImportingPackageIsFound(t *testing.T) {
	tree := writeGuardTree(t, map[string]string{
		"go.mod": importerModule, "p/p.go": importerSource, "q/q.go": importerQ, "q/q_test.go": importerQTest,
		"r/r.go": "package r\n\nimport \"example.test/m/p\"\n\nvar _ = p.Double\n",
	})
	name := "p/p.go"
	// The change made add return v + 1; reverting it restores v.
	hunk := &patch.Hunk{NewPath: &name, NewRange: wire.Range{Start: 4, Count: 1}, Body: []patch.BodyLine{
		{Prefix: '-', Payload: []byte("\treturn v\n")}, {Prefix: '+', Payload: []byte("\treturn v + 1\n")},
	}}
	base := func(file string) []byte {
		data, _ := os.ReadFile(filepath.Join(tree, filepath.FromSlash(file)))
		return data
	}
	rows := make([]GuardRow, 1)
	judgeGuards(context.Background(), tree, []guardHunk{{hunk: hunk, kind: patch.KindModify}}, rows, DefaultHunkRevertLimits, goTestRunner(tree), base)
	row := rows[0]
	if !reflect.DeepEqual(row.Candidates, []string{"q::TestWrap"}) {
		t.Fatalf("candidates = %v, want q::TestWrap", row.Candidates)
	}
	if row.Result != GuardGuarded || !reflect.DeepEqual(row.Guards, []string{"TestWrap"}) {
		t.Fatalf("row = %+v, want guarded by TestWrap", row)
	}
	if row.Citation == nil || row.Citation.Path != "q/q_test.go" || row.Citation.Test != "TestWrap" {
		t.Fatalf("citation = %+v", row.Citation)
	}
	// A change no importer test observes survives the revert: unguarded.
	survives := &patch.Hunk{NewPath: &name, NewRange: wire.Range{Start: 8, Count: 1}, Body: []patch.BodyLine{
		{Prefix: '-', Payload: []byte("\treturn (add(v)) * 2\n")}, {Prefix: '+', Payload: []byte("\treturn add(v) * 2\n")},
	}}
	rows = make([]GuardRow, 1)
	judgeGuards(context.Background(), tree, []guardHunk{{hunk: survives, kind: patch.KindModify}}, rows, DefaultHunkRevertLimits, goTestRunner(tree), base)
	if rows[0].Result != GuardUnguarded || !reflect.DeepEqual(rows[0].Candidates, []string{"q::TestWrap"}) {
		t.Fatalf("equivalent revert row = %+v, want unguarded", rows[0])
	}
}

// TestTCQV0761ImporterSearchIsBounded: importers are the direct importers of
// the root module, in directory order, at most MaxGuardImporterDirectories,
// never the package itself, a nested module or a testdata directory.
func TestTCQV0761ImporterSearchIsBounded(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"go.mod": importerModule, "p/p.go": importerSource,
		"p/p_ext_test.go":      "package p_test\n\nimport _ \"example.test/m/p\"\n",
		"nested/go.mod":        "module example.test/nested\n",
		"nested/n.go":          "package n\n\nimport _ \"example.test/m/p\"\n",
		"x/testdata/t.go":      "package t\n\nimport _ \"example.test/m/p\"\n",
		"indirect/indirect.go": "package indirect\n\nimport _ \"example.test/m/q\"\n",
		"q/q.go":               importerQ,
	}
	for i := range MaxGuardImporterDirectories + 2 {
		files[fmt.Sprintf("i%02d/i.go", i)] = "package i\n\nimport _ \"example.test/m/p\"\n"
	}
	importers := indexImporters(writeGuardTree(t, files)).of("p")
	if len(importers) != MaxGuardImporterDirectories || importers[0] != "i00" {
		t.Fatalf("importers = %v, want the first %d in directory order", importers, MaxGuardImporterDirectories)
	}
	for _, dir := range importers {
		if dir == "p" || dir == "nested" || dir == "x/testdata" || dir == "indirect" {
			t.Fatalf("importers = %v hold %s", importers, dir)
		}
	}
}
