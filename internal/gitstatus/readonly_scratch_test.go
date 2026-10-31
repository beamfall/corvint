//go:build darwin || linux

package gitstatus

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// readOnlyTree removes write permission from every entry under root and
// restores it at cleanup, so the test can still remove its temporary tree.
func readOnlyTree(t *testing.T, root string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	var modes []func()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		modes = append(modes, func() { _ = os.Chmod(path, mode) })
		return os.Chmod(path, mode&^0o222)
	})
	t.Cleanup(func() {
		for _, restore := range modes {
			restore()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

// GPK-V0-336: a read-only checkout under an unwritable TMPDIR (a read-only
// filesystem sandbox) still reads its isolated status through a fallback parent.
func TestGPKV0336StatusFallsBackFromUnwritableTMPDIROnReadOnlyCheckout(t *testing.T) {
	root := fixture(t)
	writeTest(t, filepath.Join(root, "value.go"), "package value\nconst Dirty = 1\n")
	writeTest(t, filepath.Join(root, "untracked"), "new")
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}
	want := gitTest(t, root, args...)
	unwritable := t.TempDir()
	readOnlyTree(t, root)
	readOnlyTree(t, unwritable)
	t.Setenv("TMPDIR", unwritable)
	got, err := Status(context.Background(), root, metadataLimit, testRun, args...)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("read-only status=%q want=%q error=%v", got, want, err)
	}
	if entries, err := os.ReadDir(unwritable); err != nil || len(entries) != 0 {
		t.Fatalf("unwritable TMPDIR gained entries %v error=%v", entries, err)
	}
}

// GPK-V0-336: with no writable parent the refusal names every location tried and
// the remedy; a single caller-owned parent keeps its original refusal.
func TestGPKV0336ScratchRefusalNamesEveryTriedLocation(t *testing.T) {
	root := fixture(t)
	first, second := t.TempDir(), t.TempDir()
	readOnlyTree(t, first)
	readOnlyTree(t, second)
	_, err := scratchUnder(context.Background(), []string{first, second}, root)
	if !errors.Is(err, errScratchNowhere) || RefusalClass(err) != string(classScratchDir) {
		t.Fatalf("all-unwritable error=%v class=%s", err, RefusalClass(err))
	}
	want := "Git status cannot safely observe repository metadata: private metadata scratch directory cannot be created in TMPDIR, /tmp or the user cache directory; set TMPDIR to a writable directory outside the repository"
	if RefusalMessage(err) != want {
		t.Fatalf("refusal=%q want %q", RefusalMessage(err), want)
	}
	if _, err := StatusIn(context.Background(), root, first, metadataLimit, testRun, "status", "--porcelain=v1", "-z"); !errors.Is(err, errScratchCreate) {
		t.Fatalf("caller-owned unwritable parent error=%v, want the original refusal", err)
	}
	// A fallback inside the repository is skipped, never used.
	if _, err := scratchUnder(context.Background(), []string{first, root}, root); !errors.Is(err, errScratchNowhere) {
		t.Fatalf("fallback inside the repository error=%v", err)
	}
}
