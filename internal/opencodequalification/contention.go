package opencodequalification

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	// latencyOutcomeQualifiedContended (AHI-372) is a contended miss within the AHI-012 limits scaled
	// by the same-window baseline slowdown, capped at contendedMaxScale; it is also the report result.
	latencyOutcomeQualifiedContended = "QUALIFIED_UNDER_CONTENTION"
	// contendedMaxScale is the declared maximum factor by which a contended qualification may scale
	// the AHI-012 limits, whatever the measured slowdown.
	contendedMaxScale = 1.5
	notObserved       = "NOT_OBSERVED"
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

// procLoadavgPath is the Linux load-average file; tests point it at a fixture.
var procLoadavgPath = "/proc/loadavg"

// observeContention reads load averages and the run queue from /proc/loadavg (Linux) or
// `sysctl -n vm.loadavg` (macOS); anything it cannot read stays unobserved, never zero.
func observeContention() contentionSnapshot {
	s := contentionSnapshot{cpus: runtime.NumCPU(), source: "none"}
	if b, e := os.ReadFile(procLoadavgPath); e == nil {
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
// p95 limit in a window whose observed peak load per CPU exceeds the declared threshold and, when the
// AHI-172 same-window baseline was observed, stayed within its slowdown-scaled limits, or a
// regression (any other miss, including one whose contention was not observed). A contended miss
// within the AHI-372 capped scaled limits qualifies under contention instead of retrying.
func latencyOutcome(s latencySample) string {
	if len(latencyFailures(s)) == 0 {
		return latencyOutcomePass
	}
	peak, ok := s.peakLoadPerCPU()
	if !ok || !latencyResample(s) || peak <= latencyContentionLoadPerCPU || baselineComparison(s) == baselineBeyond {
		return latencyOutcomeRegressed
	}
	if _, query, lifecycle, ok := contendedLimits(s); ok && p95(s.query) <= query && p95(s.lifecycle) <= lifecycle {
		return latencyOutcomeQualifiedContended
	}
	return latencyOutcomeContended
}

// contendedLimits are the AHI-372 qualifying limits: the AHI-012 limits scaled by the observed
// same-window slowdown, capped at contendedMaxScale and with no tolerance; false when the baseline
// was not observed.
func contendedLimits(s latencySample) (scale, query, lifecycle float64, ok bool) {
	slow, ok := s.baseline.slowdown()
	if !ok {
		return 0, 0, 0, false
	}
	scale = math.Min(slow, contendedMaxScale)
	return scale, queryP95LimitMs * scale, lifecycleP95LimitMs * scale, true
}

// contendedQualificationEvidence records what a QUALIFIED_UNDER_CONTENTION result rests on.
func contendedQualificationEvidence(s latencySample) Object {
	scale, query, lifecycle, _ := contendedLimits(s)
	slow, _ := s.baseline.slowdown()
	peak, _ := s.peakLoadPerCPU()
	return Object{"requirement": "AHI-372", "latencyQualification": "CONTENDED", "queryP95Ms": p95(s.query), "lifecycleP95Ms": p95(s.lifecycle), "quietQueryLimitMs": queryP95LimitMs, "quietLifecycleLimitMs": lifecycleP95LimitMs, "baselineSlowdown": slow, "maxScaleFactor": contendedMaxScale, "scaleFactor": scale, "scaledQueryLimitMs": query, "scaledLifecycleLimitMs": lifecycle, "peakLoadPerCPU": peak, "contentionThresholdLoadPerCPU": latencyContentionLoadPerCPU}
}

// latencyOnlyRetry reports whether latency is the only failed check and it missed under contention.
func latencyOnlyRetry(checks Object, decisive latencySample) bool {
	return latencyOnlyOutcome(checks, decisive) == latencyOutcomeContended
}

// latencyOnlyQualifiedContended reports whether latency is the only failed check and it missed
// within the AHI-372 capped contended limits.
func latencyOnlyQualifiedContended(checks Object, decisive latencySample) bool {
	return latencyOnlyOutcome(checks, decisive) == latencyOutcomeQualifiedContended
}

// latencyOnlyOutcome is the decisive latency outcome when latency is the only failed check, else "".
func latencyOnlyOutcome(checks Object, decisive latencySample) string {
	for k, v := range checks {
		if k != "latency" && !truth(v) {
			return ""
		}
	}
	if truth(checks["latency"]) {
		return ""
	}
	return latencyOutcome(decisive)
}

// retryHint names the load below which a retry is a fair quiet-host measurement.
func retryHint(s latencySample) string {
	cpus := s.end.cpus
	if cpus <= 0 {
		cpus = s.start.cpus
	}
	peak, _ := s.peakLoadPerCPU()
	hint := fmt.Sprintf("retry when the 1-minute load average is below %.1f (%.2f x %d logical CPUs); the timing window peaked at %.2f per CPU", latencyContentionLoadPerCPU*float64(cpus), latencyContentionLoadPerCPU, cpus, peak)
	if slow, ok := s.baseline.slowdown(); ok {
		hint += fmt.Sprintf("; the same-window no-op baseline ran at %.2fx its quiet reference", slow)
	}
	return hint
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
