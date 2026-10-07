package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// Usage states of a worker's token total (CAL-V0-157). KNOWN is a complete
// account, PARTIAL a lower bound and UNKNOWN no observation; an UNKNOWN total
// is never zero. A RUNNING session's total is still growing.
const (
	UsageKnown   = "KNOWN"
	UsagePartial = "PARTIAL"
	UsageUnknown = "UNKNOWN"
	UsageRunning = "RUNNING"
)

const (
	// maxUsageLine is the longest standard-output line read for usage,
	// the bound the supervisor's host readers use. A longer line is
	// skipped and leaves the total PARTIAL.
	maxUsageLine = 1 << 20
)

// maxUsageRead bounds the bytes one scan reads; bytes left over are read
// by the next scan unless the CAL-V0-143 cap removes them first. Tests
// lower it.
var maxUsageRead int64 = 8 << 20

// WorkerUsage is a worker's incremental token account (CAL-V0-157): the
// declared vocabulary, how far its live stdout.log has been read, and the
// counters and completeness facts read so far.
type WorkerUsage struct {
	Format string `json:"format"`
	// Offset is the byte offset of the next unread line in stdout.log.
	Offset  int64  `json:"offset"`
	Input   uint64 `json:"input"`
	Output  uint64 `json:"output"`
	Records int    `json:"records"`
	// Lost is set when output bytes left unread were removed (a cut, a
	// shrunk file or the final read bound), and Malformed when a usage
	// line, or a line too long to read, could not be counted.
	Lost      bool `json:"lost,omitempty"`
	Malformed bool `json:"malformed,omitempty"`
	// Failed records a host failure event, whose usage is not reported.
	Failed bool `json:"failed,omitempty"`
	// Skipping is set while the scan is inside a line it does not read.
	Skipping bool `json:"skipping,omitempty"`
	// Open is set while a Codex turn or an OpenCode step has started and
	// not finished; Stopped records that the last finished OpenCode step's
	// reason was "stop".
	Open    bool `json:"open,omitempty"`
	Stopped bool `json:"stopped,omitempty"`
}

func (u *WorkerUsage) validate() error {
	if u == nil {
		return nil
	}
	if !slices.Contains(UsageFormats, u.Format) || u.Offset < 0 || u.Records < 0 || u.Input+u.Output < u.Input {
		return errors.New("malformed usage account")
	}
	return nil
}

// State is the usage state of a finished worker, or of a running one when
// final is false (never KNOWN while it may still write).
func (u *WorkerUsage) State(final bool) string {
	if u == nil || u.Records == 0 {
		return UsageUnknown
	}
	if !final || u.Lost || u.Malformed || u.Failed || u.Skipping {
		return UsagePartial
	}
	switch u.Format {
	case "codex":
		if u.Open {
			return UsagePartial
		}
	case "opencode":
		if u.Open || !u.Stopped {
			return UsagePartial
		}
	case "claude-code":
		if u.Records != 1 {
			return UsagePartial
		}
	}
	return UsageKnown
}

// usageMarker is the event type whose lines carry usage, so a usage line
// that is not valid JSON is recognized as malformed rather than ignored.
var usageMarker = map[string][]byte{
	"codex":       []byte(`"turn.completed"`),
	"claude-code": []byte(`"result"`),
	"opencode":    []byte(`"step_finish"`),
}

// line counts one complete standard-output line. Lines that are not JSON
// objects, and events of other types, are not usage and are ignored.
func (u *WorkerUsage) line(b []byte) {
	if len(bytes.TrimSpace(b)) == 0 {
		return
	}
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(b, &head) != nil {
		if bytes.Contains(b, usageMarker[u.Format]) {
			u.Malformed = true
		}
		return
	}
	var in, out uint64
	switch u.Format {
	case "codex":
		// A repeated member on a usage line would let a later null, zero
		// or smaller counter (or type) replace an observed one.
		if bytes.Contains(b, usageMarker["codex"]) && supervisor.UniqueMembers(b) != nil {
			u.Malformed = true
			return
		}
		switch head.Type {
		case "turn.started":
			u.Open = true
			return
		case "turn.failed", "error":
			u.Failed = true
			return
		case "turn.completed":
		default:
			return
		}
		var ok bool
		if in, out, ok = supervisor.ObservedUsage(b); !ok || nullCodexCounter(b) {
			u.Malformed = true
			return
		}
		u.Open = false
	case "claude-code":
		if head.Type != "result" {
			return
		}
		var ok bool
		if in, out, ok = supervisor.ObservedClaudeUsage(b); !ok {
			u.Malformed = true
			return
		}
		// The result object reports the session's usage, so a second one
		// is not added; the larger counters stay a lower bound and the
		// total is PARTIAL (State).
		in, out = max(u.Input, in), max(u.Output, out)
		if in+out < in {
			u.Malformed = true
			return
		}
		u.Records++
		u.Input, u.Output = in, out
		return
	case "opencode":
		kind, part, reason, i, o, ok := supervisor.OpenCodeLineUsage(b)
		switch {
		case !ok && (kind == "step_finish" || bytes.Contains(b, usageMarker[u.Format])):
			u.Malformed = true
			return
		case !ok:
			return
		case kind == "step_start":
			u.Open = true
			return
		case kind == "error":
			u.Failed = true
			return
		case kind != "step_finish":
			return
		}
		in, out = i, o
		u.Open, u.Stopped = false, part == "step-finish" && reason == "stop"
	default:
		return
	}
	// Each counter and their sum must stay representable, so a KNOWN
	// total is never a wrapped one.
	if i, o := u.Input+in, u.Output+out; i < u.Input || o < u.Output || i+o < i {
		u.Malformed = true
		return
	}
	u.Records++
	u.Input += in
	u.Output += out
}

