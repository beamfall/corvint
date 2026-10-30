//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// V1-3018: automatic discovery watches only regular files (or links to
// them); a FIFO named *.go is excluded rather than opened at session start.
func TestResolveWatchedFilesExcludesNonRegular(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "a.go")
	if err := os.WriteFile(regular, []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(regular, filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "blocked.go"), 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	files, _, err := resolveWatchedFiles(root, nil)
	if err != nil || len(files) != 2 || files[0] != regular || files[1] != filepath.Join(root, "link.go") {
		t.Fatalf("files=%v err=%v", files, err)
	}
}
