package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// attemptStore is one disposable store with one externally leased attempt
// whose five-minute lease has about three minutes left.
func attemptStore(t *testing.T) (string, *store.Report) {
	t.Helper()
	if !groupreap.OwnerAvailable() {
		t.Skip("attempt run needs the owned process group API")
	}
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-12*time.Minute))
	return root, claimed[0]
}

// runAttempt invokes the CLI without atm's 0-iff-OK check: an OK attempt run
// returns the command's own nonzero status.
func runAttempt(t *testing.T, root string, a *store.Report, generation string, extra []string, argv ...string) run {
	t.Helper()
	args := append([]string{"run", "--attempt", a.AttemptID, "--generation", generation}, extra...)
	args = append(append(args, "--"), argv...)
	var out, errb bytes.Buffer
	code := cli.Run(cli.Env{Cwd: root, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("%v: envelope does not decode: %v\nstdout=%s\nstderr=%s", args, err, out.Bytes(), errb.Bytes())
	}
	return run{code: code, stdout: out.Bytes(), stderr: errb.Bytes(), res: res}
}

func gen(a *store.Report) string { return string(a.Generation) }

// recordedOutcome reads the posted outcome document named by the envelope and
// checks it decodes and agrees with the envelope.
func recordedOutcome(t *testing.T, root string, r run) *transaction.RunOutcome {
	t.Helper()
	item := r.res.Items[0]
	digest := field(item, "outcomeSha256").Str
	if digest == "" || field(item, "outcomeReceipt").Str == "" {
		t.Fatalf("outcome not recorded: %+v", r.res)
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	_ = filepath.WalkDir(repo.StateDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == digest && filepath.Base(filepath.Dir(p)) == "evidence" {
			raw, _ = os.ReadFile(p)
		}
		return nil
	})
	if raw == nil {
		t.Fatalf("no evidence/%s under %s", digest, repo.StateDir)
	}
	o, err := transaction.DecodeRunOutcome(raw)
	if err != nil {
		t.Fatalf("posted outcome: %v", err)
	}
	if o.Class != field(item, "class").Str || o.Cleanup != field(item, "cleanup").Str || o.RunID != field(item, "runId").Str {
		t.Fatalf("posted outcome %+v disagrees with envelope %+v", o, r.res)
	}
	return o
}

func waitFile(t *testing.T, p string) {
	t.Helper()
	for i := 0; i < 500; i++ {
		if _, err := os.Stat(p); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", p)
}

// TestATRV0004_ExitStatusPassesThrough: the command's status, stdout and
// stderr are preserved; the envelope alone is on stdout.
func TestATRV0004_ExitStatusPassesThrough(t *testing.T) {
	root, a := attemptStore(t)
	for _, status := range []int{0, 7} {
		r := runAttempt(t, root, a, gen(a), []string{"--timeout", "30"}, "/bin/sh", "-c", "echo to-out; echo to-err >&2; exit "+strconv.Itoa(status))
		if r.code != status || r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("status %d: code %d %+v\n%s", status, r.code, r.res, r.stderr)
		}
		if !bytes.Contains(r.stderr, []byte("to-out\n")) || !bytes.Contains(r.stderr, []byte("to-err\n")) || bytes.Contains(r.stdout, []byte("to-out")) {
			t.Fatalf("output routing: stdout=%q stderr=%q", r.stdout, r.stderr)
		}
		o := recordedOutcome(t, root, r)
		if o.Class != transaction.RunExit || o.Cleanup != transaction.CleanupReleased || o.ExitCode == nil || *o.ExitCode != status || o.Signal != nil {
			t.Fatalf("outcome %+v", o)
		}
		// Pre-launch and final heartbeats; the lease already covered 30s.
		if o.Heartbeats != 2 || o.Renewals != 0 || o.AttemptID != a.AttemptID {
			t.Fatalf("beats %d renewals %d", o.Heartbeats, o.Renewals)
		}
	}
}

// TestATRV0004_SignalAndSpawnClasses: a self-signalled child and an
// unstartable command are distinct classes with reserved statuses.
func TestATRV0004_SignalAndSpawnClasses(t *testing.T) {
	root, a := attemptStore(t)
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "30"}, "/bin/sh", "-c", "kill -9 $$")
	if r.code != 128+9 || r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("signal: code %d %+v", r.code, r.res)
	}
	if o := recordedOutcome(t, root, r); o.Class != transaction.RunSignal || o.Signal == nil || *o.Signal != 9 || o.ExitCode != nil {
		t.Fatalf("signal outcome %+v", o)
	}
	r = runAttempt(t, root, a, gen(a), []string{"--timeout", "30"}, filepath.Join(t.TempDir(), "absent"))
	if r.code != 126 || r.res.Outcome != wire.OutcomeError || !hasCode(r.res, wire.CodeNoexec) {
		t.Fatalf("spawn: code %d %+v", r.code, r.res)
	}
	if o := recordedOutcome(t, root, r); o.Class != transaction.RunSpawnFailed || o.Cleanup != transaction.CleanupNotStarted || o.StartedAt != nil {
		t.Fatalf("spawn outcome %+v", o)
	}
}

