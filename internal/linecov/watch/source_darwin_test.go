//go:build darwin

// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func vnodeFixture(t *testing.T) (*vnodeWatches, string) {
	t.Helper()
	root := t.TempDir()
	kq, err := syscall.Kqueue()
	if err != nil {
		t.Fatal(err)
	}
	w := &vnodeWatches{kq: kq, root: root, fds: map[string]int{}, paths: map[int]string{}, dirs: map[string]bool{}}
	t.Cleanup(w.closeAll)
	return w, root
}

// LPCV-V0-1086: a directory rescan rewatches a file whose inode changed under
// a kept name (an atomic replacement whose own delete event was coalesced),
// watches new entries including whole new trees, and drops gone entries.
func TestVnodeRescanReconcilesInodes(t *testing.T) {
	w, root := vnodeFixture(t)
	write(t, root, "p/p.go", "package p\n")
	write(t, root, "p/gone.go", "package p\n")
	if err := w.addTree(root); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "p", "p.go")
	before := w.fds[target]
	if before == 0 || !w.dirs[filepath.Join(root, "p")] {
		t.Fatalf("tree not watched: %v", w.fds)
	}
	// The kernel reuses the lowest free descriptor number, so a rewatch can
	// get the old number back: compare the inodes the descriptors name.
	var old syscall.Stat_t
	if err := syscall.Fstat(before, &old); err != nil {
		t.Fatal(err)
	}
	// Replace p.go without an event being processed, remove gone.go, add a tree.
	write(t, root, "p/p.go.tmp", "package p\n\nfunc F() {}\n")
	if err := os.Rename(filepath.Join(root, "p", "p.go.tmp"), target); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "p", "gone.go")); err != nil {
		t.Fatal(err)
	}
	write(t, root, "p/q/deep/d.go", "package deep\n")
	added, err := w.rescan(filepath.Join(root, "p"))
	if err != nil {
		t.Fatal(err)
	}
	after := w.fds[target]
	var now syscall.Stat_t
	if after == 0 || syscall.Fstat(after, &now) != nil || now.Ino == old.Ino ||
		!w.sameInode(after, target) {
		t.Fatalf("replaced inode kept its stale descriptor: before %d (inode %d) after %d (inode %d)",
			before, old.Ino, after, now.Ino)
	}
	if _, ok := w.fds[filepath.Join(root, "p", "gone.go")]; ok {
		t.Fatal("removed file kept its watch")
	}
	deep := filepath.Join(root, "p", "q", "deep", "d.go")
	if _, ok := w.fds[deep]; !ok || !w.dirs[filepath.Join(root, "p", "q")] {
		t.Fatalf("new tree not watched: %v", w.fds)
	}
	seen := map[string]bool{}
	for _, p := range added {
		seen[p] = true
	}
	gone := filepath.Join(root, "p", "gone.go")
	if !seen[target] || !seen[filepath.Join(root, "p", "q")] || !seen[gone] {
		t.Fatalf("rescan reported %v", added)
	}
	for path, fd := range w.fds {
		if w.paths[fd] != path {
			t.Fatalf("descriptor maps disagree at %s", path)
		}
	}
}

// LPCV-V0-1086: dropping a directory closes its whole subtree, and a rescan
// of a directory that no longer exists drops it without error.
func TestVnodeDropDirectoryTakesSubtree(t *testing.T) {
	w, root := vnodeFixture(t)
	write(t, root, "a/b/c.go", "package b\n")
	write(t, root, "a/x.go", "package a\n")
	if err := w.addTree(root); err != nil {
		t.Fatal(err)
	}
	n := len(w.fds)
	w.drop(filepath.Join(root, "a"))
	if len(w.fds) != n-4 || len(w.paths) != len(w.fds) {
		t.Fatalf("subtree not dropped: %v", w.fds)
	}
	if err := os.RemoveAll(filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	if added, err := w.rescan(filepath.Join(root, "a")); err != nil || len(added) != 0 {
		t.Fatalf("rescan of a removed directory: %v %v", added, err)
	}
}

// LPCV-V0-1086: descriptor exhaustion is named with its remedy; other errors
// pass through unchanged.
func TestDescriptorHint(t *testing.T) {
	if got := descriptorHint(syscall.EMFILE).Error(); got == syscall.EMFILE.Error() {
		t.Fatalf("EMFILE carries no remedy: %s", got)
	}
	if got := descriptorHint(syscall.ENOENT); got != syscall.ENOENT {
		t.Fatalf("ENOENT rewritten: %v", got)
	}
}
