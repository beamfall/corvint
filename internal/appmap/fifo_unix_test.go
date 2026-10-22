//go:build unix && !solaris

package appmap

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestLoadMapRefusesFIFOSwappedAfterTypeCheck pins V1-0971: a FIFO swapped in between the type
// check and the open is refused promptly instead of blocking the read.
func TestLoadMapRefusesFIFOSwappedAfterTypeCheck(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular.json")
	if err := os.WriteFile(regular, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "map.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	// The type check sees a regular file; the open then meets the FIFO.
	lstatInput = func(string) (os.FileInfo, error) { return os.Lstat(regular) }
	t.Cleanup(func() { lstatInput = os.Lstat })
	done := make(chan error, 1)
	go func() {
		_, err := LoadMap(fifo)
		done <- err
	}()
	select {
	case err := <-done:
		if codeOf(err) != "appmap-invalid-map" {
			t.Fatalf("LoadMap(FIFO) = %v, want appmap-invalid-map", err)
		}
	case <-time.After(5 * time.Second):
		// Unblock the reader so the test binary can exit.
		if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			w.Close()
		}
		t.Fatal("LoadMap blocked on a FIFO swapped in after the type check")
	}
}
