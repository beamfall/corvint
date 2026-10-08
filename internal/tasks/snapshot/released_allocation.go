package snapshot

import "github.com/Beamfall/corvint/internal/tasks/wire"

// ReleasedPoolAllocation records the allocation a live external-agent
// generation returned early with POOL_RELEASE (CAL-V0-200). It is binding
// history: pools.json quarantines the member, and the generation's author
// history still names it (CAL-V0-202). Omitted when nothing was returned, so
// every earlier attempt keeps its bytes.
type ReleasedPoolAllocation struct {
	Allocation  PoolAllocation
	ReleasedSeq wire.Size
}

func ReleasedPoolAllocationValue(x *ReleasedPoolAllocation) wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("allocation", PoolAllocationValue(&x.Allocation)).Set("releasedSeq", wire.String(string(x.ReleasedSeq))))
}

func readReleasedPoolAllocation(r *wire.Reader) *ReleasedPoolAllocation {
	r.Closed("allocation", "releasedSeq")
	x := &ReleasedPoolAllocation{ReleasedSeq: r.Field("releasedSeq").Size()}
	if a := ReadPoolAllocation(r.Field("allocation")); a != nil {
		x.Allocation = *a
	}
	return x
}

// checkReleasedPoolAllocation keeps a returned allocation on an
// external-agent attempt that holds no other one and returned it after it
// was allocated.
func (a *Attempt) checkReleasedPoolAllocation() error {
	x := a.ReleasedPoolAllocation
	if x == nil {
		return nil
	}
	if a.RuntimeID != RuntimeExternalAgent || a.PoolAllocation != nil || a.SharedAllocation != nil || x.ReleasedSeq.Uint64() <= x.Allocation.AllocatedSeq.Uint64() {
		return wire.Errorf(wire.CodeMalformed, "releasedPoolAllocation", "returned allocation binding differs")
	}
	return nil
}
