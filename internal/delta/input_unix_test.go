//go:build darwin || linux

package delta

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestDeltaCaptureFIFORefusesWithoutWriter(t *testing.T) {
	t.Run("DLT-V0-005 safe regular file capture", testDeltaCaptureFIFORefusesWithoutWriter)
}
func testDeltaCaptureFIFORefusesWithoutWriter(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "record")
	if err := syscall.Mkfifo(name, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := capture(root, "record", 1024); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO admitted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("FIFO capture blocked")
	}
}
