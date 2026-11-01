package dispatch

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// Dispatcher is one program's continuous dispatcher. Only one runs per state
// directory; the lock is held for its lifetime.
type Dispatcher struct {
	Program string
	Config  *Config
	Queue   Queue
	Out     io.Writer
	Now     func() time.Time
	// Mono, when set, replaces the monotonic reading time.Now carries in
	// host-suspend detection (ATR-V0-063); it does not advance while the
	// host sleeps.
	Mono func() time.Duration
	// HookCommand is the argv a worker's context hook runs (ATR-V0-102);
	// empty runs corvint-tasks from PATH.
	HookCommand []string
	// ResolveCommit resolves a Git revision in a directory to its full
	// commit id; a salvaged note names its attempt's commit through it
	// (ERG-V0-028). Nil leaves every note without a commit.
	ResolveCommit func(dir, rev string) (string, error)
	// ListCommits lists, newest first and at most max, the commits reachable
	// from HEAD in dir but not from base; a finished event names a worker's
	// commits since launch through it (ATR-V0-351). Nil leaves them
	// NOT_OBSERVED.
	ListCommits func(dir, base string, max int) ([]string, error)

	dir       string
	nonce     string
	ledger    *Ledger
	exits     map[string]<-chan exitStatus
	codes     map[string]exitStatus
	lock      *os.File
	reader    *readerSlot
	readerErr error
	// Tests inject faults through the real creation-owned API.
	readerStart func(*exec.Cmd) (*groupreap.Owner, error)
	// Tests replace the default start's host operations; the repeated group
	// SIGKILL opt-in (PGO-V0-017) still applies to them.
	readerHost groupreap.Primitives
	// Tests that do not exercise the reader retirement allowance lift it;
	// zero is the product's fixed one second.
	readerAllowance time.Duration
	// Tests place cancellation exactly across the checked atomic write.
	progressSave  func(*Ledger, string) error
	poolSweepSave func(*Ledger, string) error
	sweepJob      *poolSweepJob
	sweepTried    map[string]bool
	sweepNext     time.Time
	closed        bool
	closeErr      error
	// pressureSampler reads host pressure once per tick (CAL-V0-068) for the
	// selected signals only (CAL-V0-125); tests inject samples. Nil uses the
	// platform sampler. goos names the host OS for signal selection; empty
	// is runtime.GOOS.
	pressureSampler func(context.Context, time.Time, pressureWant) PressureSample
	goos            string
	// configRead re-reads the configuration file at each tick (CAL-V0-127)
	// with the stat of the descriptor it read; nil disables reload.
	// configSha256 and configAt identify the applied bytes and when they
	// were applied.
	configRead   func() ([]byte, fs.FileInfo, error)
	configSha256 string
	configAt     time.Time
	// configStat describes the configuration file without reading it;
	// configSeen is the stat of the descriptor the last successful read
	// used and configSeenSha256 those bytes' digest (CAL-V0-139).
	configStat       func() (fs.FileInfo, error)
	configSeen       fs.FileInfo
	configSeenSha256 string
	// configSeenAt is when those bytes were read; a full tick reuses their
	// digest for an unchanged stat only within idleFullEvery of it (V1-0925).
	configSeenAt time.Time
	// memberSince holds the lane member state episodes observed by this
	// run (CAL-V0-129).
	memberSince map[string]memberEpisode
	// contexts holds each running worker's ATR-V0-033 context record.
	contexts map[string]*contextState
	// launchedUnder keeps, for each worker running when a reload applied,
	// the configuration it launched under: supervision deadlines and kill
	// grace follow it, so a reload affects only later launches (CAL-V0-127).
	// A worker absent from it launched under the current configuration.
	launchedUnder map[string]*Config
	// pressureEmitted is the level and sample knowledge last reported by a
	// throttled event in this run; the zero value is calm and observed.
	pressureEmitted PressureState
	// pressureGap is set by a tick that deferred its pressure sample and
	// launched nothing (ATR-V0-181): the next sample has no CPU baseline,
	// as after Open, so utilization is never averaged across the gap.
	pressureGap bool
	// pressureHold keeps the budget at a raised pending level from the
	// first sample after a gap until that transition resolves, because the
	// skipped ticks never advanced the dwell (ATR-V0-181).
	pressureHold bool
	// pressureBaseline is set when the first sample after a gap had no CPU
	// interval with cpu selected: that tick admits no non-exempt launch and
	// the next tick samples rather than deferring (ATR-V0-181).
	pressureBaseline bool
	// poolWaitEmitted is the PSR-V0-143 pool-wait signature last reported
	// for each held ticket key, in memory only.
	poolWaitEmitted map[string]string
	// admission holds the ATR-V0-150 queue streaks in memory only, and
	// admissionFailed reports that the last observation write failed.
	admission       admissionTracker
	admissionFailed bool
	// unmatched is this tick's DSG-V0-001 NO_ROLE_MATCH tickets; idle is
	// the DSG-V0-004 idleRun streak and idleAlerted the last
	// time it alerted; idleWorkers is the previous tick's worker IDs.
	unmatched   map[string]bool
	idleRun     streak
	idleAlerted time.Time
	idleWorkers map[string]bool
	// control, set only by OpenControlled, admits each new launch and
	// pool sweep and sees every tick boundary.
	control LaunchControl
	// tickSaved reports that the last tick's final ledger save succeeded.
	tickSaved bool
	// uncertain holds launches that started but could not be identified:
	// their trees may outlive the group kill unrecorded, so a controlled
	// dispatcher never reports its boundaries recorded again.
	uncertain []*startedError
	// recoveries holds, by attempt ID, the CAL-V0-104 exit recoveries of
	// attempts whose ended worker's hand-off was refused. They are derived
	// run state, not ledger state: after a restart heal.reap still reaps
	// the expired lease of a holder that is not a running worker.
	recoveries map[string]*exitRecovery
	// poolHoldRefused is the last refusal reported for each ticket's
	// ATR-V0-120 pre-launch pool claim, so a waiting ticket is reported once.
	poolHoldRefused map[string]string
	// unsignalled names the workers whose CAL-V0-191 LEASE_EXPIRED stop this
	// process decided and has not signalled yet; only those may be kept. It
	// is never persisted, so a stop found in the ledger always completes.
	unsignalled map[string]bool
	// childCPU keeps, by worker ID, the CAL-V0-218 child CPU baseline taken
	// at the worker's last activity or first observation by this process.
	// It is never persisted: after a restart the first tick takes a new
	// baseline and counts every non-ignored child busy.
	childCPU map[string]childCPU
	// treeSeen names the workers whose process tree the last supervise
	// pass observed; the lease keeper acts only for them (CAL-V0-317).
	treeSeen map[string]bool
	// carryStale names the tickets whose ATR-V0-111 carry-over record could
	// not be removed after progress; carryForeign the newer-version records
	// already alerted. Neither is persisted.
	carryStale, carryForeign map[string]bool
	// carryUnsafe names the tickets already alerted for an unsafe rendered
	// argv under the applied configuration carryUnsafeConfig (ATR-V0-165);
	// a configuration change clears it. It is not persisted.
	carryUnsafe       map[string]bool
	carryUnsafeConfig string
	// idle is armed by a full tick that changed nothing, and idleLease is
	// the earliest future lease expiry the current tick observed
	// (CAL-V0-139).
	idle      *idleGate
	idleLease time.Time
	// retireConfirm is when the next confirming CAL-V0-144 retirement pass
	// is due while worker directories hold retirement marks; zero when none
	// is pending, so ticks without marks do no retirement work.
	retireConfirm time.Time
	// retireScan is the open workers directory stream a ATR-V0-182 pass
	// reads its next window from, nil between sweeps; retireWindow is the
	// window held while its marks settle, retireHeld the passes it has had,
	// and retireCarry the retained directories carried to the next window.
	retireScan   *os.File
	retireWindow []string
	retireHeld   int
	retireCarry  []string
	// detached holds this dispatcher's CAL-V0-147 detached run markers by
	// run ID, loaded at Open; deferredNow names, for the current tick, the
	// run each ended worker's hand-off waits for; heldBy keeps the live
	// attempts each worker held at the last observation (CAL-V0-145);
	// detachedHeld names the runs whose markers another build wrote.
	detached     map[string]*detachedMarker
	detachedHeld map[string]bool
	deferredNow  map[string]string
	// deferredContext keeps, by attempt ID, the ATR-V0-035 detail of a
	// context-stopped worker whose hand-off waits for its detached run.
	deferredContext map[string]map[string]string
	heldBy          map[string][]Attempt
	// everHeld and handedOff name, by worker, one this process observed
	// holding a live attempt and one whose attempt it handed off; a finished
	// event's attemptRelease reads them (ATR-V0-351).
	everHeld  map[string]bool
	handedOff map[string]bool
	// drainAlerted keeps one alert per drain-signal write or removal
	// failure until a write succeeds (ATR-V0-058).
	drainAlerted bool
	// churn is the ATR-V0-061 short-session record, read once; churnForeign
	// marks a newer-version record that is kept and not tracked.
	churn        *SessionChurn
	churnForeign bool
	// exhaustion is the ATR-V0-106 hard-stop record, read once;
	// exhaustionForeign marks a newer-version record that is kept.
	exhaustion        *ContextExhaustion
	exhaustionForeign bool
	// sleep is the ATR-V0-063 host-sleep record; lastNow and lastMono are
	// the previous tick's clock readings.
	sleep        *HostSleep
	sleepForeign bool
	lastNow      time.Time
	lastMono     time.Duration
	// tickSeq numbers this run's full ticks and workStateTick is the tick
	// whose failed work-state read was last counted, so a tick that reads
	// more than once counts one failed tick (CAL-V0-270).
	tickSeq, workStateTick uint64
	// holdAnnounced records that the current CAL-V0-273 launch hold
	// emitted its throttled event, so a reload that lowers the threshold
	// mid-outage still announces the hold once.
	holdAnnounced bool
	// upgradeArgs is the argv a handed-over build starts with; nil refuses
	// handover (DSG-V0-011). execImage replaces the process image; tests
	// inject it. upgradedFrom is the build this image took over from, and
	// mixedBuilds is set while a worker of another build runs (DSG-V0-014).
	upgradeArgs  []string
	execImage    func(path string, argv, env []string) error
	upgradedFrom *Build
	mixedBuilds  bool
}

// lockProgram creates the program directory and takes its single-dispatcher
// lock without waiting.
func lockProgram(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockExclusive(lock); err != nil {
		lock.Close()
		return nil, fmt.Errorf("another dispatcher holds %s: %w", dir, err)
	}
	return lock, nil
}

// Open locks the program's state directory, loads the ledger and adopts
// every worker recorded by a previous dispatcher.
func Open(program string, c *Config, q Queue, out io.Writer) (*Dispatcher, error) {
	if !platformSupported {
		_, _, _, err := launch(nil, nil, "", "")
		return nil, err
	}
	if !ValidName(program) {
		return nil, fmt.Errorf("program name must match [a-z][a-z0-9-]{0,23}")
	}
	dir := ProgramDir(c, program)
	// The shared start gate covers creating the program directory and
	// taking its lock, so a relocation either sees the lock or keeps this
	// start out (TRL-V0-003).
	// DSG-V0-013: a handed-over image already holds the lock.
	lock, inheritErr := inheritLock(dir)
	inherited := lock != nil
	if !inherited {
		gate, err := openStartGate(c)
		if err != nil {
			return nil, err
		}
		lock, err = lockProgram(dir)
		gate.Close()
		if err != nil {
			return nil, err
		}
	}
	// A host reboot is the one automatic containment proof (CAL-V0-227); it
	// is recorded before the marker goes. Any other prior unresolved reader
	// refuses before owner/ledger/event writes or adoption.
	recovered, err := resolveReaderByReboot(dir, program, time.Now(), nil)
	if err != nil {
		lock.Close()
		return nil, fmt.Errorf("%w: the reboot proof was not recorded: %v", ErrReaderQuiescence, err)
	}
	if err := checkReaderQuarantine(dir, program); err != nil {
		lock.Close()
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "requests"), 0o700); err != nil {
		lock.Close()
		return nil, err
	}
	if err := writeOwner(lock); err != nil {
		lock.Close()
		return nil, err
	}
	l, err := LoadLedger(dir, program)
	if err != nil {
		lock.Close()
		return nil, err
	}
	// The start nonce keeps worker IDs, and so holders, handoff evidence
	// and release request IDs, unique across restarts, a crash before the
	// ledger is saved, and a deleted state directory.
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		lock.Close()
		return nil, err
	}
	d := &Dispatcher{Program: program, Config: c, Queue: q, Out: out, Now: time.Now, dir: dir, nonce: hex.EncodeToString(nonce[:]), ledger: l, exits: map[string]<-chan exitStatus{}, codes: map[string]exitStatus{}, lock: lock}
	d.reconcileEscalation()
	// CAL-V0-127: a restart applies its configuration afresh.
	l.Config = nil
	// CAL-V0-068: pressure state exists only while configured. A restart
	// keeps the recorded level, so it cannot bypass a throttle, but cancels
	// pending dwell and is UNKNOWN until this run's first sample.
	switch {
	case c.Pressure == nil:
		l.Pressure = nil
	case l.Pressure == nil:
		l.Pressure = &PressureRecord{State: PressureState{Unknown: true}, Held: []HeldLaunch{}}
	default:
		st := &l.Pressure.State
		st.PendingLevel, st.PendingTicks, st.Unknown = st.Level, 0, true
		l.Pressure.Sample = PressureSample{}
	}
	d.emit(Event{Kind: "started", Message: fmt.Sprintf("dispatcher started for program %s with %d recorded worker(s)", program, len(l.Workers))})
	d.warnRoleShadows()
	if recovered != nil {
		d.emit(Event{Kind: "reader-recovered", Message: fmt.Sprintf("reader run %s quarantine lifted: %s (%s); recorded in %s", recovered.Run, recovered.Reason, recovered.Evidence, readerRecoveryDir), Detail: map[string]string{"kind": recovered.Kind, "run": recovered.Run}})
	}
	for _, w := range l.Workers {
		msg := fmt.Sprintf("adopted %s worker %s (pid %d) on %s; its exit code will not be observed", w.Role, w.ID, w.PID, d.keyText(w.Key))
		if inherited {
			msg = fmt.Sprintf("adopted %s worker %s (pid %d) on %s from the handing-over build; it finishes on its own build", w.Role, w.ID, w.PID, d.keyText(w.Key))
		}
		if id, err := supervisor.ProcessIdentity(w.PID); err == nil && !supervisor.SameProcess(id, w.LeaderIdentity) {
			msg = fmt.Sprintf("adopted %s worker %s on %s; its leader (pid %d) has ended, so any remaining processes are stopped and the run is accounted", w.Role, w.ID, d.keyText(w.Key), w.PID)
		}
		d.emit(Event{Kind: "adopted", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: msg})
	}
	d.resumeUpgrade(inherited, inheritErr)
	d.reconcileInfraRetry()
	d.loadDetached()
	d.holdAnnounced = WorkStateHoldsLaunches(d.Config, d.ledger.WorkState)
	if err := d.ledger.save(dir); err != nil {
		_ = d.lock.Close()
		return nil, err
	}
	// CAL-V0-144: marks left by an earlier run are confirmed, or started
	// over, by one pass on the first tick.
	if f, err := os.Open(filepath.Join(dir, "workers", workerRetireMarks)); err == nil {
		if names, _ := f.Readdirnames(1); len(names) > 0 {
			d.retireConfirm = time.Unix(0, 1)
		}
		f.Close()
	}
	return d, nil
}

