//go:build darwin || linux

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// attachPublishFixture holds a simulated supervisor's publish immediately
// after receipt link-in and before head rename, beside a valid finished run.
func attachPublishFixture(t *testing.T) (*fixture.Repo, []string, []byte) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	s, err := snapshot.Probe(r.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	const attemptID = "attempt:acme:main:0123456789abcdef0123456789abcdef"
	const runID = "0123456789abcdef"
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(runsDir(repo, attemptID), runID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := (&wire.Result{Command: []string{"run"}, Outcome: wire.OutcomeOK}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	ended, status, digest := "2026-10-10T00:00:01Z", 0, string(wire.Sum(result))
	rec := &runRecord{
		Profile: runRecordProfile, RunID: runID, AttemptID: attemptID, Generation: "1",
		ArgvSha256: strings.Repeat("0", 64), TimeoutSeconds: 60, State: runFinished,
		LaunchedAt: "2026-10-10T00:00:00Z", SupervisorPid: 1, SupervisorIdentity: "finished",
		EndedAt: &ended, ExitStatus: &status, ResultSha256: &digest,
	}
	if err := writeRunFile(dir, runResultName, result); err != nil {
		t.Fatal(err)
	}
	if err := writeRunRecord(dir, rec); err != nil {
		t.Fatal(err)
	}
	args := []string{"run", "--attach", "--attempt", attemptID, "--run", runID, "--wait", "0"}
	var out bytes.Buffer
	if code := Run(Env{Cwd: r.Root, Args: args, Stdout: &out}); code != 0 || !bytes.Equal(out.Bytes(), result) {
		t.Fatalf("quiet attach fixture: code=%d output=%s", code, out.Bytes())
	}
	// Link canonical receipt bytes, just as the writer's publish does. The
	// head stays at seq 1 until the reader's sleep hook releases the writer.
	staged := filepath.Join(r.CommonDir, "held-receipt.json")
	raw := wire.EncodeFile(fixture.ReceiptValue(2, s.Head.LastReceiptSha256, "MUTATION", s.Head.Generation.Uint64()))
	if _, err := snapshot.DecodeReceipt(raw); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	name, err := snapshot.ReceiptName(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Link(staged, filepath.Join(r.StateDir, "receipts", name)); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot.Probe(r.StateDir); wire.CodeOf(err) != wire.CodeRedoPending {
		t.Fatalf("link-before-rename fixture did not reach REDO_PENDING: %v", err)
	}
	return r, args, result
}

// TestCTSV0009_AttachWaitsForReceiptLinkBeforeHeadRename proves CTS-V0-009
// without a timing race: the held writer finishes only from the read pause.
func TestCTSV0009_AttachWaitsForReceiptLinkBeforeHeadRename(t *testing.T) {
	r, args, result := attachPublishFixture(t)
	before := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	runsBefore := fixture.TreeSnapshot(t, filepath.Join(r.CommonDir, "taskman-runs"))
	oldSleep := attachReadSleep
	t.Cleanup(func() { attachReadSleep = oldSleep })
	pauses := 0
	var afterWriter []fixture.Entry
	attachReadSleep = func(d time.Duration) {
		pauses++
		if pauses != 1 || d != 25*time.Millisecond {
			t.Fatalf("unexpected read pause %d: %s", pauses, d)
		}
		if err := applyHead(r, 2); err != nil {
			t.Fatal(err)
		}
		afterWriter = fixture.TreeSnapshot(t, r.StateDir)
	}
	withReadPatience(t, 2*time.Second, func() {
		var out bytes.Buffer
		if code := Run(Env{Cwd: r.Root, Args: args, Stdout: &out}); code != 0 || !bytes.Equal(out.Bytes(), result) {
			t.Fatalf("attach after writer finished: code=%d output=%s", code, out.Bytes())
		}
	})
	if pauses != 1 || fixture.SameTree(before, afterWriter) {
		t.Fatal("attach did not wait for the writer's head rename")
	}
	s, err := snapshot.Probe(r.StateDir)
	if err != nil || s.Head.LastSeq != "2" {
		t.Fatalf("writer did not settle at seq 2: %+v %v", s, err)
	}
	fixture.AssertUntouched(t, r, afterWriter, intentBefore, "attach after writer's rename")
	if !fixture.SameTree(runsBefore, fixture.TreeSnapshot(t, filepath.Join(r.CommonDir, "taskman-runs"))) {
		t.Fatal("attach changed run files")
	}
}

// TestCTSV0009_AttachReportsPendingAfterPatience proves CTS-V0-009 retains
// REDO_PENDING when the held writer does not complete within the budget.
func TestCTSV0009_AttachReportsPendingAfterPatience(t *testing.T) {
	r, args, _ := attachPublishFixture(t)
	before := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	runsBefore := fixture.TreeSnapshot(t, filepath.Join(r.CommonDir, "taskman-runs"))
	oldSleep := attachReadSleep
	t.Cleanup(func() { attachReadSleep = oldSleep })
	pauses := 0
	attachReadSleep = func(d time.Duration) { pauses++; time.Sleep(d) }
	const budget = 50 * time.Millisecond
	withReadPatience(t, budget, func() {
		var out bytes.Buffer
		start := time.Now()
		code := Run(Env{Cwd: r.Root, Args: args, Stdout: &out})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("attach envelope: %v: %s", err, out.Bytes())
		}
		if code != 1 || res.Outcome != wire.OutcomeError || len(res.Codes) != 1 || res.Codes[0] != wire.CodeRedoPending {
			t.Fatalf("pending attach: code=%d result=%+v", code, res)
		}
		warnings := strings.Join(res.Warnings, " ")
		if pauses == 0 || time.Since(start) < budget || !strings.Contains(warnings, "pauses") || !strings.Contains(warnings, "retryable") {
			t.Fatalf("attach did not report its exhausted wait: pauses=%d warnings=%s", pauses, warnings)
		}
	})
	fixture.AssertUntouched(t, r, before, intentBefore, "pending attach")
	if !fixture.SameTree(runsBefore, fixture.TreeSnapshot(t, filepath.Join(r.CommonDir, "taskman-runs"))) {
		t.Fatal("pending attach changed run files")
	}
}
