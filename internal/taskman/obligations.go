package taskman

import (
	"errors"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// obligations validates the optional TOL-V0-001 obligation-ledger reference
// with the bounds the Tasks codec enforces: a [A-Z][A-Z0-9]* prefix, a 1..1024
// revision, a head digest, consistent counts within the 256-entry bound, a
// high-water mark and an optional lastRaise that do not postdate the record's
// acceptance revision, and a current-acceptance high water that is not below
// the witnessed count. The ledger events stay with the Tasks readers, which
// hold the evidence blobs; this reader admits the reference and nothing in
// Core ranks, cites or trusts it.
func obligations(v wire.Value, _, acceptance uint64) error {
	if e := object(v, "counts head highWater lastRaise prefix revision"); e != nil {
		return errors.New("obligations reference")
	}
	if _, e := taskswire.ParseObligationPrefix("/obligations/prefix", stringAt(v, "prefix")); value(v, "prefix").Kind != wire.KindString || e != nil {
		return errors.New("obligations prefix")
	}
	if rev, e := number(value(v, "revision"), taskswire.ObligationsMaxEvents); e != nil || rev < 1 {
		return errors.New("obligations revision")
	}
	if value(v, "head").Kind != wire.KindString || !digest(stringAt(v, "head")) {
		return errors.New("obligations head")
	}
	c := value(v, "counts")
	if e := object(c, "coreTotal coreWitnessed deferred total witnessed"); e != nil {
		return errors.New("obligations counts")
	}
	var n [5]uint64
	for i, k := range []string{"witnessed", "total", "deferred", "coreWitnessed", "coreTotal"} {
		x, e := number(value(c, k), taskswire.ObligationsMaxEntries)
		if e != nil {
			return errors.New("obligations count")
		}
		n[i] = x
	}
	witnessed, total, deferred, coreWitnessed, coreTotal := n[0], n[1], n[2], n[3], n[4]
	if witnessed > total || coreWitnessed > coreTotal || coreTotal > total || coreWitnessed > witnessed || total+deferred > taskswire.ObligationsMaxEntries {
		return errors.New("obligations counts are inconsistent")
	}
	hw := value(v, "highWater")
	if e := object(hw, "acceptanceRevision witnessed"); e != nil {
		return errors.New("obligations highWater")
	}
	hwRev, e := number(value(hw, "acceptanceRevision"), taskswire.MaxCountValue)
	if e != nil || hwRev < 1 || hwRev > acceptance {
		return errors.New("obligations highWater acceptanceRevision")
	}
	hwWitnessed, e := number(value(hw, "witnessed"), taskswire.ObligationsMaxEntries)
	if e != nil || (hwRev == acceptance && hwWitnessed < witnessed) {
		return errors.New("obligations highWater witnessed")
	}
	if lr := value(v, "lastRaise"); lr.Kind != wire.KindNull {
		if e := object(lr, "acceptanceRevision attempt generation"); e != nil {
			return errors.New("obligations lastRaise")
		}
		if lr.Obj.Values["attempt"].Kind != wire.KindString {
			return errors.New("obligations lastRaise attempt")
		}
		if _, e := taskswire.ParseIdentifier("/obligations/lastRaise/attempt", stringAt(lr, "attempt")); e != nil {
			return errors.New("obligations lastRaise attempt")
		}
		if _, e := number(value(lr, "generation"), ^uint64(0)); e != nil {
			return errors.New("obligations lastRaise generation")
		}
		r, e := number(value(lr, "acceptanceRevision"), taskswire.MaxCountValue)
		if e != nil || r < 1 || r > acceptance {
			return errors.New("obligations lastRaise acceptanceRevision")
		}
	}
	return nil
}
