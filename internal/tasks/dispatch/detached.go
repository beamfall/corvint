package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Detached attempt runs under the dispatcher (CAL-V0-145..154). A worker may
// start a detached run (ATR-V0-008) and end while it runs. The dispatcher
// then leaves the run's verified supervisor alone, defers the hand-off of
// the attempt while the run is RUNNING, and once the run finishes launches
// one session for the same ticket and role with the run's outcome.

// Run record states the dispatcher reads (ATR-V0-010).
const (
	RunStarting = "STARTING"
	RunRunning  = "RUNNING"
	RunFinished = "FINISHED"
)

// DetachedRun is the dispatcher's read-only view of one strictly decoded
// detached run record (ATR-V0-010).
type DetachedRun struct {
	RunID, AttemptID, Generation, State string
	SupervisorPID                       int
	SupervisorIdentity                  string
	TimeoutSeconds                      int
	LaunchedAt                          time.Time
	ExitStatus                          *int
	// Dir is the run directory; Result and Output are its kept envelope
	// and its current output segment.
	Dir, Result, Output string
}

// RunReader is an optional Queue capability (CAL-V0-145): it reads an
// attempt's detached run records. A Queue without it, or a read error, keeps
// the dispatcher's behaviour without detached runs.
type RunReader interface {
	// AttemptRuns returns the attempt's run records that decode and name
	// that attempt and their own directory, reading at most 65 entries.
	AttemptRuns(attemptID string) ([]DetachedRun, error)
	// AttemptRun reads one run record; it returns nil and no error when
	// the record does not exist.
	AttemptRun(attemptID, runID string) (*DetachedRun, error)
}

const (
	detachedProfile = "taskman-dispatch-detached-run/0"
	// detachedDirName holds one marker per deferred run, beside the ledger
	// and never inside it.
	detachedDirName        = "detached"
	maxDetachedMarkers     = 64
	maxDetachedMarkerBytes = 4096
	// maxDetachedEntries bounds one read of the marker directory.
	maxDetachedEntries = 256
	// detachedSettle extends a run's deferral past its timeout: the
	// 10-second cleanup bound, the 120-second writer margin its lease
	// already covers (ATR-V0-002) and its result and record writes.
	detachedSettle = 3 * time.Minute
	// detachedRelaunchWindow bounds how long a finished run waits for its
	// relaunch.
	detachedRelaunchWindow = 24 * time.Hour
)

// Marker phases.
const (
	// DetachedDeferred: the hand-off of the ended worker's attempt waits for
	// the run.
	DetachedDeferred = "DEFERRED"
	// DetachedHandedOff: the hand-off was made or became moot; the run's
	// end is awaited.
	DetachedHandedOff = "HANDED_OFF"
	// DetachedRelaunch: the run finished; one session is due.
	DetachedRelaunch = "RELAUNCH"
	// DetachedLaunched: the successor was named before it was spawned.
	DetachedLaunched = "LAUNCHED"
)

// detachedMarker is one dispatcher-owned marker, detached/<runId>.json. It is
// derived state of this dispatcher, not ledger state.
type detachedMarker struct {
	Profile            string    `json:"profile"`
	RunID              string    `json:"runId"`
	AttemptID          string    `json:"attemptId"`
	Generation         string    `json:"generation"`
	Ticket             string    `json:"ticket"`
	Role               string    `json:"role"`
	Worker             string    `json:"worker"`
	SupervisorPID      int       `json:"supervisorPid"`
	SupervisorIdentity string    `json:"supervisorIdentity"`
	Phase              string    `json:"phase"`
	DeferredAt         time.Time `json:"deferredAt"`
	DeferUntil         time.Time `json:"deferUntil"`
	ReadyAt            time.Time `json:"readyAt"`
	ExitStatus         *int      `json:"exitStatus"`
	Result             string    `json:"result"`
	Output             string    `json:"output"`
	Successor          string    `json:"successor"`
}

