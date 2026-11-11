// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// RNC-V1-001: one interface, compiled in, implemented by all seven
// blocking runners in the spec's order.
func TestRNCV1001EveryRunnerImplementsTheContract(t *testing.T) {
	var names []string
	for _, runner := range Runners() {
		names = append(names, runner.Name())
		if got, ok := For(runner.Name()); !ok || got.Name() != runner.Name() {
			t.Errorf("For(%q) = %v, %v", runner.Name(), got, ok)
		}
	}
	if !slices.Equal(names, testid.Runners) {
		t.Fatalf("runners %v, want %v", names, testid.Runners)
	}
	if _, ok := For("cypress"); ok {
		t.Fatal("cypress is not a 1.0 runner")
	}
}

// RNC-V1-002: each runner supplies the inventory's own lister, marked
// static or executing; RSpec and Minitest (and Go) are static.
func TestRNCV1002EveryRunnerSuppliesItsLister(t *testing.T) {
	static := map[string]bool{testid.Go: true, testid.RSpec: true, testid.Minitest: true}
	for _, runner := range Runners() {
		want, ok := testinventory.CurrentLister(runner.Name())
		if !ok || runner.Lister() != want {
			t.Errorf("%s lister %+v, want the inventory's %+v", runner.Name(), runner.Lister(), want)
		}
		kind := testinventory.KindExecuting
		if static[runner.Name()] {
			kind = testinventory.KindStatic
		}
		if runner.Lister().Kind != kind {
			t.Errorf("%s lister kind %q, want %q", runner.Name(), runner.Lister().Kind, kind)
		}
	}
}

// RNC-V1-013: the built-in default full command of each runner.
func TestRNCV1013BuiltInDefaults(t *testing.T) {
	none := func(string) bool { return false }
	rails := func(rel string) bool { return rel == "bin/rails" }
	for runner, want := range map[string][]string{
		testid.Go:         {"go", "test", goEveryPackage},
		testid.Vitest:     {"npx", "vitest", "run"},
		testid.Jest:       {"npx", "jest"},
		testid.Playwright: {"npx", "playwright", "test"},
		testid.Pytest:     {"python", "-m", "pytest"},
		testid.RSpec:      {"bundle", "exec", "rspec"},
		testid.Minitest:   {"bundle", "exec", "rake", "test"},
	} {
		r, _ := For(runner)
		if got := r.Default(Instance{Runner: runner, Root: "."}, none); !slices.Equal(got, want) {
			t.Errorf("%s default %q, want %q", runner, got, want)
		}
	}
	minitest, _ := For(testid.Minitest)
	if got := minitest.Default(Instance{Runner: testid.Minitest, Root: "."}, rails); !slices.Equal(got, []string{"bin/rails", "test"}) {
		t.Errorf("minitest default with bin/rails %q", got)
	}
}

func command(t *testing.T, request Request) Command {
	t.Helper()
	r, _ := For(request.Instance.Runner)
	got, err := r.Command(request)
	if err != nil {
		t.Fatalf("%s Command: %v", request.Instance.Key(), err)
	}
	return got
}

