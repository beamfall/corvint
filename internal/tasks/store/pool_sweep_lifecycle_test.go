//go:build darwin || linux

package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"path/filepath"
	"strings"
	"testing"
	"time"
)

type psrOutcome struct {
	report *store.PoolSweepReport
	err    error
}

func psrStart(t *testing.T, s *leaseStore, id string) (<-chan psrOutcome, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan psrOutcome, 1)
	go func() {
		r, e := store.PoolSweep(ctx, s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: id, Root: s.root, TimeoutSeconds: "15"})
		done <- psrOutcome{r, e}
	}()
	return done, cancel
}

// psrAwaitMarker waits until the marker has content. A shell redirect creates
// the file before the command writes to it, so existence alone can be observed
// while the file is still empty.
func psrAwaitMarker(t *testing.T, path string) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		if info, e := os.Stat(path); e == nil && info.Size() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("command boundary not reached", path)
}

// PSR-V0-008: once ALL is set an owned sweep launches no further phase command;
// it only publishes its terminal observation. An owned successful verify may
// still finalize its exact delegated confirmation.
func TestPSRAllBarrierTerminal(t *testing.T) {
	for _, mode := range []string{"during-reset", "before-verify-launch", "during-verify"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "reached")
			resume := filepath.Join(dir, "resume")
			verified := filepath.Join(dir, "verify-ran")
			wait := "; while [ ! -f " + resume + " ]; do /bin/sleep 0.01; done"
			reset, verify := "printf reset > "+marker+wait, "printf x > "+verified+"; printf verified"
			switch mode {
			case "before-verify-launch":
				reset = "printf reset > " + marker
			case "during-verify":
				reset = "printf reset > " + marker
				verify = "printf x > " + verified + wait + "; printf verified"
			}
			s, _ := psrFixture(t, reset, verify, "verified", "1", false)
			ctx, cancel := context.WithCancel(context.Background())
			barrier := false
			if mode == "before-verify-launch" {
				ctx = store.PSRTestResponseFailure(ctx, func(c store.LeaseChoice, r *store.Report, e error) error {
					if c.Lease.Verb == transaction.LeasePoolObserve && !barrier && r != nil && r.Outcome.Outcome == mutation.OutcomeCompleted {
						barrier = true
						reconciliationBarrier(t, s.repo, "ALL")
					}
					return e
				})
			}
			done := make(chan psrOutcome, 1)
			go func() {
				r, e := store.PoolSweep(ctx, s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "barrier-sweep", Root: s.root, TimeoutSeconds: "15"})
				done <- psrOutcome{r, e}
			}()
			joined := false
			defer func() {
				cancel()
				if !joined {
					select {
					case <-done:
					case <-time.After(40 * time.Second):
						t.Error("sweep unjoined")
					}
				}
			}()
			switch mode {
			case "during-reset":
				psrAwaitMarker(t, marker)
			case "during-verify":
				psrAwaitMarker(t, verified)
			}
			if mode != "before-verify-launch" {
				reconciliationBarrier(t, s.repo, "ALL")
				fresh, e := psrRun(s, "fresh-under-all")
				if e == nil && fresh.Report != nil && fresh.Report.Outcome.Outcome == mutation.OutcomeCompleted {
					t.Fatal("fresh owner admitted under ALL")
				}
				if e := os.WriteFile(resume, nil, 0600); e != nil {
					t.Fatal(e)
				}
			}
			var result psrOutcome
			select {
			case result = <-done:
				joined = true
			case <-time.After(40 * time.Second):
				t.Fatal("owned sweep failed to terminate")
			}
			if result.err != nil || result.report.Pending || result.report.Report.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatal(result.report, result.err)
			}
			free := mode == "during-verify"
			if !bytes.Contains(result.report.Result, []byte(`"free":`+strconv.FormatBool(free))) {
				t.Fatal("terminal row", string(result.report.Result))
			}
			pools := psrPools(t, s)
			if free {
				if len(pools.Entries) != 0 {
					t.Fatal("owned valid verification did not free allocation")
				}
			} else {
				if _, e := os.Stat(verified); !os.IsNotExist(e) {
					t.Fatal("verify launched under ALL", e)
				}
				if len(pools.Entries) != 1 || pools.Entries[0].State != "QUARANTINED" || pools.Entries[0].Sweep != nil || !strings.Contains(pools.Entries[0].Reason, "ALL barrier") {
					t.Fatal("ALL sweep not terminal quarantine", pools)
				}
			}
			replay, e := psrRun(s, "barrier-sweep")
			if e != nil || replay.Pending || replay.Report.Outcome.Outcome != mutation.OutcomeCompleted || !bytes.Equal(replay.Result, result.report.Result) {
				t.Fatal(replay, e)
			}
			raw, e := os.ReadFile(marker)
			if e != nil || string(raw) != "reset" {
				t.Fatal("reset count changed", string(raw), e)
			}
		})
	}
}
func TestPSRConcurrentSweeps(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "reached")
	resume := filepath.Join(dir, "resume")
	s, _ := psrFixture(t, "echo reset >> "+marker+"; while [ ! -f "+resume+" ]; do /bin/sleep 0.01; done", "printf verified", "verified", "1", false)
	done, cancel := psrStart(t, s, "first")
	joined := false
	defer func() {
		cancel()
		if !joined {
			select {
			case <-done:
			case <-time.After(40 * time.Second):
				t.Error("sweep unjoined")
			}
		}
	}()
	psrAwaitMarker(t, marker)
	other, e := psrRun(s, "other")
	if e == nil && other.Report != nil && other.Report.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatal("second owner admitted")
	}
	pending, e := psrRun(s, "first")
	if e != nil || !pending.Pending {
		t.Fatal("exact replay not pending", pending, e)
	}
	if e := os.WriteFile(resume, nil, 0600); e != nil {
		t.Fatal(e)
	}
	select {
	case result := <-done:
		joined = true
		if result.err != nil {
			t.Fatal(result.err)
		}
	case <-time.After(40 * time.Second):
		t.Fatal("sweep unjoined")
	}
	raw, e := os.ReadFile(marker)
	if e != nil || string(raw) != "reset\n" {
		t.Fatal("second reset", string(raw), e)
	}
}

