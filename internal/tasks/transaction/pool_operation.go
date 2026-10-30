package transaction

import (
	"slices"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LeasePoolOperate runs one declared typed member lifecycle operation
// (PSR-V0-160..167).
const LeasePoolOperate = "POOL_OPERATE"

// Typed refusal reasons of POOL_OPERATE (PSR-V0-160, PSR-V0-161). Each
// prefixes the refusal detail; the result code stays a closed wire code.
const (
	ReasonOperationUndeclared   = "OPERATION_UNDECLARED"
	ReasonOperationStateIllegal = "OPERATION_STATE_ILLEGAL"
)

// operationStates are the member states each operation may start from
// (PSR-V0-161). FREE means no pools.json entry; a sweep-owned entry is never
// legal.
var operationStates = map[string][]string{
	"provision": {"FREE"},
	"teardown":  {"FREE"},
	"refresh":   {"FREE", "QUARANTINED"},
	"reset":     {"QUARANTINED"},
	"retarget":  {"QUARANTINED"},
}

func checkPoolOperation(l *LeaseRequest) error {
	if l.Verb != LeasePoolOperate {
		return nil
	}
	if !slices.Contains(intent.PoolOperations, l.Operation) {
		return malformed("unknown pool operation")
	}
	if (l.Operation == "retarget") != (l.Source != "") {
		return malformed("retarget takes --source, and only retarget does")
	}
	return nil
}

func planPoolOperate(c leaseContext) leaseOutcome {
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "pool operation blocked by barrier")
	}
	if c.r.Actor.Role != "OWNER" && c.r.Actor.Role != "OPERATOR" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, "pool operation requires operator attestation")
	}
	p := c.st.policy.Pool(c.l.Pool)
	if p == nil || !slices.Contains(p.Members, c.l.Member) {
		return c.fail(malformed("unknown pool member"))
	}
	op := c.l.Operation
	if p.MemberConfig[c.l.Member].Operations[op] == nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, ReasonOperationUndeclared+": pool "+p.ID+" member "+c.l.Member+" declares no "+op+" operation")
	}
	var en *snapshot.PoolEntry
	for _, x := range c.st.pools.Entries {
		if x.MemberID == c.l.Member {
			cp := x
			en = &cp
		}
	}
	state := "FREE"
	if en != nil {
		state = en.State
		if en.Sweep != nil {
			state = "SWEEP_OWNED"
		}
	}
	if !slices.Contains(operationStates[op], state) {
		code := wire.CodeQuiescenceUnproved
		if operationStates[op][0] == "FREE" {
			code = wire.CodeResourceCollision
		}
		return c.refuse(mutation.OutcomeBlocked, code, ReasonOperationStateIllegal+": "+op+" requires "+joinStates(operationStates[op])+"; member "+c.l.Member+" is "+state)
	}
	if c.l.Allocation != "" && (en == nil || string(en.AllocationID) != c.l.Allocation) {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation differs")
	}
	var source *intent.PoolSource
	if c.l.Source != "" {
		s, e := intent.ParsePoolSource("source", c.l.Source)
		if e != nil {
			return c.fail(e)
		}
		if len(p.AllowedSources) > 0 && !p.Allows(s) {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeOutOfScope, ReasonSourceNotAllowed+": source "+s.String()+" is not in pool "+p.ID+" allowedSources")
		}
		source = &s
	}
	f := c.in.LeaseFacts.Pool
	if f.RunnerPID == "" || f.RunnerPID == "0" || f.RunnerStarted == "" {
		return c.fail(malformed("pool operation lacks runner identity"))
	}
	if _, e := wire.ParseOID("command revision", f.Revision); e != nil {
		return c.fail(e)
	}
	pending := &snapshot.PoolOperationPending{Name: op, RequestID: c.r.RequestID, Source: source}
	if en == nil {
		en = &snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: p.ID, MemberID: c.l.Member, AllocationID: wire.Sum([]byte(c.r.QueueID + ":" + c.r.RequestID + ":" + c.l.Member)), DefinitionSha256: c.st.policy.MemberDefinition(p.ID, c.l.Member), AllocatedSeq: c.seq, ConfigRef: p.MemberConfig[c.l.Member].ConfigRef}, State: "PREPARING", Holder: c.r.Actor.ID, Generation: "0", PolicySha256: c.st.policy.PolicySha256(), RequestSha256: wire.Sum([]byte(c.r.QueueID + ":" + c.r.RequestID))}
	} else {
		en.State = "CLEANING"
		en.CleanupPassed = false
		en.ObservationSha256 = nil
	}
	en.RunnerPID = f.RunnerPID
	en.RunnerStarted = f.RunnerStarted
	en.CommandKind = op
	en.CommandRevision = f.Revision
	en.Operation = pending
	return c.putPool(en, false, nil)
}