// Close records the stop and releases the lock. Workers keep running and
// are adopted by the next dispatcher.
func (d *Dispatcher) Close() error {
	if d.closed {
		return d.closeErr
	}
	d.closeRetireScan()
	if d.readerErr != nil {
		// Join first so no sweep goroutine outlives Close; HOLD still writes
		// no ledger or event, so retirePoolSweep publishes nothing here.
		_ = d.retirePoolSweep()
		// Terminal UNKNOWN permits releasing the lock, not recovery. The
		// pre-spawn marker survives even when this diagnostic cannot be written.
		if d.Out != nil {
			_, _ = fmt.Fprintln(d.Out, d.readerErr)
		}
		_ = d.lock.Close()
		return d.readerErr
	}
	joinedErr := d.stopPoolSweep()
	d.emit(Event{Kind: "stopped", Message: fmt.Sprintf("dispatcher stopped; %d worker(s) left running for the next dispatcher", len(d.ledger.Workers))})
	err := errors.Join(joinedErr, d.ledger.save(d.dir))
	d.closed = true
	d.closeErr = err
	_ = d.lock.Truncate(0)
	d.lock.Close()
	return err
}

// writeOwner records the lock holder's PID and start identity so a pure
// `dispatch status` read can tell whether a dispatcher runs without taking
// the lock.
func writeOwner(f *os.File) error {
	id, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err = f.WriteAt([]byte(strconv.Itoa(os.Getpid())+" "+id+"\n"), 0)
	return err
}

// OwnerState is RUNNING when the recorded lock holder still runs with its
// recorded identity, NOT_RUNNING when no holder is recorded or it is gone,
// and UNKNOWN when the record cannot be read.
func OwnerState(dir string) string {
	st, _ := OwnerProcess(dir)
	return st
}

// Running is the number of supervised workers.
func (d *Dispatcher) Running() int { return len(d.ledger.Workers) }

// LastEvent is the last event sequence number.
func (d *Dispatcher) LastEvent() uint64 { return d.ledger.EventSeq }

// Run ticks until ctx ends or ticks reach the bound (0 is unbounded). A
// failed tick is an alert, not an exit, so the dispatcher keeps supervising.
func (d *Dispatcher) Run(ctx context.Context, ticks int) (result error) {
	defer func() {
		if e := d.retirePoolSweep(); e != nil {
			result = errors.Join(result, e)
		}
	}()
	if d.readerErr != nil {
		return d.readerErr
	}
	var last error
	for n := 0; ticks == 0 || n < ticks; n++ {
		if ctx.Err() != nil {
			return nil
		}
		if d.upgradePending() {
			return ErrUpgradeRequested // DSG-V0-012
		}
		last = d.Tick(ctx)
		d.settleMixedBuilds()
		if d.readerErr != nil {
			return d.readerErr
		}
		if d.boundary(ctx, last) {
			return ErrSettled
		}
		if last != nil && ctx.Err() == nil {
			d.emit(Event{Kind: "alert", Message: "tick failed: " + last.Error()})
		}
		if ctx.Err() != nil {
			return nil
		}
		if ticks != 0 && n+1 == ticks {
			break
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Duration(d.Config.TickSeconds) * time.Second):
		}
	}
	return last
}

// Tick is one CAL-V0-053..058 pass: observe, supervise, heal, re-observe,
// account for finished workers, launch the roster, and emit changes. Idle,
// wall and orphan enforcement runs even when the store is unreadable; ended
// workers then stay recorded and are accounted on the next readable tick.
// A tick after a full tick that changed nothing is skipped while the
// store's witness is unchanged and no deadline is due (CAL-V0-139).
func (d *Dispatcher) Tick(ctx context.Context) error {
	if d.idleSkip(ctx) {
		return nil
	}
	m := d.idleBegin()
	err := d.tick(ctx)
	d.idleSettle(m, err)
	return err
}

// ErrLedgerUnsaved marks a tick whose final ledger save failed (CAL-V0-194).
// The tick's decisions hold in memory and its events are already in the
// event log, but state.json is not confirmed to hold them: it may still hold
// the previous save, or (when only the directory sync failed) the new bytes
// without proven durability, until a later save of the current ledger succeeds.
var ErrLedgerUnsaved = errors.New("dispatcher ledger not saved")

