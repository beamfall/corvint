//go:build darwin

package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-026: partial kqueue registration releases descriptors on exhaustion.
// The file budget is lifted above the limit so registration itself exhausts it.
func TestCALV0026_ChangeGuardDescriptorExhaustion(t *testing.T) {
	setVnodeFileBudget(t, 1<<30)
	repo, file := guardRepo(t)
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(filepath.Dir(file), fmt.Sprintf("%03d", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
		t.Fatal(err)
	}
	defer syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original)
	before, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		limited := original
		limited.Cur = min(original.Cur, 64)
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limited); err != nil {
			t.Fatal(err)
		}
		g, watchErr := WatchChanges(repo)
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
			t.Fatal(err)
		}
		if watchErr == nil {
			g.Close()
			t.Fatal("descriptor exhaustion did not refuse")
		}
	}
	after, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) > len(before) {
		t.Fatalf("descriptors leaked: before %d after %d", len(before), len(after))
	}
}

func TestCALV0026_RegistrationSeesMembershipChange(t *testing.T) {
	repo, file := guardRepo(t)
	w, err := newChangeWatch()
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	if _, err := w.add(filepath.Dir(file), true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file+".new", []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.add(repo.StateDir, true); err != nil {
		t.Fatal(err)
	}
	changed, err := w.changed()
	if err != nil || !changed {
		t.Fatalf("registration gap accepted: %v %v", changed, err)
	}
}

func setVnodeFileBudget(t *testing.T, budget int64) {
	t.Helper()
	original := vnodeFileBudget
	vnodeFileBudget = func() int64 { return budget }
	t.Cleanup(func() { vnodeFileBudget = original })
}

// CAL-V0-026: store size alone never exhausts descriptors. With the derived
// budget a store larger than the soft limit is watched, not refused.
func TestCALV0026_ChangeGuardDerivedBudgetFitsLimit(t *testing.T) {
	repo, file := guardRepo(t)
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(filepath.Dir(file), fmt.Sprintf("%03d", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
		t.Fatal(err)
	}
	defer syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original)
	limited := original
	limited.Cur = min(original.Cur, 64)
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limited); err != nil {
		t.Fatal(err)
	}
	g, err := WatchChanges(repo)
	if restore := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original); restore != nil {
		t.Fatal(restore)
	}
	if err != nil {
		t.Fatalf("store size refused: %v", err)
	}
	defer g.Close()
	if err := os.WriteFile(filepath.Join(filepath.Dir(file), "099"), []byte("y"), 0600); err != nil {
		t.Fatal(err)
	}
	if wire.CodeOf(g.Check()) != wire.CodeSnapshotMoved {
		t.Fatal("write to an over-budget file admitted")
	}
}

// CAL-V0-026: a file beyond the descriptor budget is tracked by its stat
// tuple; every mutation class is reported, quiescence is not, and Close
// returns the budget.
func TestCALV0026_ChangeGuardOverBudgetFiles(t *testing.T) {
	for _, budget := range []int64{0, 2} {
		for _, name := range []string{"none", "write-same-size", "write-resize", "in-place-restored-time", "chmod", "rename-replace", "remove", "sibling"} {
			t.Run(fmt.Sprintf("%d/%s", budget, name), func(t *testing.T) {
				setVnodeFileBudget(t, budget)
				repo, file := guardRepo(t)
				dir := filepath.Dir(file)
				for i := 0; i < 4; i++ {
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%03d", i)), []byte("x"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				// x.json sorts after 000..003, so it is over budget in both cases.
				prior := watchedFileDescriptors.Load()
				g, err := WatchChanges(repo)
				if err != nil {
					t.Fatal(err)
				}
				w := g.watch.(*vnodeWatch)
				if int64(len(w.stats)) != 5-budget || w.budgeted != budget || watchedFileDescriptors.Load() != prior+budget {
					t.Fatalf("budget not applied: stats %d budgeted %d counter %d", len(w.stats), w.budgeted, watchedFileDescriptors.Load()-prior)
				}
				if w.stats[len(w.stats)-1].path != file {
					t.Fatalf("x.json not over budget: %s", w.stats[len(w.stats)-1].path)
				}
				switch name {
				case "write-same-size":
					err = os.WriteFile(file, []byte("bbbb"), 0600)
				case "write-resize":
					err = os.WriteFile(file, []byte("bbbbbb"), 0600)
				case "in-place-restored-time":
					st, _ := os.Stat(file)
					err = os.WriteFile(file, []byte("cccc"), 0600)
					if err == nil {
						err = os.Chtimes(file, st.ModTime(), st.ModTime())
					}
				case "chmod":
					err = os.Chmod(file, 0640)
				case "rename-replace":
					err = os.WriteFile(file+".tmp", []byte("aaaa"), 0600)
					if err == nil {
						err = os.Rename(file+".tmp", file)
					}
				case "remove":
					err = os.Remove(file)
				case "sibling":
					err = os.WriteFile(file+".new", []byte("new"), 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
				want := wire.CodeSnapshotMoved
				if name == "none" {
					want = ""
				}
				if got := wire.CodeOf(g.Check()); got != want {
					t.Fatalf("check = %q, want %q", got, want)
				}
				if name != "sibling" && name != "none" {
					// The stat tuple alone must see it, without the directory event.
					w.dirty = false
					for {
						var events [8]syscall.Kevent_t
						n, err := syscall.Kevent(w.queue, nil, events[:], &syscall.Timespec{})
						if err != nil || n == 0 {
							break
						}
					}
					if changed, err := w.changed(); err != nil || !changed {
						t.Fatalf("stat tuple missed %s: %v %v", name, changed, err)
					}
				}
				if err := g.Close(); err != nil {
					t.Fatal(err)
				}
				if got := watchedFileDescriptors.Load(); got != prior {
					t.Fatalf("budget not released: %d, want %d", got, prior)
				}
			})
		}
	}
}
