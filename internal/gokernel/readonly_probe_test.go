//go:build darwin || linux

package gokernel

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// GPK-V0-336: the repository probe behind `corvint impact` reads a read-only
// checkout under an unwritable TMPDIR (a read-only filesystem sandbox) instead of
// refusing repository-probe-failed.
func TestGPKV0336ProbeReadsReadOnlyCheckoutUnderUnwritableTMPDIR(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	root := testRepository(t)
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n\nconst Dirty = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unwritable := t.TempDir()
	for _, tree := range []string{root, unwritable} {
		var restore []func()
		err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.Type()&fs.ModeSymlink != 0 {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			mode := info.Mode().Perm()
			restore = append(restore, func() { _ = os.Chmod(path, mode) })
			return os.Chmod(path, mode&^0o222)
		})
		t.Cleanup(func() {
			for _, undo := range restore {
				undo()
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMPDIR", unwritable)
	repository, err := ProbeRepositoryContext(context.Background(), root)
	if err != nil {
		t.Fatalf("read-only probe error=%v code=%s", err, kernelCode(err))
	}
	if repository.DirtyPathCount != 1 {
		t.Fatalf("read-only probe dirty paths=%d, want 1", repository.DirtyPathCount)
	}
}
