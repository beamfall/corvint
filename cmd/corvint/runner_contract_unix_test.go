// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runnerToolStubs puts an executable for each runner tool and lister name
// first on PATH, plus the project's own .venv interpreter, each appending
// its name and argv to the returned log, which lies outside the repository.
func runnerToolStubs(t *testing.T, root string) string {
	t.Helper()
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "launched.log")
	script := "#!/bin/sh\nprintf '%s %s\\n' \"$0\" \"$*\" >> '" + log + "'\nexit 0\n"
	for _, name := range []string{"python", "python3", "pytest", "node", "npx", "bundle", "ruby", "rspec", "rake"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	venv := filepath.Join(root, ".venv", "bin", "python")
	if err := os.MkdirAll(filepath.Dir(venv), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venv, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func launched(t *testing.T, log string) string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

// RNC-V1-002: `corvint affected` plans a change in a pytest project from
// the committed inventory and runs no lister and no project tool; only
// `corvint inventory refresh` runs the executing pytest lister, which the
// same stubs record (the positive control).
func TestRNCV1002AffectedRunsNoListerInAPytestProject(t *testing.T) {
	root := inventoryRepository(t, map[string]string{
		".gitignore":         ".venv/\n",
		"pytest.ini":         "[pytest]\n",
		"app/calc.py":        "def add(a, b):\n    return a + b\n",
		"tests/test_calc.py": "from app.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
	})
	log := runnerToolStubs(t, root)
	base := gitOutput(t, root, "rev-parse", "HEAD")
	writeInventoryFiles(t, root, map[string]string{"app/calc.py": "def add(a, b):\n    return b + a\n"})
	commitAll(t, root, "change")
	for _, args := range [][]string{
		{"--root", root, "affected", "--base", base},
		{"--root", root, "affected", "--base", base, "--summary"},
		{"--root", root, "affected", "--worktree", "--path", "app/calc.py"},
	} {
		if code, stdout, stderr := runCLI(t, args...); code != 0 && code != 3 {
			t.Fatalf("%v: exit %d %s %s", args, code, stdout, stderr)
		}
		if got := launched(t, log); got != "" {
			t.Fatalf("%v started a runner or lister process:\n%s", args, got)
		}
	}
	if code, _, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh"); code != 0 {
		t.Fatalf("refresh: exit %d %s", code, stderr)
	}
	if got := launched(t, log); !strings.Contains(got, ".venv/bin/python -m pytest --collect-only") {
		t.Fatalf("refresh did not run the pytest lister through the stub: %q", got)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "test-inventory.json"))
	if err != nil || !strings.Contains(string(data), `"kind": "executing"`) || !strings.Contains(string(data), `"name": "pytest-collect"`) {
		t.Fatalf("inventory lister record: %v %s", err, data)
	}
}
