package python

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func pythonUnits(t *testing.T, root string) affected.Result {
	t.Helper()
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func unitImports(result affected.Result, id string) ([]string, bool) {
	for _, unit := range result.Units {
		if unit.ID == id {
			return unit.Imports, true
		}
	}
	return nil, false
}

// TestAFPV0184UnresolvedRelativeImportRaisesItsFrontier covers AFP-V0-184's
// raise: a relative import naming no observed module, or climbing to the
// repository root, raises python:relative-import-unresolved and adds no edge,
// while one that resolves raises nothing.
func TestAFPV0184UnresolvedRelativeImportRaisesItsFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pkg/__init__.py", "")
	write(t, root, "pkg/sibling.py", "VALUE = 1\n")
	write(t, root, "pkg/resolved.py", "from .sibling import VALUE\n")
	clean := pythonUnits(t, root)
	if slices.Contains(clean.Frontier, FrontierRelativeImport) {
		t.Fatalf("a resolved relative import raised %s: %v", FrontierRelativeImport, clean.Frontier)
	}
	if imports, _ := unitImports(clean, "python:pkg.resolved"); !slices.Contains(imports, "python:pkg.sibling") {
		t.Fatalf("resolved relative import edges = %v", imports)
	}
	// A missing module sits in a namespace package: under pkg/__init__.py the
	// longest observed prefix, pkg itself, would resolve it (AFP-V0-182).
	for name, test := range map[string]struct{ file, unit, body string }{
		"missing module": {"ns/broken.py", "python:ns.broken", "from .missing import VALUE\n"},
		"above the root": {"pkg/broken.py", "python:pkg.broken", "from ...outside import VALUE\n"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "pkg/__init__.py", "")
			write(t, root, test.file, test.body)
			result := pythonUnits(t, root)
			if !slices.Contains(result.Frontier, FrontierRelativeImport) {
				t.Fatalf("frontier = %v, want %s", result.Frontier, FrontierRelativeImport)
			}
			imports, ok := unitImports(result, test.unit)
			if !ok || len(imports) != 0 {
				t.Fatalf("unresolved relative import kept edges %v (unit present %v)", imports, ok)
			}
		})
	}
}

// TestAFPV0185UnreadableSourceRaisesItsFrontier covers AFP-V0-185's
// unreadable case: a Python file the source reader refuses (here one over the
// per-file read bound) raises python:unreadable-source instead of vanishing
// silently, and the readable files still form units.
func TestAFPV0185UnreadableSourceRaisesItsFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ok.py", "VALUE = 1\n")
	write(t, root, "huge.py", "")
	if err := os.Truncate(filepath.Join(root, "huge.py"), affected.MaxSourceBytes+1); err != nil {
		t.Fatal(err)
	}
	result := pythonUnits(t, root)
	if !slices.Contains(result.Frontier, FrontierUnreadableSource) {
		t.Fatalf("frontier = %v, want %s", result.Frontier, FrontierUnreadableSource)
	}
	if _, ok := unitImports(result, "python:ok"); !ok {
		t.Fatalf("the readable module lost its unit: %+v", result.Units)
	}
	if _, ok := unitImports(result, "python:huge"); ok {
		t.Fatal("the unreadable file became a unit")
	}
}
