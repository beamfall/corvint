//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package store

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"os"
	"path/filepath"
	"testing"
)

// LGL-V0-7201: history-sized checkpoint decoding never holds the writer lock.
func TestLGLV07201_CheckpointDecodeOutsideWriterLock(t *testing.T) {
	sizes := []int{70}
	if os.Getenv("CORVINT_TASKS_CHECKPOINT_DECODE_SCALE") == "1" {
		sizes = []int{13500, 40000}
	}
	for _, n := range sizes {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			var history []int
			if n != 70 {
				history = []int{n}
			}
			repo, root, claim := orphanRouteStore(t, history...)
			reads, locked, gated := 0, 0, 0
			runFaults.Store(repo.StateDir, func(point string) error {
				if point == "checkpoint:read" {
					reads++
					gateRepo := *repo
					gateRepo.LockPath = filepath.Join(repo.CommonDir, "taskman.prepare.lock")
					if writerLockProbe(t, &gateRepo) {
						gated++
					}
					if writerLockProbe(t, repo) {
						locked++
					}
				}
				return nil
			})
			defer runFaults.Delete(repo.StateDir)
			run := stagedWrite(func(ctx context.Context) (*Report, error) {
				return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "unlocked-checkpoint-heartbeat", Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: claim.AttemptID, Generation: claim.Generation}}, WallClock())
			}, nil)
			if !run.completed() || !run.fast() {
				t.Fatalf("heartbeat: %+v %v %v", run.rep, run.err, run.stages)
			}
			if reads == 0 || locked != 0 || gated != 0 {
				t.Fatalf("checkpoint reads=%d locked=%d gated=%d", reads, locked, gated)
			}
			t.Logf("receipts=%d checkpoint reads=%d under-writer-lock=%d under-gate=%d", n, reads, locked, gated)
		})
	}

}
