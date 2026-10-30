//go:build darwin || linux

package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// V1-3018: the session identity read refuses a FIFO without blocking, names a
// file over the per-file cap or the aggregate cap, and honours cancellation.
func TestDigestBoundsWatchedReads(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "a.go")
	if err := os.WriteFile(regular, []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "blocked.go")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	s := &Session{cfg: Config{ToolchainVersion: "go-test"}}

	done := make(chan error, 1)
	go func() {
		_, _, err := s.digest(context.Background(), []string{regular, fifo})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), fifo) || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("FIFO digest = %v, want a refusal naming %s", err, fifo)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("digest blocked on a writerless FIFO")
	}

	huge := filepath.Join(dir, "huge.go")
	if err := os.WriteFile(huge, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(huge, maxWatchedFileBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.digest(context.Background(), []string{regular, huge}); err == nil || !strings.Contains(err.Error(), huge) {
		t.Fatalf("oversized digest = %v, want a refusal naming %s", err, huge)
	}
	if _, err := readWatchedContent(context.Background(), regular, 4); err == nil || !strings.Contains(err.Error(), regular) {
		t.Fatalf("over-budget read = %v, want a refusal naming %s", err, regular)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := s.digest(ctx, []string{regular}); err != context.Canceled {
		t.Fatalf("cancelled digest = %v, want context.Canceled", err)
	}
	identity, digests, err := s.digest(context.Background(), []string{regular})
	if err != nil || identity == "" || len(digests) != 1 {
		t.Fatalf("regular digest = %q %v %v", identity, digests, err)
	}
}
