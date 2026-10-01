package store_test

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"testing"
)

func handoffClaim(t *testing.T, s *leaseStore, id, stage, request string, minute int) *store.Report {
	t.Helper()
	l := claimOf(id, "src/")
	l.Stage, l.Holder = stage, "holder-"+request
	r := s.lease(t, request, l, minute, nil)
	if r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("claim: %+v", r)
	}
	return r
}
func handoffSubmit(t *testing.T, s *leaseStore, a *store.Report, request string, minute int) {
	t.Helper()
	tree := gitOut(t, s.root, "rev-parse", "HEAD^{tree}")
	r := s.lease(t, request, submitOf(a, tree), minute, nil)
	if r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit: %+v", r)
	}
}

// CAL-V0-044: scope-checked handoffs exceed three generations while preserving debt.
func TestCALV0044_CleanHandoffsPreserveRetryDebt(t *testing.T) {
	t.Run("CAL-V0-044 CleanHandoffsPreserveRetryDebt", func(t *testing.T) {
		for _, debt := range []int{0, 2, 3} {
			t.Run(fmt.Sprint(debt), func(t *testing.T) {
				s := newGateStore(t)
				id := s.ticket(t, "handoff")
				for i := 0; i < debt; i++ {
					a := s.claim(t, fmt.Sprintf("debt-%d", i), id, 0, "src/")
					l := releaseOf(a)
					l.Reason = wire.CodeGateFailed
					s.lease(t, fmt.Sprintf("cancel-%d", i), l, 0, nil)
				}
				for i := 0; i < 6; i++ {
					stage, reason := "implement", wire.CodeHandoff
					if i%2 == 1 {
						stage, reason = "review", wire.CodeReviewReturned
					}
					a := handoffClaim(t, s, id, stage, fmt.Sprintf("claim-%d", i), 1)
					current := s.attempt(t, a.AttemptID)
					if current.RetryCount.Int() != int64(debt) || len(current.GateResults) != 0 || current.CandidateTreeOid != nil || current.RetryAccounting.Disposition != "NONE" {
						t.Fatalf("fresh generation: %+v", current)
					}
					handoffSubmit(t, s, a, fmt.Sprintf("submit-%d", i), 1)
					l := releaseOf(a)
					l.Reason = reason
					r := s.lease(t, fmt.Sprintf("handoff-%d", i), l, 1, nil)
					if r.Outcome.Outcome != mutation.OutcomeCompleted {
						t.Fatalf("handoff: %+v", r)
					}
					if s.attempt(t, a.AttemptID).RetryAccounting.Disposition != reason {
						t.Fatal("missing writer disposition")
					}
					if replay := s.lease(t, fmt.Sprintf("handoff-%d", i), l, 1, nil); replay.Kind != "Replay" {
						t.Fatalf("replay: %+v", replay)
					}
					l.Reason = wire.CodeContaminated
					refusedWith(t, s.lease(t, fmt.Sprintf("handoff-%d", i), l, 1, nil), mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
				}
				a := handoffClaim(t, s, id, "implement", "final", 2)
				l := releaseOf(a)
				l.Reason = wire.CodeBootTimeout
				s.lease(t, "failure", l, 2, nil)
				if debt == 3 {
					refusedWith(t, s.lease(t, "exhausted", claimOf(id, "src/"), 2, nil), mutation.OutcomeBlocked, wire.CodeRetryExhausted)
				} else {
					next := s.claim(t, "retry", id, 2, "src/")
					if s.attempt(t, next.AttemptID).RetryCount.Int() != int64(debt+1) {
						t.Fatal("failure debt lost")
					}
				}
				if s.record(t, id).AcceptanceRevision != "1" {
					t.Fatal("acceptance changed")
				}
				auditOK(t, s.repo)
			})
		}
	})
}

// CAL-V0-044: caller-selected stage and reason are not proof of clean work.
func TestCALV0044_HandoffNeedsRecordedEligibility(t *testing.T) {
	t.Run("CAL-V0-044 HandoffNeedsRecordedEligibility", func(t *testing.T) {
		for _, name := range []string{"no-submit", "no-stage", "returned-by-author", "expired", "failed-gate"} {
			t.Run(name, func(t *testing.T) {
				s := newGateStore(t)
				id := s.ticket(t, "refusal")
				stage := "implement"
				if name == "no-stage" {
					stage = ""
				}
				a := handoffClaim(t, s, id, stage, "claim", 0)
				if name != "no-submit" {
					handoffSubmit(t, s, a, "submit", 1)
				}
				if name == "failed-gate" {
					s.passes(t, "failed-run", gateOf(a, "fails"), 2)
				}
				l := releaseOf(a)
				l.Reason = wire.CodeHandoff
				if name == "returned-by-author" {
					l.Reason = wire.CodeReviewReturned
				}
				minute := 3
				if name == "expired" {
					minute = 60
				}
				r := s.lease(t, "handoff", l, minute, nil)
				if r.Outcome.Outcome == mutation.OutcomeCompleted {
					t.Fatalf("unsafe handoff: %+v", r)
				}
				got := s.attempt(t, a.AttemptID)
				if got.RetryAccounting.Disposition != "NONE" || got.Phase == "CANCELLED" {
					t.Fatalf("refusal changed attempt: %+v", got)
				}
				auditOK(t, s.repo)
			})
		}
	})
}

// CAL-V0-044: replacing a failed gate result with PASS and resubmitting does not refund it.
func TestCALV0044_FailedGateRemainsChargedAfterPassAndSubmit(t *testing.T) {
	t.Run("CAL-V0-044 FailedGateRemainsChargedAfterPassAndSubmit", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "c412-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		flag := filepath.Join(dir, "pass")
		s := newLeaseStore(t, commandGate("verify", fmt.Sprintf("test -f %q", flag), "30", true))
		id := s.ticket(t, "sticky")
		a := handoffClaim(t, s, id, "review", "claim", 0)
		handoffSubmit(t, s, a, "submit", 1)
		s.passes(t, "fails", gateOf(a, "verify"), 2)
		if err := os.WriteFile(flag, []byte("pass"), 0600); err != nil {
			t.Fatal(err)
		}
		s.passes(t, "passes", gateOf(a, "verify"), 3)
		if s.results(t, a.AttemptID)["verify"].State != "PASSED" {
			t.Fatal("positive gate control failed")
		}
		_, tree := s.commit(t, "src/revised.go")
		s.lease(t, "resubmit", submitOf(a, tree), 4, nil)
		l := releaseOf(a)
		l.Reason = wire.CodeReviewReturned
		refusedWith(t, s.lease(t, "return", l, 5, nil), mutation.OutcomeBlocked, wire.CodeMissingEvidence)
		l.Reason = wire.CodeGateFailed
		s.lease(t, "release", l, 5, nil)
		next := handoffClaim(t, s, id, "implement", "retry", 6)
		if x := s.attempt(t, next.AttemptID); x.RetryCount != "1" || x.RetryAccounting.FailedOrUnknown {
			t.Fatalf("next generation: %+v", x)
		}
		auditOK(t, s.repo)
	})
}

