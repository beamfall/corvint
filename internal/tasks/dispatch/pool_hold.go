package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PoolHoldProfile is the dispatcher-private record of the pool member the
// dispatcher holds for one worker's attempt (ATR-V0-120..124).
const PoolHoldProfile = "taskman-dispatch-pool-hold/0"

// poolHoldEnv names the hold record every worker of a holdPool role
// receives at launch (ATR-V0-121).
const poolHoldEnv = "CORVINT_DISPATCH_POOL_FILE"

// Pool hold states: HELD while the attempt holds the claimed member,
// RELEASED after the dispatcher returned it with `pool release`, and
// ATTEMPT_ENDED when the attempt ended first, which quarantined it natively
// (CAL-V0-202).
const (
	PoolHoldHeld     = "HELD"
	PoolHoldReleased = "RELEASED"
	PoolHoldEnded    = "ATTEMPT_ENDED"
)

// PoolHoldQueue is the explicit opt-in native boundary of ATR-V0-120: a
// Queue without it never claims or returns a member for a session.
type PoolHoldQueue interface {
	// PoolClaim runs the native `claim --pool` of the assignment's ticket
	// for holder, with its admission and waitlist order (PSR-V0-132), and
	// returns the attempt, generation, member and allocation it took.
	// leaseMinutes 0 takes the native default lease.
	PoolClaim(ctx context.Context, a Assignment, holder string, leaseMinutes int, requestID string) (PoolHold, error)
	// PoolRelease runs the native `pool release` (CAL-V0-201) of the
	// attempt's exact allocation.
	PoolRelease(ctx context.Context, a Attempt, allocation, requestID string) error
}

// PoolHold is one taskman-dispatch-pool-hold/0 record.
type PoolHold struct {
	Profile    string    `json:"profile"`
	Worker     string    `json:"worker"`
	State      string    `json:"state"`
	Attempt    string    `json:"attempt"`
	Generation string    `json:"generation"`
	Ticket     string    `json:"ticket"`
	Pool       string    `json:"pool"`
	Member     string    `json:"member"`
	Allocation string    `json:"allocation"`
	Acquired   time.Time `json:"acquired"`
	Ended      time.Time `json:"ended,omitzero"`
}

// PoolHoldPath is the worker's hold record, handed to the session as
// CORVINT_DISPATCH_POOL_FILE.
func PoolHoldPath(dir, worker string) string {
	return filepath.Join(dir, "workers", worker, "pool-hold.json")
}

// ReadPoolHold returns the worker's hold record, or nil when it has none or
// the record is unreadable.
func ReadPoolHold(dir, worker string) *PoolHold {
	h, err := ReadPoolHoldFile(PoolHoldPath(dir, worker))
	if err != nil || h.Worker != worker {
		return nil
	}
	return h
}

// ReadPoolHoldFile decodes one hold record, refusing another profile
// version and unknown fields.
func ReadPoolHoldFile(path string) (*PoolHold, error) {
	raw, err := readRegular(path, 4096)
	if err != nil {
		return nil, err
	}
	if err := wire.RawProfileVersion("/profile", raw, PoolHoldProfile); err != nil {
		return nil, err
	}
	var h PoolHold
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&h); err != nil {
		return nil, err
	}
	if h.Worker == "" || h.Attempt == "" || h.Allocation == "" {
		return nil, errors.New("pool hold record is malformed")
	}
	return &h, nil
}

func writePoolHold(dir string, h *PoolHold) error {
	raw, err := json.Marshal(h)
	if err != nil {
		return err
	}
	path := PoolHoldPath(dir, h.Worker)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeAtomic(path, append(raw, '\n'))
}

// attempt is the live attempt the hold names.
func (h *PoolHold) attempt() Attempt {
	return Attempt{ID: h.Attempt, Ticket: h.Ticket, Generation: h.Generation, Holder: h.Worker, Phase: "RUNNING", Live: true, Pool: h.Pool, Member: h.Member}
}

// claimPoolHold claims, before worker id is spawned, the ticket of
// assignment a with a member of the pool it requires, for a role that
// declares holdPool (ATR-V0-120), and records the hold the session reads
// (ATR-V0-121). ok is false when nothing may be spawned: the native claim
// refused, in its own admission order (PSR-V0-132), or the hold could not be
// recorded, in which case the claim is returned at once.
func (d *Dispatcher) claimPoolHold(ctx context.Context, obs *Observation, role *Role, a Assignment, id string) (*PoolHold, bool) {
	q, isHold := d.Queue.(PoolHoldQueue)
	if !isHold || !role.HoldPool || a.Pool == "" || a.Ticket == "" || strings.HasPrefix(a.Key, "lane:") {
		return nil, true
	}
	minutes := 0
	if role.KeepLease != nil {
		minutes = role.KeepLease.LeaseMinutes
	}
	h, err := q.PoolClaim(ctx, a, id, minutes, requestID("pool-claim", id, a.Ticket))
	detail := map[string]string{"action": "claim", "pool": a.Pool}
	if err != nil {
		if d.poolHoldRefused == nil {
			d.poolHoldRefused = map[string]string{}
		}
		if d.poolHoldRefused[a.Ticket] != err.Error() {
			d.poolHoldRefused[a.Ticket] = err.Error()
			detail["error"] = err.Error()
			d.emit(Event{Kind: "pool-hold", Ticket: a.Ticket, Role: a.Role, Message: fmt.Sprintf("%s not launched: the claim of %s with a member of pool %s was refused: %v; it is retried on a later tick", a.Role, d.local(obs, a.Ticket), a.Pool, err), Detail: detail})
		}
		return nil, false
	}
	delete(d.poolHoldRefused, a.Ticket)
	h.Profile, h.Worker, h.State, h.Ticket, h.Pool, h.Acquired = PoolHoldProfile, id, PoolHoldHeld, a.Ticket, a.Pool, d.Now().UTC()
	detail["attempt"], detail["generation"], detail["member"], detail["allocation"] = h.Attempt, h.Generation, h.Member, h.Allocation
	if err := writePoolHold(d.dir, &h); err != nil {
		detail["error"] = err.Error()
		d.emit(Event{Kind: "pool-hold", Ticket: a.Ticket, Role: a.Role, Worker: id, Message: fmt.Sprintf("%s not launched: the hold of %s/%s could not be recorded: %v; returning it", a.Role, h.Pool, h.Member, err), Detail: detail})
		d.returnPoolHold(ctx, &h, a.Role, true)
		return nil, false
	}
	d.emit(Event{Kind: "pool-hold", Ticket: a.Ticket, Role: a.Role, Worker: id, Message: fmt.Sprintf("claimed %s with %s/%s (attempt %s) for %s before launch", d.local(obs, a.Ticket), h.Pool, h.Member, h.Attempt, id), Detail: detail})
	return &h, true
}

