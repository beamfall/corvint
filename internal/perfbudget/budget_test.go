package perfbudget

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func float(v float64) *float64 { return &v }

// PRB-V0-031, PRB-V0-032, PRB-V0-033: the committed budgets file bounds exactly the six
// scenarios, encodes the GitHub #1598 and #1601 targets at their stated numbers as pending
// (never loosened), enforces a ceiling on wall time and peak resident set for every scenario,
// and carries a baseline with its host-contention record.
func TestPRBV0031CommittedBudgets(t *testing.T) {
	b, err := LoadBudgets("budgets.json")
	if err != nil {
		t.Fatal(err)
	}
	if b.Scale != Full.Name {
		t.Fatalf("committed budgets measure scale %q, want full", b.Scale)
	}
	want := map[string]struct {
		limit     float64
		pendingOn string
	}{
		"cli-cold-read":  {1000, "GH #1598"},
		"cli-warm-read":  {200, "GH #1598"},
		"mcp-first-call": {1000, "GH #1598"},
		"mcp-warm-call":  {100, "GH #1598"},
		"affected-path":  {1000, "GH #1601"},
	}
	for _, s := range b.Scenarios {
		target, hasTarget := want[s.ID]
		switch {
		case hasTarget && (len(s.Targets) != 1 || s.Targets[0].Metric != MetricP95Wall || s.Targets[0].Limit != target.limit || s.Targets[0].Status != StatusPending || s.Targets[0].PendingOn != target.pendingOn):
			t.Errorf("%s targets %+v, want p95 %v ms pending on %s", s.ID, s.Targets, target.limit, target.pendingOn)
		case !hasTarget && len(s.Targets) != 0:
			t.Errorf("%s has targets %+v no issue states", s.ID, s.Targets)
		}
		metrics := []string{}
		for _, c := range s.Ceilings {
			metrics = append(metrics, c.Metric)
		}
		if !slices.Contains(metrics, MetricP95Wall) || !slices.Contains(metrics, MetricPeakRSS) {
			t.Errorf("%s ceilings %v, want p95 wall and peak RSS", s.ID, metrics)
		}
	}
	if b.Baseline == nil || b.Baseline.Host.Cores < 1 || b.Baseline.Host.LoadBefore == nil || b.Baseline.Host.Contended == nil || b.Baseline.Fixture == nil || len(b.Baseline.Scenarios) != len(ScenarioIDs) {
		t.Fatalf("baseline lacks its host-contention record or measurements: %+v", b.Baseline)
	}
	if b.Baseline.Fixture.Inputs < 10000 || b.Baseline.Fixture.Records < 90000 || b.Baseline.Fixture.Scale.AdditionalRepositories < 3 {
		t.Fatalf("baseline fixture is not realistic size: %+v", b.Baseline.Fixture)
	}
	for _, m := range b.Baseline.Scenarios {
		if m.Error != "" || m.P95WallMS == nil || m.PeakRSSBytes == nil {
			t.Fatalf("baseline scenario %s unmeasured: %+v", m.ID, m)
		}
	}
}

