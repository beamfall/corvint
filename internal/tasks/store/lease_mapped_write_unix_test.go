//go:build darwin || linux

package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0241_MappedWriteAfterLeaseAuditRefusedByNextWrite is the V1-0775
// probe (ATR-V0-241). A mapped (MAP_SHARED) write raises no change event, and
// a lease write reuses its preparation audit's digests for the files it only
// reads, so a mapped write that lands after that audit, at either commit
// stage, is not refused by that lease write: the claim commits the decision it
// derived from the audited content. That window is the recorded limit. What
// this test holds is the bound on it: the claim's receipt does not absorb the
// mapped content, so the next write's audit refuses the store while the change
// is in place, and accepts it again once the file is restored.
func TestATRV0241_MappedWriteAfterLeaseAuditRefusedByNextWrite(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"sweep", "lock"} {
		for _, tc := range []struct{ target, want string }{
			{"request-projection", wire.CodeJournalForked},
			{"policy", wire.CodeIntentDiverged},
		} {
			t.Run(stage+"/"+tc.target, func(t *testing.T) {
				repo, choice := inventoryClaimRepo(t)
				file := filepath.Join(repo.PrimaryWorktree, intent.Dir, "policy.json")
				if tc.target == "request-projection" {
					p, _ := snapshot.RequestPath("inventory-create")
					file = filepath.Join(repo.StateDir, p)
				}
				before, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				flip := mapShared(t, file)
				hooks := hooksForInventory(context.Background())
				flipped := false
				hooks.commitStage = func(s string) {
					if s == stage && !flipped {
						flip()
						flipped = true
					}
				}
				ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
				report, claimErr := Lease(ctx, repo, mutation.Binding{ID: "tester", Role: "OWNER"}, choice, fixture.Timestamp)
				if !flipped {
					t.Fatalf("stage %s never reached: %+v %v", stage, report, claimErr)
				}
				published := mutationPublished(t, repo)
				_, next := Mutate(context.Background(), repo, historyActor, historyCreate("after-mapped", "after"), fixture.Timestamp)
				flip()
				if after, err := os.ReadFile(file); err != nil || string(after) != string(before) {
					t.Fatalf("mapped file not restored: %v", err)
				}
				t.Logf("claim under the mapped write: %v (receipt %q)", claimErr, report.Receipt)
				if code := wire.CodeOf(next); next == nil || code != tc.want {
					t.Fatalf("next write with the mapped change in place: %v, want %s", next, tc.want)
				}
				if after := mutationPublished(t, repo); after != published {
					t.Fatalf("refused write published: %s, before %s", after, published)
				}
				if rep, err := Mutate(context.Background(), repo, historyActor, historyCreate("after-restore", "restored"), fixture.Timestamp); err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("write after the restore: %+v %v", rep, err)
				}
			})
		}
	}
}
