// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
)

// goParse reads a go test -json stream of module example.test/m at the root.
func goParse(t *testing.T, lines ...string) ([]Result, error) {
	t.Helper()
	r, _ := For(testid.Go)
	return r.Parse(ParseRequest{Instance: Instance{Runner: testid.Go, Root: "."}, Report: []byte(strings.Join(lines, "\n")), Module: "example.test/m"})
}

func outcomeOf(results []Result, id string) string {
	for _, result := range results {
		if result.ID == id {
			return result.Outcome
		}
	}
	return ""
}

// RNC-V1-006 (review P1-2): a package that panics or times out after some
// tests passed ran fewer tests than it holds, so a full run and a
// whole-package selection of it are incomplete and failed, never complete.
// The streams are the shapes go1.27 test2json writes: a panic fails the
// test that panicked and then the package, and a timeout fails the package
// with the running test never ended. A stream that stops before its
// package ends is incomplete too.
func TestRNCV1006GoPackageThatEndsAbnormallyIsIncomplete(t *testing.T) {
	panicked := []string{
		`{"Action":"start","Package":"example.test/m/a"}`,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestA"}`,
		`{"Action":"pass","Package":"example.test/m/a","Test":"TestA","Elapsed":0}`,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestX"}`,
		`{"Action":"output","Package":"example.test/m/a","Test":"TestX","Output":"panic: boom [recovered, repanicked]\n"}`,
		`{"Action":"fail","Package":"example.test/m/a","Test":"TestX","Elapsed":0}`,
		`{"Action":"fail","Package":"example.test/m/a","Elapsed":0.005}`,
	}
	timedOut := []string{
		`{"Action":"start","Package":"example.test/m/a"}`,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestA"}`,
		`{"Action":"pass","Package":"example.test/m/a","Test":"TestA","Elapsed":0}`,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestSlow"}`,
		`{"Action":"fail","Package":"example.test/m/a","Elapsed":1.005}`,
	}
	stopped := []string{
		`{"Action":"start","Package":"example.test/m/a"}`,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestA"}`,
		`{"Action":"pass","Package":"example.test/m/a","Test":"TestA","Elapsed":0}`,
	}
	for name, stream := range map[string][]string{"panic": panicked, "timeout": timedOut, "stream stopped": stopped} {
		t.Run(name, func(t *testing.T) {
			results, err := goParse(t, stream...)
			if err != nil {
				t.Fatal(err)
			}
			if got := outcomeOf(results, "go:a"); got != OutcomeError {
				t.Fatalf("package outcome %q, want %q: %+v", got, OutcomeError, results)
			}
			if run := Reconcile(nil, results); !run.Incomplete {
				t.Fatalf("full run complete: %+v", run)
			}
			if run := Reconcile([]string{"go:a"}, results); !run.Incomplete {
				t.Fatalf("whole-package selection complete: %+v", run)
			}
		})
	}
	results, err := goParse(t, timedOut...)
	if err != nil || outcomeOf(results, "go:a::TestSlow") != OutcomeError {
		t.Fatalf("the test the timeout stopped: %+v %v", results, err)
	}
	// A package that fails after every test it started ended (TestMain
	// exiting non-zero) still fails at package level, and stays complete.
	results, err = goParse(t,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestA"}`,
		`{"Action":"pass","Package":"example.test/m/a","Test":"TestA","Elapsed":0}`,
		`{"Action":"fail","Package":"example.test/m/a","Elapsed":0.1}`)
	if err != nil || outcomeOf(results, "go:a") != OutcomeFail || Reconcile(nil, results).Incomplete {
		t.Fatalf("a package failure after its tests passed: %+v %v", results, err)
	}
}

// RNC-V1-006 (review P2-1): a test the report names twice keeps its most
// severe outcome, whichever order the report gives.
func TestRNCV1006DuplicateResultsKeepTheMostSevere(t *testing.T) {
	for name, outcomes := range map[string][]string{
		"fail then pass":  {OutcomeFail, OutcomePass},
		"pass then fail":  {OutcomePass, OutcomeFail},
		"skip then fail":  {OutcomeSkip, OutcomeFail},
		"error then fail": {OutcomeError, OutcomeFail},
	} {
		var results []Result
		for _, outcome := range outcomes {
			results = append(results, Result{ID: "go:a::TestA", Outcome: outcome, Granularity: testid.GranularityTest})
		}
		want := OutcomeFail
		if outcomes[0] == OutcomeError {
			want = OutcomeError
		}
		run := Reconcile([]string{"go:a::TestA"}, results)
		if len(run.Results) != 1 || run.Results[0].Outcome != want {
			t.Errorf("%s: %+v, want one %s", name, run.Results, want)
		}
	}
}

// RNC-V1-005 (review P2-3): a truncated go test -json line is not dropped:
// Parse keeps what it read and returns an error, so the run is incomplete.
func TestRNCV1005GoTruncatedLineIsReported(t *testing.T) {
	results, err := goParse(t,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestA"}`,
		`{"Action":"pass","Package":"example.test/m/a","Test":"TestA","Elap`,
		`{"Action":"pass","Package":"example.test/m/a","Elapsed":0.1}`,
		`# example.test/m/a build text is not an event and is skipped`)
	if err == nil {
		t.Fatalf("a truncated line was dropped silently: %+v", results)
	}
	// TestA's end was the truncated line, so it started and never ended.
	if outcomeOf(results, "go:a::TestA") != OutcomeError || outcomeOf(results, "go:a") != OutcomeError {
		t.Fatalf("results %+v", results)
	}
}

