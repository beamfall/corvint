//go:build darwin

package dispatch

import (
	"context"
	"strings"
	"time"
)

func samplePressure(ctx context.Context, now time.Time) PressureSample {
	s := PressureSample{SampledAt: now, Source: "darwin-sysctl-host"}
	raw, err := runPressureCommand(ctx, []string{"/usr/sbin/sysctl", "vm.loadavg", "vm.swapusage", "hw.logicalcpu"})
	if err != nil {
		s.Problems = []string{err.Error()}
		return s
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || (key != "vm.loadavg" && key != "vm.swapusage" && key != "hw.logicalcpu") {
			s.Problems = []string{"invalid pressure sysctl fields"}
			return s
		}
		if _, duplicate := values[key]; duplicate {
			s.Problems = []string{"duplicate pressure sysctl field"}
			return s
		}
		values[key] = strings.TrimSpace(value)
	}
	s.LoadAverage, err = parseLoad([]byte(values["vm.loadavg"]), true)
	s.LoadKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "load: "+err.Error())
	}
	s.CPUs, err = parseCPUs([]byte(values["hw.logicalcpu"]))
	s.CPUKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "cpus: "+err.Error())
	}
	s.SwapTotalBytes, s.SwapUsedBytes, err = parseDarwinSwap([]byte(values["vm.swapusage"]))
	s.SwapKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "swap: "+err.Error())
	}
	return s
}
