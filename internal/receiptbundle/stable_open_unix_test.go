//go:build unix && !solaris

package receiptbundle

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// CEM-V1-063: a bundle entry that is a FIFO when it is opened does not block
// the reader; the open returns and the identity check refuses it.
func TestStableEntryOpenDoesNotBlockOnAFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "artifact.json"), 0o600); err != nil {
		t.Skipf("NOT_RUN: mkfifo: %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	done := make(chan error, 1)
	go func() {
		file, err := openStableEntry(root, "artifact.json")
		if err == nil {
			info, statErr := file.Stat()
			file.Close()
			if statErr == nil && info.Mode().IsRegular() {
				err = os.ErrInvalid
			}
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == os.ErrInvalid {
			t.Fatal("a FIFO was reported as a regular file")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("opening a FIFO bundle entry blocked")
	}
	if _, err := readRootFile(root, "artifact.json", 16); err == nil {
		t.Fatal("a FIFO was read as an artifact")
	}
}
