package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Detached claim and pool acquire (CAL-V0-320..329, issue 1005). `--detach`
// starts the unchanged lease command in its own session under the caller's
// request ID and returns at once (or after --wait); `--attach` reads its
// kept result. The request ID is the handle. The store's request-ID replay
// keeps the mutation exactly once; the private record beside the journal only
// keeps the child's identity and its result bytes, never authority.

const (
	leaseDetachProfile    = "taskman-detached-lease/0"
	leaseDetachBase       = "taskman-requests"
	leaseDetachRequest    = "request.json"
	leaseDetachChildName  = "child.json"
	leaseDetachResultName = "result.json"
	leaseDetachLogName    = "child.log"
	leaseDetachPrevLog    = "child.1.log"
	leaseDetachLockName   = "lock"
	leaseDetachRunName    = "running"
	leaseDetachChildFlag  = "--detached-child"

	// maxLeaseDetachWait is the largest --wait, in seconds.
	maxLeaseDetachWait = 600
	// leaseDetachRetention: a finished record older than this is removed
	// when a later detach starts (CAL-V0-325).
	leaseDetachRetention = 7 * 24 * time.Hour
	maxLeaseDetachRecord = 4096
	maxLeaseDetachResult = 4 << 20
	// maxLeaseDetachLog caps what a child writes to its child.log; the
	// previous run's log is kept as child.1.log (CAL-V0-328).
	maxLeaseDetachLog = 64 << 10
	// leaseDetachPruneBatch and maxLeaseDetachPruneScan page the retention
	// scan through the whole records directory, bounded overall (CAL-V0-329).
	leaseDetachPruneBatch   = 256
	maxLeaseDetachPruneScan = 1 << 16
)

var (
	// leaseDetachCeiling bounds the detached child's lease command
	// (CAL-V0-323); it exceeds the 300-second --lock-wait maximum.
	leaseDetachCeiling = 600 * time.Second
	// leaseDetachPoll separates a detach or attach wait's record reads.
	leaseDetachPoll = 200 * time.Millisecond
	// leaseDetachLockBound bounds the wait for another caller's launch.
	leaseDetachLockBound = 10 * time.Second
	// leaseDetachStartBound is the startup window in which a recorded child
	// whose identity cannot be confirmed is still taken as starting
	// (CAL-V0-327).
	leaseDetachStartBound = 60 * time.Second
	// leaseDetachIdentity probes a process identity; tests make it fail.
	leaseDetachIdentity = supervisor.ProcessIdentity
	// leaseDetachChildStarting runs before a child takes its running lock,
	// with the child's capped log; tests delay a start and write there.
	leaseDetachChildStarting = func(io.Writer) {}
)

// leaseDetachHelp is the claim and pool acquire help member (CAL-V0-320).
const leaseDetachHelp = "--detach runs the unchanged command in its own session under --request-id (the handle) and returns at once, or after --wait SECONDS (0..600): the command's own result, or REFUSED with state PENDING and exit 75. --attach --request-id ID [--wait SECONDS] only reads: the kept result byte for byte (CLAIMED with attemptId and generation, or the refusal verbatim), PENDING, or SUPERVISOR_LOST when the process ended without a result. A repeated --detach with the same arguments never starts a second mutation; after LOST it restarts and the store replays a committed request. The detached process is bounded by a 600-second ceiling (hard stop at 660). Darwin and Linux only. Records live in taskman-requests beside the journal; finished ones older than 7 days are pruned."

// leaseDetachMode is the parsed detach, attach or child flag set.
type leaseDetachMode struct {
	detach, attach, child bool
	wait                  int
	rest                  []string
}

// leaseDetachRequested reports whether a claim or pool acquire argv carries
// a detach flag; without one the command is the unchanged lease command
// (CAL-V0-326).
func leaseDetachRequested(args []string) bool {
	_, found, _ := splitLeaseDetach(args)
	return found
}