func (d *Dispatcher) tick(ctx context.Context) (err error) {
	d.tickSaved = false
	d.tickSeq++
	if d.readerErr != nil {
		return d.readerErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Admission publishes a new ledger only after its checked write. Resolve
	// this receiver at return so an old pointer cannot overwrite that commit.
	// CAL-V0-194: a failed save fails the tick with its cause; the in-memory
	// ledger, workers included, stays authoritative and the next save
	// writes it whole.
	defer func() {
		if d.readerErr != nil {
			return
		}
		if serr := d.ledger.save(d.dir); serr != nil {
			err = errors.Join(err, fmt.Errorf("%w: %s is not confirmed durable and may still hold the previous save; this tick's state is held in memory until a later save succeeds: %w", ErrLedgerUnsaved, filepath.Join(d.dir, "state.json"), serr))
			return
		}
		d.tickSaved = true
	}()
	// CAL-V0-127: a changed configuration applies before this tick acts.
	d.reloadConfig()
	d.noteHostSleep()
	d.syncDrainSignal()
	d.syncChurn()
	obs, err := d.observe(ctx)
	if d.readerErr != nil {
		return d.readerErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d.noteHeld(obs)
	d.keepReclaimedWorkers(obs)
	ended := d.supervise()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	if len(ended) > 0 {
		// Stopping a tree can take the whole kill grace; heal decides on a
		// fresh observation.
		if obs, err = d.observe(ctx); err != nil {
			return err
		}
	}
	if d.heal(ctx, obs, ended) {
		if obs, err = d.observe(ctx); err != nil {
			return err
		}
	}
	// CAL-V0-312: keep the leases of live keepLease workers on the
	// observation heal acted on.
	d.keepLeases(ctx, obs)
	granted, pending, unparked, err := d.admitProgress(ctx, obs, ended)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, key := range unparked {
		d.account(key, true)
		d.emit(Event{Kind: "unparked", Ticket: key, Message: fmt.Sprintf("unparked %s because declared progress changed", d.keyText(key))})
	}
	d.finish(obs, ended, granted, pending)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d.unpark(obs)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d.diff(obs)
	obs, sweepErr := d.tickPoolSweep(ctx, obs)
	if ctx.Err() == nil {
		d.launchRoster(ctx, obs)
	}
	return sweepErr
}

func (d *Dispatcher) observe(ctx context.Context) (*Observation, error) {
	obs, err := d.Queue.Observe(ctx)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	d.noteLeases(obs)
	alerts := readStates(ctx, d.Config, obs.Tickets, d.stateCommand)
	if d.readerErr != nil {
		return nil, d.readerErr
	}
	// An interrupted read is not an observation. In particular, its
	// synthetic UNKNOWN states must not replace the last good baseline.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// CAL-V0-053 alerts, CAL-V0-270 and CAL-V0-274 reader health, and the
	// CAL-V0-272 running workers whose ticket this read left UNKNOWN.
	d.noteWorkState(alerts)
	d.markWorkStateUnknown(obs)
	// CAL-V0-105, CAL-V0-155: tickets the work state or a budget holds
	// leave the selection window, replanned from the same snapshot, so they
	// no longer starve the rest.
	held, unmatched := workStateHeld(d.Config, obs.Tickets)
	budgetHeld := d.budgetHeld(obs.Tickets, d.Now())
	if len(held)+len(budgetHeld) > 0 && obs.Replan != nil {
		plan := obs.Replan(held, budgetHeld)
		for i := range obs.Tickets {
			if v, ok := plan[obs.Tickets[i].ID]; ok {
				// DSG-V0-001: a state no role admits and no hold declares
				// leaves the window too, but reads NO_ROLE_MATCH.
				if v.Reason == PlanReasonWorkStateHeld && unmatched[obs.Tickets[i].ID] {
					v.Reason = PlanReasonNoRoleMatch
				}
				obs.Tickets[i].Plan, obs.Tickets[i].PlanReason = v.State, v.Reason
			}
		}
	}
	d.unmatched = unmatched
	return obs, nil
}

// admitProgress commits replay protection and consumption together. Other
// supervision facts are copied from the current ledger, never an old snapshot.
func (d *Dispatcher) admitProgress(ctx context.Context, obs *Observation, ended []*Worker) (map[string]bool, map[string]bool, []string, error) {
	granted, pending := map[string]bool{}, map[string]bool{}
	var unparked []string
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	enabled := len(d.ledger.Progress) != 0
	for _, t := range obs.Tickets {
		enabled = enabled || t.ProgressToken != ""
	}
	if !enabled {
		return granted, pending, nil, nil
	}
	raw, err := json.Marshal(d.ledger)
	if err != nil {
		return nil, nil, nil, err
	}
	var staged Ledger
	if err := json.Unmarshal(raw, &staged); err != nil {
		return nil, nil, nil, err
	}
	if staged.Progress == nil {
		staged.Progress = map[string]*ProgressHistory{}
	}
	count := 0
	for _, h := range staged.Progress {
		count += len(h.Seen)
	}
	indices := make([]int, len(obs.Tickets))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool { return obs.Tickets[indices[i]].ID < obs.Tickets[indices[j]].ID })
	changed, exhausted := false, false
	for _, i := range indices {
		t := obs.Tickets[i]
		if t.State == StateUnknown || t.ProgressToken == "" {
			continue
		}
		sum := sha256.Sum256([]byte(t.ProgressToken))
		digest := hex.EncodeToString(sum[:])
		h := staged.Progress[t.ID]
		if h != nil {
			at := sort.SearchStrings(h.Seen, digest)
			if at < len(h.Seen) && h.Seen[at] == digest {
				continue
			}
			if len(h.Seen) == maxProgressPerKey {
				exhausted = true
				continue
			}
		}
		if count == maxProgressProgram {
			exhausted = true
			continue
		}
		if h == nil {
			h = &ProgressHistory{Current: digest, Seen: []string{digest}}
			staged.Progress[t.ID] = h
			// Existing accounting receives the first token as its baseline,
			// while retaining its original durable fingerprint.
			for _, w := range staged.Workers {
				if w.Key == t.ID {
					w.BaseFingerprint, w.ProgressDigest = w.Fingerprint, digest
					w.Fingerprint = progressFingerprint(w.BaseFingerprint, digest)
				}
			}
			if b := staged.Backoff[t.ID]; b != nil {
				b.BaseFingerprint, b.ProgressDigest = b.Fingerprint, digest
				b.Fingerprint = progressFingerprint(b.BaseFingerprint, digest)
			}
		} else {
			h.Current = digest
			h.Seen = append(h.Seen, digest)
			sort.Strings(h.Seen)
		}
		count++
		changed = true
	}
	endedIDs := map[string]bool{}
	for _, w := range ended {
		endedIDs[w.ID] = true
	}
	for _, w := range staged.Workers {
		h := staged.Progress[w.Key]
		if !endedIDs[w.ID] || h == nil || h.Current == w.ProgressDigest {
			continue
		}
		if stateUnknown(obs, w.Key) {
			pending[w.ID] = true
			continue
		}
		granted[w.ID] = true
		delete(staged.Backoff, w.Key)
		changed = true
	}
	if len(granted) != 0 {
		workers := staged.Workers[:0]
		for _, w := range staged.Workers {
			if !granted[w.ID] {
				workers = append(workers, w)
			}
		}
		staged.Workers = workers
	}
	keys := make([]string, 0, len(staged.Backoff))
	for key := range staged.Backoff {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		b, h := staged.Backoff[key], staged.Progress[key]
		if b.Parked && h != nil && h.Current != b.ProgressDigest && !stateUnknown(obs, key) {
			delete(staged.Backoff, key)
			unparked = append(unparked, key)
			changed = true
		}
	}
	// ATR-V0-167: the progress accounting of a granted worker or an
	// unparked key is applied in this same save, before the worker leaves
	// the ledger, so an interrupt before finish cannot lose it. finish and
	// the unparked loop apply it again, which changes nothing.
	for _, w := range ended {
		if granted[w.ID] {
			d.accountOn(&staged, w.Key, true)
			infraEndedOn(&staged, w.Key, w.ID, true)
		}
	}
	for _, key := range unparked {
		d.accountOn(&staged, key, true)
	}
	if changed {
		if err := staged.validateProgress(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		var saveErr error
		if d.progressSave == nil {
			saveErr = staged.save(d.dir)
		} else {
			saveErr = d.progressSave(&staged, d.dir)
		}
		if saveErr != nil {
			return nil, nil, nil, fmt.Errorf("declared progress admission failed: %w", saveErr)
		}
		d.ledger = &staged
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	// Only committed digests reach the observation/fingerprint. A missing or
	// repeated old token keeps the accepted current digest.
	for i := range obs.Tickets {
		if h := d.ledger.Progress[obs.Tickets[i].ID]; h != nil {
			obs.Tickets[i].ProgressDigest = h.Current
		}
	}
	if exhausted {
		d.emit(Event{Kind: "needs-owner", Message: "declared progress lifetime capacity exhausted; new tokens are ignored without resetting history"})
	}
	return granted, pending, unparked, nil
}

// superviseProcs reads the process table for supervise; tests count it.
var superviseProcs = observeProcs

// supervise refreshes every worker's tree and activity, kills idle,
// over-wall and orphaned trees, and returns the workers whose tree is empty.
// With no worker it reads no process table.
func (d *Dispatcher) supervise() []*Worker {
	d.treeSeen = map[string]bool{}
	if len(d.ledger.Workers) == 0 {
		return nil
	}
	procs, err := superviseProcs()
	if err != nil {
		d.emit(Event{Kind: "alert", Message: "process table unavailable: " + err.Error()})
		return nil
	}
	now := d.Now()
	live := map[string]bool{}
	for _, w := range d.ledger.Workers {
		live[w.ID] = true
	}
	for id := range d.childCPU {
		if !live[id] {
			delete(d.childCPU, id)
		}
	}
	for id := range d.contexts {
		if !live[id] {
			delete(d.contexts, id)
		}
	}
	// Every tree and its activity are observed before any stop, so a stop
	// that blocks cannot age the CPU samples taken for a later worker
	// (CAL-V0-218).
	type observed struct {
		w      *Worker
		exempt map[int]string
		hard   bool // ATR-V0-035
	}
	var seen []observed
	for _, w := range d.ledger.Workers {
		select {
		case code := <-d.exits[w.ID]:
			d.codes[w.ID] = code
		default:
		}
		exempt := d.exemptFor(w)
		if err := refreshTree(w, procs, exempt); err != nil {
			d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not observe the process tree of %s; keeping it as recorded: %v", w.ID, err)})
			continue
		}
		if d.active(w, procs, d.launchConfig(w).Hosts[w.Host], now) {
			w.LastActive = now
		}
		seen = append(seen, observed{w, exempt, d.noteContext(w, now)})
		d.treeSeen[w.ID] = true
	}
	var ended []*Worker
	for _, o := range seen {
		w, exempt := o.w, o.exempt
		cfg := d.launchConfig(w)
		role := cfg.roleNamed(w.Role)
		reason := ""
		switch {
		case len(w.Members) == 0:
			ended = append(ended, w)
			continue
		case w.State == "KILLING":
			reason = w.KillReason
		case !leaderAlive(w):
			reason = "ORPHANED"
		case o.hard:
			reason = "CONTEXT"
		case role != nil && now.Sub(w.Started)-d.slept(w.Started, now) > time.Duration(role.WallSeconds)*time.Second:
			reason = "WALL"
		case role != nil && now.Sub(w.LastActive)-d.slept(w.LastActive, now) > time.Duration(role.IdleSeconds)*time.Second:
			reason = "IDLE"
		}
		if reason == "" {
			continue
		}
		if w.State != "KILLING" {
			w.State, w.KillReason = "KILLING", reason
			d.emit(Event{Kind: "killing", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("stopping %s worker %s on %s: %s", w.Role, w.ID, d.keyText(w.Key), killText[reason]), Detail: map[string]string{"reason": reason, "processes": strconv.Itoa(len(w.Members))}})
		}
		delete(d.unsignalled, w.ID) // killTree may signal from here on
		switch gone, err := killTree(w, time.Duration(cfg.KillGraceSeconds)*time.Second, exempt); {
		case err != nil:
			d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not observe the process tree of %s while stopping it; retrying next tick: %v", w.ID, err)})
		case gone:
			d.emit(Event{Kind: "killed", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("stopped the whole process tree of %s", w.ID), Detail: map[string]string{"reason": reason}})
			ended = append(ended, w)
		default:
			d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("%d process(es) of %s survived SIGKILL; still supervising", len(w.Members), w.ID)})
		}
	}
	return ended
}

var killText = map[string]string{
	"IDLE":     "no session activity and no busy tool process within the idle timeout",
	"WALL":     "the wall-clock cap was reached",
	"ORPHANED": "the worker exited but left processes behind",
	// ATR-V0-035: the applied role's hardTokens was reached.
	"CONTEXT": "its context estimate reached the role's hardTokens",
	// CAL-V0-191: set by heal after it reaps the worker's expired attempt.
	"LEASE_EXPIRED": "its attempt's lease expired beyond the grace and was reaped",
}

// idleCPUDivisor sets the CAL-V0-218 busy share: a child is busy when the
// CPU time it used since the baseline is at least 1/idleCPUDivisor (1%) of
// the wall time since then.
const idleCPUDivisor = 100

// refineChildCPU reads the counted children's precise CPU times; tests
// replace it to keep synthetic process tables.
var refineChildCPU = preciseCPU

// childCPU is a worker's CAL-V0-218 baseline: when it was taken and each
// non-ignored child's cumulative CPU time then, by childKey.
type childCPU struct {
	at  time.Time
	cpu map[string]time.Duration
}

// active reports output growth or an activity path advancing since the
// previous tick, or a busy tool process since the last activity (CAL-V0-218).
func (d *Dispatcher) active(w *Worker, procs map[int]proc, h Host, now time.Time) bool {
	dir := d.workerDir(w.ID)
	size := workerLogBytes(dir)
	busy := size != w.LogBytes
	// CAL-V0-157: usage is read before the cut, which then loses whatever
	// the read did not reach.
	readUsage(dir, w.Usage)
	// CAL-V0-143: the cut follows the comparison, so output that is then
	// truncated still counts as activity, and the next tick compares against
	// the size left after the cut.
	if cut, stdout := capWorkerLogs(dir); cut {
		size = workerLogBytes(dir)
		if stdout && w.Usage != nil {
			w.Usage.cut()
		}
	}
	w.LogBytes = size
	for _, p := range w.ActivityPaths {
		if st, err := os.Stat(p); err == nil && st.ModTime().After(w.ActivityMtime) {
			if !w.ActivityMtime.IsZero() {
				busy = true
			}
			w.ActivityMtime = st.ModTime()
		}
	}
	// Without a baseline (a new launch, or every adopted worker after a
	// restart) each non-ignored child is new, so it counts busy once.
	base, seen := d.childCPU[w.ID]
	var pids []int
	for _, m := range w.Members {
		if m.PID != w.PID && !contains(h.IdleIgnore, procs[m.PID].comm) {
			pids = append(pids, m.PID)
		}
	}
	refineChildCPU(procs, pids)
	if !busy {
		busy = busyChild(w, procs, h.IdleIgnore, base, now)
	}
	if busy || !seen {
		if d.childCPU == nil {
			d.childCPU = map[string]childCPU{}
		}
		d.childCPU[w.ID] = childSample(w, procs, h.IdleIgnore, now)
	}
	return busy
}

// heal hands off the live attempts of ended workers and reaps expired
// leases through the fenced lease transactions. It reports whether it
// attempted any store write.
func (d *Dispatcher) heal(ctx context.Context, obs *Observation, ended []*Worker) bool {
	wrote := false
	done := map[string]bool{}
	// ATR-V0-122: return the members held for ended workers before any
	// hand-off, whether or not heal.handoff is configured.
	if d.releasePoolHolds(ctx, obs, ended) {
		wrote = true
	}
	if d.recoverExits(ctx, obs, done) {
		wrote = true
	}
	d.deferredNow = map[string]string{}
	if d.Config.Heal.Handoff {
		// CAL-V0-146..148: hand-offs wait for verified detached runs.
		fresh := d.deferHandoffs(obs, ended, done)
		if d.advanceDetached(ctx, obs, done, fresh) {
			wrote = true
		}
		for _, w := range ended {
			for _, a := range obs.Attempts {
				if ctx.Err() != nil {
					return wrote
				}
				// An attempt under (or past) an exit recovery is never handed off
				// afresh: a worker re-reported as ended after a failed tick must
				// not reset the recovery's bounded tries or reuse its request IDs.
				if !a.Live || a.Holder != w.ID || done[a.ID] || d.recoveries[a.ID] != nil {
					continue
				}
				done[a.ID], wrote = true, true
				d.handoff(ctx, obs, w.ID, w.Role, a, d.contextHandoffDetail(w))
			}
		}
	}
	d.stopReapedWorkers(obs)
	if d.Config.Heal.Reap {
		now := d.Now()
		for _, a := range obs.Attempts {
			if ctx.Err() != nil {
				return wrote
			}
			if !a.Live || done[a.ID] || a.LeaseExpires.IsZero() || a.LeaseExpires.After(now) {
				continue
			}
			if w := d.worker(a.Holder); w != nil {
				// An ended worker (empty tree) is left to its hand-off and
				// the next pass, as before CAL-V0-191.
				if len(w.Members) > 0 && d.reapRunningExpired(ctx, obs, w, a, now) {
					wrote = true
				}
				continue
			}
			wrote = true
			detail := map[string]string{"attempt": a.ID, "generation": a.Generation, "holder": a.Holder}
			if err := d.Queue.Reap(ctx, a, requestID("reap", a.ID, a.Generation)); err != nil {
				detail["error"] = err.Error()
				d.emit(Event{Kind: "alert", Ticket: a.Ticket, Message: fmt.Sprintf("could not reap the expired lease of %s held by %s: %v", d.local(obs, a.Ticket), a.Holder, err), Detail: detail})
				continue
			}
			d.emit(Event{Kind: "reaped", Ticket: a.Ticket, Message: fmt.Sprintf("reaped the expired lease of %s held by %s", d.local(obs, a.Ticket), a.Holder), Detail: detail})
		}
	}
	return wrote
}

// reapRunningExpired reaps the live attempt a of the still-running worker w
// once its lease has been expired for longer than the role's grace
// (CAL-V0-191). The reap is fenced on the lease expiry observed here, and w
// is stopped with the wall-time cap's TERM-then-KILL only when the store
// reports that this request reaped the attempt: a worker that released,
// completed or renewed first keeps running. It reports whether it attempted
// a store write.
func (d *Dispatcher) reapRunningExpired(ctx context.Context, obs *Observation, w *Worker, a Attempt, now time.Time) bool {
	grace := d.launchConfig(w).roleNamed(w.Role).ExpiredLeaseGrace()
	// ATR-V0-064: time the host slept after the expiry does not count.
	if now.Sub(a.LeaseExpires)-d.slept(a.LeaseExpires, now) <= grace {
		return false
	}
	local := d.local(obs, a.Ticket)
	expires := a.LeaseExpires.UTC().Format(time.RFC3339)
	detail := map[string]string{"attempt": a.ID, "generation": a.Generation, "holder": a.Holder, "phase": a.Phase, "leaseExpires": expires, "graceSeconds": strconv.Itoa(int(grace / time.Second))}
	reaped, err := d.Queue.ReapExpired(ctx, a, requestID("reap", a.ID, a.Generation, "lease-expired", expires))
	if err != nil {
		detail["error"] = err.Error()
		d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not reap the expired lease of %s held by running worker %s: %v", local, w.ID, err), Detail: detail})
		return true
	}
	if !reaped {
		return true
	}
	d.emit(Event{Kind: "lease-expired", Ticket: a.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("reaped attempt %s on %s: its lease expired at %s, more than %ds ago, while %s still ran; stopping the worker", a.ID, local, expires, int(grace/time.Second), w.ID), Detail: detail})
	d.stopLeaseExpired(w)
	return true
}

// stopReapedWorkers stops every running worker of this dispatcher whose
// attempt the store holds as reaped (FAILED, cause LEASE_EXPIRED) while it
// holds no live attempt (CAL-V0-191). The store is the durable record of a
// reap, so a stop lost to a dispatcher that ended after the reap but before
// its ledger recorded KILLING is recovered on the next tick, whatever the
// reap switch now says.
func (d *Dispatcher) stopReapedWorkers(obs *Observation) {
	reaped, live := map[string]bool{}, map[string]bool{}
	for _, a := range obs.Attempts {
		switch {
		case a.Holder == "":
		case a.Live:
			live[a.Holder] = true
		case a.Phase == "FAILED" && a.Cause == "LEASE_EXPIRED":
			reaped[a.Holder] = true
		}
	}
	for _, w := range d.ledger.Workers {
		if len(w.Members) > 0 && w.State != "KILLING" && reaped[w.ID] && !live[w.ID] {
			d.stopLeaseExpired(w)
		}
	}
}

// keepReclaimedWorkers cancels a LEASE_EXPIRED stop that this process
// decided and has not signalled yet when this observation shows the worker
// holding a live attempt, such as one it claimed after the observation that
// decided the stop (CAL-V0-191). A stop that has reached supervision, or
// that a restart finds in the ledger (whose kill deadline may predate a
// signal sent before an unsaved tick), always runs to completion.
func (d *Dispatcher) keepReclaimedWorkers(obs *Observation) {
	if obs == nil {
		return
	}
	for _, w := range d.ledger.Workers {
		if !d.unsignalled[w.ID] || w.State != "KILLING" || w.KillReason != "LEASE_EXPIRED" || !w.KillDeadline.IsZero() {
			continue
		}
		for _, a := range obs.Attempts {
			if a.Live && a.Holder == w.ID {
				w.State, w.KillReason = "RUNNING", ""
				delete(d.unsignalled, w.ID)
				d.emit(Event{Kind: "alert", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("kept %s worker %s running: it holds live attempt %s, so its lease-expired stop is cancelled before any signal", w.Role, w.ID, a.ID), Detail: map[string]string{"attempt": a.ID, "generation": a.Generation, "reason": "LEASE_EXPIRED"}})
				break
			}
		}
	}
}

// stopLeaseExpired marks w KILLING with reason LEASE_EXPIRED, so the next
// supervision pass stops it as the wall-time cap does.
func (d *Dispatcher) stopLeaseExpired(w *Worker) {
	if w.State == "KILLING" {
		return
	}
	w.State, w.KillReason = "KILLING", "LEASE_EXPIRED"
	if d.unsignalled == nil {
		d.unsignalled = map[string]bool{}
	}
	d.unsignalled[w.ID] = true
	d.emit(Event{Kind: "killing", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("stopping %s worker %s on %s: %s", w.Role, w.ID, d.keyText(w.Key), killText["LEASE_EXPIRED"]), Detail: map[string]string{"reason": "LEASE_EXPIRED", "processes": strconv.Itoa(len(w.Members))}})
}

// handoff releases the live attempt a of the ended worker through the
// fenced lease transaction; a refused release starts its CAL-V0-104 exit
// recovery. extra adds event detail.
func (d *Dispatcher) handoff(ctx context.Context, obs *Observation, worker, role string, a Attempt, extra map[string]string) {
	evidence := ""
	if a.Candidate == "" {
		evidence = "dispatch:" + worker
	}
	err := d.Queue.Release(ctx, a, evidence, requestID("release", worker, a.ID, a.Generation))
	detail := map[string]string{"attempt": a.ID, "generation": a.Generation, "phase": a.Phase}
	maps.Copy(detail, extra)
	if err != nil {
		detail["error"] = err.Error()
		if d.Config.Heal.ExitRecoveryOn() {
			detail["recovery"] = "RETRYING"
			d.emit(Event{Kind: "handoff-refused", Ticket: a.Ticket, Role: role, Worker: worker, Message: fmt.Sprintf("could not hand off %s attempt %s after %s ended: %v; retrying, then reaping once its lease expires", d.local(obs, a.Ticket), a.ID, worker, err), Detail: detail})
			r := &exitRecovery{attempt: a, worker: worker, role: role, releases: 1, releaseErr: err.Error(), context: contextOnly(extra)}
			if now := d.Now(); a.LeaseExpires.IsZero() || a.LeaseExpires.After(now) {
				r.next = now.Add(d.recoveryBackoff(1))
			}
			if d.recoveries == nil {
				d.recoveries = map[string]*exitRecovery{}
			}
			d.recoveries[a.ID] = r
			d.stepRecovery(ctx, obs, r, a) // an already expired lease is reaped at once
			return
		}
		d.emit(Event{Kind: "handoff-refused", Ticket: a.Ticket, Role: role, Worker: worker, Message: fmt.Sprintf("could not hand off %s attempt %s after %s ended: %v", d.local(obs, a.Ticket), a.ID, worker, err), Detail: detail})
		d.emit(Event{Kind: "needs-owner", Ticket: a.Ticket, Worker: worker, Message: fmt.Sprintf("%s still holds a live %s attempt that the dispatcher could not hand off; inspect it with `corvint-tasks attempt show %s`", d.local(obs, a.Ticket), a.Phase, a.ID), Detail: detail})
		return
	}
	d.noteHandedOff(worker)
	d.emit(Event{Kind: "handoff", Ticket: a.Ticket, Role: role, Worker: worker, Message: fmt.Sprintf("handed off %s (attempt %s) after %s ended", d.local(obs, a.Ticket), a.ID, worker), Detail: detail})
}

// exitRecoveryTries bounds the hand-off releases (the first included) and
// the reaps of one CAL-V0-104 exit recovery.
const exitRecoveryTries = 3

// exitRecoveryMaxBackoff caps the delay between two recovery writes.
const exitRecoveryMaxBackoff = 5 * time.Minute

// exitRecovery is one ended worker's attempt whose hand-off was refused. It
// acts only on that attempt at the generation and holder observed when the
// worker ended, so a reclaimed or replaced attempt is never touched.
type exitRecovery struct {
	attempt             Attempt
	worker, role        string
	releases, reaps     int
	next                time.Time
	releaseErr, reapErr string
	// context is the ATR-V0-035 detail every event of the recovery carries.
	context map[string]string
	// exhausted marks a recovery that already reported needs-owner. It stays
	// recorded, writing nothing, until the attempt ends or is superseded, so
	// the same attempt is never recovered or reported twice.
	exhausted bool
}

// recoveryBackoff is the delay after the n-th write of one recovery:
// tickSeconds doubled per write, capped at five minutes.
func (d *Dispatcher) recoveryBackoff(n int) time.Duration {
	delay := time.Duration(d.Config.TickSeconds) * time.Second
	for i := 1; i < n && delay < exitRecoveryMaxBackoff; i++ {
		delay *= 2
	}
	return min(delay, exitRecoveryMaxBackoff)
}

// recoverExits advances every pending exit recovery in attempt order and
// marks the attempts it still owns as done for this heal pass. It reports
// whether it attempted any store write.
func (d *Dispatcher) recoverExits(ctx context.Context, obs *Observation, done map[string]bool) bool {
	wrote := false
	for _, id := range slices.Sorted(maps.Keys(d.recoveries)) {
		if ctx.Err() != nil {
			return wrote
		}
		r := d.recoveries[id]
		var cur *Attempt
		for i := range obs.Attempts {
			if obs.Attempts[i].ID == id {
				cur = &obs.Attempts[i]
			}
		}
		detail := map[string]string{"attempt": id, "generation": r.attempt.Generation, "holder": r.worker}
		maps.Copy(detail, r.context)
		if r.exhausted {
			// Already reported to the owner; only heal.reap still applies.
			if cur == nil || !cur.Live || cur.Generation != r.attempt.Generation || cur.Holder != r.attempt.Holder {
				delete(d.recoveries, id)
			}
			continue
		}
		switch {
		case cur == nil || !cur.Live:
			// Ended elsewhere (FAILED/FENCED, released or reaped): resolved.
			delete(d.recoveries, id)
			phase := "NOT_OBSERVED"
			if cur != nil {
				phase = cur.Phase
			}
			detail["recovery"], detail["phase"] = "ALREADY_ENDED", phase
			d.emit(Event{Kind: "handoff", Ticket: r.attempt.Ticket, Role: r.role, Worker: r.worker, Message: fmt.Sprintf("attempt %s on %s needs no hand-off any more: it ended as %s", id, d.local(obs, r.attempt.Ticket), phase), Detail: detail})
			continue
		case cur.Generation != r.attempt.Generation || cur.Holder != r.attempt.Holder:
			// Fenced: another holder or generation owns it now; never touch it.
			delete(d.recoveries, id)
			detail["recovery"], detail["current_generation"], detail["current_holder"] = "SUPERSEDED", cur.Generation, cur.Holder
			d.emit(Event{Kind: "handoff", Ticket: r.attempt.Ticket, Role: r.role, Worker: r.worker, Message: fmt.Sprintf("attempt %s on %s moved to generation %s held by %s; the dispatcher leaves it alone", id, d.local(obs, r.attempt.Ticket), cur.Generation, orText(cur.Holder, "nobody")), Detail: detail})
			continue
		}
		done[id] = true
		if d.stepRecovery(ctx, obs, r, *cur) {
			wrote = true
		}
	}
	return wrote
}

// stepRecovery makes at most one due store write for r against the current
// attempt a: a reap once the lease has expired, otherwise a hand-off
// release, each bounded with backoff. Only when both are exhausted (or no
// lease exists to reap) does it report needs-owner and mark r exhausted.
func (d *Dispatcher) stepRecovery(ctx context.Context, obs *Observation, r *exitRecovery, a Attempt) bool {
	now := d.Now()
	if now.Before(r.next) || ctx.Err() != nil {
		return false
	}
	local := d.local(obs, a.Ticket)
	detail := map[string]string{"attempt": a.ID, "generation": a.Generation, "holder": r.worker, "phase": a.Phase}
	maps.Copy(detail, r.context)
	expired := !a.LeaseExpires.IsZero() && !a.LeaseExpires.After(now)
	wrote := false
	switch {
	case expired && r.reaps < exitRecoveryTries:
		r.reaps++
		wrote = true
		detail["try"] = strconv.Itoa(r.reaps)
		if err := d.Queue.Reap(ctx, a, requestID("reap", a.ID, a.Generation, "exit", strconv.Itoa(r.reaps))); err != nil {
			r.reapErr, detail["error"] = err.Error(), err.Error()
			r.next = now.Add(d.recoveryBackoff(r.reaps))
			d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: r.worker, Message: fmt.Sprintf("could not reap the expired lease of %s after %s ended (try %d of %d): %v", local, r.worker, r.reaps, exitRecoveryTries, err), Detail: detail})
			break
		}
		delete(d.recoveries, a.ID)
		detail["recovery"] = "REAPED"
		d.emit(Event{Kind: "reaped", Ticket: a.Ticket, Worker: r.worker, Message: fmt.Sprintf("reaped the expired lease of %s after %s ended without releasing it", local, r.worker), Detail: detail})
		return true
	case !expired && r.releases < exitRecoveryTries:
		r.releases++
		wrote = true
		detail["try"] = strconv.Itoa(r.releases)
		evidence := ""
		if a.Candidate == "" {
			evidence = "dispatch:" + r.worker
		}
		if err := d.Queue.Release(ctx, a, evidence, requestID("release", r.worker, a.ID, a.Generation, "exit", strconv.Itoa(r.releases))); err != nil {
			r.releaseErr, detail["error"] = err.Error(), err.Error()
			r.next = now.Add(d.recoveryBackoff(r.releases))
			detail["recovery"] = "RETRYING"
			d.emit(Event{Kind: "handoff-refused", Ticket: a.Ticket, Role: r.role, Worker: r.worker, Message: fmt.Sprintf("could not hand off %s attempt %s after %s ended (try %d of %d): %v", local, a.ID, r.worker, r.releases, exitRecoveryTries, err), Detail: detail})
			break
		}
		delete(d.recoveries, a.ID)
		detail["recovery"] = "RELEASED"
		d.noteHandedOff(r.worker)
		d.emit(Event{Kind: "handoff", Ticket: a.Ticket, Role: r.role, Worker: r.worker, Message: fmt.Sprintf("handed off %s (attempt %s) after %s ended, on retry %d", local, a.ID, r.worker, r.releases), Detail: detail})
		return true
	case !expired && !a.LeaseExpires.IsZero():
		// Releases are exhausted: wait, without writing, for the lease to expire.
		r.next = a.LeaseExpires
		return false
	}
	if r.reaps >= exitRecoveryTries || (a.LeaseExpires.IsZero() && r.releases >= exitRecoveryTries) {
		r.exhausted = true
		detail["release_error"], detail["reap_error"] = r.releaseErr, orText(r.reapErr, "NOT_ATTEMPTED")
		delete(detail, "error")
		delete(detail, "try")
		d.emit(Event{Kind: "needs-owner", Ticket: a.Ticket, Worker: r.worker, Message: fmt.Sprintf("%s still holds a live %s attempt that the dispatcher could neither hand off (%d tries) nor reap (%d tries); inspect it with `corvint-tasks attempt show %s`", local, a.Phase, r.releases, r.reaps, a.ID), Detail: detail})
	}
	return wrote
}

