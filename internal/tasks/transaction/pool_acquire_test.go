package transaction

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-202: an attempt that returned its allocation early cannot attach to
// a supervisor, whose generation records no member history.
func TestCALV0202_ReturnedAllocationAttachRefused(t *testing.T) {
	c, a, _ := untouchedContext(t)
	a.ReleasedPoolAllocation = &snapshot.ReleasedPoolAllocation{Allocation: *a.PoolAllocation, ReleasedSeq: wire.SizeOf(a.PoolAllocation.AllocatedSeq.Uint64() + 1)}
	a.PoolAllocation, a.DirectPoolAdmission = nil, nil
	c.st.attempts[a.AttemptID] = a
	before, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(SupervisorChange{Action: "ATTACH", ProgramID: "program", OwnerPID: 1, OwnerStarted: "start", Expected: wire.Sum(before)})
	if err != nil {
		t.Fatal(err)
	}
	p := snapshot.Programs{Profile: "taskman-programs/0", QueueID: c.r.QueueID, Entries: []snapshot.Program{{ID: "program", Profile: snapshot.SupervisedProfile, OwnerPID: 1, OwnerStarted: "start", Epoch: 1, ConfigSHA256: string(a.ConfigSha256), Phase: "ADMITTED", CurrentAttempt: a.AttemptID, CurrentGeneration: string(a.Generation)}}}
	if c.in.Programs, err = p.Encode(); err != nil {
		t.Fatal(err)
	}
	c.in.LeaseFacts.Program = raw
	c.l = &LeaseRequest{Verb: LeaseSupervisor, AttemptID: a.AttemptID, Generation: a.Generation, Evidence: string(wire.Sum(raw))}
	c.r.Lease = c.l
	out := planSupervisor(c)
	if out.result == nil || out.result.Outcome.Outcome != mutation.OutcomeUnsupported || !strings.Contains(out.result.Detail, "returned its allocation early") || len(out.posts) != 0 {
		t.Fatalf("attach after early return %+v", out.result)
	}
}
