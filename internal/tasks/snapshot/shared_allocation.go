package snapshot

import "github.com/Beamfall/corvint/internal/tasks/wire"

// SharedAllocation records that an attempt was admitted onto an allocation
// its holder already held through SourceAttemptID (PSR-V0-016, PSR-V0-018).
// It is binding history: the allocation itself is the attempt's
// poolAllocation, and pools.json remains the authority for who is bound now.
type SharedAllocation struct {
	SourceAttemptID            string
	SourceGeneration, BoundSeq wire.Size
}

func SharedAllocationValue(x *SharedAllocation) wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("sourceAttemptId", wire.String(x.SourceAttemptID)).Set("sourceGeneration", wire.String(string(x.SourceGeneration))).Set("boundSeq", wire.String(string(x.BoundSeq))))
}

func readSharedAllocation(r *wire.Reader) *SharedAllocation {
	r.Closed("sourceAttemptId", "sourceGeneration", "boundSeq")
	return &SharedAllocation{SourceAttemptID: r.Field("sourceAttemptId").Identifier(), SourceGeneration: r.Field("sourceGeneration").Size(), BoundSeq: r.Field("boundSeq").Size()}
}

// checkSharedAllocation keeps a shared binding on a pooled attempt that is
// not a direct admission, from another attempt of the same queue.
func (a *Attempt) checkSharedAllocation() error {
	x := a.SharedAllocation
	if x == nil {
		return nil
	}
	q, err := AttemptQueue(x.SourceAttemptID)
	if err != nil {
		return err
	}
	if aq, _ := AttemptQueue(a.AttemptID); a.PoolAllocation == nil || a.DirectPoolAdmission != nil || x.SourceAttemptID == a.AttemptID || q.Raw != aq.Raw || x.BoundSeq.Uint64() == 0 {
		return wire.Errorf(wire.CodeMalformed, "sharedAllocation", "shared allocation binding differs")
	}
	return nil
}