// PRB-V0-030: a budgets file that loosens structure is refused.
func TestPRBV0030BudgetsValidation(t *testing.T) {
	valid := func() *Budgets {
		b := &Budgets{Schema: BudgetSchema, Scale: "full"}
		for _, id := range ScenarioIDs {
			b.Scenarios = append(b.Scenarios, ScenarioBudget{ID: id, Samples: 3, Ceilings: []Limit{{Metric: MetricP95Wall, Limit: 1, Status: StatusEnforced, Reason: "r"}, {Metric: MetricPeakRSS, Limit: 1, Status: StatusEnforced, Reason: "r"}}})
		}
		return b
	}
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(*Budgets){
		"missing scenario": func(b *Budgets) { b.Scenarios = b.Scenarios[1:] },
		"pending ceiling": func(b *Budgets) {
			b.Scenarios[0].Ceilings[0].Status, b.Scenarios[0].Ceilings[0].PendingOn = StatusPending, "x"
		},
		"pending without why": func(b *Budgets) {
			b.Scenarios[0].Targets = []Limit{{Metric: MetricP95Wall, Limit: 1, Status: StatusPending, Reason: "r"}}
		},
		"zero limit":           func(b *Budgets) { b.Scenarios[0].Ceilings[0].Limit = 0 },
		"unknown metric":       func(b *Budgets) { b.Scenarios[0].Ceilings[0].Metric = "mean" },
		"no samples":           func(b *Budgets) { b.Scenarios[0].Samples = 0 },
		"no reason":            func(b *Budgets) { b.Scenarios[0].Ceilings[0].Reason = "" },
		"wrong schema":         func(b *Budgets) { b.Schema = "x" },
		"unknown scale":        func(b *Budgets) { b.Scale = "huge" },
		"enforced pending_on":  func(b *Budgets) { b.Scenarios[0].Ceilings[0].PendingOn = "x" },
		"reordered scenarios":  func(b *Budgets) { b.Scenarios[0], b.Scenarios[1] = b.Scenarios[1], b.Scenarios[0] },
		"too many samples":     func(b *Budgets) { b.Scenarios[0].Samples = 101 },
		"negative limit":       func(b *Budgets) { b.Scenarios[0].Ceilings[0].Limit = -1 },
		"unknown status value": func(b *Budgets) { b.Scenarios[0].Ceilings[0].Status = "advisory" },
		"no rss ceiling":       func(b *Budgets) { b.Scenarios[0].Ceilings = b.Scenarios[0].Ceilings[:1] },
	} {
		b := valid()
		edit(b)
		if err := b.Validate(); err == nil {
			t.Errorf("%s: admitted", name)
		}
	}
}

// PRB-V0-026: p50 and p95 are nearest-rank percentiles; no samples give no value.
func TestPRBV0026Percentiles(t *testing.T) {
	values := []float64{9, 1, 5, 3, 7, 2, 8, 4, 6, 10}
	if p := Percentile(values, 0.5); *p != 5 {
		t.Fatalf("p50 %v", *p)
	}
	if p := Percentile(values, 0.95); *p != 10 {
		t.Fatalf("p95 %v", *p)
	}
	if p := Percentile([]float64{42}, 0.95); *p != 42 {
		t.Fatalf("single p95 %v", *p)
	}
	if Percentile(nil, 0.5) != nil {
		t.Fatal("empty percentile invented a value")
	}
	m := ScenarioMeasured{Samples: []Sample{{WallMS: 1, PeakRSS: nil}, {WallMS: 2, PeakRSS: new(int64)}}}
	m.summarize()
	if m.PeakRSSBytes != nil || m.RSSUnknown == "" {
		t.Fatalf("a sample without RSS must leave peak RSS unknown with a reason: %+v", m)
	}
}

