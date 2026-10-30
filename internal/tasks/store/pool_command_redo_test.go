//go:build unix

package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0261_PoolCommandReportsSettledRedo: health settled a receipt an
// interrupted writer left pending (head.json rewound past a committed policy
// receipt) but dropped that fact, so `health` and `pool cleanup` never showed
// the completed-redo warning other writers show. Now PoolCommand reports it.
func TestCALV0261_PoolCommandReportsSettledRedo(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("health", health)))))
	headPath := filepath.Join(s.repo.StateDir, "head.json")
	held, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("redo-policy", "2", wire.EncodeFile(v)), now(t)); e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	fixture.Write(t, headPath, held)
	if _, e := snapshot.Probe(s.repo.StateDir); wire.CodeOf(e) != wire.CodeRedoPending {
		t.Fatalf("the rewound head does not hold a pending receipt: %v", e)
	}
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "redo-health", Root: s.root, Lease: transaction.LeaseRequest{Member: "a"}}
	rep, e := store.PoolCommand(context.Background(), s.repo, operator(), choice, "health")
	if e != nil || rep == nil || rep.Outcome.Outcome != mutation.OutcomeCompleted || !rep.Redone {
		t.Fatalf("health over a pending receipt: %+v %v", rep, e)
	}
	if _, e := snapshot.Probe(s.repo.StateDir); e != nil {
		t.Fatalf("probe after health: %v", e)
	}
}