// requestID is a deterministic native request ID, so a repeated heal after
// a crash replays rather than duplicates.
func requestID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "dispatch-" + parts[0] + "-" + hex.EncodeToString(sum[:12])
}

// finish accounts for ended workers: progress resets backoff; no progress
// starts a cooldown and, at parkAfter, parks the key for the owner.
func (d *Dispatcher) finish(obs *Observation, ended []*Worker, granted, pending map[string]bool) {
	now := d.Now()
	// CAL-V0-144: retirement runs once the finished workers left the ledger,
	// and on its own once pending marks are due for confirmation.
	if len(ended) > 0 || !d.retireConfirm.IsZero() && !now.Before(d.retireConfirm) {
		defer d.retireWorkerDirs()
	}
	for _, w := range ended {
		if pending[w.ID] {
			d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: "declared progress accounting deferred until a known work state"})
			continue
		}
		if current := d.worker(w.ID); current != nil {
			w = current // first-token seeding published a cloned baseline
		}
		if exit := d.exits[w.ID]; exit != nil {
			// The tree can empty a moment before Wait reports the exit.
			select {
			case c := <-exit:
				d.codes[w.ID] = c
			case <-time.After(200 * time.Millisecond):
			}
		}
		code := "NOT_OBSERVED"
		st, exitSeen := d.codes[w.ID]
		if exitSeen {
			code = strconv.Itoa(st.code)
		}
		fp := Fingerprint(obs, w.Key)
		// An unreadable work state is not evidence of progress.
		unknown := stateUnknown(obs, w.Key)
		if unknown {
			fp = w.Fingerprint
		}
		progress := granted[w.ID] || fp != w.Fingerprint
		// CAL-V0-272: a run without progress whose work state was UNKNOWN
		// at launch, during the run or at its end is still charged as
		// CAL-V0-057 requires, and its events record the UNKNOWN state.
		workUnknown := !progress && (w.WorkStateUnknown || workStateUnknown(obs, w.Key))
		t := observed(obs, w.Key)
		class := d.classify(t, w, progress)
		// ATR-V0-350..353: the structured outcome, the observed facts, and
		// the label and summary derived from them.
		reason, signal := stopReason(w.KillReason, st, exitSeen), exitSignalText(st, exitSeen)
		commits, truncated, commitsSeen := d.finishCommits(w.ID)
		noteEvent := d.saveSalvage(w, now, obs) // ATR-V0-104, before the note is reported
		label := progressLabel(class, reason, signal, len(commits), commitsSeen)
		summary := finishSummary(d.workerDir(w.ID), reason)
		msg := fmt.Sprintf("%s worker %s finished on %s (exit %s, %s)", w.Role, w.ID, d.keyText(w.Key), code, label)
		run := d.deferredNow[w.ID]
		if run != "" {
			msg += "; its detached run " + run + " continues"
		}
		if summary != "" {
			msg += ": " + summary
		}
		detail := map[string]string{"exitCode": code, "progress": strconv.FormatBool(progress), "session": class, "killReason": w.KillReason, "runSeconds": strconv.Itoa(int(now.Sub(w.Started).Seconds())),
			"exitSignal": signal, "stopReason": reason, "progressLabel": label, "attemptRelease": d.attemptRelease(obs, w), "handoffNote": strconv.FormatBool(d.hasHandoffNote(w.ID)), "commitCount": "NOT_OBSERVED"}
		if commitsSeen {
			detail["commitCount"], detail["commits"] = strconv.Itoa(len(commits)), strings.Join(commits, ",")
			if truncated {
				detail["commitsTruncated"] = "true"
			}
		}
		if run != "" {
			detail["detachedRun"] = run
		}
		if workUnknown {
			detail["workState"] = StateUnknown
		}
		if n := carriedCount(d.dir, w.ID); n > 0 {
			detail["carryOver"] = strconv.Itoa(n) // ATR-V0-114
		}
		d.finishUsage(w, detail)
		d.finishContextDetail(w, detail, now) // ATR-V0-127, DSG-V0-009
		stalled := d.countStall(t, w, detail)
		d.emit(Event{Kind: "finished", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: msg, Detail: detail})
		d.noteSession(w, now)
		if noteEvent != nil {
			d.emit(*noteEvent) // ATR-V0-104
		}
		d.noteHardStop(w, now)                              // ATR-V0-106
		d.noteCarryOver(w, now, class, progress, run != "") // ATR-V0-111
		if stalled != nil {
			d.emit(*stalled)
		}
		d.remove(w.ID)
		// ESC-V0-008: classification precedes no-progress parking. An
		// infrastructure session is unknown progress for the ladder and a
		// held session is the typed policy's, so neither counts or parks.
		if class == SessionInfrastructure && d.chargeInfra(t, w) {
			d.accountUnknown(w.Key)
			continue
		}
		d.infraEnded(w.Key, w.ID, progress)
		// CAL-V0-148: a session that left its detached run running is not
		// a run without progress; the run's outcome decides the next one.
		if class == SessionHeld || run != "" && !progress {
			continue
		}
		if t != nil && t.EscalationUnknown && d.Config.InfrastructureRetry != nil {
			d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: "escalation material of " + d.keyText(w.Key) + " is UNKNOWN; the session is accounted as ordinary no-progress"})
		}
		d.account(w.Key, progress)
		if progress {
			delete(d.ledger.Backoff, w.Key)
			continue
		}
		b := d.ledger.Backoff[w.Key]
		if b == nil {
			b = &BackoffState{}
			d.ledger.Backoff[w.Key] = b
		}
		b.NoProgress++
		b.Fingerprint = fp
		b.BaseFingerprint, b.ProgressDigest = w.BaseFingerprint, w.ProgressDigest
		if b.ProgressDigest != "" && !unknown {
			b.BaseFingerprint = baseFingerprint(obs, w.Key)
		}
		var charged map[string]string
		note := ""
		if workUnknown {
			charged, note = map[string]string{"workState": StateUnknown}, "; its work state was UNKNOWN during the run"
		}
		if obs.ProgressSignal != "" {
			// ATR-V0-084: the backoff names the declared signal that judged
			// the run.
			if charged == nil {
				charged = map[string]string{}
			}
			charged["progressSignal"] = obs.ProgressSignal
			note += "; judged under progress signal " + obs.ProgressSignal
		}
		if b.NoProgress >= d.Config.Backoff.ParkAfter {
			b.Parked = true
			d.emit(Event{Kind: "parked", Ticket: w.Ticket, Message: fmt.Sprintf("parked %s after %d run(s) without progress%s", d.keyText(w.Key), b.NoProgress, note), Detail: charged})
			d.emit(Event{Kind: "needs-owner", Ticket: w.Ticket, Message: fmt.Sprintf("%s is parked: %d dispatcher run(s) made no progress. It resumes when its state changes, or run `corvint-tasks dispatch unpark --program %s --config FILE --key %s`", d.keyText(w.Key), b.NoProgress, d.Program, w.Key)})
			continue
		}
		b.CooldownUntil = now.Add(time.Duration(d.Config.Backoff.CooldownSeconds) * time.Second)
		d.emit(Event{Kind: "cooldown", Ticket: w.Ticket, Message: fmt.Sprintf("%s cools down until %s after a run without progress (%d of %d before parking)%s", d.keyText(w.Key), b.CooldownUntil.UTC().Format(time.RFC3339), b.NoProgress, d.Config.Backoff.ParkAfter, note), Detail: charged})
	}
	d.pruneInfra(obs)
	d.pruneStall(obs)
	if d.Config.Escalates() {
		present := map[string]bool{}
		for _, t := range obs.Tickets {
			present[t.ID] = true
		}
		for key := range d.ledger.Escalation {
			if !present[key] {
				delete(d.ledger.Escalation, key)
			}
		}
	}
}

