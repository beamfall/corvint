//go:build darwin || linux

package journal

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// CAL-V0-212: a scan keeps at most one duplicate pinned descriptor and refuses
// excess roots, rather than evicting identity bindings and admitting replacements.
func TestCALV0212_NativeDescriptorBound(t *testing.T) {
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
	for i := 0; i < 511; i++ {
		raw, err := s.Read(fmt.Sprintf("%d/x", i), 1)
		if err != nil || string(raw) != "x" {
			t.Fatalf("read %d: %q %v", i, raw, err)
		}
		if len(s.dirs) > 1 || len(s.roots) > 512 {
			t.Fatalf("descriptors: %d roots %d pins", len(s.roots), len(s.dirs))
		}
	}
	if got := count() - before; got > 513 {
		t.Fatalf("retained descriptor count: %d", got)
	}
	if _, err := s.Read("511/x", 1); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("root admission: %v", err)
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
