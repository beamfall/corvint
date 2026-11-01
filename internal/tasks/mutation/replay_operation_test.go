package mutation_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-3275: a request ID reused under a different operation is a conflict,
// never a replay of the original. The recorded mutation digest is the SHA-256
// of the whole envelope, operation included, so replay detection compares the
// operation without a separate index field.
func TestTMV0006_ReusedRequestIDUnderDifferentOperationConflicts(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	ctx := newCtx(t, owner, nil, at01)
	idx := mutation.NewMemoryIndex()
	ctx.Requests = idx

	first := apply(t, ctx, envelope("req-op", owner, "AT-01", "1", mutation.OpPrioritize, obj("priority", str("P1"), "order", str("1"))))
	want(t, first, mutation.OutcomeCompleted, "")
	if err := idx.Record(mutation.IndexEntry{RequestID: "req-op", MutationSha256: first.MutationSha256, Outcome: first.Outcome}); err != nil {
		t.Fatalf("record: %v", err)
	}
	other := apply(t, ctx, envelope("req-op", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("first"))))
	want(t, other, mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
	if other.Outcome.Replayed || other.Post != nil || other.Planned() {
		t.Fatalf("a reused request ID under another operation replayed or planned: %+v", other.Outcome)
	}
	again := apply(t, ctx, envelope("req-op", owner, "AT-01", "1", mutation.OpPrioritize, obj("priority", str("P1"), "order", str("1"))))
	if again.Outcome.Outcome != mutation.OutcomeCompleted || !again.Outcome.Replayed {
		t.Fatalf("identical retry of the original operation must replay: %+v", again.Outcome)
	}
}