// PSR-V0-009: a changed allocation or definition refuses before admission or
// execution, and historical replay precedes the now-inapplicable observation.
func TestPSRDispatchAllocationFence(t *testing.T) {
	for _, mode := range []string{"allocation", "definition"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "reset")
			s, a := psrFixture(t, "echo reset >> "+marker, "printf verified", "verified", "1", false)
			c := store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "dispatch-fence", Root: s.root, Member: "a", Allocation: string(a.PoolAllocation.AllocationID), ExpectedDefinition: a.PoolAllocation.DefinitionSha256, TimeoutSeconds: "15"}
			if mode == "allocation" {
				c.Allocation = string(wire.Sum([]byte("predecessor")))
			} else {
				c.ExpectedDefinition = wire.Sum([]byte("recorded-definition"))
			}
			before := storeDigest(t, s.repo)
			_, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
			if e == nil || wire.CodeOf(e) != wire.CodeFenced {
				t.Fatal("changed identity was admitted", e)
			}
			if _, e := os.Stat(marker); !os.IsNotExist(e) {
				t.Fatal("reset executed", e)
			}
			if storeDigest(t, s.repo) != before {
				t.Fatal("refusal changed native state")
			}
			c.Allocation = string(a.PoolAllocation.AllocationID)
			c.ExpectedDefinition = a.PoolAllocation.DefinitionSha256
			out, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
			if e != nil || out.Pending || len(psrPools(t, s).Entries) != 0 {
				t.Fatal(out, e)
			}
			// Historical replay precedes the now-inapplicable observed definition.
			c.ExpectedDefinition = wire.Sum([]byte("later definition"))
			replay, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
			if e != nil || replay.Pending || replay.Evidence != out.Evidence {
				t.Fatal(replay, e)
			}
			raw, e := os.ReadFile(marker)
			if e != nil || string(raw) != "reset\n" {
				t.Fatal("replay executed", string(raw), e)
			}
		})
	}
}

