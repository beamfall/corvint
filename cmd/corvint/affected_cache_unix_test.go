//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// AFP-V0-395: a --paths-from file replaced by a FIFO just before it is opened
// refuses with invalid-arguments instead of blocking on the open. Tests that
// replace the hook do not run in parallel.
func TestAFPV0395PathsFromFIFOCannotBlock(t *testing.T) {
	root := affectedFixtureRepository(t)
	list := filepath.Join(t.TempDir(), "paths.txt")
	if err := os.WriteFile(list, []byte("core/core.go\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	saved := affectedPathsFromBeforeOpen
	t.Cleanup(func() { affectedPathsFromBeforeOpen = saved })
	affectedPathsFromBeforeOpen = func(name string) {
		if err := os.Remove(name); err != nil {
			t.Error(err)
		}
		if err := syscall.Mkfifo(name, 0o600); err != nil {
			t.Error(err)
		}
	}
	type result struct {
		stdout []byte
		stderr string
		code   int
	}
	done := make(chan result, 1)
	go func() {
		_, stdout, stderr, code := runAffectedCLI(t, root, "--paths-from", list)
		done <- result{stdout, stderr, code}
	}()
	select {
	case got := <-done:
		if got.code != 2 || len(got.stdout) != 0 || !strings.Contains(got.stderr, `"invalid-arguments"`) {
			t.Fatalf("a FIFO --paths-from must refuse with invalid-arguments, got exit %d: %s", got.code, got.stderr)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("opening a FIFO --paths-from blocked the call")
	}
}
