package dispatch

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// exitStatus is a worker leader's observed exit: its exit code (-1 when a
// signal ended it) and the signal number, 0 when none did (ATR-V0-350).
type exitStatus struct {
	code   int
	signal int
}

// Stop reasons of a finished event (ATR-V0-350). The first five are the
// issue 1799 vocabulary; the supervisor's idle and wall caps and an exit the
// dispatcher did not observe have their own values rather than a guessed one.
const (
	StopCompleted    = "completed"
	StopOperator     = "operator-stop"
	StopContextLimit = "context-limit"
	StopLeaseExpired = "lease-expired"
	StopCrash        = "crash"
	StopIdleTimeout  = "idle-timeout"
	StopWallLimit    = "wall-limit"
	StopUnknown      = "unknown"
)

// Attempt release outcomes of a finished event (ATR-V0-351).
const (
	AttemptHandedOff = "handed-off"
	AttemptReaped    = "reaped"
	AttemptReleased  = "released"
	AttemptHeld      = "held"
	AttemptNone      = "none"
)

// StoppedBeforeSummary is the message of a stopped worker that left no
// structured final text (ATR-V0-353).
const StoppedBeforeSummary = "(stopped before summary)"

// maxFinishCommits bounds the commit list a finished event carries.
const maxFinishCommits = 32

const launchHeadFile = "launch-head.json"

var signalNames = map[int]string{1: "SIGHUP", 2: "SIGINT", 3: "SIGQUIT", 4: "SIGILL", 5: "SIGTRAP", 6: "SIGABRT", 8: "SIGFPE", 9: "SIGKILL", 11: "SIGSEGV", 13: "SIGPIPE", 14: "SIGALRM", 15: "SIGTERM"}

func signalName(n int) string {
	if s, ok := signalNames[n]; ok {
		return s
	}
	return "SIG" + strconv.Itoa(n)
}

// signalNumber is the signal that ended the worker: the one its wait status
// reports, else the shell's 128+N exit convention for N up to 31; 0 is none.
func (s exitStatus) signalNumber() int {
	if s.signal > 0 {
		return s.signal
	}
	if s.code > 128 && s.code < 160 {
		return s.code - 128
	}
	return 0
}

// exitSignalText is the exitSignal detail value (ATR-V0-350).
func exitSignalText(s exitStatus, observed bool) string {
	if !observed {
		return "NOT_OBSERVED"
	}
	if n := s.signalNumber(); n > 0 {
		return signalName(n)
	}
	return "NONE"
}

// stopReason derives a finished worker's stop reason: the dispatcher's own
// stop first, then the observed exit (ATR-V0-350).
func stopReason(killReason string, s exitStatus, observed bool) string {
	switch killReason {
	case "CONTEXT":
		return StopContextLimit
	case "LEASE_EXPIRED":
		return StopLeaseExpired
	case "IDLE":
		return StopIdleTimeout
	case "WALL":
		return StopWallLimit
	}
	if !observed {
		return StopUnknown
	}
	switch s.signalNumber() {
	case 0:
		if s.code == 0 {
			return StopCompleted
		}
		return StopCrash
	case 1, 2, 9, 15:
		return StopOperator
	}
	return StopCrash
}

// launchHead is the commit a worker's launch directory was at when it was
// spawned, kept beside its logs so a restart can still list its commits.
type launchHead struct {
	Cwd  string `json:"cwd"`
	Head string `json:"head"`
}

// resolveLaunchHead resolves HEAD in a launch directory before spawn; an
// unresolved one is "" and leaves the worker's commits NOT_OBSERVED.
func (d *Dispatcher) resolveLaunchHead(cwd string) string {
	if d.ResolveCommit == nil || d.ListCommits == nil || cwd == "" {
		return ""
	}
	head, err := d.ResolveCommit(cwd, "HEAD")
	if err != nil || !commitID(head) {
		return ""
	}
	return head
}

