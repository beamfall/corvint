//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTW-V0-013: measurement instrumentation must retain the ordinary batch
// publisher, with serial directory syncs and all base durability edges.
func TestCTWV07100_BatchEntryKeepsBasePublication(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "batch sync")
	rec := newSyncRecorder(t, s.repo.StateDir, -1)
	raw := envelope("batch-sync/0", mutation.OpRefine, id, "1", obj("labels", wire.Strings([]string{"tier-test"})))
	rep, err := store.MutateBatch(context.Background(), s.repo, operator(), [][]byte{raw}, s.at(t, 1))
	if err != nil || rep.Entries[0].Err != nil || rep.Entries[0].Report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("batch: %+v %v", rep, err)
	}
	events := rec.resolved()
	info, err := os.Stat(filepath.Join(s.repo.IntentRoot(), ".taskman", "tickets"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range events {
		if events[i].info != nil && os.SameFile(info, events[i].info) {
			events[i].dir = "intent/tickets"
		}
	}
	checkOrderingEdges(t, "batch refine", events, false)
	if rec.maxIn != 1 {
		t.Fatalf("batch publication differs from base: max sync concurrency %d", rec.maxIn)
	}
	auditOK(t, s.repo)
}

// CTW-V0-013: every publication boundary keeps the staged mutation's
// ordinary recovery and exact replay semantics.
func TestCTWV07100_BatchPublicationFailureCleansStages(t *testing.T) {
	for stop := 0; stop < 4; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			s := newLeaseStore(t)
			id := s.ticket(t, "batch failure")
			raw := envelope("batch-fault/0", mutation.OpRefine, id, "1", obj("labels", wire.Strings([]string{"tier-test"})))
			seen := 0
			restore := store.SetStorePublishFaultForTest(s.repo.StateDir, func(transaction.Artifact) error {
				seen++
				if seen == stop+1 {
					return errors.New("injected publication failure")
				}
				return nil
			})
			first, err := store.MutateBatch(context.Background(), s.repo, operator(), [][]byte{raw}, s.at(t, 1))
			restore()
			if err == nil && first.Entries[0].Err == nil {
				t.Fatal("publication fault not exercised")
			}
			if names := dirNames(t, filepath.Join(s.repo.StateDir, "staging")); len(names) != 0 {
				t.Fatalf("stages left: %v", names)
			}
			retry, err := store.MutateBatch(context.Background(), s.repo, operator(), [][]byte{raw}, s.at(t, 2))
			if err != nil || retry.Entries[0].Err != nil || retry.Entries[0].Report.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("retry: %+v %v", retry, err)
			}
			again, err := store.MutateBatch(context.Background(), s.repo, operator(), [][]byte{raw}, s.at(t, 3))
			if err != nil || again.Entries[0].Err != nil || !again.Entries[0].Report.Outcome.Replayed {
				t.Fatalf("replay: %+v %v", again, err)
			}
			auditOK(t, s.repo)
		})
	}
}