// PRB-V0-033, PRB-V0-034, PRB-V0-035: enforced limits fail when exceeded, unobserved, or their
// scenario failed; pending targets are reported and never fail.
func TestPRBV0034EvaluateVerdicts(t *testing.T) {
	b := &Budgets{Schema: BudgetSchema, Scale: "smoke"}
	for _, id := range ScenarioIDs {
		b.Scenarios = append(b.Scenarios, ScenarioBudget{ID: id, Samples: 1,
			Targets:  []Limit{{Metric: MetricP95Wall, Limit: 100, Status: StatusPending, PendingOn: "GH #1598", Reason: "r"}},
			Ceilings: []Limit{{Metric: MetricP95Wall, Limit: 1000, Status: StatusEnforced, Reason: "r"}, {Metric: MetricPeakRSS, Limit: 1 << 30, Status: StatusEnforced, Reason: "r"}}})
	}
	rss := int64(1 << 20)
	measuredAll := func(wall float64) []ScenarioMeasured {
		out := []ScenarioMeasured{}
		for _, id := range ScenarioIDs {
			out = append(out, ScenarioMeasured{ID: id, P50WallMS: float(wall), P95WallMS: float(wall), PeakRSSBytes: &rss})
		}
		return out
	}
	verdicts := func(checks []Check, scenario, kind, metric string) string {
		for _, c := range checks {
			if c.Scenario == scenario && c.Kind == kind && c.Limit.Metric == metric {
				return c.Verdict
			}
		}
		return ""
	}
	checks, ok := Evaluate(b, measuredAll(500))
	if !ok || verdicts(checks, "cli-warm-read", "target", MetricP95Wall) != VerdictPendingExceeded || verdicts(checks, "cli-warm-read", "ceiling", MetricP95Wall) != VerdictPass {
		t.Fatalf("pending target over its limit failed the run or ceiling misjudged: ok=%t %+v", ok, checks)
	}
	if checks, ok = Evaluate(b, measuredAll(50)); !ok || verdicts(checks, "cli-cold-read", "target", MetricP95Wall) != VerdictPendingPass {
		t.Fatal("pending-pass")
	}
	if checks, ok = Evaluate(b, measuredAll(5000)); ok || verdicts(checks, "full-delta", "ceiling", MetricP95Wall) != VerdictExceeded {
		t.Fatal("exceeded ceiling did not fail")
	}
	unobserved := measuredAll(50)
	unobserved[2].PeakRSSBytes, unobserved[2].RSSUnknown = nil, "NOT_OBSERVED"
	if checks, ok = Evaluate(b, unobserved); ok || verdicts(checks, "mcp-first-call", "ceiling", MetricPeakRSS) != VerdictUnknown {
		t.Fatal("unobserved enforced metric did not fail closed")
	}
	failed := measuredAll(50)
	failed[4] = ScenarioMeasured{ID: "full-delta", Error: "corpus-refused"}
	if checks, ok = Evaluate(b, failed); ok || verdicts(checks, "full-delta", "ceiling", MetricP95Wall) != VerdictUnknown || verdicts(checks, "full-delta", "target", MetricP95Wall) != VerdictPendingUnknown {
		t.Fatal("failed scenario did not fail the run")
	}
	if _, ok = Evaluate(b, measuredAll(50)[1:]); ok {
		t.Fatal("unmeasured scenario passed")
	}
}

