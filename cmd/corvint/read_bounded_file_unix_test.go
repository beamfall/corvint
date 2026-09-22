//go:build darwin || linux

package main

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FPK-V0-012, FPK-V0-021, FPK-V0-024, FPK-V0-031: special files cannot block bounded reads.
func TestReadBoundedFileRefusesFIFO(t *testing.T) {
	t.Parallel()
	fifo := filepath.Join(t.TempDir(), "input")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := readBoundedFile(fifo, 1)
		result <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("readBoundedFile FIFO error = %v", err)
		}
	case <-ctx.Done():
		t.Fatal("readBoundedFile blocked opening a FIFO")
	}
}
