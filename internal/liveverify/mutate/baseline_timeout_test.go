package mutate

import (
	"context"
	"strings"
	"testing"
	"time"
)

// fakeAlarm is what go test -json prints when its own -timeout ends the test
// binary during TestAdd (captured from go1.27): output events on the running
// test, then only the package's FAIL; TestAdd itself gets no fail event.
const fakeAlarm = `printf '%s\n' '{"Action":"run","Test":"TestAdd"}'; ` +
	`printf '%s\n' '{"Action":"output","Test":"TestAdd","Output":"panic: test timed out after 2s\n"}'; ` +
	`printf '%s\n' '{"Action":"output","Test":"TestAdd","Output":"\trunning tests:\n"}'; ` +
	`printf '%s\n' '{"Action":"output","Output":"FAIL\tcalc\t2.004s\n"}'; ` +
	`printf '%s\n' '{"Action":"fail","Elapsed":2.004}'; exit 1`

// fakeChatter prints more -json output than the 64 KiB bounded buffer keeps,
// so the alarm line arrives after the cut.
const fakeChatter = `i=0; while [ $i -lt 1200 ]; do ` +
	`printf '%s\n' '{"Action":"output","Test":"TestAdd","Output":"chatter chatter chatter chatter chatter chatter chatter\n"}'; ` +
	`i=$((i+1)); done; `

// FPK-V0-143, V1-2199: a baseline run that go test's own -timeout ends is the
// per-run limit, BUDGET_EXCEEDED naming it, never "baseline tests fail before
// mutation"; a genuinely red baseline keeps that detail.
func TestFPKV0143BaselineTestTimeoutNamesTheLimit(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	shortLimits(t)
	for _, run := range []struct {
		name, script, verdict, detail string
	}{
		{"alarm", fakeVersion + fakeAlarm, string(BudgetExceeded), "per-run limit of 2s reached during the baseline run"},
		{"alarm after the output cut", fakeVersion + fakeChatter + fakeAlarm, string(BudgetExceeded), "per-run limit of 2s reached during the baseline run"},
		{"red baseline", fakeVersion + fakeFail, string(Unsupported), "baseline tests fail before mutation: pkg/calc/calc_test.go"},
	} {
		t.Run(run.name, func(t *testing.T) {
			fakeLauncher(t, run.script)
			request := baseRequest(root, git, revision)
			request.Budget, request.MaxMutants = 6*time.Second, 2 // per-run limit 2s
			report, err := Run(context.Background(), request)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if string(report.Verdict) != run.verdict || report.Detail != run.detail {
				t.Fatalf("report = %s %q, want %s %q", report.Verdict, report.Detail, run.verdict, run.detail)
			}
		})
	}
}

// FPK-V0-143 on the grouped path prove uses: the timed-out test has no
// outcome in the package run, so its claim is re-run alone, and that run's
// limit is named.
func TestFPKV0143GroupedBaselineTestTimeoutNamesTheLimit(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	shortLimits(t)
	defaultBudget = 9 * time.Second // per-run share 1s over the default 8 mutants
	fakeLauncher(t, fakeVersion+fakeChatter+fakeAlarm)
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer exported.Close()
	reports, err := exported.JudgeGroup(context.Background(), "pkg/calc/calc.go", nil, []string{"pkg/calc/calc_test.go"})
	if err != nil {
		t.Fatalf("JudgeGroup: %v", err)
	}
	report := reports["pkg/calc/calc_test.go"]
	if report.Verdict != BudgetExceeded || report.Detail != "per-run limit of 1s reached during the baseline run" {
		t.Fatalf("report = %s %q, want BUDGET_EXCEEDED naming the 1s limit", report.Verdict, report.Detail)
	}
	if strings.Contains(report.Detail, "fail before mutation") {
		t.Fatalf("a timed-out baseline was reported red: %q", report.Detail)
	}
}
