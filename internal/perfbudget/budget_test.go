package perfbudget

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

func float(v float64) *float64 { return &v }

// PRB-V0-031, PRB-V0-032, PRB-V0-033: the committed budgets file bounds exactly the
// ScenarioIDs scenarios, encodes the GitHub #1598 and #1601 targets at their stated numbers as pending
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

// PRB-V0-050, PRB-V0-051, PRB-V0-052: the unchanged-delta scenario streams the current
// artifact's delta against itself, follows the six #1602 scenarios, carries its baseline and the
// tighter wall ceiling derived from it, and that ceiling fails a delta that rederives: the
// before-change measurement in docs/build-log/2026-10-23-corpus-delta-verified-read.md. Those
// rederiving measurements are recorded constants, so this test cannot detect a restored
// rederivation; TestDCPV1460DeltaCLIVerifiedRead's build counter is the deterministic guard.
func TestPRBV0050UnchangedDeltaBudget(t *testing.T) {
	b, err := LoadBudgets("budgets.json")
	if err != nil {
		t.Fatal(err)
	}
	if ScenarioIDs[len(ScenarioIDs)-1] != "unchanged-delta" || !slices.Equal(ScenarioIDs[:6], []string{"cli-cold-read", "cli-warm-read", "mcp-first-call", "mcp-warm-call", "full-delta", "affected-path"}) {
		t.Fatalf("scenario order %v", ScenarioIDs)
	}
	var budget ScenarioBudget
	for _, s := range b.Scenarios {
		if s.ID == "unchanged-delta" {
			budget = s
		}
	}
	var baseline ScenarioMeasured
	for _, m := range b.Baseline.Scenarios {
		if m.ID == "unchanged-delta" {
			baseline = m
		}
	}
	want := []string{"corvint", "--root", "<fixture>/app", "docs", "corpus", "delta", "--previous", "corpus.json", "--artifact", "corpus.json", "--format", "jsonl"}
	if !slices.Equal(baseline.Command, want) || baseline.P95WallMS == nil || baseline.PeakRSSBytes == nil || len(budget.Targets) != 0 {
		t.Fatalf("unchanged-delta baseline %+v, budget %+v", baseline, budget)
	}
	ceilings := map[string]float64{}
	for _, c := range budget.Ceilings {
		ceilings[c.Metric] = c.Limit
	}
	const mib = 1 << 20
	if wall := math.Ceil(2**baseline.P95WallMS/500) * 500; ceilings[MetricP95Wall] != wall {
		t.Fatalf("p95 ceiling %v, want 2x baseline rounded up to 500 ms: %v", ceilings[MetricP95Wall], wall)
	}
	if rss := math.Ceil(1.5*float64(*baseline.PeakRSSBytes)/(64*mib)) * 64 * mib; ceilings[MetricPeakRSS] != rss {
		t.Fatalf("RSS ceiling %v, want %v", ceilings[MetricPeakRSS], rss)
	}
	// The fastest and slowest before-change (rederiving) samples of the same command.
	for _, rederived := range []float64{5998.30, 7840.0} {
		rss := *baseline.PeakRSSBytes
		measured := []ScenarioMeasured{}
		for _, id := range ScenarioIDs {
			m := ScenarioMeasured{ID: id, P50WallMS: float(1), P95WallMS: float(1), PeakRSSBytes: &rss}
			if id == "unchanged-delta" {
				m.P95WallMS = float(rederived)
			}
			measured = append(measured, m)
		}
		checks, ok := Evaluate(b, measured)
		exceeded := false
		for _, c := range checks {
			exceeded = exceeded || c.Scenario == "unchanged-delta" && c.Limit.Metric == MetricP95Wall && c.Verdict == VerdictExceeded
		}
		if ok || !exceeded {
			t.Fatalf("a rederiving delta at %v ms passed the unchanged-delta ceiling", rederived)
		}
	}
}

// PRB-V0-053: a committed baseline measurement taken in another run carries that run's start
// time, measured executable and host observation, so it is not attributed to the baseline's run;
// the six measurements of the baseline's own run carry none, and a malformed provenance is refused.
func TestPRBV0053BaselineProvenance(t *testing.T) {
	b, err := LoadBudgets("budgets.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range b.Baseline.Scenarios {
		p := m.Provenance
		if m.ID != "unchanged-delta" {
			if p != nil {
				t.Errorf("%s: provenance %+v on a measurement of the baseline's own run", m.ID, p)
			}
			continue
		}
		if p == nil || p.RecordedAt == b.Baseline.RecordedAt || p.ExecutableSHA256 == b.Baseline.Fixture.BuilderSHA256 || p.ExecutableRevision != "a86d556279bf004318b6fe8b0a039961fb4a2791" || p.Host.LoadBefore == nil {
			t.Fatalf("unchanged-delta provenance %+v does not name its own run", p)
		}
	}
	valid := func() *Budgets {
		copy := *b
		copy.Baseline = &Baseline{Scenarios: []ScenarioMeasured{{ID: "unchanged-delta", Provenance: &Provenance{RecordedAt: "2026-10-23T15:39:25Z", ExecutableRevision: strings.Repeat("a", 40), ExecutableSHA256: strings.Repeat("b", 64), Host: Host{Cores: 1, Contended: new(bool)}}}}}
		return &copy
	}
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(*Provenance){
		"time":     func(p *Provenance) { p.RecordedAt = "" },
		"revision": func(p *Provenance) { p.ExecutableRevision += "+dirty" },
		"sha256":   func(p *Provenance) { p.ExecutableSHA256 = "unrecorded" },
		"host":     func(p *Provenance) { p.Host.Contended = nil },
	} {
		bad := valid()
		edit(bad.Baseline.Scenarios[0].Provenance)
		if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "provenance") {
			t.Errorf("%s: malformed provenance admitted: %v", name, err)
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

// PRB-V0-024, PRB-V0-025, PRB-V0-027, PRB-V0-028, PRB-V0-029: an end-to-end smoke run builds
// both binaries, generates the fixture through `corvint docs corpus build`, measures every
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
		bin[pkg] = filepath.Join(dir, testplatform.Executable(pkg))
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
	r, err := Run(context.Background(), Options{Budgets: b, Corvint: bin["corvint"], CorpusMCP: bin["corvint-corpus-mcp"], FixtureDir: fixtureDir, Scale: Smoke, Revision: "test"})
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
	if !r.OK || len(r.Pending) != 5 || r.Host.Cores < 1 {
		t.Fatalf("ok=%t pending=%d host=%+v checks=%+v", r.OK, len(r.Pending), r.Host, r.Checks)
	}
	for _, c := range r.Pending {
		if !strings.HasPrefix(c.Verdict, "pending-") {
			t.Fatalf("pending target judged %q", c.Verdict)
		}
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
