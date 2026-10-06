//go:build darwin

package dispatch

import (
	"context"
	"time"
)

// samplePressure leaves the CPU tick counters UNKNOWN (CAL-V0-125): Darwin
// exposes them only through host_processor_info/host_statistics, which a
// CGO_ENABLED=0 binary cannot call without forbidden linkname trampolines.
func samplePressure(ctx context.Context, now time.Time) PressureSample {
	s := PressureSample{SampledAt: now, Source: "darwin-sysctl-host"}
	raw, err := runPressureCommand(ctx, []string{"/usr/sbin/sysctl", "vm.loadavg", "kern.memorystatus_vm_pressure_level", "hw.logicalcpu"})
	if err != nil {
		s.Problems = []string{err.Error()}
		return s
	}
	return parseDarwinPressure(s, raw)
}
