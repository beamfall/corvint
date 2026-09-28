package authority

import (
	"context"
	"time"
)

type lockObserverKey struct{}

// WithLockObserver records the duration from successful flock acquisition to
// successful release for locks acquired through ctx. It adds no store writes
// or authority. The callback runs once after release and must not block.
func WithLockObserver(ctx context.Context, observe func(time.Duration)) context.Context {
	return context.WithValue(ctx, lockObserverKey{}, observe)
}
