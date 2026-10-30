package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fixtureAttemptID is an attempt identity in fixture.QueueID, the shape
// snapshot.AttemptQueue accepts.
const fixtureAttemptID = "attempt:acme:main:0123456789abcdef0123456789abcdef"

// validPoolEntry returns a pools.json entry for member m of pool "db" in
// state (PREPARING, ALLOCATED, QUARANTINED or CLEANING) that
// snapshot.DecodePools accepts under fixture.QueueID, then applies each edit
// in order (V1-2188). It carries every field the codec requires and nothing
// optional: allocatedSeq 1 and changedSeq 1, policy and request digests, a
// holder and stage, an owner attempt for ALLOCATED, and a health runner
// binding for PREPARING and CLEANING. The edited entry must still encode, or
// the test fails here rather than at a confusing codec error later.
func validPoolEntry(t testing.TB, m, state string, edits ...func(*snapshot.PoolEntry)) snapshot.PoolEntry {
	t.Helper()
	en := snapshot.PoolEntry{
		PoolAllocation: snapshot.PoolAllocation{PoolID: "db", MemberID: m, AllocationID: wire.Sum([]byte("allocation " + m)), DefinitionSha256: wire.Sum([]byte("definition " + m)), AllocatedSeq: "1"},
		State:          state, Holder: "builder", Stage: "implement", Generation: "1", ChangedSeq: "1",
		PolicySha256: wire.Sum([]byte("policy")), RequestSha256: wire.Sum([]byte("claim " + m)), RunnerPID: "0",
	}
	switch state {
	case "ALLOCATED":
		en.AttemptID = fixtureAttemptID
	case "PREPARING", "CLEANING":
		en.RunnerPID, en.RunnerStarted, en.CommandKind, en.CommandRevision = "4242", "started", "health", strings.Repeat("a", 40)
	}
	for _, edit := range edits {
		edit(&en)
	}
	q, err := wire.ParseQueueID("", fixture.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&snapshot.PoolState{QueueID: q, Entries: []snapshot.PoolEntry{en}}).Encode(); err != nil {
		t.Fatalf("validPoolEntry(%s, %s) does not encode: %v", m, state, err)
	}
	return en
}

// V1-2188: the helper yields an entry the codec accepts in every state, and
// an edit is applied before that check.
func TestValidPoolEntryEncodesInEveryState(t *testing.T) {
	for _, state := range []string{"PREPARING", "ALLOCATED", "QUARANTINED", "CLEANING"} {
		en := validPoolEntry(t, "m1", state, func(e *snapshot.PoolEntry) { e.Reason = "edited" })
		if en.State != state || en.Reason != "edited" {
			t.Fatalf("%s: %+v", state, en)
		}
	}
}
