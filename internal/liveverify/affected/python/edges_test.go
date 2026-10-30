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

// TestAFPV0680CompoundStatementBodyIsAnImport pins the V1-3044 amendment of
// AFP-V0-680: an import that is the body of a one-line compound statement is
// read, so a change to the module it names selects the test, and it raises the
// conditional-import flag like an indented import.
func TestAFPV0680CompoundStatementBodyIsAnImport(t *testing.T) {
	importers := map[string]string{
		"if":                     "if x: import b\n",
		"try":                    "try: import b\nexcept ImportError: pass\n",
		"except":                 "try: pass\nexcept ImportError: from b import B\n",
		"else no space":          "if x: pass\nelse:import b\n",
		"with as":                "with ctx() as c: import b\n",
		"def with annotations":   "def f(x: int = {1: 2}) -> dict[str, int]: import b\n",
		"colon in a string":      "if y == \"a:b\": import b\n",
		"walrus":                 "if (n := 1): import b\n",
		"body after a semicolon": "try: import a; import b\nexcept ImportError: pass\n",
		"indented":               "def f():\n    if x: import b\n",
		"continued body":         "if x: from pkg import (\n    early,\n    b,\n)\n",
		"async with":             "async def f():\n    pass\nasync with ctx(): import b\n",
		"semicolon in a string":  "if \";\" in value: import b\n",
		"semicolon in a triple":  "if v == \"\"\"a;\"b\"\"\": import b\n",
		"escaped quote":          "if v == 'it\\'s;': import b\n",
		"open triple string":     "x = \"\"\"; import b\ndone\"\"\"\n",
	}
	for name, importer := range importers {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{
				"a.py": "A = 1\n", "b.py": "B = 2\n", "pkg/__init__.py": "\n", "pkg/early.py": "\n", "pkg/b.py": "\n",
				"tests/test_x.py": importer,
			}
			dirty := "b.py"
			if name == "continued body" {
				dirty = "pkg/b.py"
			}
			got, plan := selectedFor(t, files, dirty)
			if name == "open triple string" {
				// The import may be string text: no edge, but the frontier.
				_, flags := scanImports("tests/test_x.py", importer)
				if !flags[FrontierConditionalImport] {
					t.Fatalf("flags=%v, want %s", flags, FrontierConditionalImport)
				}
				return
			}
			if !slices.Contains(got, "tests/test_x.py") {
				t.Fatalf("selected=%v unknown=%+v, want tests/test_x.py", got, plan.Unknown)
			}
			_, flags := scanImports("tests/test_x.py", importer)
			if !flags[FrontierConditionalImport] {
				t.Fatalf("flags=%v, want %s", flags, FrontierConditionalImport)
			}
		})
	}
	for _, text := range []string{
		"if x: pass\n",
		"print(\"if x: import b\")\n",
		"value = {\"if\": 1}\n",
		"iffy: int = 1\n",
		"x = \"a;import b\"\n",
		"x = 1  # ; import b\n",
	} {
		refs, flags := scanImports("tests/test_x.py", text)
		if len(refs) != 0 || len(flags) != 0 {
			t.Fatalf("%q: refs=%v flags=%v, want none", text, refs, flags)
		}
	}
}
