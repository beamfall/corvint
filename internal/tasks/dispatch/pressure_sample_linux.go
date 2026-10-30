//go:build linux

package dispatch

import (
	"context"
	"time"
)

func samplePressure(ctx context.Context, now time.Time, want pressureWant) PressureSample {
	return sampleLinuxPressure(ctx, now, want, osPressureOpen)
}
