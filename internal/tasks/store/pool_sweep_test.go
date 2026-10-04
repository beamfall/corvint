package store_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func psrFixture(t *testing.T, reset, verify, expect, attempts string, cleanup bool) (*leaseStore, *store.Report) {
	return psrFixtureConfigured(t, reset, verify, expect, attempts, cleanup, nil)
}
func psrFixtureConfigured(t *testing.T, reset, verify, expect, attempts string, cleanup bool, configure func(wire.Value)) (*leaseStore, *store.Report) {
	t.Helper()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	reuse := obj("argv", wire.Strings([]string{"/bin/sh", "-c", reset}), "envKeys", wire.Strings(nil), "timeoutSeconds", str("3"), "maxAttempts", str(attempts), "verify", obj("argv", wire.Strings([]string{"/bin/sh", "-c", verify}), "envKeys", wire.Strings(nil), "expectExit", str("0"), "expectStdout", str(expect)))
	envPath := filepath.Join(t.TempDir(), "private.env")
	if e := os.WriteFile(envPath, nil, 0600); e != nil {
		t.Fatal(e)
	}
	reuse.Obj.Set("env", str("file:"+envPath))
	config := obj("safeReuse", reuse)
	if cleanup {
		config.Obj.Set("cleanup", obj("argv", wire.Strings([]string{"/bin/sh", "-c", "printf cleanup"}), "env", wire.Strings(nil), "cwd", str("REPOSITORY"), "timeoutSeconds", str("3")))
	}
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", config))))
	if configure != nil {
		configure(v)
	}
	// Fixture codec succeeds before writes and before any injected command is reached.
	if _, e := intent.DecodePolicy(wire.EncodeFile(v)); e != nil {
		t.Fatal(e)
	}
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("psr-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	id := s.ticket(t, "sweep")
	claim := claimOf(id, "work")
	claim.Pool = "db"
	a := s.lease(t, "allocate", claim, 0, nil)
	if a.PoolAllocation == nil {
		t.Fatal("fixture allocation failed")
	}
	released := s.lease(t, "release", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation}, 0, nil)
	if released.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatal("fixture release failed")
	}
	return s, a
}
func psrPools(t *testing.T, s *leaseStore) *snapshot.PoolState {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
	if e != nil {
		t.Fatal(e)
	}
	p, e := snapshot.DecodePools(raw)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func psrRun(s *leaseStore, id string) (*store.PoolSweepReport, error) {
	return store.PoolSweep(context.Background(), s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: id, Root: s.root, TimeoutSeconds: "15"})
}
func TestPSRSweepSuccessReplaySuccessor(t *testing.T) {
	s, a := psrFixture(t, "printf reset", "printf 'literal[ok]'", "literal[ok]", "1", true)
	out, e := psrRun(s, "sweep")
	if e != nil || out.Report == nil || out.Report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("sweep %+v %v", out, e)
	}
	if len(psrPools(t, s).Entries) != 0 {
		t.Fatal("not freed")
	}
	id := s.ticket(t, "successor")
	c := claimOf(id, "next")
	c.Pool = "db"
	next := s.lease(t, "successor", c, 0, nil)
	if next.PoolAllocation == nil || next.PoolAllocation.AllocationID == a.PoolAllocation.AllocationID {
		t.Fatal("successor not distinct")
	}
	policyRaw, e := os.ReadFile(filepath.Join(filepath.Join(s.repo.PrimaryWorktree, intent.Dir), "policy.json"))
	if e != nil {
		t.Fatal(e)
	}
	policy, e := intent.DecodePolicy(policyRaw)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(policy.Pools[0].MemberConfig["a"].SafeReuse.EnvFile); e != nil {
		t.Fatal(e)
	}
	os.RemoveAll(s.root)
	replay, e := psrRun(s, "sweep")
	if e != nil || replay.Pending || !bytes.Equal(out.Result, replay.Result) {
		t.Fatalf("replay %+v %v", replay, e)
	}
	if psrPools(t, s).Entries[0].AllocationID != next.PoolAllocation.AllocationID {
		t.Fatal("replay touched successor")
	}
}
func TestPSRSweepMismatchRetries(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "reached")
	s, _ := psrFixture(t, "printf x >> "+marker, "printf wrong", "expected", "2", false)
	out, e := psrRun(s, "mismatch")
	if e != nil || out.Pending {
		t.Fatalf("result %+v %v", out, e)
	}
	p := psrPools(t, s)
	if len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" || p.Entries[0].Sweep != nil {
		t.Fatal(p)
	}
	raw, e := os.ReadFile(marker)
	if e != nil || string(raw) != "xx" {
		t.Fatalf("injected retry not reached %q %v", raw, e)
	}
}
func TestPSRSweepOwnerBarrier(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "reached")
	release := marker + ".release"
	verify := "printf reached > " + marker + "; while [ ! -e " + release + " ]; do /bin/sleep 0.01; done; printf verified"
	s, a := psrFixture(t, "printf reset", verify, "verified", "1", false)
	done := make(chan error, 1)
	go func() { _, e := psrRun(s, "barrier"); done <- e }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, e := os.Stat(marker); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("verify barrier not reached")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() {
		os.WriteFile(release, nil, 0600)
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("sweep unjoined")
		}
	})
	for i, verb := range []string{transaction.LeasePoolSafe, transaction.LeasePoolCleanup} {
		request := transaction.LeaseRequest{Verb: verb, Member: "a", Allocation: string(a.PoolAllocation.AllocationID)}
		if verb == transaction.LeasePoolSafe {
			request.Evidence = "manual"
			request.Reason = "manual"
		}
		r := s.lease(t, "manual"+string(rune('a'+i)), request, 0, nil)
		if r.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatalf("manual %s stole owner", verb)
		}
	}
	if p := psrPools(t, s); p.Entries[0].Sweep == nil || p.Entries[0].Sweep.Phase != "verify" {
		t.Fatal("owner disappeared")
	}
	if e := os.WriteFile(release, nil, 0600); e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
		done <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("sweep hung")
	}
	if len(psrPools(t, s).Entries) != 0 {
		t.Fatal("confirmation failed")
	}
}
func TestPSRSweepStderrIsNotStdout(t *testing.T) {
	s, _ := psrFixture(t, "true", "printf expected >&2", "expected", "1", false)
	_, e := psrRun(s, "stderr")
	if e != nil {
		t.Fatal(e)
	}
	p := psrPools(t, s)
	if len(p.Entries) != 1 || !strings.Contains(p.Entries[0].Reason, "STDOUT_MISMATCH") {
		t.Fatal(p)
	}
}

