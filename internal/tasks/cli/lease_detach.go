package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Detached claim and pool acquire (ATR-V0-020..029, issue 1005). `--detach`
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
	leaseDetachPrevResult = "result.1.json"
	leaseDetachLockName   = "lock"
	leaseDetachRunName    = "running"
	// leaseDetachProgressName is the child's advisory round count and
	// audit time, rewritten after each preparation round (PSR-V0-112).
	leaseDetachProgressName = "progress.json"
	// leaseDetachWalksName is the same record with the PSR-V0-152 walk
	// split, a sibling no older build opens, so progress.json keeps the
	// closed shape older readers decode (PSR-V0-152).
	leaseDetachWalksName = "progress-walks.json"
	leaseDetachChildFlag = "--detached-child"

	// maxLeaseDetachWait is the largest --wait, in seconds.
	maxLeaseDetachWait = 600
	// leaseDetachRetention: a finished record older than this is removed
	// when a later detach starts (ATR-V0-025).
	leaseDetachRetention = 7 * 24 * time.Hour
	maxLeaseDetachRecord = 4096
	maxLeaseDetachResult = 4 << 20
	// maxLeaseDetachLog caps what a child writes to its child.log; the
	// previous run's log is kept as child.1.log (ATR-V0-028).
	maxLeaseDetachLog = 64 << 10
	// leaseDetachPruneBatch and maxLeaseDetachPruneScan page the retention
	// scan through the whole records directory, bounded overall (ATR-V0-029).
	leaseDetachPruneBatch   = 256
	maxLeaseDetachPruneScan = 1 << 16
)

var (
	// leaseDetachCeiling bounds the detached child's lease command
	// (ATR-V0-023); it exceeds the 300-second --lock-wait maximum.
	leaseDetachCeiling = 600 * time.Second
	// leaseDetachPoll separates a detach or attach wait's record reads.
	leaseDetachPoll = 200 * time.Millisecond
	// leaseDetachLockBound bounds the wait for another caller's launch.
	leaseDetachLockBound = 10 * time.Second
	// leaseDetachStartBound is the startup window in which a recorded child
	// whose identity cannot be confirmed is still taken as starting
	// (ATR-V0-027).
	leaseDetachStartBound = 60 * time.Second
	// leaseDetachIdentity probes a process identity; tests make it fail.
	leaseDetachIdentity = supervisor.ProcessIdentity
	// leaseDetachChildStarting runs before a child takes its running lock,
	// with the child's capped log; tests delay a start and write there.
	leaseDetachChildStarting = func(io.Writer) {}
)

// leaseDetachHelp is the claim and pool acquire help member (ATR-V0-020).
const leaseDetachHelp = "--detach runs the unchanged command in its own session under --request-id (the handle) and returns at once, or after --wait SECONDS (0..600): the command's own result, or REFUSED with state PENDING and exit 75. --attach --request-id ID [--wait SECONDS] only reads: the kept result byte for byte (CLAIMED with attemptId and generation, or the refusal verbatim), PENDING, or SUPERVISOR_LOST when the process ended without a result. A repeated --detach with the same arguments never starts a second mutation; after LOST it restarts and the store replays a committed request. The detached process is bounded by a 600-second ceiling (hard stop at 660). Darwin and Linux only. Records live in taskman-requests beside the journal; finished ones older than 7 days are pruned."

// leaseDetachMode is the parsed detach, attach or child flag set.
type leaseDetachMode struct {
	detach, attach, child bool
	wait                  int
	rest                  []string
}

// leaseDetachRequested reports whether a claim or pool acquire argv carries
// a detach flag; without one the command is the unchanged lease command
// (ATR-V0-026).
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

// leaseDetachProgress is progress.json: how far the child's lease command
// got. It is advisory, decides nothing, and is absent before the first
// preparation round ends (PSR-V0-112). Its members are closed: an older
// reader decodes it strictly, so new members go in leaseDetachWalks.
type leaseDetachProgress struct {
	Profile           string `json:"profile"`
	PreparationRounds int    `json:"preparationRounds"`
	ResumedRounds     int    `json:"resumedRounds"`
	AuditMillis       int64  `json:"auditMillis"`
	UpdatedAt         string `json:"updatedAt"`
}

// leaseDetachWalks is progress-walks.json: the progress.json members and
// the walk split of the same report (PSR-V0-152, V1-1626).
type leaseDetachWalks struct {
	Profile            string `json:"profile"`
	PreparationRounds  int    `json:"preparationRounds"`
	ResumedRounds      int    `json:"resumedRounds"`
	AuditMillis        int64  `json:"auditMillis"`
	CompleteWalks      int    `json:"completeWalks"`
	CompleteWalksMoved int    `json:"completeWalksMoved"`
	PrefixWalks        int    `json:"prefixWalks"`
	PrefixWalksMoved   int    `json:"prefixWalksMoved"`
	ResumedWalks       int    `json:"resumedWalks"`
	ResumeDeclines     int    `json:"resumeDeclines"`
	CompleteMillis     int64  `json:"completeMillis"`
	PrefixMillis       int64  `json:"prefixMillis"`
	ResumedMillis      int64  `json:"resumedMillis"`
	UpdatedAt          string `json:"updatedAt"`
}

