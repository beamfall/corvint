package snapshot

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfilePoolOperation is the PSR-V0-162 typed operation receipt profile.
const ProfilePoolOperation = "taskman-pool-operation/0"

// PoolOperationOrphaned is the class `pool recover` records for an operation
// whose runner died before observing it (PSR-V0-165).
const PoolOperationOrphaned = "ORPHANED"

// PoolOperationClasses are the closed operation receipt classes: the pool
// observation classes plus ORPHANED.
var PoolOperationClasses = []string{"EXIT_ZERO", "EXIT_NONZERO", "SPAWN_FAILED", "TIMEOUT", "INTERRUPTED", "OUTPUT_LIMIT", "UNKNOWN", "SOURCE_CHANGED", PoolOperationOrphaned}

// PoolOperationPending binds a PREPARING or CLEANING entry to the typed
// operation it is running (PSR-V0-161). Source is retarget's target pair.
type PoolOperationPending struct {
	Name, RequestID string
	Source          *intent.PoolSource
}

// PoolOperationReceipt is the last recorded run of one member operation
// (PSR-V0-162). OutputSha256 is nil for an ORPHANED run.
type PoolOperationReceipt struct {
	PoolID, MemberID, Operation, RequestID string
	AllocationID                           wire.Digest
	Class                                  string
	Passed                                 bool
	OutputSha256                           *wire.Digest
	Source                                 *intent.PoolSource
	ObservedSeq                            wire.Size
}

func (p PoolOperationPending) value() wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("name", wire.String(p.Name)).Set("requestId", wire.String(p.RequestID)).Set("source", PoolSourceValue(p.Source)))
}

func readPoolOperationPending(r *wire.Reader) *PoolOperationPending {
	r.Closed("name", "requestId", "source")
	p := &PoolOperationPending{Name: r.Field("name").Enum(intent.PoolOperations...), RequestID: r.Field("requestId").Identifier(), Source: readPoolSourceOrNull(r.Field("source"))}
	if r.Err() == nil && (p.Name == "retarget") != (p.Source != nil) {
		r.Fail(wire.CodeMalformed, "an operation carries a source pair exactly when it is retarget")
	}
	return p
}

// Value is the receipt's canonical object.
func (p PoolOperationReceipt) Value() wire.Value {
	o := wire.NewObject().Set("profile", wire.String(ProfilePoolOperation)).Set("poolId", wire.String(p.PoolID)).Set("memberId", wire.String(p.MemberID)).Set("operation", wire.String(p.Operation)).Set("requestId", wire.String(p.RequestID)).Set("allocationId", wire.String(string(p.AllocationID))).Set("class", wire.String(p.Class)).Set("passed", wire.Bool(p.Passed)).Set("outputSha256", digestOrNull(p.OutputSha256)).Set("source", PoolSourceValue(p.Source)).Set("observedSeq", wire.String(string(p.ObservedSeq)))
	return wire.ObjectValue(o)
}

func readPoolOperationReceipt(r *wire.Reader) PoolOperationReceipt {
	// CAL-V0-131: a newer receipt is UNSUPPORTED_VERSION; the nested
	// pools.json reader relies on the first recorded error sticking.
	if r.Profile(ProfilePoolOperation) != nil {
		return PoolOperationReceipt{}
	}
	r.Closed("profile", "poolId", "memberId", "operation", "requestId", "allocationId", "class", "passed", "outputSha256", "source", "observedSeq")
	r.Field("profile").Exact(ProfilePoolOperation)
	p := PoolOperationReceipt{PoolID: r.Field("poolId").Label(), MemberID: r.Field("memberId").Label(), Operation: r.Field("operation").Enum(intent.PoolOperations...), RequestID: r.Field("requestId").Identifier(), AllocationID: r.Field("allocationId").Digest(), Class: r.Field("class").Enum(PoolOperationClasses...), Passed: r.Field("passed").Bool(), OutputSha256: r.Field("outputSha256").DigestOrNull(), Source: readPoolSourceOrNull(r.Field("source")), ObservedSeq: r.Field("observedSeq").Size()}
	if r.Err() != nil {
		return p
	}
	if (p.Class == PoolOperationOrphaned) != (p.OutputSha256 == nil) {
		r.Fail(wire.CodeMalformed, "an operation receipt lacks an output digest exactly when it is ORPHANED")
	}
	if p.Passed && p.Class != "EXIT_ZERO" {
		r.Fail(wire.CodeMalformed, "a passed operation exited zero")
	}
	if (p.Operation == "retarget") != (p.Source != nil) {
		r.Fail(wire.CodeMalformed, "an operation receipt carries a source pair exactly when it is retarget")
	}
	return p
}

// readPoolOperationReceipts reads a present "operations" list: non-empty,
// sorted by member then operation, one receipt per pair (PSR-V0-162).
// MaxPoolOperationReceipts is the number of operation receipts a pools.json
// writer retains (PSR-V0-196): two per member on average, so a full-size pool
// keeps its encoding well under MaxPoolStateBytes. Readers still accept the
// earlier one-per-member-and-operation bound so an older file stays readable;
// its next operation write trims it.
const MaxPoolOperationReceipts = 2 * intent.MaxPoolMembers

func readPoolOperationReceipts(r *wire.Reader) []PoolOperationReceipt {
	out := []PoolOperationReceipt{}
	for _, x := range r.Array(intent.MaxPoolMembers*len(intent.PoolOperations), true) {
		p := readPoolOperationReceipt(x)
		if n := len(out); n > 0 && !operationBefore(out[n-1], p) {
			x.Fail(wire.CodeMalformed, "operation receipts are sorted by member and operation, one per pair")
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		r.Fail(wire.CodeMalformed, "operation receipts must be non-empty when present")
	}
	return out
}

func operationBefore(a, b PoolOperationReceipt) bool {
	if a.MemberID != b.MemberID {
		return a.MemberID < b.MemberID
	}
	return a.Operation < b.Operation
}

func poolOperationReceiptsValue(in []PoolOperationReceipt) wire.Value {
	ops := append([]PoolOperationReceipt{}, in...)
	sort.Slice(ops, func(i, j int) bool { return operationBefore(ops[i], ops[j]) })
	values := []wire.Value{}
	for _, x := range ops {
		values = append(values, x.Value())
	}
	return wire.Array(values...)
}

// PoolOperationReceiptFor is the member's last receipt for op, or nil.
func (p *PoolState) PoolOperationReceiptFor(member, op string) *PoolOperationReceipt {
	for i := range p.Operations {
		if p.Operations[i].MemberID == member && p.Operations[i].Operation == op {
			return &p.Operations[i]
		}
	}
	return nil
}
