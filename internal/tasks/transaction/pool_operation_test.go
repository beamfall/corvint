package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-165: recovering an operation whose runner is gone records an
// ORPHANED receipt without an output digest and quarantines the member with
// the operation named; the receipt replaces only that member's receipt for the
// operation and drops receipts of members the policy no longer declares.
func TestPSRV0165_RecoverOrphanedOperation(t *testing.T) {
	policy := &intent.Policy{Pools: []intent.Pool{{ID: "db", Members: []string{"a", "b"}, MemberConfig: map[string]intent.MemberConfig{"a": {Operations: map[string]*intent.PoolCommand{"reset": {}}}}}}}
	id := wire.Sum([]byte("allocation"))
	out := wire.Sum(nil)
	en := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: id, DefinitionSha256: wire.Sum([]byte("definition")), AllocatedSeq: "1"}, State: "CLEANING", Holder: "tester", Generation: "0", PolicySha256: wire.Sum([]byte("policy")), RequestSha256: wire.Sum([]byte("request")), RunnerPID: "42", RunnerStarted: "started", CommandKind: "reset", CommandRevision: strings.Repeat("a", 40), ChangedSeq: "2", Operation: &snapshot.PoolOperationPending{Name: "reset", RequestID: "reset-1"}}
	old := []snapshot.PoolOperationReceipt{
		{PoolID: "db", MemberID: "a", Operation: "reset", RequestID: "reset-0", AllocationID: id, Class: "EXIT_ZERO", Passed: true, OutputSha256: &out, ObservedSeq: "1"},
		{PoolID: "db", MemberID: "b", Operation: "provision", RequestID: "p-b", AllocationID: id, Class: "EXIT_ZERO", Passed: true, OutputSha256: &out, ObservedSeq: "1"},
		{PoolID: "db", MemberID: "gone", Operation: "provision", RequestID: "p-gone", AllocationID: id, Class: "EXIT_ZERO", Passed: true, OutputSha256: &out, ObservedSeq: "1"},
	}
	q, e := wire.ParseQueueID("queue", fixture.QueueID)
	if e != nil {
		t.Fatal(e)
	}
	l := &LeaseRequest{Verb: LeasePoolRecover, Member: "a", Allocation: string(id), Reason: "runner died"}
	c := leaseContext{r: Request{QueueID: fixture.QueueID, RequestID: "recover", Lease: l}, l: l, seq: "3", st: inputState{policy: policy, pools: &snapshot.PoolState{QueueID: q, Entries: []snapshot.PoolEntry{en}, Operations: old}}}
	c.in.LeaseFacts.Pool = PoolFacts{RunnerGone: true}
	o := planPoolRecover(c)
	if o.result != nil {
		t.Fatalf("recover refused: %+v", o.result)
	}
	state, e := snapshot.DecodePools(o.posts["pools.json"])
	if e != nil {
		t.Fatal(e)
	}
	got := state.Entries[0]
	if got.State != "QUARANTINED" || !strings.HasPrefix(got.Reason, "orphaned reset command") || got.Operation != nil || got.RunnerPID != "0" {
		t.Fatalf("entry %+v", got)
	}
	r := state.PoolOperationReceiptFor("a", "reset")
	if len(state.Operations) != 2 || r == nil || r.RequestID != "reset-1" || r.Class != snapshot.PoolOperationOrphaned || r.Passed || r.OutputSha256 != nil || r.ObservedSeq != "3" || state.PoolOperationReceiptFor("b", "provision") == nil {
		t.Fatalf("receipts %+v", state.Operations)
	}
	// A live or unknown runner is never recovered.
	c.in.LeaseFacts.Pool.RunnerGone = false
	if o = planPoolRecover(c); o.result == nil || !strings.Contains(o.result.Detail, "runner is live") {
		t.Fatalf("live runner recovered: %+v", o)
	}
}
