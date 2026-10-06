package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Detached attempt runs (ATR-V0-008..014). A launcher starts a supervisor in
// its own session; the supervisor is the attached runner with its envelope and
// command output kept in a private run directory, and a later session attaches
// to read them.

const (
	runRecordProfile = "taskman-attempt-run-record/0"
	runStarting      = "STARTING"
	runRunning       = "RUNNING"
	runFinished      = "FINISHED"

	runRecordName        = "record.json"
	runResultName        = "result.json"
	runOutputName        = "output.log"
	runOutputPrevName    = "output.1.log"
	runSupervisorLogName = "supervisor.log"

	maxRunRecordBytes = 4096
	maxRunResultBytes = 64 << 10
	maxRunsPerAttempt = 64
	maxAttachWait     = 86400
	// runExitPending is EX_TEMPFAIL: the run has not finished; attach later.
	runExitPending = 75
	// runReadyBound covers the supervisor's pre-launch writes (one bounded
	// context) and process start.
	runReadyBound = 2*runWriteBound + 30*time.Second
)

var (
	// runSegmentBytes is one output segment; the current and the previous
	// segment are kept (ATR-V0-010).
	runSegmentBytes int64 = 8 << 20
	// runPoll separates an attach's record reads while it waits.
	runPoll = time.Second
	// detachExecutable names the program and leading arguments that run this
	// binary's CLI; tests replace it with the test binary's helper.
	detachExecutable = func() (string, []string, error) {
		exe, err := os.Executable()
		return exe, nil, err
	}
)

// runRecord is the private run record (ATR-V0-010). The supervisor is its
// only writer and replaces it atomically.
type runRecord struct {
	Profile            string  `json:"profile"`
	RunID              string  `json:"runId"`
	AttemptID          string  `json:"attemptId"`
	Generation         string  `json:"generation"`
	ArgvSha256         string  `json:"argvSha256"`
	TimeoutSeconds     int     `json:"timeoutSeconds"`
	State              string  `json:"state"`
	LaunchedAt         string  `json:"launchedAt"`
	SupervisorPid      int     `json:"supervisorPid"`
	SupervisorIdentity string  `json:"supervisorIdentity"`
	CommandPid         *int    `json:"commandPid"`
	CommandIdentity    *string `json:"commandIdentity"`
	EndedAt            *string `json:"endedAt"`
	ExitStatus         *int    `json:"exitStatus"`
	ResultSha256       *string `json:"resultSha256"`
	OutputBytes        int64   `json:"outputBytes"`
	OutputDroppedBytes int64   `json:"outputDroppedBytes"`
}

func malformedRecord(msg string) error {
	return wire.Errorf(wire.CodeMalformed, "run record", "%s", msg)
}

func encodeRunRecord(rec *runRecord) ([]byte, error) {
	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(raw) > maxRunRecordBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "run record", "run record larger than %d bytes", maxRunRecordBytes)
	}
	return raw, nil
}