// splitLeaseDetach removes --detach, --attach, --wait N and the internal
// child flag, skipping every lease flag's value so a value is never read as
// a flag.
func splitLeaseDetach(args []string) (leaseDetachMode, bool, error) {
	m := leaseDetachMode{wait: -1}
	found := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--detach" || a == "--attach" || a == leaseDetachChildFlag:
			found = true
			flag := map[string]*bool{"--detach": &m.detach, "--attach": &m.attach, leaseDetachChildFlag: &m.child}[a]
			if *flag {
				return m, found, wire.Errorf(wire.CodeMalformed, "argv", "repeated %s", a)
			}
			*flag = true
		case a == "--wait":
			found = true
			if m.wait >= 0 || i+1 >= len(args) {
				return m, found, wire.Errorf(wire.CodeMalformed, "argv", "flag --wait is repeated or has no value")
			}
			i++
			n, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil || n > maxLeaseDetachWait || (len(args[i]) > 1 && args[i][0] == '0') {
				return m, found, wire.Errorf(wire.CodeMalformed, "argv", "--wait takes whole seconds from 0 to %d", maxLeaseDetachWait)
			}
			m.wait = int(n)
		case leaseValueFlags[a] || a == "--exclude-member" || a == "--repo":
			m.rest = append(m.rest, a)
			if i+1 < len(args) {
				i++
				m.rest = append(m.rest, args[i])
			}
		default:
			m.rest = append(m.rest, a)
		}
	}
	return m, found, nil
}

// leaseDetachRecord is request.json: what the detached request is.
type leaseDetachRecord struct {
	Profile    string `json:"profile"`
	Command    string `json:"command"`
	RequestID  string `json:"requestId"`
	Holder     string `json:"holder"`
	ArgvSha256 string `json:"argvSha256"`
}

// leaseDetachChild is child.json: the detached process and its bound.
type leaseDetachChild struct {
	Pid       int    `json:"pid"`
	Identity  string `json:"identity"`
	StartedAt string `json:"startedAt"`
	CeilingAt string `json:"ceilingAt"`
}

func leaseDetachDir(repo *intent.Repository, requestID string) string {
	sum := sha256.Sum256([]byte(requestID))
	return filepath.Join(repo.CommonDir, leaseDetachBase, hex.EncodeToString(sum[:]))
}

func leaseDetachArgvDigest(name string, rest []string) string {
	sum := sha256.Sum256([]byte(name + "\x00" + strings.Join(rest, "\x00")))
	return hex.EncodeToString(sum[:])
}

// readLeaseDetachJSON decodes one closed record file of at most
// maxLeaseDetachRecord bytes. A request.json another build wrote under
// another taskman-detached-lease version is UNSUPPORTED_VERSION (CAL-V0-131);
// child.json carries no profile.
func readLeaseDetachJSON(p string, into any) error {
	raw, err := readBounded(p, maxLeaseDetachRecord)
	if err != nil {
		return err
	}
	if err := wire.RawProfileVersion("/profile", raw, leaseDetachProfile); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return malformedRecord("detached request record " + filepath.Base(p) + " does not decode: " + err.Error())
	}
	return nil
}

func writeLeaseDetachJSON(dir, name string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeRunFile(dir, name, append(raw, '\n'))
}

// leaseDetach is `claim|pool acquire ... --detach [--wait N]`,
// `... --attach --request-id ID [--holder H] [--wait N]` and the internal
// child (CAL-V0-320..329).
func leaseDetach(env Env, name string, args []string) int {
	cmd := strings.Fields(name)
	m, _, err := splitLeaseDetach(args)
	if err != nil {
		return emit(env.Stdout, errorResult(cmd, err))
	}
	modes := 0
	for _, on := range []bool{m.detach, m.attach, m.child} {
		if on {
			modes++
		}
	}
	if modes != 1 || (m.child && m.wait >= 0) {
		return emit(env.Stdout, usage(cmd, "--wait needs exactly one of --detach or --attach"))
	}
	parsed, err := parseLeaseArgs(m.rest)
	if err != nil {
		return emit(env.Stdout, errorResult(cmd, err))
	}
	requestID, holder := parsed.values["--request-id"], parsed.values["--holder"]
	if _, err := mutation.ParseRequestID("requestId", requestID); err != nil {
		return emit(env.Stdout, errorResult(cmd, err))
	}
	if m.attach {
		for k := range parsed.values {
			if k != "--request-id" && k != "--holder" {
				return emit(env.Stdout, usage(cmd, name+" --attach takes only --request-id, --holder and --wait"))
			}
		}
		if len(parsed.pos)+len(parsed.scope)+len(parsed.excluded)+len(parsed.repos) > 0 || parsed.next || parsed.timing || parsed.whole || parsed.laneUntouched || parsed.authors != "" {
			return emit(env.Stdout, usage(cmd, name+" --attach takes only --request-id, --holder and --wait"))
		}
	}
	if !detachAvailable {
		return emit(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeUnsupported}, Warnings: []string{"a detached " + name + " needs a new session, which this platform does not provide; nothing was written or started"}})
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return emit(env.Stdout, errorResult(cmd, err))
	}
	dir := leaseDetachDir(repo, requestID)
	want := leaseDetachRecord{Profile: leaseDetachProfile, Command: name, RequestID: requestID, Holder: holder, ArgvSha256: leaseDetachArgvDigest(name, m.rest)}
	switch {
	case m.child:
		return leaseDetachRunChild(env, name, dir, want, m.rest)
	case m.detach:
		if res := leaseDetachLaunch(env, name, dir, want, m.rest); res != nil {
			return emit(env.Stdout, res)
		}
	default:
		var rec leaseDetachRecord
		if err := readLeaseDetachJSON(filepath.Join(dir, leaseDetachRequest), &rec); errors.Is(err, fs.ErrNotExist) {
			return emit(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeMissingEvidence}, Warnings: []string{prose("no detached " + name + " for request id " + requestID + " in " + filepath.Dir(dir))}})
		} else if err != nil {
			return emit(env.Stdout, errorResult(cmd, err))
		} else if rec.Profile != leaseDetachProfile || rec.Command != name || rec.RequestID != requestID || (holder != "" && rec.Holder != holder) {
			return emit(env.Stdout, leaseDetachConflict(cmd, requestID))
		}
	}
	return leaseDetachAwait(env, name, dir, requestID, holder, m.wait)
}

