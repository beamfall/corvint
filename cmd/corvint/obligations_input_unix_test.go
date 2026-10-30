//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// V1-3017: --cem/--impact inputs are read through a bounded reader. A FIFO is
// refused without blocking, an oversized file is refused by its size, and an
// input at the ceiling, directly or through a symlink, reads as before.
func TestObligationsInputReadIsBounded(t *testing.T) {
	dir := t.TempDir()
	refusal := func(err error, want string) {
		t.Helper()
		var typed *gokernel.Error
		if !errors.As(err, &typed) || typed.Code != "invalid-obligations-input" || !strings.Contains(typed.Message, want) {
			t.Fatalf("refusal = %v, want invalid-obligations-input containing %q", err, want)
		}
	}

	fifo := filepath.Join(dir, "cem.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readObligationsInput("cem", fifo)
		done <- err
	}()
	select {
	case err := <-done:
		refusal(err, "cannot read cem input")
	case <-time.After(5 * time.Second):
		t.Fatal("readObligationsInput blocked on a writerless FIFO")
	}

	oversized := filepath.Join(dir, "oversized.json")
	if err := os.WriteFile(oversized, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(oversized, obligationsInputCeiling+1); err != nil {
		t.Fatal(err)
	}
	_, err := readObligationsInput("impact", oversized)
	refusal(err, "impact input exceeds")

	atCeiling := filepath.Join(dir, "cem.json")
	if err := os.Truncate(oversized, obligationsInputCeiling); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oversized, atCeiling); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(atCeiling, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{atCeiling, link} {
		data, err := readObligationsInput("cem", path)
		if err != nil || len(data) != obligationsInputCeiling {
			t.Fatalf("%s: read %d bytes, %v", path, len(data), err)
		}
	}
	_, err = readObligationsInput("cem", filepath.Join(dir, "missing.json"))
	refusal(err, "cannot read cem input")
}
