//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// WQO-V0-017: readable type evidence survives unavailable content evidence.
func TestWorkMutationRetainsSpecialTypeAndAbsentRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cemWrite(t, root, "file", "before")
	opening := workMutationManifest(context.Background(), []string{root})
	path := filepath.Join(root, "file")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	closing := workCloseManifest(opening, workMutationManifest(context.Background(), []string{root}))
	if !closing.changed || closing.monitoredComplete {
		t.Fatalf("type evidence erased %#v", closing)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	closing = workCloseManifest(opening, workMutationManifest(context.Background(), []string{root}))
	if !closing.changed || closing.monitoredComplete {
		t.Fatalf("absence evidence erased %#v", closing)
	}
}