// RNC-V1-005 (review P2-2): a test file that fails without a failing
// assertion (it did not load) is a file-level fail, and a Jest-shaped report
// whose success is false with nothing failing, or a Playwright report with
// top-level errors, returns an error: the run failed.
func TestRNCV1005FileLevelFailuresFailTheRun(t *testing.T) {
	vitest, _ := For(testid.Vitest)
	instance := Instance{Runner: testid.Vitest, Root: "web"}
	results, err := vitest.Parse(ParseRequest{Instance: instance, Root: "/repo", Report: []byte(`{"success":false,"testResults":[
		{"name":"/repo/web/src/a.test.ts","status":"passed","assertionResults":[{"ancestorTitles":[],"title":"adds","status":"passed","duration":1}]},
		{"name":"/repo/web/src/broken.test.ts","status":"failed","message":"SyntaxError: Unexpected token","assertionResults":[]}]}`)})
	if err != nil || outcomeOf(results, "vitest:web/src/broken.test.ts") != OutcomeFail {
		t.Fatalf("a file that did not load: %+v %v", results, err)
	}
	if run := Reconcile([]string{"vitest:web/src/broken.test.ts::adds"}, results); outcomeOf(run.Results, "vitest:web/src/broken.test.ts") != OutcomeFail {
		t.Fatalf("selection in the broken file: %+v", run)
	}
	if _, err := vitest.Parse(ParseRequest{Instance: instance, Root: "/repo", Report: []byte(`{"success":false,"testResults":[]}`)}); err == nil {
		t.Fatal("success false with nothing failing read as a clean run")
	}
	playwright, _ := For(testid.Playwright)
	results, err = playwright.Parse(ParseRequest{Instance: Instance{Runner: testid.Playwright, Root: "e2e"}, Root: "/repo", Report: []byte(`{"config":{"rootDir":"/repo/e2e"},"suites":[],"errors":[{"message":"Error: Cannot find module './missing'\n  at x"}]}`)})
	if err == nil {
		t.Fatalf("a Playwright report with top-level errors read as a clean run: %+v", results)
	}
}

// RNC-V1-003 (review P2-4): a file filter never reaches the runner's argv
// as a flag, and an empty inventory listing proves nothing.
func TestRNCV1003FileFilterIsNeverAFlag(t *testing.T) {
	vitest := Instance{Runner: testid.Vitest, Root: ".", Config: "vitest.config.ts"}
	got := command(t, Request{Instance: vitest, Full: []string{"npx", "vitest", "run"}, Tests: []string{"vitest:-x.test.ts"}, Files: []string{"-x.test.ts", "b.test.ts"}, Report: "/r.json"})
	if last := got.Argv[len(got.Argv)-1]; last != "x.test.ts" || !got.Exact {
		t.Fatalf("vitest: %+v", got)
	}
	if got := command(t, Request{Instance: vitest, Full: []string{"npx", "vitest", "run"}, Tests: []string{"vitest:-x.test.ts"}, Files: []string{"-x.test.ts", "ax.test.ts"}, Report: "/r.json"}); got.Exact {
		t.Fatalf("a wider filter stayed exact: %+v", got)
	}
	if got := command(t, Request{Instance: vitest, Full: []string{"npx", "vitest", "run"}, Tests: []string{"vitest:---"}, Files: []string{"---"}, Report: "/r.json"}); got.Exact || got.Tier != TierFull || strings.HasPrefix(got.Argv[len(got.Argv)-1], "---") {
		t.Fatalf("a dash-only file: %+v", got)
	}
	playwright := Instance{Runner: testid.Playwright, Root: "e2e", Config: "e2e/playwright.config.ts"}
	got = command(t, Request{Instance: playwright, Full: []string{"npx", "playwright", "test"}, Tests: []string{"playwright:e2e/-login.spec.ts"}, Files: []string{"e2e/-login.spec.ts"}, Report: "/r.json"})
	if last := got.Argv[len(got.Argv)-1]; last != `login\.spec\.ts` || !got.Exact {
		t.Fatalf("playwright: %+v", got)
	}
	if got := command(t, Request{Instance: vitest, Full: []string{"npx", "vitest", "run"}, Tests: []string{"vitest:src/a.test.ts"}, Files: []string{}, Report: "/r.json"}); got.Exact || got.Reason == "" {
		t.Fatalf("an empty listing proved the filter exact: %+v", got)
	}
}

// RNC-V1-003 (review P2-5): the Go selection keeps each flag's value,
// replaces only package patterns, and runs the full command with exact
// false for -args or a flag it cannot model.
func TestRNCV1003GoFlagValuesSurvive(t *testing.T) {
	root := Instance{Runner: testid.Go, Root: ".", Config: "go.mod"}
	got := command(t, Request{Instance: root, Full: []string{"go", "test", "-coverprofile", "./c.out", "-o", "./x", "-count=1", "-tags", "a,b", goEveryPackage}, Tests: []string{"go:a"}})
	if want := []string{"go", "test", "-json", "-coverprofile", "./c.out", "-o", "./x", "-count=1", "-tags", "a,b", "./a"}; !slices.Equal(got.Argv, want) || !got.Exact {
		t.Fatalf("argv %q exact %v, want %q", got.Argv, got.Exact, want)
	}
	for name, full := range map[string][]string{
		"-args":        {"go", "test", goEveryPackage, "-args", "-update", "golden"},
		"unknown flag": {"go", "test", "-update", goEveryPackage},
		"no value":     {"go", "test", goEveryPackage, "-run"},
	} {
		got := command(t, Request{Instance: root, Full: full, Tests: []string{"go:a"}})
		if got.Exact || got.Reason == "" || got.Tier != TierFull || !slices.Contains(got.Argv, goEveryPackage) {
			t.Errorf("%s: %+v, want the full command with exact false", name, got)
		}
	}
}