// leaseDetachWalksOf is the child's record of t (PSR-V0-112, PSR-V0-152).
func leaseDetachWalksOf(t store.LeaseTiming, at time.Time) leaseDetachWalks {
	ms := func(d time.Duration) int64 { return max(d.Milliseconds(), 0) }
	return leaseDetachWalks{Profile: leaseDetachProfile, PreparationRounds: t.Rounds, ResumedRounds: t.Resumed, AuditMillis: ms(t.SnapshotRead),
		CompleteWalks: t.CompleteWalks, CompleteWalksMoved: t.CompleteMoved, PrefixWalks: t.PrefixWalks, PrefixWalksMoved: t.PrefixMoved, ResumedWalks: t.ResumedWalks, ResumeDeclines: t.ResumeDeclines,
		CompleteMillis: ms(t.CompleteWalk), PrefixMillis: ms(t.PrefixWalk), ResumedMillis: ms(t.ResumedWalk), UpdatedAt: at.UTC().Format(time.RFC3339)}
}

// writeLeaseDetachProgress writes one report: the sibling first, then
// progress.json in the shape every build reads (PSR-V0-152).
func writeLeaseDetachProgress(dir string, w leaseDetachWalks) error {
	if err := writeLeaseDetachJSON(dir, leaseDetachWalksName, w); err != nil {
		return err
	}
	return writeLeaseDetachJSON(dir, leaseDetachProgressName, leaseDetachProgress{Profile: w.Profile, PreparationRounds: w.PreparationRounds, ResumedRounds: w.ResumedRounds, AuditMillis: w.AuditMillis, UpdatedAt: w.UpdatedAt})
}

// leaseDetachProgressValue is the PENDING item's progress member: null when
// the child has not yet finished a round or progress.json does not read.
// The walk split comes from the sibling when it reads and carries the same
// report as progress.json; otherwise (an older child wrote progress.json, or
// a read fell between the two writes) the split reads 0.
func leaseDetachProgressValue(dir string) wire.Value {
	var base leaseDetachProgress
	if err := readLeaseDetachJSON(filepath.Join(dir, leaseDetachProgressName), &base); err != nil {
		return wire.Null()
	}
	p := leaseDetachWalks{PreparationRounds: base.PreparationRounds, ResumedRounds: base.ResumedRounds, AuditMillis: base.AuditMillis, UpdatedAt: base.UpdatedAt}
	var w leaseDetachWalks
	if err := readLeaseDetachJSON(filepath.Join(dir, leaseDetachWalksName), &w); err == nil &&
		w.PreparationRounds == p.PreparationRounds && w.ResumedRounds == p.ResumedRounds && w.AuditMillis == p.AuditMillis && w.UpdatedAt == p.UpdatedAt {
		p = w
	}
	counts := []int64{int64(p.PreparationRounds), int64(p.ResumedRounds), p.AuditMillis, int64(p.CompleteWalks), int64(p.CompleteWalksMoved), int64(p.PrefixWalks), int64(p.PrefixWalksMoved), int64(p.ResumedWalks), int64(p.ResumeDeclines), p.CompleteMillis, p.PrefixMillis, p.ResumedMillis}
	for _, c := range counts {
		if c < 0 {
			return wire.Null()
		}
	}
	n := func(v int64) wire.Value { return wire.String(string(wire.SizeOf(uint64(v)))) }
	o := wire.NewObject().Set("preparationRounds", n(counts[0])).Set("resumedRounds", n(counts[1])).Set("auditMillis", n(counts[2]))
	o.Set("completeWalks", n(counts[3])).Set("completeWalksMoved", n(counts[4])).Set("prefixWalks", n(counts[5])).Set("prefixWalksMoved", n(counts[6]))
	o.Set("resumedWalks", n(counts[7])).Set("resumeDeclines", n(counts[8])).Set("completeMillis", n(counts[9])).Set("prefixMillis", n(counts[10])).Set("resumedMillis", n(counts[11]))
	return wire.ObjectValue(o.Set("updatedAt", wire.String(p.UpdatedAt)))
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
// child (ATR-V0-020..029).
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
	// One deadline bounds both the relaunch's wait for a finishing child
	// and the wait for the result (ATR-V0-024, ATR-V0-095).
	deadline := time.Now().Add(time.Duration(max(m.wait, 0)) * time.Second)
	switch {
	case m.child:
		return leaseDetachRunChild(env, name, dir, want, m.rest)
	case m.detach:
		if res := leaseDetachLaunch(env, name, dir, want, m.rest, deadline); res != nil {
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
	return leaseDetachAwait(env, name, dir, requestID, holder, deadline)
}

func leaseDetachConflict(cmd []string, requestID string) *wire.Result {
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeRequestIDConflict}, Warnings: []string{prose("request id " + requestID + " already names a detached request with other arguments, holder or command; nothing was started")}}
}