func leaseDetachConflict(cmd []string, requestID string) *wire.Result {
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeRequestIDConflict}, Warnings: []string{prose("request id " + requestID + " already names a detached request with other arguments, holder or command; nothing was started")}}
}

// leaseDetachLaunch records the request and starts its child unless a live
// child or a kept result already answers it (CAL-V0-321). It returns nil
// when the caller should wait for the result.
func leaseDetachLaunch(env Env, name, dir string, want leaseDetachRecord, rest []string) *wire.Result {
	cmd := strings.Fields(name)
	base := filepath.Dir(dir)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return errorResult(cmd, err)
	}
	pruneLeaseDetach(base, time.Now())
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return errorResult(cmd, err)
	}
	unlock, err := lockLeaseDetach(filepath.Join(dir, leaseDetachLockName), leaseDetachLockBound)
	if err != nil {
		return errorResult(cmd, err)
	}
	defer unlock()
	var rec leaseDetachRecord
	switch err := readLeaseDetachJSON(filepath.Join(dir, leaseDetachRequest), &rec); {
	case errors.Is(err, fs.ErrNotExist):
		if err := writeLeaseDetachJSON(dir, leaseDetachRequest, want); err != nil {
			return errorResult(cmd, err)
		}
	case err != nil:
		return errorResult(cmd, err)
	case rec != want:
		return leaseDetachConflict(cmd, want.RequestID)
	}
	if _, err := os.Lstat(filepath.Join(dir, leaseDetachResultName)); err == nil {
		return nil
	}
	var child leaseDetachChild
	switch live, err := leaseDetachChildLive(dir, &child, time.Now()); {
	case err != nil:
		return errorResult(cmd, err)
	case live:
		return nil
	}
	// No result and no live child: start one. A child that died before
	// keeping a result is replaced; the store's request-ID replay returns
	// what it committed, so this never claims twice.
	program, prefix, err := detachExecutable()
	if err != nil {
		return errorResult(cmd, err)
	}
	// Each run gets a fresh child.log; the previous run's log, the evidence
	// of a lost child, is kept as child.1.log (CAL-V0-328).
	logPath := filepath.Join(dir, leaseDetachLogName)
	if err := os.Rename(logPath, filepath.Join(dir, leaseDetachPrevLog)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errorResult(cmd, err)
	}
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return errorResult(cmd, err)
	}
	defer logFile.Close()
	argv := append(append(append(append([]string{}, prefix...), cmd...), rest...), leaseDetachChildFlag)
	c := exec.Command(program, argv...)
	c.Dir = env.Cwd
	c.Stderr = logFile
	c.SysProcAttr = detachAttr()
	started := time.Now().UTC()
	if err := c.Start(); err != nil {
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeError, Codes: []string{wire.CodeNoexec}, Warnings: []string{prose("the detached " + name + " did not start: " + err.Error() + "; nothing was claimed")}}
	}
	// Reap the child if it ends while this caller still lives.
	go func() { _ = c.Wait() }()
	identity, err := leaseDetachIdentity(c.Process.Pid)
	if err != nil || identity == "" {
		identity = leaseDetachUnknownIdentity
	}
	rec2 := leaseDetachChild{Pid: c.Process.Pid, Identity: identity, StartedAt: started.Format(time.RFC3339), CeilingAt: started.Add(leaseDetachCeiling).Format(time.RFC3339)}
	if err := writeLeaseDetachJSON(dir, leaseDetachChildName, rec2); err != nil {
		return errorResult(cmd, err)
	}
	return nil
}

