// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

package corpusbridge

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// DCR-V1-8000: the optimized reader must describe the same no-follow metadata
// as DirEntry.Info, including special modes and links, without per-entry heap
// allocation. The budget fails with the original e.Info reader.
func TestDCRV18000StampEntryReader(t *testing.T) {
	t.Run("DCR-V1-8000 metadata", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "file"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, "dir"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("file", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(dir, "fifo"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(dir, "file"), 0751|fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		read, release, err := stampEntryReader(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		s := stamper{horizon: time.Now().Add(time.Hour).UnixNano()}
		for _, entry := range entries {
			want, err := entry.Info()
			if err != nil {
				t.Fatal(err)
			}
			got, err := read(entry)
			if err != nil {
				t.Fatal(err)
			}
			a, ac, ae := s.line(nil, entry.Name(), want)
			b, bc, be := s.line(nil, entry.Name(), got)
			if !bytes.Equal(a, b) || ac != bc || ae != be || got.IsDir() != want.IsDir() || got.Name() != want.Name() || !got.ModTime().Equal(want.ModTime()) {
				t.Fatalf("%s: stat record %q != %q", entry.Name(), b, a)
			}
		}
		if allocs := testing.AllocsPerRun(10, func() {
			for range 100 {
				if _, err := read(entries[0]); err != nil {
					t.Fatal(err)
				}
			}
		}); allocs > 1 {
			t.Fatalf("per-entry stat allocations: %v per 100 stats", allocs)
		}
		// A vanished entry fails instead of reusing the previous metadata buffer.
		if err := os.Remove(filepath.Join(dir, entries[0].Name())); err != nil {
			t.Fatal(err)
		}
		if _, err := read(entries[0]); err == nil {
			t.Fatal("removed entry described")
		}
	})
}