// decodeRunRecord is strict: one bounded object, no unknown keys, and facts
// that agree with the record's state.
func decodeRunRecord(raw []byte) (*runRecord, error) {
	if len(raw) > maxRunRecordBytes {
		return nil, malformedRecord("run record too large")
	}
	if err := wire.RawProfileVersion("run record/profile", raw, runRecordProfile); err != nil {
		return nil, err
	}
	if err := checkRunRecordKeys(raw); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var rec runRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, malformedRecord("run record does not decode: " + err.Error())
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, malformedRecord("run record has trailing data")
	}
	if err := wire.CheckProfile("run record/profile", rec.Profile, runRecordProfile); err != nil {
		return nil, err
	}
	switch {
	case !validRunID(rec.RunID) || rec.AttemptID == "" || rec.Generation == "" || !validSha256(rec.ArgvSha256):
		return nil, malformedRecord("run record does not name its run and attempt")
	case rec.TimeoutSeconds < 1 || rec.TimeoutSeconds > maxRunTimeoutSeconds || rec.LaunchedAt == "":
		return nil, malformedRecord("run record has no valid timeout or launch time")
	case rec.SupervisorPid <= 0 || rec.SupervisorIdentity == "":
		return nil, malformedRecord("run record has no supervisor identity")
	case rec.OutputBytes < 0 || rec.OutputDroppedBytes < 0 || rec.OutputDroppedBytes > rec.OutputBytes:
		return nil, malformedRecord("run record output totals disagree")
	case rec.CommandPid != nil && *rec.CommandPid <= 0, rec.CommandIdentity != nil && rec.CommandPid == nil:
		return nil, malformedRecord("run record names no valid command")
	}
	finished := rec.EndedAt != nil || rec.ExitStatus != nil || rec.ResultSha256 != nil
	switch rec.State {
	case runStarting:
		if rec.CommandPid != nil || finished {
			return nil, malformedRecord("a starting record names a command or a result")
		}
	case runRunning:
		if rec.CommandPid == nil || finished {
			return nil, malformedRecord("a running record names no command, or a result")
		}
	case runFinished:
		if rec.EndedAt == nil || rec.ExitStatus == nil || rec.ResultSha256 == nil || *rec.ExitStatus < 0 || *rec.ExitStatus > 255 || !validSha256(*rec.ResultSha256) {
			return nil, malformedRecord("a finished record lacks a valid result")
		}
	default:
		return nil, malformedRecord("unknown run state")
	}
	return &rec, nil
}

// runRecordKeys are the record's keys, exactly as encodeRunRecord writes
// them.
var runRecordKeys = func() map[string]bool {
	keys := map[string]bool{}
	t := reflect.TypeOf(runRecord{})
	for i := 0; i < t.NumField(); i++ {
		keys[t.Field(i).Tag.Get("json")] = true
	}
	return keys
}()

// checkRunRecordKeys requires one object holding each record key exactly
// once, spelled exactly, with a scalar value. encoding/json alone accepts a
// repeated key (the last wins), matches keys case-insensitively and leaves a
// missing one zero (ATR-V0-010).
func checkRunRecordKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return malformedRecord("run record is not an object")
	}
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return malformedRecord("run record does not decode: " + err.Error())
		}
		key, _ := tok.(string)
		if !runRecordKeys[key] || seen[key] {
			return malformedRecord(fmt.Sprintf("run record has an unknown or repeated key %q", key))
		}
		seen[key] = true
		if tok, err = dec.Token(); err != nil {
			return malformedRecord("run record does not decode: " + err.Error())
		}
		if _, nested := tok.(json.Delim); nested {
			return malformedRecord(fmt.Sprintf("run record key %q is not a scalar", key))
		}
	}
	if len(seen) != len(runRecordKeys) {
		return malformedRecord("run record lacks a key")
	}
	return nil
}

