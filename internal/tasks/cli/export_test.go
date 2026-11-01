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
	"github.com/Beamfall/corvint/internal/tasks/journal"
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

// SetAttemptOutcomeReportFault lets fault rewrite the outcome write's report
// before the runner reads it; it returns the restore function.
func SetAttemptOutcomeReportFault(fault func(*store.Report)) func() {
	was := attemptOutcomeReportFault
	attemptOutcomeReportFault = fault
	return func() { attemptOutcomeReportFault = was }
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

// DecodeDoctorConfig decodes one doctor --config file (TQD-V0-020).
func DecodeDoctorConfig(raw []byte) error {
	v, err := wire.ParseInput(raw)
	if err != nil {
		return err
	}
	_, err = decodeDoctorConfig(v, "")
	return err
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
	for _, f := range doctorRepeatRefusal(failed, scan, "") {
		seqs, first = append(seqs, f.seqs), append(first, f.firstSeen)
	}
	return seqs, first
}

// DoctorTestEnded is one synthetic ended generation of ticket T: its
// retry-accounting disposition, candidate tree ("" for none) and hand-off
// evidence reference ("" for none).
type DoctorTestEnded struct {
	Generation, Disposition, Tree, Evidence string
}

// DoctorRepeatRefusalDetails runs the REPEAT_REFUSAL detector over one
// ticket's synthetic ended generations, one receipt each, under the declared
// progress signal ("" for none), and returns each finding's detail
// (TQD-V0-041).
func DoctorRepeatRefusalDetails(gens []DoctorTestEnded, signal string) []string {
	scan := &doctorScan{}
	for i, g := range gens {
		a := &snapshot.Attempt{AttemptID: "a1", TicketID: wire.TicketID{Raw: "T"}, Generation: wire.Size(g.Generation), Phase: "FAILED",
			RetryAccounting: &snapshot.RetryAccounting{Disposition: g.Disposition}, HandoffEvidence: g.Evidence}
		if g.Tree != "" {
			tree := g.Tree
			a.CandidateTreeOid = &tree
		}
		scan.events = append(scan.events, doctorEvent{seq: uint64(10 + i), at: time.Unix(int64(1000+i), 0).UTC(), attempts: []*snapshot.Attempt{a}, fresh: []bool{true}})
	}
	none := func(string) (*snapshot.GateResult, error) { return nil, errors.New("no gate") }
	var out []string
	for _, f := range doctorRepeatRefusal(none, scan, signal) {
		out = append(out, f.detail)
	}
	return out
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

// The doctor's host probes (TQD-V0-021, TQD-V0-022, TQD-V0-024) default to
// an idle host in tests, so findings never depend on what else the test
// host runs; a test opts in to the real probes with UseRealDoctorProbes.
var realDoctorProbes = struct {
	table   func() ([]doctorCmdProc, error)
	holder  func(string) (bool, int, error)
	openers func([]string, []int) (map[string][]int, error)
	files   func() (uint64, uint64, error)
}{doctorCommandTable, doctorLockHolder, doctorLockOpeners, doctorFileTable}

func init() { idleDoctorProbes() }

func idleDoctorProbes() {
	doctorCommandTable = func() ([]doctorCmdProc, error) { return nil, nil }
	doctorLockHolder = func(string) (bool, int, error) { return false, 0, nil }
	doctorLockOpeners = func([]string, []int) (map[string][]int, error) { return map[string][]int{}, nil }
	doctorFileTable = func() (uint64, uint64, error) { return 0, 100, nil }
}

// UseRealDoctorProbes restores the real host probes until the returned
// function puts the idle ones back.
func UseRealDoctorProbes() func() {
	doctorCommandTable, doctorLockHolder = realDoctorProbes.table, realDoctorProbes.holder
	doctorLockOpeners, doctorFileTable = realDoctorProbes.openers, realDoctorProbes.files
	return idleDoctorProbes
}

// DoctorTestProc is one fake process-table row.
type DoctorTestProc struct {
	Pid     int
	Elapsed time.Duration
	Command string
}

// DoctorTestHost is a fake host: its process table, the holder pid of each
// lock file by base name (-1: held, holder not recorded), the pids that
// have each lock file open by base name, and the file table.
type DoctorTestHost struct {
	Procs           []DoctorTestProc
	Holders         map[string]int
	Openers         map[string][]int
	FilesUsed, FMax uint64
	ProbeErr        error
}

// SetDoctorTestHost installs a fake host until the returned function puts
// the idle one back.
func SetDoctorTestHost(h DoctorTestHost) func() {
	doctorCommandTable = func() ([]doctorCmdProc, error) {
		var out []doctorCmdProc
		for _, p := range h.Procs {
			out = append(out, doctorCmdProc{pid: p.Pid, elapsed: p.Elapsed, command: p.Command})
		}
		return out, h.ProbeErr
	}
	doctorLockHolder = func(path string) (bool, int, error) {
		pid, ok := h.Holders[filepath.Base(path)]
		if pid < 0 {
			pid = 0
		}
		return ok, pid, nil
	}
	doctorLockOpeners = func(paths []string, _ []int) (map[string][]int, error) {
		out := map[string][]int{}
		for _, p := range paths {
			out[p] = h.Openers[filepath.Base(p)]
		}
		return out, nil
	}
	doctorFileTable = func() (uint64, uint64, error) { return h.FilesUsed, h.FMax, nil }
	return idleDoctorProbes
}

// RealDoctorFileTable reads the host's file table with the real probe.
func RealDoctorFileTable() (uint64, uint64, error) { return realDoctorProbes.files() }

// RealDoctorLockHolder runs the real lock-holder probe on path.
func RealDoctorLockHolder(path string) (bool, int, error) { return realDoctorProbes.holder(path) }

// ParseDoctorProcLocksForTest, ParseDoctorLsofForTest, ParseDoctorFileNrForTest
// and ParseDoctorElapsedForTest expose the neutral probe parsers.
func ParseDoctorProcLocksForTest(text string, major, minor, ino uint64) []int {
	return parseDoctorProcLocks(text, major, minor, ino)
}

// DoctorFdForTest is one descriptor for DoctorOwnsFlockForTest.
type DoctorFdForTest struct {
	Dev, Ino uint64
	Fdinfo   string
}

// DoctorOwnsFlockForTest exposes the TQD-V0-821 ownership decision.
func DoctorOwnsFlockForTest(fds []DoctorFdForTest, dev, ino uint64) bool {
	in := make([]doctorFd, len(fds))
	for i, fd := range fds {
		in[i] = doctorFd{dev: fd.Dev, ino: fd.Ino, fdinfo: fd.Fdinfo}
	}
	return doctorOwnsFlock(in, dev, ino)
}

// ParseDoctorProcLocksByInodeForTest exposes the TQD-V0-821 same-inode parser.
func ParseDoctorProcLocksByInodeForTest(text string, ino uint64) []int {
	return parseDoctorProcLocksByInode(text, ino)
}
func ParseDoctorLsofForTest(text string) map[uint64][]int       { return parseDoctorLsof(text) }
func ParseDoctorFileNrForTest(s string) (uint64, uint64, error) { return parseDoctorFileNr(s) }
func ParseDoctorElapsedForTest(s string) (time.Duration, error) { return parseDoctorElapsed(s) }

// SetDoctorPerReadJournal makes every doctor journal read open its own
// session, the cost before TQD-V0-033, until the returned function runs.
func SetDoctorPerReadJournal() func() {
	old := doctorJournalSource
	doctorJournalSource = func(n journal.Native) (journal.Source, func() error) { return n, func() error { return nil } }
	return func() { doctorJournalSource = old }
}

// SetDoctorWalkUnshared makes every elided-record walk read and decode its
// receipts again, the cost before V1-1193, until the returned function runs.
func SetDoctorWalkUnshared() func() {
	old := doctorWalkShared
	doctorWalkShared = false
	return func() { doctorWalkShared = old }
}

// countingSource counts receipt reads by path.
type countingSource struct {
	journal.Source
	reads map[string]int
}

func (c countingSource) Read(path string, max int) ([]byte, error) {
	if strings.HasPrefix(path, "receipts/") {
		c.reads[path]++
	}
	return c.Source.Read(path, max)
}

// CountDoctorReceiptReads counts each receipt file the doctor's journal
// session reads into reads, until the returned function runs.
func CountDoctorReceiptReads(reads map[string]int) func() {
	old := doctorJournalSource
	doctorJournalSource = func(n journal.Native) (journal.Source, func() error) {
		src, done := old(n)
		return countingSource{Source: src, reads: reads}, done
	}
	return func() { doctorJournalSource = old }
}

// LeaseDetachDir is the private record directory of a detached claim or
// pool acquire (ATR-V0-020).
func LeaseDetachDir(repo *intent.Repository, requestID string) string {
	return leaseDetachDir(repo, requestID)
}

// LeaseDetachRunningHeld reports whether a child holds the record's running
// lock (ATR-V0-021).
func LeaseDetachRunningHeld(dir string) bool {
	return leaseDetachLockHeld(filepath.Join(dir, leaseDetachRunName))
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

// SetLeaseDetachProbeGap runs gap in every attach wait between its read of
// the launch lock and its read of the child's liveness; it returns the
// restore function (ATR-V0-311).
func SetLeaseDetachProbeGap(gap func()) func() {
	was := leaseDetachProbeGap
	leaseDetachProbeGap = gap
	return func() { leaseDetachProbeGap = was }
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

// SetDoctorJournalThresholds lowers the JOURNAL_GROWTH thresholds for a test
// and returns the restore function.
func SetDoctorJournalThresholds(receipts, bytes uint64) func() {
	r, b := doctorJournalReceipts, doctorJournalBytes
	doctorJournalReceipts, doctorJournalBytes = receipts, bytes
	return func() { doctorJournalReceipts, doctorJournalBytes = r, b }
}

// SetRunWriteBudget shortens the runner's per-write budget and outcome retry
// window (ATR-V0-051); it returns the restore function.
func SetRunWriteBudget(d time.Duration) func() {
	was := runWriteBudget
	runWriteBudget = d
	return func() { runWriteBudget = was }
}

// SetSplitAfterStep installs a hook that runs after each completed `ticket
// split` step and returns the restore function.
func SetSplitAfterStep(f func(step string) error) func() {
	was := splitAfterStep
	splitAfterStep = f
	return func() { splitAfterStep = was }
}
