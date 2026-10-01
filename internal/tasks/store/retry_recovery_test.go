package store_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func exhaustedCancelled(t *testing.T, s *leaseStore, id string) *store.Report {
	t.Helper()
	var last *store.Report
	for i := 0; i <= transaction.MaxRetries; i++ {
		last = s.claim(t, fmt.Sprintf("claim-%d", i), id, i, "src")
		r := s.lease(t, fmt.Sprintf("release-%d", i), releaseOf(last), i, nil)
		if r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("release: %+v", r)
		}
	}
	refusedWith(t, s.lease(t, "exhausted", claimOf(id, "src"), 4, nil), mutation.OutcomeBlocked, wire.CodeRetryExhausted)
	return last
}

func TestCALV0043_OwnerReopensExhaustedCancelledTicket(t *testing.T) {
	t.Run("CAL-V0-013 CAL-V0-043 owner readmission", func(t *testing.T) {
		s := newLeaseStore(t)
		id := s.ticket(t, "recovery")
		old := exhaustedCancelled(t, s, id)
		before, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", old.AttemptID+".json"))
		if err != nil {
			t.Fatal(err)
		}
		req := envelope("recover", mutation.OpReopen, id, "1", obj("reason", str("owner permits a fresh attempt after cancellations")))
		r, err := store.Mutate(context.Background(), s.repo, operator(), req, s.at(t, 5))
		if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("owner recovery: %+v %v", r, err)
		}
		if *r.Outcome.ResultingRevision != "2" || *r.Outcome.ResultingAcceptanceRevision != "2" {
			t.Fatalf("revision: %+v", r)
		}
		replay, err := store.Mutate(context.Background(), s.repo, operator(), req, s.at(t, 5))
		if err != nil || !replay.Outcome.Replayed || replay.Receipt != "" {
			t.Fatalf("replay: %+v %v", replay, err)
		}
		conflict, err := store.Mutate(context.Background(), s.repo, operator(), envelope("recover", mutation.OpReopen, id, "1", obj("reason", str("different"))), s.at(t, 5))
		if err != nil || conflict.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatalf("conflicting replay: %+v %v", conflict, err)
		}
		evidence, err := os.ReadFile(filepath.Join(s.repo.StateDir, "evidence", string(wire.Sum(req))))
		if err != nil || !bytes.Equal(evidence, req) {
			t.Fatalf("request reason not retained: %v", err)
		}
		receiptRaw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", r.Receipt))
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := snapshot.DecodeReceipt(receiptRaw)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, post := range receipt.Post {
			if post.Path == "evidence/"+string(wire.Sum(req)) {
				found = true
			}
		}
		if !found || receipt.ActorID != "tester" || receipt.ActorRole != "OWNER" || receipt.ExpectedRevision == nil || *receipt.ExpectedRevision != "1" {
			t.Fatalf("reason evidence not receipt-bound: %+v", receipt)
		}
		fresh := s.claim(t, "fresh", id, 6, "src")
		a := s.attempt(t, fresh.AttemptID)
		if fresh.AttemptID == old.AttemptID || a.RetryCount != "0" || a.TicketRevision != "2" {
			t.Fatalf("fresh attempt: %+v", a)
		}
		after, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", old.AttemptID+".json"))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("prior attempt changed: %v", err)
		}
		auditOK(t, s.repo)
	})
}

func TestCALV0043_RecoveryRefusesStaleAndTamperedAttempts(t *testing.T) {
	t.Run("CAL-V0-043 canonical journal authority", func(t *testing.T) {
		s := newLeaseStore(t)
		id := s.ticket(t, "tamper")
		old := exhaustedCancelled(t, s, id)
		r, err := store.Mutate(context.Background(), s.repo, operator(), envelope("stale", mutation.OpReopen, id, "2", obj("reason", str("recover"))), s.at(t, 5))
		if err != nil || r.Outcome.Outcome != mutation.OutcomeRevisionConflict {
			t.Fatalf("stale: %+v %v", r, err)
		}
		a := s.attempt(t, old.AttemptID)
		a.RetryCount = "2"
		a.RetryReasons = map[string]wire.Count{"EXPIRED": "0", "RELEASED": "2", "FAILED": "0", "UNKNOWN": "0"}
		raw, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		fixture.Write(t, filepath.Join(s.repo.StateDir, "attempts", old.AttemptID+".json"), raw)
		r, err = store.Mutate(context.Background(), s.repo, operator(), envelope("tampered", mutation.OpReopen, id, "1", obj("reason", str("recover"))), s.at(t, 5))
		if err == nil && r.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatal("schema-valid unaudited attempt admitted")
		}
		if r.Receipt != "" {
			t.Fatal("tamper refusal wrote receipt")
		}
	})
}