func psrReuse(v wire.Value) (*wire.Object, *wire.Object) {
	pools, _ := v.Obj.Get("pools")
	members, _ := pools.Arr[0].Obj.Get("memberConfig")
	member, _ := members.Obj.Get("a")
	reuse, _ := member.Obj.Get("safeReuse")
	return member.Obj, reuse.Obj
}
func TestPSRDeadlineRetryMatrix(t *testing.T) {
	for _, mode := range []string{"retry-success", "retry-exhausted", "cleanup-failed", "shared-deadline", "total-deadline"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			resetLog := filepath.Join(dir, "reset")
			verifyLog := filepath.Join(dir, "verify")
			cleanLog := filepath.Join(dir, "clean")
			reset := "echo reset >> " + resetLog
			verify := "echo verify >> " + verifyLog + "; printf verified"
			cleanup := mode == "cleanup-failed" || mode == "shared-deadline"
			switch mode {
			case "retry-success":
				reset += "; [ -f " + dir + "/failed ] || { touch " + dir + "/failed; exit 1; }"
			case "retry-exhausted":
				reset += "; exit 1"
			case "shared-deadline":
				reset += "; /bin/sleep 0.1"
				verify += "; /bin/sleep 20"
			case "total-deadline":
				reset += "; /bin/sleep 20; exit 1"
			}
			s, _ := psrFixtureConfigured(t, reset, verify, "verified", "2", cleanup, func(v wire.Value) {
				member, reuse := psrReuse(v)
				if mode == "shared-deadline" {
					// Wide enough that loaded Git source observations still reach verify.
					reuse.Set("timeoutSeconds", str("4"))
					member.Set("cleanup", obj("argv", wire.Strings([]string{"/bin/sh", "-c", "echo clean >> " + cleanLog + "; /bin/sleep 0.1"}), "env", wire.Strings(nil), "cwd", str("REPOSITORY"), "timeoutSeconds", str("10")))
				}
				if mode == "cleanup-failed" {
					member.Set("cleanup", obj("argv", wire.Strings([]string{"/bin/sh", "-c", "echo clean >> " + cleanLog + "; exit 2"}), "env", wire.Strings(nil), "cwd", str("REPOSITORY"), "timeoutSeconds", str("3")))
				}
			})
			budget := wire.Count("15")
			if mode == "total-deadline" {
				budget = "3"
			}
			started := time.Now()
			out, e := store.PoolSweep(context.Background(), s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "matrix", Root: s.root, TimeoutSeconds: budget})
			if e != nil || out.Pending {
				t.Fatal(out, e)
			}
			if time.Since(started) > 30*time.Second {
				t.Fatal("unbounded deadline cleanup")
			}
			count := func(path string) int {
				raw, e := os.ReadFile(path)
				if os.IsNotExist(e) {
					return 0
				}
				if e != nil {
					t.Fatal(e)
				}
				return strings.Count(string(raw), "\n")
			}
			wantReset, wantVerify := 2, 0
			if mode == "retry-success" {
				wantVerify = 1
			}
			if mode == "cleanup-failed" {
				wantReset = 0
				if count(cleanLog) != 2 {
					t.Fatal("cleanup retry count")
				}
			}
			if mode == "shared-deadline" || mode == "total-deadline" {
				wantReset = 1
			}
			if mode == "shared-deadline" {
				wantVerify = 1
			}
			if count(resetLog) != wantReset || count(verifyLog) != wantVerify {
				t.Fatalf("commands reset=%d verify=%d", count(resetLog), count(verifyLog))
			}
			pools := psrPools(t, s)
			if (len(pools.Entries) == 0) != (mode == "retry-success") {
				t.Fatal("unsafe result", pools)
			}
			// Actual immutable observations bind all reached phases to the same attempt deadline.
			observations := psrObservations(t, s)
			deadlines := map[wire.Count]wire.Size{}
			for _, o := range observations {
				if prior, ok := deadlines[o.Attempt]; ok && prior != o.Timing.Deadline {
					t.Fatal("phase renewed deadline")
				}
				deadlines[o.Attempt] = o.Timing.Deadline
			}
			if len(observations) == 0 {
				t.Fatal("no durable timing evidence")
			}
		})
	}
}
func psrObservations(t *testing.T, s *leaseStore) []snapshot.PoolSweepObservation {
	t.Helper()
	entries, e := os.ReadDir(filepath.Join(s.repo.StateDir, "evidence"))
	if e != nil {
		t.Fatal(e)
	}
	out := []snapshot.PoolSweepObservation{}
	for _, en := range entries {
		raw, e := os.ReadFile(filepath.Join(s.repo.StateDir, "evidence", en.Name()))
		if e != nil {
			t.Fatal(e)
		}
		o, e := snapshot.DecodePoolSweepObservation(raw)
		if e == nil {
			out = append(out, *o)
		}
	}
	return out
}

