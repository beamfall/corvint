package store_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// timedLease runs one lease command and returns its report and the sync
// calls its journal writes made.
func (s *leaseStore) timedLease(t *testing.T, requestID string, l transaction.LeaseRequest, minutes int) (*store.Report, int) {
	t.Helper()
	var timing store.LeaseTiming
	r := s.leaseWith(store.WithLeaseTiming(context.Background(), &timing), t, requestID, l, minutes, nil)
	if r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("%s: %+v", requestID, r)
	}
	return r, timing.Syncs
}

// TestLGLV0006_OnlyLivenessWritesSkipStagingSync: on both lease routes a
// heartbeat and a renew make two sync calls per artifact they publish
// (receipt, request entry, attempt, head: each file and its destination
// directory, plus a new request shard's two), while a claim and a release
// make four per artifact, the staging directory's two included.
func TestLGLV0006_OnlyLivenessWritesSkipStagingSync(t *testing.T) {
	t.Parallel()
	for _, route := range leaseRoutes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()
			s := newLeaseStore(t)
			if route.lower {
				store.LowerWriterCheckpointForTest(t, s.repo)
			}
			id := s.ticket(t, "one")
			claim, claimSyncs := s.timedLease(t, "claim-1", claimOf(id, "src/"), 0)
			_, beat := s.timedLease(t, "heartbeat-1", heartbeatOf(claim), 1)
			_, renew := s.timedLease(t, "renew-1", renewOf(claim), 2)
			_, release := s.timedLease(t, "release-1", releaseOf(claim), 3)
			for verb, n := range map[string]int{"heartbeat": beat, "renew": renew} {
				if n != 8 && n != 10 {
					t.Errorf("%s made %d sync calls, want 8, or 10 with a new request shard", verb, n)
				}
			}
			for verb, n := range map[string]int{"claim": claimSyncs, "release": release} {
				if n < 16 || (n%4 != 0 && (n-2)%4 != 0) {
					t.Errorf("%s made %d sync calls, want four per artifact (at least 16)", verb, n)
				}
			}
			auditOK(t, s.repo)
		})
	}
}

// TestLGLV0007_LivenessStagingResidueIsAnOrphan: the only shape a heartbeat
// or renew leaves after a crash that a full session could not is a staging
// slot name its skipped sync did not retire: a second link to the receipt it
// linked or the head it renamed. With the writer lock free, audit sets that
// residue aside and verifies; the lease reads exactly as the acknowledged
// heartbeat left it; the next heartbeat clears the residue and commits; and
// nothing grants a lease.
func TestLGLV0007_LivenessStagingResidueIsAnOrphan(t *testing.T) {
	t.Parallel()
	for _, route := range leaseRoutes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()
			s := newLeaseStore(t)
			if route.lower {
				store.LowerWriterCheckpointForTest(t, s.repo)
			}
			id := s.ticket(t, "one")
			claim := s.claim(t, "claim-1", id, 0, "src/")
			beat := s.lease(t, "heartbeat-1", heartbeatOf(claim), 1, nil)
			if beat.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("heartbeat: %+v", beat)
			}
			acknowledged := s.attempt(t, claim.AttemptID)
			staging := filepath.Join(s.repo.StateDir, "staging")
			if names := dirNames(t, staging); len(names) != 0 {
				t.Fatalf("a heartbeat left staging entries: %v", names)
			}
			// The crash restores the slot names whose removal and rename were
			// not synced in staging.
			last := lastReceipt(t, s.repo.StateDir)
			if err := os.Link(last, filepath.Join(staging, "a00")); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(filepath.Join(s.repo.StateDir, "head.json"), filepath.Join(staging, "a03")); err != nil {
				t.Fatal(err)
			}
			auditOK(t, s.repo)
			sameLiveness(t, "residue", acknowledged, s.attempt(t, claim.AttemptID))
			next := s.lease(t, "heartbeat-2", heartbeatOf(claim), 2, nil)
			if next.Outcome.Outcome != mutation.OutcomeCompleted || next.AttemptID != claim.AttemptID || next.Generation != claim.Generation {
				t.Fatalf("heartbeat after residue: %+v", next)
			}
			if names := dirNames(t, staging); len(names) != 0 {
				t.Fatalf("the next writer left the residue: %v", names)
			}
			after := s.attempt(t, claim.AttemptID)
			if after.Lease == nil || acknowledged.Lease == nil || *after.Lease != *acknowledged.Lease || after.LastHeartbeatAt == nil || *after.LastHeartbeatAt == *acknowledged.LastHeartbeatAt {
				t.Fatalf("heartbeat after residue moved more than lastHeartbeatAt: %+v -> %+v", acknowledged, after)
			}
			other := s.lease(t, "claim-other", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: id, Holder: "agent-2", LeaseMinutes: "60"}, 3, nil)
			if other.Outcome.Outcome == mutation.OutcomeCompleted {
				t.Fatalf("a second holder claimed the live lease: %+v", other)
			}
			if held := s.attempt(t, claim.AttemptID); held.Lease == nil || held.Lease.Holder != acknowledged.Lease.Holder {
				t.Fatalf("lease holder moved: %+v", held.Lease)
			}
			s.consistent(t)
		})
	}
}

