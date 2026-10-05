package transaction

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
	"strings"
	"testing"
)

// CAL-V0-029.
func TestPoolAllocationTupleCorrespondence(t *testing.T) {
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
	en := snapshot.PoolEntry{PoolAllocation: allocation, State: "ALLOCATED", Holder: "builder", Stage: "implement", AttemptID: "attempt:acme:main:0123456789abcdef0123456789abcdef", Generation: "1", ChangedSeq: "1", PolicySha256: policy.PolicySha256(), RequestSha256: wire.Sum(nil)}
	raw, e := (&snapshot.PoolState{QueueID: q.QueueID, Entries: []snapshot.PoolEntry{en}}).Encode()
	if e != nil {
		t.Fatal(e)
	}
	inv, e := NewInventory([]archive.FileEntry{{Path: "pools.json", Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"member", "pool", "definition", "sequence", "config", "holder", "stage"} {
		t.Run(field, func(t *testing.T) {
			a := &snapshot.Attempt{AttemptID: en.AttemptID, Generation: "1", Phase: "RUNNING", Stage: "implement", Lease: &snapshot.Lease{Holder: "builder"}}
			copy := allocation
			a.PoolAllocation = &copy
			switch field {
			case "member":
				copy.MemberID = "b"
			case "pool":
				copy.PoolID = "else"
			case "definition":
				copy.DefinitionSha256 = wire.Sum([]byte("changed"))
			case "sequence":
				copy.AllocatedSeq = "2"
			case "config":
				copy.ConfigRef = &intent.ConfigRef{Revision: "0123456789012345678901234567890123456789", Path: "config", Blob: "0123456789012345678901234567890123456789"}
			case "holder":
				a.Lease.Holder = "other"
			case "stage":
				a.Stage = "review"
			}
			_, e := loadPools(Request{Operation: Lease, QueueID: q.QueueID.Raw}, Input{Inventory: inv, Pools: raw}, inputState{queue: q, policy: policy, attempts: map[string]*snapshot.Attempt{a.AttemptID: a}})
			if e == nil {
				t.Fatal("mismatched tuple admitted")
			}
		})
	}
}

// CAL-V0-034.
func TestPoolPlanConsumesEligibleSlots(t *testing.T) {
	policy := &intent.Policy{MaxActiveAttempts: "4", Pools: []intent.Pool{{ID: "db", Members: []string{"integrate", "open", "review"}, ReservedFor: map[string]string{"integrate": "integrate", "review": "review"}}}}
	in := PlanInput{Policy: policy, Pool: "db", Stage: "review", Reservations: &snapshot.ReservationSet{}}
	first := choose(in, PlanEntry{}, nil, nil)
	if first.State != PlanSelected {
		t.Fatalf("first %+v", first)
	}
	// Disjoint empty scopes isolate the pool cardinality from ordinary reservations.
	second := choose(in, PlanEntry{}, []PlanEntry{first}, nil)
	t.Run("CAL-V0-034 preview capacity uses claim eligibility", func(t *testing.T) {
		if second.State != PlanSelected {
			t.Fatalf("fallback slot %+v", second)
		}
		third := choose(in, PlanEntry{}, []PlanEntry{first, second}, nil)
		if third.State != PlanDeferred || third.Reason != wire.CodeResourceCollision {
			t.Fatalf("overallocated %+v", third)
		}
		in.Stage = ""
		if noStage := choose(in, PlanEntry{}, []PlanEntry{first}, nil); noStage.State != PlanDeferred || noStage.Reason != wire.CodeResourceCollision {
			t.Fatalf("reserved member admitted without stage %+v", noStage)
		}
	})
}

// CAL-V0-029 and CAL-V0-034.
func TestOrderedPoolMembersPrefersMatchingReservation(t *testing.T) {
	pool := &intent.Pool{Members: []string{"integrate", "open-a", "open-b", "review-a", "review-b"}, ReservedFor: map[string]string{"integrate": "integrate", "review-a": "review", "review-b": "review"}}
	for _, tc := range []struct {
		stage string
		want  string
	}{
		{"review", "review-a,review-b,open-a,open-b"},
		{"implement", "open-a,open-b"},
		{"", "open-a,open-b"},
	} {
		if got := strings.Join(OrderedPoolMembers(pool, tc.stage), ","); got != tc.want {
			t.Fatalf("stage %q: got %q want %q", tc.stage, got, tc.want)
		}
	}
}

// CAL-V0-028.
func TestPoolLargeProjectionCompletionStage(t *testing.T) {
	d := maximalDescriptor(Lease)
	d.QueueID = "queue:a:" + strings.Repeat("q", 32)
	add := func(role, path string, n int) {
		digest := wire.Sum([]byte(path))
		if role == "EVIDENCE" {
			path = "evidence/" + string(digest)
		}
		d.Artifacts = append(d.Artifacts, Description{Role: role, Target: path, Bytes: wire.SizeOf(uint64(n)), Sha256: digest})
	}
	add("POST", "attempts/attempt:a:"+strings.Repeat("q", 32)+":0123456789abcdef0123456789abcdef.json", wire.MaxAttemptRecordBytes)
	add("POST", "reservations.json", wire.MaxReservationSetBytes)
	add("EVIDENCE", "reservations.json", wire.MaxReservationSetBytes)
	add("POST", "intent/tickets/"+strings.Repeat("t", 64)+".json", wire.MaxTicketFileBytes)
	add("EVIDENCE", "intent/tickets/"+strings.Repeat("t", 64)+".json", wire.MaxTicketFileBytes)
	add("POST", "pools.json", snapshot.MaxPoolStateBytes)
	add("EVIDENCE", "pools.json", snapshot.MaxPoolStateBytes)
	digest := wire.Sum([]byte("manifest"))
	d.Artifacts = append(d.Artifacts, Description{Role: "POST", Target: "evidence/" + string(digest), Bytes: "1024", Sha256: digest})
	sort.Slice(d.Artifacts, func(i, j int) bool {
		a, b := d.Artifacts[i], d.Artifacts[j]
		if a.Role != b.Role {
			return a.Role < b.Role
		}
		return a.Target < b.Target
	})
	for i := range d.Artifacts {
		d.Artifacts[i].Slot = fmt.Sprintf("a%02d", i)
	}
	raw := wire.EncodeFile(d.Value())
	if len(d.Artifacts) != 11 {
		t.Fatal(len(d.Artifacts))
	}
	if _, e := snapshot.DecodeStageDescriptor(raw); e != nil {
		t.Fatalf("large completion descriptor %d bytes: %v", len(raw), e)
	}
}