// RNC-V1-003, RNC-V1-012, RNC-V1-014: Go selects packages and one -run
// pattern by canonical ID, exact only when the argv runs nothing more.
func TestRNCV1003GoSelection(t *testing.T) {
	root := Instance{Runner: testid.Go, Root: ".", Config: "go.mod"}
	fullGo := []string{"go", "test", "-count=1", goEveryPackage}
	cases := []struct {
		name   string
		tests  []string
		argv   []string
		exact  bool
		reason string
	}{
		{"one package whole", []string{"go:a"}, []string{"go", "test", "-json", "-count=1", "./a"}, true, ""},
		{"root package whole", []string{"go:."}, []string{"go", "test", "-json", "-count=1", "."}, true, ""},
		{"named tests of one package", []string{"go:a::TestB", "go:a::TestA"}, []string{"go", "test", "-json", "-count=1", "-run", "^(TestA|TestB)$", "./a"}, true, ""},
		{"same names in two packages", []string{"go:a::TestA", "go:b::TestA"}, []string{"go", "test", "-json", "-count=1", "-run", "^(TestA)$", "./a", "./b"}, true, ""},
		{"different names in two packages", []string{"go:a::TestA", "go:b::TestB"}, []string{"go", "test", "-json", "-count=1", "-run", "^(TestA|TestB)$", "./a", "./b"}, false, "each runs the union"},
		{"whole beside named", []string{"go:a", "go:b::TestB"}, []string{"go", "test", "-json", "-count=1", "./a", "./b"}, false, "run whole"},
		{"subtest", []string{"go:a::TestA::case_1"}, []string{"go", "test", "-json", "-count=1", "-run", "^(TestA)$", "./a"}, false, "top-level test"},
		{"metacharacters quoted", []string{"go:a::Test.X"}, []string{"go", "test", "-json", "-count=1", "-run", `^(Test\.X)$`, "./a"}, true, ""},
	}
	for _, c := range cases {
		got := command(t, Request{Instance: root, Full: fullGo, Tests: c.tests, Tier: TierLine})
		if !slices.Equal(got.Argv, c.argv) || got.Exact != c.exact || !strings.Contains(got.Reason, c.reason) || got.Tier != TierLine || got.Dir != "." {
			t.Errorf("%s: %+v, want argv %q exact %v reason ~%q", c.name, got, c.argv, c.exact, c.reason)
		}
		if !got.Exact && got.Reason == "" {
			t.Errorf("%s: not exact without a reason", c.name)
		}
	}
	if got := command(t, Request{Instance: root, Full: fullGo}); !slices.Equal(got.Argv, []string{"go", "test", "-json", "-count=1", goEveryPackage}) || got.Tier != TierFull || !got.Exact {
		t.Errorf("full: %+v", got)
	}
	nested := Instance{Runner: testid.Go, Root: "tools/x", Config: "tools/x/go.mod"}
	if got := command(t, Request{Instance: nested, Full: fullGo, Tests: []string{"go:tools/x/y::TestY", "go:tools/x::TestY"}}); !slices.Equal(got.Argv, []string{"go", "test", "-json", "-count=1", "-run", "^(TestY)$", ".", "./y"}) || got.Dir != "tools/x" || got.Tier != TierFile {
		t.Errorf("nested instance: %+v", got)
	}
}

// RNC-V1-014: an instance refuses a test another instance owns.
func TestRNCV1014InstanceOwnsItsTests(t *testing.T) {
	for _, c := range []struct {
		instance Instance
		test     string
	}{
		{Instance{Runner: testid.Vitest, Root: "apps/web"}, "vitest:apps/admin/a.test.ts"},
		{Instance{Runner: testid.Vitest, Root: "apps/web"}, "jest:apps/web/a.test.ts"},
		{Instance{Runner: testid.Pytest, Root: "services/a"}, "pytest:services/b/test_x.py::test_x"},
		{Instance{Runner: testid.Go, Root: "tools/x"}, "go:tools/y::TestY"},
	} {
		r, _ := For(c.instance.Runner)
		if _, err := r.Command(Request{Instance: c.instance, Full: []string{"run"}, Tests: []string{c.test}}); err == nil {
			t.Errorf("%s ran %s", c.instance.Key(), c.test)
		}
	}
	web := Instance{Runner: testid.Vitest, Root: "apps/web", Config: "apps/web/vitest.config.ts"}
	got := command(t, Request{Instance: web, Full: []string{"npx", "vitest", "run"}, Tests: []string{"vitest:apps/web/src/a.test.ts"}, Files: []string{"apps/web/src/a.test.ts", "apps/admin/src/a.test.ts"}, Report: "/tmp/r.json"})
	if got.Dir != "apps/web" || !got.Exact || !slices.Equal(got.Argv, []string{"npx", "vitest", "run", "--reporter=json", "--outputFile=/tmp/r.json", "src/a.test.ts"}) {
		t.Errorf("web instance: %+v", got)
	}
}

// RNC-V1-003: Vitest runs selected files; a run that may include more is
// recorded exact false with its reason.
func TestRNCV1003VitestDirectoryRunIsNotExact(t *testing.T) {
	instance := Instance{Runner: testid.Vitest, Root: ".", Config: "vitest.config.ts"}
	base := Request{Instance: instance, Full: []string{"npx", "vitest", "run"}, Report: "/r.json", Tier: TierFile}
	with := func(tests, files []string) Request {
		request := base
		request.Tests, request.Files = tests, files
		return request
	}
	exact := command(t, with([]string{"vitest:src/a.test.ts"}, []string{"src/a.test.ts", "src/b.test.ts"}))
	if !exact.Exact || exact.Reason != "" || !slices.Equal(exact.Argv, []string{"npx", "vitest", "run", "--reporter=json", "--outputFile=/r.json", "src/a.test.ts"}) {
		t.Errorf("exact file: %+v", exact)
	}
	for name, request := range map[string]Request{
		"no listing":        with([]string{"vitest:src/a.test.ts"}, nil),
		"substring match":   with([]string{"vitest:a.test.ts"}, []string{"a.test.ts", "src/a.test.ts"}),
		"test-level widens": with([]string{"vitest:src/a.test.ts::adds"}, []string{"src/a.test.ts"}),
	} {
		got := command(t, request)
		if got.Exact || got.Reason == "" {
			t.Errorf("%s: %+v, want exact false with a reason", name, got)
		}
	}
	if got := command(t, base); got.Tier != TierFull || !got.Exact || !slices.Equal(got.Argv, []string{"npx", "vitest", "run", "--reporter=json", "--outputFile=/r.json"}) {
		t.Errorf("full: %+v", got)
	}
}