func validRunName(s string) bool {
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

// decodeDetachedMarker is strict: one bounded object, no unknown keys, and
// facts that agree with its phase.
func decodeDetachedMarker(raw []byte, name string) (*detachedMarker, error) {
	if len(raw) > maxDetachedMarkerBytes {
		return nil, errors.New("marker too large")
	}
	// Another build's marker version is refused, not misread (CAL-V0-131).
	if err := wire.RawProfileVersion("/profile", raw, detachedProfile); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m detachedMarker
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("marker does not decode: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("marker has trailing data")
	}
	switch {
	case m.Profile != detachedProfile:
		return nil, fmt.Errorf("marker profile is not %s", detachedProfile)
	case !validRunName(m.RunID) || name != m.RunID+".json":
		return nil, errors.New("marker does not name its run")
	case m.AttemptID == "" || m.Generation == "" || m.Ticket == "" || m.Role == "" || m.Worker == "":
		return nil, errors.New("marker does not name its attempt, ticket, role and worker")
	case m.SupervisorPID <= 1 || m.SupervisorIdentity == "" || m.DeferredAt.IsZero() || m.DeferUntil.IsZero():
		return nil, errors.New("marker does not name its supervisor and deferral")
	}
	switch m.Phase {
	case DetachedDeferred, DetachedHandedOff:
		if m.ExitStatus != nil || m.Successor != "" {
			return nil, errors.New("marker records an outcome before the run finished")
		}
	case DetachedRelaunch, DetachedLaunched:
		if m.ExitStatus == nil || m.ReadyAt.IsZero() || (m.Phase == DetachedLaunched) != (m.Successor != "") {
			return nil, errors.New("marker phase disagrees with its outcome")
		}
	default:
		return nil, fmt.Errorf("unknown marker phase %q", m.Phase)
	}
	return &m, nil
}

func (d *Dispatcher) detachedDir() string { return filepath.Join(d.dir, detachedDirName) }

// readDetachedMarkers reads at most maxDetachedEntries entries of the marker
// directory. Markers that do not decode are returned by name with their
// error; leftover temporary files are listed for removal.
func readDetachedMarkers(dir string) (good []*detachedMarker, bad map[string]error, temps []string, err error) {
	f, err := os.Open(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(maxDetachedEntries)
	if err != nil && err != io.EOF {
		return nil, nil, nil, err
	}
	bad = map[string]error{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".tmp-") {
			temps = append(temps, name)
			continue
		}
		if !e.Type().IsRegular() {
			bad[name] = errors.New("not a regular file")
			continue
		}
		raw, err := readBounded(filepath.Join(dir, name), maxDetachedMarkerBytes)
		if err != nil {
			bad[name] = err
			continue
		}
		m, err := decodeDetachedMarker(raw, name)
		if err != nil {
			bad[name] = err
			continue
		}
		good = append(good, m)
	}
	sort.Slice(good, func(i, j int) bool { return good[i].RunID < good[j].RunID })
	return good, bad, temps, nil
}

// loadDetached adopts the markers of an earlier run. A marker that does not
// decode is reported and removed: it can then defer nothing and launch
// nothing. A marker of another profile version is reported and kept, unread,
// for the build that wrote it; its run is never deferred here. Markers beyond
// the bound are reported and left unread.
func (d *Dispatcher) loadDetached() {
	dir := d.detachedDir()
	good, bad, temps, err := readDetachedMarkers(dir)
	if err != nil {
		d.emit(Event{Kind: "alert", Message: "detached run markers unreadable; no deferred hand-off or relaunch is resumed: " + err.Error()})
		return
	}
	for _, name := range temps {
		_ = os.Remove(filepath.Join(dir, name))
	}
	d.detachedHeld = map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(bad)) {
		if wire.CodeOf(bad[name]) == wire.CodeUnsupportedVersion {
			d.detachedHeld[strings.TrimSuffix(name, ".json")] = true
			d.emit(Event{Kind: "alert", Message: fmt.Sprintf("kept detached run marker %s unread: %v", name, bad[name])})
			continue
		}
		d.emit(Event{Kind: "alert", Message: fmt.Sprintf("removed detached run marker %s: %v", name, bad[name])})
		_ = os.RemoveAll(filepath.Join(dir, name))
	}
	d.detached = map[string]*detachedMarker{}
	for _, m := range good {
		if len(d.detached) >= maxDetachedMarkers {
			d.emit(Event{Kind: "alert", Message: fmt.Sprintf("more than %d detached run markers; %s and later ones are left unread", maxDetachedMarkers, m.RunID)})
			break
		}
		d.detached[m.RunID] = m
	}
}