// leaseDetachLaunch records the request and starts its child unless a live
// child or a kept result already answers it (ATR-V0-021). It returns nil
// when the caller should wait for the result until deadline.
func leaseDetachLaunch(env Env, name, dir string, want leaseDetachRecord, rest []string, deadline time.Time) *wire.Result {
	cmd := strings.Fields(name)
	base := filepath.Dir(dir)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return errorResult(cmd, err)
	}
	pruneLeaseDetach(base, time.Now())
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return errorResult(cmd, err)
	}
	// The launch-lock wait shares the command deadline, so a concurrent
	// launcher of the record never holds --wait 0 (ATR-V0-024, V1-1721).
	unlock, err := lockLeaseDetach(filepath.Join(dir, leaseDetachLockName), max(min(leaseDetachLockBound, time.Until(deadline)), 0))
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
	kept, retried, err := leaseDetachKept(dir, min(leaseDetachLockBound, time.Until(deadline)))
	switch {
	case err != nil:
		return errorResult(cmd, err)
	case kept:
		return nil
	}
	// The child that kept a retried LOCK_TIMEOUT has released the running
	// lock and only exits; its process may still be visible, so it never
	// holds up the relaunch (ATR-V0-095).
	if !retried {
		var child leaseDetachChild
		switch live, err := leaseDetachChildLive(dir, &child, time.Now()); {
		case err != nil:
			return errorResult(cmd, err)
		case live:
			return nil
		}
	}
	// No result and no live child: start one. A child that died before
	// keeping a result is replaced; the store's request-ID replay returns
	// what it committed, so this never claims twice.
	program, prefix, err := detachExecutable()
	if err != nil {
		return errorResult(cmd, err)
	}
	// Each run gets a fresh child.log; the previous run's log, the evidence
	// of a lost child, is kept as child.1.log (ATR-V0-028).
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
	exited := make(chan struct{})
	go func() { _ = c.Wait(); close(exited) }()
	if detachLaunched != nil {
		detachLaunched(env.Cwd, exited)
	}
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

// leaseDetachKept reports whether a kept result answers a repeat --detach.
// A kept LOCK_TIMEOUT means the store write never completed, so once no
// child still runs the record it is moved to result.1.json and the request
// runs again under the same request id; the store's request-ID replay keeps
// the mutation exactly once (ATR-V0-066). A child that holds the running
// lock beside a kept result is only finishing, so the launcher waits up to
// bound for it: leaseDetachLockBound or less when the caller's --wait ends
// sooner. While it is still held at the bound the kept result answers
// (ATR-V0-095, ATR-V0-024). retried reports the move.
func leaseDetachKept(dir string, bound time.Duration) (kept, retried bool, err error) {
	p := filepath.Join(dir, leaseDetachResultName)
	raw, err := readBounded(p, maxLeaseDetachResult)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, false, nil
	case err != nil:
		return false, false, err
	}
	res, err := wire.DecodeResult(raw)
	if err != nil || !slices.Contains(res.Codes, wire.CodeLockTimeout) {
		return true, false, nil
	}
	release, err := lockLeaseDetach(filepath.Join(dir, leaseDetachRunName), max(bound, 0))
	if err != nil {
		return true, false, nil
	}
	defer release()
	if err := os.Rename(p, filepath.Join(dir, leaseDetachPrevResult)); err != nil {
		return false, false, err
	}
	return false, true, nil
}

// leaseDetachUnknownIdentity is child.json's identity when the launcher
// could not read the child's.
const leaseDetachUnknownIdentity = "unknown"

// leaseDetachChildLive reports whether a child of this record may still run:
// it holds the record's running lock for its whole life, or, before it has
// taken that lock, its recorded process is live. Only positive evidence ends
// a recorded child: no process at its pid, a different identity there, or,
// when its identity cannot be confirmed, no running lock once the startup
// window has passed (ATR-V0-027). child receives the record.
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
	case err == nil && child.Identity != leaseDetachUnknownIdentity && supervisor.CompareIdentity(id, child.Identity) != supervisor.IdentityUnknown:
		// A different identity is a reused pid; a Linux btime one second
		// off is the same child (PSR-V0-227).
		return supervisor.SameProcess(id, child.Identity), nil
	}
	// The identity is not yet known, or its start tick matches with a btime
	// more than a second off (PSR-V0-227): a child still starting has not
	// taken running, so it is taken as live until the startup window has
	// passed.
	started, perr := time.Parse(time.RFC3339, child.StartedAt)
	return perr == nil && now.Before(started.Add(leaseDetachStartBound)), nil
}

