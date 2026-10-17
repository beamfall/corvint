package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SessionChurnProfile is the ATR-V0-061 short-session record's profile.
const SessionChurnProfile = "taskman-dispatch-session-churn/0"

const (
	sessionChurnFile = "session-churn.json"
	maxSessionChurn  = 1 << 20
	// maxChurnDurations bounds the durations one ticket's streak keeps.
	maxChurnDurations = 100
	// ChurnReason is the hold's reason in its cooldown event detail.
	ChurnReason = "SHORT_SESSIONS"
)

// ShortSessions is the opt-in ATR-V0-060 cooldown for repeated short
// sessions on one ticket: after Count consecutive finished sessions each
// shorter than Seconds, new launches for the ticket wait HoldSeconds from
// the last session's end.
type ShortSessions struct {
	Seconds     int `json:"seconds"`
	Count       int `json:"count"`
	HoldSeconds int `json:"holdSeconds"`
}

func (s *ShortSessions) validate() error {
	if s.Seconds < 1 || s.Seconds > 86400 || s.Count < 2 || s.Count > maxChurnDurations || s.HoldSeconds < 1 || s.HoldSeconds > 86400 {
		return fmt.Errorf("shortSessions needs seconds 1..86400, count 2..%d and holdSeconds 1..86400", maxChurnDurations)
	}
	return nil
}

// ChurnStreak is one ticket's current run of short sessions.
type ChurnStreak struct {
	// Durations holds the streak's newest session lengths in seconds.
	Durations []int  `json:"durations"`
	Streak    int    `json:"streak"`
	FirstSeen string `json:"firstSeen"`
	LastEnd   string `json:"lastEnd"`
	HoldUntil string `json:"holdUntil,omitempty"`
}

// MedianSeconds is the median of the streak's kept durations.
func (s *ChurnStreak) MedianSeconds() int {
	d := append([]int(nil), s.Durations...)
	if len(d) == 0 {
		return 0
	}
	sort.Ints(d)
	if len(d)%2 == 1 {
		return d[len(d)/2]
	}
	return (d[len(d)/2-1] + d[len(d)/2]) / 2
}

// Held reports whether the streak holds new launches at now.
func (s *ChurnStreak) Held(now time.Time) bool {
	t, err := time.Parse(time.RFC3339, s.HoldUntil)
	return err == nil && t.After(now)
}

// SessionChurn is the record file of every ticket's short-session streak.
type SessionChurn struct {
	Profile string                  `json:"profile"`
	Program string                  `json:"program"`
	Tickets map[string]*ChurnStreak `json:"tickets"`
}

// SessionChurnPath is the record under the program's state directory.
func SessionChurnPath(dir string) string { return filepath.Join(dir, sessionChurnFile) }

// ReadSessionChurn reads the record at path. A missing file is
// fs.ErrNotExist; another version is UNSUPPORTED_VERSION (CAL-V0-131).
func ReadSessionChurn(path string) (*SessionChurn, error) {
	raw, err := readBounded(path, maxSessionChurn)
	if err != nil {
		return nil, err
	}
	if err := wire.RawProfileVersion("/profile", raw, SessionChurnProfile); err != nil {
		return nil, err
	}
	var c SessionChurn
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, err
	}
	if c.Program == "" || c.Tickets == nil {
		return nil, errors.New("session churn record is incomplete")
	}
	for k, s := range c.Tickets {
		if k == "" || s == nil || s.Streak < 1 || len(s.Durations) == 0 || len(s.Durations) > maxChurnDurations {
			return nil, errors.New("session churn record has a malformed streak")
		}
		for _, t := range []string{s.FirstSeen, s.LastEnd} {
			if _, err := time.Parse(time.RFC3339, t); err != nil {
				return nil, errors.New("session churn record has a malformed time")
			}
		}
		if _, err := time.Parse(time.RFC3339, s.HoldUntil); s.HoldUntil != "" && err != nil {
			return nil, errors.New("session churn record has a malformed hold")
		}
	}
	return &c, nil
}

// shortSessionsFor is the applied policy of role: the role's own, else the
// global one, else none (ATR-V0-060).
func (d *Dispatcher) shortSessionsFor(role string) *ShortSessions {
	if r := d.role(role); r != nil && r.ShortSessions != nil {
		return r.ShortSessions
	}
	return d.Config.ShortSessions
}

// shortSessionsConfigured reports whether any applied policy exists.
func (c *Config) shortSessionsConfigured() bool {
	if c.ShortSessions != nil {
		return true
	}
	for _, r := range c.Roles {
		if r.ShortSessions != nil {
			return true
		}
	}
	return false
}

