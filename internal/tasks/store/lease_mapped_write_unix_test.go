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

// ATR-V0-7203: mmap changes at either commit boundary refuse the same lease.
func TestATRV07203_MappedWriteAfterLeaseAuditRefusesCommit(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"sweep", "lock"} {
		for _, tc := range []struct{ target, want string }{
			{"request-projection", wire.CodeJournalForked},
			{"policy", wire.CodeIntentDiverged},
			{"ticket", wire.CodeIntentDiverged},
		} {
			t.Run(stage+"/"+tc.target, func(t *testing.T) {
				repo, choice := inventoryClaimRepo(t)
				file := filepath.Join(repo.PrimaryWorktree, intent.Dir, "policy.json")
				if tc.target == "ticket" {
					id, e := wire.ParseTicketID("ticket", choice.Lease.TicketID)
					if e != nil {
						t.Fatal(e)
					}
					file = filepath.Join(repo.IntentRoot(), intent.Dir, "tickets", id.Local+".json")
				}
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
				if claimErr == nil || report.Receipt != "" {
					t.Fatalf("mapped write committed: %+v %v", report, claimErr)
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
