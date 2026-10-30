package transaction

import (
	"fmt"
	"slices"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Operator hold verbs (PSR-V0-268..275): an operator outside any dispatched
// session holds one FREE pool member for a bounded window, renews it within
// the maximum, and releases it into quarantine.
const (
	LeasePoolHold        = "POOL_HOLD"
	LeasePoolHoldRenew   = "POOL_HOLD_RENEW"
	LeasePoolHoldRelease = "POOL_HOLD_RELEASE"
	// MaxPoolHoldMinutes bounds one hold from heldAt to expiresAt, renewals
	// included (PSR-V0-270).
	MaxPoolHoldMinutes = 1440
	// PoolHoldReleasedReason is the quarantine reason a released hold leaves.
	PoolHoldReleasedReason = "operator hold released; physical safe reuse unproved"
)

func isPoolHoldVerb(verb string) bool {
	return verb == LeasePoolHold || verb == LeasePoolHoldRenew || verb == LeasePoolHoldRelease
}

// PoolHoldAllocation is the allocation id a fresh `pool hold` records: it
// is derived from the queue, the request and the member, so a replay names
// the same allocation.
func PoolHoldAllocation(queueID, requestID, member string) wire.Digest {
	return wire.Sum([]byte(queueID + ":" + requestID + ":" + member))
}

func (c leaseContext) operatorOnly(what string) *leaseOutcome {
	if c.r.Actor.Role != "OWNER" && c.r.Actor.Role != "OPERATOR" {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, what+" requires operator attestation")
		return &out
	}
	return nil
}

// planPoolHold records a HELD entry for one FREE member (PSR-V0-268,
// PSR-V0-269): every allocation path skips an occupied member, so no
// dispatched claim or acquire is handed it while the hold stands.
func planPoolHold(c leaseContext) leaseOutcome {
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "pool hold blocked by barrier")
	}
	if refused := c.operatorOnly("pool hold"); refused != nil {
		return *refused
	}
	p := c.st.policy.Pool(c.l.Pool)
	if p == nil || !slices.Contains(p.Members, c.l.Member) {
		return c.fail(malformed("unknown pool member"))
	}
	for _, en := range c.st.pools.Entries {
		if en.MemberID == c.l.Member {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "member "+c.l.Member+" is "+en.State+"; only a FREE member is held")
		}
	}
	held := c.in.RecordedAt
	en := &snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: p.ID, MemberID: c.l.Member, AllocationID: PoolHoldAllocation(c.r.QueueID, c.r.RequestID, c.l.Member), DefinitionSha256: c.st.policy.MemberDefinition(p.ID, c.l.Member), AllocatedSeq: c.seq, ConfigRef: p.MemberConfig[c.l.Member].ConfigRef}, State: "HELD", Holder: c.l.Holder, Generation: "0", PolicySha256: c.st.policy.PolicySha256(), RequestSha256: wire.Sum([]byte(c.r.QueueID + ":" + c.r.RequestID)), Reason: c.l.Reason, RunnerPID: "0", Hold: &snapshot.PoolHold{HeldAt: held, ExpiresAt: addMinutes(held, c.l.LeaseMinutes)}}
	return c.holdOutcome(c.putPool(en, false, nil), en)
}

// heldEntry is the HELD entry the request's member and allocation name.
func (c leaseContext) heldEntry() (*snapshot.PoolEntry, *leaseOutcome) {
	en := c.poolEntry()
	if en == nil {
		out := c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation differs")
		return nil, &out
	}
	if en.State != "HELD" {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "member "+en.MemberID+" is "+en.State+", not an operator hold")
		return nil, &out
	}
	return en, nil
}

// planPoolHoldRenew moves an unexpired hold's expiry to now plus the
// requested minutes, never past heldAt plus MaxPoolHoldMinutes (PSR-V0-270).
func planPoolHoldRenew(c leaseContext) leaseOutcome {
	if refused := c.operatorOnly("pool hold renew"); refused != nil {
		return *refused
	}
	en, refused := c.heldEntry()
	if refused != nil {
		return *refused
	}
	if en.Hold.Expired(c.in.RecordedAt) {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "hold expired at "+string(en.Hold.ExpiresAt)+"; release it with pool hold release, then clean the member")
	}
	next := addMinutes(c.in.RecordedAt, c.l.LeaseMinutes)
	if limit := addMinutes(en.Hold.HeldAt, wire.SizeOf(MaxPoolHoldMinutes)); string(next) > string(limit) {
		return c.fail(wire.Errorf(wire.CodeLimitExceeded, "leaseMinutes", "a hold lasts at most %d minutes from heldAt %s; the latest expiry is %s", MaxPoolHoldMinutes, en.Hold.HeldAt, limit))
	}
	en.Hold = &snapshot.PoolHold{HeldAt: en.Hold.HeldAt, ExpiresAt: next}
	return c.holdOutcome(c.putPool(en, false, nil), en)
}

// planPoolHoldRelease ends a hold, expired or not, by quarantining the
// member (PSR-V0-271): the operator's job may have changed it, so the
// existing cleanup and confirm-safe path proves safe reuse before any
// allocation sees it FREE again.
func planPoolHoldRelease(c leaseContext) leaseOutcome {
	if refused := c.operatorOnly("pool hold release"); refused != nil {
		return *refused
	}
	en, refused := c.heldEntry()
	if refused != nil {
		return *refused
	}
	en.State = "QUARANTINED"
	en.Hold = nil
	en.Reason = PoolHoldReleasedReason
	return c.putPool(en, false, nil)
}

func (c leaseContext) holdOutcome(out leaseOutcome, en *snapshot.PoolEntry) leaseOutcome {
	if out.result == nil {
		out.detail = fmt.Sprintf("HELD: %s: allocation %s until %s", en.MemberID, en.AllocationID, en.Hold.ExpiresAt)
	}
	return out
}

// PoolHoldExpired reports whether en is a HELD entry whose window has
// lapsed at now (PSR-V0-272).
func PoolHoldExpired(en snapshot.PoolEntry, now time.Time) bool {
	return en.State == "HELD" && en.Hold.Expired(wire.Timestamp(now.UTC().Format(timestampLayout)))
}
