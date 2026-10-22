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

// longestRecoverReason is the longest `pool recover` reason the lease codec
// admits, found from the codec itself rather than a constant.
func longestRecoverReason(t *testing.T, q wire.QueueID, member, allocation string) string {
	t.Helper()
	admits := func(n int) bool {
		l := &LeaseRequest{Verb: LeasePoolRecover, Member: member, Allocation: allocation, Reason: strings.Repeat("r", n)}
		_, err := leaseValue(l, q)
		return err == nil
	}
	lo, hi := 1, 64*1024
	if !admits(lo) || admits(hi) {
		t.Fatalf("recover reason bound outside 1..%d", hi)
	}
	for hi-lo > 1 {
		if mid := (lo + hi) / 2; admits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return strings.Repeat("r", lo)
}

// PSR-V0-202 (V1-2361): every other member of a full-size pool is QUARANTINED
// carrying the longest entry reason a recover can write, the pool holds a
// receipt for every member and operation at maximum identifier lengths, and
// one orphaned operation is recovered with the longest admitted reason. The
// recovery, then a pooled claim of the last free member and its early
// release, all still write a pools.json within MaxPoolStateBytes. Without
// the reason bound a 4,096-byte recover reason made each entry over 4 KiB,
// so this state is past 1 MiB and every pool write is refused.
func TestPSRV0202_FullSizePoolLongestReasonsStayBounded(t *testing.T) {
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
	reason := longestRecoverReason(t, f.st.queue.QueueID, members[0], string(id))
	// The longest prefix a recover writes: an orphaned operation with the
	// longest operation name.
	longest := ""
	for _, op := range intent.PoolOperations {
		if len(op) > len(longest) {
			longest = op
		}
	}
	entryReason := "orphaned " + longest + " command; execution and physical cleanup unknown: " + reason
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
	cleaning.MemberID, cleaning.State, cleaning.Holder, cleaning.RunnerPID, cleaning.RunnerStarted = members[0], "CLEANING", "tester", "42", strings.Repeat("s", 128)
	cleaning.CommandKind, cleaning.CommandRevision = longest, strings.Repeat("a", 40)
	cleaning.Operation = &snapshot.PoolOperationPending{Name: longest, RequestID: label(longest, 0, 128)}
	st.Entries = []snapshot.PoolEntry{cleaning}
	for i, m := range members[2:] {
		en := base
		en.MemberID, en.State, en.Holder, en.ObservationSha256, en.Source = m, "QUARANTINED", label("h", i, 64), &id, src
		en.Reason = entryReason
		st.Entries = append(st.Entries, en)
	}
	f.st.pools = &st
	f.seq = seq + 1

	l := &LeaseRequest{Verb: LeasePoolRecover, Member: members[0], Allocation: string(id), Reason: reason}
	c := f.waitRequest(l)
	c.in.LeaseFacts.Pool = PoolFacts{RunnerGone: true}
	recovered := planPoolRecover(c)
	if recovered.result != nil {
		t.Fatalf("recovery with a %d-byte reason on a full-size pool refused: %+v", len(reason), *recovered.result)
	}
	raw := recovered.posts["pools.json"]
	if len(raw) > snapshot.MaxPoolStateBytes {
		t.Fatalf("pools.json is %d bytes", len(raw))
	}
	t.Logf("recover reason %d B, entry reason %d B, pools.json %d B of %d", len(reason), len(entryReason), len(raw), snapshot.MaxPoolStateBytes)
	f.apply(t, recovered)
	if got := f.st.pools.Entries; len(got) != intent.MaxPoolMembers-1 {
		t.Fatalf("entries after recovery = %d", len(got))
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

// PSR-V0-202: a recover reason past the bound is refused LIMIT_EXCEEDED by
// the lease codec, before any state is planned or written.
func TestPSRV0202_OverlongRecoverReasonRefusedTyped(t *testing.T) {
	queue, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatal(err)
	}
	q := queue.QueueID
	l := &LeaseRequest{Verb: LeasePoolRecover, Member: "m1", Allocation: string(wire.Sum([]byte("a"))), Reason: strings.Repeat("r", MaxPoolRecoverReasonBytes+1)}
	if _, err := leaseValue(l, q); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("overlong recover reason: %v, want %s", err, wire.CodeLimitExceeded)
	}
	l.Reason = strings.Repeat("r", MaxPoolRecoverReasonBytes)
	if _, err := leaseValue(l, q); err != nil {
		t.Fatalf("recover reason at the bound refused: %v", err)
	}
}
