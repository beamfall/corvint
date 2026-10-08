//go:build darwin || linux

package testrunner

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// V1-0624: the pinned-file open must neither block on a FIFO nor accept a final
// symlink that os.Root would follow, even when the path checks have passed.
func TestOpenPinnedRegularRefusesFIFOAndFinalSymlink(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "build.gradle")
	if err := os.WriteFile(regular, []byte("plugins {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("build.gradle", filepath.Join(dir, "link.gradle")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo.gradle"), 0600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if f, st, err := openPinnedRegular(root, "build.gradle"); err != nil || !st.Mode().IsRegular() {
		t.Fatalf("regular pinned file refused: %v", err)
	} else {
		f.Close()
	}
	for _, name := range []string{"link.gradle", "fifo.gradle"} {
		done := make(chan error, 1)
		go func() {
			f, _, err := openPinnedRegular(root, name)
			if f != nil {
				f.Close()
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil || err.Error() != "nonregular file refused" {
				t.Fatalf("%s: got %v, want nonregular file refused", name, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: pinned open blocked", name)
		}
	}
}

// V1-0624: the controlled interleaving. The no-follow check saw a regular file;
// the path is then replaced by a symlink to that same file, or by a FIFO, before
// the open. Both refuse without blocking.
func TestOpenCheckedRegularRefusesSwapAfterCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build.gradle")
	for _, swap := range []string{"symlink", "fifo"} {
		if err := os.WriteFile(path, []byte("plugins {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		before, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		moved := filepath.Join(dir, "moved-"+swap)
		if err = os.Rename(path, moved); err != nil {
			t.Fatal(err)
		}
		if swap == "symlink" {
			err = os.Symlink(filepath.Base(moved), path)
		} else {
			err = syscall.Mkfifo(path, 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			f, _, err := openCheckedRegular(path, before)
			if f != nil {
				f.Close()
			}
			done <- err
		}()
		select {
		case err = <-done:
			if err == nil || err.Error() != "nonregular file refused" {
				t.Fatalf("%s swap: got %v, want nonregular file refused", swap, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s swap: checked open blocked", swap)
		}
		if err = os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}
