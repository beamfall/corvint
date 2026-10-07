//go:build unix

package store_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func shareClaimOf(id, scope, minutes string, share wire.Digest) transaction.LeaseRequest {
	c := claimOf(id, scope)
	c.Pool, c.Stage, c.LeaseMinutes = "db", "implement", wire.Size(minutes)
	c.ShareAllocation = string(share)
	return c
}

// shareBound returns the only ALLOCATED entry's bound attempts, primary first.
func shareBound(t *testing.T, s *leaseStore) []string {
	t.Helper()
	out := []string{}
	for _, en := range untouchedPoolState(t, s).Entries {
		if en.State == "ALLOCATED" {
			out = append(out, en.AttemptID)
			for _, x := range en.Shared {
				out = append(out, x.AttemptID)
			}
		}
	}
	return out
}

// PSR-V0-018: an expired bound lease is reaped on its own; the others stay
// bound, a reaped primary hands the allocation over, and nothing quarantines.
func TestPSRV0018_ShareLeaseExpiry(t *testing.T) {
	for _, primaryExpires := range []bool{true, false} {
		t.Run(fmt.Sprint("primaryExpires=", primaryExpires), func(t *testing.T) {
			s := newLeaseStore(t)
			exclusionPolicy(t, s, wire.Null())
			one, two, three := s.ticket(t, "one"), s.ticket(t, "two"), s.ticket(t, "three")
			short, long := "5", "60"
			if !primaryExpires {
				short, long = long, short
			}
			a := s.lease(t, "claim-a", shareClaimOf(one, "src/a", short, ""), 0, nil)
			if a.PoolAllocation == nil {
				t.Fatalf("claim a %+v", a)
			}
			id := a.PoolAllocation.AllocationID
			b := s.lease(t, "claim-b", shareClaimOf(two, "src/b", long, id), 0, nil)
			if b.Outcome.Outcome != mutation.OutcomeCompleted || b.SharedAllocation == nil || b.SharedAllocation.SourceAttemptID != a.AttemptID || !reflect.DeepEqual(b.PoolAllocation, a.PoolAllocation) {
				t.Fatalf("share %+v", b)
			}
			if s.attempt(t, b.AttemptID).DirectPoolAdmission != nil {
				t.Fatal("shared attempt carries a direct pool admission")
			}
			expiring, staying := a, b
			if !primaryExpires {
				expiring, staying = b, a
			}
			c := s.lease(t, "claim-c", shareClaimOf(three, "src/c", "60", id), 10, nil)
			if c.Outcome.Outcome != mutation.OutcomeCompleted || len(c.Reaped) != 1 || c.Reaped[0].AttemptID != expiring.AttemptID {
				t.Fatalf("claim after expiry %+v", c)
			}
			if got := shareBound(t, s); !reflect.DeepEqual(got, []string{staying.AttemptID, c.AttemptID}) {
				t.Fatalf("bound after reap %v", got)
			}
			if x := s.attempt(t, staying.AttemptID); !x.Live() || x.Lease == nil {
				t.Fatalf("expiry disturbed a sibling %+v", x)
			}
			if x := s.attempt(t, expiring.AttemptID); x.Phase != "FAILED" || x.Cause == nil || *x.Cause != snapshot.CauseLeaseExpired {
				t.Fatalf("expired attempt %+v", x)
			}
			for _, r := range []*store.Report{staying, c} {
				if done := s.lease(t, "release-"+r.AttemptID, releaseOf(r), 10, nil); done.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("release %+v", done)
				}
			}
			p := untouchedPoolState(t, s)
			if len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" || p.Entries[0].AttemptID != c.AttemptID || len(p.Entries[0].Shared) != 0 {
				t.Fatalf("final pool %+v", p.Entries)
			}
			auditOK(t, s.repo)
		})
	}
}

