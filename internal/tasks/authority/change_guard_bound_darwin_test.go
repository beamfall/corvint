//go:build darwin

package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func guardFDCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

// CAL-V0-218: high RLIMIT values must not turn receipt count into open FDs.
func TestCALV0218_ChangeGuardBoundedDescriptors(t *testing.T) {
	for _, n := range []int{100, 3000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			repo, file := guardRepo(t)
			for i := 0; i < n; i++ {
				if err := os.WriteFile(filepath.Join(filepath.Dir(file), fmt.Sprintf("%06d", i)), []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := guardFDCount(t)
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			w := g.watch.(*vnodeWatch)
			if w.budgeted > 16 || guardFDCount(t)-before > 64 {
				t.Fatalf("unbounded watch: files=%d descriptors=%d", w.budgeted, guardFDCount(t)-before)
			}
			// Last file is outside the fixed budget; detection remains active.
			if err := os.WriteFile(file, []byte("bbbb"), 0600); err != nil {
				t.Fatal(err)
			}
			if g.Check() == nil {
				t.Fatal("uncovered file change admitted")
			}
			if err := g.Close(); err != nil {
				t.Fatal(err)
			}
			if got := guardFDCount(t); got != before {
				t.Fatalf("leaked: %d -> %d", before, got)
			}
		})
	}
}

// CAL-V0-218: a store with more than 512 directories is admitted, and closing
// the guard releases every descriptor.
func TestCALV0218_ChangeGuardManyDirectoriesAdmitted(t *testing.T) {
	repo, _ := guardRepo(t)
	for i := 0; i < 513; i++ {
		if err := os.Mkdir(filepath.Join(repo.StateDir, fmt.Sprint(i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	before := guardFDCount(t)
	g, err := WatchChanges(repo)
	if err != nil {
		t.Fatalf("directory-heavy store refused: %v", err)
	}
	if err := os.Mkdir(filepath.Join(repo.StateDir, "new"), 0700); err != nil {
		t.Fatal(err)
	}
	if g.Check() == nil {
		t.Fatal("directory change admitted")
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	if got := guardFDCount(t); got != before {
		t.Fatalf("leaked: %d -> %d", before, got)
	}
}
