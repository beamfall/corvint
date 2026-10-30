//go:build darwin

package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

func guardFDCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

// CAL-V0-219: high RLIMIT values must not turn receipt count into open FDs.
func TestCALV0219_ChangeGuardBoundedDescriptors(t *testing.T) {
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

// CAL-V0-219: a store with more than 512 directories is admitted, and closing
// the guard releases every descriptor.
func TestCALV0219_ChangeGuardManyDirectoriesAdmitted(t *testing.T) {
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

// CAL-V0-219: the 16-descriptor file budget is process-wide, not per guard, and
// closing a guard returns its share to the next one.
func TestCALV0219_ChangeGuardBudgetIsProcessWide(t *testing.T) {
	prior := watchedFileDescriptors.Load()
	repo := func() *intent.Repository {
		r, file := guardRepo(t)
		for i := 0; i < 40; i++ {
			if err := os.WriteFile(filepath.Join(filepath.Dir(file), fmt.Sprintf("%02d", i)), []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return r
	}
	open := func(r *intent.Repository) (*ChangeGuard, *vnodeWatch) {
		g, err := WatchChanges(r)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = g.Close() }) // Close is idempotent; covers early Fatal
		return g, g.watch.(*vnodeWatch)
	}
	ra, rb := repo(), repo()
	a, wa := open(ra)
	b, wb := open(rb)
	if total := watchedFileDescriptors.Load(); total > 16 || wa.budgeted+wb.budgeted > 16 || wb.budgeted >= 16 {
		t.Fatalf("budget per guard: a=%d b=%d total=%d", wa.budgeted, wb.budgeted, total)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	c, wc := open(ra)
	if wc.budgeted == 0 || wc.budgeted+wb.budgeted > 16 {
		t.Fatalf("released budget not reused: b=%d c=%d", wb.budgeted, wc.budgeted)
	}
	for _, g := range []*ChangeGuard{b, c} {
		if err := g.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if got := watchedFileDescriptors.Load(); got != prior {
		t.Fatalf("counter leaked: %d -> %d", prior, got)
	}
}
