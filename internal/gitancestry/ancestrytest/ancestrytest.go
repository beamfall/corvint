// Package ancestrytest writes the grafted and shallow checkout fixtures that the tests of every
// guarded ancestry read share (EEP-V0-770): each rewrites the parents of a commit exactly as a real
// info/grafts entry or a depth-limited clone does, so Git itself answers ancestry differently.
package ancestrytest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Graft makes commit (a revision in the checkout at root) parentless through an info/grafts entry,
// as a graft that cuts history does, and returns a function that removes the entry again.
func Graft(t testing.TB, root, commit string) func() {
	t.Helper()
	path := filepath.Join(commonDir(t, root), "info", "grafts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	return write(t, path, resolve(t, root, commit))
}

// Shallow marks commit (a revision in the checkout at root) as a shallow boundary, the state a
// `git clone --depth` checkout records, and returns a function that removes the marker again.
func Shallow(t testing.TB, root, commit string) func() {
	t.Helper()
	return write(t, filepath.Join(commonDir(t, root), "shallow"), resolve(t, root, commit))
}

func write(t testing.TB, path, commit string) func() {
	t.Helper()
	if err := os.WriteFile(path, []byte(commit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func commonDir(t testing.TB, root string) string {
	t.Helper()
	return gitText(t, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

func resolve(t testing.TB, root, commit string) string {
	t.Helper()
	return gitText(t, root, "rev-parse", "--verify", commit+"^{commit}")
}

func gitText(t testing.TB, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}
