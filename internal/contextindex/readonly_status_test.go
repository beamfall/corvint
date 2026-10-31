//go:build darwin || linux

package contextindex

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// GPK-V0-336: the standalone status read behind `corvint context` reads a
// read-only checkout under an unwritable TMPDIR through the fallback parent.
func TestGPKV0336StatusSnapshotReadsReadOnlyCheckoutUnderUnwritableTMPDIR(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	root := authorityRepository(t)
	writeTestFile(t, root, "untracked.txt", "new\n")
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
	paths, _, err := readStatusSnapshot(context.Background(), root)
	if err != nil {
		t.Fatalf("read-only status snapshot error=%v", err)
	}
	if !slices.Contains(paths, "untracked.txt") {
		t.Fatalf("read-only status snapshot paths=%v, want untracked.txt", paths)
	}
}
