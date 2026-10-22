package opencodequalification

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// probeAt is a same-window baseline whose every no-op spawn took ms.
func probeAt(ms float64) baselineSample {
	b := baselineSample{probe: "/usr/bin/true"}
	for i := 0; i < 30; i++ {
		b.ms = append(b.ms, ms)
	}
	return b
}

func TestAHI172SameWindowBaseline(t *testing.T) {
	loaded := func(lifecycleMs float64, b baselineSample) latencySample {
		s, _ := timingSample(latencyRows(series(30, 0, 300, 0), series(30, 0, lifecycleMs, 0)))
		s.start, s.end, s.baseline = loadAt(6, 12), loadAt(11.5, 12), b
		return s
	}
	t.Run("AHI-172 both slow under contention is host-contended with the comparison recorded", func(t *testing.T) {
		// The probe ran at 2.5x its quiet reference, so lifecycle 600 ms is within 250 x 2.5 x 1.5.
		s := loaded(600, probeAt(5))
		if got := latencyOutcome(s); got != latencyOutcomeContended {
			t.Fatalf("outcome = %s", got)
		}
		ev := latencyAttemptEvidence([]latencySample{s})[0]["baseline"].(Object)
		if ev["comparison"] != baselineWithin || ev["p95Ms"] != 5.0 || ev["slowdown"] != 2.5 || ev["samples"] != 30 || ev["lifecycleRelative"] != 120.0 || ev["queryRelative"] != 60.0 || ev["scaledLifecycleLimitMs"] != 937.5 || ev["probe"] != "/usr/bin/true" {
			t.Fatalf("baseline evidence = %v", ev)
		}
		if hint := retryHint(s); !strings.Contains(hint, "no-op baseline ran at 2.50x its quiet reference") {
			t.Fatalf("hint = %q", hint)
		}
	})
	t.Run("AHI-172 a candidate slower than the baseline explains is a regression under load", func(t *testing.T) {
		// Load 11.5 on 12 CPUs is above the AHI-171 threshold, but the probe was at its quiet
		// reference, so lifecycle 600 ms is beyond 250 x 1 x 1.5.
		s := loaded(600, probeAt(1))
		if got := latencyOutcome(s); got != latencyOutcomeRegressed {
			t.Fatalf("outcome = %s", got)
		}
		allPass := Object{"latency": false, "native-edit": true}
		if latencyOnlyRetry(allPass, s) {
			t.Fatal("regression beyond the baseline reported retry-later")
		}
		if ev := baselineEvidence(s); ev["comparison"] != baselineBeyond || ev["slowdown"] != 1.0 {
			t.Fatalf("baseline evidence = %v", ev)
		}
	})
	t.Run("AHI-172 a slow baseline on a quiet host does not excuse a miss", func(t *testing.T) {
		s := loaded(600, probeAt(5))
		s.start, s.end = loadAt(1, 12), loadAt(2, 12)
		if got := latencyOutcome(s); got != latencyOutcomeRegressed {
			t.Fatalf("outcome = %s", got)
		}
	})
	t.Run("AHI-172 an unobserved baseline keeps the AHI-171 load rule and says so", func(t *testing.T) {
		short := probeAt(5)
		short.ms = short.ms[:baselineMinSamples-1]
		s := loaded(289, short)
		if got := latencyOutcome(s); got != latencyOutcomeContended {
			t.Fatalf("outcome = %s", got)
		}
		ev := baselineEvidence(s)
		for _, k := range []string{"p95Ms", "slowdown", "queryRelative", "lifecycleRelative", "scaledQueryLimitMs", "scaledLifecycleLimitMs", "comparison"} {
			if ev[k] != notObserved {
				t.Fatalf("%s = %v, want NOT_OBSERVED", k, ev[k])
			}
		}
		if ev := baselineEvidence(latencySample{}); ev["probe"] != "none" || ev["samples"] != 0 {
			t.Fatalf("absent baseline evidence = %v", ev)
		}
	})
	t.Run("AHI-172 live sampler spawns the fixed probe during the window", func(t *testing.T) {
		stop := sampleBaseline(context.Background())
		time.Sleep(time.Duration(baselineMinSamples+5) * baselineProbeInterval)
		b := stop()
		if b.probe == "none" || len(b.ms) < baselineMinSamples {
			t.Fatalf("live baseline = %s with %d samples", b.probe, len(b.ms))
		}
		if _, ok := b.slowdown(); !ok {
			t.Fatal("live baseline slowdown unobserved")
		}
		saved := baselineProbePaths
		t.Cleanup(func() { baselineProbePaths = saved })
		baselineProbePaths = []string{filepath.Join(t.TempDir(), "absent")}
		if b := sampleBaseline(context.Background())(); b.probe != "none" || len(b.ms) != 0 {
			t.Fatalf("absent probe sampled: %+v", b)
		}
	})
}

// TestAHI172ProcLoadavgFixture drives the Linux /proc/loadavg reader from a fixture file, so the
// path is exercised on every host; a live Linux reading is the AHI-171 live subtest.
func TestAHI172ProcLoadavgFixture(t *testing.T) {
	saved := procLoadavgPath
	t.Cleanup(func() { procLoadavgPath = saved })
	dir := t.TempDir()
	procLoadavgPath = filepath.Join(dir, "loadavg")
	if e := os.WriteFile(procLoadavgPath, []byte("11.52 9.80 7.01 13/1834 98765\n"), 0600); e != nil {
		t.Fatal(e)
	}
	c := observeContention()
	if c.source != "/proc/loadavg" || !c.loadOK || c.load != [3]float64{11.52, 9.80, 7.01} || !c.runQueueOK || c.runQueue != 13 {
		t.Fatalf("fixture reading = %+v", c)
	}
	if ev := c.evidence(); ev["loadAverage1m"] != 11.52 || ev["runQueue"] != 13 || ev["source"] != "/proc/loadavg" {
		t.Fatalf("fixture evidence = %v", ev)
	}
	for _, bad := range []string{"11.52 9.80\n", "x 9.80 7.01 13/1834 1\n", "-1 9.80 7.01 13/1834 1\n"} {
		if e := os.WriteFile(procLoadavgPath, []byte(bad), 0600); e != nil {
			t.Fatal(e)
		}
		if c := observeContention(); c.loadOK || c.runQueueOK || c.evidence()["loadAverage1m"] != notObserved {
			t.Fatalf("malformed %q read as %+v", bad, c)
		}
	}
	if e := os.WriteFile(procLoadavgPath, []byte("1.00 2.00 3.00 x/1834 1\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if c := observeContention(); !c.loadOK || c.runQueueOK || c.evidence()["runQueue"] != notObserved {
		t.Fatalf("unparsed run queue recorded: %+v", c)
	}
}
