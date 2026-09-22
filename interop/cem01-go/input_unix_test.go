//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadBoundedRejectsNonRegularInputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "symlink")
	if err := os.Symlink(regular, symlink); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, path := range []string{dir, symlink, fifo} {
		started := time.Now()
		_, err := readBounded(ctx, path, 100)
		if !errors.Is(err, errNotRegular) {
			t.Errorf("%s: got %v", filepath.Base(path), err)
		}
		if time.Since(started) > 250*time.Millisecond {
			t.Errorf("%s blocked for %s", filepath.Base(path), time.Since(started))
		}
	}
}

func TestReadBoundedHonorsCancelledContext(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := readBounded(ctx, path, 100)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
