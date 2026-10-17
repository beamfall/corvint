package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// LeaseKeeper is the optional Queue capability of the CAL-V0-311..316 lease
// keeper: a renew that also records the attempt heartbeat (CAL-V0-231) and
// a heartbeat that never extends the lease, each fenced on the attempt's
// generation. A write the store refuses as stale wraps ErrFenced. A write
// the store answers as the replay of an earlier request with the same ID
// reports replayed: the store did not act at this call (CAL-V0-318).
type LeaseKeeper interface {
	Renew(ctx context.Context, a Attempt, minutes int, requestID string) (replayed bool, err error)
	Heartbeat(ctx context.Context, a Attempt, requestID string) (replayed bool, err error)
}

// ErrFenced marks a keeper write the store refused FENCED: the attempt is no
// longer live at the bound generation, or its lease has expired.
var ErrFenced = errors.New("attempt fenced")

// Keeper states (CAL-V0-315).
const (
	keeperWaiting  = "WAITING"
	keeperKeeping  = "KEEPING"
	keeperIdle     = "IDLE"
	keeperStopped  = "STOPPED"
	keeperRenewed  = "RENEWED"
	keeperBeat     = "HEARTBEAT"
	keeperFenced   = "FENCED"
	keeperNotHeld  = "NOT_HELD"
	keeperNewGen   = "GENERATION_CHANGED"
	keeperNoKeeper = "UNSUPPORTED"
)

