package main

import (
	"os"
	"path/filepath"
	"testing"
)

// provePythonRepository is a one-package Python project: core is the changed
// module, app imports it absolutely, rel imports it relatively, and doc only
// names it inside a comment.
func provePythonRepository(t *testing.T) string {
	t.Helper()
	root := cliRepository(t)
	files := map[string]string{
		"go.mod":          "module example.test/prove\n\ngo 1.27.0\n",
		"pkg/__init__.py": "",
		"pkg/core.py":     "def compute(values):\n    return len(values)\n",
		"pkg/app.py":      "import os\nimport pkg.core\n\nprint(pkg.core.compute([]))\n",
		"pkg/rel.py":      "\"\"\"docstring\"\"\"\nfrom .core import compute\n\nprint(compute([]))\n",
		".gitignore":      ".corvint/\n",
	}
	for relative, content := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "python fixture")
	return root
}

func TestProveImpactJudgesPythonImportRows(t *testing.T) {
	t.Parallel()
	root := provePythonRepository(t)
	receipt, _, stderr, code := runProveCLI(t, root, "pkg/core.py")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	seen := map[string]map[string]any{}
	for _, row := range proofRows(t, receipt) {
		seen[stringAt(row, "result")] = row
	}
	for path, line := range map[string]int{"pkg/app.py": 2, "pkg/rel.py": 2} {
		row, present := seen[path]
		if !present {
			t.Fatalf("no reverse-import row for %s: %v", path, seen)
		}
		if row["falsifier"] != falsifierReference || row["falsified"] != falsifiedPass || integerAt(row, "line") != line {
			t.Fatalf("%s: %v", path, row)
		}
	}
	if receipt["state"] != "READY" || proofField(t, receipt, "failed_results") != 0 {
		t.Fatalf("state %v proof %v", receipt["state"], receipt["proof"])
	}
}

func TestPythonReferenceRowIsNotRun(t *testing.T) {
	t.Parallel()
	row := proveRow{
		Falsifier: falsifierReference,
		Path:      "pkg/app.py",
		BlobHash:  "app",
		Line:      2,
		reason:    "references compute declared by pkg/core.py",
	}
	cited := map[string]citedBlob{
		"pkg/app.py":  {oid: "app", objectType: "blob", content: []byte("from .core import compute\ncompute([])\n")},
		"pkg/core.py": {oid: "core", objectType: "blob", content: []byte("def compute(values):\n    return len(values)\n")},
	}
	if got := falsifyRow(row, cited, nil); got != falsifiedNotRun {
		t.Fatalf("Python reference row = %s, want %s", got, falsifiedNotRun)
	}
}

func TestPythonImportsAtLineVerdicts(t *testing.T) {
	t.Parallel()
	absolute := []byte("import os\nimport pkg.core\n")
	fromForm := []byte("from pkg import core\n")
	relative := []byte("from .core import compute\nfrom ..top import thing\nfrom . import core\nfrom pkg import *\n")
	comment := []byte("# import pkg.core\nx = 'import pkg.core'\n")
	broken := []byte("import pkg.core\ns = 'unterminated\n")
	for _, test := range []struct {
		name     string
		path     string
		content  []byte
		imported string
		line     int
		want     bool
	}{
		{"absolute dotted", "pkg/app.py", absolute, "pkg.core", 2, true},
		{"wrong line", "pkg/app.py", absolute, "pkg.core", 1, false},
		{"from binds base and member", "pkg/app.py", fromForm, "pkg", 1, true},
		{"from member spelling", "pkg/app.py", fromForm, "pkg.core", 1, true},
		{"relative sibling resolves", "pkg/rel.py", relative, "pkg.core", 1, true},
		{"relative parent resolves", "a/pkg/rel.py", relative, "a.top", 2, true},
		{"relative under src", "src/pkg/rel.py", relative, "pkg.core", 1, true},
		{"relative climbs past root", "rel.py", relative, "top", 2, false},
		{"bare relative binds the package", "pkg/rel.py", relative, "pkg", 3, true},
		{"star import binds the module", "pkg/rel.py", relative, "pkg", 4, true},
		{"comment and string are not imports", "pkg/app.py", comment, "pkg.core", 1, false},
		{"untokenizable file", "pkg/app.py", broken, "pkg.core", 1, false},
	} {
		if got := pythonImportsAtLine(test.path, test.content, test.imported, test.line); got != test.want {
			t.Errorf("%s: got %v want %v", test.name, got, test.want)
		}
	}
	if got := importsAtLine("pkg/app.py", absolute, "pkg.core", 2); !got {
		t.Error("importsAtLine must dispatch .py to the Python grammar")
	}
}
