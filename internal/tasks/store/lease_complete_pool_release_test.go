//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LGL-V0-2150, V1-2621: complete-route writes recover a sealed base even
// without a usable old checkpoint. The following pool release resumes once,
// without a complete audit, and matches the complete route's report and bytes.
func TestLGLV02150_CompleteRouteWriteThenPoolRelease(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, route := range []string{"widen", "reap", "mutate"} {
		for _, checkpoint := range []string{"missing", "invalidated"} {
			t.Run(route+"/"+checkpoint, func(t *testing.T) {
				t.Parallel()
				f := newSealedBaseFixture(t)
				// The production derivation must run; the moving-journal seed
				// fixture deliberately keeps the old plain derivation enabled.
				plainCheckpointStores.Delete(f.repo.StateDir)
				var reap transaction.LeaseRequest
				if route == "reap" {
					created := stagedWrite(writerCreate(f.repo, "short-ticket", f.now), nil)
					if !created.completed() {
						t.Fatalf("create: %+v %v", created.rep, created.err)
					}
					attemptEntropies.Store(f.repo.StateDir, fixedEntropy(33))
					claimed := sealedRun(t, f, f.lease("short-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: "agent-2", LeaseMinutes: "5", Scope: []string{"other/"}}), nil)
					if !claimed.completed() {
						t.Fatalf("claim: %+v %v", claimed.rep, claimed.err)
					}
					reap = transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: claimed.rep.AttemptID, Generation: claimed.rep.Generation}
				}
				acquired := sealedRun(t, f, f.acquire("interleave-acquire"), nil)
				if !acquired.completed() || acquired.rep.PoolAllocation == nil {
					t.Fatalf("acquire: %+v %v", acquired.rep, acquired.err)
				}
				through := segmentThrough(t, f.repo)
				wcPath := journal.WriterCheckpointPath(f.repo.StateDir)
				if checkpoint == "invalidated" {
					invalidateWriterCheckpoint(context.Background(), f.repo)
				} else {
					for _, p := range []string{wcPath, journal.WriterBasePath(f.repo.StateDir)} {
						if err := os.Remove(p); err != nil {
							t.Fatal(err)
						}
					}
				}
				if wc, _ := boundWriterCheckpoint(f.repo); wc != nil {
					t.Fatal("checkpoint still bound before the complete-route write")
				}
				var write writerRun
				switch route {
				case "mutate":
					write = writerMutate(t, f.repo, "interleave-mutate", nil)
					f.now = WallClock()
				case "widen":
					write = sealedRun(t, f, f.lease("interleave-widen", transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation, Scope: []string{"docs/"}}), nil).writerRun
				case "reap":
					at, err := time.Parse(time.RFC3339, string(f.now))
					if err != nil {
						t.Fatal(err)
					}
					// Expire only the short claim, keeping the allocation's
					// sixty-minute attempt live for the following release.
					f.now = wire.Timestamp(at.Add(6 * time.Minute).UTC().Format(time.RFC3339))
					write = sealedRun(t, f, f.lease("interleave-reap", reap), nil).writerRun
				}
				if !write.completed() || write.fast() {
					t.Fatalf("complete-route %s: %+v %v stages %v", route, write.rep, write.err, write.stages)
				}
				if wc, _ := boundWriterCheckpoint(f.repo); wc == nil || wc.FullSeq != through {
					t.Fatalf("complete-route %s retained %+v, want FullSeq %d", route, wc, through)
				}
				parent := filepath.Dir(f.repo.PrimaryWorktree)
				pre := filepath.Join(t.TempDir(), "pre")
				copyTree(t, parent, pre)
				allocation := string(acquired.rep.PoolAllocation.AllocationID)
				released := sealedRun(t, f, f.release("interleave-release", allocation), nil)
				if !released.completed() || released.timing.Resumed != 1 || released.audits != 0 || !released.fast() {
					t.Fatalf("release: %+v %v resumed %d audits %d stages %v", released.rep, released.err, released.timing.Resumed, released.audits, released.stages)
				}
				resumedFiles := storeFiles(t, parent, wcPath)
				if err := os.RemoveAll(parent); err != nil {
					t.Fatal(err)
				}
				copyTree(t, pre, parent)
				if err := os.Remove(wcPath); err != nil {
					t.Fatal(err)
				}
				oracle := sealedRun(t, f, f.release("interleave-release", allocation), nil)
				if !oracle.completed() || oracle.fast() || oracle.audits == 0 {
					t.Fatalf("complete release: %+v %v audits %d stages %v", oracle.rep, oracle.err, oracle.audits, oracle.stages)
				}
				if !reflect.DeepEqual(*released.rep, *oracle.rep) {
					t.Fatalf("reports differ:\nresumed %+v\ncomplete %+v", *released.rep, *oracle.rep)
				}
				if completeFiles := storeFiles(t, parent, wcPath); !reflect.DeepEqual(resumedFiles, completeFiles) {
					t.Fatalf("store files differ:\nresumed %v\ncomplete %v", resumedFiles, completeFiles)
				}
				mustSegmentAudit(t, f.repo, headSeqOf(t, f.repo))
			})
		}
	}
}
