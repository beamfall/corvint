package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// writerLockHeld probes the writer lock from a second open file description;
// flock locks are per description, so a holder in this process conflicts.
func writerLockHeld(t *testing.T, repo *intent.Repository) bool {
	f, err := os.Open(repo.LockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Errorf("probe open: %v", err)
		return false
	}
	defer f.Close()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true
	}
	if err != nil {
		t.Errorf("probe flock: %v", err)
		return false
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}

// CAL-V0-026 (V1-0845): a real claim with every regular file over the
// descriptor budget reads over-budget files only without the writer lock,
// and the commit sweep runs between its "sweep" and "lock" stages. Reverting
// a locked CheckEvents to Check, or moving the sweep under the lock, makes
// locked reads nonzero; dropping the sweep makes the window empty.
func TestCALV0026_LeaseCommitSweepsOutsideWriterLock(t *testing.T) {
	repo, choice := inventoryClaimRepo(t)
	var mu sync.Mutex
	stage, locked, window := "", 0, 0
	restore := authority.SetVnodeTestHooks(0, func(string) {
		held := writerLockHeld(t, repo)
		mu.Lock()
		defer mu.Unlock()
		if held {
			locked++
		} else if stage == "sweep" {
			window++
		}
	})
	defer restore()
	hooks := hooksForInventory(context.Background())
	hooks.commitStage = func(s string) {
		mu.Lock()
		stage = s
		mu.Unlock()
	}
	ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
	report, err := Lease(ctx, repo, mutation.Binding{ID: "tester", Role: "OWNER"}, choice, fixture.Timestamp)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("claim %+v %v", report, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if stage != "lock" {
		t.Fatalf("commit stages not reached: %q", stage)
	}
	if locked != 0 || window == 0 {
		t.Fatalf("over-budget reads: %d under the writer lock, %d in the commit sweep", locked, window)
	}
	t.Logf("commit sweep read %d over-budget files; 0 under the writer lock", window)
}

// CAL-V0-026 (V1-0845), ATR-V0-7203: an in-place edit of the over-budget
// intent policy is invisible to the change guard once the commit sweep has
// read it, but the content binding under the writer lock re-reads the bound
// decision inputs, so the edit refuses the same claim at either stage and no
// receipt is published. The bound V1-0845 recorded (commit, then refuse at the
// next audit) is superseded; only a write after that final binding remains
// outside the snapshot guarantee.
func TestCALV0026_InPlaceIntentEditAfterCommitSweep(t *testing.T) {
	actor := mutation.Binding{ID: "tester", Role: "OWNER"}
	for _, at := range []string{"sweep", "lock"} {
		t.Run(at, func(t *testing.T) {
			repo, choice := inventoryClaimRepo(t)
			policy := filepath.Join(repo.PrimaryWorktree, intent.Dir, "policy.json")
			original, err := os.ReadFile(policy)
			if err != nil {
				t.Fatal(err)
			}
			restore := authority.SetVnodeTestHooks(0, func(string) {})
			defer restore()
			hooks := hooksForInventory(context.Background())
			edited := false
			hooks.commitStage = func(s string) {
				if s == at && !edited {
					edited = true
					// os.WriteFile truncates the existing inode: no directory event.
					if err := os.WriteFile(policy, append(append([]byte(nil), original...), ' '), 0o644); err != nil {
						t.Error(err)
					}
				}
			}
			ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
			report, err := Lease(ctx, repo, actor, choice, fixture.Timestamp)
			if !edited {
				t.Fatal("edit stage not reached")
			}
			if err == nil || report.Receipt != "" {
				t.Fatalf("edit at the %s stage committed: %+v %v", at, report, err)
			}
			t.Logf("edit at the %s stage refused: %s", at, wire.CodeOf(err))
			next := choice
			next.RequestID = "inventory-claim-next"
			if _, err := Lease(context.Background(), repo, actor, next, fixture.Timestamp); wire.CodeOf(err) != wire.CodeIntentDiverged {
				t.Fatalf("next audit = %v, want INTENT_DIVERGED", err)
			}
		})
	}
}
