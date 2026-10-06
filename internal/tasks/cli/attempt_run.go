package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Attempt-runner bounds (ATR-V0-002, ATR-V0-003).
const (
	runCleanupGrace   = 10 * time.Second
	runCoverageMargin = 120 * time.Second
	runWriteBound     = 60 * time.Second
	// maxRunTimeoutSeconds keeps timeout, cleanup and margin inside the
	// longest lease (1440 minutes).
	maxRunTimeoutSeconds = transaction.MaxLeaseMinutes*60 - 130
)

// Attempt-runner exit statuses (ATR-V0-004). A child exit status passes
// through unchanged; the envelope's class tells a reserved status apart.
const (
	runExitCleanupHold = 2
	runExitTimeout     = 124
	runExitLostLease   = 125
	runExitSpawn       = 126
	runExitUnrecorded  = 127
)

// attemptBeatInterval separates heartbeats while the command runs.
var attemptBeatInterval = 240 * time.Second

// Test-only fault injection; both are zero in production. attemptWriteFault,
// when set, can fail a lease write of the named verb before it is attempted;
// attemptGroupPrimitives replaces the owned group's host operations.
var (
	attemptWriteFault      func(verb string) error
	attemptGroupPrimitives groupreap.Primitives
)

// runnerSignals end a started run through the group stop and outcome path
// (ATR-V0-003). SIGKILL cannot be caught; see the spec's crash limit.
var runnerSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}

// attemptMode reports whether `run` names an attempt rather than a program.
func attemptMode(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--attempt" {
			return true
		}
	}
	return false
}

type attemptRunner struct {
	env        Env
	repo       *intent.Repository
	actor      mutation.Binding
	queueID    string
	attemptID  string
	generation wire.Size
	timeout    time.Duration
	minutes    uint64
	runID      string
	seq        int
	beats      int
	renewals   int
	// interrupt is the runner's own terminating signal, or 0.
	interrupt int
	// beatErrors counts heartbeat writes that failed without a refusal.
	beatErrors int
	lastBeat   error
	// role is the binding's role, passed on to a detached supervisor.
	role string
	// detach and supervise select the detached modes (ATR-V0-008); supervise
	// is the run ID the launcher chose.
	detach    bool
	supervise string
	// onStart, when set, observes the started command's PID (ATR-V0-009).
	onStart func(pid int)
	// output, when set, adds the detached run's output facts to the item.
	output *runOutput
}

type beatResult struct {
	report *store.Report
	err    error
}

// attemptRun is `run --attempt ID --generation G --timeout SECONDS
// [--lease-minutes N] [--role ROLE] -- COMMAND...` (ATR-V0-001). The
// command's stdout and stderr go to the caller's stderr; the one envelope
// goes to stdout.
func attemptRun(env Env, args []string) int {
	cmd := []string{"run"}
	if supervising(args) {
		// The launcher's readiness pipe must not reach any process this
		// supervisor starts, Git included (ATR-V0-009).
		protectReadiness()
	}
	if attachMode(args) {
		return attemptAttach(env, args)
	}
	r, argv, res := parseAttemptRun(env, args)
	if res != nil {
		return emit(env.Stdout, res)
	}
	if !groupreap.OwnerAvailable() {
		return emit(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeUnsupported}, Warnings: []string{"attempt run needs an owned process group, which this platform does not provide; nothing was written or started"}})
	}
	switch {
	case r.detach:
		return r.launch(argv)
	case r.supervise != "":
		return r.superviseRun(argv)
	}
	return r.run(argv)
}

// run is the attached runner: prepare, launch, supervise and record.
func (r *attemptRunner) run(argv []string) int {
	cmd := []string{"run"}
	interrupts := make(chan os.Signal, 1)
	// Notify stays in force until return, so a further signal during cleanup
	// or the outcome write is absorbed rather than killing the runner.
	signal.Notify(interrupts, runnerSignals...)
	defer signal.Stop(interrupts)
	if code, res := r.prepare(); res != nil {
		return emitCode(r.env.Stdout, res, code)
	}
	select {
	case sig := <-interrupts:
		return emitCode(r.env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Warnings: []string{"interrupted before launch; the command was not started and no outcome was recorded"}}, 128+signalNumber(sig))
	default:
	}
	outcome := r.execute(argv, interrupts)
	return r.finish(outcome)
}

