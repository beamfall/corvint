package store

import (
	"context"
	"time"
)

type leaseLockWaitKey struct{}

// WithLeaseLockWait carries a caller-chosen lock wait (CAL-V0-109, the
// `--lock-wait` of release and attempt heartbeat) to the lease writes under
// ctx. It bounds their preparation admission and writer-lock acquisition in
// place of the 30-second default. It is never part of the request, its digest
// or any receipt, so the same request ID replays with or without it.
func WithLeaseLockWait(ctx context.Context, wait time.Duration) context.Context {
	return context.WithValue(ctx, leaseLockWaitKey{}, wait)
}

// leaseLockWaitOf returns the caller's wait, or zero (the default) when none.
func leaseLockWaitOf(ctx context.Context) time.Duration {
	wait, _ := ctx.Value(leaseLockWaitKey{}).(time.Duration)
	return wait
}
