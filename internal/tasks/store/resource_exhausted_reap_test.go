//go:build darwin || linux

package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type exhaustionReapReader func([]byte) (int, error)

func (f exhaustionReapReader) Read(p []byte) (int, error) { return f(p) }

// CAL-V0-215: automatic health can discover a newly expired unrelated lease.
// The next claim round must retain health's retry prohibition after reaping it.
func TestCALV0215_AutomaticHealthExpiryReapExhaustion(t *testing.T) {
	for _, fault := range []error{syscall.EMFILE, syscall.ENFILE, wire.Errorf(wire.CodeLockTimeout, "claim", "injected")} {
		t.Run(fault.Error(), func(t *testing.T) {
			repo, oldChoice := inventoryClaimRepo(t)
			ctx := context.Background()
			at := wire.Timestamp(fixture.Timestamp)
			marker := filepath.Join(t.TempDir(), "health-ran")
			policy := fixture.PolicyValue()
			policy.Obj.Set("policyVersion", wire.String("2"))
			policy.Obj.Set("capacity", historyObject("maxActiveAttempts", wire.String("4"), "maxWorkersTotal", wire.String("4"), "classes", wire.Array()))
			policy.Obj.Vals["budgets"].Obj.Set("requireEnforcedFields", wire.Array())
			health := historyObject("argv", wire.Strings([]string{"/bin/sh", "-c", "printf x >> " + marker}), "cwd", wire.String("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", wire.String("3"))
			policy.Obj.Set("pools", wire.Array(historyObject("id", wire.String("db"), "members", wire.Strings([]string{"a"}), "memberConfig", historyObject("a", historyObject("health", health)))))
			if report, err := PolicyUpdate(ctx, repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "health-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(policy)}, at); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("fixture policy: %+v %v", report, err)
			}
			next, err := wire.Parse(historyCreate("next-ticket", "next"))
			if err != nil {
				t.Fatal(err)
			}
			next.Obj.Vals["payload"].Obj.Vals["effects"].Obj.Set("touchPaths", wire.Strings([]string{"next/"}))
			if report, err := Mutate(ctx, repo, historyActor, wire.EncodeFile(next), at); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("fixture ticket: %+v %v", report, err)
			}
			oldChoice.Lease.LeaseMinutes = "5"
			old, err := Lease(ctx, repo, historyActor, oldChoice, at)
			if err != nil || old.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("fixture lease: %+v %v", old, err)
			}
			proof, err := readLeaseProof(ctx, repo)
			if err != nil {
				t.Fatalf("fixture audit: %v", err)
			}
			if a, ok := lockedAttempt(proof, old.AttemptID); !ok || !a.Live() || a.Lease == nil || a.Lease.ExpiresAt <= at || a.Lease.ExpiresAt >= WallClock() {
				t.Fatalf("fixture lease must be live initially and expired at the health clock: %+v", a)
			}

			var reap transaction.Artifact
			restore := SetPublishFaultForTest(func(a transaction.Artifact) error {
				if a.Role == "RECEIPT" {
					r, err := snapshot.DecodeReceipt(a.Data)
					if err != nil {
						t.Fatal(err)
					}
					if r.RequestID != nil && *r.RequestID == reapID(transaction.ExpiredLease{AttemptID: old.AttemptID, Generation: old.Generation}) {
						if r.Kind != "TRANSITION" || r.Outcome != mutation.OutcomeCompleted || r.AttemptID == nil || *r.AttemptID != old.AttemptID || r.Generation == nil || *r.Generation != old.Generation {
							t.Fatalf("reap did not complete before fault injection: %+v", r)
						}
						reap = a
					}
				}
				return nil
			})
			t.Cleanup(restore)
			previousEntropy := attemptEntropy
			t.Cleanup(func() { attemptEntropy = previousEntropy })
			fired := 0
			attemptEntropy = exhaustionReapReader(func(p []byte) (int, error) {
				if reap.Target == "" {
					return previousEntropy.Read(p)
				}
				// A fresh claim round reaches entropy only after the reap returns.
				if raw, err := os.ReadFile(filepath.Join(repo.StateDir, reap.Target)); err != nil || !bytes.Equal(raw, reap.Data) {
					t.Fatalf("reap receipt not durable: %v", err)
				}
				fired++
				return 0, &os.PathError{Op: "read", Path: "attempt-entropy", Err: fault}
			})
			// Initial selection sees a live lease. Automatic health uses the live
			// clock, then the outer reap and subsequent round use that clock too.
			ctx = WithClock(ctx, func() wire.Timestamp {
				if _, err := os.Stat(marker); os.IsNotExist(err) {
					return at
				}
				return WallClock()
			})
			choice := oldChoice
			choice.RequestID = "health-next"
			choice.Lease = transaction.LeaseRequest{Verb: transaction.LeaseClaimNext, Holder: "agent-2", LeaseMinutes: "60", Stage: "implement", Pool: "db", Scope: []string{"next/"}}
			report, err := Lease(ctx, repo, historyActor, choice, at)
			restore()
			attemptEntropy = previousEntropy
			if fired != 1 || !errors.Is(err, fault) || wire.CodeOf(err) != wire.CodeOf(fault) || report == nil || !report.Unretryable {
				t.Fatalf("fired=%d report=%+v err=%v", fired, report, err)
			}
			if len(report.Reaped) != 1 || report.Reaped[0].AttemptID != old.AttemptID || len(report.ReapReceipts) != 1 {
				t.Fatalf("missing reap evidence: %+v", report)
			}
			if raw, err := os.ReadFile(marker); err != nil || string(raw) != "x" {
				t.Fatalf("health did not run exactly once: %q %v", raw, err)
			}
			proof, err = readLeaseProof(ctx, repo)
			if err != nil {
				t.Fatalf("final audit: %v", err)
			}
			// CAL-V0-011 retains the historical lease on the terminal attempt;
			// reaping fences it and removes its reservation, not its lease field.
			if a, ok := lockedAttempt(proof, old.AttemptID); !ok || a.Phase != "FAILED" || a.Quiescence != "FENCED" || a.Cause == nil || *a.Cause != snapshot.CauseLeaseExpired {
				t.Fatalf("unrelated lease was not reaped: %+v", a)
			}
			// An absent reservations record holds no entries for the reaped attempt.
			reservations, err := snapshot.DecodeReservations(proof.Records["reservations.json"].Raw)
			if len(proof.Records["reservations.json"].Raw) == 0 {
				reservations, err = &snapshot.ReservationSet{}, nil
			}
			if err != nil {
				t.Fatalf("final reservations: %v", err)
			}
			for _, entry := range reservations.Entries {
				if entry.AttemptID == old.AttemptID {
					t.Fatalf("reaped attempt still reserves resources: %+v", entry)
				}
			}
		})
	}
}
