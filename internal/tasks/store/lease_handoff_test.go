package store_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func handoffPolicyUpdate(t *testing.T, s *leaseStore, change func(wire.Value)) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, ".taskman", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := v.Obj.Get("policyVersion")
	n, err := strconv.Atoi(version.Str)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("policyVersion", wire.String(strconv.Itoa(n+1)))
	change(v)
	r, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest(fmt.Sprintf("handoff-policy-%d", n+1), version.Str, wire.EncodeFile(v)), now(t))
	if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy update: %+v %v", r, err)
	}
	s.t0 = now(t)
}

func handoffPoolStore(t *testing.T) *leaseStore {
	t.Helper()
	s := newGateStore(t)
	handoffPolicyUpdate(t, s, func(v wire.Value) {
		v.Obj.Set("pools", wire.Array(obj("id", str("lanes"), "members", wire.Strings([]string{"a", "b"})), obj("id", str("other"), "members", wire.Strings([]string{"c"}))))
	})
	return s
}

// CAL-V0-044/046: genuine policy receipts, allocation and renewal afterimages
// bind the complete interval. Restoring relevant endpoints cannot erase it.
func TestCALV0044_HandoffPolicyReceiptInterval(t *testing.T) {
	for _, name := range []string{"other-reservation", "version-no-pool", "members-add", "other-pool", "pools-add", "capacity", "budget", "retry", "gate", "roles", "environment", "cem"} {
		for _, candidate := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/candidate=%v", name, candidate), func(t *testing.T) {
				s := handoffPoolStore(t)
				id := s.ticket(t, "interval")
				if name == "other-reservation" && !candidate {
					for i := range 2 {
						failed := s.claim(t, fmt.Sprintf("failed-%d", i), id, 0, "src/")
						l := releaseOf(failed)
						l.Reason = wire.CodeGateFailed
						if r := s.lease(t, fmt.Sprintf("failure-%d", i), l, 0, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
							t.Fatalf("failure control: %+v", r)
						}
					}
				}
				claim := claimOf(id, "src/")
				claim.Stage = "review"
				if name != "version-no-pool" {
					claim.Pool = "lanes"
				}
				a := s.lease(t, "claim", claim, 0, nil)
				if a.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("claim: %+v", a)
				}
				if candidate {
					handoffSubmit(t, s, a, "submit", 0)
				}
				original, err := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, ".taskman", "policy.json"))
				if err != nil {
					t.Fatal(err)
				}
				oldAttempt := s.attempt(t, a.AttemptID)
				handoffPolicyUpdate(t, s, func(v wire.Value) {
					pools, _ := v.Obj.Get("pools")
					switch name {
					case "other-reservation":
						pools.Arr[0].Obj.Set("reservedFor", obj("b", str("implement")))
					case "members-add":
						pools.Arr[0].Obj.Set("members", wire.Strings([]string{"a", "b", "d"}))
					case "other-pool":
						pools.Arr[1].Obj.Set("reservedFor", obj("c", str("review")))
					case "pools-add":
						v.Obj.Set("pools", wire.Array(pools.Arr[0], pools.Arr[1], obj("id", str("third"), "members", wire.Strings([]string{"e"}))))
					case "capacity":
						x, _ := v.Obj.Get("capacity")
						x.Obj.Set("maxWorkersTotal", str("5"))
					case "budget":
						x, _ := v.Obj.Get("budgets")
						x.Obj.Set("ticketMultiplier", str("5"))
					case "retry":
						x, _ := v.Obj.Get("retries")
						x.Obj.Set("admissionsPerRevision", str("4"))
					case "gate":
						x, _ := v.Obj.Get("gates")
						x.Arr[0].Obj.Set("timeoutSeconds", str("31"))
					case "roles":
						v.Obj.Set("roles", obj("IMPORTER", wire.Strings([]string{"CREATE"})))
					case "environment":
						x, _ := v.Obj.Get("environment")
						x.Obj.Set("allowedEnvKeys", wire.Strings([]string{"PATH"}))
					case "cem":
						v.Obj.Set("cemRequired", wire.Bool(true))
					}
				})
				// Renewal preserves the original policy and cannot move the start
				// of the interval past the incompatible intermediate afterimage.
				r := s.lease(t, "renew", renewOf(a), 0, nil)
				if r.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("renew: %+v", r)
				}
				handoffPolicyUpdate(t, s, func(v wire.Value) {
					base, _ := wire.Parse(original)
					version, _ := v.Obj.Get("policyVersion")
					*v.Obj = *base.Obj
					v.Obj.Set("policyVersion", version)
				})
				before := fixture.TreeSnapshot(t, s.repo.StateDir)
				auditOK(t, s.repo)
				if !fixture.SameTree(before, fixture.TreeSnapshot(t, s.repo.StateDir)) {
					t.Fatal("audit mutated state")
				}
				l := releaseOf(a)
				l.Reason = wire.CodeReviewReturned
				if !candidate {
					l.Evidence = "external-review-result"
				}
				r = s.lease(t, "return", l, 0, nil)
				got := s.attempt(t, a.AttemptID)
				if name == "other-reservation" || name == "version-no-pool" {
					if r.Outcome.Outcome != mutation.OutcomeCompleted || got.Phase != "CANCELLED" || got.Quiescence != "FENCED" || got.RetryAccounting.Disposition != wire.CodeReviewReturned || got.RetryCount != oldAttempt.RetryCount || got.PolicySha256 != oldAttempt.PolicySha256 || got.ConfigSha256 != oldAttempt.ConfigSha256 || len(s.entries(t)) != 0 {
						t.Fatalf("clean return: %+v %+v", r, got)
					}
					if replay := s.lease(t, "return", l, 0, nil); replay.Kind != "Replay" {
						t.Fatalf("replay: %+v", replay)
					}
					next := handoffClaim(t, s, id, "implement", "successor", 0)
					fresh := s.attempt(t, next.AttemptID)
					if fresh.RetryCount != oldAttempt.RetryCount || fresh.PolicySha256 == oldAttempt.PolicySha256 || fresh.CandidateTreeOid != nil || len(fresh.GateResults) != 0 || fresh.RetryAccounting.Disposition != "NONE" {
						t.Fatalf("successor retained generation state: %+v", fresh)
					}
				} else {
					refusedWith(t, r, mutation.OutcomeBlocked, wire.CodeStalePolicy)
					if got.RetryAccounting.Disposition != "NONE" || got.PolicySha256 != oldAttempt.PolicySha256 || len(s.entries(t)) != 1 {
						t.Fatalf("refusal changed accounting: %+v", got)
					}
				}
				auditOK(t, s.repo)
			})
		}
	}
}

