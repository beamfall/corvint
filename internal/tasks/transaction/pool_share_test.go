package transaction

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	sharePrimary = "attempt:acme:main:0123456789abcdef0123456789abcdef"
	shareSecond  = "attempt:acme:main:1123456789abcdef0123456789abcdef"
	shareThird   = "attempt:acme:main:2123456789abcdef0123456789abcdef"
)

// PSR-V0-016: every attempt a shared entry binds must be live, at its
// generation and allocation, under the entry's holder and stage, and admitted
// as a share; a live allocated attempt the entry does not bind refuses.
func TestPSRV0016_LoadPoolsValidatesSharedAttempts(t *testing.T) {
	q, e := intent.DecodeQueue(fixture.QueueBytes())
	if e != nil {
		t.Fatal(e)
	}
	v := fixture.PolicyValue()
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a", "b"})))))
	policy, e := intent.DecodePolicy(wire.EncodeFile(v))
	if e != nil {
		t.Fatal(e)
	}
	allocation := snapshot.PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: wire.Sum([]byte("id")), DefinitionSha256: policy.MemberDefinition("db", "a"), AllocatedSeq: "1"}
	en := snapshot.PoolEntry{PoolAllocation: allocation, State: "ALLOCATED", Holder: "builder", Stage: "implement", AttemptID: sharePrimary, Generation: "1", ChangedSeq: "2", PolicySha256: policy.PolicySha256(), RequestSha256: wire.Sum(nil), Shared: []snapshot.PoolShare{{AttemptID: shareSecond, Generation: "1"}}}
	raw, e := (&snapshot.PoolState{QueueID: q.QueueID, Entries: []snapshot.PoolEntry{en}}).Encode()
	if e != nil {
		t.Fatal(e)
	}
	inv, e := NewInventory([]archive.FileEntry{{Path: "pools.json", Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	attempt := func(id string) *snapshot.Attempt {
		copy := allocation
		return &snapshot.Attempt{AttemptID: id, Generation: "1", Phase: "RUNNING", Stage: "implement", Lease: &snapshot.Lease{Holder: "builder"}, PoolAllocation: &copy}
	}
	for _, c := range []string{"ok", "unmarked", "holder", "stage", "terminal", "generation", "missing", "unbound"} {
		t.Run(c, func(t *testing.T) {
			a, b := attempt(sharePrimary), attempt(shareSecond)
			b.SharedAllocation = &snapshot.SharedAllocation{SourceAttemptID: sharePrimary, SourceGeneration: "1", BoundSeq: "2"}
			attempts := map[string]*snapshot.Attempt{a.AttemptID: a, b.AttemptID: b}
			switch c {
			case "unmarked":
				b.SharedAllocation = nil
			case "holder":
				b.Lease.Holder = "other"
			case "stage":
				b.Stage = "review"
			case "terminal":
				b.Phase = "CANCELLED"
			case "generation":
				b.Generation = "2"
			case "missing":
				delete(attempts, b.AttemptID)
			case "unbound":
				x := attempt(shareThird)
				x.SharedAllocation = b.SharedAllocation
				attempts[x.AttemptID] = x
			}
			_, e := loadPools(Request{Operation: Lease, QueueID: q.QueueID.Raw}, Input{Inventory: inv, Pools: raw}, inputState{queue: q, policy: policy, attempts: attempts})
			if (e == nil) != (c == "ok") {
				t.Fatalf("%s: %v", c, e)
			}
		})
	}
}

// PSR-V0-018: binding appends, an ending shared attempt leaves the list, an
// ending primary hands over to the first shared attempt, and an already bound
// live attempt changes nothing.
func TestPSRV0018_SharedPostHandOver(t *testing.T) {
	c := leaseContext{seq: "9"}
	en := snapshot.PoolEntry{State: "ALLOCATED", AttemptID: sharePrimary, Generation: "1", ChangedSeq: "2", Shared: []snapshot.PoolShare{{AttemptID: shareSecond, Generation: "1"}}}
	live := func(id string) *snapshot.Attempt {
		return &snapshot.Attempt{AttemptID: id, Generation: "1", Phase: "RUNNING"}
	}
	ended := func(id string) *snapshot.Attempt {
		return &snapshot.Attempt{AttemptID: id, Generation: "1", Phase: "CANCELLED"}
	}
	c.sharedPost(&en, live(sharePrimary))
	c.sharedPost(&en, live(shareSecond))
	if en.ChangedSeq != "2" {
		t.Fatal("bound live attempt changed the entry")
	}
	c.sharedPost(&en, live(shareThird))
	if want := []snapshot.PoolShare{{AttemptID: shareSecond, Generation: "1"}, {AttemptID: shareThird, Generation: "1"}}; !reflect.DeepEqual(en.Shared, want) || en.ChangedSeq != "9" {
		t.Fatalf("bind %+v", en)
	}
	c.sharedPost(&en, ended(shareSecond))
	if len(en.Shared) != 1 || en.Shared[0].AttemptID != shareThird || en.AttemptID != sharePrimary {
		t.Fatalf("shared end %+v", en)
	}
	c.sharedPost(&en, ended(sharePrimary))
	if en.Shared != nil || en.AttemptID != shareThird || en.State != "ALLOCATED" {
		t.Fatalf("hand-over %+v", en)
	}
}

// PSR-V0-019: neither the source nor a shared attempt may attach to a
// supervisor, whose stage stop would quarantine every bound attempt.
func TestPSRV0019_SharedAttachRefused(t *testing.T) {
	for _, side := range []string{"source", "shared"} {
		t.Run(side, func(t *testing.T) {
			c, a, _ := untouchedContext(t)
			if side == "source" {
				c.st.pools.Entries[0].Shared = []snapshot.PoolShare{{AttemptID: shareSecond, Generation: "1"}}
			} else {
				a.SharedAllocation = &snapshot.SharedAllocation{SourceAttemptID: shareSecond, SourceGeneration: "1", BoundSeq: "2"}
				a.DirectPoolAdmission = nil
				c.st.attempts[a.AttemptID] = a
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
			if out.result == nil || out.result.Outcome.Outcome != mutation.OutcomeUnsupported || !out.result.Outcome.HasCode(wire.CodeUnsupported) || len(out.posts) != 0 {
				t.Fatalf("attach of a shared allocation %+v", out.result)
			}
		})
	}
}
