package opencodequalification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// AHI-171 host contention: each timing window records the host's load averages, logical CPU count
// and run-queue length at its start and end, so a latency-only miss on a contended host is reported
// as retry-later instead of a bare latency failure.

const (
	// latencyContentionLoadPerCPU is the declared threshold: a timing window whose peak 1-minute load
	// average per logical CPU exceeds it is contended.
	latencyContentionLoadPerCPU = 0.5
	// RetryLaterExitCode is the qualify-opencode exit for a latency-only miss under contention.
	RetryLaterExitCode = 75

	latencyOutcomePass      = "PASS"
	latencyOutcomeRegressed = "LATENCY_REGRESSION"
	latencyOutcomeContended = "LATENCY_HOST_CONTENDED"
	resultRetryLater        = "RETRY_LATER"
	notObserved             = "NOT_OBSERVED"
)

// contentionSnapshot is one host-contention reading at a timing-window boundary.
type contentionSnapshot struct {
	load       [3]float64
	loadOK     bool
	runQueue   int
	runQueueOK bool
	cpus       int
	source     string
}

// readContention is the host reading; tests replace it.
var readContention = observeContention

// observeContention reads load averages and the run queue from /proc/loadavg (Linux) or
// `sysctl -n vm.loadavg` (macOS); anything it cannot read stays unobserved, never zero.
func observeContention() contentionSnapshot {
	s := contentionSnapshot{cpus: runtime.NumCPU(), source: "none"}
	if b, e := os.ReadFile("/proc/loadavg"); e == nil {
		f := strings.Fields(string(b))
		if len(f) >= 4 && parseLoad(&s, f[:3]) {
			s.source = "/proc/loadavg"
			if n, e := strconv.Atoi(strings.SplitN(f[3], "/", 2)[0]); e == nil {
				s.runQueue, s.runQueueOK = n, true
			}
		}
		return s
	}
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if out, e := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "vm.loadavg").Output(); e == nil {
			if f := strings.Fields(strings.Trim(strings.TrimSpace(string(out)), "{}")); len(f) >= 3 && parseLoad(&s, f[:3]) {
				s.source = "sysctl vm.loadavg"
			}
		}
	}
	return s
}

func parseLoad(s *contentionSnapshot, fields []string) bool {
	for i, f := range fields {
		v, e := strconv.ParseFloat(f, 64)
		if e != nil || v < 0 {
			return false
		}
		s.load[i] = v
	}
	s.loadOK = true
	return true
}

func (s contentionSnapshot) evidence() Object {
	o := Object{"loadAverage1m": notObserved, "loadAverage5m": notObserved, "loadAverage15m": notObserved, "runQueue": notObserved, "logicalCPUs": notObserved, "source": s.source}
	if s.source == "" {
		o["source"] = "none"
	}
	if s.loadOK {
		o["loadAverage1m"], o["loadAverage5m"], o["loadAverage15m"] = s.load[0], s.load[1], s.load[2]
	}
	if s.runQueueOK {
		o["runQueue"] = s.runQueue
	}
	if s.cpus > 0 {
		o["logicalCPUs"] = s.cpus
	}
	return o
}

// peakLoadPerCPU is the window's highest observed 1-minute load average per logical CPU.
func (s latencySample) peakLoadPerCPU() (float64, bool) {
	peak, ok := 0.0, false
	for _, c := range []contentionSnapshot{s.start, s.end} {
		if c.loadOK && c.cpus > 0 {
			if v := c.load[0] / float64(c.cpus); !ok || v > peak {
				peak, ok = v, true
			}
		}
	}
	return peak, ok
}

// latencyOutcome classifies the decisive attempt: within limits, a complete sample that missed only a
// p95 limit in a window whose observed peak load per CPU exceeds the declared threshold, or a
// regression (any other miss, including one whose contention was not observed).
func latencyOutcome(s latencySample) string {
	if len(latencyFailures(s)) == 0 {
		return latencyOutcomePass
	}
	if peak, ok := s.peakLoadPerCPU(); ok && latencyResample(s) && peak > latencyContentionLoadPerCPU {
		return latencyOutcomeContended
	}
	return latencyOutcomeRegressed
}

// latencyOnlyRetry reports whether latency is the only failed check and it missed under contention.
func latencyOnlyRetry(checks Object, decisive latencySample) bool {
	for k, v := range checks {
		if k != "latency" && !truth(v) {
			return false
		}
	}
	return !truth(checks["latency"]) && latencyOutcome(decisive) == latencyOutcomeContended
}

// retryHint names the load below which a retry is a fair quiet-host measurement.
func retryHint(s latencySample) string {
	cpus := s.end.cpus
	if cpus <= 0 {
		cpus = s.start.cpus
	}
	peak, _ := s.peakLoadPerCPU()
	return fmt.Sprintf("retry when the 1-minute load average is below %.1f (%.2f x %d logical CPUs); the timing window peaked at %.2f per CPU", latencyContentionLoadPerCPU*float64(cpus), latencyContentionLoadPerCPU, cpus, peak)
}

// RetryLaterError is a native campaign whose only failure is a latency miss under recorded host
// contention: the host is not qualified by this run, and the miss is not evidence of a regression.
type RetryLaterError struct{ Message string }

func (e *RetryLaterError) Error() string { return e.Message }

// IsRetryLater reports whether err is a retry-later campaign outcome.
func IsRetryLater(err error) bool {
	var r *RetryLaterError
	return errors.As(err, &r)
}