// TestLGLV0008_LostLivenessTailIsRefusedNotAdopted pins why heartbeat and
// renew keep their file and destination syncs: the store reads a lost or
// torn tail receipt, or a head that lost its last advance, as a fork, a
// malformed receipt or pending redo, never as an older heartbeat. Adopting
// such a tail would need a durable watermark a closed head.json cannot carry.
func TestLGLV0008_LostLivenessTailIsRefusedNotAdopted(t *testing.T) {
	t.Parallel()
	for _, shape := range []struct {
		name  string
		codes []string
		lose  func(t *testing.T, stateDir, last string, headBefore []byte)
	}{
		{"receipt lost", []string{wire.CodeJournalForked}, func(t *testing.T, _, last string, _ []byte) {
			if err := os.Remove(last); err != nil {
				t.Fatal(err)
			}
		}},
		{"receipt torn", []string{wire.CodeJournalForked, wire.CodeMalformed}, func(t *testing.T, _, last string, _ []byte) {
			info, err := os.Stat(last)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(last, info.Size()/2); err != nil {
				t.Fatal(err)
			}
		}},
		{"head advance lost", []string{wire.CodeRedoPending}, func(t *testing.T, stateDir, _ string, headBefore []byte) {
			if err := os.WriteFile(filepath.Join(stateDir, "head.json"), headBefore, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			s := newLeaseStore(t)
			id := s.ticket(t, "one")
			claim := s.claim(t, "claim-1", id, 0, "src/")
			headBefore, err := os.ReadFile(filepath.Join(s.repo.StateDir, "head.json"))
			if err != nil {
				t.Fatal(err)
			}
			if r := s.lease(t, "heartbeat-1", heartbeatOf(claim), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("heartbeat: %+v", r)
			}
			shape.lose(t, s.repo.StateDir, lastReceipt(t, s.repo.StateDir), headBefore)
			q, err := wire.ParseQueueID("queueId", fixture.QueueID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = (journal.Reader{Source: journal.Native{StateDir: s.repo.StateDir, PrimaryWorktree: s.repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: s.repo.PrimaryWorktree}).Audit()
			got := wire.CodeOf(err)
			for _, code := range shape.codes {
				if got == code {
					return
				}
			}
			t.Fatalf("audit of a %s tail: %s (%v), want one of %v", shape.name, got, err, shape.codes)
		})
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// lastReceipt is the path of the highest-numbered receipt.
func lastReceipt(t *testing.T, stateDir string) string {
	t.Helper()
	names := []string{}
	for _, name := range dirNames(t, filepath.Join(stateDir, "receipts")) {
		if strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		t.Fatal("no receipts")
	}
	sort.Strings(names)
	return filepath.Join(stateDir, "receipts", names[len(names)-1])
}
