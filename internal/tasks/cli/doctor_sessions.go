package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Host session detectors (TQD-V0-027..029) and the dispatcher work-state
// reader health report (TQD-V0-031) read each configured dispatcher's
// state directory: state.json and each worker's live stdout.log. They only
// read; an unreadable directory or log is a scan warning and no finding.
const (
	doctorMaxWorkers    = 256
	doctorMaxWorkerDirs = 4096
	// doctorSessionLogBytes is the live segment cap (CAL-V0-143) plus one
	// usage line of slack.
	doctorSessionLogBytes = 9 << 20
)

// doctorSessionCounts is one log's classified evidence: each count and the
// first and last line it spans.
type doctorSessionCounts struct {
	storm, stormFrom, stormTo       int
	errs, errFrom, errTo            int
	refusals, refusalFrom, refuseTo int
}

// doctorClassifySession counts, under format, the longest run of turn or
// step starts with no finish between them, the host error events, and the
// failed tool calls before the session's first successful one. Lines the
// format does not recognise count for nothing.
func doctorClassifySession(format string, raw []byte) doctorSessionCounts {
	var c doctorSessionCounts
	run, runFrom := 0, 0
	toolOK := false
	for n, line := 1, raw; len(line) > 0; n++ {
		cur := line
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			cur, line = line[:i], line[i+1:]
		} else {
			line = nil
		}
		for _, ev := range dispatch.ClassifyHostLine(format, cur) {
			switch ev {
			case dispatch.HostStart:
				if run == 0 {
					runFrom = n
				}
				if run++; run > c.storm {
					c.storm, c.stormFrom, c.stormTo = run, runFrom, n
				}
			case dispatch.HostFinish:
				run = 0
			case dispatch.HostError:
				if c.errs++; c.errs == 1 {
					c.errFrom = n
				}
				c.errTo = n
			case dispatch.HostToolOK:
				toolOK = true
			case dispatch.HostToolFailed:
				if !toolOK {
					if c.refusals++; c.refusals == 1 {
						c.refusalFrom = n
					}
					c.refuseTo = n
				}
			}
		}
	}
	return c
}

// doctorSessionFindings turns one worker log's counts into findings.
func doctorSessionFindings(th doctorThresholds, who, format, path string, c doctorSessionCounts) []doctorFinding {
	var out []doctorFinding
	add := func(kind, what string, n, from, to int) {
		out = append(out, doctorFinding{kind: kind, who: who, carried: true,
			detail: fmt.Sprintf("%d %s (%s) in %s lines %d-%d", n, what, format, path, from, to)})
	}
	if c.storm >= th.retryStorm {
		add("HOST_RETRY_STORM", "turn starts with no completed turn between them", c.storm, c.stormFrom, c.stormTo)
	}
	if c.errs >= th.hostErrors {
		add("HOST_ERRORS", "host error events", c.errs, c.errFrom, c.errTo)
	}
	if c.refusals >= th.earlyRefusal {
		add("EARLY_REFUSAL", "failed tool calls before the first successful one", c.refusals, c.refusalFrom, c.refuseTo)
	}
	return out
}

// doctorDispatchers reads every configured dispatcher, appending its
// work-state health to the scan and returning its session findings.
func doctorDispatchers(cfg *doctorConfig, scan *doctorScan) []doctorFinding {
	var out []doctorFinding
	for _, d := range cfg.dispatchers {
		out = append(out, doctorDispatcher1(d, cfg.thresholds, scan)...)
	}
	return out
}

