//go:build !darwin && !linux

package dispatch

import (
	"context"
	"time"
)

func samplePressure(_ context.Context, now time.Time) PressureSample {
	return PressureSample{SampledAt: now, Source: "unsupported", Problems: []string{"host pressure sampling is unsupported on this OS"}}
}
