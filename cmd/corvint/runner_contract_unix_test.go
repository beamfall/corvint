// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

// pytestAdopter is a committed pytest repository with no Go module, plus
// stub python and uv executables first on PATH that log their argv to the
// returned file and answer --version.
func pytestAdopter(t *testing.T, extra map[string]string) (root, log string) {
	t.Helper()
	files := map[string]string{
		"pytest.ini":         "[pytest]\n",
		"app/calc.py":        "def add(a, b):\n    return a + b\n",
		"tests/test_calc.py": "def test_add():\n    assert 1 + 2 == 3\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	root = inventoryRepository(t, files)
	bin := t.TempDir()
	log = filepath.Join(t.TempDir(), "runners.log")
	script := "#!/bin/sh\nprintf '%s %s\\n' \"$(basename \"$0\")\" \"$*\" >> '" + log + "'\n[ \"$1\" = --version ] && echo 'stub 1.0'\nexit 0\n"
	for _, name := range []string{"python", "uv", "make"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return root, log
}

// RNC-V1-013: the full command of each runner instance comes from
// .corvint/test-commands.json, else the built-in default, which is printed;
// an adopter's .github/cishards/test-commands.json is never read, and
// Corvint's own repository reads it with a notice while the new file is
// absent.
func TestRNCV1013CLIFullCommandScenarios(t *testing.T) {
	// A base that does not resolve gives no plan, so the run is FULL
	// (STR-V0-003).
	unresolved := strings.Repeat("e", 40)
	run := func(t *testing.T, root string) (int, string) {
		t.Helper()
		receipt := filepath.Join(t.TempDir(), "r.json")
		code, _, stderr := runCLI(t, "--root", root, "test", "--base", unresolved, "--receipt", receipt)
		if !strings.Contains(stderr, "FULL") {
			t.Fatalf("the run is not FULL: %s", stderr)
		}
		return code, stderr
	}
	t.Run("adopter with no declaration", func(t *testing.T) {
		root, log := pytestAdopter(t, nil)
		code, stderr := run(t, root)
		if code != 0 || !strings.Contains(stderr, "corvint-test: DEFAULT pytest@.: .corvint/test-commands.json has no entry for it, so the built-in default full command runs: python -m pytest\n") {
			t.Fatalf("exit %d stderr %s", code, stderr)
		}
		if got := launched(t, log); got != "python --version\npython -m pytest\n" {
			t.Fatalf("runner calls:\n%s", got)
		}
	})
	t.Run("declared command wins", func(t *testing.T) {
		root, log := pytestAdopter(t, map[string]string{".corvint/test-commands.json": `{"profile": "corvint-test-commands/0", "runners": {"pytest": ["uv", "run", "pytest", "-p", "no:cacheprovider"]}}`})
		code, stderr := run(t, root)
		if code != 0 || strings.Contains(stderr, "DEFAULT") {
			t.Fatalf("exit %d stderr %s", code, stderr)
		}
		if got := launched(t, log); got != "uv --version\nuv run pytest -p no:cacheprovider\n" {
			t.Fatalf("runner calls:\n%s", got)
		}
	})
	t.Run("old CI path ignored for adopters", func(t *testing.T) {
		root, log := pytestAdopter(t, map[string]string{".github/cishards/test-commands.json": `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"python": ["make", "py"]}}`})
		code, stderr := run(t, root)
		if code != 0 || !strings.Contains(stderr, "DEFAULT pytest@.") || strings.Contains(stderr, "NOTICE") {
			t.Fatalf("exit %d stderr %s", code, stderr)
		}
		if got := launched(t, log); got != "python --version\npython -m pytest\n" {
			t.Fatalf("runner calls:\n%s", got)
		}
	})
	t.Run("Corvint's own repository reads the old path with a notice", func(t *testing.T) {
		root, log := pytestAdopter(t, map[string]string{
			"go.mod":                              "module github.com/Beamfall/corvint\n\ngo 1.27\n",
			".github/cishards/test-commands.json": `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"python": ["make", "py"]}}`,
		})
		code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", unresolved, "--dry-run", "--full")
		if code != 0 || !strings.Contains(stderr, "corvint-test: NOTICE the full commands are read from the old path .github/cishards/test-commands.json for one release; move them to .corvint/test-commands.json\n") ||
			!strings.Contains(stdout, `"argv":["make","py"]`) || strings.Contains(stdout, `"declared:pytest@`) {
			t.Fatalf("exit %d stdout %s stderr %s", code, stdout, stderr)
		}
		if got := launched(t, log); got != "" {
			t.Fatalf("a dry run launched:\n%s", got)
		}
	})
}

// RNC-V1-004, owner decision R32: a commit that deletes the config of a
// runner instance the committed inventory names makes every runner run all
// tests with reason runner-removed; a changed path no runner owns runs every
// test with reason unowned-path, which in a pytest adopter is the built-in
// pytest default.
func TestRNCV1004RemovedRunnerAndUnownedPathRunEverything(t *testing.T) {
	t.Run("runner removed", func(t *testing.T) {
		root, _, log := sessionFixture(t, "pass")
		writeInventoryFiles(t, root, map[string]string{".rspec": "--require spec_helper\n", "spec/a_spec.rb": "describe 'a' do\n  it('works') { }\nend\n"})
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "rspec")
		if code, _, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh"); code != 0 {
			t.Fatalf("refresh: %s", stderr)
		}
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "inventory")
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		affectedGit(t, root, "rm", "-q", ".rspec")
		affectedGit(t, root, "commit", "-qm", "drop rspec")
		code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--full")
		var plan struct {
			Scope, Reason string
			Inventory     struct{ Undeclared []string }
		}
		// The Ruby spec file left behind has no runner now, so the FULL
		// plan is INCOMPLETE (exit 3) rather than guessing a command.
		if code != 3 || json.Unmarshal([]byte(stdout), &plan) != nil || plan.Scope != "FULL" || !slices.Equal(plan.Inventory.Undeclared, []string{"rspec@."}) ||
			!strings.HasPrefix(plan.Reason, "runner-removed: rspec@. is named by the inventory") || !strings.Contains(stderr, "FALLBACK runner-removed: rspec@.") {
			t.Fatalf("exit %d plan %+v stdout %s stderr %s", code, plan, stdout, stderr)
		}
		if got := launched(t, log); strings.Contains(got, "go test") {
			t.Fatalf("a dry run launched go test:\n%s", got)
		}
	})
	t.Run("unowned path", func(t *testing.T) {
		root, log := pytestAdopter(t, nil)
		base := gitOutput(t, root, "rev-parse", "HEAD")
		writeInventoryFiles(t, root, map[string]string{"notes.dat": "x\n"})
		commitAll(t, root, "notes")
		receipt := filepath.Join(t.TempDir(), "r.json")
		code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receipt)
		if code != 0 || !strings.Contains(stderr, "FALLBACK unowned-path: a changed path no runner instance owns runs every test (UNOWNED_DIRTY_PATH): notes.dat") {
			t.Fatalf("exit %d stderr %s", code, stderr)
		}
		if got := launched(t, log); got != "python --version\npython -m pytest\n" {
			t.Fatalf("runner calls:\n%s", got)
		}
	})
}