// PSR-V0-005: verify passes only on the declared exit with the stdout literal
// within the 64KiB bound; overflow or source change cannot pass.
func TestPSRPredicateAndSourceMatrix(t *testing.T) {
	for _, mode := range []string{"nonzero-expected", "wrong-exit", "dirty-reset", "dirty-verify", "output-bound", "output-overflow"} {
		t.Run(mode, func(t *testing.T) {
			reset := "printf reset"
			verify := "printf verified"
			expect := "verified"
			if mode == "nonzero-expected" || mode == "wrong-exit" {
				verify += "; exit 7"
			}
			if mode == "dirty-reset" {
				reset += "; touch changed-source"
			}
			if mode == "dirty-verify" {
				verify += "; touch changed-source"
			}
			if strings.HasPrefix(mode, "output-") {
				n := "65536"
				if mode == "output-overflow" {
					n = "65537"
				}
				verify = "/usr/bin/head -c " + n + " /dev/zero | /usr/bin/tr '\\000' x"
				expect = "xxx"
			}
			s, _ := psrFixtureConfigured(t, reset, verify, expect, "1", false, func(v wire.Value) {
				if mode == "nonzero-expected" {
					_, reuse := psrReuse(v)
					x, _ := reuse.Get("verify")
					x.Obj.Set("expectExit", str("7"))
				}
			})
			out, e := psrRun(s, "predicate")
			if e != nil || out.Pending {
				t.Fatal(out, e)
			}
			wantFree := mode == "nonzero-expected" || mode == "output-bound"
			if (len(psrPools(t, s).Entries) == 0) != wantFree {
				t.Fatal("predicate/source wrongly freed", mode, psrPools(t, s))
			}
			if mode == "dirty-reset" {
				for _, o := range psrObservations(t, s) {
					if o.Phase == "verify" {
						t.Fatal("verify ran after source changed")
					}
				}
			}
		})
	}
}
func TestPSRPhaseEnvironmentIsolation(t *testing.T) {
	secret := "literal-$(never-run)"
	t.Setenv("TOKEN", "ambient")
	envPath := filepath.Join(t.TempDir(), "env")
	if e := os.WriteFile(envPath, []byte("TOKEN="+secret+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	s, _ := psrFixtureConfigured(t, "/usr/bin/env; /bin/rm "+envPath, "/usr/bin/env", "TOKEN="+secret, "1", true, func(v wire.Value) {
		member, reuse := psrReuse(v)
		reuse.Set("env", str("file:"+envPath))
		reuse.Set("envKeys", wire.Strings([]string{"TOKEN"}))
		verify, _ := reuse.Get("verify")
		verify.Obj.Set("envKeys", wire.Strings([]string{"TOKEN"}))
		member.Set("cleanup", obj("argv", wire.Strings([]string{"/usr/bin/env"}), "env", wire.Strings(nil), "cwd", str("REPOSITORY"), "timeoutSeconds", str("3")))
		v.Obj.Set("environment", obj("allowedEnvKeys", wire.Strings([]string{"TOKEN"})))
	})
	out, e := psrRun(s, "environment")
	if e != nil || out.Pending {
		t.Fatal(out, e)
	}
	for _, o := range psrObservations(t, s) {
		raw, e := os.ReadFile(filepath.Join(s.repo.StateDir, "evidence", string(o.Log)))
		if e != nil {
			t.Fatal(e)
		}
		stdout, _, e := snapshot.DecodePoolSweepLog(raw)
		if e != nil {
			t.Fatal(e)
		}
		if o.Phase == "cleanup" && bytes.Contains(stdout, []byte("TOKEN=")) {
			t.Fatal("cleanup inherited undeclared variable")
		}
		if o.Phase != "cleanup" && !bytes.Contains(stdout, []byte("TOKEN="+secret)) {
			t.Fatal("captured environment changed", o.Phase, string(stdout))
		}
		if bytes.Contains(o.Encode(), []byte(secret)) || bytes.Contains(out.Result, []byte(secret)) {
			t.Fatal("secret leaked to structured evidence")
		}
	}
}

func TestPSRRequestConflictAndOrphan(t *testing.T) {
	// PSR-V0-001/002/009: an occupied safeReuse definition is immutable, a WORKER
	// is refused before any command runs, and a changed meaningful request with
	// the same id conflicts without mutation or execution.
	t.Run("request-and-policy-fences", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "runs")
		s, a := psrFixture(t, "printf x >> "+marker, "printf verified", "verified", "1", false)
		policyRaw, e := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, ".taskman", "policy.json"))
		if e != nil {
			t.Fatal(e)
		}
		policy, e := wire.Parse(policyRaw)
		if e != nil {
			t.Fatal(e)
		}
		policy.Obj.Set("policyVersion", str("4"))
		_, reuse := psrReuse(policy)
		reuse.Set("argv", wire.Strings([]string{"/bin/true"}))
		changed, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("occupied-definition", "3", wire.EncodeFile(policy)), now(t))
		if e != nil || changed.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatal("occupied safeReuse definition changed", changed, e)
		}
		worker := operator()
		worker.Role = "WORKER"
		c := store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "identity", Root: s.root, TimeoutSeconds: "15"}
		if _, e = store.PoolSweep(context.Background(), s.repo, worker, c); e == nil {
			t.Fatal("WORKER admitted")
		}
		if _, e = os.Stat(marker); !os.IsNotExist(e) {
			t.Fatal("role refusal executed", e)
		}
		out, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
		if e != nil || out.Pending || len(psrPools(t, s).Entries) != 0 {
			t.Fatal(out, e)
		}
		for _, mode := range []string{"budget", "member", "allocation", "actor"} {
			next := c
			actor := operator()
			switch mode {
			case "budget":
				next.TimeoutSeconds = "14"
			case "member":
				next.Member = "a"
			case "allocation":
				next.Member = "a"
				next.Allocation = string(a.PoolAllocation.AllocationID)
			case "actor":
				actor.ID = "another"
			}
			before := storeDigest(t, s.repo)
			conflict, e := store.PoolSweep(context.Background(), s.repo, actor, next)
			if e != nil || conflict.Report == nil || !conflict.Report.Outcome.HasCode(wire.CodeRequestIDConflict) || storeDigest(t, s.repo) != before {
				t.Fatal(mode, conflict, e)
			}
		}
		if raw, _ := os.ReadFile(marker); string(raw) != "x" {
			t.Fatal("conflict reran command", string(raw))
		}
	})
	// PSR-V0-003/009 (H1 retry): host death needs explicit orphan recovery with a
	// runner identity check; once it releases the owner, retrying the original
	// request ends it quarantined and not free, citing the owner placeholder,
	// without running any phase again.
	t.Run("live-unknown-gone", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "runner")
		s, a := psrFixtureConfigured(t, "echo $$ > "+marker+"; sleep 60", "printf verified", "verified", "1", false, func(v wire.Value) { _, r := psrReuse(v); r.Set("timeoutSeconds", str("20")) })
		cmd := exec.Command(os.Args[0], "-test.run=^TestPSRNativeOrphanChild$", "-test.timeout=60s")
		cmd.Dir = s.root
		cmd.Env = append(os.Environ(), "PSR_ORPHAN_REPO="+s.repo.PrimaryWorktree, "PSR_ORPHAN_ROOT="+s.root)
		cmd.WaitDelay = time.Second
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		owned := psrOwnOrphan(t, cmd)
		psrAwaitMarker(t, marker)
		raw, e := os.ReadFile(marker)
		if e != nil {
			t.Fatal(e)
		}
		resetPID, e := strconv.Atoi(strings.TrimSpace(string(raw)))
		if e != nil {
			t.Fatal(e)
		}
		owned.capture(t)
		reset, ok := owned.children[resetPID]
		if !ok || reset.group != resetPID {
			t.Fatal("reset identity not anchored to owner", resetPID)
		}
		descendant := false
		for _, child := range owned.children {
			if child.parent == resetPID {
				descendant = true
			}
		}
		if !descendant {
			t.Fatal("sleep descendant not independently observed")
		}
		state := psrPools(t, s)
		if len(state.Entries) != 1 || state.Entries[0].Sweep == nil || int(state.Entries[0].RunnerPID.Int()) != cmd.Process.Pid {
			t.Fatal("native runner owner not reached", state)
		}
		recover := transaction.LeaseRequest{Verb: transaction.LeasePoolRecover, Member: "a", Allocation: string(a.PoolAllocation.AllocationID), Reason: "explicit orphan recovery"}
		live, e := store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "live", Root: s.root, Lease: recover}, now(t))
		if e != nil {
			t.Fatal(e)
		}
		if live.Outcome.Outcome == mutation.OutcomeCompleted || !live.Outcome.HasCode(wire.CodeQuiescenceUnproved) {
			t.Fatal("live recovery admitted", live)
		}
		restore, calls := store.PSRTestUnknownRunner()
		_, e = store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "unknown", Root: s.root, Lease: recover}, now(t))
		restore()
		if e == nil || *calls != 1 {
			t.Fatal("unknown process proof not reached", e, *calls)
		}
		if p := psrPools(t, s); len(p.Entries) != 1 || p.Entries[0].Sweep == nil {
			t.Fatal("unknown proof cleared owner")
		}
		c := store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "orphan", Root: s.root, TimeoutSeconds: "30"}
		held, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
		if e != nil || !held.Pending {
			t.Fatal("owned original ended before recovery", held, e)
		}
		owned.capture(t)
		if e = cmd.Process.Kill(); e != nil {
			t.Fatal(e)
		}
		owned.join(t)
		owned.retire(t)
		gone, e := store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "gone", Root: s.root, Lease: recover}, now(t))
		if e != nil {
			t.Fatal(e)
		}
		if gone.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatal(gone)
		}
		p := psrPools(t, s)
		if len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" || p.Entries[0].Sweep != nil || p.Entries[0].CleanupPassed {
			t.Fatal("gone recovery manufactured safety", p)
		}
		oldMarker, _ := os.ReadFile(marker)
		// Explicit proved-orphan recovery released the owner, so the original ends
		// quarantined with its owner placeholder; it never runs again or frees.
		replay, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
		if e != nil || replay.Pending || replay.Report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatal("released orphan original did not end", replay, e)
		}
		var row struct {
			Owner   string
			Members []struct {
				Free        bool
				Observation string
			}
		}
		if e = json.Unmarshal(replay.Result, &row); e != nil || len(row.Members) != 1 || row.Members[0].Free || row.Members[0].Observation != row.Owner {
			t.Fatal("orphan terminal row", string(replay.Result), e)
		}
		after, _ := os.ReadFile(marker)
		if !bytes.Equal(oldMarker, after) {
			t.Fatal("orphan replay ran command")
		}
		if p := psrPools(t, s); len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" || p.Entries[0].Sweep != nil {
			t.Fatal("terminal orphan changed quarantine", p)
		}
		again, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
		if e != nil || again.Pending || !bytes.Equal(again.Result, replay.Result) {
			t.Fatal("terminal orphan replay differs", again, e)
		}
		// A distinct explicit request may try again; the original never does.
		c.RequestID = "fresh-explicit"
		c.TimeoutSeconds = "1"
		next, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
		if e != nil || next.Pending {
			t.Fatal(next, e)
		}
		after, _ = os.ReadFile(marker)
		if bytes.Equal(oldMarker, after) {
			t.Fatal("new explicit request never executed")
		}
		if p := psrPools(t, s); len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" {
			t.Fatal("timed out new request freed member", p)
		}
		auditOK(t, s.repo)
	})
}

