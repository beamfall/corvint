//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestProcessUmaskSetsStableFileMode(t *testing.T) {
	previous := syscall.Umask(0o077)
	defer syscall.Umask(previous)
	setProcessUmask()

	path := filepath.Join(t.TempDir(), "created")
	if err := os.WriteFile(path, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("process umask: got mode %04o, want 0644", got)
	}
}
