package opencodequalification

import (
	"context"
	"math"
	"os"
	"os/exec"
	"slices"
	"time"
)

// AHI-172 same-window baseline: while each timing window runs, the harness repeatedly spawns a fixed
// no-op probe that does not depend on the candidate. The probe's p95 in the window, against its
// window reference, measures how slow the host was; a latency miss under contention is
// LATENCY_HOST_CONTENDED only when the candidate stays within its limits scaled by that slowdown.
// AHI-950: the reference is calibrated per window as the window's fastest probe spawn, never below
// the declared floor, so a host whose quiet spawn is slow does not show a standing slowdown.

const (
	baselineProbeInterval = 100 * time.Millisecond
	baselineMinSamples    = 20
	baselineMaxSamples    = 4096
	// baselineQuietP95Ms is the declared floor of the per-window quiet reference for one spawn of
	// the no-op probe (AHI-950).
	baselineQuietP95Ms = 2.0
	// latencyBaselineTolerance is how far beyond its slowdown-scaled limits a candidate may be and
	// still be explained by the host.
	latencyBaselineTolerance = 1.5

	baselineWithin = "WITHIN_SCALED_LIMITS"
	baselineBeyond = "BEYOND_SCALED_LIMITS"
)

// baselineProbePaths are the fixed no-op probes, first existing executable wins.
var baselineProbePaths = []string{"/usr/bin/true", "/bin/true"}

// baselineSample is one timing window's no-op probe spawn durations in milliseconds.
type baselineSample struct {
	probe string
	ms    []float64
}

// startBaseline begins sampling for one timing window; its stop function ends the sampling and
// returns the window's samples. Tests replace it.
var startBaseline = sampleBaseline

func sampleBaseline(ctx context.Context) func() baselineSample {
	s := baselineSample{probe: "none"}
	for _, p := range baselineProbePaths {
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() && st.Mode()&0111 != 0 {
			s.probe = p
			break
		}
	}
	if s.probe == "none" {
		return func() baselineSample { return s }
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan []float64, 1)
	go func() {
		out := []float64{}
		tick := time.NewTicker(baselineProbeInterval)
		defer tick.Stop()
		for {
			if len(out) < baselineMaxSamples {
				start := time.Now()
				// A spawn cut short by the window's end fails and is not a sample.
				if exec.CommandContext(ctx, s.probe).Run() == nil {
					out = append(out, float64(time.Since(start).Microseconds())/1000)
				}
			}
			select {
			case <-ctx.Done():
				done <- out
				return
			case <-tick.C:
			}
		}
	}()
	return func() baselineSample {
		cancel()
		s.ms = <-done
		return s
	}
}

// reference is the window's calibrated quiet reference (AHI-950): its fastest probe spawn, at least
// the declared floor.
func (b baselineSample) reference() float64 {
	return math.Max(baselineQuietP95Ms, slices.Min(b.ms))
}

// slowdown is the window's probe p95 over its calibrated reference, at least 1; false when the
// window has too few samples to measure.
func (b baselineSample) slowdown() (float64, bool) {
	if len(b.ms) < baselineMinSamples {
		return 0, false
	}
	if v := p95(b.ms) / b.reference(); v > 1 {
		return v, true
	}
	return 1, true
}

// baselineComparison says whether the candidate stayed within its limits scaled by the window's
// slowdown and tolerance; empty when the baseline was not observed.
func baselineComparison(s latencySample) string {
	slow, ok := s.baseline.slowdown()
	if !ok {
		return ""
	}
	if p95(s.query) <= queryP95LimitMs*slow*latencyBaselineTolerance && p95(s.lifecycle) <= lifecycleP95LimitMs*slow*latencyBaselineTolerance {
		return baselineWithin
	}
	return baselineBeyond
}

func baselineEvidence(s latencySample) Object {
	b := s.baseline
	o := Object{"probe": b.probe, "samples": len(b.ms), "quietReferenceP95Ms": notObserved, "referenceFloorMs": baselineQuietP95Ms, "tolerance": latencyBaselineTolerance, "p95Ms": notObserved, "slowdown": notObserved, "queryRelative": notObserved, "lifecycleRelative": notObserved, "scaledQueryLimitMs": notObserved, "scaledLifecycleLimitMs": notObserved, "comparison": notObserved}
	if b.probe == "" {
		o["probe"] = "none"
	}
	slow, ok := b.slowdown()
	if !ok {
		return o
	}
	base := p95(b.ms)
	o["p95Ms"], o["slowdown"], o["quietReferenceP95Ms"] = base, slow, b.reference()
	if base > 0 && len(s.query) > 0 && len(s.lifecycle) > 0 {
		o["queryRelative"], o["lifecycleRelative"] = p95(s.query)/base, p95(s.lifecycle)/base
	}
	o["scaledQueryLimitMs"] = queryP95LimitMs * slow * latencyBaselineTolerance
	o["scaledLifecycleLimitMs"] = lifecycleP95LimitMs * slow * latencyBaselineTolerance
	o["comparison"] = baselineComparison(s)
	return o
}
