//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package doccompiler

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// SDD-V0-019 (V1-3089): an untracked FIFO owner is refused as a non-regular
// file without opening it; opening a FIFO with no writer blocks.
func TestSDDV0019ReadDryRunOwnerRefusesFIFO(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "owner.md"), 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ReadDryRunOwner(root, "owner.md")
		done <- err
	}()
	select {
	case err := <-done:
		var problem *Error
		if !errors.As(err, &problem) || problem.Code != "unsupported-documentation-source" {
			t.Fatalf("fifo owner: %v, want unsupported-documentation-source", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ReadDryRunOwner blocked on a FIFO owner")
	}
}