// CAL-V0-044: review-stage crashes remain subject to ordinary retry limits.
func TestCALV0044_ReviewExpiryStillExhausts(t *testing.T) {
	t.Run("CAL-V0-044 ReviewExpiryStillExhausts", func(t *testing.T) {
		s := newLeaseStore(t)
		id := s.ticket(t, "expiry")
		for i := 0; i <= transaction.MaxRetries; i++ {
			a := handoffClaim(t, s, id, "review", fmt.Sprintf("claim-%d", i), i*61)
			l := releaseOf(a)
			l.Reason = wire.CodeHandoff
			r := s.lease(t, fmt.Sprintf("late-%d", i), l, i*61+60, nil)
			if !r.Outcome.HasCode(wire.CodeFenced) {
				t.Fatalf("late release: %+v", r)
			}
		}
		refusedWith(t, s.lease(t, "exhausted", claimOf(id, "src/"), 4*61, nil), mutation.OutcomeBlocked, wire.CodeRetryExhausted)
		auditOK(t, s.repo)
	})
}

// CAL-V0-044: TIMEOUT remains a failure after a passing gate and resubmission.
func TestCALV0044_TimeoutRemainsSticky(t *testing.T) {
	t.Run("CAL-V0-044 TimeoutRemainsSticky", func(t *testing.T) {
		s := newGateStore(t)
		id := s.ticket(t, "timeout")
		a := handoffClaim(t, s, id, "review", "claim", 0)
		handoffSubmit(t, s, a, "submit", 1)
		s.passes(t, "timeout", gateOf(a, "slow"), 2)
		if s.results(t, a.AttemptID)["slow"].OutcomeClass != "TIMEOUT" {
			t.Fatal("timeout control failed")
		}
		s.passes(t, "pass", gateOf(a, "verify"), 3)
		handoffSubmit(t, s, a, "submit-again", 4)
		l := releaseOf(a)
		l.Reason = wire.CodeReviewReturned
		refusedWith(t, s.lease(t, "return", l, 5, nil), mutation.OutcomeBlocked, wire.CodeMissingEvidence)
		if !s.attempt(t, a.AttemptID).RetryAccounting.FailedOrUnknown {
			t.Fatal("timeout cleared")
		}
		auditOK(t, s.repo)
	})
}

