package postmergemetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func ptr(n int64) *int64 { return &n }
func hour(n int) string {
	return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(n) * time.Hour).Format(time.RFC3339Nano)
}

type zeroMeasure struct{ calls int }

func (m *zeroMeasure) Measure(_ context.Context, g GitPair) (Measurement, error) {
	m.calls++
	return Measurement{Bot: g.Bot, Approved: g.Approved, BotTree: strings.Repeat("c", 40), ApprovedTree: strings.Repeat("d", 40), GitVersion: "git fixture", Convention: "no-renames-delete-plus-add", Added: ptr(0), Deleted: ptr(0)}, nil
}
func fixture(n int) (Policy, History) {
	p := Policy{Profile: "postmerge-policy/0", Classes: []Class{{ID: "prose", MaxCorrectionBP: 10000, MinSamples: 1, DemoteRuns: 2}}}
	h := History{Profile: "postmerge-history/0", Classes: []Inventory{{Class: "prose", LastSequence: n, RunsComplete: true, EventsComplete: true, RunsThrough: hour(20000), EventsThrough: hour(20000)}}, Runs: []Run{}, Events: []Event{}}
	for i := 1; i <= n; i++ {
		h.Runs = append(h.Runs, Run{ID: string(rune('a' + i - 1)), Class: "prose", Sequence: i, At: hour(i), Outcome: "generated", Stages: []Stage{{Name: "build", Outcome: "passed", DurationMS: ptr(10)}}, Followup: Followup{Status: "created", DurationMS: ptr(20)}, Git: &GitPair{Root: "/repo", Bot: strings.Repeat("a", 40), Approved: strings.Repeat("b", 40)}})
	}
	return p, h
}
func buildFixture(t *testing.T, p Policy, h History, from, until string) Report {
	t.Helper()
	r, e := Build(context.Background(), p, h, from, until, &zeroMeasure{})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func has(xs []string, w string) bool {
	for _, x := range xs {
		if x == w {
			return true
		}
	}
	return false
}
func TestMetricsChronology(t *testing.T) {
	t.Run("PMM-V0-004 late-revert-starts-at-event", func(t *testing.T) {
		p, h := fixture(5)
		h.Events = []Event{{ID: "late", RunID: "a", At: hour(4), Kind: "revert"}}
		r := buildFixture(t, p, h, hour(0), hour(6))
		c := r.Classes[0]
		if c.RemainingDemotion != 0 || c.Demoted != 3 || c.Corrected != 1 || !r.Runs[3].Demoted || !r.Runs[4].Demoted || r.Runs[1].Demoted {
			t.Fatalf("%+v %+v", c, r.Runs)
		}
	})
	t.Run("PMM-V0-004 out-of-window-and-repeat", func(t *testing.T) {
		p, h := fixture(5)
		h.Events = []Event{{ID: "old", RunID: "a", At: hour(3), Kind: "revert"}, {ID: "again", RunID: "a", At: hour(5), Kind: "revert"}}
		r := buildFixture(t, p, h, hour(4), hour(6))
		c := r.Classes[0]
		if c.RemainingDemotion != 1 || c.Recommendation != "demoted" || c.Demoted != 2 || c.Corrected != 0 || c.RevertEvents != 1 {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("PMM-V0-004 exclusive-event-cutoff", func(t *testing.T) {
		p, h := fixture(3)
		h.Events = []Event{{ID: "future", RunID: "a", At: hour(4), Kind: "revert"}}
		r := buildFixture(t, p, h, hour(1), hour(4))
		if r.Classes[0].Corrected != 0 || r.Classes[0].RemainingDemotion != 0 || r.Classes[0].Generated != 3 {
			t.Fatal(r.Classes)
		}
	})
	t.Run("PMM-V0-004 old-event-before-window", func(t *testing.T) {
		p, h := fixture(3)
		h.Events = []Event{{ID: "old", RunID: "a", At: hour(2), Kind: "revert"}}
		r := buildFixture(t, p, h, hour(3), hour(4))
		c := r.Classes[0]
		if c.Demoted != 1 || c.RevertEvents != 0 || c.RemainingDemotion != 0 {
			t.Fatal(c)
		}
	})
	t.Run("PMM-V0-004 equal-time-events-before-run", func(t *testing.T) {
		p, h := fixture(1)
		h.Events = []Event{{ID: "tie", RunID: "a", At: hour(1), Kind: "revert"}}
		r := buildFixture(t, p, h, hour(0), hour(2))
		if r.Classes[0].RemainingDemotion != 1 || r.Classes[0].Demoted != 1 {
			t.Fatal(r.Classes)
		}
	})
	t.Run("PMM-V0-004 class-independence", func(t *testing.T) {
		p, h := fixture(1)
		p.Classes = append(p.Classes, Class{ID: "anchors", MinSamples: 1, DemoteRuns: 2})
		h.Classes = append(h.Classes, Inventory{Class: "anchors", LastSequence: 1, RunsComplete: true, EventsComplete: true, RunsThrough: hour(9), EventsThrough: hour(9)})
		r := h.Runs[0]
		r.ID = "anchor"
		r.Class = "anchors"
		h.Runs = append(h.Runs, r)
		h.Events = []Event{{ID: "revert", RunID: "a", At: hour(2), Kind: "revert"}}
		report := buildFixture(t, p, h, hour(0), hour(3))
		if report.Classes[0].Recommendation != "eligible-for-owner-consideration" || report.Classes[1].Recommendation != "demoted" {
			t.Fatal(report.Classes)
		}
	})
}
func TestMetricsCompleteness(t *testing.T) {
	t.Run("PMM-V0-005 completeness-boundaries", func(t *testing.T) {
		cases := map[string]func(*History){"missing-middle": func(h *History) { h.Runs = append(h.Runs[:1], h.Runs[2:]...) }, "missing-prefix": func(h *History) { h.Runs = h.Runs[1:] }, "missing-inventory": func(h *History) { h.Classes = nil }, "partial-runs": func(h *History) { h.Classes[0].RunsComplete = false }, "partial-events": func(h *History) { h.Classes[0].EventsComplete = false }, "run-watermark": func(h *History) { h.Classes[0].RunsThrough = hour(2) }, "event-watermark": func(h *History) { h.Classes[0].EventsThrough = hour(1) }, "future-row-watermark": func(h *History) { h.Classes[0].RunsThrough = hour(2); h.Runs[2].At = hour(6) }}
		for name, edit := range cases {
			t.Run("PMM-V0-005-"+name, func(t *testing.T) {
				p, h := fixture(3)
				edit(&h)
				r := buildFixture(t, p, h, hour(0), hour(2))
				if r.Classes[0].HistoryComplete || !has(r.Classes[0].Reasons, "history-incomplete") {
					t.Fatal(r.Classes)
				}
			})
		}
	})
}
func TestMetricsRatesAndUnknown(t *testing.T) {
	t.Run("PMM-V0-003 distinct-numerator-exact-threshold", func(t *testing.T) {
		p, h := fixture(2)
		p.Classes[0].MaxCorrectionBP = 5000
		h.Events = []Event{{ID: "c1", RunID: "a", At: hour(3), Kind: "correction"}, {ID: "c2", RunID: "a", At: hour(3), Kind: "correction"}}
		r := buildFixture(t, p, h, hour(0), hour(4))
		c := r.Classes[0]
		if c.Corrected != 1 || c.Generated != 2 || c.CorrectionEvents != 2 || c.Recommendation != "eligible-for-owner-consideration" {
			t.Fatal(c)
		}
		p.Classes[0].MaxCorrectionBP = 4999
		r = buildFixture(t, p, h, hour(0), hour(4))
		if !has(r.Classes[0].Reasons, "correction-threshold") {
			t.Fatal(r.Classes)
		}
	})
	t.Run("PMM-V0-003 separate-events-and-noop", func(t *testing.T) {
		p, h := fixture(2)
		h.Runs[1].Outcome = "no-change"
		h.Runs[1].Git = nil
		h.Runs[1].Followup = Followup{Status: "no-op", DurationMS: ptr(0)}
		h.Events = []Event{{ID: "invalid", RunID: "b", At: hour(3), Kind: "invalid-finding"}, {ID: "guard", RunID: "a", At: hour(3), Kind: "containment"}}
		r := buildFixture(t, p, h, hour(0), hour(4))
		c := r.Classes[0]
		if c.Generated != 1 || c.NoChange != 1 || c.InvalidFindingEvents != 1 || c.ContainmentEvents != 1 || c.Corrected != 0 || !has(c.Reasons, "adverse-event") {
			t.Fatal(c)
		}
	})
	for _, kind := range []string{"missing-git", "unknown-stage", "unknown-followup", "failed-run", "no-stages", "minimum"} {
		t.Run("PMM-V0-003-"+kind, func(t *testing.T) {
			p, h := fixture(1)
			switch kind {
			case "missing-git":
				h.Runs[0].Git = nil
			case "unknown-stage":
				h.Runs[0].Stages[0].Outcome = "unknown"
				h.Runs[0].Stages[0].DurationMS = nil
			case "unknown-followup":
				h.Runs[0].Followup = Followup{Status: "unknown"}
			case "failed-run":
				h.Runs[0].Outcome = "failed"
				h.Runs[0].Git = nil
			case "no-stages":
				h.Runs[0].Stages = nil
			case "minimum":
				p.Classes[0].MinSamples = 2
			}
			r := buildFixture(t, p, h, hour(0), hour(2))
			if r.Classes[0].Recommendation != "review" {
				t.Fatal(r.Classes)
			}
		})
	}
}
func TestMetricsReproducible(t *testing.T) {
	t.Run("PMM-V0-006 canonical-reproducible", func(t *testing.T) {
		p, h := fixture(3)
		h.Runs[0].Stages = append(h.Runs[0].Stages, Stage{Name: "alpha", Outcome: "passed", DurationMS: ptr(0)})
		m := &zeroMeasure{}
		r, e := Build(context.Background(), p, h, hour(0), hour(4), m)
		if e != nil || m.calls != 1 {
			t.Fatalf("%v calls=%d", e, m.calls)
		}
		a, _ := json.Marshal(r)
		h.Runs[0], h.Runs[2] = h.Runs[2], h.Runs[0]
		h.Runs[2].Stages[0], h.Runs[2].Stages[1] = h.Runs[2].Stages[1], h.Runs[2].Stages[0]
		h.Runs[0].At = strings.TrimSuffix(h.Runs[0].At, "Z") + ".000Z"
		b, _ := json.Marshal(buildFixture(t, p, h, hour(0), hour(4)))
		if !bytes.Equal(a, b) {
			t.Fatalf("noncanonical report\n%s\n%s", a, b)
		}
		if r.Authority != "none" || r.Classes[0].Recommendation != "eligible-for-owner-consideration" {
			t.Fatal(r)
		}

		if len(r.PolicySHA256) != 64 || len(r.HistorySHA256) != 64 {
			t.Fatal("missing content digests", r)
		}
		changedPolicy := p
		changedPolicy.Classes = append([]Class(nil), p.Classes...)
		changedPolicy.Classes[0].MaxCorrectionBP--
		policyReport := buildFixture(t, changedPolicy, h, hour(0), hour(4))
		if policyReport.PolicySHA256 == r.PolicySHA256 || policyReport.HistorySHA256 != r.HistorySHA256 {
			t.Fatal("policy digest binding", policyReport)
		}
		h.Runs[0].Followup.DurationMS = ptr(21)
		historyReport := buildFixture(t, p, h, hour(0), hour(4))
		if historyReport.HistorySHA256 == r.HistorySHA256 || historyReport.PolicySHA256 != r.PolicySHA256 {
			t.Fatal("history digest binding", historyReport)
		}
	})
}
func TestMetricsMalformed(t *testing.T) {
	t.Run("PMM-V0-001 malformed-history", func(t *testing.T) {
		for name, edit := range map[string]func(*History){"duplicate-run": func(h *History) { h.Runs = append(h.Runs, h.Runs[0]) }, "sequence-order": func(h *History) { h.Runs[0].Sequence = 2; h.Runs[1].Sequence = 1 }, "unknown-class": func(h *History) { h.Runs[0].Class = "absent" }, "early-event": func(h *History) { h.Events = []Event{{ID: "bad", RunID: "a", At: hour(0), Kind: "revert"}} }, "dangling": func(h *History) { h.Events = []Event{{ID: "bad", RunID: "absent", At: hour(3), Kind: "revert"}} }, "negative-duration": func(h *History) { h.Runs[0].Stages[0].DurationMS = ptr(-1) }, "offset": func(h *History) { h.Runs[0].At = "2026-09-01T00:00:00+00:00" }} {
			t.Run("PMM-V0-001-"+name, func(t *testing.T) {
				p, h := fixture(2)
				edit(&h)
				_, e := Build(context.Background(), p, h, hour(0), hour(4), &zeroMeasure{})
				if e != ErrHistory {
					t.Fatal(e)
				}
			})
		}
	})
	t.Run("PMM-V0-001 closed-wire", func(t *testing.T) {
		p, h := fixture(1)
		b, _ := json.Marshal(p)
		for _, raw := range [][]byte{append(append([]byte{}, b...), []byte("{}")...), bytes.Replace(b, []byte(`"minSamples":1`), []byte(`"minSamples":1,"minSamples":2`), 1), bytes.Replace(b, []byte(`"minSamples":1`), []byte(`"minSamples":null`), 1), bytes.Replace(b, []byte(`"minSamples":1,`), nil, 1), bytes.Replace(b, []byte(`"profile"`), []byte(`"Profile"`), 1), bytes.Replace(b, []byte(`"classes"`), []byte(`"unknown"`), 1), bytes.Repeat([]byte(" "), PolicyLimit+1)} {
			if _, e := ParsePolicy(raw); e == nil {
				t.Fatalf("accepted %s", raw)
			}
		}
		hb, _ := json.Marshal(h)
		if _, e := ParseHistory(hb); e != nil {
			t.Fatal(e)
		}
		if _, e := ParseHistory(bytes.Replace(hb, []byte(`"stages":[`), []byte(`"stages":null,"extra":[`), 1)); e == nil {
			t.Fatal("null/extra accepted")
		}
		var out Policy
		if decode([]byte(strings.Repeat("[", 34)+"0"+strings.Repeat("]", 34)), PolicyLimit, &out) == nil {
			t.Fatal("depth accepted")
		}
	})
	t.Run("PMM-V0-007 cancellation", func(t *testing.T) {
		p, h := fixture(1)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := Build(ctx, p, h, hour(0), hour(2), &zeroMeasure{}); e != ErrCancelled {
			t.Fatal(e)
		}
	})
}

func TestMetricsInventoryWitness(t *testing.T) {
	t.Run("PMM-V0-005 independent-inventory-assertions", func(t *testing.T) {
		p, h := fixture(3)
		good := buildFixture(t, p, h, hour(0), hour(4))
		if !good.Classes[0].HistoryComplete {
			t.Fatal(good.Classes)
		}
		h.Classes[0].EventsComplete = false
		bad := buildFixture(t, p, h, hour(0), hour(4))
		if bad.Classes[0].HistoryComplete || bad.Classes[0].Recommendation != "review" {
			t.Fatal(bad.Classes)
		}
		h.Classes[0].EventsComplete = true
		h.Runs = append(h.Runs[:1], h.Runs[2:]...)
		bad = buildFixture(t, p, h, hour(0), hour(4))
		if bad.Classes[0].HistoryComplete {
			t.Fatal("missing sequence accepted")
		}
	})
}
func TestMetricsPolicyBounds(t *testing.T) {
	t.Run("PMM-V0-001 policy-range-validation", func(t *testing.T) {
		for _, change := range []func(*Policy){func(p *Policy) { p.Classes[0].MaxCorrectionBP = 10001 }, func(p *Policy) { p.Classes[0].MinSamples = 0 }, func(p *Policy) { p.Classes[0].DemoteRuns = -1 }, func(p *Policy) { p.Classes = append(p.Classes, p.Classes[0]) }, func(p *Policy) { p.Classes[0].ID = "bad space" }} {
			p, _ := fixture(1)
			change(&p)
			b, _ := json.Marshal(p)
			if _, e := ParsePolicy(b); e != ErrPolicy {
				t.Fatalf("%s: %v", b, e)
			}
		}
	})
}

func TestMetricsRevertedCohort(t *testing.T) {
	t.Run("PMM-V0-004 reverted-cohort-after-cooldown", func(t *testing.T) {
		for _, n := range []int{0, 2} {
			p, h := fixture(4)
			p.Classes[0].DemoteRuns = n
			h.Events = []Event{{ID: "revert", RunID: "a", At: hour(2), Kind: "revert"}}
			r := buildFixture(t, p, h, hour(0), hour(5))
			c := r.Classes[0]
			if c.RemainingDemotion != 0 || c.Recommendation != "review" || !has(c.Reasons, "reverted-cohort") {
				t.Fatalf("N=%d %+v", n, c)
			}
			later := buildFixture(t, p, h, hour(3), hour(5))
			if later.Classes[0].Recommendation != "eligible-for-owner-consideration" {
				t.Fatal(later.Classes)
			}
		}
	})
}

func TestMetricsRetainedFacts(t *testing.T) {
	t.Run("PMM-V0-002 retained-stages-followup-and-identities", func(t *testing.T) {
		p, h := fixture(3)
		stages := []Stage{{Name: "a-pass", Outcome: "passed", DurationMS: ptr(10)}, {Name: "b-fail", Outcome: "failed", DurationMS: ptr(30)}, {Name: "c-skip", Outcome: "skipped", DurationMS: ptr(0)}, {Name: "d-unknown", Outcome: "unknown"}}
		followups := []Followup{{Status: "created", DurationMS: ptr(25)}, {Status: "no-op", DurationMS: ptr(0)}, {Status: "unknown"}}
		for i := range h.Runs {
			h.Runs[i].Stages = stages
			h.Runs[i].Followup = followups[i]
		}
		r := buildFixture(t, p, h, hour(0), hour(4))
		if len(r.Runs) != 3 || r.Authority != "none" || !strings.Contains(r.Provenance, "not-authenticated-authorship") {
			t.Fatal(r)
		}
		for i, got := range r.Runs {
			if !reflect.DeepEqual(got.Run.Stages, stages) || !reflect.DeepEqual(got.Run.Followup, followups[i]) || !reflect.DeepEqual(got.Run.Git, h.Runs[i].Git) || got.Measurement.Bot != h.Runs[i].Git.Bot || got.Measurement.Approved != h.Runs[i].Git.Approved {
				t.Fatalf("run %d lost facts: %+v", i, got)
			}
		}
	})
}

type fixtureMeasurements struct {
	values map[GitPair]Measurement
	seen   []GitPair
}

func (m *fixtureMeasurements) Measure(_ context.Context, g GitPair) (Measurement, error) {
	m.seen = append(m.seen, g)
	v, ok := m.values[g]
	if !ok {
		return Measurement{}, ErrGit
	}
	return v, nil
}
func TestMetricsEditAggregates(t *testing.T) {
	t.Run("PMM-V0-003 cohort-edit-totals-with-unknown-lines", func(t *testing.T) {
		for _, kind := range []string{"binary", "gitlink"} {
			p, h := fixture(5)
			for i := range h.Runs {
				h.Runs[i].Git.Bot = strings.Repeat(string(rune('1'+i)), 40)
			}
			h.Runs[3].Git = nil
			b, c := *h.Runs[1].Git, *h.Runs[2].Git
			known := Measurement{Bot: b.Bot, Approved: b.Approved, BotTree: strings.Repeat("c", 40), ApprovedTree: strings.Repeat("d", 40), GitVersion: "git fixture", Convention: "no-renames-delete-plus-add", Files: 2, TextAdded: 3, TextDeleted: 5, Added: ptr(3), Deleted: ptr(5)}
			partial := known
			partial.Bot = c.Bot
			partial.Files = 4
			partial.TextAdded = 7
			partial.TextDeleted = 11
			partial.Added = nil
			partial.Deleted = nil
			if kind == "binary" {
				partial.BinaryFiles = 1
			} else {
				partial.GitlinkFiles = 1
			}
			m := &fixtureMeasurements{values: map[GitPair]Measurement{b: known, c: partial}}
			r, e := Build(context.Background(), p, h, hour(2), hour(5), m)
			if e != nil {
				t.Fatal(e)
			}
			got := r.Classes[0]
			if got.Total != 3 || got.Generated != 3 || got.EditedFiles != 6 || got.TextAdded != 10 || got.TextDeleted != 16 || got.UnknownEditRuns != 2 || got.Recommendation != "review" || !reflect.DeepEqual(m.seen, []GitPair{b, c}) {
				t.Fatalf("%s: %+v seen=%+v", kind, got, m.seen)
			}
			if len(r.Runs) != 3 || r.Runs[0].Run.ID != "b" || r.Runs[2].Run.ID != "d" {
				t.Fatal(r.Runs)
			}
		}
	})
}
