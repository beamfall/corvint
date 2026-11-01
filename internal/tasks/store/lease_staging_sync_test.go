package store_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
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
// heartbeat, a renew and (LGL-V0-780) a claim make two sync calls per
// artifact they publish (each file and its destination directory, plus a new
// request shard's two), while a release makes four per artifact, the staging
// directory's two included.
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
			if claimSyncs < 8 || claimSyncs%2 != 0 || claimSyncs >= release {
				t.Errorf("claim made %d sync calls, want two per artifact, fewer than the release's %d", claimSyncs, release)
			}
			if release < 16 || (release%4 != 0 && (release-2)%4 != 0) {
				t.Errorf("release made %d sync calls, want four per artifact (at least 16)", release)
			}
			auditOK(t, s.repo)
		})
	}
}

// TestLGLV0005_LivenessRequestRedoesPendingWriteWithFullSyncs: a heartbeat
// that first redoes another write's pending post effects makes every sync for
// that redo, exactly as a claim redoing the same pending write does. Each
// trigger is a replay, so after the redo it writes nothing of its own.
func TestLGLV0005_LivenessRequestRedoesPendingWriteWithFullSyncs(t *testing.T) {
	t.Parallel()
	pending := func(t *testing.T) (*leaseStore, *store.Report, string) {
		s := newLeaseStore(t)
		one, two := s.ticket(t, "one"), s.ticket(t, "two")
		claim := s.claim(t, "claim-1", one, 0, "src/")
		if r := s.lease(t, "heartbeat-1", heartbeatOf(claim), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("heartbeat: %+v", r)
		}
		head := filepath.Join(s.repo.StateDir, "head.json")
		before, err := os.ReadFile(head)
		if err != nil {
			t.Fatal(err)
		}
		s.claim(t, "claim-2", two, 2, "docs/")
		// The crash kept claim-2's receipt and lost its head advance.
		if err := os.WriteFile(head, before, 0o644); err != nil {
			t.Fatal(err)
		}
		return s, claim, one
	}
	redo := func(t *testing.T, s *leaseStore, requestID string, l transaction.LeaseRequest, minutes int) int {
		t.Helper()
		r, syncs := s.timedLease(t, requestID, l, minutes)
		if !r.Redone {
			t.Fatalf("%s did not redo the pending claim: %+v", requestID, r)
		}
		auditOK(t, s.repo)
		return syncs
	}
	s, claim, _ := pending(t)
	beat := redo(t, s, "heartbeat-1", heartbeatOf(claim), 1)
	s, _, one := pending(t)
	full := redo(t, s, "claim-1", claimOf(one, "src/"), 0)
	if full < 4 || beat != full {
		t.Fatalf("a heartbeat's redo made %d sync calls and a claim's %d; want the same full count", beat, full)
	}
}

// TestLGLV0007_LivenessStagingResidueIsAnOrphan: the only shape a heartbeat
// or renew leaves after a crash that a full session could not is a staging
// slot name its skipped sync did not retire: a second link to an artifact it
// linked (the receipt, the request entry) or replaced (the attempt, the
// head). With the writer lock free, audit sets that residue aside and
// verifies; the lease reads exactly as the acknowledged write left it; the
// next heartbeat clears the residue and commits; and nothing grants a lease.
func TestLGLV0007_LivenessStagingResidueIsAnOrphan(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"heartbeat", "renew"} {
		for _, route := range leaseRoutes {
			t.Run(verb+"/"+route.name, func(t *testing.T) {
				t.Parallel()
				s := newLeaseStore(t)
				if route.lower {
					store.LowerWriterCheckpointForTest(t, s.repo)
				}
				id := s.ticket(t, "one")
				claim := s.claim(t, "claim-1", id, 0, "src/")
				live := heartbeatOf(claim)
				if verb == "renew" {
					live = renewOf(claim)
				}
				if r := s.lease(t, verb+"-1", live, 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("%s: %+v", verb, r)
				}
				acknowledged := s.attempt(t, claim.AttemptID)
				staging := filepath.Join(s.repo.StateDir, "staging")
				if names := dirNames(t, staging); len(names) != 0 {
					t.Fatalf("a %s left staging entries: %v", verb, names)
				}
				// The crash restores every slot name whose removal or rename
				// was not synced in staging, in the order the write staged
				// them: the receipt, its post entries, then the head.
				published := livenessArtifacts(t, s.repo.StateDir)
				for i, path := range published {
					if err := os.Link(path, filepath.Join(staging, fmt.Sprintf("a%02d", i))); err != nil {
						t.Fatal(err)
					}
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
}

// TestLGLV0780_ClaimStagingResidueIsAnOrphan: a claim that skipped its
// staging syncs can leave, after a crash, only staging slot names holding
// second links to the artifacts it published. With the writer lock free the
// audit sets them aside and verifies, the claim reads as acknowledged, a
// second holder cannot claim the same ticket, and the next writer clears the
// residue and commits (LGL-V0-780, as LGL-V0-007 for liveness writes).
func TestLGLV0780_ClaimStagingResidueIsAnOrphan(t *testing.T) {
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
			acknowledged := s.attempt(t, claim.AttemptID)
			staging := filepath.Join(s.repo.StateDir, "staging")
			if names := dirNames(t, staging); len(names) != 0 {
				t.Fatalf("a claim left staging entries: %v", names)
			}
			for i, path := range livenessArtifacts(t, s.repo.StateDir) {
				if err := os.Link(path, filepath.Join(staging, fmt.Sprintf("a%02d", i))); err != nil {
					t.Fatal(err)
				}
			}
			auditOK(t, s.repo)
			sameLiveness(t, "residue", acknowledged, s.attempt(t, claim.AttemptID))
			other := s.lease(t, "claim-other", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: id, Holder: "agent-2", LeaseMinutes: "60"}, 1, nil)
			if other.Outcome.Outcome == mutation.OutcomeCompleted {
				t.Fatalf("a second holder claimed the live lease: %+v", other)
			}
			next := s.lease(t, "heartbeat-1", heartbeatOf(claim), 2, nil)
			if next.Outcome.Outcome != mutation.OutcomeCompleted || next.AttemptID != claim.AttemptID || next.Generation != claim.Generation {
				t.Fatalf("heartbeat after residue: %+v", next)
			}
			if names := dirNames(t, staging); len(names) != 0 {
				t.Fatalf("the next writer left the residue: %v", names)
			}
			if held := s.attempt(t, claim.AttemptID); held.Lease == nil || held.Lease.Holder != acknowledged.Lease.Holder {
				t.Fatalf("lease holder moved: %+v", held.Lease)
			}
			s.consistent(t)
		})
	}
}

// livenessArtifacts lists the files the last receipt published: the receipt,
// each post entry with content, then head.json. A liveness write publishes at
// least the request entry and the attempt.
func livenessArtifacts(t *testing.T, stateDir string) []string {
	t.Helper()
	last := lastReceipt(t, stateDir)
	raw, err := os.ReadFile(last)
	if err != nil {
		t.Fatal(err)
	}
	record, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{last}
	for _, post := range record.Post {
		if post.Sha256 == nil {
			continue
		}
		path := filepath.Join(stateDir, filepath.FromSlash(post.Path))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("post %s: %v", post.Path, err)
		}
		paths = append(paths, path)
	}
	if len(paths) < 3 {
		t.Fatalf("the liveness receipt published %d post files, want the request entry and the attempt", len(paths)-1)
	}
	return append(paths, filepath.Join(stateDir, "head.json"))
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
