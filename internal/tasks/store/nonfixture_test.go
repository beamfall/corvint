package store_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// nonFixture initializes a store whose queue has fixture false and the given
// canonical writer, with the fixture policy.
func nonFixture(t *testing.T, writer string) *intent.Repository {
	t.Helper()
	repo := fixture.TempRepo(t)
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(false))
	q.Obj.Set("canonicalWriter", str(writer))
	fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), wire.EncodeFile(q))
	fixture.Write(t, filepath.Join(repo.IntentDir, "policy.json"), fixture.PolicyBytes())
	resolved, err := intent.Resolve(repo.Root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err = store.Init(context.Background(), resolved, operator(), "req-init", now(t)); err != nil {
		t.Fatalf("init: %v", err)
	}
	return resolved
}

// TestCALV0001_NonFixtureQueueTakesEveryWrite: a queue with fixture false and
// a null importMapSha256 initializes and then commits a ticket create and
// edit, an import, pause, unpause, a policy update and the writer cutover,
// and its journal audits CONSISTENT.
func TestCALV0001_NonFixtureQueueTakesEveryWrite(t *testing.T) {
	repo := nonFixture(t, "ROADMAP")
	created := mutate(t, repo, envelope("create-a", mutation.OpCreate, "", "", createPayload("a")))
	if created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create: %+v", created)
	}
	edited := mutate(t, repo, envelope("edit-a", mutation.OpPrioritize, created.Ticket, "1", obj("priority", str("P1"), "order", str("0"))))
	if edited.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("edit: %+v", edited)
	}
	imported, err := store.Import(context.Background(), repo, operator(), fixture.QueueID, importExport(importItem("BF-1", "one\n", nil)), now(t))
	if err != nil || len(imported.Batches) != 1 {
		t.Fatalf("import: %+v %v", imported, err)
	}
	for _, op := range []string{transaction.Pause, transaction.Unpause} {
		if report := changeBarrier(t, repo, op, strings.ToLower(op)); report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: %+v", op, report)
		}
	}
	policy, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-2", "1", policyVersion("2")), now(t))
	if err != nil || policy.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", policy, err)
	}
	if report := cutover(t, repo, operator(), "decision-1"); report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("cutover: %+v", report)
	}
	auditOK(t, repo)
}

// TestCALV0001_ImportMappedQueueStaysRefused: a non-fixture queue that names
// an import map or an execution cutover cannot be initialized: init refuses
// it VALIDATION_FAILED MALFORMED.
func TestCALV0001_ImportMappedQueueStaysRefused(t *testing.T) {
	enabled := obj("enabledBy", str("owner"), "decisionRef", str("decision-1"), "gateEvidence", wire.Strings([]string{string(wire.Sum([]byte("run")))}))
	for field, value := range map[string]wire.Value{
		"importMapSha256":  str(string(wire.Sum([]byte("map")))),
		"executionCutover": enabled,
	} {
		t.Run(field, func(t *testing.T) {
			repo := fixture.TempRepo(t)
			q := fixture.QueueValue()
			q.Obj.Set("fixture", wire.Bool(false))
			q.Obj.Set(field, value)
			fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), wire.EncodeFile(q))
			fixture.Write(t, filepath.Join(repo.IntentDir, "policy.json"), fixture.PolicyBytes())
			resolved, err := intent.Resolve(repo.Root)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			report, err := store.Init(context.Background(), resolved, operator(), "req-init", now(t))
			if err != nil {
				t.Fatal(err)
			}
			refusedWith(t, report, mutation.OutcomeValidationFailed, wire.CodeMalformed)
		})
	}
}

// TestCALV0002_NonFixtureQueueRefusesClaims: before an execution cutover a
// non-fixture queue refuses claim and claim --next BLOCKED CUTOVER_MISSING and
// writes nothing, while its tickets still take writes.
func TestCALV0002_NonFixtureQueueRefusesClaims(t *testing.T) {
	s := leaseStoreOn(t, nonFixture(t, "NATIVE"))
	id := s.planned(t, "one", "P1", "src/")
	before := storeDigest(t, s.repo)
	refusedWith(t, s.lease(t, "claim-1", claimOf(id), 0, nil), mutation.OutcomeBlocked, wire.CodeCutoverMissing)
	refusedWith(t, s.lease(t, "next-1", claimNext, 0, nil), mutation.OutcomeBlocked, wire.CodeCutoverMissing)
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused claim wrote")
	}
	s.planned(t, "two", "P2", "docs/")
}
