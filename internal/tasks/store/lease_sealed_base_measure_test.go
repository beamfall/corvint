//go:build darwin || linux

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestLGLV0011_PoolReleaseLatency is the opt-in LGL-V0-011 measurement, not
// a timing assertion; a plain `go test` skips it. CORVINT_TASKS_LGL_RELEASE
// names the JSON report. CORVINT_TASKS_LGL_RELEASE_STORE is the directory
// the sealed history is built in (a real disk, not tmpfs: V1-2563), kept
// for later runs; CORVINT_TASKS_LGL_RELEASE_RECEIPTS is its size (default
// 47000) and CORVINT_TASKS_LGL_RELEASE_ITERATIONS the acquire and release
// pairs (default 20). Each run restores the history, takes the store to a
// writer checkpoint whose FullSeq is the sealed sequence, as writer-route
// traffic leaves it, then times each Lease call with the in-process audit
// cache cleared, as a fresh CLI process would.
func TestLGLV0011_PoolReleaseLatency(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_LGL_RELEASE")
	if output == "" {
		t.Skip("set CORVINT_TASKS_LGL_RELEASE to a report path")
	}
	base := os.Getenv("CORVINT_TASKS_LGL_RELEASE_STORE")
	if base == "" {
		t.Fatal("set CORVINT_TASKS_LGL_RELEASE_STORE to a real-disk directory")
	}
	receipts, err := strconv.Atoi(envOr("CORVINT_TASKS_LGL_RELEASE_RECEIPTS", "47000"))
	if err != nil || receipts < 1100 {
		t.Fatal("CORVINT_TASKS_LGL_RELEASE_RECEIPTS")
	}
	iterations, err := strconv.Atoi(envOr("CORVINT_TASKS_LGL_RELEASE_ITERATIONS", "20"))
	if err != nil || iterations < 1 || iterations > 200 {
		t.Fatal("CORVINT_TASKS_LGL_RELEASE_ITERATIONS")
	}
	dir := filepath.Join(base, strconv.Itoa(receipts))
	pristine, live, ready := filepath.Join(dir, "pristine"), filepath.Join(dir, "live"), filepath.Join(dir, "ready")
	if _, err := os.Stat(ready); err != nil {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		built := time.Now()
		repo := historyStoreAt(t, filepath.Join(live, "repo"), historyTickets, receipts)
		if err := sealSegmentsLocked(context.Background(), repo); err != nil {
			t.Fatal(err)
		}
		t.Logf("built %d receipts in %v", receipts, time.Since(built))
		if out, err := exec.Command("cp", "-a", live, pristine).CombinedOutput(); err != nil {
			t.Fatalf("cp -a: %v %s", err, out)
		}
		if err := os.WriteFile(ready, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(live); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-a", pristine, live).CombinedOutput(); err != nil {
		t.Fatalf("cp -a: %v %s", err, out)
	}
	repo, err := intent.Resolve(filepath.Join(live, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	prepared := time.Now()
	if run := writerMutate(t, repo, "lgl-seed", nil); !run.completed() {
		t.Fatalf("seed: %+v %v", run.rep, run.err)
	}
	f := resumeFixtureOn(t, repo)
	fast := func(run writerRun) {
		t.Helper()
		if !run.completed() || !run.fast() {
			t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
		}
	}
	for i := 0; ; i++ {
		if wc := readWriterCheckpoint(repo); wc != nil && wc.FullSeq == segmentThrough(t, repo) {
			break
		}
		if i == 3 {
			t.Fatalf("FullSeq %+v never reached the sealed sequence %d", readWriterCheckpoint(repo), segmentThrough(t, repo))
		}
		padToSeal(t, repo, fast)
		fast(writerMutate(t, repo, fmt.Sprintf("lgl-converge-%d", i), nil))
	}
	f.now = WallClock()
	t.Logf("prepared in %v at head %d, sealed through %d", time.Since(prepared), headSeqOf(t, repo), segmentThrough(t, repo))
	type sample struct {
		Verb                       string
		WallMS                     float64
		Audits, Resumed, Declines  int
		Completed, Fast            bool
		LockHoldMS, SnapshotReadMS float64
	}
	var samples []sample
	timed := func(verb string, write func(context.Context) (*Report, error)) *Report {
		started := time.Now()
		run := sealedRun(t, f, write, nil)
		samples = append(samples, sample{Verb: verb, WallMS: ms(time.Since(started)), Audits: run.audits, Resumed: run.timing.Resumed, Declines: run.timing.ResumeDeclines, Completed: run.completed(), Fast: run.fast(), LockHoldMS: ms(run.timing.LockHold), SnapshotReadMS: ms(run.timing.SnapshotRead)})
		if !run.completed() {
			t.Fatalf("%s: %v %s %v", verb, run.err, run.rep.Detail, run.rep.Outcome.Codes)
		}
		return run.rep
	}
	loadBefore := hostLoad()
	for i := 0; i < iterations; i++ {
		// One allocation per attempt generation: each pair runs on a claim
		// of its own, created and claimed on the writer route, untimed.
		created := stagedWrite(writerCreate(repo, fmt.Sprintf("lgl-ticket-%d", i), f.now), nil)
		attemptEntropies.Store(repo.StateDir, fixedEntropy(byte(40+i)))
		claim := stagedWrite(f.lease(fmt.Sprintf("lgl-claim-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: fmt.Sprintf("agent-lgl-%d", i), LeaseMinutes: "60", Scope: []string{fmt.Sprintf("lgl/%d/", i)}}), nil)
		if !created.completed() || !claim.completed() {
			t.Fatalf("claim %d: %v / %v %s %v", i, created.err, claim.err, claim.rep.Detail, claim.rep.Outcome.Codes)
		}
		f.claim = claim.rep
		acquired := timed("pool-acquire", f.acquire(fmt.Sprintf("lgl-acquire-%d", i)))
		allocation := acquired.PoolAllocation
		timed("pool-release", f.release(fmt.Sprintf("lgl-release-%d", i), string(allocation.AllocationID)))
		// A released member stays quarantined until it is confirmed safe;
		// the confirmation takes the writer route, untimed.
		safe := stagedWrite(f.lease(fmt.Sprintf("lgl-safe-%d", i), transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: allocation.MemberID, Allocation: string(allocation.AllocationID), Evidence: "lgl-harness-reset", Reason: "measurement member reset"}), nil)
		if !safe.completed() {
			t.Fatalf("confirm safe %d: %v %s %v", i, safe.err, safe.rep.Detail, safe.rep.Outcome.Codes)
		}
		// Each claim is released, untimed, so the live attempts stay under
		// the policy's maxActiveAttempts.
		done := stagedWrite(f.lease(fmt.Sprintf("lgl-done-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation}), nil)
		if !done.completed() {
			t.Fatalf("release claim %d: %v %s %v", i, done.err, done.rep.Detail, done.rep.Outcome.Codes)
		}
	}
	loadAfter := hostLoad()
	report := map[string]any{"ticket": "V1-1702", "requirement": "LGL-V0-011", "receipts": receipts, "iterations": iterations, "host": b3Host(), "tmp": live,
		"loadBefore": loadBefore, "loadAfter": loadAfter, "qualification": b3Qualification([]float64{b3ParseLoad1(loadBefore), b3ParseLoad1(loadAfter)}, runtime.NumCPU()),
		"statistic": "nearest-rank percentiles of wall time per Lease call, in-process audit cache cleared before every call"}
	stats := map[string]b3Stats{}
	for _, verb := range []string{"pool-acquire", "pool-release"} {
		var own []b3Sample
		for _, s := range samples {
			if s.Verb == verb {
				own = append(own, b3Sample{Verb: verb, WallMS: s.WallMS, Audits: s.Audits, Resumed: s.Resumed, ResumeDeclines: s.Declines, LockHoldMS: s.LockHoldMS, SnapshotReadMS: s.SnapshotReadMS})
			}
		}
		st := b3Summarize(own)
		stats[verb] = st
		t.Logf("receipts=%d %-12s n=%d p50=%.0f p95=%.0f p99=%.0f max=%.0f ms audits=%d resumed=%d lockHold.p50=%.0f ms load=%s / %s", receipts, verb, st.Calls, st.P50MS, st.P95MS, st.P99MS, st.MaxMS, st.Audits, st.Resumed, st.PhaseP50MS["lockHold"], loadBefore, loadAfter)
	}
	report["stats"], report["samples"] = stats, samples
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
