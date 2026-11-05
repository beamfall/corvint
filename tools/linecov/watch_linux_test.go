//go:build linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"path/filepath"
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

// Greptile PR #97: a directory created after start is watched, a tracked
// directory deleted before start is tolerated, and Git-ignored output alone
// schedules no run.
func TestNativeSaveNotificationDirectoriesAndIgnored(t *testing.T) {
	root := module(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("output.json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(root, "gone")
	if err := os.MkdirAll(gone, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gone, "g.go"), []byte("package gone\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "dirs")
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events, err := saveNotifications(ctx, root)
	if err != nil {
		t.Fatalf("deleted tracked directory aborted the watcher: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "p", "output.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
		t.Fatal("ignored output scheduled a run")
	case <-time.After(500 * time.Millisecond):
	}
	fresh := filepath.Join(root, "fresh")
	if err := os.Mkdir(fresh, 0700); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events: // the mkdir itself
	case <-ctx.Done():
		t.Fatal("directory creation was missed")
	}
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(fresh, "f.go"), []byte("package fresh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-ctx.Done():
		t.Fatal("save in a new directory was missed")
	}
}
