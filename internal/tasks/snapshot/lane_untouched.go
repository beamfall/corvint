package snapshot

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const ProfileDirectPoolAdmission = "taskman-direct-pool-admission/0"
const ProfileLaneUntouched = "taskman-lane-untouched-attestation/0"

// DirectPoolAdmission is prospective writer evidence, never inferred from empty
// command metadata. Retries and prepared admissions do not inherit it.
type DirectPoolAdmission struct {
	AttemptID                        string
	Generation, OriginalAdmissionSeq wire.Size
	Allocation                       *PoolAllocation
	Holder, Stage                    string
}

// LaneUntouchedAttestation records a local operator claim, not authentication
// or observation of physical non-use. The four acknowledgements are fixed true.
type LaneUntouchedAttestation struct {
	DirectPoolAdmission
	ActorID, ActorRole, Evidence string
	RecordedSeq                  wire.Size
	RecordedAt                   wire.Timestamp
}

var untouchedAcknowledgements = []string{"noLaneAccess", "noLaneCommands", "noPhysicalCapabilityIssuedOrRetained", "operatorAcceptsSafeReuseResponsibility"}

func LaneUntouchedAcknowledgementsValue() wire.Value {
	o := wire.NewObject()
	for _, name := range untouchedAcknowledgements {
		o.Set(name, wire.Bool(true))
	}
	return wire.ObjectValue(o)
}

func directPoolValue(x *DirectPoolAdmission) *wire.Object {
	return wire.NewObject().Set("attemptId", wire.String(x.AttemptID)).Set("generation", wire.String(string(x.Generation))).Set("originalAdmissionSeq", wire.String(string(x.OriginalAdmissionSeq))).Set("allocation", PoolAllocationValue(x.Allocation)).Set("holder", wire.String(x.Holder)).Set("stage", wire.String(x.Stage))
}
func DirectPoolAdmissionValue(x *DirectPoolAdmission) wire.Value {
	return wire.ObjectValue(directPoolValue(x).Set("profile", wire.String(ProfileDirectPoolAdmission)))
}
func LaneUntouchedAttestationValue(x *LaneUntouchedAttestation) wire.Value {
	return wire.ObjectValue(directPoolValue(&x.DirectPoolAdmission).Set("profile", wire.String(ProfileLaneUntouched)).Set("actorId", wire.String(x.ActorID)).Set("actorRole", wire.String(x.ActorRole)).Set("evidence", wire.String(x.Evidence)).Set("recordedSeq", wire.String(string(x.RecordedSeq))).Set("recordedAt", wire.String(string(x.RecordedAt))).Set("acknowledgements", LaneUntouchedAcknowledgementsValue()).Set("physicalFacts", wire.String("NOT_OBSERVED")))
}
func readDirectPool(r *wire.Reader) *DirectPoolAdmission {
	x := &DirectPoolAdmission{AttemptID: r.Field("attemptId").Identifier(), Generation: r.Field("generation").Size(), OriginalAdmissionSeq: r.Field("originalAdmissionSeq").Size(), Allocation: ReadPoolAllocation(r.Field("allocation")), Holder: r.Field("holder").Label(), Stage: r.Field("stage").String()}
	if x.Stage != "" {
		r.Field("stage").Enum(intent.StageRoles...)
	}
	return x
}
func readDirectPoolAdmission(r *wire.Reader) *DirectPoolAdmission {
	r.Profile(ProfileDirectPoolAdmission)
	r.Closed("profile", "attemptId", "generation", "originalAdmissionSeq", "allocation", "holder", "stage")
	r.Field("profile").Exact(ProfileDirectPoolAdmission)
	return readDirectPool(r)
}
func readLaneUntouched(r *wire.Reader) *LaneUntouchedAttestation {
	r.Profile(ProfileLaneUntouched)
	r.Closed("profile", "attemptId", "generation", "originalAdmissionSeq", "allocation", "holder", "stage", "actorId", "actorRole", "evidence", "recordedSeq", "recordedAt", "acknowledgements", "physicalFacts")
	r.Field("profile").Exact(ProfileLaneUntouched)
	r.Field("physicalFacts").Exact("NOT_OBSERVED")
	ack := r.Field("acknowledgements")
	ack.Closed(untouchedAcknowledgements...)
	for _, name := range untouchedAcknowledgements {
		if !ack.Field(name).Bool() {
			ack.Fail(wire.CodeMalformed, "all acknowledgements must be true")
		}
	}
	return &LaneUntouchedAttestation{DirectPoolAdmission: *readDirectPool(r), ActorID: r.Field("actorId").Label(), ActorRole: r.Field("actorRole").Enum("OWNER", "OPERATOR"), Evidence: r.Field("evidence").Identifier(), RecordedSeq: r.Field("recordedSeq").Size(), RecordedAt: r.Field("recordedAt").Timestamp()}
}
func (a *Attempt) checkLaneUntouched() error {
	fail := func() error {
		return wire.Errorf(wire.CodeMalformed, "laneUntouched", "origin or terminal attestation binding differs")
	}
	origin := a.DirectPoolAdmission
	if origin == nil {
		if a.LaneUntouchedAttestation != nil {
			return fail()
		}
		return nil
	}
	// The witness describes admission, not every later lifecycle state. Keep it
	// readable when ordinary supervised work changes holder/stage/allocation.
	if origin.Allocation == nil || origin.AttemptID != a.AttemptID || origin.Generation != a.Generation || origin.OriginalAdmissionSeq.Uint64() == 0 || origin.OriginalAdmissionSeq != origin.Allocation.AllocatedSeq || origin.OriginalAdmissionSeq.Uint64() > a.PhaseSinceSeq.Uint64() {
		return fail()
	}
	x := a.LaneUntouchedAttestation
	if x == nil {
		return nil
	}
	if a.PoolAllocation == nil || a.Lease == nil || x.Allocation == nil || a.RuntimeID != RuntimeExternalAgent {
		return fail()
	}
	if origin.Holder != a.Lease.Holder || origin.Stage != a.Stage || !bytes.Equal(wire.Encode(PoolAllocationValue(origin.Allocation)), wire.Encode(PoolAllocationValue(a.PoolAllocation))) || !bytes.Equal(wire.Encode(DirectPoolAdmissionValue(origin)), wire.Encode(DirectPoolAdmissionValue(&x.DirectPoolAdmission))) {
		return fail()
	}
	if a.Phase != "CANCELLED" || a.Quiescence != "FENCED" || x.RecordedSeq != a.PhaseSinceSeq || x.RecordedSeq.Uint64() <= origin.OriginalAdmissionSeq.Uint64() || a.Lease.GrantedSeq != origin.OriginalAdmissionSeq || x.RecordedAt >= a.Lease.ExpiresAt || a.ConfigSha256 != a.PolicySha256 || a.CapabilityProfileSha256 != a.PolicySha256 {
		return fail()
	}
	if a.CandidateTreeOid != nil || a.ManifestSha256 != nil || len(a.GateResults)+len(a.Reviews)+len(a.PendingEffects) != 0 || a.Supervisor != nil || a.Supervision != nil || a.Lane != nil || a.WorktreePath != nil || a.NoExec != nil || a.SpawnNoExecCount.Int() != 0 || a.ScopeCheck != "UNKNOWN" || a.RetryAccounting == nil || a.RetryAccounting.FailedOrUnknown || a.RetryCount.Int() != 0 || a.RepairRound.Int() != 0 || len(a.PriorGenerations) != 0 {
		return fail()
	}
	return nil
}
