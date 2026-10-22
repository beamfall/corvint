//go:build darwin || linux

package store_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
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

// Automatic preparation shares the durable receipt boundary with explicit
// health. Assert the real claim/acquire CLI result, including a pre-receipt control.
func TestCALV0215_AutomaticHealthExhaustion(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		for _, verb := range []string{"claim", "acquire"} {
			for _, boundary := range []string{"before-receipt", "after-receipt", "after-health"} {
				t.Run(fmt.Sprintf("%s/%s/%s", verb, errno, boundary), func(t *testing.T) {
					committed := boundary != "before-receipt"
					s := newLeaseStore(t)
					s.root = s.repo.PrimaryWorktree
					gitRun(t, s.root, "init", "-q", "-b", "main")
					gitRun(t, s.root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
					marker := filepath.Join(t.TempDir(), "ran")
					v := fixture.PolicyValue()
					v.Obj.Set("policyVersion", str("3"))
					v.Obj.Vals["budgets"].Obj.Set("requireEnforcedFields", wire.Array())
					health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "printf x >> " + marker}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
					v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("health", health)))))
					if rep, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("health-policy", "2", wire.EncodeFile(v)), now(t)); err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
						t.Fatalf("fixture policy: %+v %v", rep, err)
					}
					id := s.ticket(t, "health")
					args := []string{"claim", id, "--holder", "agent-1", "--stage", "implement", "--scope", "one"}
					if verb == "acquire" {
						claim := s.claim(t, "initial", id, 0, "one")
						args = []string{"pool", "acquire", "--attempt", claim.AttemptID, "--generation", string(claim.Generation)}
					}
					args = append(args, "--pool", "db", "--request-id", "health-pressure")
					auditOK(t, s.repo)
					var baseline bytes.Buffer
					cli.Run(cli.Env{Cwd: s.root, Args: []string{"receipt", "audit"}, Stdout: &baseline})
					if res, err := wire.DecodeResult(baseline.Bytes()); err != nil || res.Outcome != wire.OutcomeOK {
						t.Fatalf("fixture envelope: %s %v", baseline.Bytes(), err)
					}
					armed, fired := false, 0
					var receipt transaction.Artifact
					restoreHealth := store.SetHealthPrepareHookForTest(func(member string) {
						if member != "a" {
							t.Fatalf("member: %s", member)
						}
						armed = true
					})
					t.Cleanup(restoreHealth)
					restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
						if !armed || fired != 0 {
							return nil
						}
						if boundary == "after-health" {
							ran, err := os.ReadFile(marker)
							if os.IsNotExist(err) {
								if a.Role == "RECEIPT" {
									receipt = a
								}
								return nil
							}
							if err != nil || string(ran) != "x" {
								t.Fatalf("health marker: %q %v", ran, err)
							}
							if a.Role != "RECEIPT" {
								return nil
							}
							r, err := snapshot.DecodeReceipt(a.Data)
							if err != nil || r.RequestID == nil || *r.RequestID != "health-pressure" {
								t.Fatalf("fault missed claim/acquire after passing health: %+v %v", r, err)
							}
						} else {
							if a.Role == "RECEIPT" {
								receipt = a
							}
							target := a.Role == "RECEIPT"
							if committed {
								target = a.Role == "POST" && a.Target == "pools.json"
							}
							if !target {
								return nil
							}
						}
						if receipt.Target == "" {
							t.Fatal("fault did not reach receipt boundary")
						}
						raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, receipt.Target))
						if committed {
							if err != nil || !bytes.Equal(raw, receipt.Data) {
								t.Fatalf("receipt not durable: %v", err)
							}
						} else if !os.IsNotExist(err) {
							t.Fatalf("pre-receipt fault found receipt: %v", err)
						}
						fired++
						return &os.PathError{Op: "open", Path: a.Target, Err: errno}
					})
					t.Cleanup(restore)
					var out bytes.Buffer
					cli.Run(cli.Env{Cwd: s.root, Args: args, Stdout: &out})
					restore()
					restoreHealth()
					res, err := wire.DecodeResult(out.Bytes())
					if err != nil {
						t.Fatalf("envelope: %s %v", out.Bytes(), err)
					}
					if !armed || fired != 1 || res.Outcome != wire.OutcomeError || len(res.Codes) != 1 || res.Codes[0] != wire.CodeResourceExhausted || !strings.Contains(out.String(), fmt.Sprintf(`"retryable":%v`, !committed)) {
						t.Fatalf("armed=%v fired=%d result=%s", armed, fired, out.Bytes())
					}
					if boundary == "after-health" {
						if ran, err := os.ReadFile(marker); err != nil || string(ran) != "x" {
							t.Fatalf("health did not run exactly once: %q %v", ran, err)
						}
						if state := psrPools(t, s); len(state.Entries) != 1 || state.Entries[0].State != "QUARANTINED" {
							t.Fatalf("cleanup did not quarantine member: %+v", state)
						}
						auditOK(t, s.repo)
					} else if _, err := os.Stat(marker); !os.IsNotExist(err) {
						t.Fatalf("health program ran: %v", err)
					}
				})
			}
		}
	}
}
