//go:build linux

package dispatch

import (
	"context"
	"time"
)

func samplePressure(ctx context.Context, now time.Time) PressureSample {
	s := PressureSample{SampledAt: now, Source: "linux-proc-host"}
	raw, err := readPressureFile(ctx, "/proc/loadavg", pressureLoadBytes)
	if err == nil {
		s.LoadAverage, err = parseLoad(raw, false)
	}
	s.LoadKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "load: "+err.Error())
	}
	raw, err = readPressureFile(ctx, "/proc/stat", pressureCPUBytes)
	if err == nil {
		s.CPUs, err = parseLinuxCPUs(raw)
	}
	s.CPUKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "cpus: "+err.Error())
	}
	raw, err = readPressureFile(ctx, "/proc/meminfo", pressureMemBytes)
	if err == nil {
		s.SwapTotalBytes, s.SwapUsedBytes, err = parseLinuxSwap(raw)
	}
	s.SwapKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "swap: "+err.Error())
	}
	return s
}