func doctorDispatcher1(d doctorDispatcher, th doctorThresholds, scan *doctorScan) []doctorFinding {
	warn := func(format string, a ...any) {
		scan.warnings = append(scan.warnings, prose(fmt.Sprintf("doctor dispatcher %s: ", d.programDir)+fmt.Sprintf(format, a...)))
	}
	var out []doctorFinding
	formats := map[string]string{}
	ledger, err := doctorLoadLedger(d.programDir)
	health := wire.NewObject().Set("programDir", wire.String(prose(d.programDir)))
	if err != nil {
		warn("state unreadable: %v", err)
		health.Set("workState", doctorWorkStateValue(nil, false))
		scan.dispatchers = append(scan.dispatchers, wire.ObjectValue(health))
	} else {
		health.Set("workState", doctorWorkStateValue(ledger.WorkState, true))
		scan.dispatchers = append(scan.dispatchers, wire.ObjectValue(health))
		if h := ledger.WorkState; h != nil && !h.OK {
			out = append(out, doctorFinding{kind: "WORK_STATE_FAILING", who: d.programDir, firstSeen: h.Since,
				detail: fmt.Sprintf("work-state reader failing for %d tick(s) since %s: %s", h.FailedTicks, h.Since.UTC().Format(time.RFC3339), h.LastError)})
		}
		for _, w := range ledger.Workers {
			if w != nil && w.Usage != nil {
				formats[w.ID] = w.Usage.Format
			}
		}
		out = append(out, doctorParkedOpen(d.programDir, ledger, scan.openItems)...)
	}
	out = append(out, doctorAdmission(d, th, scan, warn)...)
	root := filepath.Join(d.programDir, "workers")
	f, err := openDirNonblock(root)
	if errors.Is(err, os.ErrNotExist) {
		return out
	}
	if err != nil {
		warn("workers directory unreadable: %v", err)
		return out
	}
	entries, err := f.ReadDir(doctorMaxWorkerDirs + 1)
	f.Close()
	if err != nil && err != io.EOF {
		warn("workers directory unreadable: %v", err)
		return out
	}
	if len(entries) > doctorMaxWorkerDirs {
		warn("more than %d worker entries; sessions not read", doctorMaxWorkerDirs)
		return out
	}
	// Name order, so which sessions fit under the read limit never
	// depends on directory order.
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	read := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		format := formats[e.Name()]
		if format == "" {
			format = d.format
		}
		if format == "" {
			continue
		}
		path := filepath.Join(root, e.Name(), "stdout.log")
		st, err := os.Lstat(path)
		if err != nil || !st.Mode().IsRegular() || scan.observedAt.Sub(st.ModTime()) > th.sessionWindow {
			continue
		}
		if read++; read > doctorMaxWorkers {
			warn("more than %d recent worker sessions; the rest were not read", doctorMaxWorkers)
			break
		}
		raw, err := readBounded(path, doctorSessionLogBytes)
		if err != nil {
			warn("worker %s log unreadable: %v", e.Name(), err)
			continue
		}
		out = append(out, doctorSessionFindings(th, e.Name(), format, path, doctorClassifySession(format, raw))...)
	}
	return out
}

// doctorParkedOpen is one PARKED_WITH_OPEN_ITEMS finding per parked
// ticket key of ledger whose ticket still has pending obligation items
// (TOL-V0-125), in key order.
func doctorParkedOpen(programDir string, ledger *dispatch.Ledger, open map[string]int64) []doctorFinding {
	keys := make([]string, 0, len(ledger.Backoff))
	for k, b := range ledger.Backoff {
		if b != nil && b.Parked && open[k] > 0 {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	out := make([]doctorFinding, 0, len(keys))
	for _, k := range keys {
		out = append(out, doctorFinding{kind: "PARKED_WITH_OPEN_ITEMS", who: k, carried: true,
			detail: fmt.Sprintf("ticket %s is parked by dispatcher %s with %d open obligation item(s)", k, programDir, open[k])})
	}
	return out
}

// doctorOpenItems maps each OPEN or HELD ticket whose ledger reference
// counts pending items (total less witnessed) to that count.
func doctorOpenItems(inv *ticket.Inventory) map[string]int64 {
	out := map[string]int64{}
	for _, id := range inv.IDs() {
		rec, _ := inv.Get(id)
		if activeLedger(rec) {
			if n := rec.ObligationsRef.Counts.Total - rec.ObligationsRef.Counts.Witnessed; n > 0 {
				out[id] = n
			}
		}
	}
	return out
}

// doctorLoadLedger reads a dispatcher's state.json without following a
// final symlink or blocking on a FIFO, refusing anything but a regular file,
// then decodes it as LoadLedger does; a missing file is a fresh ledger.
func doctorLoadLedger(programDir string) (*dispatch.Ledger, error) {
	program := filepath.Base(programDir)
	raw, err := readBounded(filepath.Join(programDir, "state.json"), dispatch.MaxLedgerBytes)
	if errors.Is(err, os.ErrNotExist) {
		return &dispatch.Ledger{}, nil // never written: NOT_OBSERVED
	}
	if err != nil {
		return nil, err
	}
	return dispatch.DecodeLedger(raw, program)
}

// doctorWorkStateValue is the CAL-V0-270 reader health as dispatch status
// shows it, without the dispatcher-configuration members: health OK,
// FAILING, NOT_OBSERVED (never read) or UNREADABLE (state.json unreadable),
// and once read ok, since, lastError ("NONE" before any failure),
// failedTicks and escalated.
func doctorWorkStateValue(h *dispatch.WorkStateHealth, readable bool) wire.Value {
	o := wire.NewObject()
	switch {
	case !readable:
		return wire.ObjectValue(o.Set("health", wire.String("UNREADABLE")))
	case h == nil:
		return wire.ObjectValue(o.Set("health", wire.String("NOT_OBSERVED")))
	case h.OK:
		o.Set("health", wire.String("OK"))
	default:
		o.Set("health", wire.String("FAILING"))
	}
	last := h.LastError
	if last == "" {
		last = "NONE"
	}
	o.Set("ok", wire.Bool(h.OK)).Set("since", wire.String(h.Since.UTC().Format(time.RFC3339)))
	o.Set("lastError", wire.String(prose(last))).Set("failedTicks", wire.String(strconv.Itoa(h.FailedTicks)))
	return wire.ObjectValue(o.Set("escalated", wire.Bool(h.Escalated)))
}