// TestATRV0003_TimeoutRetiresDescendants: a timeout kills the whole owned
// group, including a backgrounded descendant, before cleanup is reported.
func TestATRV0003_TimeoutRetiresDescendants(t *testing.T) {
	root, a := attemptStore(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	start := time.Now()
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "1"}, "/bin/sh", "-c", "sleep 60 & echo $! > "+pidFile+"; wait")
	if r.code != 124 || r.res.Outcome != wire.OutcomeRefused || !hasCode(r.res, wire.CodeLimitExceeded) {
		t.Fatalf("timeout: code %d %+v", r.code, r.res)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
	if o := recordedOutcome(t, root, r); o.Class != transaction.RunTimeout || o.Cleanup != transaction.CleanupReleased {
		t.Fatalf("timeout outcome %+v", o)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; ; i++ {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			break
		}
		if i > 200 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("descendant %d survived the timeout", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestATRV0002_RefusedAuthorityStartsNothing: a stale generation is refused
// at the pre-launch heartbeat; the command never starts and nothing is
// recorded as its outcome.
func TestATRV0002_RefusedAuthorityStartsNothing(t *testing.T) {
	root, a := attemptStore(t)
	marker := filepath.Join(t.TempDir(), "ran")
	stale := strconv.FormatUint(a.Generation.Uint64()+1, 10)
	r := runAttempt(t, root, a, stale, []string{"--timeout", "30"}, "/bin/sh", "-c", "touch "+marker)
	if r.code != 125 || r.res.Outcome != wire.OutcomeRefused || !hasCode(r.res, wire.CodeFenced) {
		t.Fatalf("stale: code %d %+v", r.code, r.res)
	}
	if _, err := os.Stat(marker); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("command ran under refused authority: %v", err)
	}
	if len(r.res.Items) != 0 {
		t.Fatalf("pre-launch refusal reported an outcome: %+v", r.res.Items)
	}
}

// TestATRV0002_RenewsOnlyWhenCoverageIsShort: a timeout beyond the current
// lease renews once before launch; a covered timeout renews nothing.
func TestATRV0002_RenewsOnlyWhenCoverageIsShort(t *testing.T) {
	root, a := attemptStore(t)
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "120", "--lease-minutes", "5"}, "/bin/sh", "-c", "exit 0")
	if r.code != 0 {
		t.Fatalf("renew: code %d %+v", r.code, r.res)
	}
	if o := recordedOutcome(t, root, r); o.Renewals != 1 {
		t.Fatalf("renewals %d, want 1", o.Renewals)
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := store.AttemptRecord(context.Background(), repo, a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	expires, err := time.Parse("2006-01-02T15:04:05Z", string(rec.Lease.ExpiresAt))
	if err != nil || time.Until(expires) < 250*time.Second {
		t.Fatalf("renewed lease expires %s (%v)", rec.Lease.ExpiresAt, err)
	}
	// The renewed lease now covers a short timeout; nothing shortens it.
	r = runAttempt(t, root, a, gen(a), []string{"--timeout", "5"}, "/bin/sh", "-c", "exit 0")
	if o := recordedOutcome(t, root, r); r.code != 0 || o.Renewals != 0 {
		t.Fatalf("covered run renewed: code %d %+v", r.code, o)
	}
	after, err := store.AttemptRecord(context.Background(), repo, a.AttemptID)
	if err != nil || after.Lease.ExpiresAt < rec.Lease.ExpiresAt {
		t.Fatalf("lease shortened from %s to %+v (%v)", rec.Lease.ExpiresAt, after, err)
	}
}

// TestATRV0006_LostLeaseKillsTheRun: a heartbeat refused mid-run stops the
// group, and the fenced run's outcome is still recorded.
func TestATRV0006_LostLeaseKillsTheRun(t *testing.T) {
	root, a := attemptStore(t)
	defer cli.SetAttemptBeatInterval(200 * time.Millisecond)()
	started := filepath.Join(t.TempDir(), "started")
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 500; i++ {
			if _, err := os.Stat(started); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		repo, err := intent.Resolve(root)
		if err != nil {
			done <- err
			return
		}
		lease := transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation}
		report, err := store.Lease(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "release-under-run", Root: root, Lease: lease, Derive: store.NoScopeDeriver}, wire.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z")))
		if err == nil && report.Outcome.Outcome != mutation.OutcomeCompleted {
			err = errors.New("release refused: " + report.Detail)
		}
		done <- err
	}()
	start := time.Now()
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "60"}, "/bin/sh", "-c", "touch "+started+"; sleep 60")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if r.code != 125 || r.res.Outcome != wire.OutcomeRefused || !hasCode(r.res, wire.CodeFenced) {
		t.Fatalf("lost lease: code %d %+v\n%s", r.code, r.res, r.stderr)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("lost lease took %s", elapsed)
	}
	o := recordedOutcome(t, root, r)
	if o.Class != transaction.RunLostLease || o.Cleanup != transaction.CleanupReleased || o.LostLease == nil || *o.LostLease != wire.CodeFenced {
		t.Fatalf("lost-lease outcome %+v", o)
	}
}

