//go:build darwin || linux

package dogfoodflow

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// TestPrepareUnknownReasonsDCWV0046SkipsANonRegularFile: a FIFO at the notes
// path is never opened, so it cannot block the workflow, and publishing
// replaces it with an owner-only regular file.
func TestPrepareUnknownReasonsDCWV0046SkipsANonRegularFile(t *testing.T) {
	root := t.TempDir()
	path := root + unknownReasons
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	c := &change{flow: flow{root: root}, evidence: root}
	done := make(chan error, 1)
	go func() { done <- c.prepareUnknownReasons(nil) }()
	select {
	case err := <-done:
		if err != nil || c.unknownNotes != nil {
			t.Fatalf("prepare over a FIFO = %q, %v", c.unknownNotes, err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("prepare blocked on a FIFO at the notes path")
	}
	c.unknownNotes = []byte("hunk:a\tinsufficient-evidence\tgenerated\n")
	if err := c.publishUnknownReasons(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Errorf("published notes = %v, %v; want an owner-only regular file", info, err)
	}
}
