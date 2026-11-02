//go:build darwin || linux

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime/metrics"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// walkSplit is one walk-cost run's wall time split into CPU and I/O wait
// (V1-2186, LGL-V0-804). CPUMs is the process's user plus system time and
// SysMs its system part; GoUserCPUMs and GoGCCPUMs are the Go runtime's
// mutator and GC CPU estimates. IOWaitMs is the sampled thread time in
// uninterruptible sleep, or "NOT_OBSERVED".
type walkSplit struct {
	WallMs        float64
	CPUMs, SysMs  float64
	GoUserCPUMs   float64
	GoGCCPUMs     float64
	RunQueueMs    float64 // -1 when /proc/self/task/*/schedstat is unavailable
	IOWaitMs      any
	IOSamples     int // sampler ticks
	IOBusySamples int // ticks with at least one thread in uninterruptible sleep
	InBlocks      int64
}

type walkProbe struct {
	user, sys, goUser, goGC float64
	runQueue                time.Duration
	inBlocks                int64
}

var walkMetrics = []string{"/cpu/classes/user:cpu-seconds", "/cpu/classes/gc/total:cpu-seconds"}

func probeWalk() walkProbe {
	var p walkProbe
	var ru syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &ru) == nil {
		p.user = float64(ru.Utime.Nano()) / 1e6
		p.sys = float64(ru.Stime.Nano()) / 1e6
		p.inBlocks = int64(ru.Inblock)
	}
	samples := make([]metrics.Sample, len(walkMetrics))
	for i, name := range walkMetrics {
		samples[i].Name = name
	}
	metrics.Read(samples)
	if samples[0].Value.Kind() == metrics.KindFloat64 {
		p.goUser = samples[0].Value.Float64() * 1000
	}
	if samples[1].Value.Kind() == metrics.KindFloat64 {
		p.goGC = samples[1].Value.Float64() * 1000
	}
	p.runQueue = runQueueDelay()
	return p
}

func (p walkProbe) since(before walkProbe, wall time.Duration, io ioSample) walkSplit {
	s := walkSplit{
		WallMs: float64(wall.Microseconds()) / 1000, CPUMs: p.user + p.sys - before.user - before.sys, SysMs: p.sys - before.sys,
		GoUserCPUMs: p.goUser - before.goUser, GoGCCPUMs: p.goGC - before.goGC, RunQueueMs: -1, InBlocks: p.inBlocks - before.inBlocks,
		IOWaitMs: "NOT_OBSERVED", IOSamples: io.ticks, IOBusySamples: io.busyTicks,
	}
	if p.runQueue >= 0 && before.runQueue >= 0 {
		s.RunQueueMs = float64((p.runQueue - before.runQueue).Microseconds()) / 1000
	}
	if io.observed {
		s.IOWaitMs = float64(io.blockedThreadTicks) * float64(io.every.Microseconds()) / 1000
	}
	return s
}

// ioSampler counts, every tick, the process's threads in uninterruptible
// sleep (state D in /proc/self/task/*/stat).
type ioSampler struct {
	every time.Duration
	done  chan struct{}
	wg    sync.WaitGroup
	out   ioSample
}

type ioSample struct {
	every                                time.Duration
	observed                             bool
	ticks, busyTicks, blockedThreadTicks int
}

func startIOSampler(every time.Duration) *ioSampler {
	s := &ioSampler{every: every, done: make(chan struct{})}
	s.out.every = every
	if every <= 0 {
		return s
	}
	if _, err := os.Stat("/proc/self/task"); err != nil {
		return s
	}
	s.out.observed = true
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-tick.C:
				blocked := threadsInDiskSleep()
				s.out.ticks++
				s.out.blockedThreadTicks += blocked
				if blocked > 0 {
					s.out.busyTicks++
				}
			}
		}
	}()
	return s
}

func (s *ioSampler) stop() ioSample {
	close(s.done)
	s.wg.Wait()
	return s.out
}

