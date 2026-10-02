package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type clockKey struct{}

// WithClock gives the writers under ctx a live clock. A caller samples its
// timestamp before the writer has waited for the store lock; another writer
// that commits during that wait moves the head receipt past the sample, which
// would look like a clock stepping backward (CAL-V0-012). A writer under a
// live clock samples again once it holds the head it will plan against.
func WithClock(ctx context.Context, clock func() wire.Timestamp) context.Context {
	return context.WithValue(ctx, clockKey{}, clock)
}

// WallClock is the live clock of a command-line or supervisor writer.
func WallClock() wire.Timestamp { return poolClock() }

// recordedAt is the transaction time for a head observed after now was
// sampled: the live clock when ctx carries one and it has not gone back, else
// now. A clock that really stepped backward still reads earlier than the
// head receipt, so the model still refuses it.
func recordedAt(ctx context.Context, now wire.Timestamp) wire.Timestamp {
	if clock, ok := ctx.Value(clockKey{}).(func() wire.Timestamp); ok {
		if at := clock(); at > now {
			return at
		}
	}
	return now
}