func validSha256(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

// writeRunFile replaces name in dir through a temporary file and a rename,
// so a reader sees the old or the new bytes, never a mixture.
func writeRunFile(dir, name string, raw []byte) error {
	tmp := filepath.Join(dir, name+".tmp")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}

func writeRunRecord(dir string, rec *runRecord) error {
	raw, err := encodeRunRecord(rec)
	if err != nil {
		return err
	}
	return writeRunFile(dir, runRecordName, raw)
}

// readBounded reads a run file of at most limit bytes. It opens without
// blocking or following a final symlink and refuses anything but a regular
// file, so a FIFO put in a record's place cannot stall a reader before its
// deadline.
func readBounded(p string, limit int64) ([]byte, error) {
	f, err := openRunFile(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil {
		return nil, err
	} else if !st.Mode().IsRegular() {
		return nil, wire.Errorf(wire.CodeMalformed, filepath.Base(p), "%s is not a regular file", filepath.Base(p))
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, wire.Errorf(wire.CodeLimitExceeded, filepath.Base(p), "%s larger than %d bytes", filepath.Base(p), limit)
	}
	return raw, nil
}

func readRunRecord(dir string) (*runRecord, error) {
	raw, err := readBounded(filepath.Join(dir, runRecordName), maxRunRecordBytes)
	if err != nil {
		return nil, err
	}
	return decodeRunRecord(raw)
}

// listRuns reads at most maxRunsPerAttempt+1 entries of an attempt's run
// directory, so a directory filled by another writer is never read whole.
func listRuns(base string) ([]os.DirEntry, error) {
	f, err := os.Open(base)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(maxRunsPerAttempt + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return entries, nil
}

func validRunID(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// runsDir is the attempt's private run directory beside the journal, never
// inside it (ATR-V0-010).
func runsDir(repo *intent.Repository, attemptID string) string {
	sum := sha256.Sum256([]byte(attemptID))
	return filepath.Join(repo.CommonDir, "taskman-runs", hex.EncodeToString(sum[:16]))
}

func flagBeforeDelimiter(args []string, flag string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == flag {
			return true
		}
	}
	return false
}

func supervising(args []string) bool { return flagBeforeDelimiter(args, "--supervise") }
func attachMode(args []string) bool  { return flagBeforeDelimiter(args, "--attach") }

// runOutput keeps the command's output in at most two segments of
// runSegmentBytes and counts what it drops (ATR-V0-010). It never fails a
// write, so a full disk cannot stop the command.
type runOutput struct {
	mu      sync.Mutex
	dir     string
	f       *os.File
	cur     int64
	prev    int64
	total   int64
	dropped int64
}

func openRunOutput(dir string) (*runOutput, error) {
	f, err := os.OpenFile(filepath.Join(dir, runOutputName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	return &runOutput{dir: dir, f: f}, nil
}

func (o *runOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := len(p)
	o.total += int64(n)
	for len(p) > 0 && o.f != nil {
		if o.cur >= runSegmentBytes && !o.rotate() {
			break
		}
		k := runSegmentBytes - o.cur
		if int64(len(p)) < k {
			k = int64(len(p))
		}
		w, err := o.f.Write(p[:k])
		o.cur += int64(w)
		p = p[w:]
		if err != nil {
			break
		}
	}
	o.dropped += int64(len(p))
	return n, nil
}

// rotate makes the current segment the previous one; the segment it replaces
// is counted as dropped. A failure stops keeping output.
func (o *runOutput) rotate() bool {
	_ = o.f.Close()
	o.f = nil
	if err := os.Rename(filepath.Join(o.dir, runOutputName), filepath.Join(o.dir, runOutputPrevName)); err != nil {
		return false
	}
	o.dropped += o.prev
	o.prev, o.cur = o.cur, 0
	f, err := os.OpenFile(filepath.Join(o.dir, runOutputName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	o.f = f
	return true
}

func (o *runOutput) totals() (int64, int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.total, o.dropped
}

func (o *runOutput) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.f == nil {
		return nil
	}
	err := o.f.Close()
	o.f = nil
	return err
}

func (o *runOutput) describe(item *wire.Object) {
	total, dropped := o.totals()
	item.Set("output", wire.String(filepath.Join(o.dir, runOutputName)))
	item.Set("outputBytes", wire.String(strconv.FormatInt(total, 10))).Set("outputDroppedBytes", wire.String(strconv.FormatInt(dropped, 10)))
}

// launch is `run --attempt ... --detach -- COMMAND...` (ATR-V0-008): it
// starts a supervisor in a new session, waits only until the supervisor has
// started the command or finished, and returns without waiting for it.
func (r *attemptRunner) launch(argv []string) int {
	cmd := []string{"run"}
	if !detachAvailable {
		return emit(r.env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeUnsupported}, Warnings: []string{"a detached attempt run needs a new session, which this platform does not provide; nothing was written or started"}})
	}
	base := runsDir(r.repo, r.attemptID)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return emit(r.env.Stdout, errorResult(cmd, err))
	}
	tooMany := func(n int) int {
		return emit(r.env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeLimitExceeded}, Warnings: []string{prose("the attempt already has " + strconv.Itoa(n) + " or more detached runs in " + base + "; nothing was started")}})
	}
	entries, err := listRuns(base)
	if err != nil {
		return emit(r.env.Stdout, errorResult(cmd, err))
	}
	if len(entries) >= maxRunsPerAttempt {
		return tooMany(len(entries))
	}
	dir := filepath.Join(base, r.runID)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return emit(r.env.Stdout, errorResult(cmd, err))
	}
	started := false
	defer func() {
		if !started {
			_ = os.RemoveAll(dir)
		}
	}()
	// Count again with this run reserved, so concurrent launchers never keep
	// more than maxRunsPerAttempt runs: each one that sees too many gives up
	// its own reservation.
	if entries, err = listRuns(base); err != nil {
		return emit(r.env.Stdout, errorResult(cmd, err))
	}
	if len(entries) > maxRunsPerAttempt {
		return tooMany(len(entries) - 1)
	}
	logFile, err := os.OpenFile(filepath.Join(dir, runSupervisorLogName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return emit(r.env.Stdout, errorResult(cmd, err))
	}
	defer logFile.Close()
	readyR, readyW, err := os.Pipe()
	if err != nil {
		return emit(r.env.Stdout, errorResult(cmd, err))
	}
	program, prefix, err := detachExecutable()
	if err != nil {
		_ = readyR.Close()
		_ = readyW.Close()
		return emit(r.env.Stdout, errorResult(cmd, err))
	}
	args := append(append([]string{}, prefix...), "run", "--attempt", r.attemptID, "--generation", string(r.generation),
		"--timeout", strconv.Itoa(int(r.timeout/time.Second)), "--lease-minutes", strconv.FormatUint(r.minutes, 10),
		"--role", r.role, "--supervise", r.runID, "--")
	c := exec.Command(program, append(args, argv...)...)
	c.Dir = r.env.Cwd
	c.Stderr = logFile
	c.ExtraFiles = []*os.File{readyW}
	c.SysProcAttr = detachAttr()
	err = c.Start()
	_ = readyW.Close()
	if err != nil {
		_ = readyR.Close()
		return emit(r.env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeError, Codes: []string{wire.CodeNoexec}, Warnings: []string{prose("the detached supervisor did not start: " + err.Error() + "; nothing was written or started")}})
	}
	started = true
	// Reap the supervisor if it ends while this launcher still lives.
	exited := make(chan struct{})
	go func() { _ = c.Wait(); close(exited) }()
	ready := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, readyR); _ = readyR.Close(); close(ready) }()
	bound := time.NewTimer(runReadyBound)
	defer bound.Stop()
	select {
	case <-ready:
	case <-bound.C:
	}
	rec, err := readRunRecord(dir)
	if err == nil && rec.State == runFinished {
		return replayRun(r.env, dir, rec)
	}
	if err == nil && rec.State == runRunning && supervisorStillLive(exited, rec) {
		res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Warnings: []string{}, NotRetryable: true}
		res.Items = []wire.Value{runItem(rec, dir, r.attemptID, r.runID)}
		return emitCode(r.env.Stdout, res, 0)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
	}
	select {
	case <-exited:
		if again, err := readRunRecord(dir); err == nil && again.State == runFinished {
			return replayRun(r.env, dir, again)
		}
		return emit(r.env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeError, Codes: []string{wire.CodeSupervisorLost}, NotRetryable: true,
			Warnings: []string{prose("the detached supervisor ended without a result; see " + filepath.Join(dir, runSupervisorLogName))}, Items: []wire.Value{runItem(rec, dir, r.attemptID, r.runID)}})
	default:
	}
	return emitCode(r.env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, NotRetryable: true,
		Warnings: []string{"the detached supervisor has not reported the command started; it was not stopped; attach later with run --attach"}, Items: []wire.Value{runItem(rec, dir, r.attemptID, r.runID)}}, runExitPending)
}

