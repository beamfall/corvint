//go:build darwin

package authoritystore

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRepositoryDirectoryOpenRejectsFIFO(t *testing.T) {
	for _, fifo := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "not-directory")
		if fifo {
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			f, err := openBindingDirectory(path)
			if f != nil {
				f.Close()
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("non-directory opened")
			}
		case <-time.After(time.Second):
			t.Fatal("directory open blocked on non-directory")
		}
	}
}
