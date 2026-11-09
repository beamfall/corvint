//go:build darwin || linux

package publish

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestReadBoundedFileFIFOReplacementIsBounded exercises the public bounded
// reader across the Lstat-to-open race. A regular input replaced by a FIFO
// must refuse without waiting for a writer.
func TestReadBoundedFileFIFOReplacementIsBounded(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	aside := filepath.Join(dir, "victim.aside")
	fifo := filepath.Join(dir, "fifo")
	if err := os.WriteFile(victim, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	previous := openInputFile
	swapped := make(chan struct{})
	openInputFile = func(path string) (*os.File, error) {
		if path != victim {
			return previous(path)
		}
		if err := os.Rename(victim, aside); err != nil {
			return nil, err
		}
		if err := os.Rename(fifo, victim); err != nil {
			return nil, err
		}
		close(swapped)
		return previous(path)
	}
	t.Cleanup(func() { openInputFile = previous })

	done := make(chan error, 1)
	go func() {
		_, err := ReadBoundedFile(victim, 8, cemcode.PatchUnavailable)
		done <- err
	}()
	select {
	case <-swapped:
	case <-time.After(5 * time.Second):
		t.Fatal("bounded read did not reach the injected open")
	}
	blocked := false
	var readErr error
	select {
	case readErr = <-done:
	case <-time.After(250 * time.Millisecond):
		blocked = true
		writer, err := os.OpenFile(victim, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_ = writer.Close()
		readErr = <-done
	}
	if err := os.Remove(victim); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(aside, victim); err != nil {
		t.Fatal(err)
	}
	if blocked {
		t.Fatal("bounded read blocked after the input became a FIFO")
	}
	if cemcode.CodeOf(readErr) != cemcode.PatchUnavailable {
		t.Fatalf("got %v", readErr)
	}
}