// TestATRV0004_InterruptStopsTheRun: SIGTERM to the runner stops the group
// and returns 128+15.
func TestATRV0004_InterruptStopsTheRun(t *testing.T) {
	root, a := attemptStore(t)
	started := filepath.Join(t.TempDir(), "started")
	go func() {
		waitFile(t, started)
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "60"}, "/bin/sh", "-c", "touch "+started+"; sleep 60")
	if r.code != 128+15 || r.res.Outcome != wire.OutcomeRefused {
		t.Fatalf("interrupt: code %d %+v", r.code, r.res)
	}
	if o := recordedOutcome(t, root, r); o.Class != transaction.RunInterrupted || o.Signal == nil || *o.Signal != 15 || o.ExitCode != nil {
		t.Fatalf("interrupt outcome %+v", o)
	}
}

// TestATRV0001_UsageRefusesBeforeEffects: closed flags, a delimiter and a
// bounded timeout are required; program mode is unchanged.
func TestATRV0001_UsageRefusesBeforeEffects(t *testing.T) {
	root, a := attemptStore(t)
	for _, args := range [][]string{
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "5", "--program", "p", "--", "true"},
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "5"},
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "5", "--"},
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "86271", "--", "true"},
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "0", "--", "true"},
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "5", "--lease-minutes", "4", "--", "true"},
		{"run", "--attempt", a.AttemptID, "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "5", "--", "true"},
		{"run", "--attempt", "attempt:other:main:1", "--generation", gen(a), "--timeout", "5", "--", "true"},
	} {
		r := atm(t, root, nil, args...)
		if r.code != 1 || r.res.Outcome == wire.OutcomeOK || len(r.res.Items) != 0 {
			t.Fatalf("%v: code %d %+v", args, r.code, r.res)
		}
	}
	// `--attempt` after the delimiter belongs to program mode's argv.
	r := atm(t, root, nil, "run", "--program", "p", "--", "--attempt")
	if r.res.Outcome == wire.OutcomeOK {
		t.Fatalf("program mode accepted: %+v", r.res)
	}
}

