package python

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// ACX-V0-855 (V1-3045): pkg.py and pkg/__init__.py share the identity
// python:pkg; both files belong to that one unit, so an edit to either has an
// owner and selects the importer, never an unowned unknown.
func TestModuleAndPackageInitShareOneOwner_ACX_V0_855(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"pkg.py":          "VALUE = 1\n",
		"pkg/__init__.py": "import helper\n",
		"helper.py":       "HELP = 1\n",
		"test_user.py":    "import pkg\n",
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range []string{"pkg.py", "pkg/__init__.py", "helper.py"} {
		plan := affected.Select(graph, []string{edit})
		for _, unknown := range plan.Unknown {
			if unknown.Reason == affected.UnknownUnownedDirtyPath {
				t.Fatalf("edit %s has no owner: %v", edit, plan.Unknown)
			}
		}
		ids := []string{}
		for _, selection := range plan.Selected {
			ids = append(ids, selection.UnitID)
		}
		if !slices.Contains(ids, "python:test_user") {
			t.Fatalf("edit %s selected %v, want python:test_user", edit, ids)
		}
	}
}
