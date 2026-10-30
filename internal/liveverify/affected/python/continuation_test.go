package python

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// selectedFor builds a graph over files and returns the selected tests and
// the plan for a change to dirty.
func selectedFor(t *testing.T, files map[string]string, dirty string) ([]string, affected.Plan) {
	t.Helper()
	root := t.TempDir()
	for relative, body := range files {
		write(t, root, relative, body)
	}
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	plan := affected.Select(graph, []string{dirty})
	return plan.SelectedTests(), plan
}

// TestAFPV0187MultiLineFromImportReadsEveryName pins that every name of a
// from-import continued across lines, by parentheses or a backslash, is read
// as a possible submodule (AFP-V0-187): a change to the module named on a
// continuation line selects the test that imports it.
func TestAFPV0187MultiLineFromImportReadsEveryName(t *testing.T) {
	absolute := map[string]string{
		"parenthesised":       "from pkg import (\n    early,\n    late,\n)\n",
		"comments and as":     "from pkg import (  # first (\n    early,  # ) not a close\n    late as renamed\n)\n",
		"open on a name line": "from pkg import (early,\n                 late)\n",
		"backslash":           "from pkg import early, \\\n    late\n",
	}
	for name, importer := range absolute {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{
				"pkg/__init__.py": "\n", "pkg/early.py": "A = 1\n", "pkg/late.py": "B = 2\n",
				"tests/test_x.py": importer,
			}
			got, plan := selectedFor(t, files, "pkg/late.py")
			if !slices.Equal(got, []string{"tests/test_x.py"}) {
				t.Fatalf("selected=%v unknown=%+v, want tests/test_x.py", got, plan.Unknown)
			}
		})
	}
	t.Run("relative", func(t *testing.T) {
		files := map[string]string{
			"app/__init__.py": "\n", "app/pkg/__init__.py": "\n", "app/pkg/early.py": "A = 1\n",
			"app/pkg/late.py": "B = 2\n", "app/user.py": "from .pkg import (\n    early,\n    late,\n)\n",
			"tests/test_x.py": "import app.user\n",
		}
		got, plan := selectedFor(t, files, "app/pkg/late.py")
		if !slices.Equal(got, []string{"tests/test_x.py"}) {
			t.Fatalf("selected=%v unknown=%+v, want tests/test_x.py", got, plan.Unknown)
		}
	})
}

// TestAFPV0187ContinuationLinesAreNotStatements pins that joined continuation
// lines are consumed: the statement after the close is still read, an
// indented continuation raises no conditional-import frontier, and a statement
// still open at the end of the file keeps the names it shows.
func TestAFPV0187ContinuationLinesAreNotStatements(t *testing.T) {
	refs, flags := scanImports("m.py", "from a import (\n    b,\n    c,\n)\nimport d\n")
	modules := make([]string, 0, len(refs))
	for _, ref := range refs {
		modules = append(modules, ref.module)
	}
	if want := []string{"a", "a.b", "a.c", "d"}; !slices.Equal(modules, want) {
		t.Fatalf("modules=%v want=%v", modules, want)
	}
	if flags[FrontierConditionalImport] {
		t.Fatalf("continuation lines raised %v", flags)
	}
	refs, _ = scanImports("m.py", "from a import (\n    b,\n")
	if len(refs) != 2 || refs[1].module != "a.b" {
		t.Fatalf("an unterminated statement at end of file: refs=%+v", refs)
	}
}

// TestAFPV0189ConftestSelectsEveryTestBelowIt pins that a conftest.py change
// selects every test in its directory and below it, and no test beside or
// above it (AFP-V0-189).
func TestAFPV0189ConftestSelectsEveryTestBelowIt(t *testing.T) {
	files := map[string]string{
		"conftest.py":                    "import pytest\n",
		"tests/conftest.py":              "import pytest\n",
		"tests/test_top.py":              "def test_top(): pass\n",
		"tests/unit/test_deep.py":        "def test_deep(): pass\n",
		"other/tests/conftest.py":        "import pytest\n",
		"other/tests/test_beside.py":     "def test_beside(): pass\n",
		"pkg/module.py":                  "VALUE = 1\n",
		"pkg/tests/test_module.py":       "from pkg import module\n",
		"pkg/tests/fixtures/conftest.py": "import pytest\n",
	}
	got, plan := selectedFor(t, files, "tests/conftest.py")
	if want := []string{"tests/conftest.py", "tests/test_top.py", "tests/unit/test_deep.py"}; !slices.Equal(got, want) {
		t.Fatalf("tests/conftest.py selected=%v unknown=%+v, want %v", got, plan.Unknown, want)
	}
	got, _ = selectedFor(t, files, "conftest.py")
	for _, test := range []string{"tests/test_top.py", "tests/unit/test_deep.py", "other/tests/test_beside.py", "pkg/tests/test_module.py"} {
		if !slices.Contains(got, test) {
			t.Fatalf("root conftest.py selected=%v, missing %s", got, test)
		}
	}
	got, _ = selectedFor(t, files, "pkg/tests/fixtures/conftest.py")
	if slices.Contains(got, "pkg/tests/test_module.py") {
		t.Fatalf("a conftest below the test selected it: %v", got)
	}
}

// TestAFPV0188PytestPluginsAreImports pins that each module a literal
// pytest_plugins assignment names is an import of the assigning module, so a
// plugin change reaches the tests through the conftest, and that a value that
// is not only string literals raises the pytest-plugins frontier (AFP-V0-188).
func TestAFPV0188PytestPluginsAreImports(t *testing.T) {
	for name, conftest := range map[string]string{
		"list":      "pytest_plugins = [\"support.fixtures\", 'support.hooks']\n",
		"multiline": "pytest_plugins = (\n    \"support.hooks\",  # hooks\n)\n",
		"string":    "pytest_plugins = \"support.hooks\"\n",
		"augmented": "pytest_plugins = []\npytest_plugins += [\"support.hooks\"]\n",
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{
				"support/__init__.py": "\n", "support/fixtures.py": "\n", "support/hooks.py": "\n",
				"tests/conftest.py": conftest, "tests/test_x.py": "def test_x(): pass\n",
			}
			got, plan := selectedFor(t, files, "support/hooks.py")
			if !slices.Contains(got, "tests/test_x.py") {
				t.Fatalf("selected=%v unknown=%+v, want tests/test_x.py", got, plan.Unknown)
			}
			if plan.Scope != affected.ScopeBounded {
				t.Fatalf("scope=%s unknown=%+v", plan.Scope, plan.Unknown)
			}
		})
	}
	_, flags := scanImports("tests/conftest.py", "PLUGINS = [\"a\"]\npytest_plugins = PLUGINS\n")
	if !flags[FrontierPytestPlugins] {
		t.Fatalf("a non-literal pytest_plugins raised %v", flags)
	}
	_, flags = scanImports("tests/conftest.py", "if pytest_plugins == []:\n    pass\npytest_plugins == 1\n")
	if flags[FrontierPytestPlugins] {
		t.Fatalf("a comparison raised %v", flags)
	}
}