// PRB-V0-038, PRB-V0-039, PRB-V0-040: a run is PASS or FAIL only on a host whose contention
// record shows it was not contended; a contended host or an unobserved load is NOT_JUDGED, with
// every enforced limit of a measured scenario not-judged (observed value kept) and pending targets
// still reported, while a failed or unmeasured scenario fails the run regardless.
func TestPRBV0039ContentionWithholdsJudgement(t *testing.T) {
	b := &Budgets{Schema: BudgetSchema, Scale: "smoke"}
	for _, id := range ScenarioIDs {
		b.Scenarios = append(b.Scenarios, ScenarioBudget{ID: id, Samples: 1,
			Targets:  []Limit{{Metric: MetricP95Wall, Limit: 100, Status: StatusPending, PendingOn: "GH #1598", Reason: "r"}},
			Ceilings: []Limit{{Metric: MetricP95Wall, Limit: 1000, Status: StatusEnforced, Reason: "r"}, {Metric: MetricPeakRSS, Limit: 1 << 30, Status: StatusEnforced, Reason: "r"}}})
	}
	rss := int64(1 << 20)
	measured := func(wall float64) []ScenarioMeasured {
		out := []ScenarioMeasured{}
		for _, id := range ScenarioIDs {
			out = append(out, ScenarioMeasured{ID: id, P50WallMS: float(wall), P95WallMS: float(wall), PeakRSSBytes: &rss})
		}
		return out
	}
	host := func(before, after []float64, unknown string) Host {
		h := Host{Cores: 4}
		h.LoadBefore = before
		h.LoadUnknown = unknown
		h.finish(context.Background(), func(context.Context) ([]float64, string) { return after, unknown })
		return h
	}
	quiet := host([]float64{3.99, 1, 1}, []float64{1, 1, 1}, "")
	afterFailed := Host{Cores: 4, LoadBefore: []float64{1, 1, 1}}
	afterFailed.finish(context.Background(), func(context.Context) ([]float64, string) { return nil, "x" })
	for name, tc := range map[string]struct {
		host    Host
		wall    float64
		verdict string
		reason  string
	}{
		"quiet pass":            {quiet, 50, RunPass, ""},
		"quiet exceeded":        {quiet, 5000, RunFail, ""},
		"contended before":      {host([]float64{4, 1, 1}, []float64{1, 1, 1}, ""), 5000, RunNotJudged, "host contended"},
		"contended after":       {host([]float64{1, 1, 1}, []float64{9, 1, 1}, ""), 50, RunNotJudged, "host contended"},
		"load not observed":     {host(nil, nil, "load average NOT_OBSERVED: x"), 50, RunNotJudged, "no host-contention record (load average NOT_OBSERVED: x)"},
		"load after unobserved": {host([]float64{1, 1, 1}, nil, ""), 50, RunNotJudged, "no host-contention record"},
		"after reading failed":  {afterFailed, 50, RunNotJudged, "no host-contention record (x)"},
		"empty load reading":    {host([]float64{}, []float64{}, ""), 50, RunNotJudged, "no host-contention record"},
	} {
		checks, verdict, reason := Judge(b, tc.host, measured(tc.wall))
		if verdict != tc.verdict || !strings.HasPrefix(reason, tc.reason) || (tc.reason == "") != (reason == "") {
			t.Errorf("%s: verdict %s reason %q, want %s %q", name, verdict, reason, tc.verdict, tc.reason)
		}
		for _, c := range checks {
			switch {
			case verdict == RunNotJudged && c.Limit.Status == StatusEnforced && (c.Verdict != VerdictNotJudged || c.Reason != reason || c.Observed == nil):
				t.Errorf("%s: enforced check %+v not withheld with its observation", name, c)
			case verdict == RunNotJudged && c.Limit.Status == StatusPending && !strings.HasPrefix(c.Verdict, "pending-"):
				t.Errorf("%s: pending target %+v not reported", name, c)
			case verdict != RunNotJudged && c.Verdict == VerdictNotJudged:
				t.Errorf("%s: judged run withheld %+v", name, c)
			}
		}
	}
	contended := host([]float64{8, 1, 1}, []float64{8, 1, 1}, "")
	failed := measured(50)
	failed[4] = ScenarioMeasured{ID: "full-delta", Error: "corpus-refused"}
	checks, verdict, reason := Judge(b, contended, failed)
	if verdict != RunFail || !strings.HasPrefix(reason, "host contended") {
		t.Fatalf("a failed scenario on a contended host was %s (%q), want FAIL with the reason kept", verdict, reason)
	}
	for _, c := range checks {
		if c.Scenario == "full-delta" && c.Limit.Status == StatusEnforced && c.Verdict != VerdictUnknown {
			t.Fatalf("failed scenario check %+v", c)
		}
		if c.Scenario != "full-delta" && c.Limit.Status == StatusEnforced && c.Verdict != VerdictNotJudged {
			t.Fatalf("measured scenario check %+v judged on a contended host", c)
		}
	}
	unobserved := measured(50)
	unobserved[2].PeakRSSBytes, unobserved[2].RSSUnknown = nil, "NOT_OBSERVED"
	checks, verdict, _ = Judge(b, contended, unobserved)
	if verdict != RunFail {
		t.Fatalf("an unobserved enforced metric on a contended host was %s, want FAIL", verdict)
	}
	for _, c := range checks {
		if c.Scenario == "mcp-first-call" && c.Limit.Metric == MetricPeakRSS && (c.Verdict != VerdictUnknown || !strings.Contains(c.Reason, "NOT_OBSERVED")) {
			t.Fatalf("unobserved metric check %+v", c)
		}
	}
	if _, verdict, _ := Judge(b, contended, measured(50)[1:]); verdict != RunFail {
		t.Fatalf("an unmeasured scenario on a contended host was %s, want FAIL", verdict)
	}
}