// TestATRV0005_OutcomeBindsItsAttemptGeneration: the RUN_OUTCOME verb posts
// only a document naming its own attempt and a generation it has had.
func TestATRV0005_OutcomeBindsItsAttemptGeneration(t *testing.T) {
	root, a := attemptStore(t)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	actor := mutation.Binding{ID: "tester", Role: "OPERATOR"}
	now := wire.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	doc := func(attempt, generation string) []byte {
		started := string(now)
		code := 0
		raw, err := transaction.EncodeRunOutcome(transaction.RunOutcome{Profile: transaction.RunOutcomeProfile, RunID: "r1", AttemptID: attempt, Generation: generation, ArgvSha256: strings.Repeat("a", 64), TimeoutSeconds: 1, StartedAt: &started, EndedAt: started, Class: transaction.RunExit, ExitCode: &code, Cleanup: transaction.CleanupReleased})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	next := wire.SizeOf(a.Generation.Uint64() + 1)
	for name, c := range map[string]struct {
		generation wire.Size
		raw        []byte
	}{
		"future generation": {next, doc(a.AttemptID, string(next))},
		"other attempt":     {a.Generation, doc("attempt:x:main:9", gen(a))},
		"other generation":  {a.Generation, doc(a.AttemptID, string(next))},
	} {
		report, err := store.RecordRunOutcome(context.Background(), repo, actor, fixture.QueueID, "outcome-"+strings.ReplaceAll(name, " ", "-"), a.AttemptID, c.generation, c.raw, now)
		if err == nil && report.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatalf("%s: recorded %+v", name, report)
		}
	}
	report, err := store.RecordRunOutcome(context.Background(), repo, actor, fixture.QueueID, "outcome-ok", a.AttemptID, a.Generation, doc(a.AttemptID, gen(a)), now)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("valid outcome: %+v %v", report, err)
	}
	// An observation changes neither the attempt nor its lease.
	rec, err := store.AttemptRecord(context.Background(), repo, a.AttemptID)
	if err != nil || rec.Generation != a.Generation || !rec.Live() {
		t.Fatalf("attempt after outcome: %+v %v", rec, err)
	}
}

// TestATRV0005_OutcomeDocumentIsClosed: non-canonical, unknown-key and
// contradictory documents are refused.
func TestATRV0005_OutcomeDocumentIsClosed(t *testing.T) {
	started := "2026-10-04T00:00:00Z"
	code := 0
	base := transaction.RunOutcome{Profile: transaction.RunOutcomeProfile, RunID: "r1", AttemptID: "a", Generation: "1", ArgvSha256: strings.Repeat("a", 64), TimeoutSeconds: 1, StartedAt: &started, EndedAt: started, Class: transaction.RunExit, ExitCode: &code, Cleanup: transaction.CleanupReleased}
	raw, err := transaction.EncodeRunOutcome(base)
	if err != nil {
		t.Fatal(err)
	}
	bad := [][]byte{
		bytes.TrimSuffix(raw, []byte("\n")),
		bytes.Replace(raw, []byte(`{"profile"`), []byte(`{"extra":1,"profile"`), 1),
		bytes.Replace(raw, []byte(`"class":"EXIT"`), []byte(`"class":"SPAWN_FAILED"`), 1),
		bytes.Replace(raw, []byte(`"cleanup":"RELEASED"`), []byte(`"cleanup":"NOT_STARTED"`), 1),
		bytes.Replace(raw, []byte(`"signal":null`), []byte(`"signal":9`), 1),
		bytes.Replace(raw, []byte(`"lostLease":null`), []byte(`"lostLease":"NOPE"`), 1),
	}
	for i, b := range bad {
		if _, err := transaction.DecodeRunOutcome(b); err == nil {
			t.Fatalf("case %d accepted: %s", i, b)
		}
	}
}
