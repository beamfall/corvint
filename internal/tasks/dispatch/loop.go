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
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

	dir    string
	nonce  string
	ledger *Ledger
	exits  map[string]<-chan int
	codes  map[string]int
	lock   *os.File
	// Tests place cancellation exactly across the checked atomic write.
	progressSave  func(*Ledger, string) error
	poolSweepSave func(*Ledger, string) error
	sweepJob      *poolSweepJob
	sweepTried    map[string]bool
	sweepNext     time.Time
	closed        bool
	closeErr      error
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
	if err := os.MkdirAll(filepath.Join(dir, "requests"), 0o700); err != nil {
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
	d := &Dispatcher{Program: program, Config: c, Queue: q, Out: out, Now: time.Now, dir: dir, nonce: hex.EncodeToString(nonce[:]), ledger: l, exits: map[string]<-chan int{}, codes: map[string]int{}, lock: lock}
	d.emit(Event{Kind: "started", Message: fmt.Sprintf("dispatcher started for program %s with %d recorded worker(s)", program, len(l.Workers))})
	for _, w := range l.Workers {
		msg := fmt.Sprintf("adopted %s worker %s (pid %d) on %s; its exit code will not be observed", w.Role, w.ID, w.PID, d.keyText(w.Key))
		if id, err := supervisor.ProcessIdentity(w.PID); err == nil && id != w.LeaderIdentity {
			msg = fmt.Sprintf("adopted %s worker %s on %s; its leader (pid %d) has ended, so any remaining processes are stopped and the run is accounted", w.Role, w.ID, d.keyText(w.Key), w.PID)
		}
		d.emit(Event{Kind: "adopted", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: msg})
	}
	if err := d.ledger.save(dir); err != nil {
		_ = d.lock.Close()
		return nil, err
	}
	return d, nil
}

