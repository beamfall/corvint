package opencodequalification

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func loadAt(load1 float64, cpus int) contentionSnapshot {
	return contentionSnapshot{load: [3]float64{load1, load1, load1}, loadOK: true, runQueue: 3, runQueueOK: true, cpus: cpus, source: "test"}
}

func TestAHI171LatencyHostContention(t *testing.T) {
	slow := func(start, end contentionSnapshot) latencySample {
		s, _ := timingSample(latencyRows(series(30, 0, 300, 0), series(30, 30, 0, 289)))
		s.start, s.end = start, end
		return s
	}
	allPass := func() Object {
		return Object{"latency": false, "native-edit": true, "native-query": true, "timed-delivery": true}
	}
	t.Run("AHI-171 latency-only miss under recorded contention is retry-later", func(t *testing.T) {
		// AHI-171: load 11.5 on 12 CPUs exceeds the declared 0.5 per CPU, so the miss is the host's.
		s := slow(loadAt(6, 12), loadAt(11.5, 12))
		if got := latencyOutcome(s); got != latencyOutcomeContended {
			t.Fatalf("outcome = %s", got)
		}
		if !latencyOnlyRetry(allPass(), s) {
			t.Fatal("latency-only contended miss not retry-later")
		}
		if hint := retryHint(s); !strings.Contains(hint, "below 6.0 (0.50 x 12 logical CPUs)") || !strings.Contains(hint, "peaked at 0.96") {
			t.Fatalf("hint = %q", hint)
		}
		ev := latencyAttemptEvidence([]latencySample{s})[0]
		end := ev["contention"].(Object)["end"].(Object)
		if ev["outcome"] != latencyOutcomeContended || end["loadAverage1m"] != 11.5 || end["logicalCPUs"] != 12 || end["runQueue"] != 3 {
			t.Fatalf("attempt evidence = %v", ev)
		}
		var err error = &RetryLaterError{Message: "x"}
		if !IsRetryLater(err) || IsRetryLater(errors.New("x")) {
			t.Fatal("retry-later error not typed")
		}
	})
	t.Run("AHI-171 the same miss on a quiet host is a latency regression", func(t *testing.T) {
		s := slow(loadAt(1, 12), loadAt(2, 12))
		if got := latencyOutcome(s); got != latencyOutcomeRegressed || latencyOnlyRetry(allPass(), s) {
			t.Fatalf("quiet-host miss outcome = %s", got)
		}
	})
	t.Run("AHI-171 unobserved contention never excuses a miss", func(t *testing.T) {
		s := slow(contentionSnapshot{cpus: 12, source: "none"}, contentionSnapshot{cpus: 12, source: "none"})
		if got := latencyOutcome(s); got != latencyOutcomeRegressed {
			t.Fatalf("unobserved outcome = %s", got)
		}
		ev := s.end.evidence()
		for _, k := range []string{"loadAverage1m", "loadAverage5m", "loadAverage15m", "runQueue"} {
			if ev[k] != notObserved {
				t.Fatalf("%s = %v, want NOT_OBSERVED", k, ev[k])
			}
		}
		if (contentionSnapshot{}).evidence()["logicalCPUs"] != notObserved {
			t.Fatal("absent CPU count recorded as a value")
		}
	})
	t.Run("AHI-171 another failed check or a short sample is not retry-later", func(t *testing.T) {
		s := slow(loadAt(12, 12), loadAt(12, 12))
		checks := allPass()
		checks["native-edit"] = false
		if latencyOnlyRetry(checks, s) {
			t.Fatal("contention excused a non-latency failure")
		}
		short, _ := timingSample(latencyRows(series(10, 0, 100, 0), series(30, 0, 100, 0)))
		short.start, short.end = loadAt(12, 12), loadAt(12, 12)
		if latencyOutcome(short) != latencyOutcomeRegressed {
			t.Fatal("short sample excused by contention")
		}
		calm, _ := timingSample(latencyRows(series(30, 0, 300, 0), series(30, 0, 200, 0)))
		calm.start, calm.end = loadAt(12, 12), loadAt(12, 12)
		if latencyOutcome(calm) != latencyOutcomePass {
			t.Fatal("within-limit sample not PASS")
		}
	})
	t.Run("AHI-171 live host reading", func(t *testing.T) {
		c := observeContention()
		t.Logf("live %s reading: %v", runtime.GOOS, c.evidence())
		if c.cpus != runtime.NumCPU() {
			t.Fatalf("cpus = %d", c.cpus)
		}
		if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && !c.loadOK {
			t.Fatalf("load average unobserved on %s: %+v", runtime.GOOS, c)
		}
		if runtime.GOOS == "linux" && !c.runQueueOK {
			t.Fatalf("run queue unobserved on linux: %+v", c)
		}
	})
}
