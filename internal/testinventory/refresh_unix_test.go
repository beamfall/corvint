// SPDX-License-Identifier: Apache-2.0

//go:build unix

package testinventory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireRefusal(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("Refresh = %v, want refusal %s", err, code)
	}
}

// TIV-V1-010: a test file edited while the Jest lister runs, or a commit
// landing after a lister, refuses with inventory-refresh-drift and leaves
// the previous inventory unchanged.
func TestTIVV1010RefreshRefusesDrift(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":     "node_modules/\n",
		"jest.config.js": "module.exports = {}\n",
		"a.test.js":      "test('a', () => {})\n",
	})
	stub(t, root, "node_modules/.bin/jest", "--listTests --config jest.config.js", `echo "$(pwd -P)/a.test.js"`)
	ctx := context.Background()
	if _, err := Refresh(ctx, Options{Git: git}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, filepath.FromSlash(Path))
	old, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	stub(t, root, "node_modules/.bin/jest", "--listTests --config jest.config.js", `echo "test('b', () => {})" >> a.test.js; echo "$(pwd -P)/a.test.js"`)
	_, err = Refresh(ctx, Options{Git: git})
	requireRefusal(t, err, ReasonDrift)
	if now, _ := os.ReadFile(target); string(now) != string(old) {
		t.Fatal("drift refusal rewrote the inventory")
	}

	writeFile(t, root, "a.test.js", "test('a', () => {})\n")
	stub(t, root, "node_modules/.bin/jest", "--listTests --config jest.config.js", `echo "$(pwd -P)/a.test.js"`)
	afterLister = func(Declared) {
		runGit(t, root, "commit", "-q", "--allow-empty", "-m", "moved")
	}
	defer func() { afterLister = func(Declared) {} }()
	_, err = Refresh(ctx, Options{Git: git})
	requireRefusal(t, err, ReasonDrift)
	if now, _ := os.ReadFile(target); string(now) != string(old) {
		t.Fatal("HEAD drift refusal rewrote the inventory")
	}
}

// TIV-V1-011: a pytest collection error omits that instance with the error
// recorded, the other instances are written, and the next reader runs
// pytest in full with inventory-absent. A refresh of an unchanged tree
// rewrites nothing.
func TestTIVV1011ListerFailureRecorded(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":         ".venv/\n",
		"go.mod":             "module example.com/m\n",
		"a_test.go":          "package m\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
		"pytest.ini":         "[pytest]\n",
		"tests/test_a.py":    "import missing\n",
		".corvint/README.md": "local config\n",
	})
	stub(t, root, ".venv/bin/python", "-m pytest --collect-only -q -p no:cacheprovider --rootdir=. -c pytest.ini",
		`echo "ERROR collecting tests/test_a.py: ModuleNotFoundError" >&2; exit 2`)
	ctx := context.Background()
	result, err := Refresh(ctx, Options{Git: git})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Inventory.Instances) != 1 || result.Inventory.Instances[0].Key != "go@." {
		t.Fatalf("instances %+v", result.Inventory.Instances)
	}
	failures := result.Inventory.Failures
	if len(failures) != 1 || failures[0].Key != "pytest@." || !strings.Contains(failures[0].Error, "ModuleNotFoundError") || failures[0].Lister.Kind != KindExecuting {
		t.Fatalf("failures %+v", failures)
	}
	work, err := NewWorkTree(ctx, git)
	if err != nil {
		t.Fatal(err)
	}
	verdict := Judge(work)
	for _, instance := range verdict.Instances {
		switch instance.Key {
		case "pytest@.":
			if !instance.Full || instance.Reason != ReasonAbsent || !strings.Contains(instance.Cause, "ModuleNotFoundError") {
				t.Errorf("pytest verdict %+v", instance)
			}
		case "go@.":
			if instance.Full || instance.Tests != 1 {
				t.Errorf("go verdict %+v", instance)
			}
		}
	}
	again, err := Refresh(ctx, Options{Git: git})
	if err != nil || again.Changed {
		t.Fatalf("unchanged refresh changed=%v err=%v", again.Changed, err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".corvint"))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
}