// supervisorStillLive reports whether the launcher's supervisor has neither
// exited nor lost its recorded identity, so a RUNNING record is not reported
// for a supervisor that died after writing it (ATR-V0-008).
func supervisorStillLive(exited <-chan struct{}, rec *runRecord) bool {
	select {
	case <-exited:
		return false
	default:
	}
	live, err := processLive(rec.SupervisorPid, rec.SupervisorIdentity)
	return err == nil && live
}

// superviseRun is the launcher's re-executed supervisor (ATR-V0-009): the
// attached runner with its envelope and output kept in the run directory.
func (r *attemptRunner) superviseRun(argv []string) int {
	diag := r.env.Stderr
	fail := func(err error) int {
		_, _ = fmt.Fprintln(diag, "corvint-tasks run supervisor: "+err.Error())
		return 1
	}
	if !detachAvailable {
		return fail(errors.New("detached runs are unsupported on this platform"))
	}
	ready := os.NewFile(readinessFD, "readiness")
	var once sync.Once
	signalReady := func() {
		once.Do(func() {
			if ready != nil {
				_ = ready.Close()
			}
		})
	}
	defer signalReady()
	dir := filepath.Join(runsDir(r.repo, r.attemptID), r.runID)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return fail(fmt.Errorf("run directory %s is missing", dir))
	}
	if _, err := os.Lstat(filepath.Join(dir, runRecordName)); !errors.Is(err, fs.ErrNotExist) {
		return fail(fmt.Errorf("run %s already has a record", r.runID))
	}
	ident, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil || ident == "" {
		return fail(fmt.Errorf("own process identity unavailable: %v", err))
	}
	out, err := openRunOutput(dir)
	if err != nil {
		return fail(err)
	}
	defer out.Close()
	sum := sha256.Sum256([]byte(strings.Join(argv, "\x00")))
	rec := &runRecord{Profile: runRecordProfile, RunID: r.runID, AttemptID: r.attemptID, Generation: string(r.generation), ArgvSha256: hex.EncodeToString(sum[:]),
		TimeoutSeconds: int(r.timeout / time.Second), State: runStarting, LaunchedAt: string(nowStamp()), SupervisorPid: os.Getpid(), SupervisorIdentity: ident}
	if err := writeRunRecord(dir, rec); err != nil {
		return fail(err)
	}
	var envelope bytes.Buffer
	r.env.Stdout, r.env.Stderr = &envelope, out
	r.output = out
	// The RUNNING record is written off the runner's goroutine, so record I/O
	// never delays its timeout, heartbeats or signal handling (ATR-V0-013).
	var running chan struct{}
	r.onStart = func(pid int) {
		rec.State, rec.CommandPid = runRunning, &pid
		if id, err := supervisor.ProcessIdentity(pid); err == nil && id != "" {
			rec.CommandIdentity = &id
		}
		snapshot := *rec
		running = make(chan struct{})
		go func() {
			defer close(running)
			if err := writeRunRecord(dir, &snapshot); err != nil {
				_, _ = fmt.Fprintln(diag, "corvint-tasks run supervisor: running record not written: "+err.Error())
			}
			signalReady()
		}()
	}
	status := r.run(argv)
	if running != nil {
		// Never let the RUNNING write land after the FINISHED one.
		<-running
	}
	_ = out.Close()
	raw := envelope.Bytes()
	digest := hex.EncodeToString(sha256Of(raw))
	if err := writeRunFile(dir, runResultName, raw); err != nil {
		return fail(fmt.Errorf("result not kept: %w", err))
	}
	ended := string(nowStamp())
	rec.State, rec.EndedAt, rec.ExitStatus, rec.ResultSha256 = runFinished, &ended, &status, &digest
	rec.OutputBytes, rec.OutputDroppedBytes = out.totals()
	if err := writeRunRecord(dir, rec); err != nil {
		return fail(fmt.Errorf("finished record not written: %w", err))
	}
	return status
}