// PRB-V0-024, PRB-V0-025, PRB-V0-027, PRB-V0-028, PRB-V0-029: an end-to-end smoke run builds
// both binaries, generates the fixture through `corvint docs corpus build`, measures all six
// scenarios with wall time and peak resident set, records the host, and passes on current main
// with every #1598/#1601 target reported pending. Then a fixture reused with a changed builder
// identity is rebuilt rather than refused.
func TestPRBV0024HarnessSmokeRun(t *testing.T) {
	if testing.Short() {
		t.Skip("builds corvint and corvint-corpus-mcp")
	}
	dir := t.TempDir()
	bin := map[string]string{}
	for _, pkg := range []string{"corvint", "corvint-corpus-mcp"} {
		bin[pkg] = filepath.Join(dir, pkg)
		build := exec.Command("go", "build", "-o", bin[pkg], "../../cmd/"+pkg)
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build %s: %v %s", pkg, err, out)
		}
	}
	b, err := LoadBudgets("budgets.json")
	if err != nil {
		t.Fatal(err)
	}
	b.Scale = Smoke.Name
	for i := range b.Scenarios {
		b.Scenarios[i].Samples = 2
	}
	fixtureDir := filepath.Join(dir, "fixture")
	if err := os.Mkdir(fixtureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// A fixed quiet load keeps the verdict independent of what else this host runs.
	quiet := func(context.Context) ([]float64, string) { return []float64{0.5, 0.5, 0.5}, "" }
	r, err := Run(context.Background(), Options{Budgets: b, Corvint: bin["corvint"], CorpusMCP: bin["corvint-corpus-mcp"], FixtureDir: fixtureDir, Scale: Smoke, Revision: "test", LoadAverage: quiet})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, m := range r.Scenarios {
		ids = append(ids, m.ID)
		if m.Error != "" || len(m.Samples) == 0 || m.P50WallMS == nil || m.P95WallMS == nil || (m.PeakRSSBytes == nil && m.RSSUnknown == "") {
			t.Fatalf("scenario %s: %+v", m.ID, m)
		}
		for _, arg := range m.Command {
			if filepath.IsAbs(arg) {
				t.Fatalf("report carries a host path: %v", m.Command)
			}
		}
	}
	if !slices.Equal(ids, ScenarioIDs) {
		t.Fatalf("scenarios %v", ids)
	}
	if !r.OK || r.Verdict != RunPass || r.NotJudgedReason != "" || len(r.Pending) != 5 || r.Host.Cores < 1 {
		t.Fatalf("ok=%t verdict=%s pending=%d host=%+v checks=%+v", r.OK, r.Verdict, len(r.Pending), r.Host, r.Checks)
	}
	for _, c := range r.Pending {
		if !strings.HasPrefix(c.Verdict, "pending-") {
			t.Fatalf("pending target judged %q", c.Verdict)
		}
	}
	// PRB-V0-039: the same fixture measured on a contended host is NOT_JUDGED end to end, with
	// its measurements kept.
	busy := func(context.Context) ([]float64, string) {
		return []float64{float64(runtime.NumCPU()), 1, 1}, ""
	}
	contended, err := Run(context.Background(), Options{Budgets: b, Corvint: bin["corvint"], CorpusMCP: bin["corvint-corpus-mcp"], FixtureDir: fixtureDir, Scale: Smoke, Revision: "test", LoadAverage: busy})
	if err != nil {
		t.Fatal(err)
	}
	if contended.OK || contended.Verdict != RunNotJudged || !strings.HasPrefix(contended.NotJudgedReason, "host contended") || len(contended.Scenarios) != len(ScenarioIDs) || contended.Scenarios[0].P95WallMS == nil {
		t.Fatalf("contended run: ok=%t verdict=%s reason=%q", contended.OK, contended.Verdict, contended.NotJudgedReason)
	}
	// Reuse with a different builder identity rebuilds both artifacts with that builder.
	f, err := LoadOrGenerate(context.Background(), fixtureDir, Smoke, LibraryBuilder, "library")
	if err != nil {
		t.Fatal(err)
	}
	if f.BuilderSHA256 != "library" || f.Records != r.Fixture.Records {
		t.Fatalf("rebuilt fixture: %+v", f)
	}
	if _, err := LoadOrGenerate(context.Background(), fixtureDir, Full, LibraryBuilder, "library"); err == nil {
		t.Fatal("a smoke fixture was reused at full scale")
	}
}

// PRB-V0-036: measured processes never inherit CORVINT_ settings, the profile knobs included.
func TestPRBV0036MeasuredEnvironmentStripsSettings(t *testing.T) {
	t.Setenv("CORVINT_CPUPROFILE", "/tmp/should-not-leak")
	t.Setenv("CORVINT_INDEX_SHARDS", "4")
	for _, kv := range measuredEnv() {
		if strings.HasPrefix(kv, "CORVINT_") {
			t.Fatalf("measured environment carries %s", kv)
		}
	}
}
