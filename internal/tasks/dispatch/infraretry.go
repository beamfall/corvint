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

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Session classes of an ended worker (ESC-V0-008), in precedence order:
// checked progress, a current typed decision, scope or blocked hold, the
// worker's own infrastructure request under the ESC-V0-007 policy, and
// ordinary no-progress. A hold outranks infrastructure, so a session that
// raised both spends no retry and leaves no infrastructure hold behind the
// owner's answer.
const (
	SessionProgress       = "progress"
	SessionInfrastructure = "infrastructure"
	SessionHeld           = "held"
	SessionNoProgress     = "no-progress"
)

// observed returns the observed native ticket of a key, nil for a lane key
// or an unobserved ticket.
func observed(obs *Observation, key string) *Ticket {
	for i := range obs.Tickets {
		if obs.Tickets[i].ID == key {
			return &obs.Tickets[i]
		}
	}
	return nil
}

// classify is the ESC-V0-008 class of an ended worker. Classification
// precedes no-progress parking; without the infrastructure policy an
// infrastructure session is ordinary no-progress.
func (d *Dispatcher) classify(t *Ticket, w *Worker, progress bool) string {
	switch {
	case progress:
		return SessionProgress
	case t == nil:
		return SessionNoProgress
	case len(t.EscalationPending) > 0:
		return SessionHeld
	case d.Config.InfrastructureRetry != nil && !t.EscalationUnknown && slices.Contains(t.Infrastructure, w.ID):
		return SessionInfrastructure
	}
	return SessionNoProgress
}

// infraEnded settles the episode of a key after a session that was not an
// infrastructure session: its own retry launch is no longer running, and
// checked progress marks the episode RECOVERED without touching its debt.
func (d *Dispatcher) infraEnded(key, worker string, progress bool) {
	e := d.ledger.InfraRetry[key]
	if e == nil {
		return
	}
	if e.Launch == worker && (e.State == InfraRunning || e.State == InfraReserved) {
		e.Launch, e.State = "", InfraIdle
	}
	if progress && e.Launch == "" {
		e.State = InfraRecovered
	}
}

// chargeInfra counts one ended infrastructure session against the
// ticket's episode at its acceptance revision and reserves the next retry
// ordinal and cooldown deadline, or holds the ticket when none is left.
// Parked state and the no-progress count are untouched (ESC-V0-008). It
// reports false, leaving ordinary accounting, only when a new episode
// would exceed the ledger bound.
func (d *Dispatcher) chargeInfra(t *Ticket, w *Worker) bool {
	maxRetries, cooldown, maxCooldown := d.Config.InfrastructureRetry.Limits()
	e := d.ledger.InfraRetry[t.ID]
	if e == nil && len(d.ledger.InfraRetry) >= maxInfraEpisodes {
		d.emit(Event{Kind: "alert", Ticket: t.ID, Worker: w.ID, Message: "infrastructure retry episodes are at capacity; the session is accounted as ordinary no-progress"})
		return false
	}
	if e == nil || e.AcceptanceRevision != t.AcceptanceRevision {
		e = &InfraEpisode{AcceptanceRevision: t.AcceptanceRevision, Limit: maxRetries}
		if d.ledger.InfraRetry == nil {
			d.ledger.InfraRetry = map[string]*InfraEpisode{}
		}
		d.ledger.InfraRetry[t.ID] = e
	}
	e.Sessions = min(e.Sessions+1, maxInfraSessions)
	e.Limit = min(e.Limit, maxRetries)
	e.Launch = ""
	detail := map[string]string{"kind": "infrastructure", "acceptanceRevision": e.AcceptanceRevision, "sessions": strconv.Itoa(e.Sessions), "charged": strconv.Itoa(e.Charged), "limit": strconv.Itoa(e.Limit)}
	hold := func(state, code, why string) {
		e.State = state
		detail["code"] = code
		d.emit(Event{Kind: "needs-owner", Ticket: t.ID, Worker: w.ID, Message: fmt.Sprintf("%s is held %s: %s. Its infrastructure request stays OPEN; after fixing the cause run `corvint-tasks dispatch unpark --program %s --config FILE --key %s`", d.keyText(t.ID), code, why, d.Program, t.ID), Detail: detail})
	}
	switch {
	case t.PlanReason == wire.CodeRetryExhausted:
		hold(InfraNativeExhausted, "NATIVE_RETRY_EXHAUSTED", "the native queue has no retry left")
	case e.Limit == 0:
		hold(InfraDisabled, "INFRA_RETRY_DISABLED", "infrastructureRetry.maxRetries is 0")
	case e.Charged >= e.Limit:
		hold(InfraExhausted, "INFRA_RETRY_EXHAUSTED", fmt.Sprintf("%d infrastructure session(s) used all %d retries", e.Sessions, e.Limit))
	default:
		e.Charged++
		e.CooldownUntil = d.Now().Add(RetryCooldown(cooldown, maxCooldown, e.Charged))
		e.State = InfraWaiting
		detail["charged"], detail["code"] = strconv.Itoa(e.Charged), "INFRA_RETRY"
		d.emit(Event{Kind: "cooldown", Ticket: t.ID, Message: fmt.Sprintf("%s retries after an infrastructure session at %s (retry %d of %d)", d.keyText(t.ID), e.CooldownUntil.UTC().Format("2006-01-02T15:04:05Z"), e.Charged, e.Limit), Detail: detail})
	}
	return true
}

