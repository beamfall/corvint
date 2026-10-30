//go:build darwin

package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	changed, err := w.poll()
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
					if changed := w.sweep(); !changed {
						t.Fatalf("stat tuple missed %s", name)
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

// countSweepLstat counts the stat reads of over-budget files.
func countSweepLstat(t *testing.T) *int {
	t.Helper()
	original := sweepLstat
	count := new(int)
	sweepLstat = func(path string) (os.FileInfo, error) {
		*count++
		return original(path)
	}
	t.Cleanup(func() { sweepLstat = original })
	return count
}

// CAL-V0-026 (V1-0845): with the store beyond the descriptor budget, the check
// a lease commit makes under the writer lock reads no over-budget file, at any
// store size; the sweep before the lock and the unlocked Check read them all.
func TestCALV0026_LockedCheckIndependentOfOverBudgetFiles(t *testing.T) {
	const budget = 2
	for _, files := range []int{8, 256} {
		t.Run(fmt.Sprint(files), func(t *testing.T) {
			setVnodeFileBudget(t, budget)
			repo, file := guardRepo(t)
			for i := 0; i < files; i++ {
				if err := os.WriteFile(filepath.Join(filepath.Dir(file), fmt.Sprintf("%03d", i)), []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			over := files + 1 - budget
			if got := len(g.watch.(*vnodeWatch).stats); got != over {
				t.Fatalf("over-budget files = %d, want %d", got, over)
			}
			reads := countSweepLstat(t)
			g.Sweep()
			if *reads != over {
				t.Fatalf("sweep read %d files, want %d", *reads, over)
			}
			*reads = 0
			for i := 0; i < 2; i++ {
				if err := g.CheckEvents(); err != nil {
					t.Fatal(err)
				}
			}
			if *reads != 0 {
				t.Fatalf("locked check read %d over-budget files", *reads)
			}
			if err := g.Check(); err != nil || *reads != over {
				t.Fatalf("unlocked check: %v after %d reads, want %d", err, *reads, over)
			}
		})
	}
}

// CAL-V0-026 (V1-0845): a change the sweep saw stays reported, and a store
// writer's entry change made after the sweep is reported by the directory
// event, so no change is lost between the unlocked sweep and the locked check.
// An in-place write after the sweep is the recorded bound: only a later Check
// reports it.
func TestCALV0026_SweepThenLockedCheckLosesNoChange(t *testing.T) {
	for _, name := range []string{"before-sweep-in-place", "before-sweep-chmod", "after-sweep-rename-replace", "after-sweep-link", "after-sweep-remove", "after-sweep-sibling", "after-sweep-in-place"} {
		t.Run(name, func(t *testing.T) {
			setVnodeFileBudget(t, 0)
			repo, file := guardRepo(t)
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			if len(g.watch.(*vnodeWatch).stats) != 1 {
				t.Fatal("x.json not over budget")
			}
			// The rename and link stages sit outside the watched trees on the
			// same filesystem, so the one entry operation is the only event.
			stage := filepath.Join(t.TempDir(), "stage")
			if err := os.WriteFile(stage, []byte("cccc"), 0600); err != nil {
				t.Fatal(err)
			}
			change := func() error {
				switch name {
				case "before-sweep-in-place", "after-sweep-in-place":
					return os.WriteFile(file, []byte("bbbb"), 0600)
				case "before-sweep-chmod":
					return os.Chmod(file, 0640)
				case "after-sweep-rename-replace":
					return os.Rename(stage, file)
				case "after-sweep-link":
					return os.Link(stage, filepath.Join(filepath.Dir(file), "y.json"))
				case "after-sweep-remove":
					return os.Remove(file)
				default:
					return os.WriteFile(file+".new", []byte("new"), 0600)
				}
			}
			before := strings.HasPrefix(name, "before-sweep")
			if before {
				if err := change(); err != nil {
					t.Fatal(err)
				}
			}
			g.Sweep()
			if !before {
				if err := g.CheckEvents(); err != nil {
					t.Fatalf("clean store reported before the change: %v", err)
				}
				if err := change(); err != nil {
					t.Fatal(err)
				}
			}
			reads := countSweepLstat(t)
			got := wire.CodeOf(g.CheckEvents())
			if *reads != 0 {
				t.Fatalf("locked check read %d over-budget files", *reads)
			}
			if name == "after-sweep-in-place" {
				if got != "" {
					t.Fatalf("in-place write after the sweep: %q", got)
				}
				if wire.CodeOf(g.Check()) != wire.CodeSnapshotMoved {
					t.Fatal("unlocked check missed an in-place write")
				}
				return
			}
			if got != wire.CodeSnapshotMoved {
				t.Fatalf("locked check = %q, want SNAPSHOT_MOVED", got)
			}
		})
	}
}
