package transaction

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// recoverPrefix is the longest fixed text a recover writes ahead of the
// operator's reason: an orphaned operation with the longest operation name.
func recoverPrefix() string {
	longest := ""
	for _, op := range intent.PoolOperations {
		if len(op) > len(longest) {
			longest = op
		}
	}
	return "orphaned " + longest + " command; execution and physical cleanup unknown: "
}

// worstCasePools is a full-size pool state at the largest encoding the
// PSR-V0-202 derivation covers: 256 entries, each with a maximum-length
// configRef, a source pair, every digest and 19-digit sequences, entry 0 a
// CLEANING member running a retarget with its source; 256 REPORTED identities
// with a source; 512 operation receipts (a retarget with a source and a
// provision per member, the longest failing class). Identifiers are at
// their maximum length. Every QUARANTINED entry carries reason(member), and
// every entry a PSR-V0-261 quarantinedAt and a PSR-V0-262 recovery record.
func worstCasePools(t *testing.T, reason func(int) string) (*priorityFixture, []string, wire.Digest) {
	t.Helper()
	label := func(prefix string, i, n int) string {
		s := fmt.Sprintf("%s%03d", prefix, i)
		return s + strings.Repeat("x", n-len(s))
	}
	members := make([]string, intent.MaxPoolMembers)
	for i := range members {
		members[i] = label("m", i, wire.MaxLabelBytes)
	}
	pool := label("p", 0, wire.MaxLabelBytes)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", object("maxActiveAttempts", s("16"), "maxWorkersTotal", s("16"), "classes", wire.Array()))
	v.Obj.Set("pools", wire.Array(object("id", s(pool), "members", wire.Strings(members))))
	f := newPriorityFixture(t, wire.EncodeFile(v), nil)
	oid := strings.Repeat("a", 64)
	src := &intent.PoolSource{Revision: oid, Tree: strings.Repeat("b", 64)}
	id, out := wire.Sum([]byte("allocation")), wire.Sum(nil)
	const top = 1<<63 - 1 // the fixture's int sequence bound: 19 digits
	seqAt := func(n int) wire.Size { return wire.Size(fmt.Sprint(top - 4096 + n)) }
	longest := strings.TrimSuffix(strings.TrimPrefix(recoverPrefix(), "orphaned "), " command; execution and physical cleanup unknown: ")
	st := *f.st.pools
	n := 0
	for i, m := range members {
		for _, op := range []string{"provision", "retarget"} {
			n++
			r := snapshot.PoolOperationReceipt{PoolID: pool, MemberID: m, Operation: op, RequestID: label("r"+op, i, wire.MaxIdentifierBytes), AllocationID: id, Class: "SOURCE_CHANGED", OutputSha256: &out, ObservedSeq: seqAt(n)}
			if op == "retarget" {
				r.Source = src
			}
			st.Operations = append(st.Operations, r)
		}
		st.Identities = append(st.Identities, snapshot.PoolIdentity{PoolID: pool, MemberID: m, AllocationID: id, DefinitionSha256: id, Class: snapshot.PoolIdentityReported, Source: src, OutputSha256: out, ObservedAt: "2026-10-22T00:00:00Z", ObservedSeq: seqAt(0)})
		en := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: pool, MemberID: m, AllocationID: id, DefinitionSha256: id, AllocatedSeq: seqAt(0), ConfigRef: &intent.ConfigRef{Revision: oid, Path: label("c", i, wire.MaxIdentifierBytes), Blob: oid}}, State: "QUARANTINED", Holder: label("h", i, wire.MaxLabelBytes), Generation: seqAt(0), ChangedSeq: seqAt(0), PolicySha256: id, RequestSha256: id, ObservationSha256: &id, Source: src, RunnerPID: "0", CommandKind: longest, CommandRevision: oid, Reason: reason(i)}
		// PSR-V0-266: every entry also carries a quarantine time and a
		// recovery record at their widest.
		en.QuarantinedAt = "2026-10-22T00:00:00Z"
		en.Recovery = &snapshot.PoolRecovery{Kind: "cleanup", Result: "FAILED", DetailSha256: id, Consecutive: "2147483647", FirstObservedAt: "2026-10-22T00:00:00Z", ObservedAt: "2026-10-22T00:00:00Z", ObservedSeq: seqAt(0)}
		if i == 0 {
			// A pending retarget is the only one carrying a source pair.
			en.State, en.RunnerPID, en.RunnerStarted, en.Reason, en.CommandKind = "CLEANING", "4194304", label("s", 0, 128), "", "retarget"
			en.Operation = &snapshot.PoolOperationPending{Name: "retarget", RequestID: label("retarget", 0, wire.MaxIdentifierBytes), Source: src}
		}
		st.Entries = append(st.Entries, en)
	}
	f.st.pools = &st
	f.seq = top
	return f, members, id
}