// saveDetached writes one marker atomically.
func (d *Dispatcher) saveDetached(m *detachedMarker) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > maxDetachedMarkerBytes {
		return fmt.Errorf("marker larger than %d bytes", maxDetachedMarkerBytes)
	}
	if err := os.MkdirAll(d.detachedDir(), 0o700); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(d.detachedDir(), m.RunID+".json"), raw)
}

// retireDetached removes a marker. A failed removal is reported and the
// marker stays in memory for this run, so it is tried again.
func (d *Dispatcher) retireDetached(m *detachedMarker) {
	if err := os.Remove(filepath.Join(d.detachedDir(), m.RunID+".json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		d.emit(Event{Kind: "alert", Ticket: m.Ticket, Message: fmt.Sprintf("could not remove detached run marker %s: %v", m.RunID, err)})
		return
	}
	delete(d.detached, m.RunID)
}

// sameRun reports that run is the run m deferred for: the same run,
// attempt, generation and supervisor start identity.
func sameRun(m *detachedMarker, run *DetachedRun) bool {
	return run != nil && run.RunID == m.RunID && run.AttemptID == m.AttemptID && run.Generation == m.Generation &&
		run.SupervisorPID == m.SupervisorPID && run.SupervisorIdentity == m.SupervisorIdentity
}

// runDeadline is when a RUNNING run stops deferring anything.
func runDeadline(run DetachedRun) time.Time {
	return run.LaunchedAt.Add(time.Duration(run.TimeoutSeconds)*time.Second + detachedSettle)
}

// verifiedRuns returns the RUNNING runs of attempt a whose supervisor is a
// live session leader with its recorded start identity, other than the
// worker leader (CAL-V0-145). Any read error returns none.
func (d *Dispatcher) verifiedRuns(a Attempt, leader int) []DetachedRun {
	r, ok := d.Queue.(RunReader)
	if !ok {
		return nil
	}
	runs, err := r.AttemptRuns(a.ID)
	if err != nil {
		return nil
	}
	var out []DetachedRun
	for _, run := range runs {
		if run.State == RunRunning && run.AttemptID == a.ID && run.Generation == a.Generation && validRunName(run.RunID) &&
			detachedSupervisor(run.SupervisorPID, run.SupervisorIdentity, leader) {
			out = append(out, run)
		}
	}
	return out
}

// noteHeld keeps, from an observation, the live attempts each recorded
// worker holds, so supervision can exempt their runs' supervisors on a tick
// whose observation failed.
func (d *Dispatcher) noteHeld(obs *Observation) {
	if obs == nil {
		return
	}
	d.heldBy = map[string][]Attempt{}
	for _, a := range obs.Attempts {
		if a.Live && d.worker(a.Holder) != nil {
			d.heldBy[a.Holder] = append(d.heldBy[a.Holder], a)
		}
	}
}

// exemptFor returns the verified supervisors of the detached runs of the
// attempts w holds, by PID with their start identity (CAL-V0-145).
func (d *Dispatcher) exemptFor(w *Worker) map[int]string {
	var out map[int]string
	for _, a := range d.heldBy[w.ID] {
		for _, run := range d.verifiedRuns(a, w.PID) {
			if out == nil {
				out = map[int]string{}
			}
			out[run.SupervisorPID] = run.SupervisorIdentity
		}
	}
	return out
}

// deferHandoffs is the CAL-V0-146 deferral of the hand-off of each ended
// worker's attempt while a verified detached run of it is RUNNING. It marks
// deferred attempts done and records each deferred worker for finish.
func (d *Dispatcher) deferHandoffs(obs *Observation, ended []*Worker, done map[string]bool) map[string]bool {
	fresh := map[string]bool{}
	if _, ok := d.Queue.(RunReader); !ok && len(d.detached) == 0 {
		return fresh
	}
	now := d.Now()
	for _, w := range ended {
		if w.Ticket == "" || w.Key != w.Ticket {
			continue // lane sessions keep today's hand-off
		}
		for _, a := range obs.Attempts {
			if !a.Live || a.Holder != w.ID || done[a.ID] || d.recoveries[a.ID] != nil {
				continue
			}
			// A worker re-reported as ended (after a restart, or kept in
			// the ledger by finish) finds its marker and records no second
			// one; advanceDetached still advances that marker and decides
			// whether the hand-off waits.
			var kept *detachedMarker
			for _, m := range d.detached {
				if m.Worker == w.ID && m.AttemptID == a.ID {
					kept = m
				}
			}
			if kept != nil {
				d.deferredNow[w.ID] = kept.RunID
				continue
			}
			runs := d.verifiedRuns(a, w.PID)
			if len(runs) == 0 {
				continue
			}
			// The run whose deferral lasts longest names the relaunch.
			sort.Slice(runs, func(i, j int) bool {
				di, dj := runDeadline(runs[i]), runDeadline(runs[j])
				if !di.Equal(dj) {
					return di.After(dj)
				}
				return runs[i].RunID > runs[j].RunID
			})
			run := runs[0]
			until := runDeadline(run)
			if !now.Before(until) {
				continue
			}
			if len(d.detached) >= maxDetachedMarkers {
				d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: w.ID, Message: fmt.Sprintf("%d detached runs are already tracked; %s is handed off now although its detached run %s still runs", maxDetachedMarkers, d.local(obs, a.Ticket), run.RunID)})
				continue
			}
			m := &detachedMarker{Profile: detachedProfile, RunID: run.RunID, AttemptID: a.ID, Generation: a.Generation, Ticket: a.Ticket, Role: w.Role, Worker: w.ID, SupervisorPID: run.SupervisorPID, SupervisorIdentity: run.SupervisorIdentity, Phase: DetachedDeferred, DeferredAt: now.UTC(), DeferUntil: until.UTC()}
			if old := d.detached[run.RunID]; old != nil {
				continue // the same run already defers another attempt or worker
			}
			if d.detachedHeld[run.RunID] {
				d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: w.ID, Message: fmt.Sprintf("detached run %s has a marker of another version; %s is handed off now", run.RunID, d.local(obs, a.Ticket))})
				continue
			}
			if err := d.saveDetached(m); err != nil {
				d.emit(Event{Kind: "alert", Ticket: a.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not record the deferral for detached run %s; %s is handed off now: %v", run.RunID, d.local(obs, a.Ticket), err)})
				continue
			}
			if d.detached == nil {
				d.detached = map[string]*detachedMarker{}
			}
			d.detached[m.RunID] = m
			fresh[m.RunID] = true
			done[a.ID] = true
			d.deferredNow[w.ID] = m.RunID
			d.emit(Event{Kind: "handoff", Ticket: a.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("deferred the hand-off of %s (attempt %s) after %s ended: its detached run %s is still running, at most until %s", d.local(obs, a.Ticket), a.ID, w.ID, run.RunID, until.UTC().Format(time.RFC3339)), Detail: map[string]string{"attempt": a.ID, "generation": a.Generation, "phase": a.Phase, "handoff": "DEFERRED", "detachedRun": run.RunID}})
		}
	}
	return fresh
}

