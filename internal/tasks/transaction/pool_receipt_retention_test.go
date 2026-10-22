package transaction

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-196 (V1-1750): a pool at the 256-member bound with a receipt for
// every member and operation, at maximum identifier lengths, stays writable.
// An operation write retains the MaxPoolOperationReceipts newest receipts by
// observedSeq, and the full-size pool then still admits a pooled claim and an
// early pool release. Without the bound all 1280 receipts are kept, and a
// 256-entry pools.json at these lengths measured 1,048,666 bytes, over
// MaxPoolStateBytes, so every pool write was refused LIMIT_EXCEEDED.
func TestPSRV0196_FullSizePoolReceiptsStayBounded(t *testing.T) {
	label := func(prefix string, i, n int) string {
		s := fmt.Sprintf("%s%03d", prefix, i)
		return s + strings.Repeat("x", n-len(s))
	}
	members := make([]string, intent.MaxPoolMembers)
	for i := range members {
		members[i] = label("m", i, 64)
	}
	pool := label("p", 0, 64)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", object("maxActiveAttempts", s("16"), "maxWorkersTotal", s("16"), "classes", wire.Array()))
	v.Obj.Set("pools", wire.Array(object("id", s(pool), "members", wire.Strings(members))))
	rec := priorityTicket("AA-02", "P1", pool)
	f := newPriorityFixture(t, wire.EncodeFile(v), []*ticket.Record{rec})
	id, out := wire.Sum([]byte("allocation")), wire.Sum(nil)
	src := &intent.PoolSource{Revision: strings.Repeat("a", 64), Tree: strings.Repeat("b", 64)}
	st := *f.st.pools
	seq := 0
	for i, m := range members {
		for _, op := range intent.PoolOperations {
			seq++
			r := snapshot.PoolOperationReceipt{PoolID: pool, MemberID: m, Operation: op, RequestID: label("r"+op, i, 128), AllocationID: id, Class: "EXIT_ZERO", Passed: true, OutputSha256: &out, ObservedSeq: wire.Size(fmt.Sprint(seq))}
			if op == "retarget" {
				r.Source = src
			}
			st.Operations = append(st.Operations, r)
		}
	}
	base := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: pool, AllocationID: id, DefinitionSha256: wire.Sum([]byte("definition")), AllocatedSeq: "1"}, Generation: "0", PolicySha256: wire.Sum([]byte("policy")), RequestSha256: wire.Sum([]byte("request")), RunnerPID: "0", ChangedSeq: "2"}
	cleaning := base
	cleaning.MemberID, cleaning.State, cleaning.Holder, cleaning.RunnerPID, cleaning.RunnerStarted = members[0], "CLEANING", "tester", "42", "started"
	cleaning.CommandKind, cleaning.CommandRevision = "reset", strings.Repeat("a", 40)
	cleaning.Operation = &snapshot.PoolOperationPending{Name: "reset", RequestID: label("reset", 0, 128)}
	st.Entries = []snapshot.PoolEntry{cleaning}
	for i, m := range members[2:] {
		en := base
		en.MemberID, en.State, en.Holder, en.ObservationSha256 = m, "QUARANTINED", label("h", i, 64), &id
		en.Reason = ReasonIdentityUnproved + ": retarget EXIT_ZERO; confirm-safe identity observation required"
		st.Entries = append(st.Entries, en)
	}
	f.st.pools = &st
	f.seq = seq + 1

	l := &LeaseRequest{Verb: LeasePoolRecover, Member: members[0], Allocation: string(id), Reason: "runner died"}
	c := f.waitRequest(l)
	c.in.LeaseFacts.Pool = PoolFacts{RunnerGone: true}
	recovered := planPoolRecover(c)
	if recovered.result != nil {
		t.Fatalf("recovery of a full-size pool refused: %+v", *recovered.result)
	}
	if n := len(recovered.posts["pools.json"]); n > snapshot.MaxPoolStateBytes {
		t.Fatalf("pools.json is %d bytes", n)
	}
	f.apply(t, recovered)
	ops := f.st.pools
	if len(ops.Operations) != snapshot.MaxPoolOperationReceipts {
		t.Fatalf("retained %d receipts, want %d", len(ops.Operations), snapshot.MaxPoolOperationReceipts)
	}
	if r := ops.PoolOperationReceiptFor(members[0], "reset"); r == nil || r.Class != snapshot.PoolOperationOrphaned || r.ObservedSeq != wire.Size(fmt.Sprint(seq+1)) {
		t.Fatalf("new receipt %+v", r)
	}
	if ops.PoolOperationReceiptFor(members[255], "teardown") == nil || ops.PoolOperationReceiptFor(members[1], "provision") != nil {
		t.Fatal("retention did not keep the newest receipts by observedSeq")
	}

	a := f.admitted(t, f.claim(rec.TicketID.Raw, pool))
	if a.PoolAllocation == nil || a.PoolAllocation.MemberID != members[1] || len(f.st.pools.Entries) != intent.MaxPoolMembers {
		t.Fatalf("acquire of the last free member: %+v entries=%d", a.PoolAllocation, len(f.st.pools.Entries))
	}
	early := &LeaseRequest{Verb: LeasePoolRelease, AttemptID: a.AttemptID, Generation: a.Generation, Allocation: string(a.PoolAllocation.AllocationID)}
	if released := viaPlanLease(f.waitRequest(early)); released.result != nil {
		t.Fatalf("release on a full-size pool refused: %+v", *released.result)
	}
}
