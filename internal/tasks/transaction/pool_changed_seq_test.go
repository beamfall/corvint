package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0895 (PSR-V0-013): a PREPARING member that a live attempt takes to
// ALLOCATED records that receipt's sequence as its changedSeq, so pool status
// reports when the member was allocated, not when preparation began.
func TestPSRV0013_PreparedAllocationAdvancesChangedSeq(t *testing.T) {
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	id := wire.Sum([]byte("prepared"))
	rev := strings.Repeat("a", 40)
	definition := wire.Sum([]byte("definition"))
	alloc := snapshot.PoolAllocation{PoolID: "db", MemberID: "m1", AllocationID: id, DefinitionSha256: definition, AllocatedSeq: "3"}
	en := snapshot.PoolEntry{PoolAllocation: alloc, State: "PREPARING", Holder: "builder", Stage: "implement", ChangedSeq: "3", PolicySha256: wire.Sum([]byte("policy")), RequestSha256: wire.Sum([]byte("claim")), CommandKind: "health", CommandRevision: rev}
	obs := snapshot.PoolObservation{AllocationID: id, DefinitionSha256: definition, Kind: "health", Revision: rev, Tree: rev, Class: "EXIT_ZERO", Passed: true, GroupClean: true, OutputSha256: wire.Sum(nil), EnvironmentSha256: wire.Sum(nil)}
	c := leaseContext{st: inputState{policy: &intent.Policy{}, pools: &snapshot.PoolState{QueueID: q, Entries: []snapshot.PoolEntry{en}}}, seq: "7"}
	c.in.LeaseFacts.Pool = PoolFacts{AllocationID: id, Observation: obs.Encode()}
	a := &snapshot.Attempt{AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Generation: "1", Phase: "RUNNING", PoolAllocation: &alloc}
	posts := map[string][]byte{}
	if err := c.poolPosts(a, posts); err != nil {
		t.Fatal(err)
	}
	state, err := snapshot.DecodePools(posts["pools.json"])
	if err != nil {
		t.Fatal(err)
	}
	got := state.Entries[0]
	if got.State != "ALLOCATED" || got.ChangedSeq != "7" {
		t.Fatalf("member %s changedSeq %s, want ALLOCATED at 7", got.State, got.ChangedSeq)
	}
	if c.st.pools.Entries[0].ChangedSeq != "3" {
		t.Fatal("the input pool state was modified")
	}
}
