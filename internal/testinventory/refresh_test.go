// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// goOnlyRepo is a repository with one Go instance and its static lister,
// so refresh runs no executable.
func goOnlyRepo(t *testing.T) (string, Git) {
	t.Helper()
	return gitRepo(t, map[string]string{
		"go.mod":    "module example.com/m\n",
		"a_test.go": "package m\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
	})
}

// TIV-V1-010: one final drift check runs just before the write, also when
// the options select no instance and so no lister ran its own check; a
// commit landing at that point refuses and writes nothing.
func TestTIVV1010FinalDriftCheckBeforeWrite(t *testing.T) {
	root, git := goOnlyRepo(t)
	target := filepath.Join(root, filepath.FromSlash(Path))
	for _, options := range []Options{{Git: git, Runner: "jest"}, {Git: git}} {
		beforeWrite = func() { runGit(t, root, "commit", "-q", "--allow-empty", "-m", "moved") }
		_, err := Refresh(context.Background(), options)
		beforeWrite = func() {}
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != ReasonDrift {
			t.Fatalf("runner %q: Refresh = %v, want refusal %s", options.Runner, err, ReasonDrift)
		}
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("runner %q: drift refusal wrote the inventory (%v)", options.Runner, err)
		}
	}
}

// TIV-V1-010: the source digest leaves out writeAtomic's temporary files, so
// a temporary inventory file appearing beside the inventory during refresh
// is not drift.
func TestTIVV1010TemporaryInventoryFileIsNotDrift(t *testing.T) {
	root, git := goOnlyRepo(t)
	afterLister = func(Declared) { writeFile(t, root, ".corvint/.test-inventory-1.tmp", "partial") }
	beforeWrite = func() { writeFile(t, root, ".corvint/.test-inventory-2.tmp", "partial") }
	defer func() { afterLister, beforeWrite = func(Declared) {}, func() {} }()
	result, err := Refresh(context.Background(), Options{Git: git})
	if err != nil || !result.Changed {
		t.Fatalf("Refresh changed=%v err=%v, want the inventory written", result.Changed, err)
	}
	files := map[string]string{"a": "1", ".corvint/.test-inventory-9.tmp": "2"}
	if sourceDigest(files) != sourceDigest(map[string]string{"a": "1"}) {
		t.Fatal("sourceDigest counts a temporary inventory file")
	}
	if sourceDigest(map[string]string{"a": "1", "x/.test-inventory-9.tmp": "2"}) == sourceDigest(map[string]string{"a": "1"}) {
		t.Fatal("sourceDigest leaves out a same-named file outside .corvint/")
	}
}

// A Git.Root below the top level is refused: Git names paths from the top
// level, so a subdirectory root would read files under the wrong names.
func TestTIVV1005WorkTreeRefusesSubdirectoryRoot(t *testing.T) {
	root, git := goOnlyRepo(t)
	writeFile(t, root, "sub/s_test.go", "package sub\n")
	git.Root = filepath.Join(root, "sub")
	if _, err := NewWorkTree(context.Background(), git); err == nil {
		t.Fatal("NewWorkTree accepted a root below the Git top level")
	}
	if _, err := Refresh(context.Background(), Options{Git: git}); err == nil {
		t.Fatal("Refresh accepted a root below the Git top level")
	}
}
