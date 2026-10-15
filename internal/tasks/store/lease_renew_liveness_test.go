package store_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// leaseRoutes runs a lease sequence on the complete route and on the
// writer-checkpoint route that served lease commands use.
var leaseRoutes = []struct {
	name  string
	lower bool
}{{"complete", false}, {"writer-checkpoint", true}}

func heartbeatOf(a *store.Report) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: a.AttemptID, Generation: a.Generation}
}

func sameLiveness(t *testing.T, label string, before, after *snapshot.Attempt) {
	t.Helper()
	if after.Lease == nil || before.Lease == nil || *after.Lease != *before.Lease || after.LastHeartbeatAt == nil || *after.LastHeartbeatAt != *before.LastHeartbeatAt || after.Phase != before.Phase {
		t.Fatalf("%s moved the live lease: %+v %v -> %+v %v", label, before.Lease, before.LastHeartbeatAt, after.Lease, after.LastHeartbeatAt)
	}
}

// TestCALV0230_SupersededGenerationRenewIsFencedAfterReclaim: claim, renew,
// release, then re-claim the same ticket (same attempt, next generation).
// A fresh-request renew or heartbeat with the superseded generation is
// refused REVISION_CONFLICT FENCED with a recorded refusal, and the live
// lease's expiresAt and lastHeartbeatAt are unchanged. Reusing the request
// ID of the earlier generation's renew replays its committed result
// (CAL-V0-112) without a write, so the live lease is still unchanged.
func TestCALV0230_SupersededGenerationRenewIsFencedAfterReclaim(t *testing.T) {
	t.Parallel()
	for _, route := range leaseRoutes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()
			s := newLeaseStore(t)
			if route.lower {
				store.LowerWriterCheckpointForTest(t, s.repo)
			}
			id := s.ticket(t, "one")
			first := s.claim(t, "claim-1", id, 0, "src")
			if r := s.lease(t, "renew-1", renewOf(first), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("renew: %+v", r)
			}
			if r := s.lease(t, "release-1", releaseOf(first), 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("release: %+v", r)
			}
			second := s.claim(t, "claim-2", id, 3, "src")
			if second.AttemptID != first.AttemptID || second.Generation == first.Generation {
				t.Fatalf("re-claim did not take the next generation: %s/%s -> %s/%s", first.AttemptID, first.Generation, second.AttemptID, second.Generation)
			}
			live := s.attempt(t, second.AttemptID)
			for i, stale := range []transaction.LeaseRequest{renewOf(first), heartbeatOf(first)} {
				report := s.lease(t, "stale-"+stale.Verb, stale, 4+i, nil)
				refusedWith(t, report, mutation.OutcomeRevisionConflict, wire.CodeFenced)
				if report.Receipt == "" {
					t.Fatalf("stale %s refusal was not recorded", stale.Verb)
				}
				sameLiveness(t, "stale "+stale.Verb, live, s.attempt(t, second.AttemptID))
			}
			replay := s.lease(t, "renew-1", renewOf(first), 6, nil)
			if !replay.Outcome.Replayed || replay.Generation != first.Generation {
				t.Fatalf("same-request renew did not replay its committed result: %+v", replay)
			}
			sameLiveness(t, "renew replay", live, s.attempt(t, second.AttemptID))
			if r := s.lease(t, "renew-2", renewOf(second), 7, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("current-generation renew: %+v", r)
			}
			auditOK(t, s.repo)
		})
	}
}

// TestCALV0231_RenewRecordsLastHeartbeat: a successful renew by the current
// holder at the current generation records lastHeartbeatAt at its recorded
// time, as a heartbeat does; a fenced renew records no signal, and a replay
// does not refresh it.
func TestCALV0231_RenewRecordsLastHeartbeat(t *testing.T) {
	t.Parallel()
	for _, route := range leaseRoutes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()
			s := newLeaseStore(t)
			if route.lower {
				store.LowerWriterCheckpointForTest(t, s.repo)
			}
			claim := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src")
			if a := s.attempt(t, claim.AttemptID); a.LastHeartbeatAt == nil || *a.LastHeartbeatAt != s.at(t, 0) {
				t.Fatalf("claim signal: %v", a.LastHeartbeatAt)
			}
			if r := s.lease(t, "renew-1", renewOf(claim), 30, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("renew: %+v", r)
			}
			renewed := s.attempt(t, claim.AttemptID)
			if renewed.LastHeartbeatAt == nil || *renewed.LastHeartbeatAt != s.at(t, 30) || renewed.Lease.ExpiresAt != s.at(t, 90) {
				t.Fatalf("renew did not record liveness: %+v %v", renewed.Lease, renewed.LastHeartbeatAt)
			}
			if r := s.lease(t, "renew-1", renewOf(claim), 50, nil); !r.Outcome.Replayed {
				t.Fatalf("renew replay: %+v", r)
			}
			sameLiveness(t, "renew replay", renewed, s.attempt(t, claim.AttemptID))
			stale := renewOf(claim)
			stale.Generation = wire.SizeOf(claim.Generation.Uint64() + 1)
			refusedWith(t, s.lease(t, "renew-wrong-generation", stale, 60, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
			sameLiveness(t, "fenced renew", renewed, s.attempt(t, claim.AttemptID))
			auditOK(t, s.repo)
		})
	}
}
