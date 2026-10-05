package dispatch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// typedSession classifies an ended worker by the validated typed requests
// it raised itself on the current acceptance revision (ESC-V0-008): held
// lists its decision, scope and blocked requests, infrastructure its
// infrastructure requests. Text is never classified; a worker matches a
// request only by the holder recorded in the request's own source.
func typedSession(obs *Observation, w *Worker) (t *Ticket, held, infrastructure []string) {
	for i := range obs.Tickets {
		if obs.Tickets[i].ID != w.Key {
			continue
		}
		t = &obs.Tickets[i]
		if t.AcceptanceRevision == "" {
			// Requests cannot be bound to an episode without the acceptance
			// revision; ordinary no-progress accounting applies.
			break
		}
		for _, r := range t.Requests {
			if r.Holder != w.ID || (r.State != "OPEN" && r.State != "ANSWERED") {
				continue
			}
			if r.Kind == "infrastructure" {
				infrastructure = append(infrastructure, r.ID)
			} else {
				held = append(held, r.ID)
			}
		}
	}
	return t, held, infrastructure
}

// accountUnknown is the CAL-V0-057 ladder accounting of an infrastructure
// session (ESC-V0-008): unknown progress resets only the proved failure
// streak and keeps every recorded tier as a launch floor.
func (d *Dispatcher) accountUnknown(key string) {
	if !d.Config.Escalates() || strings.HasPrefix(key, "lane:") {
		return
	}
	e := d.ledger.Escalation[key]
	if e == nil {
		return
	}
	e.Streak = 0
	if len(e.Tiers) == 0 {
		delete(d.ledger.Escalation, key)
		return
	}
	e.RetainTiers = true
}

// infraEnded charges one ended infrastructure session to its ticket's
// ESC-V0-007 episode, once per session, and decides whether another
// automatic retry may follow: RETRY_WAIT until the ordinal's cooldown, or
// EXHAUSTED with a named reason. The native request stays OPEN either way.
func (d *Dispatcher) infraEnded(t *Ticket, w *Worker, requests []string, fp string, now time.Time) {
	e := d.ledger.InfraRetry[w.Key]
	if e == nil || e.AcceptanceRevision != t.AcceptanceRevision {
		e = &InfraRetry{AcceptanceRevision: t.AcceptanceRevision, State: InfraReady, Sessions: []string{}}
		if d.ledger.InfraRetry == nil {
			d.ledger.InfraRetry = map[string]*InfraRetry{}
		}
		d.ledger.InfraRetry[w.Key] = e
	}
	if slices.Contains(e.Sessions, w.ID) {
		return
	}
	e.Sessions = append(e.Sessions, w.ID)
	if len(e.Sessions) > maxInfraSessions {
		e.Sessions = e.Sessions[len(e.Sessions)-maxInfraSessions:]
	}
	for _, id := range requests {
		if !slices.Contains(e.Requests, id) {
			e.Requests = append(e.Requests, id)
		}
	}
	sort.Strings(e.Requests)
	if len(e.Requests) > 64 {
		e.Requests = e.Requests[len(e.Requests)-64:]
	}
	resolved := e.Pending != nil && e.Pending.Worker == w.ID
	if resolved {
		e.Pending = nil
	}
	if e.State == InfraRecovered {
		e.Count = 0
	}
	e.Fingerprint = fp
	maxRetries, _, _ := d.Config.InfrastructureRetry.Limits()
	n := e.Count + 1
	switch {
	case e.State == InfraUnknown && !resolved:
		// An unresolved launch stays a hold; this session does not resolve it.
	case t.PlanReason == wire.CodeRetryExhausted:
		e.State, e.Reason, e.NextEligible = InfraExhausted, NativeRetryExhausted, time.Time{}
	case maxRetries == 0:
		e.State, e.Reason, e.NextEligible = InfraExhausted, InfraRetryDisabled, time.Time{}
	case n > maxRetries:
		e.State, e.Reason, e.NextEligible = InfraExhausted, InfraRetryExhausted, time.Time{}
	default:
		e.State, e.Reason = InfraWait, ""
		e.NextEligible = now.Add(d.Config.InfrastructureRetry.RetryCooldown(n))
	}
	d.emit(d.infraEvent(w.Key, e, fmt.Sprintf("%s worker %s raised infrastructure request(s) %s", w.Role, w.ID, strings.Join(requests, ", "))))
}

// infraSettled ends a retry reservation of a session that did not raise an
// infrastructure request: the charged count is kept and ordinary
// no-progress accounting applies to it.
func (d *Dispatcher) infraSettled(w *Worker) {
	e := d.ledger.InfraRetry[w.Key]
	if e == nil || e.Pending == nil || e.Pending.Worker != w.ID {
		return
	}
	e.Pending = nil
	e.State, e.Reason, e.NextEligible = InfraReady, "", time.Time{}
	if !slices.Contains(e.Sessions, w.ID) {
		e.Sessions = append(e.Sessions, w.ID)
		if len(e.Sessions) > maxInfraSessions {
			e.Sessions = e.Sessions[len(e.Sessions)-maxInfraSessions:]
		}
	}
}

// infraRecovered marks an episode RECOVERED on checked work progress: its
// local consecutive debt resets, the native request stays OPEN.
func (d *Dispatcher) infraRecovered(key, why string) {
	e := d.ledger.InfraRetry[key]
	if e == nil || e.State == InfraRecovered {
		return
	}
	e.State, e.Reason, e.Count, e.NextEligible, e.Pending, e.Fingerprint = InfraRecovered, "", 0, time.Time{}, nil, ""
	d.emit(d.infraEvent(key, e, "work progress recovered the infrastructure retry episode ("+why+")"))
}