func parseAttemptRun(env Env, args []string) (*attemptRunner, []string, *wire.Result) {
	cmd := []string{"run"}
	values := map[string]string{}
	detach := false
	i := 0
	for ; i < len(args) && args[i] != "--"; i += 2 {
		switch args[i] {
		case "--attempt", "--generation", "--timeout", "--lease-minutes", "--role", "--supervise":
		case "--detach":
			if detach {
				return nil, nil, usage(cmd, "repeated run flag --detach")
			}
			detach = true
			i--
			continue
		default:
			return nil, nil, usage(cmd, "attempt run accepts --attempt, --generation, --timeout, --lease-minutes, --role and --detach before --")
		}
		if i+1 >= len(args) || args[i+1] == "--" {
			return nil, nil, usage(cmd, "missing value for "+args[i])
		}
		if _, seen := values[args[i]]; seen {
			return nil, nil, usage(cmd, "repeated run flag "+args[i])
		}
		values[args[i]] = args[i+1]
	}
	if i >= len(args) || i+1 >= len(args) {
		return nil, nil, usage(cmd, "attempt run requires -- followed by the command")
	}
	argv := args[i+1:]
	if values["--attempt"] == "" || values["--generation"] == "" || values["--timeout"] == "" {
		return nil, nil, usage(cmd, "attempt run requires --attempt, --generation and --timeout")
	}
	supervise, superviseSet := values["--supervise"]
	if superviseSet && (detach || !validRunID(supervise)) {
		return nil, nil, usage(cmd, "--supervise is the detached launcher's own form and takes its run ID")
	}
	timeout, err := strconv.ParseUint(values["--timeout"], 10, 32)
	if err != nil || timeout < 1 || timeout > maxRunTimeoutSeconds {
		return nil, nil, usage(cmd, "--timeout must be whole seconds in 1.."+strconv.Itoa(maxRunTimeoutSeconds))
	}
	minutes := uint64(transaction.DefaultLeaseMinutes)
	if m, ok := values["--lease-minutes"]; ok {
		minutes, err = strconv.ParseUint(m, 10, 32)
		if err != nil || minutes < transaction.MinLeaseMinutes || minutes > transaction.MaxLeaseMinutes {
			return nil, nil, usage(cmd, "--lease-minutes must be in 5..1440")
		}
	}
	generation, err := wire.ParseSize("generation", values["--generation"])
	if err != nil {
		return nil, nil, errorResult(cmd, err)
	}
	role := values["--role"]
	if role == "" {
		role = "OPERATOR"
	}
	actor, err := initActor(role)
	if err != nil {
		return nil, nil, errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return nil, nil, errorResult(cmd, err)
	}
	observed, err := snapshot.Probe(repo.StateDir)
	if err != nil {
		return nil, nil, errorResult(cmd, err)
	}
	queueID := observed.Head.QueueID.Raw
	if aq, err := snapshot.AttemptQueue(values["--attempt"]); err != nil || aq.Raw != queueID {
		return nil, nil, errorResult(cmd, wire.Errorf(wire.CodeMalformed, "attempt", "--attempt must name an attempt of this queue"))
	}
	runID := supervise
	if runID == "" {
		var nonce [8]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return nil, nil, errorResult(cmd, err)
		}
		runID = hex.EncodeToString(nonce[:])
	}
	r := &attemptRunner{env: env, repo: repo, actor: actor, role: role, queueID: queueID, attemptID: values["--attempt"], generation: generation, timeout: time.Duration(timeout) * time.Second, minutes: minutes, runID: runID, detach: detach, supervise: supervise}
	return r, argv, nil
}

func nowStamp() wire.Timestamp {
	return wire.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z"))
}

// nextID is the run's next derived request id; it is called only from the
// runner's own goroutine.
func (r *attemptRunner) nextID() string {
	r.seq++
	return "run-" + r.runID + "-" + strconv.Itoa(r.seq)
}

func (r *attemptRunner) leaseWrite(ctx context.Context, id, verb string, minutes uint64) (*store.Report, error) {
	if attemptWriteFault != nil {
		if err := attemptWriteFault(verb); err != nil {
			return nil, err
		}
	}
	lease := transaction.LeaseRequest{Verb: verb, AttemptID: r.attemptID, Generation: r.generation}
	if verb == transaction.LeaseRenew {
		lease.LeaseMinutes = wire.SizeOf(minutes)
	}
	choice := store.LeaseChoice{QueueID: r.queueID, RequestID: id, Root: r.env.Cwd, Lease: lease, Derive: store.NoScopeDeriver}
	return store.Lease(ctx, r.repo, r.actor, choice, nowStamp())
}