func sha256Of(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}

// runItem describes a run that has not finished, or whose record is absent.
func runItem(rec *runRecord, dir, attemptID, runID string) wire.Value {
	item := wire.NewObject()
	item.Set("runId", wire.String(runID)).Set("attemptId", wire.String(attemptID))
	item.Set("runDir", wire.String(dir)).Set("output", wire.String(filepath.Join(dir, runOutputName)))
	if rec == nil {
		item.Set("state", wire.Null())
		return wire.ObjectValue(item)
	}
	item.Set("generation", wire.String(rec.Generation)).Set("state", wire.String(rec.State)).Set("launchedAt", wire.String(rec.LaunchedAt))
	item.Set("supervisorPid", wire.String(strconv.Itoa(rec.SupervisorPid))).Set("commandPid", intOrNull(rec.CommandPid))
	return wire.ObjectValue(item)
}

// replayRun prints a finished run's kept envelope after checking it against
// its record, and returns the run's exit status (ATR-V0-011).
func replayRun(env Env, dir string, rec *runRecord) int {
	cmd := []string{"run"}
	raw, err := readBounded(filepath.Join(dir, runResultName), maxRunResultBytes)
	if err != nil {
		return emit(env.Stdout, errorResult(cmd, err))
	}
	if hex.EncodeToString(sha256Of(raw)) != *rec.ResultSha256 {
		return emit(env.Stdout, errorResult(cmd, malformedRecord("the kept result does not match its record")))
	}
	if _, err := wire.DecodeResult(raw); err != nil {
		return emit(env.Stdout, errorResult(cmd, err))
	}
	_, _ = env.Stdout.Write(raw)
	return *rec.ExitStatus
}