// account updates the CAL-V0-057 ladder of a ticket after one finished
// session: no progress extends its streak; progress resets it and returns
// every deescalating role to its base model.
func (d *Dispatcher) account(key string, progress bool) { d.accountOn(d.ledger, key, progress) }

// accountOn is account on the given ledger.
func (d *Dispatcher) accountOn(l *Ledger, key string, progress bool) {
	if !d.Config.Escalates() || strings.HasPrefix(key, "lane:") {
		return
	}
	e := l.Escalation[key]
	if !progress {
		if e == nil {
			e = &EscalationState{}
			if l.Escalation == nil {
				l.Escalation = map[string]*EscalationState{}
			}
			l.Escalation[key] = e
		}
		e.Streak++
		return
	}
	if e == nil {
		return
	}
	e.Streak = 0
	for role := range e.Tiers {
		if r := d.role(role); r == nil || r.Deescalates() {
			delete(e.Tiers, role)
		}
	}
	if len(e.Tiers) == 0 {
		delete(l.Escalation, key)
	}
}

// reconcileEscalation drops ladder records the configuration no longer
// supports: all of them without a ladder, and the tiers of roles that no
// longer escalate or have fewer tiers.
func (d *Dispatcher) reconcileEscalation() {
	if !d.Config.Escalates() {
		d.ledger.Escalation = nil
		return
	}
	for _, e := range d.ledger.Escalation {
		for role, tier := range e.Tiers {
			if r := d.role(role); r == nil || len(r.Escalate) == 0 {
				delete(e.Tiers, role)
			} else if tier > len(r.Escalate) {
				e.Tiers[role] = len(r.Escalate)
			}
		}
	}
}

// LaunchTier is the tier a role launches at on a ticket with this ladder
// record: the streak's tier, or the role's last tier when it is higher and
// the role does not deescalate.
func LaunchTier(r *Role, e *EscalationState) int {
	if r == nil || len(r.Escalate) == 0 || e == nil {
		return 0
	}
	tier := r.TierFor(e.Streak)
	if last := e.Tiers[r.Name]; !r.Deescalates() && last > tier {
		tier = min(last, len(r.Escalate))
	}
	return tier
}

// UnparkRequest is one queued operator request file.
type UnparkRequest struct {
	Unpark string `json:"unpark"`
}

// unpark treats a known state change of a backed-off key with no worker as
// progress made outside a session: it resets the key's ladder and releases
// the key when parked, or resets its parking count while keeping cooldown.
// It also consumes operator unpark requests, which
// release a key without resetting its ladder.
func (d *Dispatcher) unpark(obs *Observation) {
	keys := make([]string, 0, len(d.ledger.Backoff))
	for k := range d.ledger.Backoff {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b := d.ledger.Backoff[k]
		if stateUnknown(obs, k) || Fingerprint(obs, k) == b.Fingerprint || d.busy(k) {
			continue
		}
		d.account(k, true)
		if b.Parked {
			delete(d.ledger.Backoff, k)
			d.emit(Event{Kind: "unparked", Ticket: ticketOf(k), Message: fmt.Sprintf("unparked %s because its state changed", d.keyText(k))})
		} else {
			// CAL-V0-057: only sessions after this progress may park the key.
			b.NoProgress = 0
		}
	}
	entries, err := os.ReadDir(filepath.Join(d.dir, "requests"))
	if err != nil {
		d.emit(Event{Kind: "alert", Message: "unpark requests unreadable: " + err.Error()})
		return
	}
	for _, e := range entries {
		path := filepath.Join(d.dir, "requests", e.Name())
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := readBounded(path, 4096)
		var r UnparkRequest
		if err == nil {
			err = json.Unmarshal(raw, &r)
		}
		if err == nil && r.Unpark == "" {
			err = errors.New("empty unpark key")
		}
		if err != nil {
			d.emit(Event{Kind: "alert", Message: fmt.Sprintf("discarded unpark request %s: %v", e.Name(), err)})
		} else {
			if _, ok := d.ledger.Backoff[r.Unpark]; ok {
				delete(d.ledger.Backoff, r.Unpark)
				d.emit(Event{Kind: "unparked", Ticket: ticketOf(r.Unpark), Message: fmt.Sprintf("unparked %s at the operator's request", d.keyText(r.Unpark))})
			}
			if d.releaseInfra(r.Unpark) {
				d.emit(Event{Kind: "unparked", Ticket: ticketOf(r.Unpark), Message: fmt.Sprintf("released the infrastructure retry hold of %s at the operator's request; its retry count is kept", d.keyText(r.Unpark))})
			}
		}
		os.Remove(path)
	}
}

// stateUnknown reports a ticket key whose program work state is UNKNOWN,
// or whose obligation ledger the observation could not fold (TOL-V0-124).
func stateUnknown(obs *Observation, key string) bool {
	for _, t := range obs.Tickets {
		if t.ID == key {
			return t.State == StateUnknown || t.EscalationUnknown || t.Obligations != nil && t.Obligations.Unknown
		}
	}
	return false
}

func ticketOf(key string) string {
	if strings.HasPrefix(key, "lane:") {
		return ""
	}
	return key
}