// nullCodexCounter reports a turn.completed line whose input or output
// counter is JSON null, which ObservedUsage would read as 0: a null counter
// is not an observation.
func nullCodexCounter(b []byte) bool {
	var ev struct {
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(b, &ev) != nil {
		return true
	}
	for _, k := range []string{"input_tokens", "output_tokens"} {
		if bytes.Equal(bytes.TrimSpace(ev.Usage[k]), []byte("null")) {
			return true
		}
	}
	return false
}

// scan reads complete lines of f from Offset up to end, at most
// maxUsageRead bytes, and advances Offset past each line it consumed. With
// final, the worker has exited and a trailing line without a newline is
// read too. A file shorter than Offset was cut by something else: the
// bytes in between are lost.
func (u *WorkerUsage) scan(f *os.File, end int64, final bool) {
	if end < u.Offset {
		u.Lost, u.Skipping, u.Offset = true, true, 0
	}
	stop := min(end, u.Offset+maxUsageRead)
	if stop <= u.Offset {
		return
	}
	buf := make([]byte, stop-u.Offset)
	n, _ := f.ReadAt(buf, u.Offset)
	buf = buf[:n]
	for len(buf) > 0 {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			switch {
			case final && u.Offset+int64(len(buf)) == end:
				if !u.Skipping {
					u.measured(buf)
				}
				u.Skipping = false
			case len(buf) > maxUsageLine:
				// The line is already longer than the bound: skip to
				// its end, which a later scan finds.
				u.Malformed, u.Skipping = true, true
			default:
				return // an incomplete line waits for its newline
			}
			u.Offset += int64(len(buf))
			return
		}
		if u.Skipping {
			u.Skipping = false
		} else {
			u.measured(buf[:i])
		}
		u.Offset += int64(i + 1)
		buf = buf[i+1:]
	}
}

// measured counts a line within the bound and refuses a longer one.
func (u *WorkerUsage) measured(b []byte) {
	if len(b) > maxUsageLine {
		u.Malformed = true
		return
	}
	u.line(b)
}

// cut accounts for the CAL-V0-143 truncation of stdout.log. Bytes the
// read did not reach, or that the worker appended between the cut's size
// check and the truncation, are gone and cannot be proven absent, so every
// cut leaves the total PARTIAL; the first line after it may continue a torn
// one and is skipped, which keeps the counters a lower bound.
func (u *WorkerUsage) cut() {
	u.Lost, u.Skipping, u.Offset = true, true, 0
}

// drainUsage is the final bounded read of an ended worker's stdout.log.
// Output it could not read is lost, so the total is not KNOWN.
func drainUsage(dir string, u *WorkerUsage) {
	if u == nil {
		return
	}
	f, err := os.Open(filepath.Join(dir, "stdout.log"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) || u.Offset > 0 {
			u.Lost = true
		}
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		u.Lost = true
		return
	}
	u.scan(f, st.Size(), true)
	if u.Offset < st.Size() {
		u.Lost = true
	}
}

// readUsage is one supervising tick's incremental read of a running
// worker's stdout.log.
func readUsage(dir string, u *WorkerUsage) {
	if u == nil {
		return
	}
	f, err := os.Open(filepath.Join(dir, "stdout.log"))
	if err != nil {
		return // read again next tick; drainUsage decides at the end
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil {
		u.scan(f, st.Size(), false)
	}
}

// finishUsage drains an ended worker's usage, settles its session and adds
// its total and declared model, tier and effort to the finished event
// (CAL-V0-157, CAL-V0-159). An UNKNOWN total is written UNKNOWN, never 0.
func (d *Dispatcher) finishUsage(w *Worker, detail map[string]string) {
	drainUsage(d.workerDir(w.ID), w.Usage)
	state := w.Usage.State(true)
	d.settleSession(w.ID, state, w.Usage)
	detail["usage"] = state
	detail["inputTokens"], detail["outputTokens"], detail["tokens"] = UsageUnknown, UsageUnknown, UsageUnknown
	if state != UsageUnknown {
		detail["inputTokens"] = strconv.FormatUint(w.Usage.Input, 10)
		detail["outputTokens"] = strconv.FormatUint(w.Usage.Output, 10)
		detail["tokens"] = strconv.FormatUint(w.Usage.Input+w.Usage.Output, 10)
	}
	if w.Model != "" {
		detail["model"], detail["tier"] = w.Model, strconv.Itoa(w.Tier)
	}
	if w.Effort != "" {
		detail["effort"] = w.Effort
	}
}