func (d *Dispatcher) infraEvent(key string, e *InfraRetry, why string) Event {
	maxRetries, _, _ := d.Config.InfrastructureRetry.Limits()
	detail := map[string]string{"state": e.State, "count": strconv.Itoa(e.Count), "maxRetries": strconv.Itoa(maxRetries), "acceptanceRevision": e.AcceptanceRevision, "requests": strings.Join(e.Requests, ",")}
	msg := fmt.Sprintf("%s: infrastructure retry %s", d.keyText(key), e.State)
	switch e.State {
	case InfraWait:
		detail["nextEligible"] = e.NextEligible.UTC().Format(time.RFC3339)
		msg += fmt.Sprintf(", retry %d of %d eligible at %s", e.Count+1, maxRetries, detail["nextEligible"])
	case InfraExhausted, InfraUnknown:
		detail["reason"] = e.Reason
		msg += " (" + e.Reason + "); the native request stays OPEN"
	}
	return Event{Kind: "infra-retry", Ticket: ticketOf(key), Message: msg + ": " + why, Detail: detail}
}

// reconcileInfra drops episodes whose ticket left the observation or
// changed acceptance revision, and recovers one whose known work state
// changed while no worker ran (progress made outside a session).
func (d *Dispatcher) reconcileInfra(obs *Observation) {
	tickets := map[string]*Ticket{}
	for i := range obs.Tickets {
		tickets[obs.Tickets[i].ID] = &obs.Tickets[i]
	}
	for _, key := range sortedInfraKeys(d.ledger.InfraRetry) {
		e := d.ledger.InfraRetry[key]
		t := tickets[key]
		if t == nil || t.AcceptanceRevision != e.AcceptanceRevision {
			if e.Pending != nil && d.worker(e.Pending.Worker) != nil {
				continue // its running retry is accounted when it ends
			}
			delete(d.ledger.InfraRetry, key)
			continue
		}
		if e.Fingerprint != "" && !d.busy(key) && !stateUnknown(obs, key) && Fingerprint(obs, key) != e.Fingerprint {
			d.infraRecovered(key, "its state changed")
		}
	}
}

// infraHeld reports the keys whose episode holds automatic launches now.
func (d *Dispatcher) infraHeld(now time.Time) map[string]bool {
	held := map[string]bool{}
	for k, e := range d.ledger.InfraRetry {
		switch e.State {
		case InfraExhausted, InfraUnknown, InfraRetrying:
			held[k] = true
		case InfraWait:
			held[k] = e.NextEligible.After(now)
		}
	}
	return held
}

// reserveInfra charges and persists a retry launch before it is issued
// (ESC-V0-007). A reservation already recorded is reused without another
// charge. ok is false when the retry may not launch; the caller then
// launches nothing for the key.
func (d *Dispatcher) reserveInfra(key, id string) (ok bool) {
	e := d.ledger.InfraRetry[key]
	if e == nil || e.State != InfraWait {
		return e == nil || (e.State != InfraRetrying && e.State != InfraExhausted && e.State != InfraUnknown)
	}
	prior := *e
	if e.Pending == nil {
		maxRetries, _, _ := d.Config.InfrastructureRetry.Limits()
		n := e.Count + 1
		if n > maxRetries {
			// A narrowed policy holds the retry; the count is kept.
			e.State, e.Reason, e.NextEligible = InfraExhausted, InfraRetryExhausted, time.Time{}
			if maxRetries == 0 {
				e.Reason = InfraRetryDisabled
			}
			d.emit(d.infraEvent(key, e, "the policy allows no further retry"))
			return false
		}
		e.Count = n
		e.Pending = &InfraReservation{Worker: id, Ordinal: n, Deadline: e.NextEligible}
	} else if e.Pending.Worker != id {
		return false
	}
	e.State = InfraRetrying
	if err := d.ledger.save(d.dir); err != nil {
		*e = prior
		d.emit(Event{Kind: "alert", Ticket: ticketOf(key), Message: "infrastructure retry not launched: its reservation could not be saved: " + err.Error()})
		return false
	}
	return true
}

// infraLaunchFailed records a reserved retry whose launch failed: a failure
// proven before spawn keeps the reservation for reuse; a started but
// unidentified tree is UNKNOWN, never replaced automatically.
func (d *Dispatcher) infraLaunchFailed(key, id string, started bool) {
	e := d.ledger.InfraRetry[key]
	if e == nil || e.Pending == nil || e.Pending.Worker != id {
		return
	}
	if started {
		e.State, e.Reason = InfraUnknown, SpawnAmbiguous
		d.emit(d.infraEvent(key, e, "the retry launch started an unidentified process"))
		return
	}
	e.State = InfraWait
}

// restoreInfra resolves reservations a previous dispatcher left without a
// recorded worker: a launch proven never started (no worker directory) is
// reusable; anything else is UNKNOWN.
func (d *Dispatcher) restoreInfra() {
	for _, key := range sortedInfraKeys(d.ledger.InfraRetry) {
		e := d.ledger.InfraRetry[key]
		if e.State != InfraRetrying || d.worker(e.Pending.Worker) != nil {
			continue
		}
		_, err := os.Lstat(d.workerDir(e.Pending.Worker))
		if errors.Is(err, fs.ErrNotExist) {
			e.State = InfraWait
			continue
		}
		e.State, e.Reason = InfraUnknown, ReservationUnresolved
		d.emit(d.infraEvent(key, e, "a reserved retry launch "+e.Pending.Worker+" has no recorded worker"))
	}
}

func sortedInfraKeys(m map[string]*InfraRetry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