// launchRoster starts every roster assignment as an independent worker.
func (d *Dispatcher) launchRoster(ctx context.Context, obs *Observation) {
	now := d.Now()
	busy := make([]Busy, 0, len(d.ledger.Workers))
	for _, w := range d.ledger.Workers {
		busy = append(busy, Busy{Role: w.Role, Key: w.Key, Slot: w.Slot, Tier: w.Tier})
	}
	skip := map[string]bool{}
	for k, b := range d.ledger.Backoff {
		skip[k] = b.Parked || b.CooldownUntil.After(now)
	}
	d.churnHeld(skip, now)
	for k := range d.ledger.InfraRetry {
		if d.infraHolds(obs, k) {
			skip[k] = true
		}
	}
	for _, m := range obs.Members {
		if d.sweepLaneHeld(m.Pool, m.Member) {
			skip[laneKey(m.Pool, m.Member)] = true
		}
	}
	d.stampMemberAges(obs, now)
	d.syncSpend(now)
	// CAL-V0-273: the opt-in hold launches nothing while the work state
	// has been unreadable for the configured number of ticks.
	if WorkStateHoldsLaunches(d.Config, d.ledger.WorkState) {
		d.recordAdmission(obs, nil, d.pressureLabel(), now)
		return
	}
	tierOf := func(role, key string) int { return LaunchTier(d.role(role), d.ledger.Escalation[key]) }
	// CAL-V0-149: a finished detached run relaunches its ticket's role.
	relaunch := d.relaunches()
	prefer := preferredRoles(relaunch)
	var (
		adm            *admission
		spend          *spendGate
		room           *poolRoom
		launches, held []Assignment
		pressure       string
		budget         *PressureBudget
	)
	plan := func() {
		// ATR-V0-150..152: the pressure label after this tick's sample.
		pressure = d.pressureLabel()
		adm = &admission{calm: pressure == AdmissionPressureCalm}
		spend = d.spendGate(now)
		spend.observe(obs.Tickets)
		spend.holdDeferred(obs.Tickets, prefer)
		room = newPoolRoom(obs, d.ledger.Workers)
		launches, held = roster(d.Config, obs, busy, skip, tierOf, budget, prefer, spend, room, adm)
	}
	// ATR-V0-181: with no worker running and a recorded calm level, the
	// sample waits until a candidate passes every static fence. Calm is
	// the most permissive label, so a roster that consults no budget at
	// calm launches nothing whatever the host reads, and the tick samples
	// nothing. A consulted deferral refuses, and the roster is planned
	// again after the sample. While a worker runs, every tick samples, so
	// the hysteresis keeps tracking a host that is doing work.
	deferred := false
	if d.Config.Pressure != nil && d.ledger.Pressure != nil && len(d.ledger.Workers) == 0 && !d.pressureBaseline && d.pressureLabel() == AdmissionPressureCalm {
		budget = &PressureBudget{deferred: true}
		plan()
		deferred = !budget.consulted
	}
	if deferred {
		d.pressureGap = true
	} else {
		var ok bool
		if budget, ok = d.pressureBudget(ctx, obs); !ok {
			d.recordAdmission(obs, nil, d.pressureLabel(), now)
			return
		}
		plan()
	}
	// ATR-V0-150: the observation is written after the launch loop, so it
	// counts the workers actually recorded.
	defer d.recordAdmission(obs, adm, pressure, now)
	d.recordHeld(obs, held)
	d.recordPoolWait(room)
	// Holds are recorded after the launches, so a budget event counts them.
	defer d.recordBudget(spend)
	retried := map[string]bool{}
	for _, a := range launches {
		// A cancelled dispatcher (service stop) launches nothing further.
		if ctx.Err() != nil {
			return
		}
		// A retry is one launch: other roles wait for its session to end.
		if retried[a.Key] {
			continue
		}
		role := d.role(a.Role)
		host := d.Config.Hosts[role.Host]
		model := role.ModelAt(a.Tier)
		d.ledger.LaunchSeq++
		// Program and role names cannot contain '.', so the ID never collides
		// across programs or roles; the start nonce keeps it unique per run.
		id := fmt.Sprintf("%s.%s.%d.%s-%d", d.Program, a.Role, a.Slot, d.nonce, d.ledger.LaunchSeq)
		// ESC-V0-007: a due retry writes its launch identity into the
		// checked ledger before anything can spawn, and a republish after
		// a proved no-spawn reuses that identity without another charge.
		ep := d.infraDue(obs, a.Key)
		if ep != nil {
			retried[a.Key] = true
			if ep.Launch != "" {
				// The saved identity names its role and slot: it launches
				// only as that assignment, never under a new identity.
				slot, ok := d.reservedSlot(ep.Launch, a.Role)
				if !ok {
					ep.State, ep.Launch = InfraUnknown, ""
					d.emit(Event{Kind: "needs-owner", Ticket: a.Ticket, Message: fmt.Sprintf("%s is held: its reserved infrastructure retry was not for role %s, which the roster now selects. Run `corvint-tasks dispatch unpark --program %s --config FILE --key %s` to let it launch again; its retry count is kept", d.keyText(a.Key), a.Role, d.Program, a.Key), Detail: map[string]string{"kind": "infrastructure", "code": "INFRA_RETRY_UNKNOWN", "acceptanceRevision": ep.AcceptanceRevision}})
					continue
				}
				if slot != a.Slot && slotBusy(d.ledger.Workers, launches, a.Role, slot) {
					continue // its slot is still in use; it waits
				}
				a.Slot, id = slot, ep.Launch
			}
			ep.Launch, ep.State = id, InfraReserved
			if err := d.ledger.save(d.dir); err != nil {
				ep.State = InfraWaiting
				d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: id, Message: "infrastructure retry not launched: its reservation could not be saved: " + err.Error()})
				continue
			}
		}
		// A controlled dispatcher holds the fence from its final control read
		// until the worker is recorded; a refusal launches nothing further.
		release, ok := d.admit(ctx, id)
		if !ok {
			if ep != nil {
				ep.State = InfraWaiting // nothing spawned; the identity is kept
			}
			return
		}
		// ATR-V0-141: the review packet is read before any pre-launch claim,
		// because a claim opens a generation without the submitted
		// candidate the packet diffs.
		var packet string
		var packetErr error
		reviewPacket := a.Ticket != "" && !strings.HasPrefix(a.Key, "lane:") && role.ReviewPacket.on()
		if reviewPacket {
			packet, packetErr = d.reviewPacketFor(ctx, obs, role.ReviewPacket, a.Ticket)
		}
		// ATR-V0-120: a holdPool role's ticket is claimed with its pool
		// member before anything can spawn; a refusal launches nothing.
		hold, ok := d.claimPoolHold(ctx, obs, role, a, id)
		if !ok {
			if ep != nil {
				ep.State = InfraWaiting // nothing spawned; the identity is kept
			}
			release(true)
			continue
		}
		// CAL-V0-149: the successor is named before anything can spawn, so a
		// crash can lose the relaunch but never repeat it.
		var run *detachedMarker
		// An infrastructure retry of the same role is that session too.
		if m := relaunch[a.Ticket]; m != nil && a.Ticket != "" && m.Role == a.Role {
			m.Phase, m.Successor = DetachedLaunched, id
			if err := d.saveDetached(m); err != nil {
				m.Phase, m.Successor = DetachedRelaunch, ""
				if ep != nil {
					ep.State = InfraWaiting // nothing spawned; the identity is kept
				}
				if hold != nil {
					d.returnPoolHold(ctx, hold, a.Role, true) // ATR-V0-122
				}
				release(true)
				d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: id, Message: fmt.Sprintf("relaunch for detached run %s not started: its marker could not be saved: %v", m.RunID, err)})
				continue
			}
			run = m
		}
		// ATR-V0-340..342: the launch directory; an unresolved one under
		// fallback REFUSE is proved before spawn.
		cwd, cwdSource, cwdErr := d.launchCwd(obs, role, a.Ticket, a.Local)
		values := map[string]string{"{cwd}": cwd, "{program}": d.Program, "{role}": a.Role, "{slot}": strconv.Itoa(a.Slot), "{worker}": id, "{holder}": id, "{ticket}": a.Ticket, "{ticketLocal}": a.Local, "{state}": a.State, "{pool}": a.Pool, "{member}": a.Member, "{workRoot}": d.Config.WorkRoot, "{model}": model, "{effort}": role.Effort, "{nextStage}": a.NextStage, "{contextHookSettings}": ContextHookSettingsPath(d.dir, id)}
		// {operatorNote} renders only into the role prompt (ON-V0-011); argv,
		// env and activity paths never see the operator prose directly. So
		// does {detachedRun} (CAL-V0-149), empty for an ordinary launch.
		promptValues := maps.Clone(values)
		promptValues["{operatorNote}"] = RenderOperatorNote(a.Ticket, a.OperatorNote)
		promptValues["{detachedRun}"] = ""
		if run != nil {
			promptValues["{detachedRun}"] = detachedLine(run)
		}
		values["{prompt}"] = Render(role.Prompt, promptValues)
		// ATR-V0-113: the carry-over block follows the rendered prompt, so
		// its untrusted text is never expanded.
		var carryText string
		var carried []CarryEntry
		if a.Ticket != "" && !strings.HasPrefix(a.Key, "lane:") && role.CarryOver.on() {
			carryText, carried = d.carryOverFor(role.CarryOver, a.Ticket, Fingerprint(obs, a.Key))
			// ATR-V0-133: a store-derived argv value must not turn the
			// launched argv into one that runs the block as code.
			if err := renderedArgvSafe(host.Argv, values); carryText != "" && err != nil {
				if d.carryUnsafe == nil || d.carryUnsafeConfig != d.configSha256 {
					d.carryUnsafe, d.carryUnsafeConfig = map[string]bool{}, d.configSha256
				}
				if !d.carryUnsafe[a.Ticket] {
					d.carryUnsafe[a.Ticket] = true
					d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: id, Message: fmt.Sprintf("carry-over not offered: host %q %v", role.Host, err)})
				}
				carryText, carried = "", nil
			}
			if carryText != "" {
				values["{prompt}"] += "\n\n" + carryText
			}
		}
		// ATR-V0-141: a review role's prompt carries the bounded review
		// packet after any carry-over block; it is never expanded either.
		if reviewPacket {
			err := packetErr
			if err == nil {
				err = renderedArgvSafe(host.Argv, values)
			}
			if err != nil {
				d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: id, Message: fmt.Sprintf("review packet not offered: %v", err)})
				packet = ""
			}
			if packet != "" {
				values["{prompt}"] += "\n\n" + packet
			}
		}
		argv := make([]string, len(host.Argv))
		for i, s := range host.Argv {
			argv[i] = Render(s, values)
		}
		env := os.Environ()
		hostEnv := make([]string, 0, len(host.Env))
		for k, v := range host.Env {
			hostEnv = append(hostEnv, k+"="+Render(v, values))
		}
		sort.Strings(hostEnv)
		env = append(env, hostEnv...)
		// ATR-V0-105: only the dispatcher names a hand-off note; an inherited
		// or host-set one could hand this ticket another ticket's note.
		env = slices.DeleteFunc(env, func(kv string) bool { return strings.HasPrefix(kv, handoffNoteEnv+"=") })
		env = append(env, "CORVINT_DISPATCH_PROGRAM="+d.Program, "CORVINT_DISPATCH_ROLE="+a.Role, "CORVINT_DISPATCH_SLOT="+strconv.Itoa(a.Slot), "CORVINT_DISPATCH_TICKET="+a.Ticket, "CORVINT_DISPATCH_WORKER="+id)
		env = append(env, "CORVINT_DISPATCH_DRAIN_FILE="+DrainSignalPath(d.dir)) // ATR-V0-058
		if role.Cwd != nil {
			env = append(env, "CORVINT_DISPATCH_CWD="+cwd) // ATR-V0-344
		}
		// ATR-V0-125: only the dispatcher names the notice file.
		env = slices.DeleteFunc(env, func(kv string) bool { return strings.HasPrefix(kv, contextNoticeEnv+"=") })
		if contextFormat(role.UsageFormat) {
			env = append(env, "CORVINT_DISPATCH_CONTEXT_FILE="+ContextRecordPath(d.dir, id)) // ATR-V0-034
			if hostUsesNoticeFile(host) {
				env = append(env, contextNoticeEnv+"="+ContextNoticePath(d.dir, id)) // ATR-V0-125
			}
		}
		env = append(env, "CORVINT_DISPATCH_EVIDENCE_DIR="+EvidenceDir(d.dir, id)) // ATR-V0-104
		// DSG-V0-010: the session's tools run the build that launched it,
		// unless the host names one.
		build := SelfBuild()
		if _, set := host.Env[tasksBinEnv]; !set && len(d.HookCommand) > 0 {
			env = slices.DeleteFunc(env, func(kv string) bool { return strings.HasPrefix(kv, tasksBinEnv+"=") })
			env = append(env, tasksBinEnv+"="+d.HookCommand[0])
		}
		// ATR-V0-121: only the dispatcher names the pool hold record.
		env = slices.DeleteFunc(env, func(kv string) bool { return strings.HasPrefix(kv, poolHoldEnv+"=") })
		if hold != nil {
			env = append(env, poolHoldEnv+"="+PoolHoldPath(d.dir, id)) // ATR-V0-121
		}
		note := d.handoffNoteFor(obs, a.Ticket)
		if note != "" {
			env = append(env, handoffNoteEnv+"="+note) // ATR-V0-105
		}
		if run != nil {
			env = append(env, detachedEnv(run)...)
		}
		// ATR-V0-102/104: the evidence directory and any hook settings exist
		// before spawn; a failure here is proved before spawn.
		var pid int
		var identity string
		var exit <-chan exitStatus
		err := cwdErr
		if err == nil {
			err = os.MkdirAll(EvidenceDir(d.dir, id), 0o700)
		}
		if err == nil && hostUsesContextHook(host) {
			_, err = d.writeHookSettings(id)
		}
		if err == nil && len(carried) > 0 {
			err = writeCarryMarker(d.dir, id, a.Ticket, carried) // ATR-V0-113
		}
		if err == nil {
			// DSG-V0-010: the build record is advisory; one that cannot be
			// written reads UNKNOWN and never blocks the launch.
			_ = writeWorkerBuild(d.workerDir(id), id, build)
		}
		if err != nil {
			err = fmt.Errorf("before spawn: %w", err)
		} else {
			head := d.resolveLaunchHead(cwd) // ATR-V0-351: before the worker can commit
			pid, identity, exit, err = launch(argv, env, cwd, d.workerDir(id))
			if err == nil {
				d.saveLaunchHead(id, cwd, head)
			}
		}
		if err != nil {
			b := d.ledger.Backoff[a.Key]
			if b == nil {
				b = &BackoffState{Fingerprint: Fingerprint(obs, a.Key)}
				if h := d.ledger.Progress[a.Key]; h != nil {
					b.BaseFingerprint, b.ProgressDigest = baseFingerprint(obs, a.Key), h.Current
				}
				d.ledger.Backoff[a.Key] = b
			}
			b.CooldownUntil = now.Add(time.Duration(d.Config.TickSeconds) * time.Second * 10)
			// Only a failure proven before spawn resolves the intent; a
			// started but unidentified tree keeps it unresolved.
			var started *startedError
			if errors.As(err, &started) {
				d.uncertain = append(d.uncertain, started)
				// CAL-V0-156: a launch that may have started is charged,
				// and its usage is never observed.
				d.recordSession(a, id, UsageUnknown, now)
			}
			if ep != nil && started != nil {
				// An ambiguous spawn consumes the reservation and holds.
				ep.State, ep.Launch = InfraUnknown, ""
				d.emit(Event{Kind: "needs-owner", Ticket: a.Ticket, Worker: id, Message: fmt.Sprintf("%s is held UNKNOWN: its infrastructure retry may have started. Inspect it, then run `corvint-tasks dispatch unpark --program %s --config FILE --key %s`", d.keyText(a.Key), d.Program, a.Key), Detail: map[string]string{"kind": "infrastructure", "code": "INFRA_RETRY_UNKNOWN", "acceptanceRevision": ep.AcceptanceRevision}})
			} else if ep != nil {
				ep.State = InfraWaiting // proved before spawn; the identity is kept
			}
			if run != nil && started == nil {
				// Proved before spawn: the relaunch stays due.
				run.Phase, run.Successor = DetachedRelaunch, ""
				if err := d.saveDetached(run); err != nil {
					d.emit(Event{Kind: "alert", Ticket: a.Ticket, Message: fmt.Sprintf("relaunch for detached run %s failed before spawn and its marker could not be restored; it is not retried: %v", run.RunID, err)})
				}
			}
			if hold != nil && started == nil {
				// ATR-V0-122: proved before spawn, so the claim is returned
				// now; a spawn that may have started keeps it until the
				// ATR-V0-124 sweep.
				d.returnPoolHold(ctx, hold, a.Role, true)
			}
			release(started == nil)
			var failDetail map[string]string
			if role.Cwd != nil {
				failDetail = cwdDetail(map[string]string{}, cwd, cwdSource) // ATR-V0-343
				if cwdErr != nil {
					failDetail["code"] = CwdUnresolved
				}
			}
			d.emit(Event{Kind: "launch-failed", Ticket: a.Ticket, Role: a.Role, Worker: id, Message: fmt.Sprintf("could not launch %s on %s: %v", a.Role, d.keyText(a.Key), err), Detail: failDetail})
			continue
		}
		w := &Worker{ID: id, Role: a.Role, Host: role.Host, Slot: a.Slot, Key: a.Key, Ticket: a.Ticket, Pool: a.Pool, Member: a.Member, PID: pid, LeaderIdentity: identity, Members: []Proc{{PID: pid, Identity: identity}}, Started: now, LastActive: now, State: "RUNNING", Fingerprint: Fingerprint(obs, a.Key), Tier: a.Tier, Model: model, Effort: role.Effort, WorkStateUnknown: workStateUnknown(obs, a.Key)}
		if role.UsageFormat != "" {
			w.Usage = &WorkerUsage{Format: role.UsageFormat}
		}
		if h := d.ledger.Progress[a.Key]; h != nil {
			w.BaseFingerprint, w.ProgressDigest = baseFingerprint(obs, a.Key), h.Current
		}
		for _, p := range host.ActivityPaths {
			w.ActivityPaths = append(w.ActivityPaths, Render(p, values))
		}
		d.ledger.Workers = append(d.ledger.Workers, w)
		d.recordSession(a, id, UsageRunning, now)
		d.seedStall(obs, a.Key)
		d.exits[id] = exit
		if ep != nil {
			ep.State = InfraRunning
		}
		escalation := d.escalated(role, a, model)
		// Record the worker before anything else, so a crash cannot leave
		// an untracked tree.
		saveErr := d.ledger.save(d.dir)
		if saveErr != nil {
			d.emit(Event{Kind: "alert", Worker: id, Message: "ledger unwritable after launch: " + saveErr.Error()})
		}
		// An unsaved worker leaves its launch intent unresolved.
		release(saveErr == nil)
		if escalation != nil {
			escalation.Worker = id
			d.emit(*escalation)
		}
		detail := map[string]string{"host": role.Host, "pid": strconv.Itoa(pid), "slot": strconv.Itoa(a.Slot), "state": a.State}
		if a.NextStage != "" {
			detail["nextStage"] = a.NextStage
		}
		if model != "" {
			detail["model"], detail["tier"] = model, strconv.Itoa(a.Tier)
		}
		carryDetail(detail, carried, carryText) // ATR-V0-114
		reviewPacketDetail(detail, packet)      // ATR-V0-141
		if role.Effort != "" {
			detail["effort"] = role.Effort
		}
		if n := a.OperatorNote; n != nil {
			detail["operatorNote"], detail["operatorNoteRevision"] = n.State, n.Revision
		}
		if run != nil {
			detail["detachedRun"], detail["runExit"] = run.RunID, strconv.Itoa(*run.ExitStatus)
		}
		// ATR-V0-127: how the session's context warnings are delivered and
		// whether a salvaged note was offered.
		if mode, _, ok := ContextDelivery(d.dir, id, role.UsageFormat, &host); ok {
			detail["contextDelivery"] = mode
		}
		if role.Cwd != nil {
			cwdDetail(detail, cwd, cwdSource) // ATR-V0-343
		}
		detail["handoffNote"] = "NONE"
		if note != "" {
			detail["handoffNote"] = note
		}
		d.emit(Event{Kind: "launched", Ticket: a.Ticket, Role: a.Role, Worker: id, Message: fmt.Sprintf("launched %s slot %d on %s as %s (%s, pid %d)", a.Role, a.Slot, d.keyText(a.Key), id, role.Host, pid), Detail: detail})
	}
}

