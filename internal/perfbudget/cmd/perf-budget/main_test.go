package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		"unknown flag":    {"--bogus"},
		"positional":      {"extra"},
		"missing budgets": {"--budgets", filepath.Join(t.TempDir(), "absent.json")},
		"unknown scale":   {"--budgets", smoke, "--scale", "huge"},
		"scale mismatch":  {"--budgets", smoke, "--scale", "smoke"},
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
