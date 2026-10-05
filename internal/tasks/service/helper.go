package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Helper wrappers (SERVICE500-007). `service run-helper` is the foreground
// manager entry point for one registered helper H. It is the only process
// that spawns H: a per-helper controller lock (U) admits one wrapper, and
// every spawn runs under the shared fence F from the final control read
// through the durable intent, the spawn, descendant verification and the
// acknowledged record. The lock order is U-helper→F; a helper never takes
// the service lock O or the dispatcher lock L, and never latches control.
//
// The helper profile is supported only where a helper's descendants can be
// proved retired: on Linux the wrapper is a child subreaper, so every
// orphaned descendant (including one that left the helper's process group
// or session) is reparented to it, and wait4 reporting ECHILD after a
// kill-and-reap sweep proves that no descendant remains. Elsewhere helpers
// are refused UNSUPPORTED at install and by run-helper.

// HelperProfileSupported reports whether goos can run the helper profile.
func HelperProfileSupported(goos string) bool { return goos == "linux" }

const (
	HelperRecordName = "taskman-user-service-helper/0"

	maxHelperRecord = 8 << 10
	maxHelperIntent = 4096
	// helperRetireBound bounds one retirement: the group kill, the leader
	// reap and the descendant sweep.
	helperRetireBound = 10 * time.Second
)

// helperFile names one helper's private state: ".lock" (U), ".intent.json"
// (the durable tree claim), ".json" (the record) and ".pulse.json".
func helperFile(id, suffix string) string { return "helper-" + id + suffix }

const (
	helperLockSuffix   = ".lock"
	helperIntentSuffix = ".intent.json"
	helperRecordSuffix = ".json"
	helperPulseSuffix  = ".pulse.json"
)

// helperUnit is the helper's log unit under logs/.
func helperUnit(id string) string { return "helper-" + id }

// helperEnvAllowlist is the small OS-required environment a helper inherits
// from its wrapper; everything else comes from the helper's explicit env.
var helperEnvAllowlist = []string{"HOME", "LANG", "LC_ALL", "LOGNAME", "PATH", "TZ", "USER", "XDG_RUNTIME_DIR"}

// helperIntent is the durable claim on one helper process tree. It is
// written under F before the spawn (SPAWNING) and acknowledged with the
// verified leader (RUNNING). It is removed only after the tree is proved
// retired; while it exists no wrapper spawns that helper, resume refuses
// and a stop's close is not OBSERVED. A wrapper that finds one at start
// cannot prove the earlier tree retired and holds; an operator verifies
// that no process of that tree remains and removes the file.
type helperIntent struct {
	Helper         string      `json:"helper"`
	ManifestSha256 wire.Digest `json:"manifestSha256"`
	Token          string      `json:"token"`
	Generation     uint64      `json:"generation"`
	State          string      `json:"state"`
	PID            int         `json:"pid,omitempty"`
	Identity       string      `json:"identity,omitempty"`
}

// helperRecord is one helper's durable failure/status record (<=8KiB):
// its latest generation, state, restart debt and last exit.
type helperRecord struct {
	Profile        string      `json:"profile"`
	Program        string      `json:"program"`
	Helper         string      `json:"helper"`
	ManifestSha256 wire.Digest `json:"manifestSha256"`
	State          string      `json:"state"`
	Generation     uint64      `json:"generation"`
	Hold           string      `json:"hold,omitempty"`
	LastExit       string      `json:"lastExit,omitempty"`
	Debt           Debt        `json:"debt"`
	At             int64       `json:"at"`
}

var helperRecordStates = map[string]bool{"STARTING": true, "RUNNING": true, "RETIRED": true, "HOLD": true}

func decodeStrict(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return wire.Errorf(wire.CodeMalformed, "/", "trailing data")
	}
	return nil
}