// PSR-V0-202 (V1-2361): the recover reason cap is derived, not chosen. The
// worst-case state with every reason empty encodes to B0 bytes; each of the
// 256 entries can then carry the fixed prefix plus a reason whose JSON
// escaping at most doubles it (prose admits '"', '\\', TAB, LF and CR, each
// escaped to two bytes; every other admitted byte is copied). The cap must
// satisfy B0 + 256*(prefix + 2*cap) <= MaxPoolStateBytes. The state is then
// rebuilt with every QUARANTINED reason at that escaping worst case, and the
// recovery of entry 0 with the longest admitted all-escaping reason still
// writes within the bound. With the 512-byte cap this was refused
// LIMIT_EXCEEDED: the derivation allows less.
func TestPSRV0202_WorstCasePoolsStayBoundedAtReasonCap(t *testing.T) {
	prefix := recoverPrefix()
	f, _, _ := worstCasePools(t, func(int) string { return "" })
	empty, err := f.st.pools.Encode()
	if err != nil {
		t.Fatalf("worst-case state with empty reasons: %v", err)
	}
	derived := (snapshot.MaxPoolStateBytes - len(empty) - intent.MaxPoolMembers*len(prefix)) / (2 * intent.MaxPoolMembers)
	t.Logf("B0 %d B, prefix %d B, derived cap %d B, MaxPoolRecoverReasonBytes %d B", len(empty), len(prefix), derived, MaxPoolRecoverReasonBytes)
	if MaxPoolRecoverReasonBytes > derived {
		t.Fatalf("MaxPoolRecoverReasonBytes %d exceeds the derived worst-case cap %d", MaxPoolRecoverReasonBytes, derived)
	}

	worst := strings.Repeat(`"`, MaxPoolRecoverReasonBytes)
	f, members, id := worstCasePools(t, func(int) string { return prefix + worst })
	before, err := f.st.pools.Encode()
	if err != nil {
		t.Fatalf("worst-case state at the cap: %v", err)
	}
	l := &LeaseRequest{Verb: LeasePoolRecover, Member: members[0], Allocation: string(id), Reason: worst}
	if _, err := leaseValue(l, f.st.queue.QueueID); err != nil {
		t.Fatalf("recover reason at the cap refused by the codec: %v", err)
	}
	c := f.waitRequest(l)
	c.in.LeaseFacts.Pool = PoolFacts{RunnerGone: true}
	recovered := planPoolRecover(c)
	if recovered.result != nil {
		t.Fatalf("recovery of the worst-case pool refused: %+v", *recovered.result)
	}
	raw := recovered.posts["pools.json"]
	t.Logf("pools.json %d B before recovery, %d B after, bound %d", len(before), len(raw), snapshot.MaxPoolStateBytes)
	if len(raw) > snapshot.MaxPoolStateBytes {
		t.Fatalf("pools.json is %d bytes", len(raw))
	}
	f.apply(t, recovered)
	if en := f.st.pools.Entries[0]; en.MemberID != members[0] || en.State != "QUARANTINED" || en.Reason != "orphaned retarget command; execution and physical cleanup unknown: "+worst {
		t.Fatalf("recovered entry %+v", en)
	}
}

// PSR-V0-202: a fresh recover whose reason is past the cap is refused
// LIMIT_EXCEEDED at planning, with nothing posted.
func TestPSRV0202_OverlongFreshRecoverRefusedTyped(t *testing.T) {
	f, members, id := worstCasePools(t, func(int) string { return "" })
	l := &LeaseRequest{Verb: LeasePoolRecover, Member: members[0], Allocation: string(id), Reason: strings.Repeat("r", MaxPoolRecoverReasonBytes+1)}
	c := f.waitRequest(l)
	c.in.LeaseFacts.Pool = PoolFacts{RunnerGone: true}
	got := planPoolRecover(c)
	if got.result == nil || !got.result.Outcome.HasCode(wire.CodeLimitExceeded) || len(got.posts) != 0 {
		t.Fatalf("overlong fresh recover: %+v posts=%d", got.result, len(got.posts))
	}
}

// PSR-V0-202: the cap binds fresh admission only. A recover recorded before
// it, with a reason up to the 4,096-byte codec bound, still digests and
// replays its recorded outcome under its original request ID.
func TestPSRV0202_RecordedLongRecoverReasonReplays(t *testing.T) {
	r := admin(Lease, "recover-1")
	r.Lease = &LeaseRequest{Verb: LeasePoolRecover, Member: "m1", Allocation: string(wire.Sum([]byte("a"))), Reason: strings.Repeat("r", 4096)}
	d, err := Digest(r)
	if err != nil {
		t.Fatalf("a recorded 4,096-byte recover reason no longer digests: %v", err)
	}
	seq := wire.Size("7")
	outcome := mutation.Outcome{RequestID: r.RequestID, Outcome: mutation.OutcomeCompleted, ReceiptSeq: &seq, Codes: []string{}}
	record := wire.EncodeFile(object("requestId", s(r.RequestID), "seq", s(string(seq)), "mutationSha256", s(string(d)), "outcome", outcome.Value()))
	got := Model(r, Input{Replay: ReplayObservation{State: "FOUND", Record: record}})
	if got.Kind != "Replay" || !got.Outcome.Replayed || got.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("replay of a recorded long recover: %+v", got)
	}
}
