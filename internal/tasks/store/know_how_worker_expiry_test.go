package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0022_LeaseExpiresBetweenAdmissionAndCommit (V1-1007): a WORKER
// KNOWHOW_ADD whose caller sampled its time while the lease was live, but
// whose writer holds the store lock only after the lease expired, is refused
// KNOWHOW_WORKER_ATTEMPT_STALE and writes nothing. The writer resamples the
// live clock under the lock and checks the attempt it read under that same
// lock, so the instant the add would be recorded at decides. The same add
// with the in-lock clock still before expiry commits, so the refusal is the
// expiry and nothing else.
func TestKHNV0022_LeaseExpiresBetweenAdmissionAndCommit(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	v.Obj.Set("knowHow", obj("workerAdd", wire.Bool(true)))
	if r, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("policy-know-how", "2", wire.EncodeFile(v)), now(t)); err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", r, err)
	}
	id := s.planned(t, "home", "P1", "src/")
	claim := s.claim(t, "claim-home", id, 0) // lease ends at t0 + 60 minutes
	worker := mutation.Binding{ID: "agent-1", Role: "WORKER"}
	add := func(requestID string) []byte {
		p := knowHowAddPayload("rebuild the parser first")
		p.Obj.Set("attempt", str(claim.AttemptID))
		p.Obj.Set("generation", str(string(claim.Generation)))
		env, err := wire.Parse(envelope(requestID, mutation.OpKnowHowAdd, id, "1", p))
		if err != nil {
			t.Fatal(err)
		}
		env.Obj.Set("actor", obj("id", str(worker.ID), "role", str(worker.Role)))
		return wire.EncodeFile(env)
	}
	at := func(minutes int) context.Context {
		return store.WithClock(context.Background(), func() wire.Timestamp { return s.at(t, minutes) })
	}

	before := storeDigest(t, s.repo)
	r, err := store.Mutate(at(61), s.repo, worker, add("kh-expired"), s.at(t, 59))
	if err != nil {
		t.Fatalf("expired add: %v", err)
	}
	if r.Outcome.Outcome == mutation.OutcomeCompleted || !strings.Contains(r.Detail, mutation.KnowHowWorkerAttemptStale) {
		t.Fatalf("add after expiry was not refused stale: %+v", r)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused add wrote store state")
	}
	r, err = store.Mutate(at(59), s.repo, worker, add("kh-live"), s.at(t, 59))
	if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("add before expiry: %+v %v", r, err)
	}
	auditOK(t, s.repo)
}
