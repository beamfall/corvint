package store_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// generatedSpecIndex is the generated spec index set V1-1639 and V1-0812
// both listed (V1-2119).
var generatedSpecIndex = []string{"docs/specs/INDEX.json", "docs/specs/README.md", "docs/specs/REQUIREMENTS.tsv"}

// sharedLeaseStore is leaseStoreOn whose policy also declares shared
// reservation paths; nil shared leaves the reservations key absent.
func sharedLeaseStore(t *testing.T, shared []string) *leaseStore {
	t.Helper()
	repo, _ := initialized(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("2"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	if shared != nil {
		v.Obj.Set("reservations", obj("sharedPaths", wire.Strings(shared)))
	}
	report, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-lease", "1", wire.EncodeFile(v)), now(t))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
	return &leaseStore{repo: repo, root: gitRoot(t), t0: now(t)}
}

// TestNTPV0026_SharedSpecIndexDoesNotCollide reproduces the V1-1639 vs
// V1-0812 collision: a parked attempt lists the generated spec index files,
// and a second attempt's widen to the same files is refused
// RESOURCE_COLLISION without the policy declaration and admitted with it,
// for a claim as well as a widen, while a hand-authored spec path or the
// uncovered docs/specs/ directory key still collides (V1-2119).
func TestNTPV0026_SharedSpecIndexDoesNotCollide(t *testing.T) {
	t.Parallel()
	parkedScope := []string{"docs/specs/INDEX.json", "docs/specs/README.md", "docs/specs/REQUIREMENTS.tsv", "docs/specs/a-v0.md"}
	widenOf := func(c *store.Report, scope ...string) transaction.LeaseRequest {
		return transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: c.AttemptID, Generation: c.Generation, Scope: scope}
	}

	t.Run("undeclared generated paths stay exclusive", func(t *testing.T) {
		s := sharedLeaseStore(t, nil)
		parked, second := s.ticket(t, "parked"), s.ticket(t, "second")
		s.claim(t, "claim-parked", parked, 0, parkedScope...)
		c := s.claim(t, "claim-second", second, 1, "docs/specs/b-v0.md")
		refusedWith(t, s.lease(t, "widen-second", widenOf(c, generatedSpecIndex...), 2, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
		auditOK(t, s.repo)
	})

	t.Run("declared generated paths are shared", func(t *testing.T) {
		s := sharedLeaseStore(t, generatedSpecIndex)
		parked, second, third := s.ticket(t, "parked"), s.ticket(t, "second"), s.ticket(t, "third")
		s.claim(t, "claim-parked", parked, 0, parkedScope...)
		c := s.claim(t, "claim-second", second, 1, "docs/specs/b-v0.md")
		if r := s.lease(t, "widen-second", widenOf(c, generatedSpecIndex...), 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("widen to the shared generated paths: %+v", r)
		}
		s.claim(t, "claim-third", third, 3, "docs/specs/REQUIREMENTS.tsv", "docs/specs/c-v0.md")
		auditOK(t, s.repo)
	})

	t.Run("hand-authored paths keep exclusive semantics", func(t *testing.T) {
		s := sharedLeaseStore(t, generatedSpecIndex)
		parked, second := s.ticket(t, "parked"), s.ticket(t, "second")
		s.claim(t, "claim-parked", parked, 0, parkedScope...)
		c := s.claim(t, "claim-second", second, 1, "docs/specs/b-v0.md")
		refusedWith(t, s.lease(t, "widen-hand", widenOf(c, "docs/specs/a-v0.md"), 2, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
		refusedWith(t, s.lease(t, "widen-dir", widenOf(c, "docs/specs/"), 3, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
		auditOK(t, s.repo)
	})
}