// loadChurn reads the record once per dispatcher. A newer version is
// preserved and disables tracking; another unreadable record is replaced
// after one alert (ATR-V0-061).
func (d *Dispatcher) loadChurn() bool {
	if d.churn != nil {
		return true
	}
	if d.churnForeign {
		return false
	}
	path := SessionChurnPath(d.dir)
	c, err := ReadSessionChurn(path)
	switch {
	case err == nil && c.Program == d.Program:
		d.churn = c
		return true
	case errors.Is(err, fs.ErrNotExist):
	case wire.CodeOf(err) == wire.CodeUnsupportedVersion:
		d.churnForeign = true
		d.emit(Event{Kind: "alert", Message: "the session churn record " + path + " is a newer version; it is kept and short sessions are not tracked: " + err.Error()})
		return false
	default:
		msg := "the session churn record " + path + " does not read and is replaced"
		if err != nil {
			msg += ": " + err.Error()
		}
		d.emit(Event{Kind: "alert", Message: msg})
	}
	d.churn = &SessionChurn{Profile: SessionChurnProfile, Program: d.Program, Tickets: map[string]*ChurnStreak{}}
	return true
}

func (d *Dispatcher) saveChurn() {
	path := SessionChurnPath(d.dir)
	if len(d.churn.Tickets) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			d.emit(Event{Kind: "alert", Message: "the session churn record " + path + " could not be removed: " + err.Error()})
		}
		return
	}
	raw, _ := json.Marshal(d.churn)
	if err := writeAtomic(path, append(raw, '\n')); err != nil {
		d.emit(Event{Kind: "alert", Message: "the session churn record " + path + " could not be written: " + err.Error()})
	}
}

// syncChurn drops the record when no policy is applied, so that removing
// the setting lifts every hold on the next tick (ATR-V0-060).
func (d *Dispatcher) syncChurn() {
	if d.Config.shortSessionsConfigured() {
		return
	}
	if d.churn == nil {
		if _, err := os.Stat(SessionChurnPath(d.dir)); err != nil || !d.loadChurn() {
			return
		}
	}
	if len(d.churn.Tickets) > 0 {
		d.churn.Tickets = map[string]*ChurnStreak{}
		d.saveChurn()
	}
}

// noteSession extends or resets the ticket's short-session streak after one
// finished session and holds its next launch when the streak reaches the
// policy's count (ATR-V0-060). Running sessions are never touched.
func (d *Dispatcher) noteSession(w *Worker, now time.Time) {
	p := d.shortSessionsFor(w.Role)
	if p == nil || strings.HasPrefix(w.Key, "lane:") || !d.loadChurn() {
		return
	}
	secs := int(now.Sub(w.Started).Seconds())
	if secs >= p.Seconds {
		if _, ok := d.churn.Tickets[w.Key]; ok {
			delete(d.churn.Tickets, w.Key)
			d.saveChurn()
		}
		return
	}
	s := d.churn.Tickets[w.Key]
	if s == nil {
		s = &ChurnStreak{FirstSeen: now.UTC().Format(time.RFC3339)}
		d.churn.Tickets[w.Key] = s
	}
	s.Streak++
	s.Durations = append(s.Durations, secs)
	if len(s.Durations) > maxChurnDurations {
		s.Durations = s.Durations[len(s.Durations)-maxChurnDurations:]
	}
	s.LastEnd = now.UTC().Format(time.RFC3339)
	if s.Streak >= p.Count {
		s.HoldUntil = now.Add(time.Duration(p.HoldSeconds) * time.Second).UTC().Format(time.RFC3339)
		d.emit(Event{Kind: "cooldown", Ticket: w.Ticket, Role: w.Role, Message: fmt.Sprintf("%s holds new launches until %s after %d consecutive sessions shorter than %ds (median %ds, first seen %s)", d.keyText(w.Key), s.HoldUntil, s.Streak, p.Seconds, s.MedianSeconds(), s.FirstSeen),
			Detail: map[string]string{"reason": ChurnReason, "sessions": strconv.Itoa(s.Streak), "medianSeconds": strconv.Itoa(s.MedianSeconds()), "firstSeen": s.FirstSeen, "holdUntil": s.HoldUntil}})
	}
	d.saveChurn()
}

// churnHeld marks every ticket whose streak holds launches at now.
func (d *Dispatcher) churnHeld(skip map[string]bool, now time.Time) {
	if !d.Config.shortSessionsConfigured() || !d.loadChurn() {
		return
	}
	for k, s := range d.churn.Tickets {
		if s.Held(now) {
			skip[k] = true
		}
	}
}
