// SPDX-License-Identifier: Apache-2.0

//go:build linux && amd64

package corpusbridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

// DCR-V1-7802: relative stat must retain the lstat oracle's complete records,
// including symlinks, directories and Unix permission bits. It must not allocate
// per file, and a vanished entry must abstain rather than disappear from a stamp.
func TestDCRV17802RelativeStatMatchesLstat(t *testing.T) {
	dir := t.TempDir()
	for _, mode := range []os.FileMode{0600, 0755, os.ModeSetuid | 0755, os.ModeSetgid | 0755, os.ModeSticky | 0777} {
		name := filepath.Join(dir, mode.String())
		if err := os.WriteFile(name, []byte("body"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(name, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := &stamper{ctx: context.Background(), horizon: 1<<63 - 1}
	want := s.describe(entries)
	got := s.describeAt(f, entries)
	if want.err != nil || got.err != nil || !bytes.Equal(got.b, want.b) || got.coarse != want.coarse || !reflect.DeepEqual(got.dirs, want.dirs) {
		t.Fatalf("relative stat differs from lstat: got %+v want %+v", got, want)
	}
	allocs := testing.AllocsPerRun(10, func() {
		if c := s.describeAt(f, entries); c.err != nil {
			t.Fatal(c.err)
		}
	})
	if allocs > 4 {
		t.Fatalf("relative stats allocated %.0f times, want at most four per chunk", allocs)
	}
	if err := os.Remove(filepath.Join(dir, entries[0].Name())); err != nil {
		t.Fatal(err)
	}
	if c := s.describeAt(f, entries); c.err == nil {
		t.Fatal("stamped a vanished entry")
	}
}
