package store

import (
	"context"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
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

// testClocks maps a store's state directory to a clock a test registered for
// that store alone (V1-2743), so a test can advance one store past a program
// wall while parallel tests' stores keep real time. Production code never
// registers one, so every store reads time.Now.
var testClocks sync.Map

// storeNow is the wall time the workflow, supervisor and pool writers of repo
// read: time.Now unless a test registered a clock for this store.
func storeNow(repo *intent.Repository) time.Time {
	if repo != nil {
		if c, ok := testClocks.Load(repo.StateDir); ok {
			return c.(func() time.Time)()
		}
	}
	return time.Now()
}

// storeClock is poolClock read from storeNow(repo).
func storeClock(repo *intent.Repository) func() wire.Timestamp {
	return func() wire.Timestamp {
		return wire.Timestamp(storeNow(repo).UTC().Format("2006-01-02T15:04:05Z"))
	}
}
