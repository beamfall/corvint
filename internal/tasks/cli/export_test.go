package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SetAttemptBeatInterval shortens the attempt runner's heartbeat interval for
// a test and returns the restore function.
func SetAttemptBeatInterval(d time.Duration) func() {
	was := attemptBeatInterval
	attemptBeatInterval = d
	return func() { attemptBeatInterval = was }
}

// SetAttemptWriteFault makes fault decide, before each lease write of the
// runner, whether that write fails; it returns the restore function.
func SetAttemptWriteFault(fault func(verb string) error) func() {
	was := attemptWriteFault
	attemptWriteFault = fault
	return func() { attemptWriteFault = was }
}

// SetAttemptGroupSignalFault makes the owned group's SIGKILL fail without
// being sent, so retirement cannot be proved and cleanup HOLDs; it returns
// the restore function. Use it only with a command that exits by itself.
func SetAttemptGroupSignalFault() func() {
	was := attemptGroupPrimitives
	attemptGroupPrimitives.KillGroup = func(int) error { return errors.New("injected group signal failure") }
	return func() { attemptGroupPrimitives = was }
}

// SetDetachExecutable makes a detached launcher start program with prefix
// before the CLI arguments; it returns the restore function.
func SetDetachExecutable(program string, prefix ...string) func() {
	was := detachExecutable
	detachExecutable = func() (string, []string, error) { return program, prefix, nil }
	return func() { detachExecutable = was }
}

// SetRunPoll shortens an attach's wait between record reads.
func SetRunPoll(d time.Duration) func() {
	was := runPoll
	runPoll = d
	return func() { runPoll = was }
}

// SetRunSegmentBytes shrinks a detached run's output segment.
func SetRunSegmentBytes(n int64) func() {
	was := runSegmentBytes
	runSegmentBytes = n
	return func() { runSegmentBytes = was }
}

// RunDirForTest is the private directory of an attempt's detached runs.
func RunDirForTest(cwd, attemptID string) (string, error) {
	repo, err := intent.Resolve(cwd)
	if err != nil {
		return "", err
	}
	return runsDir(repo, attemptID), nil
}

// ObserveTickets runs the native dispatcher observation at cwd and returns
// its ticket views (ERG-V0-009 gate exposure).
func ObserveTickets(cwd string) ([]dispatch.Ticket, error) {
	o, err := dispatchQueue{env: Env{Cwd: cwd, Stdout: io.Discard, Stderr: io.Discard}}.Observe(context.Background())
	if err != nil {
		return nil, err
	}
	return o.Tickets, nil
}

// DispatchQueueForTest is the dispatcher's native store boundary at cwd.
func DispatchQueueForTest(cwd string) dispatch.Queue {
	return dispatchQueue{env: Env{Cwd: cwd, Stdout: io.Discard, Stderr: io.Discard}}
}

// DecodeRunRecord exposes the run-record decoder to the CAL-V0-131 format
// table.
func DecodeRunRecord(raw []byte) error {
	_, err := decodeRunRecord(raw)
	return err
}

// SetObligationQualifiedVersions replaces the TOL-V0-009 qualified Playwright
// version list for a test and returns the restore function.
func SetObligationQualifiedVersions(v []string) func() {
	was := obligationQualifiedVersions
	obligationQualifiedVersions = func() []string { return append([]string(nil), v...) }
	return func() { obligationQualifiedVersions = was }
}

// SubmitObligationWitness submits a caller-composed OBLIGATIONS_WITNESS
// payload as OWNER with the given writer report check (nil for none), so a
// test can present a payload that differs from the recomputation.
func SubmitObligationWitness(env Env, target, expected, requestID string, check *mutation.ObligationReportCheck, payload wire.Value) *wire.Result {
	cmd := []string{"ticket", "obligations", "witness"}
	actor, err := initActor("OWNER")
	if err == nil {
		payload, err = mutation.CanonicalPayload(ticket.OpObligationsWitness, payload)
	}
	if err != nil {
		return errorResult(cmd, err)
	}
	ctx := writerContext()
	if check != nil {
		ctx = store.WithObligationReport(ctx, check)
	}
	f := mutateFlags{role: "OWNER", requestID: requestID, target: target, expected: expected}
	return submitMutationContext(ctx, env, cmd, ticket.OpObligationsWitness, actor, f, payload)
}

// SetJobInterruptContext replaces the run-batch and qualify job context
// (TOL-V0-033), so a test can interrupt a job without a signal.
func SetJobInterruptContext(f func() (context.Context, context.CancelFunc)) func() {
	was := jobInterruptContext
	jobInterruptContext = f
	return func() { jobInterruptContext = was }
}

