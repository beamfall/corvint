package opencodequalification

import (
	"strings"
	"testing"
)

// contendedAt is a decisive re-sample with query p95 300 ms, the given lifecycle p95, a 1-minute
// load of load1 on 10 logical CPUs and a no-op baseline at slowdown times its 2 ms quiet reference.
func contendedAt(lifecycleMs, load1, slowdown float64) latencySample {
	s, _ := timingSample(latencyRows(series(30, 0, 300, 0), series(30, 0, lifecycleMs, 0)))
	s.start, s.end, s.baseline = loadAt(load1, 10), loadAt(load1, 10), probeAt(baselineQuietP95Ms*slowdown)
	return s
}

// TestAHI372ContendedQualification drives the GitHub #1982 shapes: a 10-CPU host at 1-minute load
// 11 to 17 whose no-op baseline ran 7.7 to 10 times its quiet reference while lifecycle p95 was
// 257 to 296 ms against the absolute 250 ms limit.
func TestAHI372ContendedQualification(t *testing.T) {
	allPass := func() Object { return Object{"latency": false, "native-edit": true, "timed-delivery": true} }
	t.Run("AHI-372 issue shapes within the capped scaled limits qualify under contention", func(t *testing.T) {
		for _, c := range []struct{ lifecycle, load, slowdown float64 }{{257, 11, 7.7}, {296, 17, 10}} {
			s := contendedAt(c.lifecycle, c.load, c.slowdown)
			if got := latencyOutcome(s); got != latencyOutcomeQualifiedContended {
				t.Fatalf("%v: outcome = %s", c, got)
			}
			if !latencyOnlyQualifiedContended(allPass(), s) || latencyOnlyRetry(allPass(), s) {
				t.Fatalf("%v: not a contended qualification", c)
			}
			ev := contendedQualificationEvidence(s)
			if ev["latencyQualification"] != "CONTENDED" || ev["lifecycleP95Ms"] != c.lifecycle || ev["queryP95Ms"] != 300.0 || ev["baselineSlowdown"] != c.slowdown || ev["maxScaleFactor"] != contendedMaxScale || ev["scaleFactor"] != contendedMaxScale || ev["scaledLifecycleLimitMs"] != 375.0 || ev["scaledQueryLimitMs"] != 750.0 || ev["peakLoadPerCPU"] != c.load/10 {
				t.Fatalf("%v: evidence = %v", c, ev)
			}
		}
	})
	t.Run("AHI-372 a miss beyond the cap but within the baseline-scaled limits stays retry-later", func(t *testing.T) {
		// 400 ms is beyond 250 x 1.5 but within 250 x 7.7 x 1.5, so the host may explain it.
		s := contendedAt(400, 11, 7.7)
		if got := latencyOutcome(s); got != latencyOutcomeContended || !latencyOnlyRetry(allPass(), s) || latencyOnlyQualifiedContended(allPass(), s) {
			t.Fatalf("outcome = %s", got)
		}
	})
	t.Run("AHI-372 a miss beyond the baseline-scaled limits stays a regression", func(t *testing.T) {
		// 3000 ms is beyond 250 x 7.7 x 1.5 = 2887.5 ms.
		s := contendedAt(3000, 11, 7.7)
		if got := latencyOutcome(s); got != latencyOutcomeRegressed || latencyOnlyQualifiedContended(allPass(), s) {
			t.Fatalf("outcome = %s", got)
		}
	})
	t.Run("AHI-372 a slowdown below the cap scales by the slowdown itself", func(t *testing.T) {
		if got := latencyOutcome(contendedAt(270, 11, 1.1)); got != latencyOutcomeQualifiedContended {
			t.Fatalf("270 ms at 1.1x outcome = %s", got)
		}
		if got := latencyOutcome(contendedAt(280, 11, 1.1)); got != latencyOutcomeContended {
			t.Fatalf("280 ms at 1.1x outcome = %s", got)
		}
	})
	t.Run("AHI-372 a quiet host, an unobserved baseline or another failed check never qualifies", func(t *testing.T) {
		if got := latencyOutcome(contendedAt(257, 2, 7.7)); got != latencyOutcomeRegressed {
			t.Fatalf("quiet host outcome = %s", got)
		}
		short := contendedAt(257, 11, 7.7)
		short.baseline.ms = short.baseline.ms[:baselineMinSamples-1]
		if got := latencyOutcome(short); got != latencyOutcomeContended {
			t.Fatalf("unobserved baseline outcome = %s", got)
		}
		checks := allPass()
		checks["native-edit"] = false
		if latencyOnlyQualifiedContended(checks, contendedAt(257, 11, 7.7)) {
			t.Fatal("contention qualified a non-latency failure")
		}
	})
	t.Run("AHI-372 the consumer record distinguishes contended from quiet qualification", func(t *testing.T) {
		contendedRecord := func(t *testing.T, lifecycleMs float64) (Object, Object, Identity) {
			n, f, id := recordFixture(t)
			s := contendedAt(lifecycleMs, 11, 7.7)
			m := object(n["metrics"])
			life := []any{}
			for _, v := range s.lifecycle {
				life = append(life, v)
			}
			m["lifecycle"], m["lifecycleP95Ms"] = life, p95(s.lifecycle)
			b, _ := jsonBytes(Object{"baseline": baselineEvidence(s), "qualification": contendedQualificationEvidence(s)})
			d, _ := decode(b)
			m["latencyBaseline"], m["latencyOutcome"] = d["baseline"], latencyOutcome(s)
			n["contendedQualification"], n["result"] = d["qualification"], latencyOutcomeQualifiedContended
			object(n["checks"])["latency"] = false
			return n, f, id
		}
		n, f, id := contendedRecord(t, 257)
		r, e := BuildRecord(n, f, id, id)
		if e != nil {
			t.Fatal(e)
		}
		if r["result"] != latencyOutcomeQualifiedContended || object(r["conformance"])["latency"] != latencyOutcomeQualifiedContended || object(r["contendedQualification"])["latencyQualification"] != "CONTENDED" {
			t.Fatalf("record = %v / %v", r["result"], r["conformance"])
		}
		for name, mutate := range map[string]func(Object){
			"beyond the capped limit": func(n Object) {
				m := object(n["metrics"])
				life := array(m["lifecycle"])
				for i := range life {
					life[i] = 400.0
				}
				m["lifecycleP95Ms"] = 400.0
			},
			"inflated scale":           func(n Object) { object(n["contendedQualification"])["scaleFactor"] = 7.7 },
			"uncapped declared max":    func(n Object) { object(n["contendedQualification"])["maxScaleFactor"] = 10.0 },
			"unobserved baseline":      func(n Object) { object(object(n["metrics"])["latencyBaseline"])["slowdown"] = notObserved },
			"quiet host":               func(n Object) { object(n["contendedQualification"])["peakLoadPerCPU"] = 0.2 },
			"peak load missing":        func(n Object) { delete(object(n["contendedQualification"]), "peakLoadPerCPU") },
			"peak load not observed":   func(n Object) { object(n["contendedQualification"])["peakLoadPerCPU"] = notObserved },
			"another failed check":     func(n Object) { object(n["checks"])["native-edit"] = false },
			"pass claimed on the miss": func(n Object) { n["result"] = "PASS" },
		} {
			n, f, id := contendedRecord(t, 257)
			mutate(n)
			if _, e := BuildRecord(n, f, id, id); e == nil {
				t.Fatalf("%s: contended record accepted", name)
			}
		}
		// A quiet sample cannot be relabelled as a contended qualification.
		n, f, id = recordFixture(t)
		n["result"] = latencyOutcomeQualifiedContended
		if _, e := BuildRecord(n, f, id, id); e == nil || !strings.Contains(e.Error(), "contended qualification") {
			t.Fatalf("relabelled quiet record: %v", e)
		}
		// The quiet record is unchanged: PASS with conformance PASS and no contended evidence.
		n, f, id = recordFixture(t)
		if r, e := BuildRecord(n, f, id, id); e != nil || r["result"] != "PASS" || object(r["conformance"])["latency"] != "PASS" || r["contendedQualification"] != nil {
			t.Fatalf("quiet record = %v, %v", r["result"], e)
		}
	})
}
