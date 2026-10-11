//go:build darwin || linux

package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// CAL-V0-220: a scan holds a root and a pin per parent directory, independent of
// file count, reads past 511 parents, and releases everything on close.
func TestCALV0220_NativeDescriptorBound(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 513; i++ {
		dir := filepath.Join(base, fmt.Sprint(i))
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "x"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fdPath := "/dev/fd"
	if runtime.GOOS == "linux" {
		fdPath = "/proc/self/fd"
	}
	count := func() int {
		entries, err := os.ReadDir(fdPath)
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	before := count()
	s := newNativeRead(Native{StateDir: base, PrimaryWorktree: base})
	defer s.close()
	for i := 0; i < 513; i++ {
		raw, err := s.Read(fmt.Sprintf("%d/x", i), 1)
		if err != nil || string(raw) != "x" {
			t.Fatalf("read %d: %q %v", i, raw, err)
		}
	}
	// Descriptors are one root and one pin per parent: bounded by directory
	// count, and a revisited parent reuses its pin (CAL-V0-070).
	if len(s.roots) > 514 || len(s.dirs) > 513 {
		t.Fatalf("%d roots, %d pins", len(s.roots), len(s.dirs))
	}
	held := count()
	if raw, err := s.Read("0/x", 1); err != nil || string(raw) != "x" || count() != held {
		t.Fatalf("revisit: %q %v descriptors %d -> %d", raw, err, held, count())
	}
	if got := count() - before; got > 2*513+8 {
		t.Fatalf("retained descriptor count: %d", got)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != before {
		t.Fatalf("leaked descriptors: %d -> %d", before, got)
	}
	if len(s.roots) != 0 || len(s.dirs) != 0 {
		t.Fatal("partial scan retained handles")
	}
}
