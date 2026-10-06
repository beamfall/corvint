package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
)

// copyTree copies the regular files and directories under src to dst.
func copyTree(tb testing.TB, src, dst string) {
	tb.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode().Perm())
	})
	if err != nil {
		tb.Fatal(err)
	}
}

// storeBytes is every persisted byte a Mutate can write: the state dir, the
// intent tree and the retained checkpoint. Lock files carry no state.
func storeBytes(tb testing.TB, repo *intent.Repository) map[string]string {
	tb.Helper()
	out := map[string]string{}
	for _, root := range []string{repo.StateDir, filepath.Join(repo.PrimaryWorktree, intent.Dir)} {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			raw, err := os.ReadFile(p)
			out[p] = string(raw)
			return err
		})
		if err != nil {
			tb.Fatal(err)
		}
	}
	if raw, err := os.ReadFile(repo.StateDir + ".checkpoint.json"); err == nil {
		out["checkpoint"] = string(raw)
	}
	return out
}

type pinnedStep struct {
	Stages []string
	Report *Report
	Err    string
}

// CAL-V0-070: Mutate's watched inventory read through pinned parent roots
// leaves byte-identical receipts, projections, head, checkpoint and reports
// to the ordinary per-file walk over the same sequence of completed, replayed
// and refused requests, and refuses outside changes at the same stage with
// the same error. The ordinary run is replayed on a byte copy of the store
// restored at the same path, since a store is bound to its primary worktree.
func TestCALV0070_PinnedInventoryMutateParity(t *testing.T) {
	repo := historyStore(t, 70)
	aside := filepath.Join(t.TempDir(), "aside")
	copyTree(t, repo.PrimaryWorktree, aside)
	now := WallClock() // later than every fixture receipt; the same for both runs
	_, ticketFile := mutationBoundaryFiles(t, repo)
	run := func(pinned bool) ([]pinnedStep, map[string]string) {
		old := pinnedMutationInventory
		pinnedMutationInventory = pinned
		defer func() { pinnedMutationInventory = old }()
		if err := os.RemoveAll(repo.PrimaryWorktree); err != nil {
			t.Fatal(err)
		}
		copyTree(t, aside, repo.PrimaryWorktree)
		var steps []pinnedStep
		mutate := func(id, title string, edit func() func()) {
			if edit != nil {
				defer edit()()
			}
			var step pinnedStep
			ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) {
				step.Stages = append(step.Stages, stage)
			})
			rep, err := Mutate(ctx, repo, historyActor, historyCreate(id, title), now)
			step.Report = rep
			if err != nil {
				step.Err = err.Error()
			}
			steps = append(steps, step)
		}
		mutate("pin-1", "pinned one", nil)
		mutate("pin-2", "pinned two", nil)
		mutate("pin-1", "pinned one", nil)              // replay
		mutate("pin-1", "pinned changed", nil)          // same id, different request
		mutate("pin-3", "pinned three", func() func() { // edit outside the journal
			raw, err := os.ReadFile(ticketFile)
			if err != nil {
				t.Fatal(err)
			}
			historyWrite(t, ticketFile, append(append([]byte{}, raw...), '\n'))
			return func() { historyWrite(t, ticketFile, raw) }
		})
		mutate("pin-4", "pinned four", func() func() { // stray state directory
			p := filepath.Join(repo.StateDir, "receipts", "nested")
			if err := os.Mkdir(p, 0o755); err != nil {
				t.Fatal(err)
			}
			return func() { os.Remove(p) }
		})
		mutate("pin-5", "pinned five", nil)
		return steps, storeBytes(t, repo)
	}
	ordinarySteps, ordinaryBytes := run(false)
	pinnedSteps, pinnedBytes := run(true)
	if !reflect.DeepEqual(pinnedSteps, ordinarySteps) {
		for i := range ordinarySteps {
			if i < len(pinnedSteps) && !reflect.DeepEqual(pinnedSteps[i], ordinarySteps[i]) {
				t.Errorf("step %d:\n pinned   %+v\n ordinary %+v", i, pinnedSteps[i], ordinarySteps[i])
			}
		}
		t.Fatal("steps differ")
	}
	if !reflect.DeepEqual(pinnedBytes, ordinaryBytes) {
		for p, raw := range ordinaryBytes {
			if pinnedBytes[p] != raw {
				t.Errorf("bytes differ: %s", p)
			}
		}
		t.Fatalf("store bytes differ (%d / %d files)", len(pinnedBytes), len(ordinaryBytes))
	}
	completed, refused := 0, 0
	for i, s := range pinnedSteps {
		if s.Err != "" || s.Report == nil || s.Report.Outcome.Outcome != mutation.OutcomeCompleted {
			refused++
		} else {
			completed++
		}
		outcome := ""
		if s.Report != nil {
			outcome = fmt.Sprint(s.Report.Outcome.Outcome, s.Report.Outcome.Codes)
		}
		t.Logf("step %d: stages %v outcome %s err %q", i, s.Stages, outcome, s.Err)
	}
	if completed < 3 || refused < 2 {
		t.Fatalf("sequence lost its cases: %d completed, %d refused", completed, refused)
	}
	if strings.Join(pinnedSteps[0].Stages, ",") != "watched,audited,observed" {
		t.Fatalf("clean create did not reuse the merged audit: %v", pinnedSteps[0].Stages)
	}
}

// CAL-V0-070: a pinned watched inventory that fails, here at a parent open
// or at a parent-root retirement, is never used: Mutate closes the watch and
// takes the ordinary fresh inventory and audit, and completes as before.
func TestCALV0070_PinnedInventoryFailureFallsBackFresh(t *testing.T) {
	repo := historyStore(t, 70)
	for _, kind := range []string{"open", "close"} {
		t.Run(kind, func(t *testing.T) {
			hooks := inventoryHooks{}
			failed := 0
			switch kind {
			case "open":
				hooks.open = func(string) (*os.Root, error) { failed++; return nil, errors.New("injected open") }
			case "close":
				hooks.close = func(r *os.Root) error { r.Close(); failed++; return errors.New("injected close") }
			}
			var stages []string
			ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
			ctx = context.WithValue(ctx, mutationStageKey{}, func(stage string) { stages = append(stages, stage) })
			rep, err := Mutate(ctx, repo, historyActor, historyCreate("fallback-"+kind, "fallback "+kind), WallClock())
			if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted || failed == 0 {
				t.Fatalf("Mutate: %v %+v (injected %d)", err, rep, failed)
			}
			if want := []string{"watched", "audited", "observed", "fresh"}; !reflect.DeepEqual(stages, want) {
				t.Fatalf("stages %v, want %v", stages, want)
			}
		})
	}
}

// BenchmarkCALV0070_WatchedInventory is observeMutation's inventory on a
// 2,000-receipt store: the ordinary no-follow walk from "/" per file against
// pinned parent roots.
func BenchmarkCALV0070_WatchedInventory(b *testing.B) {
	repo := historyStore(b, 2000)
	for _, pinned := range []bool{false, true} {
		b.Run(map[bool]string{false: "ordinary", true: "pinned"}[pinned], func(b *testing.B) {
			old := pinnedMutationInventory
			pinnedMutationInventory = pinned
			defer func() { pinnedMutationInventory = old }()
			for i := 0; i < b.N; i++ {
				if _, err := pinnedInventory(context.Background(), repo); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
