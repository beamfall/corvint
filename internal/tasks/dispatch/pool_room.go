package dispatch

import (
	"fmt"
	"strconv"
)

// poolRoom is the PSR-V0-140 launch room of each observed pool: its free
// members less the pending workers, which are running sessions for an OPEN
// ticket of the pool whose ticket holds no member of it yet. A ticket whose
// claim needs a member of a pool with no room is held, not launched, so no
// session starts only to find every member taken. A pool the observation
// does not report has no gate.
type poolRoom struct {
	free, pending, room map[string]int
	stalled             map[string]bool
	held                []Assignment
	heldKeys            map[string]bool
}

// newPoolRoom derives the room from obs.PoolFree and the worker ledger, or
// returns nil (no gate) when the observation reports no pool. A KILLING
// worker, or one whose ticket is not observed OPEN, is a stale record and
// counts for nothing (PSR-V0-142).
func newPoolRoom(obs *Observation, workers []*Worker) *poolRoom {
	if obs == nil || obs.PoolFree == nil {
		return nil
	}
	p := &poolRoom{free: map[string]int{}, pending: map[string]int{}, room: map[string]int{}, heldKeys: map[string]bool{}, stalled: obs.PoolStalled}
	for pool, n := range obs.PoolFree {
		p.free[pool] = n
	}
	open := map[string]bool{}
	for _, t := range obs.Tickets {
		if t.Status == "OPEN" {
			open[t.ID] = true
		}
	}
	holds := map[string]bool{}
	for _, a := range obs.Attempts {
		if a.Live && a.Pool != "" {
			holds[a.Pool+"\x00"+a.Ticket] = true
		}
	}
	for _, w := range workers {
		if w == nil || w.State != "RUNNING" || w.Pool == "" || w.Ticket == "" || !open[w.Ticket] || holds[w.Pool+"\x00"+w.Ticket] {
			continue
		}
		if _, ok := p.free[w.Pool]; ok {
			p.pending[w.Pool]++
		}
	}
	for pool, n := range p.free {
		p.room[pool] = n - p.pending[pool]
	}
	return p
}

// gated reports whether a's launch spends pool room: a ticket assignment
// that names an observed pool. Lane roles and poolless tickets never do.
func (p *poolRoom) gated(a Assignment) bool {
	if p == nil || a.Pool == "" || a.Ticket == "" {
		return false
	}
	_, ok := p.free[a.Pool]
	return ok
}

// admits holds a gated candidate while its pool has no room (PSR-V0-140)
// or every member's recovery is stalled (PSR-V0-265).
func (p *poolRoom) admits(a Assignment) bool {
	if !p.gated(a) || (p.room[a.Pool] > 0 && !p.stalled[a.Pool]) {
		return true
	}
	p.hold(a)
	return false
}

// hold records a gated assignment as held once, so its pool-wait is
// reported (PSR-V0-143, PSR-V0-267).
func (p *poolRoom) hold(a Assignment) {
	if p.gated(a) && !p.heldKeys[a.Key] {
		p.heldKeys[a.Key] = true
		p.held = append(p.held, a)
	}
}

// charge spends one unit of room for a launched gated candidate (PSR-V0-141).
func (p *poolRoom) charge(a Assignment) {
	if p.gated(a) {
		p.room[a.Pool]--
	}
}

// signature keys one hold's reported numbers, so an unchanged hold
// is not re-reported every tick.
func (p *poolRoom) signature(a Assignment) string {
	return fmt.Sprintf("%s/%d/%d/%d/%t", a.Pool, p.free[a.Pool], p.pending[a.Pool], p.room[a.Pool], p.stalled[a.Pool])
}

// recordPoolWait emits one `pool-wait` event per held ticket whose pool,
// free count or pending count changed since it was last reported, so
// status shows each waiting ticket's reason and the computed room
// (PSR-V0-143). A ticket that is no longer held is forgotten.
func (d *Dispatcher) recordPoolWait(p *poolRoom) {
	if p == nil {
		d.poolWaitEmitted = nil
		return
	}
	next := map[string]string{}
	for _, a := range p.held {
		sig := p.signature(a)
		next[a.Key] = sig
		if d.poolWaitEmitted[a.Key] == sig {
			continue
		}
		room := p.room[a.Pool]
		msg, reason := fmt.Sprintf("%s waits: pool %s has %d free member(s), %d launched session(s) not yet holding one and room %d after this tick's launches", d.keyText(a.Key), a.Pool, p.free[a.Pool], p.pending[a.Pool], room), "POOL_ROOM"
		if p.stalled[a.Pool] {
			// PSR-V0-265: no launch starts until an operator repairs a member.
			msg, reason = fmt.Sprintf("%s waits: every member of pool %s is QUARANTINED with a stalled recovery (POOL_MEMBER_RECOVERY_STALLED); read `corvint-tasks pool status --pool %s`, repair a member, then confirm it safe", d.keyText(a.Key), a.Pool, a.Pool), "POOL_MEMBER_RECOVERY_STALLED"
		}
		d.emit(Event{Kind: "pool-wait", Ticket: a.Ticket, Role: a.Role, Message: msg, Detail: map[string]string{"pool": a.Pool, "free": strconv.Itoa(p.free[a.Pool]), "pending": strconv.Itoa(p.pending[a.Pool]), "room": strconv.Itoa(room), "reason": reason}})
	}
	d.poolWaitEmitted = next
}
