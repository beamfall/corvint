//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/localcompletion"
)

// TestDogfoodHandoffReceiptUnavailable pins SESSION-V0-019: a receipt is read
// only as a bounded regular file, so a symlink, a FIFO, an oversized or a
// missing file is refused before any decoding.
func TestDogfoodHandoffReceiptUnavailable(t *testing.T) {
	t.Parallel()
	root, key := handoffRepository(t)
	file, _ := emitHandoff(t, root, key)
	directory := t.TempDir()
	symlink := filepath.Join(directory, "symlink.json")
	if err := os.Symlink(file, symlink); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(directory, "fifo.json")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(directory, "oversized.json")
	if err := os.WriteFile(oversized, make([]byte, localcompletion.MaxPlanBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"symlink": symlink, "fifo": fifo, "oversized": oversized, "missing": filepath.Join(directory, "missing.json")} {
		if code, _, stderr := consumeHandoff(t, root, key, path); code != 2 || !strings.Contains(stderr, "handoff-receipt-unavailable") {
			t.Errorf("%s: %d %s", name, code, stderr)
		}
	}
}