// leaseDetachRunChild is the detached child: the unchanged lease command
// under the ceiling, its exact result bytes kept for attach (ATR-V0-022,
// ATR-V0-023).
func leaseDetachRunChild(env Env, name, dir string, want leaseDetachRecord, rest []string) int {
	var rec leaseDetachRecord
	// A record this build cannot read keeps its own code, such as
	// UNSUPPORTED_VERSION for another taskman-detached-lease version.
	if err := readLeaseDetachJSON(filepath.Join(dir, leaseDetachRequest), &rec); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return emit(env.Stdout, errorResult(strings.Fields(name), err))
	} else if err != nil || rec != want {
		return emit(env.Stdout, usage(strings.Fields(name), leaseDetachChildFlag+" is internal to a detached "+name))
	}
	// An interrupt from here on ends the command with a kept result: the
	// store write's ended context is LOCK_TIMEOUT (ATR-V0-053).
	interrupted, stopInterrupt := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopInterrupt()
	env.leaseInterrupted = interrupted
	// What the child itself writes to child.log is capped (ATR-V0-028).
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
	env.leaseProgress = func(t store.LeaseTiming) {
		if err := writeLeaseDetachProgress(dir, leaseDetachWalksOf(t, time.Now())); err != nil {
			_, _ = env.Stderr.Write([]byte("corvint-tasks detached " + name + ": progress: " + err.Error() + "\n"))
		}
	}
	var out bytes.Buffer
	code := emit(&out, leaseCommand(env, name, rest))
	if err := writeRunFile(dir, leaseDetachResultName, out.Bytes()); err != nil {
		_, _ = env.Stderr.Write([]byte("corvint-tasks detached " + name + ": " + err.Error() + "\n"))
		return 1
	}
	return code
}

// leaseDetachProbeGap, when set, runs in leaseDetachAwait between its attempt
// on the launch lock and its read of the child's liveness; tests use it to
// complete or start a launch in that gap. It is nil outside tests.
var leaseDetachProbeGap func()

// detachLaunched, when set, receives the working directory of each detached
// child a launcher starts and a channel closed once this process has reaped
// it, so tests join the children they start (ATR-V0-310). It is nil outside
// tests.
var detachLaunched func(cwd string, exited <-chan struct{})

// leaseDetachAwait answers from the kept result, or reports PENDING or LOST,
// waiting until deadline for the result (ATR-V0-022, ATR-V0-024). It only
// reads.
func leaseDetachAwait(env Env, name, dir, requestID, holder string, deadline time.Time) int {
	cmd := strings.Fields(name)
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
		// A launcher holds the launch lock exclusively from before it starts
		// a child until child.json records it. The attach takes the lock
		// shared, without waiting, before it reads the child's liveness and
		// keeps it until the answer is chosen: no launch can begin in
		// between, and one already under way holds the lock and is PENDING
		// (ATR-V0-311, V1-1559). The shared lock is never waited for, so a
		// launcher is held up by at most this one read.
		release, launching, err := leaseDetachShareLaunch(filepath.Join(dir, leaseDetachLockName))
		if err != nil {
			return emit(env.Stdout, errorResult(cmd, err))
		}
		if leaseDetachProbeGap != nil {
			leaseDetachProbeGap()
		}
		live, err := leaseDetachChildLive(dir, &child, time.Now())
		finished := false
		if err == nil && !live && !launching {
			if _, serr := os.Lstat(filepath.Join(dir, leaseDetachResultName)); serr == nil {
				finished = true // it finished between the two reads
			} else {
				state = "LOST"
			}
		}
		release()
		if err != nil {
			return emit(env.Stdout, errorResult(cmd, err))
		}
		if finished {
			continue
		}
		if state == "LOST" {
			return emit(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeSupervisorLost}, NotRetryable: true,
				Warnings: []string{prose("the detached " + name + " ended without keeping a result; nothing was signalled; repeat " + name + " --detach with the same arguments and request id: the store replays a committed request and otherwise runs it once")},
				Items:    []wire.Value{leaseDetachItem(name, dir, requestID, holder, state, &child)}})
		}
		if !time.Now().Before(deadline) {
			item := leaseDetachItem(name, dir, requestID, holder, state, &child)
			item.Obj.Set("progress", leaseDetachProgressValue(dir))
			return emitCode(env.Stdout, &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused,
				Warnings: []string{prose("the detached " + name + " has not finished; attach again with " + name + " --attach --request-id " + requestID)},
				Items:    []wire.Value{item}}, runExitPending)
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
// records never hide finished ones behind them (ATR-V0-029). Best effort.
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
// (ATR-V0-028).
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