// SetWitnessEvidenceReadHook runs f after a report witness reads the
// commit's source evidence (TOL-V0-033).
func SetWitnessEvidenceReadHook(f func()) func() {
	was := witnessEvidenceReadHook
	witnessEvidenceReadHook = f
	return func() { witnessEvidenceReadHook = was }
}

// FailJobPoolRelease makes a job's `pool release` refuse with code, without
// releasing, while `pool acquire` runs for real (TOL-V0-030).
func FailJobPoolRelease(code string) func() {
	was := jobLeaseCommand
	jobLeaseCommand = func(env Env, name string, args []string) *wire.Result {
		if name == "pool release" {
			return &wire.Result{Command: strings.Fields(name), Outcome: wire.OutcomeRefused, Codes: []string{code}, Warnings: []string{"injected release refusal"}}
		}
		return was(env, name, args)
	}
	return func() { jobLeaseCommand = was }
}

// SetManifestHashHook runs f after the manifest hashes each regular file
// and returns the restore function.
func SetManifestHashHook(f func(path string)) func() {
	was := manifestHashHook
	manifestHashHook = f
	return func() { manifestHashHook = was }
}

// SetPriorSummaryReadHook runs f after a prior summary is opened, before its
// content is read, and returns the restore function.
func SetPriorSummaryReadHook(f func(file string)) func() {
	was := priorSummaryReadHook
	priorSummaryReadHook = f
	return func() { priorSummaryReadHook = was }
}

// SetDoctorClock replaces the doctor's clock for a test and returns the
// restore function.
func SetDoctorClock(now func() time.Time) func() {
	was := doctorClock
	doctorClock = now
	return func() { doctorClock = was }
}

// SetDoctorPluginTimeout shortens the per-plugin timeout for a test and
// returns the restore function.
func SetDoctorPluginTimeout(d time.Duration) func() {
	was := doctorPluginTimeout
	doctorPluginTimeout = d
	return func() { doctorPluginTimeout = was }
}