// escalated records the tier a laddered role launched at and returns the
// CAL-V0-058 escalated event when it rises.
func (d *Dispatcher) escalated(r *Role, a Assignment, model string) *Event {
	if len(r.Escalate) == 0 {
		return nil
	}
	e := d.ledger.Escalation[a.Key]
	from := 0
	if e != nil {
		from = e.Tiers[r.Name]
	}
	if a.Tier == from {
		return nil
	}
	if e == nil {
		e = &EscalationState{}
		if d.ledger.Escalation == nil {
			d.ledger.Escalation = map[string]*EscalationState{}
		}
		d.ledger.Escalation[a.Key] = e
	}
	if e.Tiers == nil {
		e.Tiers = map[string]int{}
	}
	if a.Tier == 0 {
		delete(e.Tiers, r.Name)
	} else {
		e.Tiers[r.Name] = a.Tier
	}
	if a.Tier < from {
		if e.Streak == 0 && len(e.Tiers) == 0 {
			delete(d.ledger.Escalation, a.Key)
		}
		return nil
	}
	t := r.Escalate[a.Tier-1]
	detail := map[string]string{"fromTier": strconv.Itoa(from), "toTier": strconv.Itoa(a.Tier), "fromModel": r.ModelAt(from), "model": model, "streak": strconv.Itoa(e.Streak)}
	if t.Notes != "" {
		detail["notes"] = t.Notes
	}
	return &Event{Kind: "escalated", Ticket: a.Ticket, Role: r.Name, Message: fmt.Sprintf("escalated %s on %s from %s to %s after %d consecutive session(s) without progress", r.Name, d.keyText(a.Key), r.ModelAt(from), model, e.Streak), Detail: detail}
}

// loopEscalations raises the CAL-V0-102 typed blocked escalation once per
// LOOP_DETECTED episode (ticket, signal, acceptance revision and newest
// counted generation): when the dispatcher first observes a hold, including
// in its baseline observation, a new episode, or one still pending. The
// persisted Seen state keeps a restart from raising an episode twice. It
// skips the append when the readable log already holds the episode's
// event, so a failure after the bytes landed, or a crash before the ledger
// save, raises no duplicate. An episode whose event cannot be appended
// stays pending, so the next tick or a restart raises it again
// (CAL-V0-103). The native escalation writer
// needs the worker's live claim, so this is a dispatcher event only.
func (d *Dispatcher) loopEscalations(obs *Observation, old, now *Seen) {
	for _, id := range slices.Sorted(maps.Keys(now.Loops)) {
		h := now.Loops[id]
		if old != nil {
			if was, ok := old.Loops[id]; ok && !was.Pending && sameLoopEpisode(was, h) {
				continue
			}
		}
		if loopEventRecorded(d.dir, id, h) {
			continue
		}
		gens := strings.Join(h.Generations, ",")
		detail := map[string]string{"kind": "blocked", "code": "LOOP_DETECTED", "signal": h.Signal, "acceptanceRevision": h.AcceptanceRevision, "generations": gens}
		judged := ""
		if obs.ProgressSignal != "" && h.Signal == "NO_PROGRESS" {
			// ATR-V0-083: the hold names the declared signal that judged it.
			detail["progressSignal"] = obs.ProgressSignal
			judged = " under progress signal " + obs.ProgressSignal
		}
		if err := d.record(Event{Kind: "needs-owner", Ticket: id, Message: fmt.Sprintf("%s is held LOOP_DETECTED (%s%s) over generations %s; only the owner can acknowledge it with `corvint-tasks ticket reopen` (CAL-V0-103)", d.local(obs, id), h.Signal, judged, gens), Detail: detail}); err != nil {
			h.Pending = true
			now.Loops[id] = h
		}
	}
}

// sameLoopEpisode reports whether two recorded holds are one episode.
func sameLoopEpisode(a, b LoopHold) bool {
	return a.Signal == b.Signal && a.AcceptanceRevision == b.AcceptanceRevision && len(a.Generations) > 0 && len(b.Generations) > 0 && a.Generations[len(a.Generations)-1] == b.Generations[len(b.Generations)-1]
}

// diff emits state, claim, release and lane changes against the previous
// observation. The first observation only records a baseline.
func (d *Dispatcher) diff(obs *Observation) {
	now := &Seen{Tickets: map[string]string{}, Claims: map[string]string{}, Lanes: map[string]string{}}
	stale := WorkStateStale(d.Config, d.ledger.WorkState, d.Now())
	for _, t := range obs.Tickets {
		state, plan, reason := t.State, t.Plan, t.PlanReason
		// CAL-V0-276: within the window a failed read leaves the last known
		// work state, and the plan it held, in the baseline, so one slow
		// read emits no state event.
		if stale && state == StateUnknown && d.ledger.Seen != nil {
			if was := strings.Split(d.ledger.Seen.Tickets[t.ID], "|"); len(was) == 4 {
				state, plan, reason = was[1], was[2], was[3]
			}
		}
		now.Tickets[t.ID] = strings.Join([]string{t.Status, state, plan, reason}, "|")
		if len(t.EscalationPending) > 0 {
			if now.Escalations == nil {
				now.Escalations = map[string][]string{}
			}
			now.Escalations[t.ID] = append([]string(nil), t.EscalationPending...)
		}
		switch {
		case t.EscalationUnknown:
			now.RequestsUnknown = append(now.RequestsUnknown, t.ID)
		case len(t.OpenRequests) > 0:
			if now.Requests == nil {
				now.Requests = map[string][]OpenRequest{}
			}
			now.Requests[t.ID] = append([]OpenRequest(nil), t.OpenRequests...)
		}
		if o := t.Obligations; o != nil {
			if now.Obligations == nil {
				now.Obligations = map[string]string{}
			}
			now.Obligations[t.ID] = fmt.Sprintf("%d/%d", o.Witnessed, o.Total)
		}
		if t.Loop != nil && len(t.Loop.Generations) > 0 {
			if now.Loops == nil {
				now.Loops = map[string]LoopHold{}
			}
			now.Loops[t.ID] = LoopHold{Signal: t.Loop.Signal, AcceptanceRevision: t.Loop.AcceptanceRevision, Generations: append([]string(nil), t.Loop.Generations...)}
		}
	}
	sort.Strings(now.RequestsUnknown)
	for _, a := range obs.Attempts {
		if a.Live {
			now.Claims[a.ID] = strings.Join([]string{a.Ticket, a.Holder, a.Phase, a.Stage}, "|")
		}
	}
	for _, m := range obs.Members {
		now.Lanes[laneKey(m.Pool, m.Member)] = m.State
	}
	old := d.ledger.Seen
	d.ledger.Seen = now
	d.loopEscalations(obs, old, now)
	if old == nil {
		return
	}
	for _, id := range sortedKeys(now.Tickets) {
		was, ok := old.Tickets[id]
		if !ok || was == now.Tickets[id] {
			continue
		}
		o, n := strings.Split(was, "|"), strings.Split(now.Tickets[id], "|")
		if len(o) == 4 && len(n) == 4 && o[0] == n[0] && terminalStatus(n[0]) && o[2] == n[2] && o[3] == n[3] {
			continue // CAL-V0-280: only a terminal ticket's work state changed
		}
		d.emit(Event{Kind: "state", Ticket: id, Message: fmt.Sprintf("%s changed: status %s→%s, work state %s→%s, plan %s→%s", d.local(obs, id), o[0], n[0], o[1], n[1], planText(o[2], o[3]), planText(n[2], n[3]))})
		if n[3] == "RETRY_EXHAUSTED" && o[3] != n[3] {
			d.emit(Event{Kind: "needs-owner", Ticket: id, Message: fmt.Sprintf("%s exhausted its retries; only the owner can re-admit it with `corvint-tasks ticket reopen` (CAL-V0-043)", d.local(obs, id))})
		}
		if n[3] == PlanReasonMirrorsDecided && o[3] != n[3] {
			d.emit(Event{Kind: "needs-owner", Ticket: id, Message: fmt.Sprintf("%s has only obligation items decided on their owner tickets; it is not dispatched and is offered to the owner for completion (TOL-V0-123)", d.local(obs, id))})
		}
	}
	for _, id := range sortedKeys(now.Claims) {
		if _, ok := old.Claims[id]; !ok {
			c := strings.Split(now.Claims[id], "|")
			// CAL-V0-096: the claim names its pool member; both are empty
			// for an attempt without a pool allocation.
			pool, member, on := "", "", ""
			for _, a := range obs.Attempts {
				if a.ID == id && a.Member != "" {
					pool, member, on = a.Pool, a.Member, fmt.Sprintf(" on %s/%s", a.Pool, a.Member)
				}
			}
			d.emit(Event{Kind: "claim", Ticket: c[0], Message: fmt.Sprintf("%s claimed %s for %s%s (attempt %s)", c[1], d.local(obs, c[0]), orText(c[3], "work"), on, id), Detail: map[string]string{"attempt": id, "holder": c[1], "phase": c[2], "pool": pool, "member": member}})
		}
	}
	for _, id := range sortedKeys(old.Claims) {
		if _, ok := now.Claims[id]; !ok {
			c := strings.Split(old.Claims[id], "|")
			phase := "ended"
			for _, a := range obs.Attempts {
				if a.ID == id {
					phase = a.Phase
				}
			}
			d.emit(Event{Kind: "release", Ticket: c[0], Message: fmt.Sprintf("%s's attempt %s on %s ended as %s", orText(c[1], "the supervisor"), id, d.local(obs, c[0]), phase), Detail: map[string]string{"attempt": id, "holder": c[1], "phase": phase}})
		}
	}
	for _, k := range sortedKeys(now.Lanes) {
		if was, ok := old.Lanes[k]; ok && was != now.Lanes[k] {
			d.emit(Event{Kind: "lane", Message: fmt.Sprintf("pool member %s changed from %s to %s", strings.TrimPrefix(k, "lane:"), was, now.Lanes[k])})
		}
	}
}