// processLive reports whether pid is still the process recorded with
// identity; a PID alone is never trusted (ATR-V0-012).
func processLive(pid int, identity string) (bool, error) {
	id, err := supervisor.ProcessIdentity(pid)
	if err != nil {
		return false, err
	}
	return id != "" && id == identity, nil
}

// attemptAttach is `run --attach --attempt ID [--run RUN] [--wait SECONDS]`
// (ATR-V0-011, ATR-V0-012). It only reads: it never writes a lease, a record
// or a signal.
func attemptAttach(env Env, args []string) int {
	cmd := []string{"run"}
	values := map[string]string{}
	attach := false
	for i := 0; i < len(args); {
		switch args[i] {
		case "--attach":
			if attach {
				return emit(env.Stdout, usage(cmd, "repeated run flag --attach"))
			}
			attach = true
			i++
			continue
		case "--attempt", "--run", "--wait":
		default:
			return emit(env.Stdout, usage(cmd, "run --attach accepts only --attempt, --run and --wait"))
		}
		if i+1 >= len(args) {
			return emit(env.Stdout, usage(cmd, "missing value for "+args[i]))
		}
		if _, seen := values[args[i]]; seen {
			return emit(env.Stdout, usage(cmd, "repeated run flag "+args[i]))
		}
		values[args[i]] = args[i+1]
		i += 2
	}
	attemptID, runID := values["--attempt"], values["--run"]
	if attemptID == "" {
		return emit(env.Stdout, usage(cmd, "run --attach requires --attempt"))
	}
	if _, set := values["--run"]; set && !validRunID(runID) {
		return emit(env.Stdout, usage(cmd, "--run must be a 16-hex run ID"))
	}
	wait := -1
	if w, set := values["--wait"]; set {
		n, err := strconv.ParseUint(w, 10, 32)
		if err != nil || n > maxAttachWait {
			return emit(env.Stdout, usage(cmd, "--wait must be whole seconds in 0.."+strconv.Itoa(maxAttachWait)))
		}
		wait = int(n)
	}
	if !detachAvailable {
		return emit(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeUnsupported}, Warnings: []string{"detached attempt runs are unsupported on this platform"}})
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return emit(env.Stdout, errorResult(cmd, err))
	}
	observed, err := snapshot.Probe(repo.StateDir)
	if err != nil {
		return emit(env.Stdout, errorResult(cmd, err))
	}
	if aq, err := snapshot.AttemptQueue(attemptID); err != nil || aq.Raw != observed.Head.QueueID.Raw {
		return emit(env.Stdout, errorResult(cmd, wire.Errorf(wire.CodeMalformed, "attempt", "--attempt must name an attempt of this queue")))
	}
	base := runsDir(repo, attemptID)
	if runID == "" {
		var res *wire.Result
		if runID, res = soleRun(base, attemptID); res != nil {
			return emit(env.Stdout, res)
		}
	}
	dir := filepath.Join(base, runID)
	deadline := time.Now().Add(time.Duration(wait) * time.Second)
	for {
		rec, err := readRunRecord(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return emit(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeMissingEvidence}, Warnings: []string{prose("no record for detached run " + runID + " of this attempt in " + base)}})
		}
		if err != nil {
			return emit(env.Stdout, errorResult(cmd, err))
		}
		if rec.AttemptID != attemptID || rec.RunID != runID {
			return emit(env.Stdout, errorResult(cmd, malformedRecord("the run record names another run or attempt")))
		}
		if rec.State == runFinished {
			return replayRun(env, dir, rec)
		}
		live, err := processLive(rec.SupervisorPid, rec.SupervisorIdentity)
		if err != nil {
			return emit(env.Stdout, errorResult(cmd, err))
		}
		if !live {
			// It may have finished between the two reads.
			if again, err := readRunRecord(dir); err == nil && again.State == runFinished && again.RunID == runID && again.AttemptID == attemptID {
				return replayRun(env, dir, again)
			}
			return emit(env.Stdout, supervisorLost(rec, dir, attemptID, runID))
		}
		if wait >= 0 && !time.Now().Before(deadline) {
			return emitCode(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, NotRetryable: true,
				Warnings: []string{"the detached run has not finished; nothing was stopped; attach again later"}, Items: []wire.Value{runItem(rec, dir, attemptID, runID)}}, runExitPending)
		}
		time.Sleep(runPoll)
	}
}

