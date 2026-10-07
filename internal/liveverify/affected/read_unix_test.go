//go:build darwin || linux

package affected

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestReadSourceRefusesNonRegularFilesOnOpenDescriptor pins the disk reader's
// admission after it stopped stat-ing the path first (V1-0416): a symlink,
// a directory and a FIFO are refused as ErrInvalidUnit, a FIFO without
// blocking, an over-bound file as ErrWalkLimit, and a missing optional file
// as fs.ErrNotExist.
func TestReadSourceRefusesNonRegularFilesOnOpenDescriptor(t *testing.T) {
	t.Parallel()
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
	for _, tc := range []struct {
		name    string
		wantErr error
	}{
		{"link.go", ErrInvalidUnit},
		{"dir.go", ErrInvalidUnit},
		{"fifo.go", ErrInvalidUnit},
		{"large.go", ErrWalkLimit},
		{"missing.go", fs.ErrNotExist},
	} {
		started := time.Now()
		body, err := ReadSource(root, tc.name)
		if !errors.Is(err, tc.wantErr) || body != nil {
			t.Fatalf("%s: body=%d bytes, err=%v, want %v", tc.name, len(body), err, tc.wantErr)
		}
		if elapsed := time.Since(started); elapsed > 5*time.Second {
			t.Fatalf("%s: refusal took %s", tc.name, elapsed)
		}
	}
}