func TestCALV0046_NoTreeHandoffAndIntegrate(t *testing.T) {
	for _, stage := range []string{"implement", "review", "integrate"} {
		t.Run(stage, func(t *testing.T) {
			s := newLeaseStore(t)
			id := s.ticket(t, "external")
			setRetryPolicy(t, s, 0, 0)
			a := handoffClaim(t, s, id, stage, "claim", 0)
			l := releaseOf(a)
			l.Reason = wire.CodeHandoff
			if stage == "review" {
				l.Reason = wire.CodeReviewReturned
			}
			l.Evidence = "local:external-review"
			r := s.lease(t, "handoff", l, 1, nil)
			if r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("handoff %+v", r)
			}
			got := s.attempt(t, a.AttemptID)
			if got.HandoffEvidence != l.Evidence || got.CandidateTreeOid != nil || got.Phase != "CANCELLED" || got.Quiescence != "FENCED" {
				t.Fatalf("terminal %+v", got)
			}
			if replay := s.lease(t, "handoff", l, 1, nil); replay.Kind != "Replay" {
				t.Fatalf("replay %+v", replay)
			}
			l.Evidence = "local:different"
			refusedWith(t, s.lease(t, "handoff", l, 1, nil), mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
			next := handoffClaim(t, s, id, stage, "successor", 2)
			current := s.attempt(t, next.AttemptID)
			if current.RetryCount != "0" || current.HandoffEvidence != "" || current.CandidateTreeOid != nil {
				t.Fatalf("successor inherited evidence/debt: %+v", current)
			}
			// With budget zero, a later charged cancellation still exhausts the ticket.
			s.lease(t, "charged", releaseOf(next), 2, nil)
			refusedWith(t, s.lease(t, "exhausted", claimOf(id, "src/"), 2, nil), mutation.OutcomeBlocked, wire.CodeRetryExhausted)
			auditOK(t, s.repo)
		})
	}
	s := newGateStore(t)
	id := s.ticket(t, "integrator")
	a := handoffClaim(t, s, id, "integrate", "claim", 0)
	handoffSubmit(t, s, a, "submit", 1)
	l := releaseOf(a)
	l.Reason = wire.CodeHandoff
	if r := s.lease(t, "tree-handoff", l, 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("integrate tree handoff: %+v", r)
	}
}

func TestCALV0046_NoTreeRefusals(t *testing.T) {
	for _, name := range []string{"candidate", "no-stage", "review-return", "policy-changed", "expired", "generation"} {
		t.Run(name, func(t *testing.T) {
			s := newGateStore(t)
			id := s.ticket(t, "external")
			stage := "implement"
			if name == "no-stage" {
				stage = ""
			}
			a := handoffClaim(t, s, id, stage, "claim", 0)
			code := wire.CodeMissingEvidence
			l := releaseOf(a)
			l.Reason = wire.CodeHandoff
			l.Evidence = "local:external"
			minute := 1
			switch name {
			case "candidate":
				handoffSubmit(t, s, a, "submit", 0)
			case "no-stage":
				code = wire.CodeTicketState
			case "review-return":
				l.Reason = wire.CodeReviewReturned
				code = wire.CodeTicketState
			case "policy-changed":
				setRetryPolicy(t, s, 4, 0)
				code = wire.CodeStalePolicy
			case "expired":
				minute = 60
				code = wire.CodeFenced
			case "generation":
				l.Generation = wire.SizeOf(l.Generation.Uint64() + 1)
				code = wire.CodeFenced
			}
			r := s.lease(t, "handoff", l, minute, nil)
			if !r.Outcome.HasCode(code) {
				t.Fatalf("expected %s: %+v", code, r)
			}
			got := s.attempt(t, a.AttemptID)
			if got.HandoffEvidence != "" || got.RetryAccounting.Disposition != "NONE" {
				t.Fatalf("refusal changed clean evidence: %+v", got)
			}
			auditOK(t, s.repo)
		})
	}
}
