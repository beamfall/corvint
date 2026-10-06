package snapshot

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
)

const MaxPoolStateBytes = 1 << 20
const ProfilePools = "taskman-pool-state/0"

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
}
type PoolState struct {
	QueueID wire.QueueID
	Entries []PoolEntry
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
	r.Profile(ProfilePools)
	r.Closed("profile", "queueId", "entries")
	r.Field("profile").Exact(ProfilePools)
	p := &PoolState{QueueID: r.Field("queueId").QueueID(), Entries: []PoolEntry{}}
	seen := map[string]bool{}
	for _, x := range r.Field("entries").Array(intent.MaxPoolMembers, true) {
		x.Closed(wire.OptionalKeys(x.Value(), []string{"allocation", "state", "holder", "stage", "attemptId", "generation", "changedSeq", "policySha256", "requestSha256", "observationSha256", "reason", "runnerPid", "runnerStarted", "commandKind", "commandRevision", "cleanupPassed"}, "sweep")...)
		en := PoolEntry{RunnerPID: x.Field("runnerPid").Count(), RunnerStarted: x.Field("runnerStarted").Prose(0, 128), CommandKind: x.Field("commandKind").String(), CommandRevision: x.Field("commandRevision").String(), CleanupPassed: x.Field("cleanupPassed").Bool(), PoolAllocation: *ReadPoolAllocation(x.Field("allocation")), State: x.Field("state").Enum("PREPARING", "ALLOCATED", "QUARANTINED", "CLEANING"), Holder: x.Field("holder").Label(), Stage: x.Field("stage").String(), AttemptID: x.Field("attemptId").String(), Generation: x.Field("generation").Size(), ChangedSeq: x.Field("changedSeq").Size(), PolicySha256: x.Field("policySha256").Digest(), RequestSha256: x.Field("requestSha256").Digest(), ObservationSha256: x.Field("observationSha256").DigestOrNull(), Reason: x.Field("reason").Prose(0, 4096)}
		if wire.Has(x.Value(), "sweep") {
			en.Sweep = ReadPoolSweepOwner(x.Field("sweep"))
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
			x.Field("commandKind").Enum("health", "cleanup", "sweep")
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
		values = append(values, value)
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(ProfilePools)).Set("queueId", wire.String(p.QueueID.Raw)).Set("entries", wire.Array(values...))))
	_, err := DecodePools(raw)
	return raw, err
}
