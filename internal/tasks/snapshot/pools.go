package snapshot

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
)

// MaxPoolStateBytes bounds pools.json. It is 3 MiB, derived from a superset
// of every reachable state: every entry with every optional field, every
// identifier all-escaping and full slot and waiter lists (PSR-V0-900).
const MaxPoolStateBytes = wire.MaxPoolStateBytes
const ProfilePools = "taskman-pool-state/0"

// MaxSharedAttempts bounds the attempts one ALLOCATED member holds at once,
// the original included (PSR-V0-016). A small fixed bound keeps one member's
// blast radius, its pools.json entry and its pool status row bounded.
const MaxSharedAttempts = 4

// PoolShare is one further attempt bound to an ALLOCATED entry's allocation
// by `claim --share-allocation` (PSR-V0-016..019).
type PoolShare struct {
	AttemptID  string
	Generation wire.Size
}

type PoolAllocation struct {
	PoolID, MemberID               string
	AllocationID, DefinitionSha256 wire.Digest
	AllocatedSeq                   wire.Size
	ConfigRef                      *intent.ConfigRef
}
type PoolEntry struct {
	RunnerPID                                   wire.Count
	RunnerStarted, CommandKind, CommandRevision string
	CleanupPassed                               bool
	Sweep                                       *PoolSweepOwner
	PoolAllocation
	State, Holder, Stage, AttemptID string
	Generation, ChangedSeq          wire.Size
	PolicySha256, RequestSha256     wire.Digest
	ObservationSha256               *wire.Digest
	Reason                          string
	// Shared lists the further bound attempts in binding order; it is
	// written only when non-empty, so legacy bytes stay identical.
	Shared []PoolShare
	// Source is the approved pair `pool acquire --source` targeted
	// (PSR-V0-081); written only when present.
	Source *intent.PoolSource
	// Operation binds a PREPARING or CLEANING entry to the typed lifecycle
	// operation it runs (PSR-V0-161); written only when present.
	Operation *PoolOperationPending
	// QuarantinedAt is when the member entered QUARANTINED (PSR-V0-261); it
	// is carried through CLEANING and written only when present, so a
	// legacy entry stays NOT_OBSERVED and its bytes stay identical.
	QuarantinedAt wire.Timestamp
	// Recovery is the last failed readiness observation (PSR-V0-260/262);
	// written only when present.
	Recovery *PoolRecovery
	// Hold is the bounded operator hold window of a HELD entry
	// (PSR-V0-268); written exactly when the state is HELD.
	Hold *PoolHold
}

// PoolHold is when an operator hold began and when it lapses (PSR-V0-268,
// PSR-V0-270). A lapsed hold still occupies its member until it is released.
type PoolHold struct {
	HeldAt, ExpiresAt wire.Timestamp
}

// Expired reports whether the hold window has lapsed at now.
func (h *PoolHold) Expired(now wire.Timestamp) bool {
	return h != nil && string(now) >= string(h.ExpiresAt)
}

// PoolHoldValue is the wire object of a hold window.
func PoolHoldValue(h *PoolHold) wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("heldAt", wire.String(string(h.HeldAt))).Set("expiresAt", wire.String(string(h.ExpiresAt))))
}

type PoolState struct {
	QueueID wire.QueueID
	Entries []PoolEntry
	// Slots are the capacity-slot holders and waiters (PSR-V0-031); the
	// "slots" key is written only when non-empty, so legacy bytes stay
	// identical.
	Slots []SlotEntry
	// Identities are the last recorded identity observation per member
	// (PSR-V0-083); the key is written only when non-empty.
	Identities []PoolIdentity
	// Waiters are the pool waitlist entries (PSR-V0-130..137); the key is
	// written only when non-empty.
	Waiters []PoolWaiter
	// Operations are the last recorded typed operation receipt per member
	// and operation (PSR-V0-162); the key is written only when non-empty.
	Operations []PoolOperationReceipt
}

