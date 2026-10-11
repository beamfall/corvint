//go:build unix

package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-215: a receipt can commit before the first post file fails to open.
// A replay cannot finish these multi-step workflows, so they must forbid retry.
func TestCALV0215_CommittedPoolExhaustion(t *testing.T) {
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		for _, kind := range []string{"health", "sweep"} {
			t.Run(kind+"/"+errno.Error(), func(t *testing.T) {
				marker := filepath.Join(t.TempDir(), "ran")
				var s *leaseStore
				if kind == "sweep" {
					s, _ = psrFixture(t, "printf x >> "+marker, "printf ok", "ok", "1", false)
					if len(psrPools(t, s).Entries) != 1 {
						t.Fatal("fixture lacks quarantined member")
					}
				} else {
					s = newLeaseStore(t)
					v := fixture.PolicyValue()
					v.Obj.Set("policyVersion", str("3"))
					v.Obj.Vals["budgets"].Obj.Set("requireEnforcedFields", wire.Array())
					health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "printf x >> " + marker}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
					v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("health", health)))))
					if rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("health-policy", "2", wire.EncodeFile(v)), now(t)); e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
						t.Fatalf("fixture: %+v %v", rep, e)
					}
				}
				auditOK(t, s.repo)
				fired := 0
				restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
					if a.Role == "POST" {
						fired++
						return &os.PathError{Op: "open", Path: a.Target, Err: errno}
					}
					return nil
				})
				t.Cleanup(restore)
				var receipt string
				var forbidden bool
				var err error
				choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "exhausted", Root: s.root, Lease: transaction.LeaseRequest{Member: "a"}}
				sweep := store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "exhausted", Root: s.root, TimeoutSeconds: "15"}
				if kind == "health" {
					rep, e := store.PoolCommand(context.Background(), s.repo, operator(), choice, "health")
					err = e
					receipt = rep.Receipt
					forbidden = rep.Unretryable
				} else {
					rep, e := store.PoolSweep(context.Background(), s.repo, operator(), sweep)
					err = e
					receipt = rep.Report.Receipt
					forbidden = rep.Unretryable
				}
				restore()
				if fired != 1 || receipt == "" || !errors.Is(err, errno) || wire.CodeOf(err) != wire.CodeResourceExhausted || !forbidden {
					t.Fatalf("fault=%d receipt=%s forbidden=%v err=%v", fired, receipt, forbidden, err)
				}
				if _, e := os.Stat(marker); !os.IsNotExist(e) {
					t.Fatalf("program ran: %v", e)
				}
				if kind == "health" {
					rep, e := store.PoolCommand(context.Background(), s.repo, operator(), choice, "health")
					if e != nil || rep.Kind != "Replay" {
						t.Fatalf("replay: %+v %v", rep, e)
					}
				} else {
					rep, e := store.PoolSweep(context.Background(), s.repo, operator(), sweep)
					if e != nil || !rep.Pending {
						t.Fatalf("replay: %+v %v", rep, e)
					}
				}
				if _, e := os.Stat(marker); !os.IsNotExist(e) {
					t.Fatalf("replay ran program: %v", e)
				}
			})
		}
	}
}

// CAL-V0-215: DISPATCH receipt publication precedes later post/head writes;
// even a failed transition has left the reviewer's selectable phase.
func TestCALV0215_CommittedDispatchExhaustion(t *testing.T) {
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		for _, committed := range []bool{false, true} {
			boundary := "before-receipt"
			if committed {
				boundary = "after-receipt"
			}
			t.Run(errno.Error()+"/"+boundary, func(t *testing.T) {
				f := buildProgramFixture(t, true, false, nil)
				self, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
				if err != nil {
					t.Fatal(err)
				}
				a, err := w.RunRole(ctx, "implementer", "")
				if err != nil || a.Phase != "BUILT" {
					t.Fatalf("fixture: %+v %v", a, err)
				}
				aID := a.AttemptID
				auditOK(t, f.s.repo)
				armed, fired := false, 0
				var receipt transaction.Artifact
				restoreRun := store.SetRunFaultForTest(f.s.repo.StateDir, func(point string) error {
					if point == "transition:DISPATCH" {
						armed = true
					}
					return nil
				})
				defer restoreRun()
				restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
					if !armed {
						return nil
					}
					if a.Role == "RECEIPT" {
						receipt = a
					}
					// Evidence POSTs precede the receipt. Only the attempt POST
					// proves the durable DISPATCH boundary we need to exercise.
					target := a.Role == "POST" && strings.HasPrefix(a.Target, "evidence/")
					if committed {
						target = a.Role == "POST" && a.Target == "attempts/"+aID+".json"
					}
					if !target {
						return nil
					}
					if committed {
						if receipt.Target == "" {
							t.Fatal("attempt POST reached without DISPATCH receipt")
						}
						raw, e := os.ReadFile(filepath.Join(f.s.repo.StateDir, receipt.Target))
						if e != nil || !bytes.Equal(raw, receipt.Data) {
							t.Fatalf("DISPATCH receipt not durable: %s: %v", receipt.Target, e)
						}
					} else if receipt.Target != "" {
						t.Fatal("evidence fault occurred after receipt publication")
					}
					fired++
					// Disarm before noExec tries its separate cleanup transaction.
					armed = false
					return errno
				})
				t.Cleanup(restore)
				_, err = w.RunRole(ctx, "reviewer", "")
				restore()
				restoreRun()
				if fired != 1 || !errors.Is(err, errno) || wire.CodeOf(err) != wire.CodeResourceExhausted || wire.RetryForbidden(err) != committed {
					t.Fatalf("fired=%d forbidden=%v want=%v err=%v", fired, wire.RetryForbidden(err), committed, err)
				}
				if _, e := os.Stat(filepath.Join(f.scripts, "review-args")); !os.IsNotExist(e) {
					t.Fatalf("reviewer ran after failed dispatch: %v", e)
				}
			})
		}
	}
}
