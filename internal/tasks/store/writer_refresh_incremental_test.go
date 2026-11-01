//go:build darwin || linux

package store

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// PSR-V0-891 (V1-0916): over randomized write sequences (ticket creates,
// heartbeats and renews of a live claim, in a random order and number, so
// the writer route both carries its tail in the delta and re-bases it), the
// incremental refresh's checkpoint is byte-identical, base and delta, to the
// complete refresh's at the same head.
func TestPSRV0891_IncrementalRefreshMatchesTheCompleteRefresh(t *testing.T) {
	t.Parallel()
	for seed := int64(1); seed <= 6; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewSource(seed))
			f := newResumeFixture(t)
			// Every write samples the wall clock, so receipts stay in order.
			lease := func(id string, l transaction.LeaseRequest) func(context.Context) (*Report, error) {
				return func(ctx context.Context) (*Report, error) {
					return Lease(ctx, f.repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: f.root, Lease: l}, WallClock())
				}
			}
			writes := 10 + rng.Intn(110)
			for i := 0; i < writes; i++ {
				id := fmt.Sprintf("diff-%d-%d", seed, i)
				var run writerRun
				switch rng.Intn(3) {
				case 0:
					run = writerMutate(t, f.repo, id, nil)
				case 1:
					run = stagedWrite(lease(id, transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation}), nil)
				default:
					run = stagedWrite(lease(id, transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation, LeaseMinutes: "60"}), nil)
				}
				if !run.completed() {
					t.Fatalf("write %d: %+v %v %v", i, run.rep, run.err, run.stages)
				}
			}
			head, err := readHead(f.repo)
			if err != nil {
				t.Fatal(err)
			}
			_, _, incremental, ok := incrementalRefresh(f.repo, head)
			if !ok {
				t.Fatalf("seed %d: the incremental refresh declined after %d writes", seed, writes)
			}
			proof, obs, err := journalReader(f.repo, head).AuditWriterRefresh()
			if err != nil || proof.Mode != journal.ModeFull {
				t.Fatalf("complete refresh: %v mode %v", err, proof)
			}
			complete, err := proof.WriterCheckpoint(obs.Files)
			if err != nil {
				t.Fatal(err)
			}
			incremental.Invalidation = complete.Invalidation
			ib, id, err := incremental.Encode()
			if err != nil {
				t.Fatal(err)
			}
			cb, cd, err := complete.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if incremental.FullSeq != head.LastSeq.Uint64() || !bytes.Equal(ib, cb) || !bytes.Equal(id, cd) {
				t.Fatalf("seed %d after %d writes: incremental FullSeq %d base %d bytes delta %d bytes, complete FullSeq %d base %d bytes delta %d bytes", seed, writes, incremental.FullSeq, len(ib), len(id), complete.FullSeq, len(cb), len(cd))
			}
			t.Logf("seed %d: %d writes, head %d, base %d bytes, delta %d bytes", seed, writes, head.LastSeq.Uint64(), len(cb), len(cd))
		})
	}
}

// PSR-V0-891: the refresh is incremental while the head stays within the
// checkpoint's WriterFullBound window and complete once it crosses a
// multiple of it, and the scheduled refresh retains the incremental
// checkpoint with FullSeq at the head.
func TestPSRV0891_FullRefreshStaysTheBackstop(t *testing.T) {
	for _, tc := range []struct {
		fullSeq, head uint64
		full          bool
	}{{1, 4095, false}, {600, 4095, false}, {4095, 4096, true}, {4096, 8191, false}, {5000, 8200, true}} {
		if got := fullRefreshDue(tc.fullSeq, tc.head); got != tc.full {
			t.Errorf("fullRefreshDue(%d, %d) = %v, want %v", tc.fullSeq, tc.head, got, tc.full)
		}
	}
	f := newResumeFixture(t)
	for i := 0; i < 12; i++ {
		if run := writerMutate(t, f.repo, fmt.Sprintf("backstop-%d", i), nil); !run.completed() {
			t.Fatalf("write %d: %v", i, run.err)
		}
	}
	var stages []string
	ctx := context.WithValue(context.Background(), mutationStageKey{}, func(s string) { stages = append(stages, s) })
	refreshWriterCheckpoint(ctx, f.repo)
	head, err := readHead(f.repo)
	if err != nil {
		t.Fatal(err)
	}
	wc, _ := boundWriterCheckpoint(f.repo)
	if wc == nil || wc.FullSeq != head.LastSeq.Uint64() || len(stages) == 0 || stages[0] != "refresh.incremental" {
		t.Fatalf("scheduled refresh retained %+v at head %d, stages %v", wc, head.LastSeq.Uint64(), stages)
	}
	// The next writer resumes from the incremental checkpoint.
	if run := writerMutate(t, f.repo, "backstop-after", nil); !run.completed() || !run.fast() {
		t.Fatalf("write after the incremental refresh: %+v %v %v", run.rep, run.err, run.stages)
	}
}

// completeRefreshOnly makes repo's scheduled refresh the complete audit until
// t ends: for tests of what only the complete refresh, the PSR-V0-891
// backstop, detects or retains.
func completeRefreshOnly(t testing.TB, repo *intent.Repository) {
	completeRefreshes.Store(repo.StateDir, struct{}{})
	t.Cleanup(func() { completeRefreshes.Delete(repo.StateDir) })
}
