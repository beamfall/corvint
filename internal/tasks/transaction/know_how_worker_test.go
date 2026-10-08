package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0022_WorkerAttemptObservation derives the observation a WORKER
// KNOWHOW_ADD is checked against from real attempt records: a lease is live
// strictly before its expiry, a terminal phase is never live, a supervised
// attempt's lease does not expire by time (as for every lease command), an
// absent or unleased attempt observes nothing, and other roles get nil.
func TestKHNV0022_WorkerAttemptObservation(t *testing.T) {
	const now = wire.Timestamp("2026-10-07T12:00:00Z")
	id := "att-1"
	env := &mutation.Envelope{Operation: mutation.OpKnowHowAdd, Payload: &mutation.KnowHowAddPayload{Attempt: &id}}
	worker := Request{Operation: Mutate, Actor: mutation.Binding{ID: "agent", Role: "WORKER"}}
	attempt := func(phase string, expires wire.Timestamp, supervised bool) *snapshot.Attempt {
		a := &snapshot.Attempt{AttemptID: id, TicketID: fixture.Ticket("AT-01").TicketID, Generation: "3", Phase: phase,
			Lease: &snapshot.Lease{Holder: "agent", ExpiresAt: expires}}
		if supervised {
			a.Supervision = &snapshot.Supervision{WorkerHolder: "agent"}
		}
		return a
	}
	for _, c := range []struct {
		name string
		a    *snapshot.Attempt
		live bool
	}{
		{"running before expiry", attempt("RUNNING", "2026-10-07T12:00:01Z", false), true},
		{"expiry at the recorded instant", attempt("RUNNING", now, false), false},
		{"expired", attempt("RUNNING", "2026-10-07T11:00:00Z", false), false},
		{"completed", attempt("COMPLETED", "2026-10-07T13:00:00Z", false), false},
		{"cancelled", attempt("CANCELLED", "2026-10-07T13:00:00Z", false), false},
		{"failed", attempt("FAILED", "2026-10-07T13:00:00Z", false), false},
		{"supervised past its lease time", attempt("RUNNING", "2026-10-07T11:00:00Z", true), true},
	} {
		o := workerKnowHowAttempt(worker, inputState{attempts: map[string]*snapshot.Attempt{id: c.a}}, env, now)
		if o == nil || o.Live != c.live || o.TicketID != c.a.TicketID.Raw || o.Generation != "3" || o.Holder != "agent" {
			t.Fatalf("%s: %+v", c.name, o)
		}
	}
	unleased := attempt("RUNNING", now, false)
	unleased.Lease = nil
	for name, st := range map[string]inputState{"absent": {}, "unleased": {attempts: map[string]*snapshot.Attempt{id: unleased}}} {
		if o := workerKnowHowAttempt(worker, st, env, now); o == nil || o.Live || o.TicketID != "" || o.Holder != "" {
			t.Fatalf("%s: %+v", name, o)
		}
	}
	if o := workerKnowHowAttempt(worker, inputState{}, &mutation.Envelope{Operation: mutation.OpKnowHowAdd, Payload: &mutation.KnowHowAddPayload{}}, now); o == nil || o.Live {
		t.Fatalf("no attempt named: %+v", o)
	}
	owner := worker
	owner.Actor.Role = "OWNER"
	if o := workerKnowHowAttempt(owner, inputState{attempts: map[string]*snapshot.Attempt{id: attempt("RUNNING", "2026-10-07T13:00:00Z", false)}}, env, now); o != nil {
		t.Fatalf("OWNER observation: %+v", o)
	}
}
