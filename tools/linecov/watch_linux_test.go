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
