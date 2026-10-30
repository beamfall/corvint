package snapshot

import (
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PoolRecoveryStallFailures is the count of consecutive identical failed
// readiness observations after which a QUARANTINED member's recovery is
// stalled (PSR-V0-262).
const PoolRecoveryStallFailures = 3

// PoolRecoveryStallAge is the quarantine age after which a QUARANTINED
// member's recovery is stalled (PSR-V0-262). It applies only when the entry
// recorded quarantinedAt; a legacy entry without it never ages.
const PoolRecoveryStallAge = 24 * time.Hour

// Stall triggers (PSR-V0-262).
const (
	PoolStallConsecutiveFailures = "CONSECUTIVE_FAILURES"
	PoolStallQuarantineAge       = "QUARANTINE_AGE"
)

// PoolRecovery is the last failed readiness observation `pool observe`
// recorded for a QUARANTINED or CLEANING member (PSR-V0-260, PSR-V0-262).
// DetailSha256 names the bounded, redacted detail document kept under
// evidence/; Consecutive counts the identical (same kind and digest)
// failures in a row, starting at FirstObservedAt.
type PoolRecovery struct {
	Kind, Result                string
	DetailSha256                wire.Digest
	Consecutive                 wire.Count
	FirstObservedAt, ObservedAt wire.Timestamp
	ObservedSeq                 wire.Size
}

func readPoolRecovery(r *wire.Reader) *PoolRecovery {
	r.Closed("kind", "result", "detailSha256", "consecutive", "firstObservedAt", "observedAt", "observedSeq")
	x := &PoolRecovery{Kind: r.Field("kind").Enum("health", "cleanup"), Result: r.Field("result").Enum("FAILED"), DetailSha256: r.Field("detailSha256").Digest(), Consecutive: r.Field("consecutive").Count(), FirstObservedAt: r.Field("firstObservedAt").Timestamp(), ObservedAt: r.Field("observedAt").Timestamp(), ObservedSeq: r.Field("observedSeq").Size()}
	if r.Err() == nil && (x.Consecutive.Int() < 1 || x.ObservedAt < x.FirstObservedAt || x.ObservedSeq.Uint64() == 0) {
		r.Fail(wire.CodeMalformed, "recovery counts at least one failure, observed no earlier than the first")
	}
	return x
}

func (x *PoolRecovery) value() wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("kind", wire.String(x.Kind)).Set("result", wire.String(x.Result)).Set("detailSha256", wire.String(string(x.DetailSha256))).Set("consecutive", wire.String(string(x.Consecutive))).Set("firstObservedAt", wire.String(string(x.FirstObservedAt))).Set("observedAt", wire.String(string(x.ObservedAt))).Set("observedSeq", wire.String(string(x.ObservedSeq))))
}

// PoolRecoveryStall reports whether a QUARANTINED entry's recovery is
// stalled at now and which trigger fired (PSR-V0-262): at least
// PoolRecoveryStallFailures consecutive identical failures, else a recorded
// quarantinedAt at least PoolRecoveryStallAge before now. Any other state,
// and an entry with neither signal, is never stalled; an unparseable now
// never ages an entry.
func PoolRecoveryStall(en PoolEntry, now time.Time) (string, bool) {
	if en.State != "QUARANTINED" {
		return "", false
	}
	if en.Recovery != nil && en.Recovery.Consecutive.Int() >= PoolRecoveryStallFailures {
		return PoolStallConsecutiveFailures, true
	}
	if en.QuarantinedAt == "" || now.IsZero() {
		return "", false
	}
	since, e := time.Parse("2006-01-02T15:04:05Z", string(en.QuarantinedAt))
	if e != nil || now.Sub(since) < PoolRecoveryStallAge {
		return "", false
	}
	return PoolStallQuarantineAge, true
}

// MembersStalled reports whether members is non-empty and every one of them
// has a QUARANTINED entry in poolID whose recovery is stalled at now
// (PSR-V0-263). A FREE member, or any occupied member that is not a stalled
// QUARANTINED one, means the pool is not stalled.
func (p *PoolState) MembersStalled(poolID string, members []string, now time.Time) bool {
	if p == nil || len(members) == 0 {
		return false
	}
	byMember := map[string]PoolEntry{}
	for _, en := range p.Entries {
		if en.PoolID == poolID {
			byMember[en.MemberID] = en
		}
	}
	for _, m := range members {
		en, ok := byMember[m]
		if !ok {
			return false
		}
		if _, stalled := PoolRecoveryStall(en, now); !stalled {
			return false
		}
	}
	return true
}