func bounded() (context.Context, context.CancelFunc) {
	return context.WithTimeout(writerContext(), runWriteBound)
}

// refusal is the closed code of a lease write that did not complete, or ""
// when it completed. A refusal without a code (an unauthorized binding)
// carries MALFORMED, as wire.CodeOf does for an uncoded error; FENCED is
// reported only when the writer refused with it.
func refusal(report *store.Report) string {
	if report.Outcome.Outcome == mutation.OutcomeCompleted {
		return ""
	}
	if report.Outcome.HasCode(wire.CodeFenced) {
		return wire.CodeFenced
	}
	if len(report.Outcome.Codes) == 0 {
		return wire.CodeMalformed
	}
	return report.Outcome.Codes[0]
}

// beatFailed keeps a heartbeat write error as a warning (ATR-V0-006).
func (r *attemptRunner) beatFailed(err error) {
	r.beatErrors++
	r.lastBeat = err
}

// prepare heartbeats, then renews only when the current lease does not cover
// the timeout, cleanup and margin, and never shortens it (ATR-V0-002). A
// refusal here starts nothing and records no outcome.
func (r *attemptRunner) prepare() (int, *wire.Result) {
	cmd := []string{"run"}
	refuse := func(code int, codes []string, msg string) (int, *wire.Result) {
		return code, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: codes, Warnings: []string{prose(msg + "; the command was not started and no outcome was recorded")}}
	}
	lost := func(why string) (int, *wire.Result) {
		code := runExitLostLease
		if why != wire.CodeFenced {
			code = 1
		}
		return refuse(code, []string{why}, "the attempt's authority was refused before launch")
	}
	ctx, cancel := bounded()
	defer cancel()
	report, err := r.leaseWrite(ctx, r.nextID(), transaction.LeaseHeartbeat, 0)
	if err != nil {
		return 1, errorResult(cmd, err)
	}
	if why := refusal(report); why != "" {
		return lost(why)
	}
	r.beats++
	need := r.timeout + runCleanupGrace + runCoverageMargin
	a, err := store.AttemptRecord(ctx, r.repo, r.attemptID)
	if err != nil {
		return 1, errorResult(cmd, err)
	}
	if !covers(a, r.generation, time.Now().Add(need)) {
		minutes := uint64((need + time.Minute - 1) / time.Minute)
		if r.minutes > minutes {
			minutes = r.minutes
		}
		if minutes > transaction.MaxLeaseMinutes {
			return refuse(1, []string{wire.CodeLimitExceeded}, "the timeout and cleanup do not fit the longest lease")
		}
		report, err = r.leaseWrite(ctx, r.nextID(), transaction.LeaseRenew, minutes)
		if err != nil {
			return 1, errorResult(cmd, err)
		}
		if why := refusal(report); why != "" {
			return lost(why)
		}
		r.renewals++
		if a, err = store.AttemptRecord(ctx, r.repo, r.attemptID); err != nil {
			return 1, errorResult(cmd, err)
		}
	}
	if !covers(a, r.generation, time.Now().Add(r.timeout+runCleanupGrace)) {
		return refuse(1, []string{wire.CodeLimitExceeded}, "the lease does not cover the timeout and cleanup")
	}
	return 0, nil
}

func covers(a *snapshot.Attempt, generation wire.Size, until time.Time) bool {
	if a.Generation != generation || !a.Live() || a.Lease == nil {
		return false
	}
	expires, err := time.Parse("2006-01-02T15:04:05Z", string(a.Lease.ExpiresAt))
	return err == nil && !expires.Before(until)
}