// RNC-V1-003: Playwright filters by escaped file pattern and writes its
// report through PLAYWRIGHT_JSON_OUTPUT_NAME.
func TestRNCV1003PlaywrightSelection(t *testing.T) {
	instance := Instance{Runner: testid.Playwright, Root: "e2e", Config: "e2e/playwright.config.ts"}
	got := command(t, Request{Instance: instance, Full: []string{"npx", "playwright", "test"}, Tests: []string{"playwright:e2e/tests/login.spec.ts::chromium::logs in"}, Files: []string{"e2e/tests/login.spec.ts"}, Report: "/r.json", Tier: TierFlow})
	if got.Dir != "e2e" || got.Exact || got.Tier != TierFlow || !slices.Equal(got.Env, []string{"PLAYWRIGHT_JSON_OUTPUT_NAME=/r.json"}) ||
		!slices.Equal(got.Argv, []string{"npx", "playwright", "test", "--reporter=json", `tests/login\.spec\.ts`}) {
		t.Errorf("playwright: %+v", got)
	}
}

// RNC-V1-004: a runner without an implementation runs its full command for
// any selection, with a printed reason, and has no report parser.
func TestRNCV1004UnimplementedRunnerRunsFull(t *testing.T) {
	for _, runner := range []string{testid.Jest, testid.Pytest, testid.RSpec, testid.Minitest} {
		r, _ := For(runner)
		instance := Instance{Runner: runner, Root: "."}
		full := r.Default(instance, nil)
		got := command(t, Request{Instance: instance, Full: full, Tests: []string{runner + ":t/x"}})
		if !slices.Equal(got.Argv, full) || got.Tier != TierFull || got.Exact || !strings.HasPrefix(got.Reason, ReasonNotImplemented) {
			t.Errorf("%s: %+v", runner, got)
		}
		if _, err := r.Parse(ParseRequest{Instance: instance}); !errors.Is(err, ErrNotImplemented) {
			t.Errorf("%s Parse: %v", runner, err)
		}
	}
}

// RNC-V1-005, RNC-V1-012: the Go report gives one outcome and duration per
// test under its canonical ID, and a package that failed (to build, or with
// a failing test) is one package-level fail result.
func TestRNCV1005GoReport(t *testing.T) {
	report := strings.Join([]string{
		`{"Action":"run","Package":"example.test/m/a","Test":"TestA"}`,
		`{"Action":"pass","Package":"example.test/m/a","Test":"TestA","Elapsed":0.25}`,
		`{"Action":"fail","Package":"example.test/m/a","Test":"TestB/case_1","Elapsed":0.0014}`,
		`{"Action":"skip","Package":"example.test/m","Test":"TestRoot","Elapsed":0}`,
		`{"Action":"pass","Package":"example.test/m","Elapsed":0}`,
		`# example.test/m/broken`,
		`{"Action":"fail","Package":"example.test/m/broken","Elapsed":0.1,"FailedBuild":"example.test/m/broken"}`,
		`{"Action":"fail","Package":"example.test/m/a","Elapsed":0.3}`,
	}, "\n")
	r, _ := For(testid.Go)
	got, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Go, Root: "."}, Report: []byte(report), Module: "example.test/m"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "go:a::TestA", Outcome: OutcomePass, DurationMillis: 250, Granularity: testid.GranularityTest},
		{ID: "go:a::TestB::case_1", Outcome: OutcomeFail, DurationMillis: 1, Granularity: testid.GranularityTest},
		{ID: "go:.::TestRoot", Outcome: OutcomeSkip, Granularity: testid.GranularityTest},
		{ID: "go:broken", Outcome: OutcomeFail, DurationMillis: 100, Granularity: testid.GranularityPackage},
		{ID: "go:a", Outcome: OutcomeFail, DurationMillis: 300, Granularity: testid.GranularityPackage},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results\n got %+v\nwant %+v", got, want)
	}
	if _, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Go, Root: "."}, Report: []byte(`{"Action":"pass","Package":"other.test/x","Test":"TestX"}`), Module: "example.test/m"}); err == nil {
		t.Fatal("a package outside the module was mapped by guess")
	}
}