// threadsInDiskSleep counts the threads whose /proc state is D.
func threadsInDiskSleep() int {
	tasks, err := os.ReadDir("/proc/self/task")
	if err != nil {
		return 0
	}
	n := 0
	for _, task := range tasks {
		raw, err := os.ReadFile(filepath.Join("/proc/self/task", task.Name(), "stat"))
		if err != nil {
			continue
		}
		if threadState(raw) == 'D' {
			n++
		}
	}
	return n
}

// threadState is the state letter of a /proc stat line. It follows the
// parenthesised command name, which may itself hold spaces or parentheses.
func threadState(raw []byte) byte {
	if i := strings.LastIndexByte(string(raw), ')'); i >= 0 && i+2 < len(raw) {
		return raw[i+2]
	}
	return 0
}

// medianSplit is the per-field median of splits (IOWaitMs only when every
// run observed it).
func medianSplit(splits []walkSplit) map[string]any {
	med := func(get func(walkSplit) float64) float64 {
		v := make([]float64, len(splits))
		for i, s := range splits {
			v[i] = get(s)
		}
		sort.Float64s(v)
		return v[len(v)/2]
	}
	out := map[string]any{
		"wallMs": med(func(s walkSplit) float64 { return s.WallMs }), "cpuMs": med(func(s walkSplit) float64 { return s.CPUMs }),
		"sysMs": med(func(s walkSplit) float64 { return s.SysMs }), "goUserCpuMs": med(func(s walkSplit) float64 { return s.GoUserCPUMs }),
		"goGcCpuMs": med(func(s walkSplit) float64 { return s.GoGCCPUMs }), "runQueueMs": med(func(s walkSplit) float64 { return s.RunQueueMs }),
		"ioWaitMs": "NOT_OBSERVED",
	}
	for _, s := range splits {
		if _, ok := s.IOWaitMs.(float64); !ok {
			return out
		}
	}
	out["ioWaitMs"] = med(func(s walkSplit) float64 { return s.IOWaitMs.(float64) })
	return out
}

// walkCommand is the argv and the CORVINT_TASKS_WALK_COST settings a run
// was started with.
func walkCommand() string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CORVINT_TASKS_WALK_COST") {
			env = append(env, kv)
		}
	}
	sort.Strings(env)
	return strings.Join(append(env, os.Args...), " ")
}

// binarySum is the SHA-256 of the running test binary, or NOT_OBSERVED.
func binarySum() string {
	path, err := os.Executable()
	if err != nil {
		return "NOT_OBSERVED"
	}
	f, err := os.Open(path)
	if err != nil {
		return "NOT_OBSERVED"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "NOT_OBSERVED"
	}
	return hex.EncodeToString(h.Sum(nil))
}

// LGL-V0-804: the sampler reads the state after the last ')' of the command
// name, and a run that did not observe I/O wait keeps the median
// NOT_OBSERVED rather than zero.
func TestLGLV0804_WalkSplitParsesStateAndKeepsUnobserved(t *testing.T) {
	for raw, want := range map[string]byte{
		"12 (store.test) D 1 2":  'D',
		"13 (a) b) (c) S 1 2":    'S',
		"14 (worker (x)) R 0":    'R',
		"15 truncated (no state": 0,
	} {
		if got := threadState([]byte(raw)); got != want {
			t.Errorf("threadState(%q) = %q, want %q", raw, got, want)
		}
	}
	observed := []walkSplit{{WallMs: 3, IOWaitMs: 2.0}, {WallMs: 1, IOWaitMs: 0.0}, {WallMs: 2, IOWaitMs: 4.0}}
	if m := medianSplit(observed); m["wallMs"] != 2.0 || m["ioWaitMs"] != 2.0 {
		t.Fatalf("median of observed runs: %v", m)
	}
	mixed := append(observed, walkSplit{WallMs: 9, IOWaitMs: "NOT_OBSERVED"})
	if m := medianSplit(mixed); m["ioWaitMs"] != "NOT_OBSERVED" {
		t.Fatalf("median with an unobserved run: %v", m)
	}
	if s := startIOSampler(0).stop(); s.observed || s.ticks != 0 {
		t.Fatalf("a disabled sampler sampled: %+v", s)
	}
}
