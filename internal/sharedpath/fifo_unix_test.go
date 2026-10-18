//go:build darwin || linux

package sharedpath

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Codex finding 11 on #1059: a FIFO configuration or notes file is refused
// without blocking on open.
func TestSPCV0001FIFOInputsRefusedWithoutBlocking(t *testing.T) {
	f := newFixture(t)
	base := f.commit("base", map[string]string{"README": "r\n"})
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan [2]error, 1)
	go func() {
		_, configErr := LoadConfig(fifo)
		_, notesErr := Check(context.Background(), CheckRequest{Root: f.root, Base: base, NotesFile: fifo, Config: testConfig()})
		done <- [2]error{configErr, notesErr}
	}()
	select {
	case errs := <-done:
		if errs[0] == nil || errs[0].Error() != CodeConfigUnavailable || errs[1] == nil || errs[1].Error() != CodeNotesUnavailable {
			t.Fatalf("FIFO inputs: %v", errs)
		}
	case <-time.After(2 * time.Minute):
		t.Fatal("a FIFO input blocked")
	}
}
