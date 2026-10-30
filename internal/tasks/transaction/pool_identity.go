package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Source reason tokens (PSR-V0-081, PSR-V0-084). They prefix a refusal detail
// or a quarantine reason; the result code stays a closed wire code.
const (
	ReasonSourceNotAllowed   = "SOURCE_NOT_ALLOWED"
	ReasonSourceDrift        = "SOURCE_DRIFT"
	ReasonIdentityUnproved   = "SOURCE_IDENTITY_UNPROVED"
	ReasonIdentityAtTeardown = "lane-untouched release; source identity unproved"
)

// identityGuarded reports whether an occupied member's FREE transition needs
// a matching identity observation: its definition declares an identity
// command (PSR-V0-082). The definition is immutable while occupied.
func identityGuarded(policy *intent.Policy, en *snapshot.PoolEntry) bool {
	p := policy.Pool(en.PoolID)
	return p != nil && p.MemberConfig[en.MemberID].Identity != nil
}

// acquireSource checks a POOL_ACQUIRE --source against the pool's current
// allow-list (PSR-V0-081). It refuses before any allocation is planned.
func (c leaseContext) acquireSource(pool *intent.Pool) (*intent.PoolSource, *leaseOutcome) {
	if c.l.Source == "" {
		return nil, nil
	}
	s, e := intent.ParsePoolSource("source", c.l.Source)
	if e != nil {
		out := c.fail(e)
		return nil, &out
	}
	if len(pool.AllowedSources) == 0 {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeOutOfScope, ReasonSourceNotAllowed+": pool "+pool.ID+" declares no allowedSources")
		return nil, &out
	}
	if !pool.Allows(s) {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeOutOfScope, ReasonSourceNotAllowed+": source "+s.String()+" is not in pool "+pool.ID+" allowedSources")
		return nil, &out
	}
	return &s, nil
}

// identityGuard admits the FREE transition of a guarded member only with a
// bound identity observation that reports the pool's current defaultSource
// (PSR-V0-083, PSR-V0-084). Otherwise the member stays QUARANTINED: an
// absent observation writes nothing; a failed, malformed or drifted one is
// recorded with its reason and the result is BLOCKED. state holds every other
// entry; on admission the returned identity joins its records.
func (c leaseContext) identityGuard(found *snapshot.PoolEntry, state snapshot.PoolState) (*snapshot.PoolIdentity, *leaseOutcome) {
	if !identityGuarded(c.st.policy, found) {
		return nil, nil
	}
	refuse := func(out leaseOutcome) (*snapshot.PoolIdentity, *leaseOutcome) { return nil, &out }
	raw := c.in.LeaseFacts.Pool.Identity
	if len(raw) == 0 {
		return refuse(c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, ReasonIdentityUnproved+": no identity observation"))
	}
	id, e := snapshot.DecodePoolIdentity(raw)
	if e != nil {
		return refuse(c.fail(e))
	}
	if id.PoolID != found.PoolID || id.MemberID != found.MemberID || id.AllocationID != found.AllocationID || id.DefinitionSha256 != found.DefinitionSha256 || id.ObservedSeq != "0" {
		return refuse(c.fail(malformed("pool identity binding differs")))
	}
	id.ObservedSeq = c.seq
	def := *c.st.policy.Pool(found.PoolID).DefaultSource
	state.Identities = withIdentity(c.st.policy, state.Identities, *id)
	if id.Class == snapshot.PoolIdentityReported && *id.Source == def {
		return id, nil
	}
	reason := ReasonIdentityUnproved + ": identity command " + id.Class + "; default " + def.String()
	if id.Class == snapshot.PoolIdentityReported {
		reason = ReasonSourceDrift + ": observed " + id.Source.String() + "; default " + def.String()
	}
	en := *found
	en.Reason = reason
	en.ChangedSeq = c.seq
	state.Entries = append(append([]snapshot.PoolEntry{}, state.Entries...), en)
	raw, e = state.Encode()
	if e != nil {
		return refuse(c.fail(e))
	}
	return refuse(leaseOutcome{posts: map[string][]byte{"pools.json": raw}, effect: &leaseEffect{kind: "TRANSITION", attemptID: found.AttemptID, generation: found.Generation, outcome: mutation.OutcomeBlocked, codes: []string{wire.CodeQuiescenceUnproved}}, detail: "QUARANTINED: " + found.MemberID + ": " + reason})
}

// withIdentity replaces the member's last identity record and drops the
// records of members the current policy no longer declares in that pool, so
// the records stay within the policy's member bound (PSR-V0-083).
func withIdentity(policy *intent.Policy, list []snapshot.PoolIdentity, id snapshot.PoolIdentity) []snapshot.PoolIdentity {
	out := []snapshot.PoolIdentity{}
	for _, x := range list {
		if x.MemberID != id.MemberID && poolDeclares(policy, x.PoolID, x.MemberID) {
			out = append(out, x)
		}
	}
	return append(out, id)
}

func poolDeclares(policy *intent.Policy, pool, member string) bool {
	if p := policy.Pool(pool); p != nil {
		for _, m := range p.Members {
			if m == member {
				return true
			}
		}
	}
	return false
}