func (d *Dispatcher) saveLaunchHead(id, cwd, head string) {
	if head == "" {
		return
	}
	raw, err := json.Marshal(launchHead{Cwd: cwd, Head: head})
	if err != nil {
		return
	}
	// A failed write leaves the worker's commits NOT_OBSERVED.
	_ = writeAtomic(filepath.Join(d.workerDir(id), launchHeadFile), append(raw, '\n'))
}

// finishCommits lists the commits made in worker id's launch directory since
// its launch, newest first; ok is false when they were not observed
// (ATR-V0-351).
func (d *Dispatcher) finishCommits(id string) (commits []string, truncated, ok bool) {
	if d.ListCommits == nil {
		return nil, false, false
	}
	raw, err := readRegular(filepath.Join(d.workerDir(id), launchHeadFile), 64<<10)
	if err != nil {
		return nil, false, false
	}
	var h launchHead
	if json.Unmarshal(raw, &h) != nil || h.Cwd == "" || !commitID(h.Head) {
		return nil, false, false
	}
	list, err := d.ListCommits(h.Cwd, h.Head, maxFinishCommits+1)
	if err != nil {
		return nil, false, false
	}
	for _, c := range list {
		if !commitID(c) {
			return nil, false, false
		}
	}
	if len(list) > maxFinishCommits {
		return list[:maxFinishCommits], true, true
	}
	return list, false, true
}

func commitID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

// attemptRelease reports what became of the attempts worker w held
// (ATR-V0-351): still held in obs, handed off by this dispatcher, reaped on
// lease expiry, released by the worker itself, or none observed held.
func (d *Dispatcher) attemptRelease(obs *Observation, w *Worker) string {
	if obs != nil {
		for _, a := range obs.Attempts {
			if a.Live && a.Holder == w.ID {
				return AttemptHeld
			}
		}
	}
	switch {
	case d.handedOff[w.ID]:
		return AttemptHandedOff
	case w.KillReason == "LEASE_EXPIRED" && d.everHeld[w.ID]:
		return AttemptReaped
	case d.everHeld[w.ID]:
		return AttemptReleased
	}
	return AttemptNone
}

func (d *Dispatcher) noteHandedOff(worker string) {
	if d.worker(worker) == nil {
		return
	}
	if d.handedOff == nil {
		d.handedOff = map[string]bool{}
	}
	d.handedOff[worker] = true
}

// hasHandoffNote reports whether a valid hand-off note of worker id exists.
func (d *Dispatcher) hasHandoffNote(id string) bool {
	n, err := ReadHandoffNote(HandoffNotePath(d.dir, id))
	return err == nil && n.Worker == id
}

// progressLabel is the finished event's human progress label, derived from
// the accounting class and the observed facts rather than the output stream
// (ATR-V0-352). A stop other than completion is named with its signal.
func progressLabel(class, reason, signal string, commits int, commitsSeen bool) string {
	stop := ""
	if reason != StopCompleted && reason != StopUnknown {
		stop = "stopped (" + reason + ")"
		if reason == StopCrash {
			stop = "failed (" + reason + ")"
		}
		if signal != "NONE" && signal != "NOT_OBSERVED" {
			stop = "stopped by " + signal + " (" + reason + ")"
		}
	}
	base := map[string]string{SessionProgress: "progress recorded", SessionInfrastructure: "infrastructure session", SessionHeld: "held by a typed escalation"}[class]
	if base == "" {
		switch {
		case commitsSeen && commits > 0:
			base = fmt.Sprintf("%d %s, store state unchanged", commits, plural(commits, "commit", "commits"))
		case stop != "":
			return stop + " before recording progress"
		default:
			base = "no progress"
		}
	}
	if stop != "" {
		return stop + "; " + base
	}
	return base
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// finishSummary is the summary a finished message carries (ATR-V0-353): a
// worker that did not complete, or whose exit was not observed, carries its
// last structured final text only, never a raw tail.
func finishSummary(logDir, reason string) string {
	text, structured := summaryText(logDir, reason != StopCompleted)
	if reason == StopCompleted {
		return text
	}
	if !structured || text == "" {
		return StoppedBeforeSummary
	}
	return text
}
