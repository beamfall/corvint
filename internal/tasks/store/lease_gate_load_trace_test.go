//go:build darwin || linux

package store

import (
	"os"
	"path/filepath"
	"runtime/metrics"
	"runtime/trace"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// processProbe is a per-call snapshot of process-wide clocks the
// CAL-V0-263 load harness differences around one lease call (V1-2540): CPU
// time, the kernel's run-queue delay summed over the process's threads, and
// the Go runtime's goroutine scheduling latency, stop-the-world pause and GC
// cycle counters. A wall-time gap that none of the named lease stages
// accounts for is CPU work when CPU grows with it, scheduler delay when the
// run-queue or goroutine latency grows with it, and blocking (a syscall or
// lock wait off the CPU) when neither does.
type processProbe struct {
	at       time.Time
	cpu      time.Duration
	runDelay time.Duration // -1 when /proc/self/task/*/schedstat is unavailable
	gcCycles uint64
	schedLat *metrics.Float64Histogram
	pauses   *metrics.Float64Histogram
}

var probeMetrics = []string{"/sched/latencies:seconds", "/sched/pauses/total/gc:seconds", "/gc/cycles/total:gc-cycles"}

func probeProcess() processProbe {
	p := processProbe{at: time.Now(), runDelay: runQueueDelay()}
	var ru syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &ru) == nil {
		p.cpu = time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
	}
	samples := make([]metrics.Sample, len(probeMetrics))
	for i, name := range probeMetrics {
		samples[i].Name = name
	}
	metrics.Read(samples)
	if samples[0].Value.Kind() == metrics.KindFloat64Histogram {
		p.schedLat = copyHistogram(samples[0].Value.Float64Histogram())
	}
	if samples[1].Value.Kind() == metrics.KindFloat64Histogram {
		p.pauses = copyHistogram(samples[1].Value.Float64Histogram())
	}
	if samples[2].Value.Kind() == metrics.KindUint64 {
		p.gcCycles = samples[2].Value.Uint64()
	}
	return p
}

func copyHistogram(h *metrics.Float64Histogram) *metrics.Float64Histogram {
	return &metrics.Float64Histogram{Counts: append([]uint64(nil), h.Counts...), Buckets: h.Buckets}
}

// runQueueDelay sums the second schedstat field (time spent runnable on a
// run queue, in ns) over every thread of this process; -1 off Linux. The
// sum covers only live threads, so a thread that exits between two probes
// takes its delay with it: a delta can undercount and, after such an exit,
// go negative, which marks that sample's run delay as unknown.
func runQueueDelay() time.Duration {
	tasks, err := os.ReadDir("/proc/self/task")
	if err != nil {
		return -1
	}
	var total int64
	for _, task := range tasks {
		raw, err := os.ReadFile(filepath.Join("/proc/self/task", task.Name(), "schedstat"))
		if err != nil {
			continue // the thread exited between the listing and the read
		}
		fields := strings.Fields(string(raw))
		if len(fields) < 2 {
			continue
		}
		if n, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
			total += n
		}
	}
	return time.Duration(total)
}

// histogramDelta returns the summed (bucket lower bound, so a lower bound)
// and largest observed value, in ms, of the samples recorded between a and b.
func histogramDelta(a, b *metrics.Float64Histogram) (sumMS, maxMS float64) {
	if a == nil || b == nil || len(a.Counts) != len(b.Counts) {
		return 0, 0
	}
	for i := range b.Counts {
		n := b.Counts[i] - a.Counts[i]
		if n == 0 {
			continue
		}
		lo := b.Buckets[i]
		if lo < 0 || lo != lo {
			lo = 0
		}
		sumMS += float64(n) * lo * 1000
		hi := b.Buckets[i+1]
		if hi > 1e9 {
			hi = lo
		}
		maxMS = max(maxMS, hi*1000)
	}
	return sumMS, maxMS
}

// processDelta is what one call added to the process clocks.
type processDelta struct {
	CPUMS, RunQueueDelayMS     float64
	GoSchedLatencySumMS        float64 `json:",omitempty"`
	GoSchedLatencyMaxMS        float64 `json:",omitempty"`
	GCPauseSumMS, GCPauseMaxMS float64 `json:",omitempty"`
	GCCycles                   uint64  `json:",omitempty"`
}

func (a processProbe) to(b processProbe) processDelta {
	d := processDelta{CPUMS: ms(b.cpu - a.cpu), RunQueueDelayMS: -1, GCCycles: b.gcCycles - a.gcCycles}
	if a.runDelay >= 0 && b.runDelay >= 0 {
		d.RunQueueDelayMS = ms(b.runDelay - a.runDelay)
	}
	d.GoSchedLatencySumMS, d.GoSchedLatencyMaxMS = histogramDelta(a.schedLat, b.schedLat)
	d.GCPauseSumMS, d.GCPauseMaxMS = histogramDelta(a.pauses, b.pauses)
	return d
}

// writeFlightTrace retains the flight recorder's window in name, removing a
// partial file when the write or close fails.
func writeFlightTrace(recorder *trace.FlightRecorder, name string) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	_, err = recorder.WriteTo(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}
