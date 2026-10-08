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
