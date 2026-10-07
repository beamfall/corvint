package dispatch

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StoreWitness is an optional Queue capability (CAL-V0-139): a cheap value
// that changes whenever anything Observe reads may have changed. An error,
// or a Queue without it, means every tick reads the store.
type StoreWitness interface {
	Witness() (string, error)
}

// idleFullEvery bounds how long ticks may skip the store read: a full tick
// still runs at least this often as a safety net (CAL-V0-139). Tests shorten
// it.
var idleFullEvery = 60 * time.Second

// idleGate is armed by a full tick that changed nothing. It holds what that
// tick read and the earliest time a clock-driven decision can change.
type idleGate struct {
	witness string
	ledger  *Ledger
	config  *Config
	since   time.Time
	due     time.Time
}

// idleMark is what a full tick saw before it ran.
type idleMark struct {
	witness string
	ok      bool
	state   [32]byte
	since   time.Time
}

// idleEligible reports whether this dispatcher's tick depends only on the
// store, its own ledger and the clock. Workers, recoveries, uncertain
// launches, a pool sweep in flight or pending, a work-state reader and
// pressure sampling all read or change something else, so they always tick
// in full.
func (d *Dispatcher) idleEligible() bool {
	if len(d.ledger.Workers) > 0 || d.sweepJob != nil || len(d.recoveries) > 0 || len(d.uncertain) > 0 {
		return false
	}
	if d.Config.WorkState != nil || d.Config.Pressure != nil {
		return false
	}
	if d.Config.PoolSweep != nil {
		if d.sweepNext.IsZero() {
			return false
		}
		for _, r := range d.ledger.PoolSweeps {
			if sweepPending(r.Phase) {
				return false
			}
		}
	}
	return true
}

// idleSkip reports that this tick can be skipped: the last full tick was a
// fixed point, nothing it read has changed and no deadline is due. Any
// doubt clears the gate and ticks in full.
func (d *Dispatcher) idleSkip(ctx context.Context) bool {
	g := d.idle
	d.idle = nil
	if g == nil || d.readerErr != nil || ctx.Err() != nil || !d.tickSaved {
		return false
	}
	if g.ledger != d.ledger || g.config != d.Config || !d.idleEligible() {
		return false
	}
	if now := d.Now(); now.Before(g.since) || !now.Before(g.due) {
		return false
	}
	w, ok := d.Queue.(StoreWitness)
	if !ok {
		return false
	}
	if v, err := w.Witness(); err != nil || v != g.witness {
		return false
	}
	if d.unparkRequested() {
		return false
	}
	d.idle = g
	return true
}

// unparkRequested reports a pending operator request file, or any doubt.
func (d *Dispatcher) unparkRequested() bool {
	entries, err := os.ReadDir(filepath.Join(d.dir, "requests"))
	if err != nil {
		return !os.IsNotExist(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			return true
		}
	}
	return false
}

// idleBegin records the witness and the saved ledger before a full tick.
// The witness is taken before the store is read, so a change during the
// tick is seen by the next one.
func (d *Dispatcher) idleBegin() idleMark {
	m := idleMark{since: d.Now()}
	d.idleLease = time.Time{}
	if !d.tickSaved || !d.idleEligible() {
		return m
	}
	w, ok := d.Queue.(StoreWitness)
	if !ok {
		return m
	}
	v, err := w.Witness()
	if err != nil {
		return m
	}
	raw, err := readBounded(filepath.Join(d.dir, "state.json"), maxLedger)
	if err != nil {
		return m
	}
	m.witness, m.ok, m.state = v, true, sha256.Sum256(raw)
	return m
}

// idleSettle arms the gate after a full tick that returned no error, saved
// its ledger and left the saved ledger byte-identical, so it emitted no
// event, launched nothing and changed no record.
func (d *Dispatcher) idleSettle(m idleMark, err error) {
	if !m.ok || err != nil || !d.tickSaved || d.readerErr != nil || !d.idleEligible() {
		return
	}
	raw, e := readBounded(filepath.Join(d.dir, "state.json"), maxLedger)
	if e != nil || sha256.Sum256(raw) != m.state {
		return
	}
	now := d.Now()
	due := m.since.Add(idleFullEvery)
	early := func(t time.Time) {
		if t.After(now) && t.Before(due) {
			due = t
		}
	}
	for _, b := range d.ledger.Backoff {
		if !b.Parked {
			early(b.CooldownUntil)
		}
	}
	for _, e := range d.ledger.InfraRetry {
		if e.State == InfraWaiting {
			early(e.CooldownUntil)
		}
	}
	early(d.idleLease)
	if d.Config.PoolSweep != nil {
		early(d.sweepNext)
	}
	if !now.Before(due) {
		return
	}
	d.idle = &idleGate{witness: m.witness, ledger: d.ledger, config: d.Config, since: m.since, due: due}
}

// noteLeases keeps the earliest future lease expiry this tick observed: an
// expiry can make heal reap without any store change.
func (d *Dispatcher) noteLeases(obs *Observation) {
	now := d.Now()
	for _, a := range obs.Attempts {
		if a.Live && a.LeaseExpires.After(now) && (d.idleLease.IsZero() || a.LeaseExpires.Before(d.idleLease)) {
			d.idleLease = a.LeaseExpires
		}
	}
}