// maxInfraSessions bounds the per-episode session count; the count only
// reports, so it saturates instead of refusing.
const maxInfraSessions = 1024

// accountUnknown is ESC-V0-008 unknown progress for the CAL-V0-057 ladder:
// it resets only the proved failure suffix beyond each role's selected tier
// threshold, so every role keeps the tier it would launch at.
func (d *Dispatcher) accountUnknown(key string) {
	e := d.ledger.Escalation[key]
	if e == nil {
		return
	}
	floor := 0
	for _, r := range d.Config.Roles {
		if k := r.TierFor(e.Streak); k > 0 {
			floor = max(floor, r.Escalate[k-1].After)
		}
	}
	e.Streak = min(e.Streak, floor)
}

// infraHolds reports whether the key's current episode keeps it from
// launching: a named hold, a future retry deadline, or a retry launch that
// is reserved or running. An episode of an older acceptance revision no
// longer applies.
func (d *Dispatcher) infraHolds(obs *Observation, key string) bool {
	e := d.ledger.InfraRetry[key]
	if e == nil || d.Config.InfrastructureRetry == nil {
		return false
	}
	t := observed(obs, key)
	if t == nil || t.AcceptanceRevision != e.AcceptanceRevision {
		return false
	}
	return e.Holds(d.Now()) || e.State == InfraReserved || e.State == InfraRunning
}

// infraDue returns the key's episode when its reserved retry is due.
func (d *Dispatcher) infraDue(obs *Observation, key string) *InfraEpisode {
	e := d.ledger.InfraRetry[key]
	if e == nil || d.Config.InfrastructureRetry == nil || e.State != InfraWaiting {
		return nil
	}
	if t := observed(obs, key); t == nil || t.AcceptanceRevision != e.AcceptanceRevision {
		return nil
	}
	return e
}

// pruneInfra drops episodes of tickets the observation no longer has, and
// of an older acceptance revision once nothing of theirs runs.
func (d *Dispatcher) pruneInfra(obs *Observation) {
	for key, e := range d.ledger.InfraRetry {
		t := observed(obs, key)
		if t == nil || (t.AcceptanceRevision != e.AcceptanceRevision && e.Launch == "") {
			delete(d.ledger.InfraRetry, key)
		}
	}
	if len(d.ledger.InfraRetry) == 0 {
		d.ledger.InfraRetry = nil
	}
}

