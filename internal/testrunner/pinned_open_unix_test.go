//go:build darwin || linux

package testrunner

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// V1-0624: the pinned-file check must neither block on a FIFO nor accept a final
// symlink that os.Root would follow, and it must not need to open "/" or any
// ancestor as a directory: a search-only ancestor stands in for a sandbox such
// as Landlock that denies reading them.
func TestCheckPinnedFileRefusesFIFOAndFinalSymlink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the search-only ancestor")
	}
	dir := filepath.Join(t.TempDir(), "search-only")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	body := []byte("plugins {}\n")
	regular := filepath.Join(dir, "build.gradle")
	if err := os.WriteFile(regular, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("build.gradle", filepath.Join(dir, "link.gradle")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo.gradle"), 0600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	if err := os.Chmod(dir, 0100); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0700) })
	if err := checkPinnedFile("", regular, Digest(body)); err != nil {
		t.Fatalf("regular pinned file below a search-only ancestor refused: %v", err)
	}
	for name, want := range map[string]string{"link.gradle": "symlink input refused", "fifo.gradle": "nonregular file refused"} {
		done := make(chan error, 1)
		go func() { done <- checkPinnedFile(dir, name, Digest(body)) }()
		select {
		case err := <-done:
			if err == nil || err.Error() != want {
				t.Fatalf("%s: got %v, want %s", name, err, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: pinned check blocked", name)
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