// leaseDetachUnknownIdentity is child.json's identity when the launcher
// could not read the child's.
const leaseDetachUnknownIdentity = "unknown"

// leaseDetachChildLive reports whether a child of this record may still run:
// it holds the record's running lock for its whole life, or, before it has
// taken that lock, its recorded process is live. Only positive evidence ends
// a recorded child: no process at its pid, a different identity there, or,
// when its identity cannot be confirmed, no running lock once the startup
// window has passed (CAL-V0-327). child receives the record.
func leaseDetachChildLive(dir string, child *leaseDetachChild, now time.Time) (bool, error) {
	if leaseDetachLockHeld(filepath.Join(dir, leaseDetachRunName)) {
		_ = readLeaseDetachJSON(filepath.Join(dir, leaseDetachChildName), child)
		return true, nil
	}
	switch err := readLeaseDetachJSON(filepath.Join(dir, leaseDetachChildName), child); {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	}
	id, err := leaseDetachIdentity(child.Pid)
	switch {
	case child.Pid <= 0 || (err == nil && id == ""):
		return false, nil // no such process
	case err == nil && child.Identity != leaseDetachUnknownIdentity:
		return id == child.Identity, nil // a different identity is a reused pid
	}
	// The identity is not yet known: a child still starting has not taken
	// running, so it is taken as live until the startup window has passed.
	started, perr := time.Parse(time.RFC3339, child.StartedAt)
	return perr == nil && now.Before(started.Add(leaseDetachStartBound)), nil
}

// leaseDetachRunChild is the detached child: the unchanged lease command
// under the ceiling, its exact result bytes kept for attach (CAL-V0-322,
// CAL-V0-323).
func leaseDetachRunChild(env Env, name, dir string, want leaseDetachRecord, rest []string) int {
	var rec leaseDetachRecord
	if err := readLeaseDetachJSON(filepath.Join(dir, leaseDetachRequest), &rec); err != nil || rec != want {
		return emit(env.Stdout, usage(strings.Fields(name), leaseDetachChildFlag+" is internal to a detached "+name))
	}
	// What the child itself writes to child.log is capped (CAL-V0-328).
	env.Stderr = &leaseDetachLogWriter{w: env.Stderr, left: maxLeaseDetachLog}
	leaseDetachChildStarting(env.Stderr)
	// One child at a time runs a record, and none after a kept result:
	// a launcher whose caller died before recording this child may have
	// started another.
	release, err := lockLeaseDetach(filepath.Join(dir, leaseDetachRunName), 0)
	if err != nil {
		return 1
	}
	defer release()
	if _, err := os.Lstat(filepath.Join(dir, leaseDetachResultName)); err == nil {
		return 0
	}
	// A child still running a minute past its ceiling ends without a
	// result; attach then reports it LOST.
	stop := time.AfterFunc(leaseDetachCeiling+time.Minute, func() { os.Exit(124) })
	defer stop.Stop()
	env.leaseDeadline = time.Now().Add(leaseDetachCeiling)
	var out bytes.Buffer
	code := emit(&out, leaseCommand(env, name, rest))
	if err := writeRunFile(dir, leaseDetachResultName, out.Bytes()); err != nil {
		_, _ = env.Stderr.Write([]byte("corvint-tasks detached " + name + ": " + err.Error() + "\n"))
		return 1
	}
	return code
}