func TestPSRAmbiguousResponseReconciliation(t *testing.T) {
	for _, mode := range []string{"confirm-response", "confirm-and-readback-response", "finish-response"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "runs")
			s, a := psrFixture(t, "printf x >> "+marker, "printf verified", "verified", "1", false)
			if state := psrPools(t, s); len(state.Entries) != 1 || state.Entries[0].State != "QUARANTINED" {
				t.Fatal("fixture not quarantined")
			}
			calls := 0
			target := transaction.LeasePoolSafe
			if mode == "finish-response" {
				target = transaction.LeasePoolSweepFinish
			}
			ctx := store.PSRTestResponseFailure(context.Background(), func(c store.LeaseChoice, r *store.Report, e error) error {
				if c.Lease.Verb == target && r != nil && r.Outcome.Outcome == mutation.OutcomeCompleted && r.Outcome.ReceiptSeq != nil && calls < map[string]int{"confirm-response": 1, "confirm-and-readback-response": 2, "finish-response": 1}[mode] {
					if state := psrPools(t, s); len(state.Entries) != 0 {
						t.Fatal("response injection before durable confirm")
					}
					calls++
					return fmt.Errorf("injected committed response loss")
				}
				return e
			})
			out, e := store.PoolSweep(ctx, s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "ambiguous", Root: s.root, TimeoutSeconds: "15"})
			if mode == "confirm-and-readback-response" {
				if e == nil || !out.Pending || calls != 2 {
					t.Fatalf("readback failure path: %+v %v %d", out, e, calls)
				}
			} else if e != nil || out.Pending || len(out.Result) == 0 || calls != 1 {
				t.Fatalf("bounded reconciliation: %+v %v %d", out, e, calls)
			}
			if raw, _ := os.ReadFile(marker); string(raw) != "x" {
				t.Fatal("command not reached once", string(raw))
			}
			id := s.ticket(t, "successor")
			claim := claimOf(id, "next")
			claim.Pool = "db"
			next := s.lease(t, "successor", claim, 0, nil)
			if next.PoolAllocation == nil || next.PoolAllocation.AllocationID == a.PoolAllocation.AllocationID {
				t.Fatal("successor fixture")
			}
			policyRaw, err := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, intent.Dir, "policy.json"))
			if err != nil {
				t.Fatal(err)
			}
			policy, err := intent.DecodePolicy(policyRaw)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Remove(policy.Pools[0].MemberConfig["a"].SafeReuse.EnvFile); err != nil {
				t.Fatal(err)
			}
			if err = os.RemoveAll(s.root); err != nil {
				t.Fatal(err)
			}
			replay, err := psrRun(s, "ambiguous")
			if err != nil || replay.Pending || len(replay.Result) == 0 {
				t.Fatalf("historical reconcile: %+v %v", replay, err)
			}
			again, err := psrRun(s, "ambiguous")
			if err != nil || !bytes.Equal(replay.Result, again.Result) || (len(out.Result) > 0 && !bytes.Equal(out.Result, replay.Result)) {
				t.Fatal("historical terminal differs", err)
			}
			if state := psrPools(t, s); len(state.Entries) != 1 || state.Entries[0].AllocationID != next.PoolAllocation.AllocationID {
				t.Fatal("successor mutated")
			}
			if raw, _ := os.ReadFile(marker); string(raw) != "x" {
				t.Fatal("reset rerun", string(raw))
			}
			t.Log("COMMIT_RESPONSE_LOSS_REACHED", mode, calls)
		})
	}
}
func TestPSRFirstCleanupProbeFailureRetiresChild(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "child")
	// The child inherits the owned group but closes both captured pipes; the
	// leader exits, so cmd.Wait cannot retire the child on our behalf.
	script := "/bin/sleep 60 </dev/null >/dev/null 2>&1 & echo $! > " + marker + "; exit 0"
	s, _ := psrFixture(t, script, "printf verified", "verified", "1", false)
	if state := psrPools(t, s); len(state.Entries) != 1 || state.Entries[0].State != "QUARANTINED" {
		t.Fatal("fixture not ready")
	}
	calls := store.PSRTestFirstCleanupProbe(t, marker)
	start := time.Now()
	out, e := psrRun(s, "probe-failed")
	if e != nil || out.Pending || *calls != 1 {
		t.Fatalf("probe injection not reached %+v %v %d", out, e, *calls)
	}
	raw, e := os.ReadFile(marker)
	if e != nil {
		t.Fatal("child path not reached", e)
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(raw)))
	if e != nil {
		t.Fatal(e)
	}
	status, err := exec.Command("/bin/ps", "-p", fmt.Sprint(pid), "-o", "stat=").Output()
	if err == nil && !strings.HasPrefix(strings.TrimSpace(string(status)), "Z") {
		t.Fatalf("known child survived: %s", status)
	}
	if time.Since(start) > 8*time.Second {
		t.Fatal("cleanup unbounded")
	}
	if state := psrPools(t, s); len(state.Entries) != 1 || state.Entries[0].State != "QUARANTINED" || !strings.Contains(state.Entries[0].Reason, "UNKNOWN") {
		t.Fatal("uncertainty admitted")
	}
	t.Log("FIRST_CLEANUP_PROBE_FAILURE_REACHED_CHILD_RETIRED", pid)
}