// KeeperState is one worker's CAL-V0-315 lease keeper record: the attempt
// and generation it is bound to, when its next write is due, and what it
// last did. A STOPPED binding is never written again.
type KeeperState struct {
	State      string    `json:"state"`
	Attempt    string    `json:"attempt,omitempty"`
	Generation string    `json:"generation,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Due        time.Time `json:"due"`
	LastKept   time.Time `json:"lastKept"`
	LastAction string    `json:"lastAction,omitempty"`
	Renewals   int       `json:"renewals"`
	Heartbeats int       `json:"heartbeats"`
	Failing    bool      `json:"failing,omitempty"`
}

func (k *KeeperState) validate() error {
	if k == nil {
		return nil
	}
	bound := k.Attempt != "" && k.Generation != ""
	switch {
	case k.Renewals < 0 || k.Heartbeats < 0:
		return fmt.Errorf("lease keeper counts must not be negative")
	case k.LastAction != "" && k.LastAction != keeperRenewed && k.LastAction != keeperBeat:
		return fmt.Errorf("lease keeper last action %q is unknown", k.LastAction)
	case k.State == keeperWaiting && (k.Attempt != "" || k.Generation != "" || k.Reason != ""):
		return fmt.Errorf("a waiting lease keeper is bound to no attempt")
	case (k.State == keeperKeeping || k.State == keeperIdle) && (!bound || k.Reason != ""):
		return fmt.Errorf("a %s lease keeper needs an attempt and generation and no reason", k.State)
	case k.State == keeperStopped && (!bound || (k.Reason != keeperFenced && k.Reason != keeperNotHeld && k.Reason != keeperNewGen && k.Reason != keeperNoKeeper)):
		return fmt.Errorf("a stopped lease keeper needs an attempt, generation and known reason")
	case k.State != keeperWaiting && k.State != keeperKeeping && k.State != keeperIdle && k.State != keeperStopped:
		return fmt.Errorf("lease keeper state %q is unknown", k.State)
	}
	return nil
}

// keepLeases is the CAL-V0-312..314 keeper pass of one readable tick, run
// after heal on its observation. It acts only for a running worker with a
// live leader and a nonempty tree whose launch role declares keepLease, so
// an exited, ended or stopping worker is never kept. The tree and leader
// must come from this tick's process observation (CAL-V0-317): a worker
// whose tree supervise could not observe keeps only its recorded members,
// which may name processes that have exited, and is not kept this tick.
func (d *Dispatcher) keepLeases(ctx context.Context, obs *Observation) {
	if obs == nil {
		return
	}
	now := d.Now()
	for _, w := range d.ledger.Workers {
		if ctx.Err() != nil {
			return
		}
		role := d.launchConfig(w).roleNamed(w.Role)
		if role == nil || role.KeepLease == nil || !d.treeSeen[w.ID] || len(w.Members) == 0 || w.State != "RUNNING" || !leaderAlive(w) {
			continue
		}
		d.keepLease(ctx, w, role.KeepLease, obs, now)
	}
}

// heldAttempt is the live attempt worker id holds, preferring the bound one.
func heldAttempt(obs *Observation, id, bound string) *Attempt {
	var first *Attempt
	for i := range obs.Attempts {
		a := &obs.Attempts[i]
		if !a.Live || a.Holder != id {
			continue
		}
		if a.ID == bound {
			return a
		}
		if first == nil {
			first = a
		}
	}
	return first
}

func (d *Dispatcher) keepLease(ctx context.Context, w *Worker, kl *KeepLease, obs *Observation, now time.Time) {
	if w.Keeper == nil {
		w.Keeper = &KeeperState{State: keeperWaiting}
	}
	k := w.Keeper
	cur := heldAttempt(obs, w.ID, k.Attempt)
	// CAL-V0-314: a binding the worker no longer holds live at its
	// generation stops once, and is never written again.
	if k.Attempt != "" && k.State != keeperStopped {
		switch {
		case cur == nil || cur.ID != k.Attempt:
			d.stopKeeper(w, keeperNotHeld, "")
		case cur.Generation != k.Generation:
			d.stopKeeper(w, keeperNewGen, "")
		}
	}
	if cur == nil || (k.State == keeperStopped && cur.ID == k.Attempt && cur.Generation == k.Generation) {
		return
	}
	interval := time.Duration(kl.IntervalSeconds) * time.Second
	if cur.ID != k.Attempt || cur.Generation != k.Generation {
		// CAL-V0-312: a new binding is due at once.
		w.Keeper = &KeeperState{State: keeperKeeping, Attempt: cur.ID, Generation: cur.Generation, Due: now, Renewals: k.Renewals, Heartbeats: k.Heartbeats}
		k = w.Keeper
		d.keeperEvent(w, "bound", fmt.Sprintf("keeping the lease of attempt %s (generation %s) held by %s every %ds", cur.ID, cur.Generation, w.ID, kl.IntervalSeconds), nil)
	}
	// CAL-V0-312: idle is the supervision activity signal (CAL-V0-056, CAL-V0-218)
	// older than maxIdleSeconds; an idle worker's lease is left to run out.
	idleFor := now.Sub(w.LastActive) - d.slept(w.LastActive, now)
	if idleFor > time.Duration(kl.MaxIdleSeconds)*time.Second {
		if k.State != keeperIdle {
			k.State = keeperIdle
			d.keeperEvent(w, "idle", fmt.Sprintf("not keeping the lease of attempt %s: %s has shown no activity for %ds, over maxIdleSeconds %d", k.Attempt, w.ID, int(idleFor/time.Second), kl.MaxIdleSeconds), map[string]string{"idleSeconds": strconv.Itoa(int(idleFor / time.Second))})
		}
		return
	}
	if k.State == keeperIdle {
		k.State = keeperKeeping
		d.keeperEvent(w, "resumed", fmt.Sprintf("keeping the lease of attempt %s again: %s is active", k.Attempt, w.ID), nil)
	}
	if now.Before(k.Due) {
		return
	}
	lk, ok := d.Queue.(LeaseKeeper)
	if !ok {
		d.stopKeeper(w, keeperNoKeeper, "")
		d.emit(Event{Kind: "alert", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("cannot keep the lease of attempt %s: this queue cannot renew or heartbeat an attempt", k.Attempt), Detail: map[string]string{"attempt": k.Attempt, "generation": k.Generation, "reason": keeperNoKeeper}})
		return
	}
	// CAL-V0-313: a lease that still covers two intervals is not renewed;
	// the heartbeat alone keeps the holder from STALE_HOLDER.
	due := k.Due.UTC().Format(time.RFC3339Nano)
	action, kind := keeperBeat, "heartbeat"
	if cur.LeaseExpires.Sub(now) <= 2*interval {
		action, kind = keeperRenewed, "renew"
	}
	write := func(id string) (bool, error) {
		if action == keeperBeat {
			return lk.Heartbeat(ctx, *cur, id)
		}
		return lk.Renew(ctx, *cur, kl.LeaseMinutes, id)
	}
	replayed, err := write(requestID("keep", cur.ID, cur.Generation, kind, due))
	if err == nil && replayed {
		// CAL-V0-318: the replay answers an earlier write, made before a
		// lost ledger save, and moved neither the lease nor the heartbeat
		// now. Write once more under a request ID of this time, so the next
		// due time follows a write that acted now.
		_, err = write(requestID("keep", cur.ID, cur.Generation, kind, due, "after-replay", now.UTC().Format(time.RFC3339Nano)))
	}
	switch {
	case errors.Is(err, ErrFenced):
		d.stopKeeper(w, keeperFenced, err.Error())
		return
	case err != nil:
		// One alert per failure episode; the write is retried next tick.
		if !k.Failing {
			k.Failing = true
			d.emit(Event{Kind: "alert", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("could not keep the lease of attempt %s held by %s; retrying each tick: %v", k.Attempt, w.ID, err), Detail: map[string]string{"attempt": k.Attempt, "generation": k.Generation, "action": "failed", "error": err.Error()}})
		}
		return
	}
	k.Failing = false
	k.LastKept, k.LastAction, k.Due = now, action, now.Add(interval)
	detail := map[string]string{}
	if action == keeperRenewed {
		k.Renewals++
		detail["leaseMinutes"] = strconv.Itoa(kl.LeaseMinutes)
		d.keeperEvent(w, "renewed", fmt.Sprintf("renewed the lease of attempt %s held by %s for %d minutes", k.Attempt, w.ID, kl.LeaseMinutes), detail)
		return
	}
	k.Heartbeats++
	detail["leaseExpires"] = cur.LeaseExpires.UTC().Format(time.RFC3339)
	d.keeperEvent(w, "heartbeat", fmt.Sprintf("recorded the heartbeat of attempt %s held by %s; its lease covers the next interval, so it was not renewed", k.Attempt, w.ID), detail)
}

// stopKeeper ends w's binding once with reason, recording one event.
func (d *Dispatcher) stopKeeper(w *Worker, reason, cause string) {
	k := w.Keeper
	k.State, k.Reason, k.Failing = keeperStopped, reason, false
	detail := map[string]string{"reason": reason}
	if cause != "" {
		detail["error"] = cause
	}
	d.keeperEvent(w, "stopped", fmt.Sprintf("stopped keeping the lease of attempt %s (generation %s) held by %s: %s", k.Attempt, k.Generation, w.ID, reason), detail)
}

func (d *Dispatcher) keeperEvent(w *Worker, action, message string, extra map[string]string) {
	detail := map[string]string{"action": action, "attempt": w.Keeper.Attempt, "generation": w.Keeper.Generation}
	for k, v := range extra {
		detail[k] = v
	}
	d.emit(Event{Kind: "lease-keeper", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: message, Detail: detail})
}