// reconcileInfraRetry settles recorded episodes when a dispatcher opens. A
// reload may only narrow: the limit falls to the configured maxRetries and a
// reserved ordinal beyond it holds. A reservation without its recorded worker
// is consumed unless its log directory proves no spawn, and is then UNKNOWN.
func (d *Dispatcher) reconcileInfraRetry() {
	if d.Config.InfrastructureRetry == nil {
		return // inert while the policy is absent; its debt is kept
	}
	maxRetries, _, _ := d.Config.InfrastructureRetry.Limits()
	keys := make([]string, 0, len(d.ledger.InfraRetry))
	for k := range d.ledger.InfraRetry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		e := d.ledger.InfraRetry[key]
		e.Limit = min(e.Limit, maxRetries)
		switch e.State {
		case InfraReserved:
			switch {
			case d.worker(e.Launch) != nil:
				e.State = InfraRunning
			case noSpawn(d.workerDir(e.Launch)):
				e.State = InfraWaiting
			default:
				e.State, e.Launch = InfraUnknown, ""
				d.emit(Event{Kind: "needs-owner", Ticket: key, Message: fmt.Sprintf("%s is held UNKNOWN: a reserved infrastructure retry may have started before the dispatcher stopped. Inspect it, then run `corvint-tasks dispatch unpark --program %s --config FILE --key %s`", d.keyText(key), d.Program, key), Detail: map[string]string{"kind": "infrastructure", "code": "INFRA_RETRY_UNKNOWN", "acceptanceRevision": e.AcceptanceRevision}})
			}
		case InfraRunning:
			if d.worker(e.Launch) == nil {
				e.State, e.Launch = InfraIdle, ""
			}
		}
		if e.State == InfraWaiting && e.Charged > e.Limit {
			e.State, e.Launch = InfraExhausted, ""
			if e.Limit == 0 {
				e.State = InfraDisabled
			}
			d.emit(Event{Kind: "needs-owner", Ticket: key, Message: fmt.Sprintf("%s is held: the configured infrastructureRetry.maxRetries %d is below its reserved retry %d", d.keyText(key), e.Limit, e.Charged), Detail: map[string]string{"kind": "infrastructure", "code": "INFRA_RETRY_" + map[bool]string{true: "DISABLED", false: "EXHAUSTED"}[e.Limit == 0], "acceptanceRevision": e.AcceptanceRevision}})
		}
	}
}

// noSpawn reports a launch proved never to have spawned: launch creates the
// worker's log directory before it starts the process.
func noSpawn(dir string) bool {
	_, err := os.Lstat(dir)
	return errors.Is(err, fs.ErrNotExist)
}

// releaseInfra is the operator unpark of a held episode: it may launch
// again, and its sessions, charges and limit stay, so a release never refills.
func (d *Dispatcher) releaseInfra(key string) bool {
	e := d.ledger.InfraRetry[key]
	if e == nil {
		return false
	}
	switch e.State {
	case InfraExhausted, InfraDisabled, InfraNativeExhausted, InfraUnknown:
		e.State = InfraIdle
		return true
	}
	return false
}

// reservedSlot returns the slot of a reserved launch identity when it was
// reserved for role in this program. Program and role names cannot contain
// '.', so the identity's role and slot are its second and third fields.
func (d *Dispatcher) reservedSlot(id, role string) (int, bool) {
	rest, ok := strings.CutPrefix(id, d.Program+"."+role+".")
	if !ok {
		return 0, false
	}
	n, _, _ := strings.Cut(rest, ".")
	slot, err := strconv.Atoi(n)
	return slot, err == nil && slot > 0 && strconv.Itoa(slot) == n
}

// slotBusy reports whether a live worker or another launch of this tick
// holds the role's slot.
func slotBusy(workers []*Worker, launches []Assignment, role string, slot int) bool {
	for _, w := range workers {
		if w.Role == role && w.Slot == slot {
			return true
		}
	}
	for _, a := range launches {
		if a.Role == role && a.Slot == slot {
			return true
		}
	}
	return false
}
