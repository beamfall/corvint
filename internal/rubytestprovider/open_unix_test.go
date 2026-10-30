//go:build darwin || linux

package rubytestprovider

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// LPCV-V0-092: the bounded reader never follows a final symlink and never
// blocks on a FIFO that replaced an admitted file.
func TestLPCVV0092BoundedReaderRefusesSymlinkAndFIFO(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "real.rb"), "# real\n", 0o644)
	if err := os.Symlink("real.rb", filepath.Join(dir, "link.rb")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe.rb"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openRoot(t, dir)
	if _, err := readBounded(root, "link.rb", MaxSpecFileBytes); err == nil {
		t.Fatal("readBounded followed a final symlink")
	}
	done := make(chan error, 1)
	go func() {
		file, err := openNoFollow(root, "pipe.rb")
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
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("openNoFollow blocked on a FIFO")
	}
	if _, err := readBounded(root, "pipe.rb", MaxSpecFileBytes); err == nil {
		t.Fatal("readBounded admitted a FIFO")
	}
	if data, err := readBounded(root, "real.rb", MaxSpecFileBytes); err != nil || string(data) != "# real\n" {
		t.Fatalf("regular read: %q %v", data, err)
	}
}
