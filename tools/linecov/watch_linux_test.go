//go:build linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestNativeSaveNotification(t *testing.T) {
	root := module(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events, err := saveNotifications(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(root, "p", "save.tmp")
	if err := os.WriteFile(tmp, []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(root, "p", "p.go")); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-events:
		if !ok {
			t.Fatal("watcher closed before save")
		}
	case <-ctx.Done():
		t.Fatal("atomic editor save was missed")
	}
}

// LPCV-V0-7607: watch existing untracked, newly created and moved directories,
// including saves after their initial parent event has been consumed.
func TestNativeNewDirectorySaves(t *testing.T) {
	for _, mode := range []string{"existing-untracked", "created", "moved"} {
		t.Run(mode, func(t *testing.T) {
			root := module(t)
			dir := filepath.Join(root, "new", "nested")
			if mode == "existing-untracked" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			events, err := saveNotifications(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "created" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			} else if mode == "moved" {
				source := filepath.Join(t.TempDir(), "source")
				if err := os.MkdirAll(filepath.Join(source, "nested"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(source, filepath.Dir(dir)); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "existing-untracked" {
				awaitSave(t, ctx, events)
				quietSaves(t, ctx, events)
			}
			for i := 0; i < 3; i++ {
				if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package new\n"), 0600); err != nil {
					t.Fatal(err)
				}
				awaitSave(t, ctx, events)
				quietSaves(t, ctx, events)
			}
		})
	}
}
func awaitSave(t *testing.T, ctx context.Context, events <-chan struct{}) {
	t.Helper()
	select {
	case _, ok := <-events:
		if !ok {
			t.Fatal("watcher stopped before save")
		}
	case <-ctx.Done():
		t.Fatal("save was missed")
	}
}

// A bounded quiet interval prevents a queued mkdir or earlier save from
// satisfying the next save. This consumes events, without filesystem polling.
func quietSaves(t *testing.T, ctx context.Context, events <-chan struct{}) {
	t.Helper()
	timer := time.NewTimer(30 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-events:
			if !ok {
				t.Fatal("watcher stopped")
			}
			timer.Reset(30 * time.Millisecond)
		case <-timer.C:
			return
		case <-ctx.Done():
			t.Fatal("watcher never became quiet")
		}
	}
}

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