func joinStates(states []string) string {
	out := states[0]
	for _, s := range states[1:] {
		out += " or " + s
	}
	return out
}

// observeOperation settles a typed operation's observation (PSR-V0-162,
// PSR-V0-164). A passed operation that started FREE frees an unguarded member
// again; any other run leaves it QUARANTINED with the operation named in the
// reason. An identity-guarded member is never freed here: an operation may
// change its physical source, so it stays QUARANTINED until confirm-safe's
// identity probe reports the defaultSource (PSR-V0-084). Either way the
// member's receipt for the operation is replaced.
func observeOperation(c leaseContext, en *snapshot.PoolEntry, o *snapshot.PoolObservation) leaseOutcome {
	out := o.OutputSha256
	rec := snapshot.PoolOperationReceipt{PoolID: en.PoolID, MemberID: en.MemberID, Operation: en.Operation.Name, RequestID: en.Operation.RequestID, AllocationID: en.AllocationID, Class: o.Class, Passed: o.Passed, OutputSha256: &out, Source: en.Operation.Source, ObservedSeq: c.seq}
	guarded := identityGuarded(c.st.policy, en)
	remove := o.Passed && en.State == "PREPARING" && !guarded
	d := wire.Sum(c.in.LeaseFacts.Pool.Observation)
	en.ObservationSha256 = &d
	en.CleanupPassed = false
	en.State = "QUARANTINED"
	if remove {
		en.State = "FREE"
	}
	en.Reason = en.Operation.Name + " " + o.Class + "; operator safe-reuse confirmation required"
	if guarded {
		en.Reason = ReasonIdentityUnproved + ": " + en.Operation.Name + " " + o.Class + "; confirm-safe identity observation required"
	}
	en.RunnerPID = "0"
	en.RunnerStarted = ""
	en.Operation = nil
	return c.withOperationReceipt(rec).putPool(en, remove, c.in.LeaseFacts.Pool.Observation)
}

// recoverOperation records an orphaned operation (PSR-V0-165).
func recoverOperation(c leaseContext, en *snapshot.PoolEntry) leaseOutcome {
	rec := snapshot.PoolOperationReceipt{PoolID: en.PoolID, MemberID: en.MemberID, Operation: en.Operation.Name, RequestID: en.Operation.RequestID, AllocationID: en.AllocationID, Class: snapshot.PoolOperationOrphaned, Source: en.Operation.Source, ObservedSeq: c.seq}
	en.State = "QUARANTINED"
	en.Reason = "orphaned " + en.Operation.Name + " command; execution and physical cleanup unknown: " + c.l.Reason
	en.RunnerPID = "0"
	en.RunnerStarted = ""
	en.CleanupPassed = false
	en.Operation = nil
	return c.withOperationReceipt(rec).putPool(en, false, nil)
}

// withOperationReceipt is c over a pool state whose receipts replace the
// member's receipt for rec's operation and drop receipts of members no pool
// declares any longer. It then retains at most MaxPoolOperationReceipts,
// evicting the oldest by observedSeq (PSR-V0-196); rec is the newest.
func (c leaseContext) withOperationReceipt(rec snapshot.PoolOperationReceipt) leaseContext {
	state := *c.st.pools
	state.Operations = []snapshot.PoolOperationReceipt{}
	for _, x := range c.st.pools.Operations {
		if (x.MemberID != rec.MemberID || x.Operation != rec.Operation) && poolDeclares(c.st.policy, x.PoolID, x.MemberID) {
			state.Operations = append(state.Operations, x)
		}
	}
	if excess := len(state.Operations) + 1 - snapshot.MaxPoolOperationReceipts; excess > 0 {
		sort.SliceStable(state.Operations, func(i, j int) bool {
			return state.Operations[i].ObservedSeq.Uint64() > state.Operations[j].ObservedSeq.Uint64()
		})
		state.Operations = state.Operations[:len(state.Operations)-excess]
	}
	state.Operations = append(state.Operations, rec)
	c.st.pools = &state
	return c
}