func ReadPoolAllocation(r *wire.Reader) *PoolAllocation {
	r.Closed(wire.OptionalKeys(r.Value(), []string{"poolId", "memberId", "allocationId", "definitionSha256", "allocatedSeq"}, "configRef")...)
	a := &PoolAllocation{PoolID: r.Field("poolId").Label(), MemberID: r.Field("memberId").Label(), AllocationID: r.Field("allocationId").Digest(), DefinitionSha256: r.Field("definitionSha256").Digest(), AllocatedSeq: r.Field("allocatedSeq").Size()}
	if wire.Has(r.Value(), "configRef") {
		a.ConfigRef = intent.ReadConfigRef(r.Field("configRef"))
	}
	return a
}
func PoolAllocationValue(a *PoolAllocation) wire.Value {
	o := wire.NewObject().Set("poolId", wire.String(a.PoolID)).Set("memberId", wire.String(a.MemberID)).Set("allocationId", wire.String(string(a.AllocationID))).Set("definitionSha256", wire.String(string(a.DefinitionSha256))).Set("allocatedSeq", wire.String(string(a.AllocatedSeq)))
	if a.ConfigRef != nil {
		o.Set("configRef", intent.ConfigRefValue(a.ConfigRef))
	}
	return wire.ObjectValue(o)
}
func DecodePools(data []byte) (*PoolState, error) {
	if len(data) > MaxPoolStateBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "pools", "pool state bound")
	}
	v, e := wire.Parse(data)
	if e != nil {
		return nil, e
	}
	r := wire.NewReader(v, "/")
	if e := r.Profile(ProfilePools); e != nil {
		return nil, e
	}
	r.Closed(wire.OptionalKeys(v, []string{"profile", "queueId", "entries"}, "slots", "identities", "waiters", "operations")...)
	r.Field("profile").Exact(ProfilePools)
	p := &PoolState{QueueID: r.Field("queueId").QueueID(), Entries: []PoolEntry{}}
	seen := map[string]bool{}
	for _, x := range r.Field("entries").Array(intent.MaxPoolMembers, true) {
		x.Closed(wire.OptionalKeys(x.Value(), []string{"allocation", "state", "holder", "stage", "attemptId", "generation", "changedSeq", "policySha256", "requestSha256", "observationSha256", "reason", "runnerPid", "runnerStarted", "commandKind", "commandRevision", "cleanupPassed"}, "sweep", "shared", "source", "operation", "quarantinedAt", "recovery", "hold")...)
		en := PoolEntry{RunnerPID: x.Field("runnerPid").Count(), RunnerStarted: x.Field("runnerStarted").Prose(0, 128), CommandKind: x.Field("commandKind").String(), CommandRevision: x.Field("commandRevision").String(), CleanupPassed: x.Field("cleanupPassed").Bool(), PoolAllocation: *ReadPoolAllocation(x.Field("allocation")), State: x.Field("state").Enum("PREPARING", "ALLOCATED", "QUARANTINED", "CLEANING", "HELD"), Holder: x.Field("holder").Label(), Stage: x.Field("stage").String(), AttemptID: x.Field("attemptId").String(), Generation: x.Field("generation").Size(), ChangedSeq: x.Field("changedSeq").Size(), PolicySha256: x.Field("policySha256").Digest(), RequestSha256: x.Field("requestSha256").Digest(), ObservationSha256: x.Field("observationSha256").DigestOrNull(), Reason: x.Field("reason").Prose(0, 4096)}
		if wire.Has(x.Value(), "sweep") {
			en.Sweep = ReadPoolSweepOwner(x.Field("sweep"))
		}
		if wire.Has(x.Value(), "shared") {
			en.Shared = readPoolShares(x.Field("shared"), p.QueueID, en)
		}
		if wire.Has(x.Value(), "source") {
			en.Source = readPoolSourceOrNull(x.Field("source"))
			if en.Source == nil {
				x.Fail(wire.CodeMalformed, "a recorded source is a pair")
			}
		}
		if wire.Has(x.Value(), "operation") {
			en.Operation = readPoolOperationPending(x.Field("operation"))
			if x.Err() == nil && ((en.State != "PREPARING" && en.State != "CLEANING") || en.CommandKind != en.Operation.Name) {
				x.Fail(wire.CodeMalformed, "a pending operation binds a PREPARING or CLEANING entry running it")
			}
		}
		if wire.Has(x.Value(), "quarantinedAt") {
			en.QuarantinedAt = x.Field("quarantinedAt").Timestamp()
		}
		if wire.Has(x.Value(), "recovery") {
			en.Recovery = readPoolRecovery(x.Field("recovery"))
		}
		if wire.Has(x.Value(), "hold") {
			h := x.Field("hold")
			h.Closed("heldAt", "expiresAt")
			en.Hold = &PoolHold{HeldAt: h.Field("heldAt").Timestamp(), ExpiresAt: h.Field("expiresAt").Timestamp()}
			if x.Err() == nil && string(en.Hold.ExpiresAt) <= string(en.Hold.HeldAt) {
				x.Fail(wire.CodeMalformed, "a hold expires after it began")
			}
		}
		// PSR-V0-268: a HELD entry is an operator hold and nothing else: it
		// carries its window, no attempt, runner, command or other binding.
		if (en.State == "HELD") != (en.Hold != nil) {
			x.Fail(wire.CodeMalformed, "a hold window binds exactly a HELD entry")
		}
		if en.State == "HELD" && (en.AttemptID != "" || en.RunnerPID.Int() != 0 || en.RunnerStarted != "" || en.CommandKind != "" || en.Sweep != nil || wire.Has(x.Value(), "shared") || en.Source != nil || en.Operation != nil) {
			x.Fail(wire.CodeMalformed, "a HELD entry carries no attempt, runner, command, sweep, share, source or operation")
		}
		if (en.QuarantinedAt != "" || en.Recovery != nil) && en.State != "QUARANTINED" && en.State != "CLEANING" {
			x.Fail(wire.CodeMalformed, "quarantinedAt and recovery bind a QUARANTINED or CLEANING entry")
		}
		if en.Stage != "" {
			x.Field("stage").Enum(intent.StageRoles...)
		}
		if en.AttemptID != "" {
			if q, e := AttemptQueue(en.AttemptID); e != nil || q.Raw != p.QueueID.Raw {
				x.Fail(wire.CodeMalformed, "invalid pool owner attempt")
			}
		}
		if en.AllocatedSeq.Uint64() == 0 || en.ChangedSeq.Uint64() < en.AllocatedSeq.Uint64() {
			x.Fail(wire.CodeMalformed, "invalid allocation sequence")
		}
		if en.CommandKind != "" {
			x.Field("commandKind").Enum(append([]string{"health", "cleanup", "sweep"}, intent.PoolOperations...)...)
			x.Field("commandRevision").OID()
		}
		if en.State == "PREPARING" || en.State == "CLEANING" {
			if en.RunnerPID.Int() <= 0 || en.RunnerStarted == "" || en.CommandKind == "" {
				x.Fail(wire.CodeMalformed, "pending command lacks runner binding")
			}
		}
		if seen[en.MemberID] {
			x.Fail(wire.CodeDuplicateID, "duplicate pool occupancy")
		}
		seen[en.MemberID] = true
		if en.State == "ALLOCATED" && en.AttemptID == "" {
			x.Fail(wire.CodeMalformed, "allocated member lacks owner")
		}
		p.Entries = append(p.Entries, en)
	}
	if r.Err() == nil && wire.Has(v, "slots") {
		p.Slots = readSlotEntries(r.Field("slots"), p.QueueID)
	}
	if r.Err() == nil && wire.Has(v, "waiters") {
		p.Waiters = readPoolWaiters(r.Field("waiters"), p.QueueID)
	}
	if r.Err() == nil && wire.Has(v, "operations") {
		p.Operations = readPoolOperationReceipts(r.Field("operations"))
	}
	if r.Err() == nil && wire.Has(v, "identities") {
		members := map[string]bool{}
		for _, x := range r.Field("identities").Array(intent.MaxPoolMembers, true) {
			id := readPoolIdentity(x)
			if members[id.MemberID] || (len(p.Identities) > 0 && p.Identities[len(p.Identities)-1].MemberID > id.MemberID) {
				x.Fail(wire.CodeMalformed, "identities are sorted by member, one per member")
			}
			members[id.MemberID] = true
			p.Identities = append(p.Identities, id)
		}
		if len(p.Identities) == 0 {
			r.Fail(wire.CodeMalformed, "identities must be non-empty when present")
		}
	}
	return p, r.Err()
}
func (p *PoolState) Encode() ([]byte, error) {
	entries := append([]PoolEntry{}, p.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].MemberID < entries[j].MemberID })
	values := []wire.Value{}
	for _, e := range entries {
		if e.RunnerPID == "" {
			e.RunnerPID = "0"
		}
		value := wire.ObjectValue(wire.NewObject().Set("runnerPid", wire.String(string(e.RunnerPID))).Set("runnerStarted", wire.String(e.RunnerStarted)).Set("commandKind", wire.String(e.CommandKind)).Set("commandRevision", wire.String(e.CommandRevision)).Set("cleanupPassed", wire.Bool(e.CleanupPassed)).Set("allocation", PoolAllocationValue(&e.PoolAllocation)).Set("state", wire.String(e.State)).Set("holder", wire.String(e.Holder)).Set("stage", wire.String(e.Stage)).Set("attemptId", wire.String(e.AttemptID)).Set("generation", wire.String(string(e.Generation))).Set("changedSeq", wire.String(string(e.ChangedSeq))).Set("policySha256", wire.String(string(e.PolicySha256))).Set("requestSha256", wire.String(string(e.RequestSha256))).Set("observationSha256", digestOrNull(e.ObservationSha256)).Set("reason", wire.String(e.Reason)))
		if e.Sweep != nil {
			value.Obj.Set("sweep", PoolSweepOwnerValue(e.Sweep))
		}
		if len(e.Shared) > 0 {
			shares := []wire.Value{}
			for _, x := range e.Shared {
				shares = append(shares, wire.ObjectValue(wire.NewObject().Set("attemptId", wire.String(x.AttemptID)).Set("generation", wire.String(string(x.Generation)))))
			}
			value.Obj.Set("shared", wire.Array(shares...))
		}
		if e.Source != nil {
			value.Obj.Set("source", PoolSourceValue(e.Source))
		}
		if e.Operation != nil {
			value.Obj.Set("operation", e.Operation.value())
		}
		if e.QuarantinedAt != "" {
			value.Obj.Set("quarantinedAt", wire.String(string(e.QuarantinedAt)))
		}
		if e.Recovery != nil {
			value.Obj.Set("recovery", e.Recovery.value())
		}
		if e.Hold != nil {
			value.Obj.Set("hold", PoolHoldValue(e.Hold))
		}
		values = append(values, value)
	}
	doc := wire.NewObject().Set("profile", wire.String(ProfilePools)).Set("queueId", wire.String(p.QueueID.Raw)).Set("entries", wire.Array(values...))
	if len(p.Slots) > 0 {
		doc.Set("slots", slotEntriesValue(p.Slots))
	}
	if len(p.Waiters) > 0 {
		doc.Set("waiters", poolWaitersValue(p.Waiters))
	}
	if len(p.Operations) > 0 {
		doc.Set("operations", poolOperationReceiptsValue(p.Operations))
	}
	if len(p.Identities) > 0 {
		ids := append([]PoolIdentity{}, p.Identities...)
		sort.Slice(ids, func(i, j int) bool { return ids[i].MemberID < ids[j].MemberID })
		values := []wire.Value{}
		for _, x := range ids {
			values = append(values, x.Value())
		}
		doc.Set("identities", wire.Array(values...))
	}
	raw := wire.EncodeFile(wire.ObjectValue(doc))
	_, err := DecodePools(raw)
	return raw, err
}

// readPoolShares reads a present "shared" list: non-empty, bounded, only on an
// ALLOCATED entry, and naming distinct same-queue attempts other than the
// entry's own (PSR-V0-017).
func readPoolShares(r *wire.Reader, q wire.QueueID, en PoolEntry) []PoolShare {
	out := []PoolShare{}
	seen := map[string]bool{en.AttemptID: true}
	for _, x := range r.Array(MaxSharedAttempts-1, true) {
		x.Closed("attemptId", "generation")
		share := PoolShare{AttemptID: x.Field("attemptId").Identifier(), Generation: x.Field("generation").Size()}
		if aq, e := AttemptQueue(share.AttemptID); e != nil || aq.Raw != q.Raw {
			x.Fail(wire.CodeMalformed, "invalid shared attempt")
		}
		if seen[share.AttemptID] {
			x.Fail(wire.CodeDuplicateID, "duplicate shared attempt")
		}
		seen[share.AttemptID] = true
		out = append(out, share)
	}
	if len(out) == 0 || en.State != "ALLOCATED" {
		r.Fail(wire.CodeMalformed, "shared attempts must be non-empty on an ALLOCATED entry")
	}
	return out
}