// execute runs the command as the leader of an owned process group and
// retires the whole group before it returns (ATR-V0-003).
func (r *attemptRunner) execute(argv []string, interrupts <-chan os.Signal) transaction.RunOutcome {
	sum := sha256.Sum256([]byte(strings.Join(argv, "\x00")))
	out := transaction.RunOutcome{Profile: transaction.RunOutcomeProfile, RunID: r.runID, AttemptID: r.attemptID, Generation: string(r.generation), ArgvSha256: hex.EncodeToString(sum[:]), TimeoutSeconds: int(r.timeout / time.Second), Cleanup: transaction.CleanupNotStarted}
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = r.env.Cwd
	var output io.Writer = r.env.Stderr
	if _, ok := output.(*os.File); !ok {
		gate := &gatedWriter{w: output}
		defer gate.shut()
		output = gate
	}
	command.Stdin, command.Stdout, command.Stderr = r.env.Stdin, output, output
	command.WaitDelay = time.Second
	owner, err := groupreap.StartWith(command, attemptGroupPrimitives)
	if err != nil {
		out.Class, out.EndedAt = transaction.RunSpawnFailed, string(nowStamp())
		return out
	}
	started := string(nowStamp())
	out.StartedAt = &started
	if r.onStart != nil {
		r.onStart(command.Process.Pid)
	}
	deadline := time.NewTimer(r.timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(attemptBeatInterval)
	defer ticker.Stop()
	beats := make(chan beatResult, 1)
	beating := false
	trigger := ""
	for trigger == "" {
		select {
		case <-owner.Exited():
			trigger = transaction.RunExit
		case <-deadline.C:
			trigger = transaction.RunTimeout
		case sig := <-interrupts:
			trigger = transaction.RunInterrupted
			r.interrupt = signalNumber(sig)
		case <-ticker.C:
			if !beating {
				beating = true
				id := r.nextID()
				go func() {
					ctx, cancel := bounded()
					defer cancel()
					report, err := r.leaseWrite(ctx, id, transaction.LeaseHeartbeat, 0)
					beats <- beatResult{report, err}
				}()
			}
		case b := <-beats:
			beating = false
			// A write error is not a refusal: the lease already covers the
			// timeout and cleanup, so only a refused beat ends the run.
			if b.err != nil {
				r.beatFailed(b.err)
			} else {
				if why := refusal(b.report); why != "" {
					trigger = transaction.RunLostLease
					out.LostLease = &why
				} else {
					r.beats++
				}
			}
		}
	}
	if trigger != transaction.RunExit {
		owner.Stop()
	}
	limit, cancel := context.WithTimeout(context.Background(), runCleanupGrace)
	result := owner.FinishBounded(groupreap.RetirementBound{Done: limit.Done()})
	cancel()
	if beating {
		if b := <-beats; b.err != nil {
			r.beatFailed(b.err)
		} else if refusal(b.report) == "" {
			r.beats++
		}
	}
	out.EndedAt = string(nowStamp())
	out.Class = trigger
	if result.State != groupreap.Released {
		out.Cleanup, out.Class = transaction.CleanupHold, transaction.RunCleanupHold
		return out
	}
	out.Cleanup = transaction.CleanupReleased
	// The child's own status, never the runner's signal: an interrupted
	// run's child usually dies from the group SIGKILL (ATR-V0-004).
	var exit *exec.ExitError
	if errors.As(result.WaitErr, &exit) {
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			n := int(ws.Signal())
			out.Signal = &n
		} else {
			code := exit.ExitCode()
			out.ExitCode = &code
		}
	} else if result.WaitErr == nil {
		zero := 0
		out.ExitCode = &zero
	}
	if trigger == transaction.RunExit && out.Signal != nil {
		out.Class = transaction.RunSignal
	}
	return out
}

// gatedWriter carries child output to a writer that is not an *os.File. Such
// a writer is fed by an os/exec copy goroutine that only the reap joins, and a
// HOLD leaves the leader unreaped, so the gate is shut before execute returns
// and any later output is refused instead of reaching the caller's writer.
type gatedWriter struct {
	mu     sync.Mutex
	w      io.Writer
	closed bool
}

func (g *gatedWriter) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return 0, io.ErrClosedPipe
	}
	return g.w.Write(p)
}

func (g *gatedWriter) shut() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

func signalNumber(sig os.Signal) int {
	if s, ok := sig.(syscall.Signal); ok {
		return int(s)
	}
	return int(syscall.SIGTERM)
}

