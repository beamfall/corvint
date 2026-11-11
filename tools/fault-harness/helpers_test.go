// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// fixtureRepo builds a tiny synthetic Git repository in a temporary
// directory. Its files never belong to the checkout.
type fixtureRepo struct {
	t    *testing.T
	root string
}

func newFixtureRepo(t *testing.T) *fixtureRepo {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("local process-group harness supports Darwin and Linux")
	}
	r := &fixtureRepo{t: t, root: t.TempDir()}
	r.git("init", "--quiet")
	return r
}

func (r *fixtureRepo) write(name, body string) {
	r.t.Helper()
	name = filepath.Join(r.root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		r.t.Fatal(err)
	}
	if err := execfile.WriteFile(name, []byte(body), 0o700); err != nil {
		r.t.Fatal(err)
	}
}

func (r *fixtureRepo) git(args ...string) string {
	r.t.Helper()
	out, err := git(context.Background(), r.root, args...)
	if err != nil {
		r.t.Fatal(err)
	}
	return out
}

// commit commits everything and returns the new HEAD.
func (r *fixtureRepo) commit(message string) string {
	r.t.Helper()
	r.git("add", "--all")
	r.git("commit", "--quiet", "-m", message)
	return r.git("rev-parse", "HEAD")
}

// writeGoModule writes a two-package module: leaf, and other, whose test
// asserts a value computed from leaf.
func (r *fixtureRepo) writeGoModule() {
	r.write("go.mod", "module oracle.invalid/leaf\n\ngo 1.27.3\n") //corvint:fixture-path
	r.write("leaf.go", "package leaf\n\nfunc Value() int { return 1 }\n")
	r.write("leaf_test.go", "package leaf\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {\n\tif Value() != 1 {\n\t\tt.Fatal(\"value\")\n\t}\n}\n")
	r.write("other/other.go", "package other\n\nimport \"oracle.invalid/leaf\"\n\nfunc Double() int { return 2 * leaf.Value() }\n")
	r.write("other/other_test.go", "package other\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double() != 2 {\n\t\tt.Fatal(\"double\")\n\t}\n}\n")
}

// fakePlanner writes a planner stub that selects the units listed, one per
// line, in the file mode; it does not compute dependencies.
func fakePlanner(t *testing.T) (bin, mode string) {
	t.Helper()
	dir := t.TempDir()
	bin, mode = filepath.Join(dir, "planner"), filepath.Join(dir, "mode")
	body := `#!/bin/sh
set -eu
[ "$1" = affected ] && [ "$2" = --base ]
selected=''
while IFS= read -r unit; do
  [ -n "$unit" ] || continue
  entry=$(printf '{"unitId":"%s","witness":{"kind":"OWN","dirtyPath":"leaf.go","viaLength":0}}' "$unit")
  selected="${selected:+$selected,}$entry"
done < "` + mode + `"
printf '{"ok":true,"plan":{"scope":"BOUNDED","selected":[%s],"unknown":[]},"summary":{"line":"fixture"}}\n' "$selected"
`
	if err := execfile.WriteFile(bin, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	setMode(t, mode)
	return bin, mode
}

func setMode(t *testing.T, mode string, units ...string) {
	t.Helper()
	body := ""
	for _, u := range units {
		body += u + "\n"
	}
	if err := execfile.WriteFile(mode, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
