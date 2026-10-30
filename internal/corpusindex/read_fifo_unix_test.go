//go:build darwin || linux

package corpusindex

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// V1-3006: a writerless FIFO is refused promptly instead of blocking the open,
// and a cancelled context stops a regular read.
func TestReadFileRefusesAFIFOWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "index.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ReadFile(fifo, MaxBytes)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || err.Error() != "unsafe or oversized input" {
			t.Fatalf("FIFO read = %v, want refusal", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadFile blocked on a writerless FIFO")
	}
	regular := filepath.Join(dir, "index.json")
	if err := os.WriteFile(regular, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadFileContext(ctx, regular, MaxBytes); err != context.Canceled {
		t.Fatalf("cancelled read = %v, want context.Canceled", err)
	}
	if raw, err := ReadFileContext(context.Background(), regular, MaxBytes); err != nil || string(raw) != "{}" {
		t.Fatalf("regular read = %q, %v", raw, err)
	}
	if _, err := ReadFileContext(context.Background(), regular, 1); err == nil {
		t.Fatal("over-limit regular file read succeeded")
	}
}