// WriteDoctorTestRun writes a RUNNING detached run record for the attempt,
// naming pid and identity as its supervisor and command, as a launched
// runner would.
func WriteDoctorTestRun(root, attemptID, generation string, pid int, identity string) error {
	repo, err := intent.Resolve(root)
	if err != nil {
		return err
	}
	runID := "0123456789abcdef"
	dir := filepath.Join(runsDir(repo, attemptID), runID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return writeRunRecord(dir, &runRecord{Profile: runRecordProfile, RunID: runID, AttemptID: attemptID, Generation: generation,
		ArgvSha256: strings.Repeat("a", 64), TimeoutSeconds: 3600, State: runRunning, LaunchedAt: time.Now().UTC().Format(time.RFC3339),
		SupervisorPid: pid, SupervisorIdentity: identity, CommandPid: &pid, CommandIdentity: &identity})
}

// DecodeDoctorCache decodes one doctor summary cache (TQD-V0-011).
func DecodeDoctorCache(raw []byte) error {
	_, err := decodeDoctorCache(raw)
	return err
}

// DoctorTestAttempt is one synthetic attempt afterimage for
// DoctorRepeatRefusalSeqs; Fresh marks the receipt that created it.
type DoctorTestAttempt struct {
	Ticket, Attempt string
	Gates           []string
	Fresh           bool
}

// DoctorTestEvent is one synthetic scanned receipt.
type DoctorTestEvent struct {
	Seq      uint64
	At       time.Time
	Attempts []DoctorTestAttempt
}

// DoctorRepeatRefusalSeqs runs the REPEAT_REFUSAL detector over synthetic
// receipts whose every gate digest names a FAILED result for tree, and
// returns each finding's evidence receipts and firstSeen.
func DoctorRepeatRefusalSeqs(events []DoctorTestEvent, tree string) ([][]uint64, []time.Time) {
	scan := &doctorScan{}
	for _, e := range events {
		ev := doctorEvent{seq: e.Seq, at: e.At}
		for _, a := range e.Attempts {
			ev.attempts = append(ev.attempts, &snapshot.Attempt{AttemptID: a.Attempt, TicketID: wire.TicketID{Raw: a.Ticket}, Generation: "1", Phase: "RUNNING", GateResults: a.Gates})
			ev.fresh = append(ev.fresh, a.Fresh)
		}
		scan.events = append(scan.events, ev)
	}
	failed := func(string) (*snapshot.GateResult, error) {
		return &snapshot.GateResult{State: "FAILED", CandidateTreeOid: tree}, nil
	}
	var seqs [][]uint64
	var first []time.Time
	for _, f := range doctorRepeatRefusal(failed, scan) {
		seqs, first = append(seqs, f.seqs), append(first, f.firstSeen)
	}
	return seqs, first
}

// SetDoctorCacheHookForTest runs hook at the named steps of a refresh
// ("open" before the cache directory is opened, "lock" before the
// destination is rechecked and replaced) and returns the restore function.
func SetDoctorCacheHookForTest(hook func(stage string)) func() {
	old := doctorCacheHook
	doctorCacheHook = hook
	return func() { doctorCacheHook = old }
}

// DoctorPluginNamesForTest lists the plugins doctor --plugins would run
// from dir (TQD-V0-010).
func DoctorPluginNamesForTest(dir string) ([]string, error) { return doctorPluginNames(dir) }

// DoctorPluginRunForTest runs c as doctor runs a plugin (TQD-V0-010,
// PGO-V0-009).
func DoctorPluginRunForTest(c *exec.Cmd) error { return doctorPluginRun(c) }

// ReadRunConfigForTest exercises classification without running a job.
var ReadRunConfigForTest = readRunConfig

// SetRunConfigOpenForTest injects a config open/read failure without descriptor pressure.
func SetRunConfigOpenForTest(open func(string) (io.ReadCloser, error)) func() {
	was := runConfigOpen
	runConfigOpen = open
	return func() { runConfigOpen = was }
}

// HolderStatus classifies a recorded heartbeat at now against ttlSeconds,
// exactly as attempt show and queue status do (CAL-V0-048, CAL-V0-120).
func HolderStatus(a *snapshot.Attempt, now time.Time, ttlSeconds int64) string {
	return holderStatus(a, now, ttlSeconds)
}

// SetImplementationDigest replaces the preflight cache's check
// implementation digest for a test and returns the restore function.
func SetImplementationDigest(f func() (string, error)) func() {
	was := implementationDigest
	implementationDigest = f
	return func() { implementationDigest = was }
}

// SetLocalNow replaces the local cache and ledger clock for a test and
// returns the restore function.
func SetLocalNow(f func() time.Time) func() {
	was := localNow
	localNow = f
	return func() { localNow = was }
}

// PruneCacheDir is pruneCacheDir for a test.
func PruneCacheDir(dir string, maxEntries int, maxAge time.Duration) {
	pruneCacheDir(dir, maxEntries, maxAge)
}

// PreflightCacheBounds are the cache's entry count and age bounds.
const (
	PreflightCacheMaxEntries = preflightCacheMaxEntries
	PreflightCacheMaxAge     = preflightCacheMaxAge
)

// LeaseDetachDir is the private record directory of a detached claim or
// pool acquire (ATR-V0-020).
func LeaseDetachDir(repo *intent.Repository, requestID string) string {
	return leaseDetachDir(repo, requestID)
}

// SetLeaseDetachCeiling shortens a detached child's ceiling (ATR-V0-023).
func SetLeaseDetachCeiling(d time.Duration) func() {
	was := leaseDetachCeiling
	leaseDetachCeiling = d
	return func() { leaseDetachCeiling = was }
}

// SetLeaseDetachPoll shortens a detach or attach wait's poll.
func SetLeaseDetachPoll(d time.Duration) func() {
	was := leaseDetachPoll
	leaseDetachPoll = d
	return func() { leaseDetachPoll = was }
}

// PruneLeaseDetach runs the finished-record retention at now (ATR-V0-025).
func PruneLeaseDetach(repo *intent.Repository, now time.Time) {
	pruneLeaseDetach(filepath.Join(repo.CommonDir, leaseDetachBase), now)
}

// SetLeaseDetachIdentity replaces a detached claim's process-identity probe
// (ATR-V0-027).
func SetLeaseDetachIdentity(probe func(int) (string, error)) func() {
	was := leaseDetachIdentity
	leaseDetachIdentity = probe
	return func() { leaseDetachIdentity = was }
}

// SetLeaseDetachChildStarting runs hook in a detached child before it takes
// its running lock, with the child's capped log (ATR-V0-027, ATR-V0-028).
func SetLeaseDetachChildStarting(hook func(io.Writer)) {
	leaseDetachChildStarting = hook
}

// MaxLeaseDetachLog is the cap on what a detached child writes to child.log.
const MaxLeaseDetachLog = maxLeaseDetachLog

// LeaseDetachProfile is the ATR-V0-020 request.json profile.
const LeaseDetachProfile = leaseDetachProfile

// ReadLeaseDetachRequest decodes the detached request record at p
// (ATR-V0-020, CAL-V0-131).
func ReadLeaseDetachRequest(p string) error {
	var rec leaseDetachRecord
	return readLeaseDetachJSON(p, &rec)
}
