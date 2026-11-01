package testsupport

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeTB records a skip or fatal and, like testing.T, ends its goroutine.
type fakeTB struct {
	testing.TB
	skipped, failed string
}

func (f *fakeTB) Helper() {}
func (f *fakeTB) Skipf(format string, args ...any) {
	f.skipped = fmt.Sprintf(format, args...)
	runtime.Goexit()
}
func (f *fakeTB) Fatalf(format string, args ...any) {
	f.failed = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// nodeOrSkip runs NodeOrSkip on its own goroutine, as Skipf and Fatalf end it.
func nodeOrSkip(f *fakeTB) (got string) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		got = NodeOrSkip(f, "the probe")
	}()
	<-done
	return got
}

// ASD-V0-973 (V1-3801): with node absent the helper skips naming the tool, and
// under CI it fails instead; with node present it returns that executable.
func TestASDV0973NodeOrSkip(t *testing.T) {
	empty := t.TempDir()
	t.Run("absent skips", func(t *testing.T) {
		t.Setenv("PATH", empty)
		t.Setenv("CI", "")
		f := &fakeTB{TB: t}
		if got := nodeOrSkip(f); got != "" || f.failed != "" || !strings.Contains(f.skipped, "NOT_RUN: the probe needs node, which is not on PATH") {
			t.Fatalf("got %q skipped=%q failed=%q", got, f.skipped, f.failed)
		}
	})
	t.Run("absent under CI fails", func(t *testing.T) {
		t.Setenv("PATH", empty)
		t.Setenv("CI", "true")
		f := &fakeTB{TB: t}
		nodeOrSkip(f)
		if f.skipped != "" || !strings.Contains(f.failed, "the probe needs node, which CI provisions") {
			t.Fatalf("skipped=%q failed=%q", f.skipped, f.failed)
		}
	})
	t.Run("present returns it", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a PATH executable needs an extension on Windows")
		}
		dir := t.TempDir()
		node := filepath.Join(dir, "node")
		if err := os.WriteFile(node, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		f := &fakeTB{TB: t}
		if got := nodeOrSkip(f); got != node || f.skipped != "" || f.failed != "" {
			t.Fatalf("got %q skipped=%q failed=%q", got, f.skipped, f.failed)
		}
	})
}
