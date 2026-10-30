package store_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// dirCounts counts the files under each top-level directory of the state
// dir; attempt IDs and request digests differ between two fresh stores, so
// the shape, not the names, is compared.
func dirCounts(t *testing.T, stateDir string) map[string]int {
	t.Helper()
	counts := map[string]int{}
	err := filepath.WalkDir(stateDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(stateDir, p)
		counts[strings.SplitN(rel, string(filepath.Separator), 2)[0]]++
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return counts
}

// TestLGLV0013_LeaseStagesTogetherPublishesTheSameStore: on both lease
// routes a claim, heartbeat, renew and release staged together leave every
// artifact's destination holding exactly that artifact's planned bytes,
// make the same sync calls and leave the same number of files in each state
// directory as the same commands staged one artifact at a time; staging is
// empty after each and the audit passes.
func TestLGLV0013_LeaseStagesTogetherPublishesTheSameStore(t *testing.T) {
	for _, route := range leaseRoutes {
		t.Run(route.name, func(t *testing.T) {
			run := func(together bool) (map[string]int, []int) {
				defer store.SetLeaseStagesTogetherForTest(together)()
				s := newLeaseStore(t)
				if route.lower {
					store.LowerWriterCheckpointForTest(t, s.repo)
				}
				id := s.ticket(t, "one")
				var arts []transaction.Artifact
				defer store.SetPublishFaultForTest(func(a transaction.Artifact) error { arts = append(arts, a); return nil })()
				claim, c := s.timedLease(t, "claim-1", claimOf(id, "src/"), 0)
				syncs := []int{c}
				steps := []transaction.LeaseRequest{heartbeatOf(claim), renewOf(claim), releaseOf(claim)}
				for i := -1; i < len(steps); i++ {
					if i >= 0 {
						arts = nil
						_, n := s.timedLease(t, fmt.Sprintf("live-%d", i), steps[i], i+1)
						syncs = append(syncs, n)
					}
					if len(arts) < 4 {
						t.Fatalf("together=%v step %d published %d artifacts", together, i, len(arts))
					}
					for _, a := range arts {
						b, err := os.ReadFile(filepath.Join(s.repo.StateDir, filepath.FromSlash(a.Target)))
						if err != nil || !bytes.Equal(b, a.Data) {
							t.Fatalf("together=%v step %d: %s does not hold its planned bytes: %v", together, i, a.Target, err)
						}
					}
					if names := dirNames(t, filepath.Join(s.repo.StateDir, "staging")); len(names) != 0 {
						t.Fatalf("together=%v step %d left staging entries: %v", together, i, names)
					}
				}
				auditOK(t, s.repo)
				return dirCounts(t, s.repo.StateDir), syncs
			}
			seqCounts, seqSyncs := run(false)
			togCounts, togSyncs := run(true)
			if !reflect.DeepEqual(seqSyncs, togSyncs) {
				t.Fatalf("sync calls: sequential %v, together %v", seqSyncs, togSyncs)
			}
			if !reflect.DeepEqual(seqCounts, togCounts) {
				t.Fatalf("state files: sequential %v, together %v", seqCounts, togCounts)
			}
		})
	}
}

// TestLGLV0013_LeaseFaultAfterStagingLeavesNoSlot: a heartbeat that stops
// before any one of its artifacts is published, after all of them were
// staged together, removes every staged slot; the store then reads, the
// next heartbeat completes (redoing a committed receipt's posts when the
// stop came after it), and the audit passes.
func TestLGLV0013_LeaseFaultAfterStagingLeavesNoSlot(t *testing.T) {
	errInjected := errors.New("injected stop")
	for stop := 0; stop < 4; stop++ {
		t.Run(fmt.Sprintf("before-artifact-%d", stop), func(t *testing.T) {
			s := newLeaseStore(t)
			id := s.ticket(t, "one")
			claim := s.claim(t, "claim-1", id, 0, "src/")
			seen, fired := 0, false
			restore := store.SetPublishFaultForTest(func(transaction.Artifact) error {
				if !fired && seen == stop {
					fired = true
					return errInjected
				}
				seen++
				return nil
			})
			choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "heartbeat-1", Root: s.root, Lease: heartbeatOf(claim)}
			r, err := store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, 1))
			restore()
			if !fired || !errors.Is(err, errInjected) {
				t.Fatalf("the stop before artifact %d did not fail the heartbeat (fired %v): %+v %v", stop, fired, r, err)
			}
			if names := dirNames(t, filepath.Join(s.repo.StateDir, "staging")); len(names) != 0 {
				t.Fatalf("staging entries after the stop: %v", names)
			}
			if r := s.lease(t, "heartbeat-2", heartbeatOf(claim), 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("the next heartbeat: %+v", r)
			}
			auditOK(t, s.repo)
		})
	}
}