// A separate native engine process lets recovery observe a real live/gone owner
// while retaining the fixture's deliberately separate authority and Git source.
func TestPSRNativeOrphanChild(t *testing.T) {
	authority := os.Getenv("PSR_ORPHAN_REPO")
	if authority == "" {
		t.Skip("subprocess helper")
	}
	repo, e := intent.Resolve(authority)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	_, e = store.PoolSweep(ctx, repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "orphan", Root: os.Getenv("PSR_ORPHAN_ROOT"), TimeoutSeconds: "30"})
	if e != nil {
		t.Fatal(e)
	}
}

// Contention is injected after the replay's finish lookup, so the historical
// phase writer must consume the same invocation deadline as initial admission.
func TestPSRHistoricalReplayDeadline(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "runs")
	s, a := psrFixture(t, "printf x >> "+marker, "printf verified", "verified", "1", false)
	c := store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "bounded-history", Root: s.root, TimeoutSeconds: "5"}
	confirmed, withheld := 0, 0
	seed := store.PSRTestResponseFailure(context.Background(), func(choice store.LeaseChoice, r *store.Report, e error) error {
		if choice.Lease.Verb == transaction.LeasePoolSafe && r != nil && r.Outcome.Outcome == mutation.OutcomeCompleted {
			confirmed++
			return fmt.Errorf("injected durable confirm response loss")
		}
		if choice.Lease.Verb == transaction.LeasePoolSweepFinish && confirmed == 1 {
			withheld++
			return fmt.Errorf("injected finish lookup response loss")
		}
		return e
	})
	original, e := store.PoolSweep(seed, s.repo, operator(), c)
	if e == nil || original == nil || !original.Pending || confirmed != 1 || withheld != 1 || len(psrPools(t, s).Entries) != 0 {
		t.Fatal("historical fixture not reached", original, e, confirmed, withheld)
	}
	id := s.ticket(t, "successor")
	claim := claimOf(id, "next")
	claim.Pool = "db"
	next := s.lease(t, "successor", claim, 0, nil)
	if next.PoolAllocation == nil || next.PoolAllocation.AllocationID == a.PoolAllocation.AllocationID {
		t.Fatal("successor fixture")
	}
	before := storeDigest(t, s.repo)
	var held *authority.Lock
	defer func() {
		if held != nil {
			_ = held.Close()
		}
	}()
	reached, expired := 0, 0
	replayCtx := store.PSRTestResponseFailure(context.Background(), func(choice store.LeaseChoice, r *store.Report, e error) error {
		if choice.Lease.Verb == transaction.LeasePoolSweepFinish && reached == 0 {
			reached++
			var err error
			held, err = authority.AcquireLock(context.Background(), s.repo, authority.LockOptions{})
			if err != nil {
				return err
			}
		}
		if choice.Lease.Verb == transaction.LeasePoolObserve && e != nil {
			expired++
		}
		return e
	})
	start := time.Now()
	replay, e := store.PoolSweep(replayCtx, s.repo, operator(), c)
	elapsed := time.Since(start)
	if !errors.Is(e, context.DeadlineExceeded) || replay == nil || !replay.Pending || reached != 1 || expired != 1 || elapsed > 7*time.Second {
		t.Fatal("historical deadline not reached", replay, e, reached, expired, elapsed)
	}
	if replay.Report.Outcome.ReceiptSeq == nil || original.Report.Outcome.ReceiptSeq == nil || *replay.Report.Outcome.ReceiptSeq != *original.Report.Outcome.ReceiptSeq {
		t.Fatal("original identity changed")
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	held = nil
	if storeDigest(t, s.repo) != before {
		t.Fatal("expired replay mutated store or successor")
	}
	if raw, _ := os.ReadFile(marker); string(raw) != "x" {
		t.Fatal("expired replay executed", string(raw))
	}
	successful, e := store.PoolSweep(context.Background(), s.repo, operator(), c)
	if e != nil || successful.Pending || len(successful.Result) == 0 {
		t.Fatal("ordinary historical reconciliation failed", successful, e)
	}
	if p := psrPools(t, s); len(p.Entries) != 1 || p.Entries[0].AllocationID != next.PoolAllocation.AllocationID {
		t.Fatal("successor changed")
	}
	auditOK(t, s.repo)
}

