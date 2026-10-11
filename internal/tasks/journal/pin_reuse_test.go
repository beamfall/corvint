//go:build darwin || linux

package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// CAL-V0-220 (CAL-V0-070 item 3): every read parent keeps exactly one pinned
// descriptor for the audit attempt, and a revisited parent reuses that same
// descriptor rather than retiring and reopening it.
func TestCALV0220_RevisitedParentReusesPin(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const parents = 8
	for i := 0; i < parents; i++ {
		dir := filepath.Join(base, fmt.Sprint(i))
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "x"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := newNativeRead(Native{StateDir: base, PrimaryWorktree: base})
	defer s.close()
	pins := map[string]*os.File{}
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < parents; i++ {
			if raw, err := s.Read(fmt.Sprintf("%d/x", i), 1); err != nil || string(raw) != "x" {
				t.Fatalf("pass %d read %d: %q %v", pass, i, raw, err)
			}
		}
		if len(s.dirs) != parents {
			t.Fatalf("pass %d: %d pins, want one per parent (%d)", pass, len(s.dirs), parents)
		}
		for dir, d := range s.dirs {
			if pass == 0 {
				pins[dir] = d
			} else if pins[dir] != d {
				t.Fatalf("parent %s re-pinned on revisit", dir)
			}
		}
	}
}
