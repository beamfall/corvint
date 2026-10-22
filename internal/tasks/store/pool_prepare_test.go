//go:build darwin || linux

package store_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// psrPrepare declares memberConfig.prepare with the V1-1324 load-tolerant
// attempt bound; attemptSeconds overrides that bound when nonempty.
func psrPrepare(script, attemptSeconds string) func(wire.Value) {
	return func(v wire.Value) {
		psrLoadTolerant(v)
		member, reuse := psrReuse(v)
		if attemptSeconds != "" {
			reuse.Set("timeoutSeconds", str(attemptSeconds))
		}
		member.Set("prepare", obj("argv", wire.Strings([]string{"/bin/sh", "-c", script}), "env", wire.Strings(nil), "cwd", str("REPOSITORY"), "timeoutSeconds", str("3600")))
	}
}

// psrAwaitFile waits for a nonempty file or the sweep's own return; the bound
// only detects a hang.
func psrAwaitFile(t *testing.T, path string, done <-chan psrOutcome) {
	t.Helper()
	end := time.Now().Add(4 * time.Minute)
	for time.Now().Before(end) {
		if info, e := os.Stat(path); e == nil && info.Size() > 0 {
			return
		}
		select {
		case r := <-done:
			t.Fatalf("sweep ended before %s: %+v %v", path, r.report, r.err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatal("command boundary not reached", path)
}

func psrPhaseObservation(t *testing.T, s *leaseStore, phase string) []snapshot.PoolSweepObservation {
	t.Helper()
	out := []snapshot.PoolSweepObservation{}
	for _, o := range psrObservations(t, s) {
		if o.Phase == phase {
			out = append(out, o)
		}
	}
	return out
}

// PSR-V0-024, PSR-V0-025: a declared prepare runs after a passed verify while
// the sweep still owns the member (never FREE), passes before confirmation,
// retains only its output digests, and an exact replay never reruns it.
func TestPSRV0025PrepareRunsOwnedBeforeConfirm(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	runs, resume := filepath.Join(dir, "runs"), filepath.Join(dir, "resume")
	script := "printf x >> " + runs + "; while [ ! -f " + resume + " ]; do /bin/sleep 0.01; done; printf prepared-stdout-bytes; printf prepared-stderr-bytes >&2"
	s, a := psrFixtureConfigured(t, "printf reset", "printf verified", "verified", "1", true, psrPrepare(script, ""))
	done := make(chan psrOutcome, 1)
	go func() {
		r, e := psrRunLoaded(s, "prepare-sweep")
		done <- psrOutcome{r, e}
	}()
	psrAwaitFile(t, runs, done)
	held := psrPools(t, s)
	if len(held.Entries) != 1 || held.Entries[0].State != "CLEANING" || held.Entries[0].Sweep == nil || held.Entries[0].Sweep.Phase != "prepare" || held.Entries[0].AllocationID != a.PoolAllocation.AllocationID {
		t.Fatalf("prepare ran outside the owned quarantine: %s", psrEntryStates(held))
	}
	if e := os.WriteFile(resume, nil, 0600); e != nil {
		t.Fatal(e)
	}
	var result psrOutcome
	select {
	case result = <-done:
	case <-time.After(4 * time.Minute):
		t.Fatal("sweep unjoined")
	}
	if result.err != nil || result.report.Pending || !bytes.Contains(result.report.Result, []byte(`"free":true`)) {
		t.Fatalf("prepared sweep: %+v %v pool=%s", result.report, result.err, psrEntryStates(psrPools(t, s)))
	}
	if len(psrPools(t, s).Entries) != 0 {
		t.Fatal("prepared member not freed")
	}
	verify, prepare := psrPhaseObservation(t, s, "verify"), psrPhaseObservation(t, s, "prepare")
	if len(verify) != 1 || len(prepare) != 1 || !prepare[0].Passed || prepare[0].Class != "EXIT_ZERO" || prepare[0].Previous == nil {
		t.Fatalf("observations verify=%+v prepare=%+v", verify, prepare)
	}
	if *prepare[0].Previous != wire.Sum(verify[0].Encode()) {
		t.Fatal("prepare does not follow the passed verify")
	}
	// PSR-V0-025: digest-only output; the log evidence holds no output bytes.
	if prepare[0].Stdout != wire.Sum([]byte("prepared-stdout-bytes")) || prepare[0].Stderr != wire.Sum([]byte("prepared-stderr-bytes")) || prepare[0].Log != wire.Sum(snapshot.EncodePoolSweepLog(nil, nil)) {
		t.Fatalf("prepare digests %+v", prepare[0])
	}
	evidence, e := os.ReadDir(filepath.Join(s.repo.StateDir, "evidence"))
	if e != nil {
		t.Fatal(e)
	}
	for _, en := range evidence {
		raw, e := os.ReadFile(filepath.Join(s.repo.StateDir, "evidence", en.Name()))
		if e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(raw, []byte("prepared-stdout-bytes")) || bytes.Contains(raw, []byte("prepared-stderr-bytes")) {
			t.Fatal("prepare output retained in evidence", en.Name())
		}
	}
	replay, e := psrRunLoaded(s, "prepare-sweep")
	if e != nil || replay.Pending || !bytes.Equal(replay.Result, result.report.Result) {
		t.Fatalf("replay %+v %v", replay, e)
	}
	if raw, _ := os.ReadFile(runs); string(raw) != "x" {
		t.Fatal("prepare rerun", string(raw))
	}
}

// PSR-V0-026: a failed prepare is terminal: no retry even with maxAttempts 2,
// the member stays QUARANTINED with a recorded reason and is never freed.
// PSR-V0-025: prepare shares the attempt deadline, so a prepare whose own
// bound is 3600s ends as TIMEOUT at the member's attempt deadline.
func TestPSRV0026PrepareFailureStaysQuarantined(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, script, attempt, class string }{
		{"exit-nonzero", "exit 3", "", "EXIT_NONZERO"},
		{"shared-deadline", "/bin/sleep 300", "20", "TIMEOUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			resets, runs := filepath.Join(dir, "resets"), filepath.Join(dir, "runs")
			s, _ := psrFixtureConfigured(t, "printf x >> "+resets, "printf verified", "verified", "2", false, psrPrepare("printf x >> "+runs+"; "+tc.script, tc.attempt))
			started := time.Now()
			out, e := psrRunLoaded(s, "prepare-fails")
			if e != nil || out.Pending || !bytes.Contains(out.Result, []byte(`"free":false`)) {
				t.Fatalf("result %+v %v pool=%s", out, e, psrEntryStates(psrPools(t, s)))
			}
			if elapsed := time.Since(started); tc.class == "TIMEOUT" && elapsed > 200*time.Second {
				t.Fatalf("prepare outlived the shared deadline: %v", elapsed)
			}
			p := psrPools(t, s)
			if len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" || p.Entries[0].Sweep != nil || p.Entries[0].Reason != tc.class+"; pool prepare failed" {
				t.Fatalf("not quarantined with reason: %s", psrEntryStates(p))
			}
			prepare := psrPhaseObservation(t, s, "prepare")
			if len(prepare) != 1 || prepare[0].Passed || prepare[0].Class != tc.class || prepare[0].Attempt != "1" {
				t.Fatalf("prepare observation %+v", prepare)
			}
			for path, want := range map[string]string{resets: "x", runs: "x"} {
				if raw, _ := os.ReadFile(path); string(raw) != want {
					t.Fatalf("%s ran %q, want once", filepath.Base(path), raw)
				}
			}
			replay, e := psrRunLoaded(s, "prepare-fails")
			if e != nil || !bytes.Equal(replay.Result, out.Result) {
				t.Fatalf("replay %+v %v", replay, e)
			}
		})
	}
}