// finish heartbeats once more after a released run that kept its authority,
// records the outcome, and reports it (ATR-V0-004, ATR-V0-005).
func (r *attemptRunner) finish(out transaction.RunOutcome) int {
	if out.Cleanup == transaction.CleanupReleased && out.Class != transaction.RunLostLease {
		ctx, cancel := bounded()
		report, err := r.leaseWrite(ctx, r.nextID(), transaction.LeaseHeartbeat, 0)
		cancel()
		if err != nil {
			r.beatFailed(err)
		} else {
			if why := refusal(report); why != "" {
				out.Class, out.LostLease = transaction.RunLostLease, &why
			} else {
				r.beats++
			}
		}
	}
	out.Heartbeats, out.Renewals = r.beats, r.renewals
	// The child already ran: reissuing `attempt run` would run it again, so a
	// coded result here is never retryable whatever its codes (CAL-V0-078).
	res := &wire.Result{Command: []string{"run"}, Outcome: wire.OutcomeOK, Codes: []string{}, Warnings: []string{}, NotRetryable: true}
	item := wire.NewObject()
	receipt, digest := wire.Null(), wire.Null()
	raw, err := transaction.EncodeRunOutcome(out)
	if err == nil && attemptWriteFault != nil {
		err = attemptWriteFault(transaction.LeaseRunOutcome)
	}
	if err == nil {
		digest = wire.String(string(wire.Sum(raw)))
		var report *store.Report
		ctx, cancel := bounded()
		report, err = store.RecordRunOutcome(ctx, r.repo, r.actor, r.queueID, r.nextID(), r.attemptID, r.generation, raw, nowStamp())
		cancel()
		if err == nil && report.Outcome.Outcome != mutation.OutcomeCompleted {
			err = wire.Errorf(refusal(report), "run outcome", "the outcome receipt was refused: %s", report.Detail)
		}
		if err == nil {
			receipt = wire.String(report.Receipt)
		}
	}
	status := runStatus(out, r.interrupt)
	if r.beatErrors > 0 {
		res.Warnings = append(res.Warnings, prose(strconv.Itoa(r.beatErrors)+" heartbeat write(s) failed and were not retried; the lease covered the run before launch; last error: "+r.lastBeat.Error()))
	}
	switch out.Class {
	case transaction.RunTimeout:
		res.Outcome, res.Codes = wire.OutcomeRefused, []string{wire.CodeLimitExceeded}
	case transaction.RunLostLease:
		res.Outcome, res.Codes = wire.OutcomeRefused, []string{*out.LostLease}
	case transaction.RunInterrupted:
		res.Outcome = wire.OutcomeRefused
	case transaction.RunSpawnFailed:
		res.Outcome, res.Codes = wire.OutcomeError, []string{wire.CodeNoexec}
	case transaction.RunCleanupHold:
		res.Outcome, res.Codes = wire.OutcomeError, []string{wire.CodeSurvivors}
	}
	if err != nil {
		res.Outcome = wire.OutcomeError
		res.Codes = append(res.Codes, wire.CodeOf(err))
		res.Warnings = append(res.Warnings, prose("the outcome was not recorded: "+err.Error()))
		if status != runExitCleanupHold {
			status = runExitUnrecorded
		}
	}
	item.Set("runId", wire.String(r.runID)).Set("attemptId", wire.String(r.attemptID)).Set("generation", wire.String(string(r.generation)))
	item.Set("class", wire.String(out.Class)).Set("cleanup", wire.String(out.Cleanup)).Set("exitStatus", wire.String(strconv.Itoa(status)))
	item.Set("childExit", intOrNull(out.ExitCode)).Set("childSignal", intOrNull(out.Signal)).Set("lostLease", stringPtrOrNull(out.LostLease))
	item.Set("heartbeats", wire.String(strconv.Itoa(out.Heartbeats))).Set("renewals", wire.String(strconv.Itoa(out.Renewals)))
	item.Set("outcomeSha256", digest).Set("outcomeReceipt", receipt)
	if r.output != nil {
		r.output.describe(item)
	}
	res.Items = []wire.Value{wire.ObjectValue(item)}
	return emitCode(r.env.Stdout, res, status)
}

// runStatus maps a recorded class to the runner's exit status; an
// interrupted run reports the runner's own signal.
func runStatus(out transaction.RunOutcome, interrupt int) int {
	switch out.Class {
	case transaction.RunInterrupted:
		return 128 + interrupt
	case transaction.RunCleanupHold:
		return runExitCleanupHold
	case transaction.RunLostLease:
		return runExitLostLease
	case transaction.RunTimeout:
		return runExitTimeout
	case transaction.RunSpawnFailed:
		return runExitSpawn
	}
	if out.Signal != nil {
		return 128 + *out.Signal
	}
	if out.ExitCode != nil {
		return *out.ExitCode
	}
	return 1
}

func intOrNull(n *int) wire.Value {
	if n == nil {
		return wire.Null()
	}
	return wire.String(strconv.Itoa(*n))
}

func stringPtrOrNull(s *string) wire.Value {
	if s == nil {
		return wire.Null()
	}
	return wire.String(*s)
}

// emitCode writes the envelope and returns code instead of emit's 0/1, so the
// runner's exit status stays the command's.
func emitCode(w interface{ Write([]byte) (int, error) }, res *wire.Result, code int) int {
	if emit(w, res) != 0 && code == 0 {
		return 1
	}
	return code
}