// leaseDetachAwait answers from the kept result, or reports PENDING or LOST,
// waiting up to wait seconds for the result (CAL-V0-322, CAL-V0-324). It
// only reads.
func leaseDetachAwait(env Env, name, dir, requestID, holder string, wait int) int {
	cmd := strings.Fields(name)
	deadline := time.Now().Add(time.Duration(max(wait, 0)) * time.Second)
	for {
		raw, err := readBounded(filepath.Join(dir, leaseDetachResultName), maxLeaseDetachResult)
		if err == nil {
			res, err := wire.DecodeResult(raw)
			if err != nil {
				return emit(env.Stdout, errorResult(cmd, err))
			}
			_, _ = env.Stdout.Write(raw)
			if res.Outcome == wire.OutcomeOK {
				return 0
			}
			return 1
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return emit(env.Stdout, errorResult(cmd, err))
		}
		var child leaseDetachChild
		state := "PENDING"
		live, err := leaseDetachChildLive(dir, &child, time.Now())
		if err != nil {
			return emit(env.Stdout, errorResult(cmd, err))
		}
		// A launcher that still holds the lock is starting the child.
		if !live && !leaseDetachLockHeld(filepath.Join(dir, leaseDetachLockName)) {
			state = "LOST"
		}
		if state == "LOST" {
			if _, err := os.Lstat(filepath.Join(dir, leaseDetachResultName)); err == nil {
				continue // it finished between the two reads
			}
			return emit(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeSupervisorLost}, NotRetryable: true,
				Warnings: []string{prose("the detached " + name + " ended without keeping a result; nothing was signalled; repeat " + name + " --detach with the same arguments and request id: the store replays a committed request and otherwise runs it once")},
				Items:    []wire.Value{leaseDetachItem(name, dir, requestID, holder, state, &child)}})
		}
		if !time.Now().Before(deadline) {
			return emitCode(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused,
				Warnings: []string{prose("the detached " + name + " has not finished; attach again with " + name + " --attach --request-id " + requestID)},
				Items:    []wire.Value{leaseDetachItem(name, dir, requestID, holder, state, &child)}}, runExitPending)
		}
		time.Sleep(min(leaseDetachPoll, time.Until(deadline)))
	}
}

func leaseDetachItem(name, dir, requestID, holder, state string, child *leaseDetachChild) wire.Value {
	o := wire.NewObject().Set("requestId", wire.String(requestID)).Set("command", wire.String(name)).Set("holder", stringOrNull(holder)).Set("state", wire.String(state)).Set("record", wire.String(dir))
	if child != nil && child.Pid > 0 {
		o.Set("childPid", wire.String(strconv.Itoa(child.Pid))).Set("startedAt", wire.String(child.StartedAt)).Set("ceilingAt", wire.String(child.CeilingAt))
	} else {
		o.Set("childPid", wire.Null()).Set("startedAt", wire.Null()).Set("ceilingAt", wire.Null())
	}
	return wire.ObjectValue(o)
}

// pruneLeaseDetach removes finished records (a kept result) older than
// leaseDetachRetention; a pending or lost record is kept. It pages through
// the whole directory, at most maxLeaseDetachPruneScan entries, so unfinished
// records never hide finished ones behind them (CAL-V0-329). Best effort.
func pruneLeaseDetach(base string, now time.Time) {
	f, err := openDirNonblock(base)
	if err != nil {
		return
	}
	defer f.Close()
	var expired []string
	for scanned := 0; scanned < maxLeaseDetachPruneScan; {
		entries, err := f.ReadDir(min(leaseDetachPruneBatch, maxLeaseDetachPruneScan-scanned))
		for _, e := range entries {
			if !e.IsDir() || len(e.Name()) != sha256.Size*2 {
				continue
			}
			info, err := os.Lstat(filepath.Join(base, e.Name(), leaseDetachResultName))
			if err == nil && info.Mode().IsRegular() && now.Sub(info.ModTime()) > leaseDetachRetention {
				expired = append(expired, e.Name())
			}
		}
		scanned += len(entries)
		if err != nil || len(entries) == 0 {
			break
		}
	}
	// Removal waits for the scan so the open listing is never reordered.
	for _, name := range expired {
		_ = os.RemoveAll(filepath.Join(base, name))
	}
}

// leaseDetachLogWriter caps the bytes a detached child writes to its
// child.log; past the cap it reports success and drops the rest
// (CAL-V0-328).
type leaseDetachLogWriter struct {
	w    io.Writer
	left int
}

func (l *leaseDetachLogWriter) Write(p []byte) (int, error) {
	n := min(len(p), l.left)
	if n > 0 {
		l.left -= n
		_, _ = l.w.Write(p[:n])
	}
	return len(p), nil
}