// Process identities come from an independently observed live owner tree, never
// from a command marker. No numeric process group is used as a signal target.
type psrProcessIdentity struct {
	pid, parent, group int
	started            string
}
type psrOwnedOrphan struct {
	cmd      *exec.Cmd
	done     chan error
	owner    psrProcessIdentity
	children map[int]psrProcessIdentity
	groups   map[int]bool
	joined   bool
	retired  bool
}

func psrProcessTable() (map[int]psrProcessIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw, e := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,ppid=,pgid=,lstart=").Output()
	if e != nil {
		return nil, e
	}
	rows := map[int]psrProcessIdentity{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 8 {
			return nil, fmt.Errorf("unrecognized process identity row %q", line)
		}
		pid, e := strconv.Atoi(fields[0])
		if e != nil {
			return nil, e
		}
		parent, e := strconv.Atoi(fields[1])
		if e != nil {
			return nil, e
		}
		group, e := strconv.Atoi(fields[2])
		if e != nil {
			return nil, e
		}
		rows[pid] = psrProcessIdentity{pid, parent, group, strings.Join(fields[3:], " ")}
	}
	return rows, nil
}
func psrSameProcess(a, b psrProcessIdentity) bool {
	return a.pid == b.pid && a.started == b.started && a.group == b.group
}
func psrOwnOrphan(t *testing.T, cmd *exec.Cmd) *psrOwnedOrphan {
	t.Helper()
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	o := &psrOwnedOrphan{cmd: cmd, done: make(chan error, 1), children: map[int]psrProcessIdentity{}, groups: map[int]bool{}}
	go func() { o.done <- cmd.Wait() }()
	// Register before any fallible identity/marker read. Cooperative termination
	// lets the native executor cancel and join its independently created group.
	t.Cleanup(func() { o.cleanup(t) })
	rows, e := psrProcessTable()
	if e != nil {
		t.Fatal("owner identity HOLD", e)
	}
	var ok bool
	o.owner, ok = rows[cmd.Process.Pid]
	if !ok {
		t.Fatal("owner identity absent HOLD")
	}
	return o
}
func (o *psrOwnedOrphan) cleanup(t *testing.T) {
	t.Helper()
	if o.retired {
		return
	}
	var observation error
	if !o.joined {
		observation = o.captureTree()
		if observation != nil {
			t.Errorf("cleanup observation HOLD: %v", observation)
		}
		// The retained os.Process handle remains the owner even when observation
		// fails; cooperative cancellation is still required, but cannot prove absence.
		if e := o.cmd.Process.Signal(syscall.SIGTERM); e != nil && e != os.ErrProcessDone {
			t.Errorf("owner termination HOLD: %v", e)
		}
		o.join(t)
	}
	if observation != nil {
		return
	}
	o.retire(t)
}
func (o *psrOwnedOrphan) capture(t *testing.T) {
	t.Helper()
	if e := o.captureTree(); e != nil {
		t.Fatal("process capture HOLD", e)
	}
}
func (o *psrOwnedOrphan) captureTree() error {
	rows, e := psrProcessTable()
	if e != nil {
		return e
	}
	root, ok := rows[o.cmd.Process.Pid]
	if !ok || (o.owner.pid != 0 && !psrSameProcess(root, o.owner)) {
		return fmt.Errorf("owner identity changed")
	}

	if o.owner.pid == 0 {
		o.owner = root
	}
	known := map[int]bool{root.pid: true}
	for changed := true; changed; {
		changed = false
		for pid, row := range rows {
			if !known[pid] && known[row.parent] {
				known[pid] = true
				o.children[pid] = row
				if row.group != root.group {
					o.groups[row.group] = true
				}
				changed = true
			}
		}
	}
	return nil
}

