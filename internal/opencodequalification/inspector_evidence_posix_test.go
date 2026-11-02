//go:build darwin || linux

package opencodequalification

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestAHI620WitnessLogRefusesFIFO: a FIFO witness log is refused without blocking the timed-out witness.
func TestAHI620WitnessLogRefusesFIFO(t *testing.T) {
	shortWitnessTimeout(t)
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "witness.jsonl"), 0o600); err != nil {
		t.Skipf("mkfifo unsupported: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- waitFrame(context.Background(), root, "sidebar", []string{"Corvint"}, nil, 0) }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "the witness plugin never logged loaded") {
			t.Fatalf("FIFO witness log: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("waitFrame blocked on a FIFO witness log")
	}
	if _, err := readRegularBounded(filepath.Join(root, "witness.jsonl"), maxEvidenceBytes); !errors.Is(err, errNotRegularEvidence) {
		t.Fatalf("FIFO witness log: err %v, want errNotRegularEvidence", err)
	}
}
