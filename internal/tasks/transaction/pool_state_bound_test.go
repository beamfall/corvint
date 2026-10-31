package transaction

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// escaping is an n-byte text made only of '"' and '\\', each of which JSON
// escapes to two bytes, distinct for each i below 2^n.
func escaping(i, n int) string {
	b := make([]byte, n)
	for k := range b {
		b[k] = '"'
		if k < 20 && i>>k&1 == 1 {
			b[k] = '\\'
		}
	}
	return string(b)
}

// PSR-V0-900 (V1-2447): every field that contributes to pools.json has a
// bound, and the superset of them all encodes within MaxPoolStateBytes. The
// state starts from the PSR-V0-202/266 worst case and then gives every entry
// every optional field at once, whatever its state allows (sweep owner,
// MaxSharedAttempts-1 shares, a pending operation with a source, a hold, a
// running process, a stage), every Identifier, label and prose field its
// maximum length made of characters JSON escapes, every reason the recover
// prefix plus an all-escaping reason at the cap, and full slots and waiters
// lists. No reachable state is larger, so no pool write is wedged by size;
// a fresh input past any field bound is refused before a write (the
// PSR-V0-202 reason, MaxSlotEntries and MaxPoolWaiters admission, the
// Identifier and label codecs), and an over-bound encoding by Encode's own
// LIMIT_EXCEEDED.
func TestPSRV0900_PoolsSupersetWorstCaseStaysBounded(t *testing.T) {
	prefix := recoverPrefix()
	f, _, id := worstCasePools(t, func(int) string { return prefix + strings.Repeat(`"`, MaxPoolRecoverReasonBytes) })
	st := *f.st.pools
	pool := escaping(0, wire.MaxLabelBytes)
	tree := strings.Repeat("c", 64)
	seq := wire.Size(fmt.Sprint(uint64(1<<63 - 1)))
	attempt := func(i int) string {
		// attempt:<authority>:<queue>:<32 hex>, at the Identifier bound.
		q := strings.TrimPrefix(f.st.queue.QueueID.Raw, "queue:")
		local := fmt.Sprintf("%032x", i)
		pad := wire.MaxIdentifierBytes - len("attempt:"+q+":"+local)
		return "attempt:" + q + strings.Repeat("x", pad) + ":" + local
	}
	member := func(i int) string { return escaping(i, wire.MaxLabelBytes) }
	for i := range st.Entries {
		en := &st.Entries[i]
		en.PoolID, en.MemberID = pool, member(i)
		en.Holder = escaping(i, wire.MaxLabelBytes)
		en.ConfigRef.Path = escaping(i, wire.MaxIdentifierBytes)
		en.RunnerPID, en.RunnerStarted, en.Stage = "4194304", escaping(i, 128), "integrate"
		en.AttemptID, en.Generation = attempt(i), seq
		en.Sweep = &snapshot.PoolSweepOwner{RequestSha256: id, Previous: &id, Phase: "prepare", Attempt: "2", Tree: tree}
		for k := 1; k < snapshot.MaxSharedAttempts; k++ {
			en.Shared = append(en.Shared, snapshot.PoolShare{AttemptID: attempt(i*snapshot.MaxSharedAttempts + k), Generation: seq})
		}
		en.Operation = &snapshot.PoolOperationPending{Name: "retarget", RequestID: escaping(i, wire.MaxIdentifierBytes), Source: en.Source}
		en.Hold = &snapshot.PoolHold{HeldAt: "2026-10-22T00:00:00Z", ExpiresAt: "2026-10-22T00:00:01Z"}
	}
	for i := range st.Identities {
		st.Identities[i].PoolID, st.Identities[i].MemberID = pool, member(i)
	}
	for i := range st.Operations {
		st.Operations[i].PoolID, st.Operations[i].MemberID, st.Operations[i].RequestID = pool, member(i/2), escaping(i, wire.MaxIdentifierBytes)
	}
	for i := 0; i < snapshot.MaxSlotEntries; i++ {
		st.Slots = append(st.Slots, snapshot.SlotEntry{SlotsID: escaping(i, wire.MaxLabelBytes), AttemptID: attempt(i), Generation: seq, Holder: escaping(i, wire.MaxLabelBytes), Count: wire.Count(fmt.Sprint(intent.MaxSlotCapacity)), Exclusive: true, Class: escaping(i, wire.MaxLabelBytes), Priority: "P0", Seq: seq, State: snapshot.SlotWaiting})
	}
	for i := 0; i < snapshot.MaxPoolWaiters; i++ {
		ticket := "ticket:" + strings.TrimPrefix(f.st.queue.QueueID.Raw, "queue:") + ":"
		ticket += fmt.Sprintf("%0*d", wire.MaxIdentifierBytes-len(ticket), i)
		st.Waiters = append(st.Waiters, snapshot.PoolWaiter{PoolID: escaping(i, wire.MaxLabelBytes), TicketID: ticket, Kind: snapshot.WaiterGrace, AttemptID: attempt(i), Generation: seq, Holder: escaping(i, wire.MaxLabelBytes), Priority: "P0", Seq: seq, Since: "2026-10-22T00:00:00Z", ExpiresAt: "2026-10-22T00:00:01Z"})
	}
	// The superset is no valid state (a HELD entry has no attempt), so
	// Encode's decode refuses it; its bytes are the bound being checked.
	raw, err := st.Encode()
	t.Logf("superset pools.json %d B, bound %d B (%v)", len(raw), snapshot.MaxPoolStateBytes, err)
	if len(raw) > snapshot.MaxPoolStateBytes {
		t.Fatalf("superset worst case %d B exceeds the %d B pools.json bound", len(raw), snapshot.MaxPoolStateBytes)
	}
	if wire.CodeOf(err) == wire.CodeLimitExceeded {
		t.Fatalf("superset refused as over the bound: %v", err)
	}

	// Past the bound, Encode refuses LIMIT_EXCEEDED, so the planner posts
	// nothing (PSR-V0-202's planning refusal is the same check).
	over := st
	over.Entries = append([]snapshot.PoolEntry{}, st.Entries...)
	for i := range over.Entries {
		over.Entries[i].Reason = strings.Repeat(`"`, 4096)
	}
	if _, err := over.Encode(); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("an over-bound pools.json was not refused LIMIT_EXCEEDED: %v", err)
	}
}
