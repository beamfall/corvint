// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-9000: an unresolved source alias prevents a file-kind bound from
// removing the scanner's original unbounded read reasons.
func TestReadKindsUnresolvedAlias_AFPV09000(t *testing.T) {
	for _, target := range []string{"missing.txt", "loop.go"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.mod":         "module example.test/m\n",
				"scan/s_test.go": "//corvint:read-kinds .go go.mod\npackage scan\nvar root = \"..\"\n",
			})
			if err := os.Symlink(target, filepath.Join(root, "loop.go")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			g, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			p := affected.Select(g, []string{"data/imports.txt"})
			if len(p.Selected) != 1 || p.Selected[0].UnitID != "go:example.test/m/scan" || p.Selected[0].Witness.Kind != affected.WitnessUnboundedReader {
				t.Fatalf("unresolved alias lost scanner's unbounded reads: %+v", p.Selected)
			}
		})
	}
}

// AFP-V0-9000: only an explicit file-local declaration bounds a test's root
// scan. The identical path remains selected for a real unbounded reader.
func TestReadKindsDeclaration_AFPV09000(t *testing.T) {
	marker := "//" + "corvint:read-kinds .go go.mod\n"
	for _, tc := range []struct {
		name, file, body string
		bounded          bool
	}{
		{"declared test", "scan_test.go", marker + "package scan\nvar root = \"../..\"\n", true},
		{"unmarked test", "scan_test.go", "package scan\nvar root = \"../..\"\n", false},
		{"non-test", "scan.go", marker + "package scan\nvar root = \"../config.json\"\n", false},
		{"misplaced", "scan_test.go", "package scan\n" + marker + "var root = \"../..\"\n", false},
		{"inline header", "scan_test.go", "/* other */ " + marker + "package scan\nvar root = \"../..\"\n", false},
		{"duplicate", "scan_test.go", marker + marker + "package scan\nvar root = \"../..\"\n", false},
		{"invalid kinds", "scan_test.go", "//corvint:read-kinds .go\npackage scan\nvar root = \"../..\"\n", false},
		{"string", "scan_test.go", "package scan\nvar text = `" + marker + "`\nvar root = \"../..\"\n", false},
		{"unparsed", "scan_test.go", marker + "package scan\nvar root = \"../..\"\nfunc broken( {", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.mod":             "module example.test/m\n",
				"scan/" + tc.file:    tc.body,
				"scan/other_test.go": "package scan\n",
				"reader/r_test.go":   "package reader\nvar root = \"..\"\n",
				"changed/c.go":       "package changed\n",
				"changed/c_test.go":  "package changed\n",
			})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			selected := func(path string) []string {
				var ids []string
				for _, s := range affected.Select(graph, []string{path}).Selected {
					ids = append(ids, s.UnitID)
				}
				return ids
			}
			scan := "go:example.test/m/scan"
			for _, path := range []string{".corvint/data.json", "script/heavy.sh", "docs/specs/example.md"} {
				ids := selected(path)
				if slices.Contains(ids, scan) == tc.bounded {
					t.Errorf("%s selects %v; bounded=%v", path, ids, tc.bounded)
				}
				if !slices.Contains(ids, "go:example.test/m/reader") {
					t.Errorf("true reader of %s omitted: %v", path, ids)
				}
			}
			for _, path := range []string{"changed/c.go", "changed/c_test.go", "go.mod", "nested/go.mod"} {
				if !slices.Contains(selected(path), scan) {
					t.Errorf("source reader of %s omitted: %v", path, selected(path))
				}
			}
		})
	}
	// Another test file's unrestricted reads cannot be bounded by its neighbour.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"go.mod": "module example.test/m\n", "scan/s_test.go": marker + "package scan\nvar root = \"..\"\n", "scan/u_test.go": "package scan\nvar root = \"..\"\n"})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(affected.Select(graph, []string{".corvint/data.json"}).Selected) != 1 {
		t.Fatal("a declaration bounded another file")
	}
}

// AFP-V0-9005: a fixture-local parent path supplies no checkout-wide read,
// while the identical path of an unmarked reader still selects it. A source
// importer cannot inherit a test-only exemption or lose its compile edge.
func TestFixtureReadKinds_AFPV09005(t *testing.T) {
	marker := "//" + "corvint:read-kinds fixtures\n"
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"fixture/f.go":       "package fixture\n",
		"fixture/f_test.go":  marker + "package fixture\nvar cases = []string{\"../../..\",\"../.corvint/data.json\"}\n",
		"real/r_test.go":     "package real\nvar cases = []string{\"../../..\",\"../.corvint/data.json\"}\n",
		"importer/i_test.go": "package importer\nimport _ \"example.test/m/fixture\"\n",
	})
	g, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	ids := func(p string) []string {
		var ids []string
		for _, s := range affected.Select(g, []string{p}).Selected {
			ids = append(ids, s.UnitID)
		}
		return ids
	}
	// The ordinary literal is kept, even in the fixture-declared file.
	if !slices.Contains(ids(".corvint/data.json"), "go:example.test/m/fixture") {
		t.Fatal("ordinary literal removed")
	}
	got := ids("unrelated/data.json")
	if slices.Contains(got, "go:example.test/m/fixture") || !slices.Contains(got, "go:example.test/m/real") {
		t.Fatalf("fixture vs real readers: %v", got)
	}
	if !slices.Contains(ids("fixture/f.go"), "go:example.test/m/importer") {
		t.Fatal("compile dependency removed")
	}
	writeFiles(t, root, map[string]string{"fixture/other_test.go": "package fixture\nvar root = \"..\"\n"})
	g, err = affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ids("unrelated/data.json"), "go:example.test/m/fixture") {
		t.Fatal("another file's real root read removed")
	}
}
