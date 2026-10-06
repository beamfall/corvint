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
	statErr := err
	if err == nil {
		s.CPUs, err = parseLinuxCPUs(raw)
	}
	s.CPUKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "cpus: "+err.Error())
	}
	// CAL-V0-125: the same /proc/stat read carries the cumulative ticks.
	err = statErr
	if err == nil {
		s.CPUBusyTicks, s.CPUTotalTicks, err = parseLinuxCPUTicks(raw)
	}
	s.CPUTicksKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "cpu ticks: "+err.Error())
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
