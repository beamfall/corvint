package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/perfbudget"
)

// PRB-V0-029: the command exits 2, distinct from a budget failure's 1, when the harness cannot
// run: bad arguments, an unreadable budgets file, or a scale other than the budgets'.
func TestPRBV0029HarnessCannotRunExitsTwo(t *testing.T) {
	smoke := filepath.Join(t.TempDir(), "budgets.json")
	committed, err := os.ReadFile(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(smoke, committed, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"unknown flag":            {"--bogus"},
		"positional":              {"extra"},
		"missing budgets":         {"--budgets", filepath.Join(t.TempDir(), "absent.json")},
		"unknown scale":           {"--budgets", smoke, "--scale", "huge"},
		"scale mismatch baseline": {"--budgets", smoke, "--scale", "smoke", "--record-baseline"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Errorf("%s: exit %d stdout %q", name, code, &stdout)
		}
		if name != "unknown flag" && name != "positional" && !strings.HasPrefix(stderr.String(), "perf-budget: ") {
			t.Errorf("%s: stderr %q", name, &stderr)
		}
	}
}

// PRB-V0-041: PASS exits 0 and FAIL 1; NOT_JUDGED exits 0 unless --require-judgement, then 3,
// so it never shares a status with a budget failure or a harness that cannot run.
func TestPRBV0041VerdictExitStatus(t *testing.T) {
	for _, tc := range []struct {
		verdict string
		require bool
		want    int
	}{
		{perfbudget.RunPass, false, 0},
		{perfbudget.RunPass, true, 0},
		{perfbudget.RunFail, false, 1},
		{perfbudget.RunFail, true, 1},
		{perfbudget.RunNotJudged, false, 0},
		{perfbudget.RunNotJudged, true, 3},
		{"", false, 1},
	} {
		if got := verdictStatus(tc.verdict, tc.require); got != tc.want {
			t.Errorf("verdict %q require %t: exit %d, want %d", tc.verdict, tc.require, got, tc.want)
		}
	}
}

// PRB-V0-042: `make perf-budget` passes --require-judgement exactly when
// PERF_BUDGET_REQUIRE_JUDGEMENT=1.
func TestPRBV0042MakeTargetRequireJudgement(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	for value, want := range map[string]bool{"": false, "0": false, "1": true} {
		// -f names the Makefile: without it make finds it through a directory listing of the
		// root, which the declared read scope (only the Makefile itself) does not grant.
		out, err := exec.Command("make", "-s", "-n", "-C", filepath.Join("..", "..", "..", ".."), "-f", "Makefile", "perf-budget", "PERF_BUDGET_REQUIRE_JUDGEMENT="+value).CombinedOutput()
		if err != nil {
			t.Fatalf("make -n perf-budget: %v %s", err, out)
		}
		if got := strings.Contains(string(out), "--require-judgement"); got != want || !strings.Contains(string(out), "/perf-budget\"") {
			t.Errorf("PERF_BUDGET_REQUIRE_JUDGEMENT=%q: recipe %s", value, out)
		}
	}
}

// PRB-V0-043: --record-baseline refuses a NOT_JUDGED run, and a run with a failed scenario,
// leaving the budgets file byte-identical; a judged run is recorded with its host record.
func TestPRBV0043NotJudgedRunIsNotBaselined(t *testing.T) {
	committed, err := os.ReadFile(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	budgets, err := perfbudget.LoadBudgets(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := func(verdict string, failed bool) *perfbudget.Report {
		r := &perfbudget.Report{Verdict: verdict, NotJudgedReason: "host contended", StartedAt: "2026-10-23T00:00:00Z", Revision: "r", Host: perfbudget.Host{Cores: 4}, Fixture: budgets.Baseline.Fixture}
		if verdict == perfbudget.RunPass {
			r.NotJudgedReason = ""
		}
		for _, id := range perfbudget.ScenarioIDs {
			m := perfbudget.ScenarioMeasured{ID: id, Samples: []perfbudget.Sample{{WallMS: 1}}}
			if failed {
				m.Error = "refused"
			}
			r.Scenarios = append(r.Scenarios, m)
		}
		return r
	}
	path := filepath.Join(t.TempDir(), "budgets.json")
	for name, r := range map[string]*perfbudget.Report{
		"not judged":      report(perfbudget.RunNotJudged, false),
		"failed scenario": report(perfbudget.RunFail, true),
		"contended fail":  report(perfbudget.RunFail, false),
	} {
		if err := os.WriteFile(path, committed, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := recordBaseline(path, budgets, r); err == nil || !strings.HasPrefix(err.Error(), "baseline not recorded") {
			t.Errorf("%s: recorded (%v)", name, err)
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, committed) {
			t.Errorf("%s: budgets file changed", name)
		}
	}
	if err := recordBaseline(path, budgets, report(perfbudget.RunPass, false)); err != nil {
		t.Fatal(err)
	}
	recorded, err := perfbudget.LoadBudgets(path)
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Baseline.Revision != "r" || recorded.Baseline.Host.Cores != 4 {
		t.Fatalf("judged run not recorded: %+v", recorded.Baseline)
	}
	if budgets.Baseline.Revision == "r" {
		t.Fatal("recording mutated the loaded budgets")
	}
}

// PRB-V0-056: a pending or advisory target's progress line names its profile and that it is not
// enforced, so a target within its limit never reads as a passing limit.
func TestPRBV0056CheckLineNamesUnenforcedTargets(t *testing.T) {
	for _, tc := range []struct {
		limit perfbudget.Limit
		want  string
	}{
		{perfbudget.Limit{Metric: perfbudget.MetricP95Wall, Status: perfbudget.StatusPending, Profile: perfbudget.ProfileExperimental, PendingOn: "GH #1598"}, "cli-warm-read target p95_wall_ms: pending-within (experimental target, not enforced: pending on GH #1598)"},
		{perfbudget.Limit{Metric: perfbudget.MetricP95Wall, Status: perfbudget.StatusAdvisory, Profile: perfbudget.ProfileCore}, "cli-warm-read target p95_wall_ms: advisory-within (core target, not enforced: advisory)"},
		{perfbudget.Limit{Metric: perfbudget.MetricP95Wall, Status: perfbudget.StatusEnforced}, "cli-warm-read ceiling p95_wall_ms: pass"},
	} {
		kind, verdict := "target", map[string]string{perfbudget.StatusPending: perfbudget.VerdictPendingWithin, perfbudget.StatusAdvisory: perfbudget.VerdictAdvisoryWithin, perfbudget.StatusEnforced: perfbudget.VerdictPass}[tc.limit.Status]
		if tc.limit.Status == perfbudget.StatusEnforced {
			kind = "ceiling"
		}
		if got := checkLine(perfbudget.Check{Scenario: "cli-warm-read", Kind: kind, Limit: tc.limit, Verdict: verdict}); got != tc.want {
			t.Errorf("line %q, want %q", got, tc.want)
		}
	}
}

// PRB-V0-970: an unknown or repeated --scenario, or --record-baseline-from with a run flag,
// exits 2 before anything runs.
func TestPRBV0970ScenarioFilterArguments(t *testing.T) {
	report := filepath.Join(t.TempDir(), "report.json")
	for name, args := range map[string][]string{
		"unknown":       {"--scenario", "cli-hot-read"},
		"repeated":      {"--scenario", "cli-warm-read", "--scenario", "cli-warm-read"},
		"from + run":    {"--record-baseline-from", report, "--scenario", "cli-warm-read"},
		"from + record": {"--record-baseline-from", report, "--record-baseline"},
	} {
		var stdout, stderr bytes.Buffer
		args = append([]string{"--budgets", filepath.Join("..", "..", "budgets.json")}, args...)
		if code := run(context.Background(), args, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Errorf("%s: exit %d stdout %q stderr %q", name, code, &stdout, &stderr)
		}
	}
}

// judgedReport is a PASS report on an uncontended host at a clean revision.
func judgedReport(t *testing.T, budgets *perfbudget.Budgets, selected []string) *perfbudget.Report {
	t.Helper()
	contended := false
	fixture := *budgets.Baseline.Fixture
	fixture.BuilderSHA256 = strings.Repeat("b", 64)
	r := &perfbudget.Report{Schema: perfbudget.ReportSchema, Verdict: perfbudget.RunPass, OK: true, StartedAt: "2026-10-31T00:00:00Z",
		Revision: strings.Repeat("a", 40), Host: perfbudget.Host{Cores: 32, Contended: &contended, LoadBefore: []float64{1, 1, 1}, LoadAfter: []float64{1, 1, 1}},
		Fixture: &fixture, Selected: selected, Checks: []perfbudget.Check{}, Pending: []perfbudget.Check{}, Advisory: []perfbudget.Check{}}
	ids := selected
	if len(ids) == 0 {
		ids = perfbudget.ScenarioIDs
	}
	for _, id := range ids {
		wall := 7.0
		r.Scenarios = append(r.Scenarios, perfbudget.ScenarioMeasured{ID: id, Command: []string{"corvint"}, Samples: []perfbudget.Sample{{WallMS: 7}}, P50WallMS: &wall, P95WallMS: &wall})
	}
	return r
}

// PRB-V0-970: a filtered --record-baseline replaces only the selected measurements, each with
// its run's provenance, and refuses a dirty revision or a budgets file without a baseline.
func TestPRBV0970FilteredBaselineReplacesOnlySelected(t *testing.T) {
	committed, err := os.ReadFile(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	budgets, err := perfbudget.LoadBudgets(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "budgets.json")
	dirty := judgedReport(t, budgets, []string{"mcp-warm-call"})
	dirty.Revision += "+dirty"
	empty := *budgets
	empty.Baseline = nil
	for name, attempt := range map[string]func() error{
		"dirty revision": func() error { return recordBaseline(path, budgets, dirty) },
		"no baseline":    func() error { return recordBaseline(path, &empty, judgedReport(t, budgets, []string{"mcp-warm-call"})) },
	} {
		if err := os.WriteFile(path, committed, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := attempt(); err == nil || !strings.HasPrefix(err.Error(), "baseline not recorded") {
			t.Errorf("%s: recorded (%v)", name, err)
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, committed) {
			t.Errorf("%s: budgets file changed", name)
		}
	}
	if err := recordBaseline(path, budgets, judgedReport(t, budgets, []string{"mcp-warm-call"})); err != nil {
		t.Fatal(err)
	}
	recorded, err := perfbudget.LoadBudgets(path)
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Baseline.Revision != budgets.Baseline.Revision {
		t.Fatalf("a filtered run replaced the baseline's own revision: %s", recorded.Baseline.Revision)
	}
	for index, m := range recorded.Baseline.Scenarios {
		was := budgets.Baseline.Scenarios[index]
		if m.ID != "mcp-warm-call" {
			if !reflect.DeepEqual(m, was) {
				t.Fatalf("unselected scenario %s changed", m.ID)
			}
			continue
		}
		if m.Provenance == nil || m.Provenance.ExecutableRevision != strings.Repeat("a", 40) || m.Provenance.ExecutableSHA256 != strings.Repeat("b", 64) || *m.P50WallMS != 7 {
			t.Fatalf("selected scenario not replaced with provenance: %+v", m)
		}
	}
}

// PRB-V0-971: --record-baseline-from records a judged report written elsewhere and refuses a
// contended one, trailing content, or another scale, leaving the budgets file untouched.
func TestPRBV0971RecordBaselineFromReport(t *testing.T) {
	committed, err := os.ReadFile(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	budgets, err := perfbudget.LoadBudgets(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "budgets.json")
	write := func(name string, r *perfbudget.Report, trailer string) string {
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, append(data, trailer...), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	contended := judgedReport(t, budgets, nil)
	contended.Verdict, contended.NotJudgedReason, contended.OK = perfbudget.RunNotJudged, "host contended", false
	smoke := judgedReport(t, budgets, nil)
	smoke.Fixture.Scale = perfbudget.Smoke
	mutate := func(name string, change func(*perfbudget.Report)) string {
		r := judgedReport(t, budgets, nil)
		change(r)
		return write(name+".json", r, "")
	}
	schemaOnly := filepath.Join(dir, "schema-only.json")
	if err := os.WriteFile(schemaOnly, []byte(`{"schema":"`+perfbudget.ReportSchema+`","fixture":{"scale":{"name":"full"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, report := range map[string]string{
		"schema only":       schemaOnly,
		"no revision":       mutate("no-revision", func(r *perfbudget.Report) { r.Revision = "" }),
		"dirty revision":    mutate("dirty", func(r *perfbudget.Report) { r.Revision += "+dirty" }),
		"no start":          mutate("no-start", func(r *perfbudget.Report) { r.StartedAt = "" }),
		"no builder":        mutate("no-builder", func(r *perfbudget.Report) { r.Fixture.BuilderSHA256 = "" }),
		"unobserved host":   mutate("unobserved", func(r *perfbudget.Report) { r.Host = perfbudget.Host{Cores: 4} }),
		"missing scenario":  mutate("missing", func(r *perfbudget.Report) { r.Scenarios = r.Scenarios[1:] }),
		"repeated scenario": mutate("repeated", func(r *perfbudget.Report) { r.Scenarios = append(r.Scenarios, r.Scenarios[0]) }),
		"unmeasured":        mutate("unmeasured", func(r *perfbudget.Report) { r.Scenarios[2].Samples, r.Scenarios[2].P50WallMS = nil, nil }),
		"selected mismatch": mutate("selected", func(r *perfbudget.Report) { r.Selected = []string{"cli-warm-read"} }),
		"selected repeated": mutate("selected-repeated", func(r *perfbudget.Report) {
			r.Selected = []string{"cli-warm-read", "cli-warm-read"}
			r.Scenarios = r.Scenarios[1:2]
		}),
		"contended": write("contended.json", contended, "\n"),
		"trailing":  write("trailing.json", judgedReport(t, budgets, nil), "}"),
		"scale":     write("smoke.json", smoke, ""),
		"absent":    filepath.Join(dir, "absent.json"),
	} {
		if err := os.WriteFile(path, committed, 0o600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), []string{"--budgets", path, "--record-baseline-from", report}, &stdout, &stderr); code != 2 {
			t.Errorf("%s: exit %d stderr %q", name, code, &stderr)
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, committed) {
			t.Errorf("%s: budgets file changed", name)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"--budgets", path, "--record-baseline-from", write("quiet.json", judgedReport(t, budgets, nil), "\n")}, &stdout, &stderr); code != 0 {
		t.Fatalf("judged report: exit %d stderr %q", code, &stderr)
	}
	recorded, err := perfbudget.LoadBudgets(path)
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Baseline.Revision != strings.Repeat("a", 40) || recorded.Baseline.Host.Cores != 32 || len(recorded.Baseline.Scenarios) != len(perfbudget.ScenarioIDs) {
		t.Fatalf("report not recorded as the baseline: %+v", recorded.Baseline)
	}
}

// PRB-V0-120 (V1-3840): a --scale smoke run against the committed full-scale budgets is not
// refused; it runs as measurements only, every check not-judged with the scale reason, the run
// NOT_JUDGED (FAIL only for a scenario that did not measure), and the progress line names which
// budget rows applied. Recording a baseline at another scale stays refused (above).
func TestPRBV0120SmokeScaleRunIsMeasurementsOnly(t *testing.T) {
	observed := 12.0
	report := &perfbudget.Report{
		Checks: []perfbudget.Check{
			{Scenario: "cli-cold-read", Kind: "ceiling", Limit: perfbudget.Limit{Metric: perfbudget.MetricP95Wall, Status: perfbudget.StatusEnforced}, Observed: &observed, Verdict: perfbudget.VerdictPass},
			{Scenario: "cli-cold-read", Kind: "target", Limit: perfbudget.Limit{Metric: perfbudget.MetricP95Wall, Status: perfbudget.StatusPending}, Observed: &observed, Verdict: perfbudget.VerdictPendingWithin},
		},
		Pending: []perfbudget.Check{{Scenario: "cli-cold-read"}},
		Verdict: perfbudget.RunPass, OK: true,
	}
	reason := `budgets b.json bound scale "full", not "smoke", so no budget row applies to this run (measurements only)`
	withholdForScale(report, reason)
	if report.Verdict != perfbudget.RunNotJudged || report.OK || report.NotJudgedReason != reason || len(report.Pending) != 0 {
		t.Fatalf("report %+v", report)
	}
	for _, c := range report.Checks {
		if c.Verdict != perfbudget.VerdictNotJudged || c.Reason != reason || c.Observed == nil {
			t.Errorf("check %+v", c)
		}
	}
	if got := appliedLine(report, "b.json", "full", reason); got != "budget rows applied: none; "+reason {
		t.Errorf("applied line %q", got)
	}
	if got := appliedLine(report, "b.json", "full", ""); got != `budget rows applied: 2 checks from b.json at scale "full"` {
		t.Errorf("applied line %q", got)
	}
	failed := &perfbudget.Report{Checks: []perfbudget.Check{{Scenario: "x", Verdict: perfbudget.VerdictUnknown}}, Verdict: perfbudget.RunFail}
	withholdForScale(failed, reason)
	if failed.Verdict != perfbudget.RunFail {
		t.Errorf("an unmeasured scenario at smoke scale reads %s, want FAIL", failed.Verdict)
	}
	same := &perfbudget.Report{Verdict: perfbudget.RunPass, OK: true}
	withholdForScale(same, "")
	if same.Verdict != perfbudget.RunPass || !same.OK {
		t.Errorf("a run at the budgets scale was withheld: %+v", same)
	}
}
