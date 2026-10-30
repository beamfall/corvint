package cli

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/store"
)

// writerContext lets a store writer sample recordedAt again once it holds the
// head it plans against, so waiting behind another writer is not refused as a
// clock that stepped backward (CAL-V0-012).
func writerContext() context.Context {
	return store.WithClock(context.Background(), store.WallClock)
}