func (o *psrOwnedOrphan) join(t *testing.T) {
	t.Helper()
	if o.joined {
		return
	}
	select {
	case <-o.done:
		o.joined = true
	case <-time.After(35 * time.Second):
		t.Fatal("owner join HOLD")
	}
}
func (o *psrOwnedOrphan) retire(t *testing.T) {
	t.Helper()
	if o.retired {
		return
	}
	// Identity is checked immediately before every individual signal. A changed
	// identity is a failed proof, never permission to signal a recycled PID.
	for pid, identity := range o.children {
		rows, e := psrProcessTable()
		if e != nil {
			t.Fatal("retirement observation HOLD", e)
		}
		current, alive := rows[pid]
		if !alive {
			continue
		}
		if !psrSameProcess(current, identity) {
			t.Fatal("retirement identity changed HOLD", pid)
		}
		if e := syscall.Kill(pid, syscall.SIGKILL); e != nil && e != syscall.ESRCH {
			t.Fatal("retirement signal HOLD", e)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		rows, e := psrProcessTable()
		if e != nil {
			t.Fatal("absence observation HOLD", e)
		}
		remaining := false
		for pid, row := range rows {
			if _, owned := o.children[pid]; owned || o.groups[row.group] {
				remaining = true
			}
		}
		if !remaining {
			o.retired = true
			o.children = nil
			o.groups = nil
			t.Log("inner reset identities and groups ABSENT; cleanup obligation cleared")
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("inner retirement HOLD: identity/group remains")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPSROrphanEarlyInitializationCleanup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "runner")
	s, _ := psrFixtureConfigured(t, "echo $$ > "+marker+"; sleep 60", "printf verified", "verified", "1", false, func(v wire.Value) { _, r := psrReuse(v); r.Set("timeoutSeconds", str("20")) })
	cmd := exec.Command(os.Args[0], "-test.run=^TestPSRNativeOrphanChild$", "-test.timeout=60s")
	cmd.Dir = s.root
	cmd.Env = append(os.Environ(), "PSR_ORPHAN_REPO="+s.repo.PrimaryWorktree, "PSR_ORPHAN_ROOT="+s.root)
	cmd.WaitDelay = time.Second
	owned := psrOwnOrphan(t, cmd)
	psrAwaitMarker(t, marker)
	// Simulate failure before marker parsing. Ownership comes only from ps.
	owned.capture(t)
	if len(owned.children) < 2 || len(owned.groups) == 0 {
		t.Fatal("early failure fixture not reached")
	}
	owned.cleanup(t)
	if !owned.retired {
		t.Fatal("early initialization cleanup did not prove retirement")
	}
}

// PSR-V0-003: a same-request reconcile whose confirm lookup misses while the
// original runner is live must not end not-free when that runner commits the
// confirmation (releasing the owner) before the reconcile's owner snapshot.
func TestPSRReconcileOwnerReleaseRace(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "runs")
	s, _ := psrFixture(t, "printf x >> "+marker, "printf verified", "verified", "1", false)
	verified, resume, confirmed, finished := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var confirmOnce, finishOnce sync.Once
	wait := func(ch <-chan struct{}, what string) {
		select {
		case <-ch:
		case <-time.After(30 * time.Second):
			t.Error("interleaving not reached:", what)
		}
	}
	phase := func() string {
		raw, e := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
		if e != nil {
			return ""
		}
		p, e := snapshot.DecodePools(raw)
		if e != nil || len(p.Entries) != 1 || p.Entries[0].Sweep == nil {
			return ""
		}
		return p.Entries[0].Sweep.Phase
	}
	paused := false
	// The runner pauses after its verify receipt commits, before it confirms,
	// and again after confirming until the reconcile has written its finish.
	runner := store.PSRTestResponseFailure(context.Background(), func(c store.LeaseChoice, r *store.Report, e error) error {
		completed := e == nil && r != nil && r.Outcome.Outcome == mutation.OutcomeCompleted && r.Outcome.ReceiptSeq != nil
		if completed && !paused && c.Lease.Verb == transaction.LeasePoolObserve && phase() == "confirm" {
			paused = true
			close(verified)
			wait(resume, "reconcile confirm lookup")
		}
		if completed && c.Lease.Verb == transaction.LeasePoolSafe {
			confirmOnce.Do(func() { close(confirmed) })
			wait(finished, "reconcile finish")
		}
		return e
	})
	// The reconcile's confirm lookup misses; the runner then commits confirm
	// before the reconcile takes its owner snapshot.
	looked := false
	reconciler := store.PSRTestResponseFailure(context.Background(), func(c store.LeaseChoice, r *store.Report, e error) error {
		if looked && c.Lease.Verb == transaction.LeasePoolSweepFinish {
			finishOnce.Do(func() { close(finished) })
		}
		if !looked && c.Lease.Verb == transaction.LeasePoolSafe && (r == nil || r.Kind != "Replay") {
			looked = true
			close(resume)
			wait(confirmed, "runner confirmation")
		}
		return e
	})
	choice := store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "race", Root: s.root, TimeoutSeconds: "60"}
	done := make(chan psrOutcome, 1)
	go func() {
		r, e := store.PoolSweep(runner, s.repo, operator(), choice)
		done <- psrOutcome{r, e}
	}()
	wait(verified, "runner verify")
	b, e := store.PoolSweep(reconciler, s.repo, operator(), choice)
	if !looked {
		close(resume)
	}
	finishOnce.Do(func() { close(finished) })
	var a psrOutcome
	select {
	case a = <-done:
	case <-time.After(90 * time.Second):
		t.Fatal("runner did not return")
	}
	if !looked {
		t.Fatal("reconcile confirm lookup not reached")
	}
	free := func(name string, r *store.PoolSweepReport, err error) []byte {
		t.Helper()
		var row struct {
			Members []struct{ Free bool }
		}
		if err != nil || r == nil || r.Pending || json.Unmarshal(r.Result, &row) != nil || len(row.Members) != 1 || !row.Members[0].Free {
			if r == nil {
				r = &store.PoolSweepReport{}
			}
			t.Fatalf("%s did not finish free: pending=%v err=%v result=%s", name, r.Pending, err, r.Result)
		}
		return r.Result
	}
	if !bytes.Equal(free("reconcile", b, e), free("runner", a.report, a.err)) {
		t.Fatal("runner and reconcile results differ")
	}
	if p := psrPools(t, s); len(p.Entries) != 0 {
		t.Fatal("confirmed member not freed", p)
	}
	if raw, _ := os.ReadFile(marker); string(raw) != "x" {
		t.Fatal("reset not reached once", string(raw))
	}
}
