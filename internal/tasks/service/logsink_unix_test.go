//go:build darwin || linux

package service

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A FIFO planted at a stream log path neither blocks status nor the
// stream writer: both refuse it as a non-regular file.
func TestSERVICE500_LogFIFONeverBlocks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, logDir, "main")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "stderr.log"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Host{UID: os.Getuid()}.logExcerpt(root, "main", "stderr")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("excerpt read a FIFO")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("excerpt blocked on a FIFO")
	}
	s := openLogSink(dir, "stderr")
	_, _ = s.Write([]byte("hello"))
	waitFor(t, "LOG_IO_ERROR", func() bool { return s.Stats().IOError != "" })
	if st := s.Close(time.Second); !strings.HasPrefix(st.IOError, "LOG_IO_ERROR: ") || st.Dropped != 5 {
		t.Fatalf("stats %+v", st)
	}
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		t.Fatal("writer blocked on a FIFO")
	}
}
