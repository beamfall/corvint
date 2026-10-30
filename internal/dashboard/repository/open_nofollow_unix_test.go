//go:build darwin || linux

package repository

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestOpenNoFollowFileDoesNotBlockOnFIFO covers the window between a
// caller's Lstat and its open: a regular file swapped for a FIFO there must
// reach the caller's fstat regular-file check instead of blocking the open.
func TestOpenNoFollowFileDoesNotBlockOnFIFO(t *testing.T) {
	directory := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(directory, "config"), 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	done := make(chan *os.File, 1)
	go func() {
		file, _ := openNoFollowFile(root, "config")
		done <- file
	}()
	select {
	case file := <-done:
		if file != nil {
			_ = file.Close()
		}
	case <-time.After(5 * time.Second):
		t.Fatal("openNoFollowFile blocked on a FIFO")
	}
}