// advanceDetached moves every marker not created this tick one step
// (CAL-V0-147, CAL-V0-148). It reports whether it attempted a store write.
func (d *Dispatcher) advanceDetached(ctx context.Context, obs *Observation, done, fresh map[string]bool) bool {
	wrote := false
	reader, _ := d.Queue.(RunReader)
	now := d.Now()
	for _, id := range slices.Sorted(maps.Keys(d.detached)) {
		if ctx.Err() != nil {
			return wrote
		}
		m := d.detached[id]
		if fresh[id] {
			continue
		}
		var run *DetachedRun
		readErr := errors.New("no run reader")
		if reader != nil && (m.Phase == DetachedDeferred || m.Phase == DetachedHandedOff) {
			run, readErr = reader.AttemptRun(m.AttemptID, m.RunID)
		}
		running := readErr == nil && sameRun(m, run) && run.State == RunRunning && now.Before(m.DeferUntil)
		if running && !detachedSupervisor(run.SupervisorPID, run.SupervisorIdentity, 0) {
			// It may have finished between the record read and the check.
			if again, err := reader.AttemptRun(m.AttemptID, m.RunID); err == nil && sameRun(m, again) && again.State == RunFinished {
				run = again
			}
			running = false
		}
		finished := readErr == nil && sameRun(m, run) && run.State == RunFinished && run.ExitStatus != nil
		switch m.Phase {
		case DetachedDeferred:
			var a *Attempt
			for i := range obs.Attempts {
				if obs.Attempts[i].ID == m.AttemptID {
					a = &obs.Attempts[i]
				}
			}
			ours := a != nil && a.Live && a.Generation == m.Generation && a.Holder == m.Worker
			if ours && (done[a.ID] || d.recoveries[a.ID] != nil) {
				continue
			}
			if ours && running {
				done[a.ID] = true
				continue
			}
			if ours {
				done[a.ID], wrote = true, true
				d.handoff(ctx, obs, m.Worker, m.Role, *a, map[string]string{"detachedRun": m.RunID})
			}
			m.Phase = DetachedHandedOff
			if !finished && !running {
				d.endDetached(obs, m, run, readErr)
				continue
			}
			fallthrough
		case DetachedHandedOff:
			if finished {
				m.Phase, m.ExitStatus, m.ReadyAt = DetachedRelaunch, run.ExitStatus, now.UTC()
				m.Result, m.Output = run.Result, run.Output
				if err := d.saveDetached(m); err != nil {
					d.emit(Event{Kind: "alert", Ticket: m.Ticket, Message: fmt.Sprintf("could not record that detached run %s finished; it is retried next tick: %v", m.RunID, err)})
				}
				continue
			}
			if running {
				if err := d.saveDetached(m); err != nil {
					d.emit(Event{Kind: "alert", Ticket: m.Ticket, Message: fmt.Sprintf("could not record the hand-off for detached run %s: %v", m.RunID, err)})
				}
				continue
			}
			d.endDetached(obs, m, run, readErr)
		case DetachedRelaunch:
			if reason := d.relaunchMoot(obs, m, now); reason != "" {
				d.emit(Event{Kind: "alert", Ticket: m.Ticket, Message: fmt.Sprintf("no session is relaunched for detached run %s of %s: %s", m.RunID, d.local(obs, m.Ticket), reason), Detail: map[string]string{"detachedRun": m.RunID, "attempt": m.AttemptID}})
				d.retireDetached(m)
			}
		case DetachedLaunched:
			if d.worker(m.Successor) == nil {
				d.retireDetached(m)
			}
		}
	}
	return wrote
}

