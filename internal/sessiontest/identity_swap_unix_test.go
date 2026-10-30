//go:build unix && !solaris

package sessiontest

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestSTRV0060PathDigestRefusesSwap pins STR-V0-060 (V1-2129): a dirty path
// replaced between pathDigest's Lstat and its open by a symlink (to another
// file or to its own renamed inode), a FIFO with no writer, or another regular
// file is refused with errPathChanged inside a
// deadline; the link is never followed and the FIFO never blocks the digest.
func TestSTRV0060PathDigestRefusesSwap(t *testing.T) {
	swaps := map[string]func(t *testing.T, path, other string){
		"symlink": func(t *testing.T, path, other string) {
			if err := os.Remove(path); err != nil {
				t.Error(err)
			}
			if err := os.Symlink(other, path); err != nil {
				t.Error(err)
			}
		},
		// The link names the original file's own inode, so only a no-follow
		// open, not the post-open identity check, refuses it.
		"symlink-to-same-inode": func(t *testing.T, path, _ string) {
			moved := path + ".orig"
			if err := os.Rename(path, moved); err != nil {
				t.Error(err)
			}
			if err := os.Symlink(moved, path); err != nil {
				t.Error(err)
			}
		},
		"fifo": func(t *testing.T, path, _ string) {
			if err := os.Remove(path); err != nil {
				t.Error(err)
			}
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Error(err)
			}
		},
		"other-regular-file": func(t *testing.T, path, other string) {
			if err := os.Rename(other, path); err != nil {
				t.Error(err)
			}
		},
	}
	for label, swap := range swaps {
		t.Run(label, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "a.go")
			other := filepath.Join(dir, "secret")
			if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(other, []byte("not the dirty file\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			pathDigestSwapHook = func(name string) {
				if name == path {
					swap(t, path, other)
				}
			}
			t.Cleanup(func() { pathDigestSwapHook = nil })
			type result struct {
				digest string
				err    error
			}
			done := make(chan result, 1)
			go func() {
				digest, err := pathDigest(path)
				done <- result{digest, err}
			}()
			select {
			case got := <-done:
				if !errors.Is(got.err, errPathChanged) {
					t.Fatalf("pathDigest after a %s swap = %q, %v; want errPathChanged", label, got.digest, got.err)
				}
			case <-time.After(5 * time.Second):
				// Unblock a reader stuck on the FIFO so the test binary can exit.
				if w, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
					w.Close()
				}
				t.Fatalf("pathDigest blocked after a %s swap", label)
			}
		})
	}
	// Without a swap the same file still digests.
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if digest, err := pathDigest(path); err != nil || len(digest) != len("sha256:")+64 {
		t.Fatalf("unswapped digest %q, %v", digest, err)
	}
}