// PSR-V0-026: when the committed prepare observation's response is lost, the
// same-request replay reconciles from receipts and stays pending while the
// sweep still owns the member; it never re-executes the completed prepare.
func TestPSRV0026PendingReplayNeverRerunsPrepare(t *testing.T) {
	t.Parallel()
	runs := filepath.Join(t.TempDir(), "runs")
	s, _ := psrFixtureConfigured(t, "printf reset", "printf verified", "verified", "1", false, psrPrepare("printf x >> "+runs, ""))
	lost := false
	ctx := store.PSRTestResponseFailure(psrLoaded(context.Background()), func(c store.LeaseChoice, r *store.Report, e error) error {
		if !lost && c.Lease.Verb == transaction.LeasePoolObserve && e == nil && r != nil && r.Outcome.Outcome == mutation.OutcomeCompleted && psrOwnerPhase(t, s) == "confirm" {
			lost = true
			return fmt.Errorf("injected committed prepare response loss")
		}
		return e
	})
	out, e := store.PoolSweep(ctx, s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "prepare-lost", Root: s.root, TimeoutSeconds: psrLoadedSweepSeconds})
	if e == nil || !lost || !out.Unretryable {
		t.Fatalf("response loss not reached: %+v %v lost=%v", out, e, lost)
	}
	if len(psrPhaseObservation(t, s, "prepare")) != 1 || psrOwnerPhase(t, s) != "confirm" {
		t.Fatalf("prepare not committed: %s", psrEntryStates(psrPools(t, s)))
	}
	replay, e := psrRunLoaded(s, "prepare-lost")
	if e != nil || !replay.Pending {
		t.Fatalf("replay %+v %v", replay, e)
	}
	if raw, _ := os.ReadFile(runs); string(raw) != "x" {
		t.Fatal("completed prepare re-executed", string(raw))
	}
	if p := psrPools(t, s); len(p.Entries) != 1 || p.Entries[0].State == "FREE" {
		t.Fatal("pending replay freed the member", psrEntryStates(p))
	}
}

// PSR-V0-025: under an ALL barrier a declared prepare is a further phase
// command, so it never launches; the owned sweep ends quarantined.
func TestPSRV0025PrepareNeverLaunchesUnderAll(t *testing.T) {
	t.Parallel()
	runs := filepath.Join(t.TempDir(), "runs")
	s, _ := psrFixtureConfigured(t, "printf reset", "printf verified", "verified", "1", false, psrPrepare("printf x >> "+runs, ""))
	barrier := false
	ctx := store.PSRTestResponseFailure(psrLoaded(context.Background()), func(c store.LeaseChoice, r *store.Report, e error) error {
		if !barrier && c.Lease.Verb == transaction.LeasePoolObserve && e == nil && r != nil && r.Outcome.Outcome == mutation.OutcomeCompleted && psrOwnerPhase(t, s) == "prepare" {
			barrier = true
			reconciliationBarrier(t, s.repo, "ALL")
		}
		return e
	})
	out, e := store.PoolSweep(ctx, s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "prepare-all", Root: s.root, TimeoutSeconds: psrLoadedSweepSeconds})
	if e != nil || !barrier || out.Pending || !bytes.Contains(out.Result, []byte(`"free":false`)) {
		t.Fatalf("result %+v %v barrier=%v", out, e, barrier)
	}
	if _, e := os.Stat(runs); !os.IsNotExist(e) {
		t.Fatal("prepare launched under ALL", e)
	}
	p := psrPools(t, s)
	if len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" || !strings.Contains(p.Entries[0].Reason, "ALL barrier") {
		t.Fatal("ALL sweep not terminal quarantine", psrEntryStates(p))
	}
}
