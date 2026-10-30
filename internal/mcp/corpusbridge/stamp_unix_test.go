// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package corpusbridge

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestDCPV1314StampRefusesFIFOConfig: a FIFO swapped in for the repository
// config makes the stamp unavailable at once instead of blocking a read.
func TestDCPV1314StampRefusesFIFOConfig(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	dir := stampRepository(t)
	config := filepath.Join(dir, ".git", "config")
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(config, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	t.Cleanup(func() {
		// Release a reader blocked on the FIFO, if any.
		if f, err := os.OpenFile(config, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
	})
	done := make(chan string, 1)
	go func() { done <- checkoutStamp(context.Background(), []string{dir}) }()
	select {
	case got := <-done:
		if got != "" {
			t.Fatalf("stamp %q over a FIFO config, want unavailable", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stamp blocked reading a FIFO config")
	}
}
