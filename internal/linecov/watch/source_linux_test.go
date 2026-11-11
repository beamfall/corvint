//go:build linux

// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// LPCV-V0-7607: overflow rescans; a lost watch is restored. An unwatchable
// root must end automatic watching rather than silently retain missing watches.
func TestNativeLostDirectoryEvents(t *testing.T) {
	root := t.TempDir()
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	w := &directoryWatches{fd: fd, root: root, paths: map[string]int{}}
	if err := w.refresh(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "lost", "nested")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	event := func(wd int, mask uint32) []byte {
		b := make([]byte, syscall.SizeofInotifyEvent)
		binary.NativeEndian.PutUint32(b, uint32(wd))
		binary.NativeEndian.PutUint32(b[4:], mask)
		return b
	}
	if err := w.events(event(-1, syscall.IN_Q_OVERFLOW)); err != nil {
		t.Fatal(err)
	}
	wd, ok := w.paths[dir]
	if !ok {
		t.Fatal("overflow did not restore nested watches")
	}
	// Lose the delete/recreate events too: a path's presence in the map does
	// not prove that its current inode is watched.
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := w.events(event(-1, syscall.IN_Q_OVERFLOW)); err != nil {
		t.Fatal(err)
	}
	if w.paths[dir] == wd {
		t.Fatal("overflow retained the deleted inode's watch")
	}
	wd = w.paths[dir]
	if _, err := syscall.InotifyRmWatch(fd, uint32(wd)); err != nil {
		t.Fatal(err)
	}
	if err := w.events(event(wd, syscall.IN_IGNORED)); err != nil {
		t.Fatal(err)
	}
	if next, ok := w.paths[dir]; !ok || next == wd {
		t.Fatal("ignored watch was not restored")
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := w.events(event(-1, syscall.IN_Q_OVERFLOW)); err == nil {
		t.Fatal("lost root silently retained incomplete watching")
	}
}
