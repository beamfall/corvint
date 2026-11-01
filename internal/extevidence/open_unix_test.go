//go:build unix

package extevidence

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// withinDeadline runs read and fails the test when it does not return in time;
// a FIFO left blocked is released by opening its write side, so no goroutine
// outlives the test.
func withinDeadline(t *testing.T, fifo string, read func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); read() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		if writer, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			writer.Close()
		}
		<-done
		t.Fatal("the read blocked on a FIFO")
	}
}

// TestEEPV0805_LegacyLoaderRefusesSpecialFilesAndBoundsReads reproduces
// V1-0562 on the legacy file loader and the pinned-source open: before the
// fix a FIFO passed os.Stat (size 0) and blocked os.ReadFile forever, and a
// symlink to /dev/zero passed it and was read without bound. Both are now
// refused through one non-blocking descriptor; an oversized file is invalid;
// an ordinary record reads the same bytes as os.ReadFile (EEP-V0-805).
func TestEEPV0805_LegacyLoaderRefusesSpecialFilesAndBoundsReads(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fifo := filepath.Join(dir, "provider.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	var fifoEntry provider
	withinDeadline(t, fifo, func() { fifoEntry = load(context.Background(), rootRepository{dir: dir}, fifo) })
	if fifoEntry.state != StateUnavailable || !strings.Contains(fifoEntry.reason, "regular file") {
		t.Fatalf("FIFO record: state %q reason %q", fifoEntry.state, fifoEntry.reason)
	}
	withinDeadline(t, fifo, func() {
		if file, err := openRegular(fifo); err == nil {
			file.Close()
			t.Error("openRegular admitted a FIFO")
		}
		if file, err := openSource(fifo); err != nil {
			t.Errorf("non-blocking open of a FIFO: %v", err)
		} else {
			file.Close()
		}
	})

	if _, err := os.Stat("/dev/zero"); err == nil {
		zero := filepath.Join(dir, "zero.json")
		if err := os.Symlink("/dev/zero", zero); err != nil {
			t.Fatal(err)
		}
		entry := load(context.Background(), rootRepository{dir: dir}, zero)
		if entry.state != StateUnavailable || !strings.Contains(entry.reason, "regular file") {
			t.Fatalf("device record: state %q reason %q", entry.state, entry.reason)
		}
	}

	large := filepath.Join(dir, "large.json")
	if err := os.WriteFile(large, bytes.Repeat([]byte{' '}, MaxRecordBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if entry := load(context.Background(), rootRepository{dir: dir}, large); entry.state != StateInvalid || entry.reason != "record exceeds 1048576 bytes" {
		t.Fatalf("oversized record: state %q reason %q", entry.state, entry.reason)
	}

	ordinary := filepath.Join(dir, "ordinary.json")
	body := []byte(`{"schema":"corvint.external-evidence/v0"}`)
	if err := os.WriteFile(ordinary, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readRecordFile(ordinary)
	want, _ := os.ReadFile(ordinary)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("ordinary record: %q, %v; want %q", got, err, want)
	}
}
