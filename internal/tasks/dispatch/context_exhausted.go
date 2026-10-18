package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ContextExhaustedProfile is the ATR-V0-107 hard-stop record's profile.
const ContextExhaustedProfile = "taskman-dispatch-context-exhausted/0"

// Hard-stop states (ATR-V0-106).
const (
	ContextCounting  = "COUNTING"
	ContextExhausted = "CONTEXT_EXHAUSTED"
)

const (
	contextExhaustedFile = "context-exhausted.json"
	maxContextExhausted  = 1 << 20
	// defaultExhaustedAfter is exhaustedAfter when the role sets none.
	defaultExhaustedAfter = 3
	maxExhaustedAfter     = 100
	maxHardStopEntries    = 4096
)

// HardStopStreak is one ticket and role's run of consecutive hard stops.
type HardStopStreak struct {
	Key        string `json:"key"`
	Role       string `json:"role"`
	HardStops  int    `json:"hardStops"`
	State      string `json:"state"`
	LastWorker string `json:"lastWorker"`
	LastAt     string `json:"lastAt"`
}

// ContextExhaustion is the record of every ticket and role's streak.
type ContextExhaustion struct {
	Profile string           `json:"profile"`
	Program string           `json:"program"`
	Entries []HardStopStreak `json:"entries"`
}

// ContextExhaustedPath is the record under the program's state directory.
func ContextExhaustedPath(dir string) string { return filepath.Join(dir, contextExhaustedFile) }

// exhaustedAfter is the role's applied ATR-V0-106 count.
func (c *ContextLimits) exhaustedAfter() int {
	if c.ExhaustedAfter == 0 {
		return defaultExhaustedAfter
	}
	return int(c.ExhaustedAfter)
}

// ReadContextExhausted reads the record at path. A missing file is
// fs.ErrNotExist; another version is UNSUPPORTED_VERSION (CAL-V0-131).
func ReadContextExhausted(path string) (*ContextExhaustion, error) {
	raw, err := readBounded(path, maxContextExhausted)
	if err != nil {
		return nil, err
	}
	if err := wire.RawProfileVersion("/profile", raw, ContextExhaustedProfile); err != nil {
		return nil, err
	}
	var c ContextExhaustion
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, err
	}
	if c.Program == "" || c.Entries == nil || len(c.Entries) > maxHardStopEntries {
		return nil, errors.New("context exhaustion record is incomplete")
	}
	for _, e := range c.Entries {
		if _, err := time.Parse(time.RFC3339, e.LastAt); err != nil || e.Key == "" || e.Role == "" || e.LastWorker == "" || e.HardStops < 1 || (e.State != ContextCounting && e.State != ContextExhausted) {
			return nil, errors.New("context exhaustion record has a malformed entry")
		}
	}
	return &c, nil
}

// loadExhaustion reads the record once per dispatcher. A newer version is
// preserved and disables counting; another unreadable record is replaced
// after one alert (ATR-V0-107).
func (d *Dispatcher) loadExhaustion() bool {
	if d.exhaustion != nil {
		return true
	}
	if d.exhaustionForeign {
		return false
	}
	path := ContextExhaustedPath(d.dir)
	c, err := ReadContextExhausted(path)
	switch {
	case err == nil && c.Program == d.Program:
		d.exhaustion = c
		return true
	case errors.Is(err, fs.ErrNotExist):
	case wire.CodeOf(err) == wire.CodeUnsupportedVersion:
		d.exhaustionForeign = true
		d.emit(Event{Kind: "alert", Message: "the context exhaustion record " + path + " is a newer version; it is kept and hard stops are not counted: " + err.Error()})
		return false
	default:
		msg := "the context exhaustion record " + path + " does not read and is replaced"
		if err != nil {
			msg += ": " + err.Error()
		}
		d.emit(Event{Kind: "alert", Message: msg})
	}
	d.exhaustion = &ContextExhaustion{Profile: ContextExhaustedProfile, Program: d.Program, Entries: []HardStopStreak{}}
	return true
}

func (d *Dispatcher) saveExhaustion() {
	path := ContextExhaustedPath(d.dir)
	if len(d.exhaustion.Entries) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			d.emit(Event{Kind: "alert", Message: "the context exhaustion record " + path + " could not be removed: " + err.Error()})
		}
		return
	}
	raw, _ := json.Marshal(d.exhaustion)
	if err := writeAtomic(path, append(raw, '\n')); err != nil {
		d.emit(Event{Kind: "alert", Message: "the context exhaustion record " + path + " could not be written: " + err.Error()})
	}
}

// noteHardStop extends or resets the ticket and role's streak of hard stops
// after one finished session, and marks it CONTEXT_EXHAUSTED when it reaches
// the role's exhaustedAfter, alerting once after the record is saved
// (ATR-V0-106). Any other ending resets the streak even when the role no
// longer counts. A finish replayed after a restart, whose worker is already
// the streak's LastWorker, changes nothing. It holds nothing.
func (d *Dispatcher) noteHardStop(w *Worker, now time.Time) {
	if strings.HasPrefix(w.Key, "lane:") {
		return
	}
	r := d.role(w.Role)
	counting := r != nil && r.Context != nil && r.Context.HardTokens != 0
	if w.KillReason != "CONTEXT" {
		if !counting && d.exhaustion == nil && !d.exhaustionForeign {
			if _, err := os.Lstat(ContextExhaustedPath(d.dir)); err != nil {
				return
			}
		}
		if !d.loadExhaustion() {
			return
		}
		if i := d.streakIndex(w.Key, w.Role); i >= 0 {
			d.exhaustion.Entries = append(d.exhaustion.Entries[:i], d.exhaustion.Entries[i+1:]...)
			d.saveExhaustion()
		}
		return
	}
	if !counting || !d.loadExhaustion() {
		return
	}
	i := d.streakIndex(w.Key, w.Role)
	if i >= 0 && d.exhaustion.Entries[i].LastWorker == w.ID {
		return
	}
	if i < 0 {
		if len(d.exhaustion.Entries) >= maxHardStopEntries {
			d.exhaustion.Entries = d.exhaustion.Entries[1:]
		}
		d.exhaustion.Entries = append(d.exhaustion.Entries, HardStopStreak{Key: w.Key, Role: w.Role, State: ContextCounting})
		i = len(d.exhaustion.Entries) - 1
	}
	e := &d.exhaustion.Entries[i]
	e.HardStops++
	e.LastWorker, e.LastAt = w.ID, now.UTC().Format(time.RFC3339)
	n := r.Context.exhaustedAfter()
	reached := e.HardStops >= n && e.State != ContextExhausted
	if reached {
		e.State = ContextExhausted
	}
	stops := e.HardStops
	d.saveExhaustion()
	if reached {
		d.emit(Event{Kind: "alert", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("%s is context-exhausted for role %s: %d consecutive sessions were stopped at hardTokens; split the ticket, raise the role's thresholds or hand it to a larger-context role", d.keyText(w.Key), w.Role, stops),
			Detail: map[string]string{"code": ContextExhausted, "role": w.Role, "hardStops": strconv.Itoa(stops), "exhaustedAfter": strconv.Itoa(n)}})
	}
}

// streakIndex is the entry of key and role, or -1.
func (d *Dispatcher) streakIndex(key, role string) int {
	for j, e := range d.exhaustion.Entries {
		if e.Key == key && e.Role == role {
			return j
		}
	}
	return -1
}
