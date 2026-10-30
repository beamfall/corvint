package gokernel

import (
	"context"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/observations"
)

// adapterCodeAppendBound caps how long an event waits for its adapterCodes rows, matching the Go
// adapters' bounded SOL-V0-010 append; an append still waiting is abandoned, not cancelled.
const adapterCodeAppendBound = 500 * time.Millisecond

// adapterCodeAppends counts appends still running, so a test can wait for an abandoned one.
var adapterCodeAppends sync.WaitGroup

// recordAdapterCodes appends one SOL-V0-010 row per admitted adapter code and waits for them no
// longer than adapterCodeAppendBound or ctx.
func recordAdapterCodes(ctx context.Context, root string, request EventRequest, codes []string) {
	if len(codes) == 0 {
		return
	}
	done := make(chan struct{})
	adapterCodeAppends.Add(1)
	go func() {
		defer adapterCodeAppends.Done()
		defer close(done)
		for _, code := range codes {
			_ = observations.Append(root, observations.AdapterDegradationEvent(request.Host, request.Event, code, request.CorvintVersion, time.Now()))
		}
	}()
	timer := time.NewTimer(adapterCodeAppendBound)
	defer timer.Stop()
	select {
	case <-done:
	case <-ctx.Done():
	case <-timer.C:
	}
}
