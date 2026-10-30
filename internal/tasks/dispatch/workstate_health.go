package dispatch

import (
	"errors"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"
)

const (
	// defaultWorkStateEscalateAfter is the CAL-V0-271 threshold when the
	// configuration names none; maxWorkStateTicks bounds both thresholds.
	defaultWorkStateEscalateAfter = 3
	maxWorkStateTicks             = 1440
	maxWorkStateError             = 512
	maxWorkStateFailedTicks       = 1 << 30
	maxWorkStateTimeout           = 600
	maxWorkStateStale             = 86400
)

// WorkStateHealth is the CAL-V0-270 health of the configured work-state
// reader. OK is the outcome of the newest read; Since is when the current
// run of successful or failed reads began; FailedTicks counts the ticks
// with a failed read since the last successful one (0 while OK); LastError
// is the newest failure, kept after recovery; Escalated records that this
// failure episode raised its one CAL-V0-271 escalation.
type WorkStateHealth struct {
	OK          bool      `json:"ok"`
	Since       time.Time `json:"since"`
	LastError   string    `json:"lastError,omitempty"`
	FailedTicks int       `json:"failedTicks"`
	Escalated   bool      `json:"escalated,omitempty"`
}

func (h *WorkStateHealth) validate() error {
	if h == nil {
		return nil
	}
	if h.Since.IsZero() || h.FailedTicks < 0 || h.FailedTicks > maxWorkStateFailedTicks || len(h.LastError) > maxWorkStateError || !utf8.ValidString(h.LastError) {
		return errors.New("invalid work state health")
	}
	if h.OK != (h.FailedTicks == 0) || h.OK && h.Escalated {
		return errors.New("inconsistent work state health")
	}
	return nil
}

// noteWorkState emits one read's CAL-V0-053 alerts, the first of an outage
// marked UNAVAILABLE (CAL-V0-274), and records the read's outcome
// (CAL-V0-270). A read with any alert failed. A tick counts once however
// often it reads; a failure episode escalates once (CAL-V0-271) and its end
// emits one recovery event. Without a configured reader no health is kept.
func (d *Dispatcher) noteWorkState(alerts []string) {
	w := d.Config.WorkState
	now := d.Now()
	h := d.ledger.WorkState
	// CAL-V0-053 keeps one alert per failed read. The first one of an
	// outage is also its one provider-level UNAVAILABLE event (CAL-V0-274).
	for i, a := range alerts {
		e := Event{Kind: "alert", Message: a}
		if i == 0 && w != nil && (h == nil || h.OK) {
			since := now.UTC().Format(time.RFC3339)
			e.Message += "; work state unavailable since " + since
			e.Detail = map[string]string{"provider": "workState", "health": "UNAVAILABLE", "since": since}
		}
		d.emit(e)
	}
	if w == nil {
		d.ledger.WorkState = nil
		return
	}
	if len(alerts) == 0 {
		if h == nil {
			d.ledger.WorkState = &WorkStateHealth{OK: true, Since: now}
			return
		}
		if h.OK {
			return
		}
		d.emit(Event{Kind: "alert", Message: fmt.Sprintf("work state reader recovered after %d failed tick(s) since %s", h.FailedTicks, h.Since.UTC().Format(time.RFC3339)), Detail: map[string]string{"provider": "workState", "health": "RECOVERED", "failedTicks": strconv.Itoa(h.FailedTicks), "escalated": strconv.FormatBool(h.Escalated)}})
		h.OK, h.Since, h.FailedTicks, h.Escalated = true, now, 0, false
		d.holdAnnounced = false
		return
	}
	msg := alerts[0]
	if len(alerts) > 1 {
		msg += fmt.Sprintf(" (and %d more)", len(alerts)-1)
	}
	if h == nil || h.OK {
		h = &WorkStateHealth{Since: now}
		d.ledger.WorkState = h
	}
	h.LastError = boundUTF8(msg, maxWorkStateError)
	if h.FailedTicks > 0 && d.workStateTick == d.tickSeq {
		return // this tick already counted
	}
	d.workStateTick = d.tickSeq
	h.FailedTicks = min(h.FailedTicks+1, maxWorkStateFailedTicks)
	if !h.Escalated && h.FailedTicks >= w.EscalateAfter() {
		h.Escalated = true
		d.emit(Event{Kind: "needs-owner", Message: fmt.Sprintf("the work state reader failed on %d consecutive tick(s) since %s: %s. ticket work states are UNKNOWN until it recovers, and runs without progress meanwhile are recorded as such", h.FailedTicks, h.Since.UTC().Format(time.RFC3339), h.LastError), Detail: map[string]string{"provider": "workState", "health": "FAILING", "failedTicks": strconv.Itoa(h.FailedTicks)}})
	}
	held := WorkStateHoldsLaunches(d.Config, h)
	if held && !d.holdAnnounced {
		d.emit(Event{Kind: "throttled", Message: fmt.Sprintf("new launches are held after %d consecutive tick(s) without a work state (workState.holdLaunchesAfterTicks %d); they resume when a read succeeds", h.FailedTicks, w.HoldLaunchesAfterTicks), Detail: map[string]string{"provider": "workState", "failedTicks": strconv.Itoa(h.FailedTicks)}})
	}
	d.holdAnnounced = held
}

// WorkStateHoldsLaunches reports the CAL-V0-273 launch hold: the policy is
// set and at least that many consecutive ticks failed to read.
func WorkStateHoldsLaunches(c *Config, h *WorkStateHealth) bool {
	return c.WorkState != nil && c.WorkState.HoldLaunchesAfterTicks > 0 && h != nil && !h.OK && h.FailedTicks >= c.WorkState.HoldLaunchesAfterTicks
}

// StaleAfter is the CAL-V0-276 window: the configured seconds, or one tick.
func (w *WorkState) StaleAfter(tickSeconds int) time.Duration {
	if w.StaleAfterSeconds == 0 {
		return time.Duration(tickSeconds) * time.Second
	}
	return time.Duration(w.StaleAfterSeconds) * time.Second
}

// WorkStateStale reports that the reader is failing within its CAL-V0-276
// window, so the state-event baseline still holds each ticket's last known
// work state, now stale. The observation itself stays UNKNOWN (CAL-V0-053).
func WorkStateStale(c *Config, h *WorkStateHealth, now time.Time) bool {
	return c.WorkState != nil && h != nil && !h.OK && now.Sub(h.Since) < c.WorkState.StaleAfter(c.TickSeconds)
}

// terminalStatus names the ticket statuses whose work state emits no state
// event of its own (CAL-V0-280).
func terminalStatus(status string) bool {
	return status == "COMPLETED" || status == "ARCHIVED"
}

// markWorkStateUnknown records, on each running ticket worker, that a
// read left its ticket's work state UNKNOWN (CAL-V0-272).
func (d *Dispatcher) markWorkStateUnknown(obs *Observation) {
	for _, w := range d.ledger.Workers {
		if !w.WorkStateUnknown && workStateUnknown(obs, w.Key) {
			w.WorkStateUnknown = true
		}
	}
}

// workStateUnknown reports a ticket key whose program work state, not its
// escalation material, is UNKNOWN.
func workStateUnknown(obs *Observation, key string) bool {
	for _, t := range obs.Tickets {
		if t.ID == key {
			return t.State == StateUnknown
		}
	}
	return false
}