// supervisorLost reports a supervisor that is gone, or whose PID now names
// another process, before it kept a result. Nothing is signalled.
func supervisorLost(rec *runRecord, dir, attemptID, runID string) *wire.Result {
	warnings := []string{prose("the detached run's supervisor (pid " + strconv.Itoa(rec.SupervisorPid) + ") is gone or its process identity changed before it kept a result; nothing was signalled; a RUN_OUTCOME may still be in the journal")}
	if rec.CommandPid != nil && rec.CommandIdentity != nil {
		if live, err := processLive(*rec.CommandPid, *rec.CommandIdentity); err == nil && live {
			warnings = append(warnings, prose("the command (pid "+strconv.Itoa(*rec.CommandPid)+") is still running unsupervised; it was not signalled"))
		}
	}
	return &wire.Result{Command: []string{"run"}, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeSupervisorLost}, NotRetryable: true, Warnings: warnings, Items: []wire.Value{runItem(rec, dir, attemptID, runID)}}
}

// soleRun picks the attempt's only detached run; none, or several, refuse.
func soleRun(base, attemptID string) (string, *wire.Result) {
	cmd := []string{"run"}
	entries, err := listRuns(base)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", errorResult(cmd, err)
	}
	if len(entries) > maxRunsPerAttempt {
		return "", &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeLimitExceeded}, Warnings: []string{prose("more than " + strconv.Itoa(maxRunsPerAttempt) + " entries in " + base + "; name a run with --run")}}
	}
	var runs []string
	for _, e := range entries {
		if e.IsDir() && validRunID(e.Name()) {
			runs = append(runs, e.Name())
		}
	}
	sort.Strings(runs)
	switch len(runs) {
	case 0:
		return "", &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeMissingEvidence}, Warnings: []string{"no detached run of this attempt"}}
	case 1:
		return runs[0], nil
	}
	items := make([]wire.Value, 0, len(runs))
	for _, id := range runs {
		rec, _ := readRunRecord(filepath.Join(base, id))
		items = append(items, runItem(rec, filepath.Join(base, id), attemptID, id))
	}
	return "", &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Warnings: []string{"the attempt has several detached runs; name one with --run"}, Items: items}
}
