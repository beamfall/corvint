package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// HostSleepProfile is the ATR-V0-063 host-sleep record's profile.
const HostSleepProfile = "taskman-dispatch-host-sleep/0"

const (
	hostSleepFile = "host-sleep.json"
	maxHostSleep  = 64 << 10
	// maxSleepGaps bounds the gaps the record keeps, newest last.
	maxSleepGaps = 64
	// minSleepGap is the smallest wall-over-monotonic jump recorded as a
	// suspend; a smaller one is clock noise or a small wall-clock step.
	minSleepGap = 30 * time.Second
)

// SleepGap is one detected host suspend: wall time [From, To] in which the
// monotonic clock did not advance by Seconds.
type SleepGap struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Seconds int    `json:"seconds"`
}

func (g SleepGap) span() (time.Time, time.Time, bool) {
	from, e1 := time.Parse(time.RFC3339, g.From)
	to, e2 := time.Parse(time.RFC3339, g.To)
	return from, to, e1 == nil && e2 == nil && g.Seconds > 0 && !to.Before(from)
}

// HostSleep is the record of the program's recent suspend gaps.
type HostSleep struct {
	Profile string     `json:"profile"`
	Program string     `json:"program"`
	Gaps    []SleepGap `json:"gaps"`
}

// HostSleepPath is the record under the program's state directory.
func HostSleepPath(dir string) string { return filepath.Join(dir, hostSleepFile) }

// ReadHostSleep reads the record at path. A missing file is
// fs.ErrNotExist; another version is UNSUPPORTED_VERSION (CAL-V0-131).
func ReadHostSleep(path string) (*HostSleep, error) {
	raw, err := readBounded(path, maxHostSleep)
	if err != nil {
		return nil, err
	}
	if err := wire.RawProfileVersion("/profile", raw, HostSleepProfile); err != nil {
		return nil, err
	}
	var h HostSleep
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&h); err != nil {
		return nil, err
	}
	if h.Program == "" || len(h.Gaps) == 0 || len(h.Gaps) > maxSleepGaps {
		return nil, errors.New("host sleep record is incomplete")
	}
	for _, g := range h.Gaps {
		if _, _, ok := g.span(); !ok {
			return nil, errors.New("host sleep record has a malformed gap")
		}
	}
	return &h, nil
}

// noteHostSleep compares the wall and monotonic clocks since the previous
// tick and records a jump of at least minSleepGap as one suspend gap
// (ATR-V0-063). It runs before any cap or lease decision of the tick. The
// monotonic reading is the one time.Now carries, or Mono when set; a clock
// without either (an injected wall clock) never detects a gap.
func (d *Dispatcher) noteHostSleep() {
	now := d.Now()
	var mono time.Duration
	if d.Mono != nil {
		mono = d.Mono()
	}
	prev, prevMono, had := d.lastNow, d.lastMono, !d.lastNow.IsZero()
	d.lastNow, d.lastMono = now, mono
	if !had {
		d.loadSleep()
		return
	}
	elapsed := now.Sub(prev) // monotonic when both readings carry it
	if d.Mono != nil {
		elapsed = mono - prevMono
	}
	gap := now.Round(0).Sub(prev.Round(0)) - elapsed
	if gap < minSleepGap {
		return
	}
	prevWall := prev.Round(0)
	gap = gap.Truncate(time.Second)
	g := SleepGap{From: prevWall.UTC().Format(time.RFC3339), To: prevWall.Add(gap).UTC().Format(time.RFC3339), Seconds: int(gap / time.Second)}
	d.loadSleep()
	d.sleep.Gaps = append(d.sleep.Gaps, g)
	if len(d.sleep.Gaps) > maxSleepGaps {
		d.sleep.Gaps = d.sleep.Gaps[len(d.sleep.Gaps)-maxSleepGaps:]
	}
	path := HostSleepPath(d.dir)
	raw, _ := json.Marshal(d.sleep)
	detail := map[string]string{"hostSlept": strconv.Itoa(g.Seconds), "from": g.From, "to": g.To}
	msg := fmt.Sprintf("the host was suspended for about %ds between %s and %s; idle, wall-time and expired-lease timers exclude the gap", g.Seconds, g.From, g.To)
	if d.sleepForeign {
		msg += "; the newer-version record " + path + " is kept unchanged"
	} else if err := writeAtomic(path, append(raw, '\n')); err != nil {
		msg += "; the record " + path + " could not be written: " + err.Error()
	}
	d.emit(Event{Kind: "alert", Message: msg, Detail: detail})
}

// loadSleep reads the record once. A newer version is kept on disk after
// one alert and the gaps are then held in memory only; another unreadable
// record is replaced, so the gap just detected is never lost.
func (d *Dispatcher) loadSleep() {
	if d.sleep != nil {
		return
	}
	path := HostSleepPath(d.dir)
	h, err := ReadHostSleep(path)
	switch {
	case err == nil && h.Program == d.Program:
		d.sleep = h
		return
	case err == nil, errors.Is(err, fs.ErrNotExist):
	case wire.CodeOf(err) == wire.CodeUnsupportedVersion:
		d.sleepForeign = true
		d.emit(Event{Kind: "alert", Message: "the host sleep record " + path + " is a newer version; it is kept and suspend gaps are held in memory only: " + err.Error()})
	default:
		d.emit(Event{Kind: "alert", Message: "the host sleep record " + path + " does not read and is replaced: " + err.Error()})
	}
	d.sleep = &HostSleep{Profile: HostSleepProfile, Program: d.Program}
}

// slept is the suspended time inside [since, now] across the recorded gaps
// (ATR-V0-064). Without a recorded gap it is zero.
func (d *Dispatcher) slept(since, now time.Time) time.Duration {
	if d.sleep == nil {
		return 0
	}
	var total time.Duration
	for _, g := range d.sleep.Gaps {
		from, to, _ := g.span()
		lo, hi := from, to
		if since.After(lo) {
			lo = since
		}
		if now.Before(hi) {
			hi = now
		}
		if hi.After(lo) {
			total += hi.Sub(lo)
		}
	}
	return total
}
