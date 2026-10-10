package store_test

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func setRetryPolicy(t *testing.T, s *leaseStore, limit int, minute int) {
	t.Helper()
	loaded, e := intent.Load(s.repo.PrimaryWorktree)
	if e != nil {
		t.Fatal(e)
	}
	v, e := wire.Parse(loaded.Policy.Raw)
	if e != nil {
		t.Fatal(e)
	}
	n := loaded.Policy.PolicyVersion
	v.Obj.Set("policyVersion", str(fmt.Sprint(n.Uint64()+1)))
	retries, _ := v.Obj.Get("retries")
	retries.Obj.Set("admissionsPerRevision", str(fmt.Sprint(limit)))
	r, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest(fmt.Sprintf("policy-%s-%d", n, limit), string(n), wire.EncodeFile(v)), s.at(t, minute))
	if e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", r, e)
	}
}

// CAL-V0-045: initial admission plus N charged retries, including zero and >3;
// current policy controls explicit/next admission and safe OWNER readmission.
func TestCALV0045_PolicyControlsAdmissionAndRecovery(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{0, 1, 3, 4, 16} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			s := newLeaseStore(t)
			id := s.ticket(t, "budget")
			setRetryPolicy(t, s, limit, 0)
			var last *store.Report
			for i := 0; i <= limit; i++ {
				l := claimOf(id, "src/")
				if i%2 == 1 {
					l.Verb = transaction.LeaseClaimNext
					l.TicketID = ""
				}
				last = s.lease(t, fmt.Sprintf("claim-%d", i), l, 0, nil)
				if last.Outcome.Outcome != mutation.OutcomeCompleted || s.attempt(t, last.AttemptID).RetryCount.Int() != int64(i) {
					t.Fatalf("claim%d %+v", i, last)
				}
				r := s.lease(t, fmt.Sprintf("release-%d", i), releaseOf(last), 0, nil)
				if r.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("release %+v", r)
				}
				if i < limit {
					r, e := store.Mutate(context.Background(), s.repo, operator(), envelope(fmt.Sprintf("early-%d", i), mutation.OpReopen, id, "1", obj("reason", str("too early"))), s.at(t, 0))
					if e != nil || r.Outcome.Outcome == mutation.OutcomeCompleted {
						t.Fatalf("premature owner recovery: %+v %v", r, e)
					}
				}
			}
			refusedWith(t, s.lease(t, "exhausted", claimOf(id, "src/"), 0, nil), mutation.OutcomeBlocked, wire.CodeRetryExhausted)
			r, e := store.Mutate(context.Background(), s.repo, operator(), envelope("recover", mutation.OpReopen, id, "1", obj("reason", str("fresh acceptance"))), s.at(t, 0))
			if e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("recovery %+v %v", r, e)
			}
			fresh := s.claim(t, "fresh", id, 0, "src/")
			if s.attempt(t, fresh.AttemptID).RetryCount != "0" {
				t.Fatal("fresh acceptance kept old debt")
			}
			auditOK(t, s.repo)
		})
	}
}

func TestCALV0045_RecoveryUsesCurrentPolicy(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	id := s.ticket(t, "current")
	setRetryPolicy(t, s, 0, 0)
	a := s.claim(t, "claim", id, 0, "src/")
	s.lease(t, "release", releaseOf(a), 0, nil)
	setRetryPolicy(t, s, 4, 0)
	r, e := store.Mutate(context.Background(), s.repo, operator(), envelope("recover", mutation.OpReopen, id, "1", obj("reason", str("no longer exhausted"))), s.at(t, 0))
	if e != nil || r.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("recovery ignored raised budget: %+v %v", r, e)
	}
	next := s.claim(t, "next", id, 0, "src/")
	if s.attempt(t, next.AttemptID).RetryCount != "1" {
		t.Fatal("raising policy erased debt")
	}
}
