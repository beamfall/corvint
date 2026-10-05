package cli

import (
	"strconv"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// preparationAdmission reports writer-admission pressure for queue status
// (CAL-V0-074). The read takes no lock and writes nothing; unobservable facts
// are NOT_OBSERVED. No per-mutation writer cost is recorded in the store, so
// the wait estimate is NOT_OBSERVED rather than invented.
func preparationAdmission(repo *intent.Repository) wire.Value {
	q := authority.ObservePreparationQueue(repo)
	notObserved := wire.String("NOT_OBSERVED")
	o := wire.NewObject()
	o.Set("capacity", wire.String("64"))
	o.Set("estimatedWait", notObserved)
	o.Set("estimatedWaitBasis", wire.String("no recorded per-mutation writer cost"))
	if q.NotObserved != "" {
		o.Set("snapshot", notObserved)
		o.Set("method", notObserved)
		o.Set("notObservedReason", wire.String(q.NotObserved))
		o.Set("registeredWriters", notObserved)
		o.Set("unpublishedSlots", notObserved)
		o.Set("wouldBeRank", notObserved)
		o.Set("registryActive", wire.Null())
		return wire.ObjectValue(o)
	}
	o.Set("snapshot", wire.String("RACY"))
	o.Set("method", wire.String(q.Method))
	o.Set("notObservedReason", wire.Null())
	o.Set("registeredWriters", wire.String(string(wire.CountOf(int64(q.Registered)))))
	o.Set("unpublishedSlots", wire.String(string(wire.CountOf(int64(q.Unpublished)))))
	if rank, ok := q.WouldBeRank(); ok {
		o.Set("wouldBeRank", wire.String(strconv.FormatUint(rank, 10)))
	} else {
		o.Set("wouldBeRank", notObserved)
	}
	o.Set("registryActive", wire.Bool(q.RegistryActive))
	return wire.ObjectValue(o)
}
