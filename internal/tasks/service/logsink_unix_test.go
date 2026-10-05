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
	root := resolvedTempDir(t)
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

// A symlinked logs/ or logs/<unit> is refused: appends, rotation and
// status never leave the pinned state root.
func TestSERVICE500_LogDirectorySymlinkIsRefused(t *testing.T) {
	for _, link := range []string{logDir, filepath.Join(logDir, "main")} {
		root, outside := resolvedTempDir(t), resolvedTempDir(t)
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, link)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
		u := openUnitLogs(root, "main", "stderr")
		_, _ = u.streams["stderr"].Write([]byte("hello"))
		waitFor(t, "LOG_IO_ERROR", func() bool { return u.streams["stderr"].Stats().IOError != "" })
		u.publish()
		u.close()
		if entries, _ := os.ReadDir(outside); len(entries) != 0 {
			t.Fatalf("%s: wrote %d entries through the symlink", link, len(entries))
		}
	}
}

// Shutdown settles the pipe readers before the final counters, so output
// a retired tree left in its pipes is counted; a pipe held open beyond the
// bound is closed instead of holding shutdown.
func TestSERVICE500_HelperDrainsSettleBeforeFinalCounters(t *testing.T) {
	w := &helperWrapper{logs: openUnitLogs(resolvedTempDir(t), helperUnit("web"), "stdout", "stderr"), drainBound: 2 * time.Second}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.startDrains([2]*os.File{outR, errR})
	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = outW.Write([]byte("late"))
		outW.Close()
		errW.Close()
	}()
	w.settleDrains()
	w.logs.close()
	if st := w.logs.streams["stdout"].Stats(); st.Written != 4 || st.Dropped != 0 {
		t.Fatalf("final counters missed the tree's last output: %+v", st)
	}

	held := &helperWrapper{logs: openUnitLogs(resolvedTempDir(t), helperUnit("web"), "stdout", "stderr"), drainBound: 50 * time.Millisecond}
	heldR, heldW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer heldW.Close()
	quietR, quietW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	quietW.Close()
	held.startDrains([2]*os.File{heldR, quietR})
	start := time.Now()
	held.settleDrains()
	if d := time.Since(start); d > time.Second {
		t.Fatalf("a held pipe blocked shutdown for %v", d)
	}
	held.logs.close()
}
