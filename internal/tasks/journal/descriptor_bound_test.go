//go:build darwin || linux

package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// CAL-V0-219: a scan keeps at most one duplicate pinned descriptor and reads
// past 511 parents successfully, keeping each root's identity binding.
func TestCALV0219_NativeDescriptorBound(t *testing.T) {
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
		if len(s.dirs) > 1 {
			t.Fatalf("%d read pins", len(s.dirs))
		}
	}
	if got := count() - before; got > 520 {
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