func TestCALV0044_ConcurrentCompatibleReturnsCommitOnce(t *testing.T) {
	s := handoffPoolStore(t)
	id := s.ticket(t, "concurrent")
	c := claimOf(id, "src/")
	c.Stage, c.Pool = "review", "lanes"
	a := s.lease(t, "claim", c, 0, nil)
	handoffPolicyUpdate(t, s, func(v wire.Value) {
		p, _ := v.Obj.Get("pools")
		p.Arr[0].Obj.Set("reservedFor", obj("b", str("review")))
	})
	l := releaseOf(a)
	l.Reason, l.Evidence = wire.CodeReviewReturned, "external-review"
	reports := race(t, func() (*store.Report, error) { return s.try("return-a", l, s.at(t, 0)) }, func() (*store.Report, error) { return s.try("return-b", l, s.at(t, 0)) })
	completed := 0
	for _, r := range reports {
		if r.Outcome.Outcome == mutation.OutcomeCompleted {
			completed++
		} else if !r.Outcome.HasCode(wire.CodeFenced) {
			t.Fatalf("losing return: %+v", r)
		}
	}
	if completed != 1 || len(s.entries(t)) != 0 {
		t.Fatalf("committed %d returns", completed)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", a.AttemptID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	refusedWith(t, s.lease(t, "old-renew", renewOf(a), 0, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	after, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", a.AttemptID+".json"))
	if !bytes.Equal(raw, after) {
		t.Fatal("fenced holder mutated attempt")
	}
	s.consistent(t)
}

// The moving-policy racer must either follow a committed return or make the
// return stale. A prepared compatibility observation cannot cross that commit.
func TestCALV0044_ConcurrentRelevantPolicyAndReturn(t *testing.T) {
	s := handoffPoolStore(t)
	id := s.ticket(t, "policy-race")
	c := claimOf(id, "src/")
	c.Stage, c.Pool = "review", "lanes"
	a := s.lease(t, "claim", c, 0, nil)
	handoffPolicyUpdate(t, s, func(v wire.Value) {
		p, _ := v.Obj.Get("pools")
		p.Arr[0].Obj.Set("reservedFor", obj("b", str("implement")))
	})
	raw, err := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, ".taskman", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("policyVersion", str("5")).Set("cemRequired", wire.Bool(true))
	l := releaseOf(a)
	l.Reason, l.Evidence = wire.CodeReviewReturned, "external-review"
	at := s.at(t, 0)
	reports := race(t,
		func() (*store.Report, error) { return s.try("return", l, at) },
		func() (*store.Report, error) {
			return store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("relevant-policy", "4", wire.EncodeFile(v)), at)
		},
	)
	returned, policy := reports[0], reports[1]
	if policy.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy racer: %+v", policy)
	}
	if returned.Outcome.Outcome == mutation.OutcomeCompleted {
		if returned.Outcome.ReceiptSeq.Uint64() >= policy.Outcome.ReceiptSeq.Uint64() {
			t.Fatal("stale compatibility crossed the policy commit")
		}
	} else {
		refusedWith(t, returned, mutation.OutcomeBlocked, wire.CodeStalePolicy)
	}
	s.consistent(t)
}

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