// readHelperIntent reads one helper's intent: nil when verified absent.
func (h Host) readHelperIntent(root, id string) (*helperIntent, []byte, error) {
	name := helperFile(id, helperIntentSuffix)
	raw, err := h.readPrivate(filepath.Join(root, name), maxHelperIntent)
	if absent(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var in helperIntent
	if err := decodeStrict(raw, &in); err != nil || in.Helper != id || in.Token == "" || (in.State != "SPAWNING" && in.State != "RUNNING") {
		return nil, nil, wire.Errorf(wire.CodeUncertainEffect, "/"+name, "helper intent is unreadable")
	}
	return &in, raw, nil
}

func writeHelperIntent(root string, in helperIntent) (wire.Digest, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	if len(raw) > maxHelperIntent {
		return "", wire.Errorf(wire.CodeLimitExceeded, "/helperIntent", "helper intent exceeds %d bytes", maxHelperIntent)
	}
	return wire.Sum(raw), writeAtomic(root, helperFile(in.Helper, helperIntentSuffix), raw)
}

// readHelperRecord reads one helper's record: a fresh RETIRED record at
// generation 0 when verified absent.
func (h Host) readHelperRecord(root, program, id string) (helperRecord, error) {
	name := helperFile(id, helperRecordSuffix)
	raw, err := h.readPrivate(filepath.Join(root, name), maxHelperRecord)
	if absent(err) {
		return helperRecord{Profile: HelperRecordName, Program: program, Helper: id, State: "RETIRED", Debt: Debt{Fences: map[string]string{}}}, nil
	}
	if err != nil {
		return helperRecord{}, err
	}
	var r helperRecord
	if err := decodeStrict(raw, &r); err != nil || r.Profile != HelperRecordName || r.Program != program || r.Helper != id || !helperRecordStates[r.State] {
		return helperRecord{}, wire.Errorf(wire.CodeUncertainEffect, "/"+name, "helper record is unreadable")
	}
	if r.Debt.Fences == nil {
		r.Debt.Fences = map[string]string{}
	}
	return r, nil
}

func writeHelperRecord(root string, r helperRecord) error {
	raw, err := encodeHelperRecord(r)
	if err != nil {
		return err
	}
	return writeAtomic(root, helperFile(r.Helper, helperRecordSuffix), raw)
}

func encodeHelperRecord(r helperRecord) ([]byte, error) {
	r.Hold, r.LastExit = truncate(r.Hold, 512), truncate(r.LastExit, 256)
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxHelperRecord {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/helperRecord", "helper record exceeds %d bytes", maxHelperRecord)
	}
	return raw, nil
}

// helperIntentIDs lists the helper ids whose intent file exists in root,
// whatever the current manifest declares.
func (h Host) helperIntentIDs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, "helper-") && strings.HasSuffix(n, helperIntentSuffix) {
			ids = append(ids, strings.TrimSuffix(strings.TrimPrefix(n, "helper-"), helperIntentSuffix))
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// helperIntentStates is Stop's view, under F, of every helper tree claim
// for the strong acknowledgement: an acknowledged RUNNING tree is a
// committed effect that the suppression now retires, while a SPAWNING or
// unreadable claim is an unresolved launch.
func (h Host) helperIntentStates(root string) []string {
	ids, err := h.helperIntentIDs(root)
	if err != nil {
		return []string{"UNKNOWN"}
	}
	out := []string{}
	for _, id := range ids {
		in, _, err := h.readHelperIntent(root, id)
		switch {
		case err != nil:
			out = append(out, "UNKNOWN")
		case in == nil:
		case in.State == "RUNNING":
			out = append(out, "COMMITTED")
		default:
			out = append(out, "UNRESOLVED")
		}
	}
	return out
}

// helperTreesSettled reports whether no helper intent exists: every helper
// tree that was ever spawned is proved retired. An unreadable directory is
// not settled.
func (h Host) helperTreesSettled(root string) bool {
	ids, err := h.helperIntentIDs(root)
	return err == nil && len(ids) == 0
}

// helperSpawner starts helper process trees under the platform's
// descendant ownership.
type helperSpawner interface {
	// Quiet proves the wrapper has no child left from an earlier
	// generation.
	Quiet() error
	Start(cmd *exec.Cmd) (helperProc, error)
}

// helperProc is one owned helper tree.
type helperProc interface {
	PID() int
	// Verify proves the started leader is owned and contained (its own
	// process group, this wrapper's child, a readable start identity, the
	// wrapper still a subreaper) and returns its start identity.
	Verify() (string, error)
	// Exited is closed when the leader exits; it stays unreaped.
	Exited() <-chan struct{}
	// Retire kills the tree and proves within bound that no descendant
	// remains. A nil error is that proof; exit describes the leader's exit.
	Retire(bound time.Duration) (exit string, err error)
}

// HelperOptions configure the internal `service run-helper` entry point.
type HelperOptions struct {
	RunOptions
	Helper string
	// RetireBound bounds one helper tree retirement (default 10s).
	RetireBound time.Duration

	// spawner and clock are test seams; nil uses the platform runtime and
	// boot clock.
	spawner func() (helperSpawner, error)
	clock   func() (boot string, now uint64, known bool)
	getenv  func(string) string
}

// helperWrapper is one run-helper process's state.
type helperWrapper struct {
	o    HelperOptions
	ctx  context.Context
	root string
	sp   helperSpawner
	logs *unitLogs
	self string
	// drainBound bounds settleDrains (default logCloseBound).
	drainBound time.Duration

	stamp, hstamp exeStamp
	seen          wire.Digest
	hold, note    string

	// the owned generation, when one runs
	proc  helperProc
	gen   uint64
	ident wire.Digest
	token string
	// pending is a retirement whose record is not yet published.
	pending *helperSettlement
	// drains forwards the current tree's stdout and stderr pipes into the
	// logs; readers are their read ends.
	drains  *sync.WaitGroup
	readers []*os.File
}

// helperSettlement is one retired generation's outcome awaiting its record.
type helperSettlement struct {
	gen         uint64
	token       string
	exit        string
	reason      string
	err         error
	charge      bool
	termination string
	// recorded marks that this generation's record was durably written and
	// only the intent removal remains. A retry then never rewrites the
	// record: once the intent is gone a reconciled resume may own it.
	recorded bool
}

// releaseHelperLock closes the helper's logs and then releases U. When the
// bounded close leaves log I/O outstanding (a stalled filesystem), U stays
// held until that I/O actually retires, so a successor wrapper never
// shares the log files with a late writer or publisher; process exit
// releases it otherwise.
func releaseHelperLock(logs *unitLogs, unlock func()) {
	if logs != nil && !logs.close() {
		go func() { logs.wait(); unlock() }()
		return
	}
	if unlock != nil {
		unlock()
	}
}

// RunHelper is the foreground helper wrapper. It runs helper H only while
// control is RUNNING and bound to the installed manifest, the program's
// executable and dispatch config pins match, H is in the manifest with an
// unchanged helper executable, the helper's restart debt is ELIGIBLE and no
// earlier H tree is unretired. DRAINING, STOPPED, drift, interruption and
// any helper exit retire the whole tree; only an unexpected exit charges
// restart debt, and only after the tree is proved retired. Unproved
// retirement keeps the intent and holds. It returns when ctx ends.
func RunHelper(ctx context.Context, o HelperOptions) error {
	root, err := o.StateRoot(o.Program)
	if err != nil {
		return err
	}
	if o.Manifest != filepath.Join(root, manifestFile) {
		return wire.Errorf(wire.CodeUnsupported, "/manifest", "manifest must be the registry manifest %s", filepath.Join(root, manifestFile))
	}
	if !serviceName.MatchString(o.Helper) {
		return wire.Errorf(wire.CodeMalformed, "/helper", "helper id must match [a-z][a-z0-9-]{0,23}")
	}
	if !HelperProfileSupported(o.GOOS) {
		return wire.Errorf(wire.CodeUnsupported, "/helper", "the helper profile is unsupported on %s: detached helper descendants cannot be proved retired there", o.GOOS)
	}
	if o.Poll <= 0 {
		o.Poll = 2 * time.Second
	}
	if o.Pulse <= 0 {
		o.Pulse = 10 * time.Second
	}
	if o.RetireBound <= 0 {
		o.RetireBound = helperRetireBound
	}
	if o.spawner == nil {
		o.spawner = helperRuntime
	}
	if o.clock == nil {
		o.clock = bootClock
	}
	if o.getenv == nil {
		o.getenv = os.Getenv
	}
	sp, err := o.spawner()
	if err != nil {
		return err
	}
	w := &helperWrapper{o: o, ctx: ctx, root: root, sp: sp}
	w.self, _ = supervisor.ProcessIdentity(os.Getpid())
	return w.loop()
}

func (w *helperWrapper) loop() error {
	var unlock func()
	// Only the wrapper holding U opens and publishes the helper's logs, and
	// it closes them before releasing U, so a refused duplicate never
	// overwrites the owner's counters.
	defer func() {
		w.settleDrains()
		releaseHelperLock(w.logs, unlock)
	}()
	ticker := time.NewTicker(w.o.Poll)
	defer ticker.Stop()
	var pulseAt time.Time
	pulsed := ""
	for {
		if unlock == nil {
			u, err := w.controller()
			if err != nil {
				// Another wrapper owns this helper: this one never spawns,
				// writes no helper state and only waits.
				w.hold = "another run-helper controls helper " + w.o.Helper
			} else {
				unlock, w.hold = u, ""
				w.logs = openUnitLogs(w.root, helperUnit(w.o.Helper), "stdout", "stderr")
			}
		}
		if unlock != nil {
			w.tick()
			if w.ctx.Err() != nil && w.proc != nil {
				w.retire(false, "run-helper interrupted")
				w.flush()
			}
			state := "IDLE"
			switch {
			case w.hold != "":
				state = "HOLD"
			case w.proc != nil:
				state = "RUNNING"
			}
			if w.seen != "" && (pulseAt.IsZero() || w.o.now().Sub(pulseAt) >= w.o.Pulse || state != pulsed) {
				text := w.hold
				if text == "" {
					text = w.note
				}
				p := Pulse{Program: w.o.Program, ManifestSha256: w.seen, PID: os.Getpid(), Identity: w.self, State: state, Hold: truncate(text, 1024), At: w.o.now().Unix()}
				if raw, err := EncodePulse(p); err == nil && writeAtomic(w.root, helperFile(w.o.Helper, helperPulseSuffix), raw) == nil {
					pulseAt, pulsed = w.o.now(), state
				}
			}
			w.logs.publish()
		}
		if w.ctx.Err() != nil {
			return nil
		}
		var exited <-chan struct{}
		if w.proc != nil {
			exited = w.proc.Exited()
		}
		select {
		case <-w.ctx.Done():
		case <-ticker.C:
		case <-exited:
		}
	}
}

// controller takes U, the helper's nonblocking controller lock.
func (w *helperWrapper) controller() (func(), error) {
	f, err := os.OpenFile(filepath.Join(w.root, helperFile(w.o.Helper, helperLockSuffix)), os.O_RDWR|os.O_CREATE|noFollow, 0o600)
	if err != nil {
		return nil, err
	}
	if err := tryLock(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() { f.Close() }, nil
}

// tick is one observation while holding U.
func (w *helperWrapper) tick() {
	w.note = ""
	if !w.flush() {
		return
	}
	d, spec, path, hold := w.observe()
	if d.ident != "" {
		w.seen = d.ident
	}
	want := hold == "" && d.run && d.desired == "RUNNING" && w.ctx.Err() == nil
	if w.proc != nil {
		select {
		case <-w.proc.Exited():
			w.retire(true, "helper exited")
			w.flush()
			return
		default:
		}
		switch {
		case !want && hold != "":
			w.retire(false, "retired on hold: "+hold)
		case !want:
			w.retire(false, "retired: control is not RUNNING")
		case d.ident != w.ident:
			w.retire(false, "retired: the installed manifest changed")
		default:
			// A helper has no health signal beyond its process, so a
			// running generation never certifies a healthy reset: only a
			// reconciled resume resets helper restart debt.
			w.hold = ""
			return
		}
		w.flush()
		return
	}
	w.hold = hold
	if want {
		w.spawn(d, spec, path)
	}
}

// observe is the main's observation plus the helper's own pins: H must be
// in the installed profile and its executable must still hash to the
// manifest's helper pin.
func (w *helperWrapper) observe() (desiredRun, *Helper, string, string) {
	d := w.o.observe(w.root, &w.stamp)
	if d.hold != "" || !d.run {
		return d, nil, "", d.hold
	}
	var spec *Helper
	for i := range d.profile.Helpers {
		if d.profile.Helpers[i].ID == w.o.Helper {
			spec = &d.profile.Helpers[i]
		}
	}
	pin, ok := d.manifest.HelperExecutables[w.o.Helper]
	if spec == nil || !ok {
		return d, nil, "", "helper " + w.o.Helper + " is not in the installed manifest"
	}
	sha, path, err := w.o.fileSha(spec.Argv[0], &w.hstamp)
	if err != nil || sha != pin.Sha256 || path != pin.Path {
		return d, nil, "", "helper executable differs from the installed pin"
	}
	return d, spec, path, ""
}

// spawn starts one generation. F is held from the final control read
// through the intent, the start, descendant verification and the
// acknowledged record; it is released before any retirement wait.
func (w *helperWrapper) spawn(d desiredRun, spec *Helper, path string) {
	if err := w.sp.Quiet(); err != nil {
		w.hold = "a child of an earlier helper generation remains: " + describe(err)
		return
	}
	boot, now, known := w.o.clock()
	unlock, err := w.o.fence(w.root)
	if err != nil {
		w.hold = "control fence: " + describe(err)
		return
	}
	locked := true
	release := func() {
		if locked {
			locked = false
			unlock()
		}
	}
	defer release()
	// An interruption that arrived while waiting for F starts nothing.
	if w.ctx.Err() != nil {
		return
	}
	c, err := w.o.readControl(w.root)
	if err != nil || c.ManifestIdentity != d.ident || c.Desired != "RUNNING" {
		return
	}
	if d.legacy != nil {
		// Helpers only read the legacy stop file; the main latches it.
		switch w.o.legacyPresence(*d.legacy) {
		case "ABSENT":
		case "PRESENT":
			return
		default:
			w.hold = "legacy stop file presence is UNKNOWN"
			return
		}
	}
	id := w.o.Helper
	if in, _, err := w.o.readHelperIntent(w.root, id); err != nil || in != nil {
		w.hold = "an earlier helper tree is not proved retired; verify that no process of it remains, then remove " + helperFile(id, helperIntentSuffix)
		return
	}
	rec, err := w.o.readHelperRecord(w.root, w.o.Program, id)
	if err != nil {
		w.hold = "helper record: " + describe(err)
		return
	}
	if rec.State == "HOLD" {
		w.hold = "helper is held: " + rec.Hold + "; review, then stop and resume the program"
		return
	}
	debt, eligible := Eligibility(rec.Debt, boot, now, known)
	if eligible != "ELIGIBLE" {
		if !reflect.DeepEqual(debt, rec.Debt) {
			rec.Debt, rec.At = debt, w.o.now().Unix()
			_ = writeHelperRecord(w.root, rec)
		}
		if eligible == "BACKOFF" {
			w.note = "BACKOFF: restart debt " + strconv.Itoa(int(debt.Failures)) + " until uptime " + strconv.FormatUint(debt.EligibleAfter, 10) + "s"
			return
		}
		w.hold = "restart debt " + eligible
		return
	}
	if rec.Generation == ^uint64(0) {
		w.hold = "helper generation overflow"
		return
	}
	gen := rec.Generation + 1
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		w.hold = "helper token: " + describe(err)
		return
	}
	in := helperIntent{Helper: id, ManifestSha256: d.ident, Token: hex.EncodeToString(nonce[:]), Generation: gen, State: "SPAWNING"}
	if _, err := writeHelperIntent(w.root, in); err != nil {
		w.hold = "helper intent: " + describe(err)
		return
	}
	rec.ManifestSha256, rec.State, rec.Generation, rec.Hold, rec.Debt, rec.At = d.ident, "STARTING", gen, "", debt, w.o.now().Unix()
	if err := writeHelperRecord(w.root, rec); err != nil {
		w.hold = "helper record: " + describe(err)
		w.abandon(in)
		return
	}
	cmd, reads, err := w.command(spec, path)
	if err != nil {
		w.hold = "helper output: " + describe(err)
		w.abandon(in)
		return
	}
	proc, err := w.sp.Start(cmd)
	cmd.Stdout.(*os.File).Close()
	cmd.Stderr.(*os.File).Close()
	if err != nil {
		reads[0].Close()
		reads[1].Close()
		// A failed start executed nothing and left nothing live. Its
		// backoff runs from this observation, after the failure.
		boot, now, known = w.o.clock()
		s := &helperSettlement{gen: gen, token: in.Token, exit: describe(err), reason: "helper start failed", charge: true, termination: "PROVED_NO_LIVE_UNEXECUTED"}
		if err := w.record(s, boot, now, known); err != nil {
			w.pending = s
		}
		return
	}
	w.startDrains(reads)
	w.proc, w.gen, w.ident, w.token = proc, gen, d.ident, in.Token
	identity, err := proc.Verify()
	if err == nil {
		in.State, in.PID, in.Identity = "RUNNING", proc.PID(), identity
		if _, err = writeHelperIntent(w.root, in); err == nil {
			rec.State, rec.At = "RUNNING", w.o.now().Unix()
			err = writeHelperRecord(w.root, rec)
		}
		if err == nil {
			w.hold = ""
			return
		}
	}
	// An unacknowledged tree is retired before anything else; F is released
	// first.
	release()
	w.retire(true, "helper ownership not verified: "+describe(err))
	w.flush()
}

// abandon removes an intent whose generation started nothing. A removal
// failure keeps it, which holds every later spawn.
func (w *helperWrapper) abandon(in helperIntent) {
	raw, err := json.Marshal(in)
	if err == nil {
		_ = w.o.removeExact(filepath.Join(w.root, helperFile(in.Helper, helperIntentSuffix)), wire.Sum(raw))
	}
}

// command builds the helper's exec: the pinned resolved path with the
// declared argv, the declared cwd, an explicit environment and pipes into
// the bounded log sinks.
func (w *helperWrapper) command(spec *Helper, path string) (*exec.Cmd, [2]*os.File, error) {
	env := map[string]string{}
	for _, k := range helperEnvAllowlist {
		if v := w.o.getenv(k); v != "" {
			env[k] = v
		}
	}
	for k, v := range spec.Env {
		env[k] = v
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	list := make([]string, 0, len(keys))
	for _, k := range keys {
		list = append(list, k+"="+env[k])
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, [2]*os.File{}, err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		return nil, [2]*os.File{}, err
	}
	cmd := &exec.Cmd{Path: path, Args: append([]string(nil), spec.Argv...), Dir: spec.Cwd, Env: list, Stdout: outW, Stderr: errW}
	return cmd, [2]*os.File{outR, errR}, nil
}

// drain copies one helper stream into its sink until every holder of the
// pipe's write end has exited. Sink writes never block.
func drain(r *os.File, sink io.Writer) {
	defer r.Close()
	_, _ = io.Copy(sink, r)
}

func (w *helperWrapper) startDrains(reads [2]*os.File) {
	w.drains, w.readers = &sync.WaitGroup{}, []*os.File{reads[0], reads[1]}
	for i, stream := range []string{"stdout", "stderr"} {
		w.drains.Add(1)
		go func(r *os.File, sink io.Writer) { defer w.drains.Done(); drain(r, sink) }(reads[i], w.logs.streams[stream])
	}
}

// settleDrains waits at most the close bound for the pipe readers to
// forward what a retired tree wrote, so the counters published next (and
// the final ones) include it. A reader still blocked at the bound, whose
// pipe some process outside the proved tree holds open, is closed; bytes
// left unread in that pipe are not counted.
func (w *helperWrapper) settleDrains() {
	if w.drains == nil {
		return
	}
	done := make(chan struct{})
	go func(wg *sync.WaitGroup) { wg.Wait(); close(done) }(w.drains)
	bound := w.drainBound
	if bound <= 0 {
		bound = logCloseBound
	}
	select {
	case <-done:
	case <-time.After(bound):
		for _, r := range w.readers {
			_ = r.Close()
		}
		select {
		case <-done:
		case <-time.After(bound):
		}
	}
	w.drains, w.readers = nil, nil
}

// retire kills the owned tree and proves it retired outside F, then
// leaves the outcome pending for its record. charge marks an unexpected
// end (an exit or a failed verification); an exit observed after the
// wrapper was interrupted is an intentional stop.
func (w *helperWrapper) retire(charge bool, reason string) {
	exit, err := w.proc.Retire(w.o.RetireBound)
	if err == nil {
		w.settleDrains()
	}
	w.pending = &helperSettlement{gen: w.gen, token: w.token, exit: exit, reason: reason, err: err, charge: charge && w.ctx.Err() == nil, termination: "PROVED_TERMINATED"}
	w.proc = nil
}

// flush publishes a pending retirement under F; it reports whether nothing
// remains pending.
func (w *helperWrapper) flush() bool {
	if w.pending == nil {
		return true
	}
	boot, now, known := w.o.clock()
	unlock, err := w.o.fence(w.root)
	if err != nil {
		w.hold = "recording helper retirement: " + describe(err)
		return false
	}
	defer unlock()
	if err := w.record(w.pending, boot, now, known); err != nil {
		w.hold = "recording helper retirement: " + describe(err)
		return false
	}
	w.pending = nil
	return true
}

// record publishes a retired generation's outcome under a held F. Proved
// retirement records the outcome (charging restart debt once, by
// generation, for an unexpected end) before removing the intent;
// unproved retirement records HOLD and keeps the intent. A retry after a
// partial publication replays: the generation's debt fence charges once.
func (w *helperWrapper) record(s *helperSettlement, boot string, now uint64, known bool) error {
	id := w.o.Helper
	rec, err := w.o.readHelperRecord(w.root, w.o.Program, id)
	if err != nil {
		return err
	}
	in, raw, err := w.o.readHelperIntent(w.root, id)
	if err != nil {
		return err
	}
	if in != nil && in.Token != s.token {
		return wire.Errorf(wire.CodeUncertainEffect, "/"+helperFile(id, helperIntentSuffix), "helper intent belongs to another wrapper")
	}
	intentPath := filepath.Join(w.root, helperFile(id, helperIntentSuffix))
	if s.recorded {
		// Only the intent removal remains. Resume refuses while the intent
		// exists, so the record is still this generation's; once the
		// intent is gone the record may already be reconciled by resume.
		if in == nil {
			return nil
		}
		return w.o.removeExact(intentPath, wire.Sum(raw))
	}
	if in == nil {
		// The intent was removed before this generation's outcome was
		// recorded (an operator recovery): the record may be reconciled
		// already, so writing this outcome could undo it. Hold.
		return wire.Errorf(wire.CodeUncertainEffect, "/"+helperFile(id, helperIntentSuffix), "helper intent was removed before this generation's outcome was recorded")
	}
	if rec.Generation != s.gen {
		return wire.Errorf(wire.CodeUncertainEffect, "/"+helperFile(id, helperRecordSuffix), "helper record generation changed")
	}
	priorState, priorHold := rec.State, rec.Hold
	rec.LastExit, rec.At = s.reason+": "+s.exit, w.o.now().Unix()
	if s.err != nil {
		w.hold = "helper tree retirement unproved: " + describe(s.err)
		rec.State, rec.Hold = "HOLD", w.hold
		return writeHelperRecord(w.root, rec)
	}
	rec.State, rec.Hold = "RETIRED", ""
	if s.charge {
		debt, result, cerr := ChargeFailure(rec.Debt, FailureObservation{Generation: strconv.FormatUint(s.gen, 10), Outcome: "FAILED", BootID: boot, Termination: s.termination, Now: now, TimeKnown: known, IdentityKnown: true})
		rec.Debt = debt
		switch result {
		case "BACKOFF":
		case "REPLAY":
			// This generation's outcome is already recorded; a retry after a
			// partial publication keeps the recorded state.
			rec.State, rec.Hold = priorState, priorHold
		case "SERVICE_RESTART_HOLD":
			rec.State, rec.Hold = "HOLD", "SERVICE_RESTART_HOLD after "+strconv.Itoa(int(debt.Failures))+" failures"
		default:
			rec.State, rec.Hold = "HOLD", "restart debt not charged: "+describe(cerr)
		}
	}
	if err := writeHelperRecord(w.root, rec); err != nil {
		return err
	}
	s.recorded = true
	return w.o.removeExact(intentPath, wire.Sum(raw))
}

// Resume operation journal (SERVICE500-003, SERVICE500-006). Before resume
// resets any helper record it durably publishes, under F, the operation it
// is about to apply: the request and its hash, the digest of the control it
// observed (Before; After is resumeAfter(Before) and is never stored) and,
// for every declared helper, the digests of its record before and after the
// reset. A retry of the same request reconciles against that journal: each
// record must still be its Before (it is then reset) or its After (already
// reset), and anything else is newer debt, which the retry refuses rather
// than erases. A retry whose control has changed since is refused. A
// different resume request that finds an unfinished journal supersedes it:
// it first records the superseded request in the control request ledger
// under a marker hash, so a later retry of that request is a
// REQUEST_ID_CONFLICT and never resets the debt charged after it. The
// journal is removed after the control and ledger writes.
const (
	ResumeOperationName = "taskman-user-service-resume-operation/0"

	resumeOperationFile = "resume-operation.json"
	maxResumeOperation  = 8 << 10
)

type resumeHelperReset struct {
	Helper string      `json:"helper"`
	Before wire.Digest `json:"before"`
	After  wire.Digest `json:"after"`
}

type resumeOperation struct {
	Profile       string              `json:"profile"`
	Program       string              `json:"program"`
	RequestID     string              `json:"requestId"`
	RequestSha256 wire.Digest         `json:"requestSha256"`
	BeforeControl wire.Digest         `json:"beforeControl"`
	At            int64               `json:"at"`
	Helpers       []resumeHelperReset `json:"helpers"`
}

// supersededResume is the ledger hash that marks an unfinished resume as
// superseded: it never equals the request's own hash.
func supersededResume(op *resumeOperation) wire.Digest {
	return wire.Sum([]byte("SUPERSEDED\n" + op.RequestID + "\n" + string(op.RequestSha256)))
}

// helperRecordDigest is the digest of a record's canonical encoding, with
// the normalization readHelperRecord applies.
func helperRecordDigest(r helperRecord) (wire.Digest, error) {
	if r.Debt.Fences == nil {
		r.Debt.Fences = map[string]string{}
	}
	raw, err := encodeHelperRecord(r)
	if err != nil {
		return "", err
	}
	return wire.Sum(raw), nil
}

// resumedHelper is the record a reconciled resume publishes: debt and HOLD
// reset, tree RETIRED, stamped with the operation's time.
func resumedHelper(rec helperRecord, at int64) (helperRecord, bool) {
	next := rec
	next.Debt, next.State, next.Hold = resetDebt(rec.Debt), "RETIRED", ""
	if next.Debt.Fences == nil {
		next.Debt.Fences = map[string]string{}
	}
	if reflect.DeepEqual(next, rec) {
		return rec, false
	}
	next.At = at
	return next, true
}

// readResumeOperation reads the resume journal: nil when verified absent.
func (h Host) readResumeOperation(root, program string) (*resumeOperation, []byte, error) {
	raw, err := h.readPrivate(filepath.Join(root, resumeOperationFile), maxResumeOperation)
	if absent(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, wire.Errorf(wire.CodeUncertainEffect, "/"+resumeOperationFile, "resume operation journal is UNKNOWN: %v", err)
	}
	var op resumeOperation
	if err := decodeStrict(raw, &op); err != nil || op.Profile != ResumeOperationName || op.Program != program || op.RequestID == "" || op.RequestSha256 == "" || op.BeforeControl == "" {
		return nil, nil, wire.Errorf(wire.CodeUncertainEffect, "/"+resumeOperationFile, "resume operation journal is unreadable")
	}
	return &op, raw, nil
}

// resumeHelpers is resume's helper reconciliation under F. It refuses while
// any helper tree is not proved retired, binds (or finds) this request's
// durable operation and applies it, resetting each declared helper's
// restart debt and HOLD (a reconciled resume). It returns the updated
// request ledger, the helpers the operation resets and the journal's
// digest, which the caller removes after publishing control.
func (h Host) resumeHelpers(root string, m *Manifest, c Control, rs []controlRequest, request string, hash wire.Digest) ([]controlRequest, []string, wire.Digest, error) {
	ids, err := h.helperIntentIDs(root)
	if err != nil {
		return rs, nil, "", wire.Errorf(wire.CodeUncertainEffect, "/helpers", "helper tree state is UNKNOWN: %v", err)
	}
	if len(ids) > 0 {
		return rs, nil, "", wire.Errorf(wire.CodeUncertainEffect, "/helpers", "helper trees %s are not proved retired; resume waits until run-helper retires them or an operator verifies that no process remains and removes the intent", strings.Join(ids, ", "))
	}
	p, err := DecodeProfile(m.ProfileRaw)
	if err != nil {
		return rs, nil, "", err
	}
	rawControl, err := EncodeControl(c)
	if err != nil {
		return rs, nil, "", err
	}
	before := wire.Sum(rawControl)
	op, raw, err := h.readResumeOperation(root, m.Program)
	if err != nil {
		return rs, nil, "", err
	}
	switch {
	case op == nil:
	case op.RequestID == request && op.RequestSha256 != hash:
		return rs, nil, "", wire.Errorf(wire.CodeRequestIDConflict, "/requestId", "request id was already used for a different resume operation")
	case op.RequestID == request && op.BeforeControl != before:
		return rs, nil, "", wire.Errorf(wire.CodeResourceCollision, "/control", "resume request %s was interrupted and control has changed since; it is refused so later debt is kept; issue a new resume request", request)
	case op.RequestID == request:
		reset, err := h.applyResume(root, m.Program, p, op)
		return rs, reset, wire.Sum(raw), err
	default:
		// Another request's journal: finished when the ledger binds its
		// own hash, otherwise superseded (an existing marker is kept).
		if _, ok := lookupRequest(rs, op.RequestID); !ok {
			if rs, err = h.rememberRequest(root, rs, op.RequestID, supersededResume(op)); err != nil {
				return rs, nil, "", err
			}
		}
	}
	next := &resumeOperation{Profile: ResumeOperationName, Program: m.Program, RequestID: request, RequestSha256: hash, BeforeControl: before, At: h.now().Unix(), Helpers: []resumeHelperReset{}}
	for _, x := range p.Helpers {
		rec, err := h.readHelperRecord(root, m.Program, x.ID)
		if err != nil {
			return rs, nil, "", err
		}
		d0, err := helperRecordDigest(rec)
		if err != nil {
			return rs, nil, "", err
		}
		post, _ := resumedHelper(rec, next.At)
		d1, err := helperRecordDigest(post)
		if err != nil {
			return rs, nil, "", err
		}
		next.Helpers = append(next.Helpers, resumeHelperReset{Helper: x.ID, Before: d0, After: d1})
	}
	raw, err = json.Marshal(next)
	if err != nil {
		return rs, nil, "", err
	}
	if len(raw) > maxResumeOperation {
		return rs, nil, "", wire.Errorf(wire.CodeLimitExceeded, "/resumeOperation", "resume operation exceeds %d bytes", maxResumeOperation)
	}
	if err := writeAtomic(root, resumeOperationFile, raw); err != nil {
		return rs, nil, "", err
	}
	reset, err := h.applyResume(root, m.Program, p, next)
	return rs, reset, wire.Sum(raw), err
}

// applyResume publishes op's resets: a record at its Before digest is
// reset, one at its After digest is already reset, and any other record is
// newer debt that the operation refuses to erase.
func (h Host) applyResume(root, program string, p *Profile, op *resumeOperation) ([]string, error) {
	if len(op.Helpers) != len(p.Helpers) {
		return nil, wire.Errorf(wire.CodeResourceCollision, "/helpers", "resume operation %s does not bind the declared helpers", op.RequestID)
	}
	reset := []string{}
	for i, x := range p.Helpers {
		j := op.Helpers[i]
		if j.Helper != x.ID {
			return nil, wire.Errorf(wire.CodeResourceCollision, "/helpers", "resume operation %s does not bind the declared helpers", op.RequestID)
		}
		rec, err := h.readHelperRecord(root, program, x.ID)
		if err != nil {
			return nil, err
		}
		d, err := helperRecordDigest(rec)
		if err != nil {
			return nil, err
		}
		switch d {
		case j.After:
		case j.Before:
			post, changed := resumedHelper(rec, op.At)
			if d1, err := helperRecordDigest(post); err != nil || d1 != j.After {
				return nil, wire.Errorf(wire.CodeResourceCollision, "/helpers/"+x.ID, "helper %s's reset does not match resume operation %s", x.ID, op.RequestID)
			}
			if changed {
				if err := writeHelperRecord(root, post); err != nil {
					return nil, err
				}
			}
		default:
			return nil, wire.Errorf(wire.CodeResourceCollision, "/helpers/"+x.ID, "helper %s's record changed after resume operation %s was journaled; the newer debt is kept and the resume is refused; issue a new resume request", x.ID, op.RequestID)
		}
		if j.Before != j.After {
			reset = append(reset, x.ID)
		}
	}
	return reset, nil
}

// helperStatus is Status's pure read of one helper: its tree intent,
// record, wrapper pulse and log counters.
func (h Host) helperStatus(root, program, id string, ident wire.Digest) *wire.Object {
	o := wire.NewObject().Set("id", wire.String(id))
	switch in, _, err := h.readHelperIntent(root, id); {
	case err != nil:
		o.Set("tree", wire.String("UNKNOWN"))
	case in == nil:
		o.Set("tree", wire.String("ABSENT"))
	default:
		o.Set("tree", wire.String(in.State)).Set("generation", wire.String(strconv.FormatUint(in.Generation, 10)))
	}
	if rec, err := h.readHelperRecord(root, program, id); err != nil {
		o.Set("record", wire.String("UNKNOWN"))
	} else {
		o.Set("record", wire.ObjectValue(wire.NewObject().Set("state", wire.String(rec.State)).Set("generation", wire.String(strconv.FormatUint(rec.Generation, 10))).Set("failures", wire.String(strconv.Itoa(int(rec.Debt.Failures)))).Set("eligibleAfter", wire.String(strconv.FormatUint(rec.Debt.EligibleAfter, 10))).Set("hold", optionalString(rec.Hold)).Set("lastExit", optionalString(rec.LastExit))))
	}
	state, p := h.pulseStateAt(root, helperFile(id, helperPulseSuffix), ident)
	pulse := wire.NewObject().Set("state", wire.String(state))
	if p != nil {
		pulse.Set("pid", wire.String(strconv.Itoa(p.PID))).Set("at", wire.String(strconv.FormatInt(p.At, 10))).Set("hold", optionalString(p.Hold))
	}
	o.Set("pulse", wire.ObjectValue(pulse))
	o.Set("logs", h.logStatus(root, helperUnit(id), "stdout", "stderr"))
	return o
}

// logStatus reports one unit's published log counters. The bounded
// excerpt is withheld: log text is unscreened user output, and status
// exposes no secrets; only its sanitized byte count is reported.
func (h Host) logStatus(root, unit string, streams ...string) wire.Value {
	stats, err := h.readLogStatus(root, unit)
	if absent(err) {
		return wire.String("ABSENT")
	}
	if err != nil {
		return wire.String("UNKNOWN")
	}
	o := wire.NewObject().Set("dir", wire.String(filepath.Join(root, logDir, unit)))
	for _, s := range streams {
		st, ok := stats[s]
		if !ok {
			o.Set(s, wire.String("UNKNOWN"))
			continue
		}
		so := wire.NewObject().Set("written", wire.String(strconv.FormatUint(st.Written, 10))).Set("dropped", wire.String(strconv.FormatUint(st.Dropped, 10))).Set("ioError", optionalString(st.IOError))
		if text, err := h.logExcerpt(root, unit, s); err == nil {
			so.Set("excerpt", wire.String("WITHHELD")).Set("excerptBytes", wire.String(strconv.Itoa(len(text))))
		} else {
			so.Set("excerpt", wire.String("UNAVAILABLE"))
		}
		o.Set(s, wire.ObjectValue(so))
	}
	return wire.ObjectValue(o)
}