// PSR-V0-016, PSR-V0-021: an interrupted share claim either left no receipt
// and retries fresh, or redoes its committed receipt; either way the binding
// lands once and replays byte-identically.
func TestPSRV0016_ShareCrashReplay(t *testing.T) {
	fixtureFor := func(t *testing.T) (*leaseStore, *store.Report, transaction.LeaseRequest) {
		s := newLeaseStore(t)
		exclusionPolicy(t, s, wire.Null())
		one, two := s.ticket(t, "one"), s.ticket(t, "two")
		a := s.lease(t, "claim-a", shareClaimOf(one, "src/a", "60", ""), 0, nil)
		if a.PoolAllocation == nil {
			t.Fatalf("claim a %+v", a)
		}
		return s, a, shareClaimOf(two, "src/b", "60", a.PoolAllocation.AllocationID)
	}
	s, _, share := fixtureFor(t)
	var order []string
	restore := store.SetPublishFaultForTest(func(x transaction.Artifact) error {
		order = append(order, x.Role+":"+x.Target)
		return nil
	})
	func() { defer restore(); s.lease(t, "share", share, 0, nil) }()
	receiptAt := -1
	for i, key := range order {
		if strings.HasPrefix(key, "RECEIPT") && receiptAt < 0 {
			receiptAt = i
		}
	}
	if receiptAt < 0 || receiptAt >= len(order)-1 {
		t.Fatalf("publication order %v", order)
	}
	t.Logf("share claim publication order: %v", order)
	for k := range order {
		t.Run(fmt.Sprintf("%02d", k), func(t *testing.T) {
			s, a, share := fixtureFor(t)
			head, count := journalState(t, s.repo)
			n, fired := 0, false
			restore := store.SetPublishFaultForTest(func(transaction.Artifact) error {
				at := n
				n++
				if at == k {
					fired = true
					return errInjected
				}
				return nil
			})
			var err error
			func() {
				defer restore()
				_, err = store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "share", Root: s.root, Lease: share}, s.at(t, 0))
			}()
			if !fired || !errors.Is(err, errInjected) {
				t.Fatal("fault not reached", k, err)
			}
			committed := k > receiptAt
			if gotHead, gotCount := journalState(t, s.repo); gotHead != head || gotCount != count+btoi(committed) {
				t.Fatal("wrong receipt fence", k)
			}
			if _, err := snapshot.Probe(s.repo.StateDir); committed != (wire.CodeOf(err) == wire.CodeRedoPending) || !committed && err != nil {
				t.Fatal("interrupted snapshot", committed, err)
			}
			b := s.lease(t, "share", share, 0, nil)
			if b.Outcome.Outcome != mutation.OutcomeCompleted || b.Redone != committed || b.SharedAllocation == nil {
				t.Fatalf("recovered %+v", b)
			}
			if got := shareBound(t, s); !reflect.DeepEqual(got, []string{a.AttemptID, b.AttemptID}) {
				t.Fatalf("bound after recovery %v", got)
			}
			auditOK(t, s.repo)
			before := storeDigest(t, s.repo)
			again := s.lease(t, "share", share, 0, nil)
			if !again.Outcome.Replayed || again.AttemptID != b.AttemptID || !reflect.DeepEqual(again.SharedAllocation, b.SharedAllocation) || storeDigest(t, s.repo) != before {
				t.Fatal("replay changed payload or state")
			}
		})
	}
}

// PSR-V0-019: a shared allocation cannot be released lane-untouched, so the
// refusal writes nothing. (Supervisor attach is refused in transaction.)
func TestPSRV0019_ShareExcludesLaneUntouched(t *testing.T) {
	s := newLeaseStore(t)
	exclusionPolicy(t, s, wire.Null())
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	a := s.lease(t, "claim-a", shareClaimOf(one, "src/a", "60", ""), 0, nil)
	b := s.lease(t, "claim-b", shareClaimOf(two, "src/b", "60", a.PoolAllocation.AllocationID), 0, nil)
	if b.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("share %+v", b)
	}
	before := storeDigest(t, s.repo)
	rel := releaseOf(a)
	rel.LaneUntouched, rel.Evidence = true, "local:unused"
	if r := s.lease(t, "untouched", rel, 0, nil); r.Outcome.Outcome == mutation.OutcomeCompleted || r.LaneUntouchedAttestation != nil || storeDigest(t, s.repo) != before {
		t.Fatalf("lane-untouched release of a shared allocation %+v", r)
	}
}
