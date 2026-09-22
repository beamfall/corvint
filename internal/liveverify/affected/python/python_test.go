package python

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestParseImportLineReadsEveryStatementForm(t *testing.T) {
	cases := map[string][]importRef{
		"import os":                       {{module: "os"}},
		"import os, sys":                  {{module: "os"}, {module: "sys"}},
		"import numpy as np":              {{module: "numpy"}},
		"import a.b.c":                    {{module: "a.b.c"}},
		"import os  # trailing":           {{module: "os"}},
		"from core import VALUE":          {{module: "core"}},
		"from a.b import (":               {{module: "a.b"}},
		"from . import sibling":           {{module: "", dots: 1}},
		"from .neighbor import thing":     {{module: "neighbor", dots: 1}},
		"from ..parent.deep import thing": {{module: "parent.deep", dots: 2}},
		"from x import":                   {{module: "x"}},
	}
	for line, want := range cases {
		t.Run(line, func(t *testing.T) {
			got := parseImportLine(line)
			if len(got) != len(want) {
				t.Fatalf("got=%+v want=%+v", got, want)
			}
			for index := range want {
				if got[index].module != want[index].module || got[index].dots != want[index].dots {
					t.Fatalf("got=%+v want=%+v", got, want)
				}
			}
		})
	}
}

func TestScanImportsFlagsWhatSourceTextCannotSettle(t *testing.T) {
	_, flags := scanImports("m.py", "import importlib\n")
	if !flags[FrontierDynamicImport] {
		t.Fatal("importlib did not raise the dynamic-import frontier")
	}
	_, flags = scanImports("m.py", "if ready:\n    import heavy\n")
	if !flags[FrontierConditionalImport] {
		t.Fatal("an indented import did not raise the conditional-import frontier")
	}
	refs, flags := scanImports("m.py", "import os\n")
	if flags[FrontierConditionalImport] || flags[FrontierDynamicImport] {
		t.Fatalf("a plain import raised a frontier: %v", flags)
	}
	if len(refs) != 1 || refs[0].owner != "m.py" {
		t.Fatalf("refs=%+v", refs)
	}
}

func TestModuleAliasesExposeSourceRootLayouts(t *testing.T) {
	got := moduleAliases("src/pkg/mod.py")
	want := map[string]bool{"src.pkg.mod": true, "pkg.mod": true}
	if len(got) != len(want) {
		t.Fatalf("got=%v", got)
	}
	for _, alias := range got {
		if !want[alias] {
			t.Fatalf("unexpected alias %s in %v", alias, got)
		}
	}
	if aliases := moduleAliases("pkg/__init__.py"); len(aliases) != 1 || aliases[0] != "pkg" {
		t.Fatalf("package __init__ alias=%v", aliases)
	}
}

func TestIsTestFileMatchesDiscoveryConventions(t *testing.T) {
	positive := []string{"tests/test_a.py", "test_a.py", "a_test.py", "pkg/test/helper.py"}
	negative := []string{"src/a.py", "pkg/latest/a.py", "contest.py"}
	for _, path := range positive {
		if !isTestFile(path) {
			t.Fatalf("%s should be a test file", path)
		}
	}
	for _, path := range negative {
		if isTestFile(path) {
			t.Fatalf("%s should not be a test file", path)
		}
	}
}

func TestResolveRefPrefersTheLongestObservedModule(t *testing.T) {
	moduleNames := map[string]string{
		"pkg":            "python:pkg.__init__",
		"pkg.sub":        "python:pkg.sub",
		"src.pkg.nested": "python:src.pkg.nested",
	}
	cases := map[string]string{
		"pkg.sub":            "python:pkg.sub",
		"pkg.sub.attribute":  "python:pkg.sub",
		"pkg.other":          "python:pkg.__init__",
		"unrelated.external": "",
	}
	for name, want := range cases {
		got, ok := resolveRef(importRef{module: name}, moduleNames)
		if want == "" {
			if ok {
				t.Fatalf("%s resolved to %s, want no match", name, got)
			}
			continue
		}
		if !ok || got != want {
			t.Fatalf("%s resolved to %q want %q", name, got, want)
		}
	}
}

func TestResolveRefAnchorsRelativeImportsAtTheImportingPackage(t *testing.T) {
	moduleNames := map[string]string{
		"pkg.sibling": "python:pkg.sibling",
		"top":         "python:top",
	}
	got, ok := resolveRef(importRef{module: "sibling", dots: 1, owner: "pkg/mod.py"}, moduleNames)
	if !ok || got != "python:pkg.sibling" {
		t.Fatalf("single-dot resolved to %q ok=%v", got, ok)
	}
	if _, ok := resolveRef(importRef{module: "missing", dots: 1, owner: "pkg/mod.py"}, moduleNames); ok {
		t.Fatal("an unresolvable relative import must not resolve")
	}
	if _, ok := resolveRef(importRef{module: "x", dots: 3, owner: "pkg/mod.py"}, moduleNames); ok {
		t.Fatal("a relative import above the repository root must not resolve")
	}
}

// A rename inside a package must not let two files claim one module identity;
// the graph rejects duplicate ownership, so the plugin must not produce it.
func TestUnitsGivesEachFileOneIdentity(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/a.py", "VALUE = 1\n")
	write(t, root, "src/b.py", "from a import VALUE\n")
	write(t, root, "tests/test_b.py", "from b import VALUE\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(result.Units) != 3 {
		t.Fatalf("units=%d", len(result.Units))
	}
	plan := affected.Select(graph, []string{"src/a.py"})
	if got := plan.SelectedTests(); len(got) != 1 || got[0] != "tests/test_b.py" {
		t.Fatalf("selected=%v", got)
	}
}

func write(t *testing.T, root, relative, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