// endDetached retires a marker whose run ended without a kept result, or
// whose record is missing or unreadable: no session is relaunched for it.
func (d *Dispatcher) endDetached(obs *Observation, m *detachedMarker, run *DetachedRun, readErr error) {
	state := "SUPERVISOR_LOST"
	switch {
	case readErr != nil:
		state = StateUnknown
	case run == nil:
		state = "MISSING"
	case !sameRun(m, run):
		state = "REPLACED"
	case run.State == RunRunning && !d.Now().Before(m.DeferUntil):
		state = "OVERDUE"
	}
	msg := fmt.Sprintf("detached run %s of %s ended without a kept result (%s); no session is relaunched for it", m.RunID, d.local(obs, m.Ticket), state)
	if readErr != nil {
		msg += ": " + readErr.Error()
	}
	d.emit(Event{Kind: "alert", Ticket: m.Ticket, Message: msg, Detail: map[string]string{"detachedRun": m.RunID, "attempt": m.AttemptID, "runState": state}})
	d.retireDetached(m)
}

// relaunchMoot names why a finished run's relaunch no longer applies, or
// returns "".
func (d *Dispatcher) relaunchMoot(obs *Observation, m *detachedMarker, now time.Time) string {
	if r := d.role(m.Role); r == nil || r.Lane != nil || r.Cap == 0 {
		return "role " + m.Role + " no longer launches ticket sessions"
	}
	if now.Sub(m.ReadyAt) > detachedRelaunchWindow {
		return "it waited longer than " + detachedRelaunchWindow.String()
	}
	seen := false
	for _, t := range obs.Tickets {
		seen = seen || t.ID == m.Ticket
	}
	if !seen {
		return "the ticket is no longer observed"
	}
	for _, a := range obs.Attempts {
		if a.Live && a.Ticket == m.Ticket && a.Holder != m.Worker {
			return "another holder took the ticket"
		}
	}
	// The run's own ended worker may stay in the ledger while its progress
	// is accounted; the roster launches nothing on the ticket until it leaves.
	for _, w := range d.ledger.Workers {
		if w.Ticket == m.Ticket && w.ID != m.Worker {
			return "worker " + w.ID + " already runs on the ticket"
		}
	}
	return ""
}