// RNC-V1-005: the Vitest JSON reporter report (Jest's shape).
func TestRNCV1005VitestReport(t *testing.T) {
	report := `{"testResults":[{"name":"/repo/web/src/a.test.ts","assertionResults":[
		{"ancestorTitles":["math"],"title":"adds","status":"passed","duration":3.4},
		{"ancestorTitles":["math"],"title":"divides","status":"failed","duration":1},
		{"ancestorTitles":[],"title":"later","status":"todo","duration":null}]}]}`
	r, _ := For(testid.Vitest)
	got, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Vitest, Root: "web"}, Report: []byte(report), Root: "/repo"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "vitest:web/src/a.test.ts::math::adds", Outcome: OutcomePass, DurationMillis: 3, Granularity: testid.GranularityTest},
		{ID: "vitest:web/src/a.test.ts::math::divides", Outcome: OutcomeFail, DurationMillis: 1, Granularity: testid.GranularityTest},
		{ID: "vitest:web/src/a.test.ts::later", Outcome: OutcomeSkip, Granularity: testid.GranularityTest},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results\n got %+v\nwant %+v", got, want)
	}
	if _, err := r.Parse(ParseRequest{Report: []byte(`{"testResults":[{"name":"/elsewhere/a.test.ts"}]}`), Root: "/repo"}); err == nil {
		t.Fatal("a file outside the repository was mapped")
	}
}

// RNC-V1-005: the Playwright JSON report, one result per spec and project.
func TestRNCV1005PlaywrightReport(t *testing.T) {
	report := `{"config":{"rootDir":"/repo/e2e"},"suites":[{"title":"login.spec.ts","file":"tests/login.spec.ts","specs":[
		{"title":"logs in","tests":[{"projectName":"chromium","status":"expected","results":[{"duration":120}]},
		                             {"projectName":"firefox","status":"flaky","results":[{"duration":100},{"duration":90}]}]}],
		"suites":[{"title":"errors","specs":[{"title":"bad password","tests":[{"projectName":"chromium","status":"unexpected","results":[{"duration":5}]}]}]}]}]}`
	r, _ := For(testid.Playwright)
	got, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Playwright, Root: "e2e"}, Report: []byte(report), Root: "/repo"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "playwright:e2e/tests/login.spec.ts::chromium::logs in", Outcome: OutcomePass, DurationMillis: 120, Granularity: testid.GranularityTest},
		{ID: "playwright:e2e/tests/login.spec.ts::firefox::logs in", Outcome: OutcomePass, DurationMillis: 190, Granularity: testid.GranularityTest},
		{ID: "playwright:e2e/tests/login.spec.ts::chromium::errors::bad password", Outcome: OutcomeFail, DurationMillis: 5, Granularity: testid.GranularityTest},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results\n got %+v\nwant %+v", got, want)
	}
}

// RNC-V1-006: a selected test the report does not name is NOT_RUN and the
// run is INCOMPLETE; a file-level result answers a test of its file only
// when the report names no test there.
func TestRNCV1006MissingResultsAreNotRun(t *testing.T) {
	results := []Result{
		{ID: "go:a::TestA", Outcome: OutcomePass, Granularity: testid.GranularityTest},
		{ID: "go:a::TestExtra", Outcome: OutcomePass, Granularity: testid.GranularityTest},
		{ID: "go:broken", Outcome: OutcomeFail, Granularity: testid.GranularityPackage},
	}
	run := Reconcile([]string{"go:a::TestA", "go:a::TestGone", "go:broken::TestX", "go:c"}, results)
	if !run.Incomplete || !slices.Equal(run.NotRun, []string{"go:a::TestGone", "go:c"}) {
		t.Fatalf("run %+v", run)
	}
	outcomes := map[string]string{}
	for _, result := range run.Results {
		outcomes[result.ID] = result.Outcome
	}
	if outcomes["go:a::TestGone"] != OutcomeNotRun || outcomes["go:c"] != OutcomeNotRun || outcomes["go:a::TestExtra"] != OutcomePass || len(run.Results) != 5 {
		t.Fatalf("results %+v", run.Results)
	}
	if run := Reconcile([]string{"go:a"}, results); run.Incomplete {
		t.Fatalf("a package selection answered by its tests: %+v", run)
	}
	if run := Reconcile(nil, nil); !run.Incomplete {
		t.Fatal("a full run with an empty report is complete")
	}
	if run := Reconcile([]string{"go:a::TestA"}, nil); !run.Incomplete || run.Results[0].Outcome != OutcomeNotRun {
		t.Fatalf("a missing report: %+v", run)
	}
}