func planText(state, reason string) string {
	if state == "" {
		return "none"
	}
	if reason == "" {
		return state
	}
	return state + " (" + reason + ")"
}

func orText(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (d *Dispatcher) emit(e Event) { _ = d.record(e) }

// record is emit returning the event-log append failure, for a caller that
// must retry an event it cannot lose.
func (d *Dispatcher) record(e Event) error {
	d.ledger.EventSeq++
	e.Profile, e.Seq, e.Program = EventProfile, d.ledger.EventSeq, d.Program
	e.At = d.Now().UTC().Format(time.RFC3339)
	err := appendEvent(d.dir, e)
	if err != nil && d.Out != nil {
		fmt.Fprintf(d.Out, "%s alert event log unwritable: %v\n", e.At, err)
	}
	if d.Out != nil {
		fmt.Fprintf(d.Out, "%s %s %s\n", e.At, e.Kind, e.Message)
	}
	return err
}

// pressureBudget samples host pressure once and advances its hysteresis
// (CAL-V0-068). It returns a nil budget when pressure is not configured and
// false, so nothing launches, when the context ended during sampling or the
// budget cannot be built (with an alert).
func (d *Dispatcher) pressureBudget(ctx context.Context, obs *Observation) (*PressureBudget, bool) {
	c, rec := d.Config.Pressure, d.ledger.Pressure
	if c == nil || rec == nil {
		return nil, true
	}
	sampler := d.pressureSampler
	if sampler == nil {
		sampler = samplePressure
	}
	goos := d.goos
	if goos == "" {
		goos = runtime.GOOS
	}
	// CAL-V0-125: only the selected signals are sampled. With cpu selected,
	// the previous sample of this run supplies the tick baseline; Open
	// clears it and a reload that adds pressure starts without one, so the
	// first sample after either has UNKNOWN CPU utilization. Without cpu,
	// no tick or utilization field reaches the ledger.
	want := c.want(goos)
	sample := sampler(ctx, d.Now().UTC(), want)
	prev, gap := rec.Sample, d.pressureGap
	if gap {
		prev, d.pressureGap = PressureSample{}, false
	}
	if want.cpu {
		sample = withCPUUtilization(prev, sample)
	} else {
		sample.CPUBusyTicks, sample.CPUTotalTicks, sample.CPUTicksKnown = 0, 0, false
		sample.CPUUtilization, sample.CPUUtilizationKnown = 0, false
	}
	sample = boundPressureSample(sample)
	if ctx.Err() != nil {
		return nil, false
	}
	next, err := stepPressure(*c, goos, rec.State, sample)
	if err != nil {
		// Unreachable for a validated config and a ledger normalized by
		// Open; keep the level as an UNKNOWN sample would.
		next = PressureState{Level: rec.State.Level, PendingLevel: rec.State.Level, Unknown: true}
	}
	rec.Sample, rec.State = sample, next
	running := make([]Assignment, 0, len(d.ledger.Workers))
	for _, w := range d.ledger.Workers {
		a := Assignment{Role: w.Role, Key: w.Key, Ticket: w.Ticket}
		if w.Ticket != "" {
			a.Local = d.local(obs, w.Ticket)
		}
		running = append(running, a)
	}
	// ATR-V0-181: the samples skipped during a gap never advanced the
	// dwell, so from the first sample after one until the pending
	// transition resolves the budget uses a raised pending level rather
	// than launching while it dwells. The recorded state keeps the ordinary
	// hysteresis.
	if gap {
		d.pressureHold = true
	}
	admit := next
	if d.pressureHold {
		if next.PendingLevel > next.Level {
			admit.Level = next.PendingLevel
		} else {
			d.pressureHold = false
		}
	}
	// With cpu selected, the first sample after a gap has no interval, so
	// its utilization is UNKNOWN: it admits nothing that pressure governs,
	// and the next tick samples a fresh interval instead of deferring.
	baseline := want.cpu && gap && !sample.CPUUtilizationKnown
	d.pressureBaseline = baseline
	b, err := NewPressureBudget(c, admit, running, d.Config.Pinned)
	if err != nil {
		d.emit(Event{Kind: "alert", Message: "pressure budget unavailable; launches withheld: " + err.Error()})
		return nil, false
	}
	if baseline {
		b.enabled, b.cap = true, 0
	}
	return b, true
}

// recordHeld stores the held set and emits one throttled event when the
// level, sample knowledge or held keys changed since the previous tick.
func (d *Dispatcher) recordHeld(obs *Observation, held []Assignment) {
	rec := d.ledger.Pressure
	if rec == nil {
		return
	}
	next := make([]HeldLaunch, 0, len(held))
	for _, a := range held {
		if len(next) == maxPressureHeld {
			break
		}
		next = append(next, HeldLaunch{Role: a.Role, Key: a.Key, Ticket: a.Ticket})
	}
	prevKeys, nextKeys := make([]string, 0, len(rec.Held)), make([]string, 0, len(next))
	for _, h := range rec.Held {
		prevKeys = append(prevKeys, h.Key)
	}
	for _, h := range next {
		nextKeys = append(nextKeys, h.Key)
	}
	sort.Strings(prevKeys)
	sort.Strings(nextKeys)
	changed := strings.Join(prevKeys, "\n") != strings.Join(nextKeys, "\n")
	st := rec.State
	rec.Held = next
	if !changed && st.Level == d.pressureEmitted.Level && st.Unknown == d.pressureEmitted.Unknown {
		return
	}
	d.pressureEmitted = st
	detail := map[string]string{"level": strconv.Itoa(st.Level), "sample": "OBSERVED", "held": strconv.Itoa(len(held))}
	if st.Unknown {
		detail["sample"] = StateUnknown
	}
	load, swap := StateUnknown, StateUnknown
	if x, ok := rec.Sample.LoadPerCPU(); ok {
		load = strconv.FormatFloat(x, 'f', 3, 64)
	}
	if x, ok := rec.Sample.SwapFraction(); ok {
		swap = strconv.FormatFloat(x, 'f', 3, 64)
	}
	detail["loadPerCpu"], detail["swapFraction"] = load, swap
	memory, memoryText := StateUnknown, "memory "+StateUnknown
	if x, ok := rec.Sample.MemoryPressure(); ok {
		memory = strconv.Itoa(x)
		memoryText = "memory pressure level " + memory
	} else if swap != StateUnknown {
		memoryText = "swap " + swap
	}
	detail["memoryPressureLevel"] = memory
	detail["cpuUtilization"] = StateUnknown
	if x, ok := rec.Sample.CPUUtilizationFraction(); ok {
		detail["cpuUtilization"] = strconv.FormatFloat(x, 'f', 3, 64)
	}
	detail["reason"] = PressureReasonText(st)
	names := make([]string, 0, 10)
	for _, h := range next {
		if len(names) == 10 {
			break
		}
		if h.Ticket != "" {
			names = append(names, d.local(obs, h.Ticket))
		} else {
			names = append(names, d.keyText(h.Key))
		}
	}
	list := "none"
	if len(names) > 0 {
		list = strings.Join(names, ", ")
		if len(held) > len(names) {
			list += fmt.Sprintf(" and %d more", len(held)-len(names))
		}
	}
	msg := fmt.Sprintf("host pressure level %d (reason %s; load per CPU %s, %s); holding %d new launch(es): %s", st.Level, detail["reason"], load, memoryText, len(held), list)
	if st.Unknown {
		msg = fmt.Sprintf("host pressure sample UNKNOWN; keeping level %d (reason %s; load per CPU %s, %s); holding %d new launch(es): %s", st.Level, detail["reason"], load, memoryText, len(held), list)
	}
	d.emit(Event{Kind: "throttled", Message: msg, Detail: detail})
}

func (d *Dispatcher) workerDir(id string) string { return filepath.Join(d.dir, "workers", id) }

func (d *Dispatcher) role(name string) *Role { return d.Config.roleNamed(name) }

func (c *Config) roleNamed(name string) *Role {
	for i := range c.Roles {
		if c.Roles[i].Name == name {
			return &c.Roles[i]
		}
	}
	return nil
}

// launchConfig is the configuration worker w launched under (CAL-V0-127).
func (d *Dispatcher) launchConfig(w *Worker) *Config {
	if c := d.launchedUnder[w.ID]; c != nil {
		return c
	}
	return d.Config
}

// busy reports a key with a worker in the ledger.
func (d *Dispatcher) busy(key string) bool {
	for _, w := range d.ledger.Workers {
		if w.Key == key {
			return true
		}
	}
	return false
}

func (d *Dispatcher) worker(id string) *Worker {
	for _, w := range d.ledger.Workers {
		if w.ID == id {
			return w
		}
	}
	return nil
}

func (d *Dispatcher) remove(id string) {
	kept := d.ledger.Workers[:0]
	for _, w := range d.ledger.Workers {
		if w.ID != id {
			kept = append(kept, w)
		}
	}
	d.ledger.Workers = kept
	delete(d.exits, id)
	delete(d.codes, id)
	delete(d.everHeld, id)
	delete(d.handedOff, id)
}

func (d *Dispatcher) keyText(key string) string {
	if strings.HasPrefix(key, "lane:") {
		return "pool member " + strings.TrimPrefix(key, "lane:")
	}
	if i := strings.LastIndexByte(key, ':'); i >= 0 {
		return key[i+1:]
	}
	return key
}

func (d *Dispatcher) local(obs *Observation, id string) string {
	for _, t := range obs.Tickets {
		if t.ID == id {
			return t.Local
		}
	}
	return d.keyText(id)
}

// LaunchControl links a controlled dispatcher to its service control
// (SERVICE500-003).
type LaunchControl interface {
	// Admit takes the fence and admits one new effect, or refuses it. The
	// intent names the effect: a worker id, or a pool sweep request id. On
	// admission the dispatcher calls release once; recorded reports that
	// the effect's outcome is durable: it is in the saved ledger, or
	// nothing started.
	Admit(intent string) (release func(recorded bool), err error)
	// Boundary runs between ticks, never during one. recorded reports that
	// the tick's final ledger save succeeded, so every launch this
	// dispatcher made is in the saved ledger; settled that the tick also
	// completed with no recorded worker and no pending or in-flight pool
	// sweep. A true result ends Run with ErrSettled.
	Boundary(recorded, settled bool) bool
}

// ErrSettled ends Run when the controlled dispatcher's control settled it.
var ErrSettled = errors.New("dispatcher settled by its service control")

// OpenControlled is Open for a dispatcher whose every new worker launch and
// pool sweep native call first passes control. It takes the lifetime lock
// before the fence; supervision, healing and accounting of recorded workers
// are never fenced.
func OpenControlled(program string, c *Config, q Queue, out io.Writer, control LaunchControl) (*Dispatcher, error) {
	if control == nil {
		return nil, errors.New("a controlled dispatcher needs a launch control")
	}
	d, err := Open(program, c, q, out)
	if err != nil {
		return nil, err
	}
	d.control = control
	return d, nil
}

// OwnerProcess is OwnerState with the recorded holder's PID, which is 0
// unless the state is RUNNING.
func OwnerProcess(dir string) (string, int) {
	raw, err := readBounded(filepath.Join(dir, "lock"), 4096)
	if errors.Is(err, fs.ErrNotExist) {
		return "NOT_RUNNING", 0
	}
	if err != nil {
		return "UNKNOWN", 0
	}
	pid, id, ok := strings.Cut(strings.TrimSpace(string(raw)), " ")
	if !ok {
		return "NOT_RUNNING", 0
	}
	n, err := strconv.Atoi(pid)
	if err != nil {
		return "UNKNOWN", 0
	}
	live, err := detachedProcessIdentity(n)
	if err != nil {
		return "UNKNOWN", 0
	}
	switch supervisor.CompareIdentity(live, id) { // PSR-V0-227
	case supervisor.IdentitySame:
		return "RUNNING", n
	case supervisor.IdentityUnknown:
		return "UNKNOWN", 0
	}
	return "NOT_RUNNING", 0
}

// Settled reports a saved ledger with no recorded worker and no pending
// pool sweep: what a draining service needs before it may stop.
func Settled(l *Ledger) bool {
	if len(l.Workers) != 0 {
		return false
	}
	for _, r := range l.PoolSweeps {
		if sweepPending(r.Phase) {
			return false
		}
	}
	return true
}

// Records reports whether a saved ledger records the effect an admission
// intent named: a worker or a pool sweep request.
func Records(l *Ledger, intent string) bool {
	for _, w := range l.Workers {
		if w.ID == intent {
			return true
		}
	}
	for _, r := range l.PoolSweeps {
		if r.RequestID == intent {
			return true
		}
	}
	return false
}

// admit passes a controlled dispatcher's fence for intent. A cancel that
// arrives while the fence is awaited starts nothing. An uncontrolled
// dispatcher always admits.
func (d *Dispatcher) admit(ctx context.Context, intent string) (func(bool), bool) {
	if d.control == nil {
		return func(bool) {}, true
	}
	release, err := d.control.Admit(intent)
	if err != nil {
		return nil, false
	}
	if ctx.Err() != nil {
		release(true)
		return nil, false
	}
	return release, true
}

// boundary reports a completed tick to a controlled dispatcher's control.
func (d *Dispatcher) boundary(ctx context.Context, last error) bool {
	if d.control == nil || ctx.Err() != nil {
		return false
	}
	recorded := d.tickSaved && len(d.uncertain) == 0
	settled := recorded && last == nil && d.sweepJob == nil && Settled(d.ledger)
	return d.control.Boundary(recorded, settled)
}

// startedError is a launch that started a process but failed after it, so
// its effect is uncertain rather than proven absent. It keeps the owned pid
// and wait channel.
type startedError struct {
	pid  int
	exit <-chan exitStatus
	err  error
}

func (e *startedError) Error() string { return e.err.Error() }
func (e *startedError) Unwrap() error { return e.err }