// returnPoolHold returns the held allocation with `pool release` and, when
// end is set, also hands the attempt off, for a claim whose session never
// started or whose worker is gone (ATR-V0-122, ATR-V0-124). It reports
// whether the allocation left the attempt.
func (d *Dispatcher) returnPoolHold(ctx context.Context, h *PoolHold, role string, end bool) bool {
	q, ok := d.Queue.(PoolHoldQueue)
	if !ok {
		return false
	}
	a := h.attempt()
	detail := map[string]string{"action": "release", "attempt": h.Attempt, "generation": h.Generation, "pool": h.Pool, "member": h.Member, "allocation": h.Allocation}
	h.Ended = d.Now().UTC()
	if err := q.PoolRelease(ctx, a, h.Allocation, requestID("pool-release", h.Worker, h.Attempt, h.Generation)); err != nil {
		detail["error"] = err.Error()
		d.emit(Event{Kind: "pool-hold", Ticket: h.Ticket, Role: role, Worker: h.Worker, Message: fmt.Sprintf("could not return %s/%s from attempt %s: %v; ending the attempt quarantines it", h.Pool, h.Member, h.Attempt, err), Detail: detail})
		if end {
			_ = d.Queue.Release(ctx, a, "dispatch:"+h.Worker, requestID("release", h.Worker, h.Attempt, h.Generation))
		}
		return false
	}
	h.State = PoolHoldReleased
	detail["state"] = h.State
	_ = writePoolHold(d.dir, h)
	if end {
		a.Pool, a.Member = "", ""
		if err := d.Queue.Release(ctx, a, "dispatch:"+h.Worker, requestID("release", h.Worker, h.Attempt, h.Generation)); err != nil {
			detail["handoffError"] = err.Error()
		}
	}
	d.emit(Event{Kind: "pool-hold", Ticket: h.Ticket, Role: role, Worker: h.Worker, Message: fmt.Sprintf("returned %s/%s from attempt %s held for %s", h.Pool, h.Member, h.Attempt, h.Worker), Detail: detail})
	return true
}

// releasePoolHolds returns, before any hand-off, the member the dispatcher
// holds for each ended worker's attempt (ATR-V0-122), and the member of a
// live attempt whose holder is no recorded worker but whose hold this
// dispatcher wrote, such as a claim whose spawn or ledger save a crash
// interrupted, ending that attempt too (ATR-V0-124). An attempt that already
// ended quarantined its member natively (CAL-V0-202): the hold records that
// and nothing is written.
func (d *Dispatcher) releasePoolHolds(ctx context.Context, obs *Observation, ended []*Worker) (wrote bool) {
	if _, ok := d.Queue.(PoolHoldQueue); !ok || obs == nil {
		return false
	}
	live := func(h *PoolHold) bool {
		for _, a := range obs.Attempts {
			if a.ID == h.Attempt && a.Generation == h.Generation && a.Live && a.Pool == h.Pool && a.Member == h.Member {
				return true
			}
		}
		return false
	}
	for _, w := range ended {
		if ctx.Err() != nil {
			return wrote
		}
		h := ReadPoolHold(d.dir, w.ID)
		if h == nil || h.State != PoolHoldHeld {
			continue
		}
		if !live(h) {
			h.State, h.Ended = PoolHoldEnded, d.Now().UTC()
			_ = writePoolHold(d.dir, h)
			d.emit(Event{Kind: "pool-hold", Ticket: h.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("%s/%s left attempt %s when the attempt ended", h.Pool, h.Member, h.Attempt), Detail: map[string]string{"action": "release", "attempt": h.Attempt, "generation": h.Generation, "pool": h.Pool, "member": h.Member, "allocation": h.Allocation, "state": h.State}})
			continue
		}
		wrote = true
		d.returnPoolHold(ctx, h, w.Role, false)
	}
	for _, a := range obs.Attempts {
		if ctx.Err() != nil {
			return wrote
		}
		if !a.Live || a.Pool == "" || a.Holder == "" || d.worker(a.Holder) != nil || filepath.Base(a.Holder) != a.Holder {
			continue
		}
		if h := ReadPoolHold(d.dir, a.Holder); h != nil && h.State == PoolHoldHeld && h.Attempt == a.ID && live(h) {
			wrote = true
			d.returnPoolHold(ctx, h, "", true)
		}
	}
	return wrote
}
