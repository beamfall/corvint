package transaction

import (
	"encoding/json"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0039_SupervisorAttachRefusedWithSlots: an attempt with a live slot
// entry, held or waiting, cannot attach to a supervisor, whose attempts are
// never expired, reaped or slot-released; an entry of an older generation is
// dead and does not refuse.
func TestPSRV0039_SupervisorAttachRefusedWithSlots(t *testing.T) {
	for _, state := range []string{"none", snapshot.SlotHeld, snapshot.SlotWaiting, "dead"} {
		t.Run(state, func(t *testing.T) {
			c, a, _ := untouchedContext(t)
			c.st.policy.Slots = []intent.SlotResource{{ID: "gpu", Capacity: "2", Reserved: map[string]wire.Count{}}}
			if state != "none" {
				en := snapshot.SlotEntry{SlotsID: "gpu", AttemptID: a.AttemptID, Generation: a.Generation, Holder: "builder", Count: "1", Priority: "P2", Seq: "2", State: state}
				if state == "dead" {
					en.State, en.Generation = snapshot.SlotHeld, wire.SizeOf(a.Generation.Uint64()+1)
				}
				c.st.pools.Slots = []snapshot.SlotEntry{en}
			}
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
			refused := out.result != nil && out.result.Outcome.Outcome == mutation.OutcomeUnsupported && out.result.Outcome.HasCode(wire.CodeUnsupported) && len(out.posts) == 0
			if want := state == snapshot.SlotHeld || state == snapshot.SlotWaiting; refused != want {
				t.Fatalf("%s: refused %v, result %+v", state, refused, out.result)
			}
			if !refused && out.result != nil {
				t.Fatalf("%s: attach %+v", state, out.result)
			}
		})
	}
}