// Close records the stop and releases the lock. Workers keep running and
// are adopted by the next dispatcher.
func (d *Dispatcher) Close() error {
	if d.closed {
		return d.closeErr
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
	raw, err := readBounded(filepath.Join(dir, "lock"), 4096)
	if errors.Is(err, fs.ErrNotExist) {
		return "NOT_RUNNING"
	}
	if err != nil {
		return "UNKNOWN"
	}
	pid, id, ok := strings.Cut(strings.TrimSpace(string(raw)), " ")
	if !ok {
		return "NOT_RUNNING"
	}
	n, err := strconv.Atoi(pid)
	if err != nil {
		return "UNKNOWN"
	}
	live, err := supervisor.ProcessIdentity(n)
	switch {
	case err != nil:
		return "UNKNOWN"
	case live != "" && live == id:
		return "RUNNING"
	}
	return "NOT_RUNNING"
}

// Running is the number of supervised workers.
func (d *Dispatcher) Running() int { return len(d.ledger.Workers) }

// LastEvent is the last event sequence number.
func (d *Dispatcher) LastEvent() uint64 { return d.ledger.EventSeq }

// Run ticks until ctx ends or ticks reach the bound (0 is unbounded). A
// failed tick is an alert, not an exit, so the dispatcher keeps supervising.
func (d *Dispatcher) Run(ctx context.Context, ticks int) (result error) {
	defer func() { result = errors.Join(result, d.stopPoolSweep()) }()
	var last error
	for n := 0; ticks == 0 || n < ticks; n++ {
		if ctx.Err() != nil {
			return nil
		}
		if last = d.Tick(ctx); last != nil && ctx.Err() == nil {
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
func (d *Dispatcher) Tick(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Admission publishes a new ledger only after its checked write. Resolve
	// this receiver at return so an old pointer cannot overwrite that commit.
	defer func() { _ = d.ledger.save(d.dir) }()
	obs, err := d.observe(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
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
	granted, pending, unparked, err := d.admitProgress(ctx, obs, ended)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, key := range unparked {
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
		d.launchRoster(obs)
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
	alerts := ReadStates(ctx, d.Config, obs.Tickets)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	for _, a := range alerts {
		d.emit(Event{Kind: "alert", Message: a})
	}
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

// supervise refreshes every worker's tree and activity, kills idle,
// over-wall and orphaned trees, and returns the workers whose tree is empty.
func (d *Dispatcher) supervise() []*Worker {
	procs, err := observeProcs()
	if err != nil {
		d.emit(Event{Kind: "alert", Message: "process table unavailable: " + err.Error()})
		return nil
	}
	now := d.Now()
	var ended []*Worker
	for _, w := range d.ledger.Workers {
		select {
		case code := <-d.exits[w.ID]:
			d.codes[w.ID] = code
		default:
		}
		if err := refreshTree(w, procs); err != nil {
			d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not observe the process tree of %s; keeping it as recorded: %v", w.ID, err)})
			continue
		}
		role := d.role(w.Role)
		host := d.Config.Hosts[w.Host]
		if d.active(w, procs, host) {
			w.LastActive = now
		}
		reason := ""
		switch {
		case len(w.Members) == 0:
			ended = append(ended, w)
			continue
		case w.State == "KILLING":
			reason = w.KillReason
		case !leaderAlive(w):
			reason = "ORPHANED"
		case role != nil && now.Sub(w.Started) > time.Duration(role.WallSeconds)*time.Second:
			reason = "WALL"
		case role != nil && now.Sub(w.LastActive) > time.Duration(role.IdleSeconds)*time.Second:
			reason = "IDLE"
		}
		if reason == "" {
			continue
		}
		if w.State != "KILLING" {
			w.State, w.KillReason = "KILLING", reason
			d.emit(Event{Kind: "killing", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("stopping %s worker %s on %s: %s", w.Role, w.ID, d.keyText(w.Key), killText[reason]), Detail: map[string]string{"reason": reason, "processes": strconv.Itoa(len(w.Members))}})
		}
		switch gone, err := killTree(w, time.Duration(d.Config.KillGraceSeconds)*time.Second); {
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
	"IDLE":     "no session activity and no running tool process within the idle timeout",
	"WALL":     "the wall-clock cap was reached",
	"ORPHANED": "the worker exited but left processes behind",
}

// active reports output growth, an activity path advancing, or a running
// tool process since the previous tick.
func (d *Dispatcher) active(w *Worker, procs map[int]proc, h Host) bool {
	busy := false
	var size int64
	for _, name := range []string{"stdout.log", "stderr.log"} {
		if st, err := os.Stat(filepath.Join(d.workerDir(w.ID), name)); err == nil {
			size += st.Size()
		}
	}
	if size != w.LogBytes {
		w.LogBytes, busy = size, true
	}
	for _, p := range w.ActivityPaths {
		if st, err := os.Stat(p); err == nil && st.ModTime().After(w.ActivityMtime) {
			if !w.ActivityMtime.IsZero() {
				busy = true
			}
			w.ActivityMtime = st.ModTime()
		}
	}
	return busy || busyChild(w, procs, h.IdleIgnore)
}

// heal hands off the live attempts of ended workers and reaps expired
// leases through the fenced lease transactions. It reports whether it
// attempted any store write.
func (d *Dispatcher) heal(ctx context.Context, obs *Observation, ended []*Worker) bool {
	wrote := false
	done := map[string]bool{}
	if d.Config.Heal.Handoff {
		for _, w := range ended {
			for _, a := range obs.Attempts {
				if !a.Live || a.Holder != w.ID {
					continue
				}
				done[a.ID], wrote = true, true
				evidence := ""
				if a.Candidate == "" {
					evidence = "dispatch:" + w.ID
				}
				err := d.Queue.Release(ctx, a, evidence, requestID("release", w.ID, a.ID, a.Generation))
				detail := map[string]string{"attempt": a.ID, "generation": a.Generation, "phase": a.Phase}
				if err != nil {
					detail["error"] = err.Error()
					d.emit(Event{Kind: "handoff-refused", Ticket: a.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("could not hand off %s attempt %s after %s ended: %v", d.local(obs, a.Ticket), a.ID, w.ID, err), Detail: detail})
					d.emit(Event{Kind: "needs-owner", Ticket: a.Ticket, Worker: w.ID, Message: fmt.Sprintf("%s still holds a live %s attempt that the dispatcher could not hand off; inspect it with `corvint-tasks attempt show %s`", d.local(obs, a.Ticket), a.Phase, a.ID), Detail: detail})
					continue
				}
				d.emit(Event{Kind: "handoff", Ticket: a.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("handed off %s (attempt %s) after %s ended", d.local(obs, a.Ticket), a.ID, w.ID), Detail: detail})
			}
		}
	}
	if d.Config.Heal.Reap {
		now := d.Now()
		for _, a := range obs.Attempts {
			if !a.Live || done[a.ID] || a.LeaseExpires.IsZero() || a.LeaseExpires.After(now) || d.worker(a.Holder) != nil {
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
		if c, ok := d.codes[w.ID]; ok {
			code = strconv.Itoa(c)
		}
		fp := Fingerprint(obs, w.Key)
		// An unreadable work state is not evidence of progress.
		unknown := stateUnknown(obs, w.Key)
		if unknown {
			fp = w.Fingerprint
		}
		progress := granted[w.ID] || fp != w.Fingerprint
		summary := Summary(d.workerDir(w.ID))
		msg := fmt.Sprintf("%s worker %s finished on %s (exit %s, %s)", w.Role, w.ID, d.keyText(w.Key), code, map[bool]string{true: "progress recorded", false: "no progress"}[progress])
		if summary != "" {
			msg += ": " + summary
		}
		d.emit(Event{Kind: "finished", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: msg, Detail: map[string]string{"exitCode": code, "progress": strconv.FormatBool(progress), "killReason": w.KillReason, "runSeconds": strconv.Itoa(int(now.Sub(w.Started).Seconds()))}})
		d.remove(w.ID)
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
		if b.NoProgress >= d.Config.Backoff.ParkAfter {
			b.Parked = true
			d.emit(Event{Kind: "parked", Ticket: w.Ticket, Message: fmt.Sprintf("parked %s after %d run(s) without progress", d.keyText(w.Key), b.NoProgress)})
			d.emit(Event{Kind: "needs-owner", Ticket: w.Ticket, Message: fmt.Sprintf("%s is parked: %d dispatcher run(s) made no progress. It resumes when its state changes, or run `corvint-tasks dispatch unpark --program %s --config FILE --key %s`", d.keyText(w.Key), b.NoProgress, d.Program, w.Key)})
			continue
		}
		b.CooldownUntil = now.Add(time.Duration(d.Config.Backoff.CooldownSeconds) * time.Second)
		d.emit(Event{Kind: "cooldown", Ticket: w.Ticket, Message: fmt.Sprintf("%s cools down until %s after a run without progress (%d of %d before parking)", d.keyText(w.Key), b.CooldownUntil.UTC().Format(time.RFC3339), b.NoProgress, d.Config.Backoff.ParkAfter)})
	}
}

// UnparkRequest is one queued operator request file.
type UnparkRequest struct {
	Unpark string `json:"unpark"`
}

// unpark releases parked keys whose state changed and consumes operator
// unpark requests.
func (d *Dispatcher) unpark(obs *Observation) {
	keys := make([]string, 0, len(d.ledger.Backoff))
	for k := range d.ledger.Backoff {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if b := d.ledger.Backoff[k]; b.Parked && !stateUnknown(obs, k) && Fingerprint(obs, k) != b.Fingerprint {
			delete(d.ledger.Backoff, k)
			d.emit(Event{Kind: "unparked", Ticket: ticketOf(k), Message: fmt.Sprintf("unparked %s because its state changed", d.keyText(k))})
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
		} else if _, ok := d.ledger.Backoff[r.Unpark]; ok {
			delete(d.ledger.Backoff, r.Unpark)
			d.emit(Event{Kind: "unparked", Ticket: ticketOf(r.Unpark), Message: fmt.Sprintf("unparked %s at the operator's request", d.keyText(r.Unpark))})
		}
		os.Remove(path)
	}
}

// stateUnknown reports a ticket key whose program work state is UNKNOWN.
func stateUnknown(obs *Observation, key string) bool {
	for _, t := range obs.Tickets {
		if t.ID == key {
			return t.State == StateUnknown
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
func (d *Dispatcher) launchRoster(obs *Observation) {
	now := d.Now()
	busy := make([]Busy, 0, len(d.ledger.Workers))
	for _, w := range d.ledger.Workers {
		busy = append(busy, Busy{Role: w.Role, Key: w.Key, Slot: w.Slot})
	}
	skip := map[string]bool{}
	for k, b := range d.ledger.Backoff {
		skip[k] = b.Parked || b.CooldownUntil.After(now)
	}
	for _, m := range obs.Members {
		if d.sweepLaneHeld(m.Pool, m.Member) {
			skip[laneKey(m.Pool, m.Member)] = true
		}
	}
	for _, a := range Roster(d.Config, obs, busy, skip) {
		role := d.role(a.Role)
		host := d.Config.Hosts[role.Host]
		d.ledger.LaunchSeq++
		// Program and role names cannot contain '.', so the ID never collides
		// across programs or roles; the start nonce keeps it unique per run.
		id := fmt.Sprintf("%s.%s.%d.%s-%d", d.Program, a.Role, a.Slot, d.nonce, d.ledger.LaunchSeq)
		values := map[string]string{"{program}": d.Program, "{role}": a.Role, "{slot}": strconv.Itoa(a.Slot), "{worker}": id, "{holder}": id, "{ticket}": a.Ticket, "{ticketLocal}": a.Local, "{state}": a.State, "{pool}": a.Pool, "{member}": a.Member, "{workRoot}": d.Config.WorkRoot}
		values["{prompt}"] = Render(role.Prompt, values)
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
		env = append(env, "CORVINT_DISPATCH_PROGRAM="+d.Program, "CORVINT_DISPATCH_ROLE="+a.Role, "CORVINT_DISPATCH_SLOT="+strconv.Itoa(a.Slot), "CORVINT_DISPATCH_TICKET="+a.Ticket, "CORVINT_DISPATCH_WORKER="+id)
		pid, identity, exit, err := launch(argv, env, d.Config.WorkRoot, d.workerDir(id))
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
			d.emit(Event{Kind: "launch-failed", Ticket: a.Ticket, Role: a.Role, Worker: id, Message: fmt.Sprintf("could not launch %s on %s: %v", a.Role, d.keyText(a.Key), err)})
			continue
		}
		w := &Worker{ID: id, Role: a.Role, Host: role.Host, Slot: a.Slot, Key: a.Key, Ticket: a.Ticket, Pool: a.Pool, Member: a.Member, PID: pid, LeaderIdentity: identity, Members: []Proc{{PID: pid, Identity: identity}}, Started: now, LastActive: now, State: "RUNNING", Fingerprint: Fingerprint(obs, a.Key)}
		if h := d.ledger.Progress[a.Key]; h != nil {
			w.BaseFingerprint, w.ProgressDigest = baseFingerprint(obs, a.Key), h.Current
		}
		for _, p := range host.ActivityPaths {
			w.ActivityPaths = append(w.ActivityPaths, Render(p, values))
		}
		d.ledger.Workers = append(d.ledger.Workers, w)
		d.exits[id] = exit
		// Record the worker before anything else, so a crash cannot leave
		// an untracked tree.
		if err := d.ledger.save(d.dir); err != nil {
			d.emit(Event{Kind: "alert", Worker: id, Message: "ledger unwritable after launch: " + err.Error()})
		}
		d.emit(Event{Kind: "launched", Ticket: a.Ticket, Role: a.Role, Worker: id, Message: fmt.Sprintf("launched %s slot %d on %s as %s (%s, pid %d)", a.Role, a.Slot, d.keyText(a.Key), id, role.Host, pid), Detail: map[string]string{"host": role.Host, "pid": strconv.Itoa(pid), "slot": strconv.Itoa(a.Slot), "state": a.State}})
	}
}

// diff emits state, claim, release and lane changes against the previous
// observation. The first observation only records a baseline.
func (d *Dispatcher) diff(obs *Observation) {
	now := &Seen{Tickets: map[string]string{}, Claims: map[string]string{}, Lanes: map[string]string{}}
	for _, t := range obs.Tickets {
		now.Tickets[t.ID] = strings.Join([]string{t.Status, t.State, t.Plan, t.PlanReason}, "|")
	}
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
	if old == nil {
		return
	}
	for _, id := range sortedKeys(now.Tickets) {
		was, ok := old.Tickets[id]
		if !ok || was == now.Tickets[id] {
			continue
		}
		o, n := strings.Split(was, "|"), strings.Split(now.Tickets[id], "|")
		d.emit(Event{Kind: "state", Ticket: id, Message: fmt.Sprintf("%s changed: status %s→%s, work state %s→%s, plan %s→%s", d.local(obs, id), o[0], n[0], o[1], n[1], planText(o[2], o[3]), planText(n[2], n[3]))})
		if n[3] == "RETRY_EXHAUSTED" && o[3] != n[3] {
			d.emit(Event{Kind: "needs-owner", Ticket: id, Message: fmt.Sprintf("%s exhausted its retries; only the owner can re-admit it with `corvint-tasks ticket reopen` (CAL-V0-043)", d.local(obs, id))})
		}
	}
	for _, id := range sortedKeys(now.Claims) {
		if _, ok := old.Claims[id]; !ok {
			c := strings.Split(now.Claims[id], "|")
			d.emit(Event{Kind: "claim", Ticket: c[0], Message: fmt.Sprintf("%s claimed %s for %s (attempt %s)", c[1], d.local(obs, c[0]), orText(c[3], "work"), id), Detail: map[string]string{"attempt": id, "holder": c[1], "phase": c[2]}})
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

func (d *Dispatcher) emit(e Event) {
	d.ledger.EventSeq++
	e.Profile, e.Seq, e.Program = EventProfile, d.ledger.EventSeq, d.Program
	e.At = d.Now().UTC().Format(time.RFC3339)
	if err := appendEvent(d.dir, e); err != nil && d.Out != nil {
		fmt.Fprintf(d.Out, "%s alert event log unwritable: %v\n", e.At, err)
	}
	if d.Out != nil {
		fmt.Fprintf(d.Out, "%s %s %s\n", e.At, e.Kind, e.Message)
	}
}

func (d *Dispatcher) workerDir(id string) string { return filepath.Join(d.dir, "workers", id) }

func (d *Dispatcher) role(name string) *Role {
	for i := range d.Config.Roles {
		if d.Config.Roles[i].Name == name {
			return &d.Config.Roles[i]
		}
	}
	return nil
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