func TestCALV0043_RecoveryPreservesRequiredGateFailures(t *testing.T) {
	t.Run("CAL-V0-043 fresh acceptance needs fresh gates", func(t *testing.T) {
		s := newLeaseStore(t, commandGate("verify", "test -f pass", "30", true))
		id := s.ticket(t, "gates")
		commit, tree := s.commit(t, "src")
		var old *store.Report
		for i := 0; i <= transaction.MaxRetries; i++ {
			old = s.claim(t, fmt.Sprintf("claim-%d", i), id, i, "src")
			s.lease(t, fmt.Sprintf("submit-%d", i), submitOf(old, tree), i, nil)
			s.passes(t, fmt.Sprintf("failed-gate-%d", i), gateOf(old, "verify"), i)
			refusedWith(t, s.lease(t, fmt.Sprintf("complete-%d", i), completeOf(old, commit), i, nil), mutation.OutcomeBlocked, wire.CodeGateFailed)
			s.lease(t, fmt.Sprintf("release-%d", i), releaseOf(old), i, nil)
		}
		prior := s.attempt(t, old.AttemptID)
		if len(prior.GateResults) == 0 {
			t.Fatal("no retained failed gate")
		}
		r, err := store.Mutate(context.Background(), s.repo, operator(), envelope("recover-gates", mutation.OpReopen, id, "1", obj("reason", str("owner authorizes another attempt"))), s.at(t, 5))
		if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("recovery: %+v %v", r, err)
		}
		fresh := s.claim(t, "fresh-gate", id, 6, "src")
		s.lease(t, "fresh-submit", submitOf(fresh, tree), 6, nil)
		refusedWith(t, s.lease(t, "fresh-complete", completeOf(fresh, commit), 6, nil), mutation.OutcomeBlocked, wire.CodeMissingGate)
		if len(s.attempt(t, fresh.AttemptID).GateResults) != 0 {
			t.Fatal("inherited old gate")
		}
		// A tracked passing candidate still requires a newly run gate.
		commit, tree = s.commit(t, "pass")
		s.lease(t, "resubmit", submitOf(fresh, tree), 7, nil)
		s.passes(t, "fresh-pass", gateOf(fresh, "verify"), 7)
		done := s.lease(t, "fresh-done", completeOf(fresh, commit), 7, nil)
		if done.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("completion after gate: %+v", done)
		}
		if got := s.attempt(t, old.AttemptID); len(got.GateResults) != len(prior.GateResults) || got.GateResults[0] != prior.GateResults[0] {
			t.Fatal("old gate history changed")
		}
		auditOK(t, s.repo)
	})
}

func TestCALV0043_RecoveryInvalidatesOldApprovals(t *testing.T) {
	t.Run("CAL-V0-043 prior acceptance approvals cannot authorize new claim", func(t *testing.T) {
		s := newLeaseStore(t)
		payload := createPayload("approval")
		payload.Obj.Set("executionClass", str("APPROVAL_REQUIRED"))
		created := mutate(t, s.repo, envelope("create-approval", mutation.OpCreate, "", "", payload))
		id := created.Ticket
		grant := obj("grantId", str("old-run"), "actor", str("tester"), "operation", str("RUN"), "targetRevision", str("1"), "scope", wire.Strings(nil))
		r := mutate(t, s.repo, envelope("grant-old", mutation.OpGrantApproval, id, "1", grant))
		if r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("grant: %+v", r)
		}
		s.t0 = now(t)
		exhaustedCancelled(t, s, id)
		r, err := store.Mutate(context.Background(), s.repo, operator(), envelope("recover-approval", mutation.OpReopen, id, "2", obj("reason", str("retry approved, RUN still separately required"))), s.at(t, 5))
		if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("reopen: %+v %v", r, err)
		}
		refusedWith(t, s.lease(t, "old-grant-denied", claimOf(id, "src"), 6, nil), mutation.OutcomeBlocked, wire.CodeApprovalMissing)
		auditOK(t, s.repo)
	})
}

func TestCALV0043_RecoveryPublicationFaults(t *testing.T) {
	t.Run("CAL-V0-043 retained reason crash and redo", func(t *testing.T) {
		for _, phase := range []string{"before-receipt", "after-receipt"} {
			t.Run(phase, func(t *testing.T) {
				s := newLeaseStore(t)
				id := s.ticket(t, "crash")
				old := exhaustedCancelled(t, s, id)
				prior, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", old.AttemptID+".json"))
				req := envelope("crash-recover", mutation.OpReopen, id, "1", obj("reason", str("durable reason across recovery")))
				injected := errors.New("publication interrupted")
				restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
					if phase == "before-receipt" && a.Role == "RECEIPT" {
						return injected
					}
					if phase == "after-receipt" && a.Role == "POST" && strings.HasPrefix(a.Target, "intent/tickets/") {
						return injected
					}
					return nil
				})
				r, err := store.Mutate(context.Background(), s.repo, operator(), req, s.at(t, 5))
				restore()
				if err == nil || r.Outcome.Outcome == mutation.OutcomeCompleted && r.Receipt != "" && phase == "before-receipt" {
					t.Fatalf("fault not injected: %+v %v", r, err)
				}
				if phase == "before-receipt" && r.Receipt != "" {
					t.Fatal("uncommitted envelope treated as recovery")
				}
				recovered, err := store.Mutate(context.Background(), s.repo, operator(), req, s.at(t, 5))
				if err != nil || recovered.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("retry: %+v %v", recovered, err)
				}
				if phase == "after-receipt" && (!recovered.Redone || !recovered.Outcome.Replayed) {
					t.Fatalf("committed receipt not redone/replayed: %+v", recovered)
				}
				retained, err := os.ReadFile(filepath.Join(s.repo.StateDir, "evidence", string(wire.Sum(req))))
				if err != nil || !bytes.Equal(retained, req) {
					t.Fatal("reason changed on redo", err)
				}
				after, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", old.AttemptID+".json"))
				if !bytes.Equal(prior, after) {
					t.Fatal("history changed")
				}
				auditOK(t, s.repo)
			})
		}
	})
}
