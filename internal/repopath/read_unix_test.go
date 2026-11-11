// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package repopath

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// ReadSource refuses a non-canonical path, a symlink, a directory and a FIFO
// as ErrNotCanonical, an over-bound file as ErrWalkLimit, and a missing file
// as fs.ErrNotExist; the planner's own test covers the socket and the
// unreadable directory.
func TestReadSourceRefusesNonRegularFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "regular.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("regular.go", filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "fifo.go"), 0o644); err != nil {
		t.Fatal(err)
	}
	large, err := os.Create(filepath.Join(root, "large.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(MaxSourceBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := large.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := ReadSource(root, "regular.go")
	if err != nil || string(body) != "package x\n" {
		t.Fatalf("regular file = %q, %v", body, err)
	}
	for name, want := range map[string]error{"../regular.go": ErrNotCanonical, "link.go": ErrNotCanonical, "dir.go": ErrNotCanonical, "fifo.go": ErrNotCanonical, "large.go": ErrWalkLimit, "missing.go": fs.ErrNotExist} {
		if body, err := ReadSource(root, name); !errors.Is(err, want) || body != nil {
			t.Fatalf("%s: body=%d bytes, err=%v, want %v", name, len(body), err, want)
		}
	}
}