// relaunches returns the pending relaunch of each ticket: the marker whose
// run became ready first.
func (d *Dispatcher) relaunches() map[string]*detachedMarker {
	out := map[string]*detachedMarker{}
	for _, m := range d.detached {
		if m.Phase != DetachedRelaunch {
			continue
		}
		if old := out[m.Ticket]; old == nil || m.ReadyAt.Before(old.ReadyAt) || m.ReadyAt.Equal(old.ReadyAt) && m.RunID < old.RunID {
			out[m.Ticket] = m
		}
	}
	return out
}

// preferredRoles maps each ticket with a pending relaunch to the role that
// relaunches it (CAL-V0-149).
func preferredRoles(relaunch map[string]*detachedMarker) map[string]string {
	out := map[string]string{}
	for t, m := range relaunch {
		out[t] = m.Role
	}
	return out
}

// detachedLine is the one-line {detachedRun} rendering of a relaunch's run
// outcome (CAL-V0-149).
func detachedLine(m *detachedMarker) string {
	return fmt.Sprintf("detached run %s of attempt %s finished with exit status %d; result %s; output %s; replay with: corvint-tasks run --attach --attempt %s --run %s",
		m.RunID, m.AttemptID, *m.ExitStatus, orText(m.Result, "NOT_OBSERVED"), orText(m.Output, "NOT_OBSERVED"), m.AttemptID, m.RunID)
}

// detachedEnv is the CAL-V0-149 launch environment of a relaunch.
func detachedEnv(m *detachedMarker) []string {
	return []string{"CORVINT_DISPATCH_RUN_ID=" + m.RunID, "CORVINT_DISPATCH_RUN_ATTEMPT=" + m.AttemptID, "CORVINT_DISPATCH_RUN_EXIT=" + strconv.Itoa(*m.ExitStatus), "CORVINT_DISPATCH_RUN_RESULT=" + m.Result, "CORVINT_DISPATCH_RUN_OUTPUT=" + m.Output}
}

// DetachedView is the read-only status view of one marker (CAL-V0-150).
type DetachedView struct {
	RunID, AttemptID, Generation, Ticket, Role, Worker, Phase, Successor string
	SupervisorPID                                                        int
	SupervisorIdentity                                                   string
	DeferUntil                                                           time.Time
	ExitStatus                                                           *int
}

// DetachedMarkers reads a dispatcher's markers for status without writing:
// at most maxDetachedEntries entries and maxDetachedMarkers markers, each at
// most 4 KiB. It returns the entries that do not decode, by name, with their
// error; another profile version is UNSUPPORTED_VERSION.
func DetachedMarkers(dir string) ([]DetachedView, map[string]error, error) {
	good, bad, _, err := readDetachedMarkers(filepath.Join(dir, detachedDirName))
	if err != nil {
		return nil, nil, err
	}
	var out []DetachedView
	for _, m := range good {
		if len(out) >= maxDetachedMarkers {
			break
		}
		out = append(out, DetachedView{RunID: m.RunID, AttemptID: m.AttemptID, Generation: m.Generation, Ticket: m.Ticket, Role: m.Role, Worker: m.Worker, Phase: m.Phase, Successor: m.Successor, SupervisorPID: m.SupervisorPID, SupervisorIdentity: m.SupervisorIdentity, DeferUntil: m.DeferUntil, ExitStatus: m.ExitStatus})
	}
	return out, bad, nil
}

// RunState is the CAL-V0-150 state of a marker's run from one record read:
// RUNNING, FINISHED or SUPERVISOR_LOST, MISSING when the record is gone,
// REPLACED when it names another run or supervisor, or UNKNOWN when it or
// the supervisor's identity cannot be read.
func RunState(v DetachedView, run *DetachedRun, err error) string {
	m := &detachedMarker{RunID: v.RunID, AttemptID: v.AttemptID, Generation: v.Generation, SupervisorPID: v.SupervisorPID, SupervisorIdentity: v.SupervisorIdentity}
	switch {
	case err != nil:
		return StateUnknown
	case run == nil:
		return "MISSING"
	case !sameRun(m, run):
		return "REPLACED"
	case run.State == RunFinished:
		return RunFinished
	}
	id, err := supervisor.ProcessIdentity(run.SupervisorPID)
	switch {
	case err != nil:
		return StateUnknown
	case id != "" && id == run.SupervisorIdentity:
		return RunRunning
	}
	return "SUPERVISOR_LOST"
}
