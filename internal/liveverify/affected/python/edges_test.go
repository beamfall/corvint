package python

import (
	"slices"
	"testing"
)

// TestAFPV0680EverySemicolonStatementIsAnImport pins AFP-V0-680 (V1-3001):
// each import statement of a line holding several, separated by semicolons,
// is read, wherever it stands on the line, so a change to the module the
// later statement names selects the test.
func TestAFPV0680EverySemicolonStatementIsAnImport(t *testing.T) {
	importers := map[string]string{
		"two imports":                "import a; import b\n",
		"an import and a from":       "import a; from b import B\n",
		"a list then an import":      "import a, os; import b\n",
		"after an assignment":        "x = 1; import b\n",
		"after a from":               "from a import A; import b\n",
		"a continued last import":    "import a; from pkg import (\n    early,\n    b,\n)\n",
		"after a string with a hash": "x = \"#\"; import b\n",
	}
	for name, importer := range importers {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{
				"a.py": "A = 1\n", "b.py": "B = 2\n", "pkg/__init__.py": "\n", "pkg/early.py": "\n", "pkg/b.py": "\n",
				"tests/test_x.py": importer,
			}
			dirty := "b.py"
			if name == "a continued last import" {
				dirty = "pkg/b.py"
			}
			got, plan := selectedFor(t, files, dirty)
			if !slices.Equal(got, []string{"tests/test_x.py"}) {
				t.Fatalf("selected=%v unknown=%+v, want tests/test_x.py", got, plan.Unknown)
			}
		})
	}
	refs, flags := scanImports("tests/test_x.py", "import a; import b\nimport c\n")
	var modules []string
	for _, ref := range refs {
		modules = append(modules, ref.module)
	}
	if !slices.Equal(modules, []string{"a", "b", "c"}) || len(flags) != 0 {
		t.Fatalf("modules=%v flags=%v, want a b c and no flag", modules, flags)
	}
}

// TestAFPV0681SubmoduleImportReachesEveryAncestorPackage pins AFP-V0-681
// (V1-3002): importing pkg.sub runs pkg/__init__.py first, so the importer
// has an edge to every existing ancestor package's __init__ unit and a change
// to it selects a test that imports only the submodule.
func TestAFPV0681SubmoduleImportReachesEveryAncestorPackage(t *testing.T) {
	importers := map[string]string{
		"import":      "import pkg.inner.leaf\n",
		"from module": "from pkg.inner.leaf import f\n",
		"from name":   "from pkg.inner import leaf\n",
	}
	for name, importer := range importers {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{
				"pkg/__init__.py": "X = 1\n", "pkg/inner/__init__.py": "\n", "pkg/inner/leaf.py": "def f(): pass\n",
				"tests/test_x.py": importer,
			}
			for _, dirty := range []string{"pkg/__init__.py", "pkg/inner/__init__.py", "pkg/inner/leaf.py"} {
				got, plan := selectedFor(t, files, dirty)
				if !slices.Equal(got, []string{"tests/test_x.py"}) {
					t.Fatalf("%s: selected=%v unknown=%+v, want tests/test_x.py", dirty, got, plan.Unknown)
				}
			}
		})
	}
}

// TestAFPV0682SourceRootAliasReachesEveryOwner pins AFP-V0-682 (V1-3003):
// when lib/core.py and src/core.py are both importable as core, which one an
// interpreter loads depends on its path, so `import core` has an edge to both
// owners, the unit's imports show the ambiguity, and a change to either
// selects the consumer.
func TestAFPV0682SourceRootAliasReachesEveryOwner(t *testing.T) {
	files := map[string]string{
		"lib/core.py": "A = 1\n", "src/core.py": "B = 2\n", "tests/test_x.py": "import core\n",
	}
	root := t.TempDir()
	for relative, body := range files {
		write(t, root, relative, body)
	}
	imports, ok := unitImports(pythonUnits(t, root), "python:tests.test_x")
	if !ok || !slices.Equal(imports, []string{"python:lib.core", "python:src.core"}) {
		t.Fatalf("imports=%v, want both owners of core", imports)
	}
	for _, dirty := range []string{"lib/core.py", "src/core.py"} {
		got, plan := selectedFor(t, files, dirty)
		if !slices.Equal(got, []string{"tests/test_x.py"}) {
			t.Fatalf("%s: selected=%v unknown=%+v, want tests/test_x.py", dirty, got, plan.Unknown)
		}
	}
}
