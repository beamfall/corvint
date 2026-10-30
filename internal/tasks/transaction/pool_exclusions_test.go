package transaction

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-065: this literal was captured from the pre-extension public source,
// independently of the candidate encoder; absence must retain exact bytes.
func TestCALV0065_AbsentPreimage(t *testing.T) {
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	l := LeaseRequest{Verb: LeaseClaim, TicketID: "ticket:acme:main:AT-001", Holder: "builder", LeaseMinutes: "60"}
	want := `{"attemptId":null,"base":null,"branch":null,"generation":null,"holder":"builder","leaseMinutes":"60","reason":null,"scope":null,"ticketId":"ticket:acme:main:AT-001","verb":"CLAIM","wholeRepository":false}`
	v, err := leaseValue(&l, q)
	if err != nil || string(wire.Encode(v)) != want {
		t.Fatalf("historical bytes changed: %s %v", wire.Encode(v), err)
	}
	if wire.Sum(wire.Encode(v)) != "cc6fd09baf3cc118c7609a8528fe2c10f78532203b2052c476384b94c40ee0d8" {
		t.Fatal("historical digest changed")
	}
	l.Pool = "db"
	l.ExcludeMembers = []string{"a"}
	a, err := leaseValue(&l, q)
	if err != nil {
		t.Fatal(err)
	}
	l.ExcludeMembers = []string{"b"}
	b, err := leaseValue(&l, q)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(wire.Encode(a), wire.Encode(b)) {
		t.Fatal("exclusions do not bind request digest")
	}
}

// CAL-V0-065: syntax is independent of current membership, but closed fields,
// canonical sets and the absolute bound remain binding before replay.
func TestCALV0065_RequestShapeAndCurrentMembership(t *testing.T) {
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	base := LeaseRequest{Verb: LeaseClaim, TicketID: "ticket:acme:main:AT-001", Holder: "builder", LeaseMinutes: "60", Pool: "db"}
	for _, set := range [][]string{{}, {""}, {"a", "a"}, {"b", "a"}, {"bad\nlabel"}, make([]string, 257)} {
		l := base
		l.ExcludeMembers = set
		if _, err := leaseValue(&l, q); err == nil {
			t.Fatalf("invalid set accepted: %v", set)
		}
	}
	l := base
	l.Pool = ""
	l.ExcludeMembers = []string{"a"}
	if _, err := leaseValue(&l, q); err == nil {
		t.Fatal("exclusions without pool accepted")
	}
	l = LeaseRequest{Verb: LeaseRenew, AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Generation: "1", LeaseMinutes: "60", Pool: "db", ExcludeMembers: []string{"a"}}
	if _, err := leaseValue(&l, q); err == nil {
		t.Fatal("non-claim shape accepted")
	}
	l = base
	l.ExcludeMembers = []string{"removed"}
	if _, err := leaseValue(&l, q); err != nil {
		t.Fatalf("current membership used before replay: %v", err)
	}
	policy := &intent.Policy{Pools: []intent.Pool{{ID: "db", Members: []string{"a", "b"}}, {ID: "other", Members: []string{"removed"}}}}
	if err := CheckPoolExclusions(l.Pool, l.ExcludeMembers, policy); err == nil {
		t.Fatal("foreign member accepted fresh")
	}
	l.ExcludeMembers = []string{"a", "b"}
	if err := CheckPoolExclusions(l.Pool, l.ExcludeMembers, policy); err != nil {
		t.Fatal(err)
	}
	all := make([]string, 256)
	for i := range all {
		all[i] = fmt.Sprintf("m%03d", i)
	}
	if err := checkExcludedMembers("db", all); err != nil {
		t.Fatal(err)
	}
}

// CAL-V0-065: exclusions preserve reserved-first ordering, reject exhausted
// capacity, and cannot be bypassed by a valid bound passing health observation.
func TestCALV0065_AllocationPreviewAndPreparedAdmission(t *testing.T) {
	q, _ := intent.DecodeQueue(fixture.QueueBytes())
	policy := &intent.Policy{Pools: []intent.Pool{{ID: "db", Members: []string{"open", "review-a", "review-b"}, ReservedFor: map[string]string{"review-a": "review", "review-b": "review"}}}}
	pool := &policy.Pools[0]
	if got := OrderedPoolMembers(pool, "review", []string{"review-a"}); !slices.Equal(got, []string{"review-b", "open"}) {
		t.Fatalf("order: %v", got)
	}
	if got := OrderedPoolMembers(pool, "", []string{"open"}); len(got) != 0 {
		t.Fatalf("stage bypass: %v", got)
	}
	l := &LeaseRequest{Verb: LeaseClaim, TicketID: "ticket:acme:main:AT-001", Holder: "builder", LeaseMinutes: "60", Pool: "db", Stage: "review", ExcludeMembers: []string{"review-a"}}
	c := leaseContext{r: Request{QueueID: fixture.QueueID, RequestID: "claim", Lease: l}, l: l, st: inputState{queue: q, policy: policy, pools: &snapshot.PoolState{}}, seq: "1"}
	a, err := c.allocate(nil)
	if err != nil || a.MemberID != "review-b" {
		t.Fatalf("allocate %+v %v", a, err)
	}
	in := PlanInput{Policy: policy, Pool: l.Pool, Stage: l.Stage, ExcludeMembers: l.ExcludeMembers, Pools: c.st.pools}
	if poolSlots(in) != 2 {
		t.Fatal("preview did not filter same members")
	}
	l.ExcludeMembers = []string{"open", "review-a", "review-b"}
	in.ExcludeMembers = l.ExcludeMembers
	if a, err = c.allocate(nil); a != nil || err == nil || !strings.Contains(err.Error(), wire.CodeResourceCollision) {
		t.Fatalf("all excluded: %+v %v", a, err)
	}
	if poolSlots(in) != 0 {
		t.Fatal("preview admitted excluded capacity")
	}
	l.ExcludeMembers = []string{"review-a"}
	id := wire.Sum([]byte("prepared"))
	rev := strings.Repeat("a", 40)
	definition := wire.Sum([]byte("definition"))
	en := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: "db", MemberID: "review-a", AllocationID: id, DefinitionSha256: definition}, State: "PREPARING", Holder: l.Holder, Stage: l.Stage, RequestSha256: PoolClaimBinding(l, q.QueueID), CommandKind: "health", CommandRevision: rev}
	obs := snapshot.PoolObservation{AllocationID: id, DefinitionSha256: definition, Kind: "health", Revision: rev, Tree: rev, Class: "EXIT_ZERO", Passed: true, GroupClean: true, OutputSha256: wire.Sum(nil), EnvironmentSha256: wire.Sum(nil)}
	c.st.pools.Entries = []snapshot.PoolEntry{en}
	c.in.LeaseFacts.Pool = PoolFacts{AllocationID: id, Observation: obs.Encode()}
	if _, err = c.observation(&en); err != nil {
		t.Fatalf("control observation invalid: %v", err)
	}
	if a, err = c.allocate(nil); a != nil || err == nil || !strings.Contains(err.Error(), wire.CodeResourceCollision) {
		t.Fatalf("prepared exclusion bypass %+v %v", a, err)
	}
	// An otherwise identical bound observation admits an eligible member.
	en.MemberID = "review-b"
	c.st.pools.Entries[0] = en
	if a, err = c.allocate(nil); err != nil || a.MemberID != "review-b" {
		t.Fatalf("eligible prepared %+v %v", a, err)
	}
	pool.ReservedFor["review-b"] = "integrate"
	if _, err = c.allocate(nil); err == nil {
		t.Fatal("prepared stage drift bypass")
	}
}
