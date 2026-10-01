package dynamic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("testdata", name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestNativeRunnerGeneratedMixedReports(t *testing.T) {
	for _, c := range []struct {
		runner, file string
		complete     bool
	}{{"node-test", "node-report.jsonl", true}, {"pytest", "pytest.xml", false}, {"bun-test", "bun.xml", true}, {"deno-test", "deno.xml", true}, {"jest", "jest.json", true}, {"vitest", "vitest.json", true}, {"ava", "ava.tap", true}, {"rspec", "rspec.json", true}, {"unittest", "unittest.json", true}, {"minitest", "minitest.json", true}, {"test-unit", "testunit.json", true}, {"webdriverio", "wdio.json", true}, {"nightwatch", "nightwatch.json", true}, {"testcafe", "testcafe.json", true}, {"storybook-vitest", "storybook-vitest.json", true}} {
		t.Run(c.runner, func(t *testing.T) {
			in := tr.Input{Runner: c.runner, ExitCode: 1, Reports: map[string][]byte{c.file: fixture(t, c.file)}}
			if c.runner == "ava" {
				in.Stdout = in.Reports[c.file]
				in.Reports = nil
			}
			o, e := Parse(in)
			if e != nil {
				t.Fatal(e)
			}
			if o.Complete != c.complete {
				t.Fatalf("complete=%v problems=%+v", o.Complete, o.Problems)
			}
			states := map[string]int{}
			for _, x := range o.Tests {
				states[x.State]++
			}
			for _, s := range []string{tr.Passed, tr.Failed, tr.Skipped} {
				if states[s] != 1 {
					t.Fatalf("states=%v tests=%+v", states, o.Tests)
				}
			}
			for _, flag := range []string{"timeout", "interrupt", "overflow"} {
				copy := in
				copy.TimedOut = flag == "timeout"
				copy.Interrupted = flag == "interrupt"
				copy.Overflow = flag == "overflow"
				bad, e := Parse(copy)
				if e != nil || bad.Complete {
					t.Fatalf("boundary %s accepted: %+v %v", flag, bad, e)
				}
			}
		})
	}
}
func TestNativeCollectionAndRetryReports(t *testing.T) {
	for _, c := range []struct {
		runner, file string
		exit         int
		complete     bool
	}{{"storybook-test-runner", "storybook-legacy-collection.json", 1, false}, {"jest", "jest-collection.json", 1, false}, {"vitest", "vitest-collection.json", 1, false}, {"ava", "ava-collection.tap", 1, false}, {"jest", "jest-retry.json", 0, true}, {"nightwatch", "nightwatch-retry.json", 0, true}, {"vitest", "vitest-retry.json", 0, false}} {
		t.Run(c.file, func(t *testing.T) {
			in := tr.Input{Runner: c.runner, ExitCode: c.exit, Reports: map[string][]byte{c.file: fixture(t, c.file)}}
			if c.runner == "ava" {
				in.Stdout = in.Reports[c.file]
				in.Reports = nil
			}
			o, e := Parse(in)
			if e != nil {
				t.Fatal(e)
			}
			if o.Complete != c.complete {
				t.Fatalf("%+v", o)
			}
			if strings.Contains(c.file, "retry") && o.Tests[0].State != tr.Flaky {
				t.Fatalf("retry was flattened: %+v", o)
			}
			if c.file == "jest-retry.json" && len(o.Tests[0].Attempts) != 2 {
				t.Fatalf("attempts lost: %+v", o)
			}
		})
	}
}
func TestMissingMalformedUnknownAndExitConflict(t *testing.T) {
	for _, r := range Runners() {
		o, e := Parse(tr.Input{Runner: r})
		if e == nil && o.Complete {
			t.Fatalf("missing report accepted: %s", r)
		}
	}
	for _, b := range [][]byte{[]byte(`{}`), []byte(`{"numTotalTests":0,"success":true,"testResults":[]} trailing`), []byte(`{"numTotalTests":1,"success":true,"testResults":[{"name":"a","status":"passed","assertionResults":[{"fullName":"test","status":"invented"}]}]}`)} {
		o, e := Parse(tr.Input{Runner: "jest", Reports: map[string][]byte{"r": b}})
		if e == nil && o.Complete {
			t.Fatalf("bad report accepted: %s", b)
		}
	}
	o, e := Parse(tr.Input{Runner: "jest", ExitCode: 0, Reports: map[string][]byte{"r": fixture(t, "jest.json")}})
	if e != nil || o.Complete {
		t.Fatalf("exit conflict accepted %+v %v", o, e)
	}
}

func TestPlaywrightNativeRetriesAndGlobalErrors(t *testing.T) {
	b := fixture(t, "playwright.json")
	o, e := Parse(tr.Input{Runner: "playwright", ExitCode: 1, Reports: map[string][]byte{"r": b}})
	if e != nil || !o.Complete || len(o.Tests) != 4 {
		t.Fatalf("%+v %v", o, e)
	}
	flaky := 0
	for _, x := range o.Tests {
		if x.State == tr.Flaky {
			flaky++
			if len(x.Attempts) != 2 || x.Attempts[0].State != tr.Failed || x.Attempts[1].State != tr.Passed {
				t.Fatalf("retry lost: %+v", x)
			}
		}
	}
	if flaky != 1 {
		t.Fatalf("flaky=%d", flaky)
	}
	bad := []byte(`{"suites":[],"errors":[{"message":"collection failed"}],"stats":{"expected":0,"unexpected":0,"flaky":0,"skipped":0}}`)
	o, e = Parse(tr.Input{Runner: "playwright", ExitCode: 1, Reports: map[string][]byte{"r": bad}})
	if e != nil || o.Complete || len(o.Problems) == 0 {
		t.Fatalf("global failure accepted: %+v %v", o, e)
	}
}

func TestBrowserNativeSchemaBoundaries(t *testing.T) {
	// These are schema contract cases from official reporter source, not runtime qualification.
	for _, c := range []struct {
		runner, body string
		count        int
	}{
		{"testcafe", `{"endTime":"2026-10-01T00:00:00Z","total":2,"passed":1,"skipped":1,"fixtures":[{"name":"suite","path":"a.js","tests":[{"name":"p","errs":[]},{"name":"f","errs":["failure"]},{"name":"s","errs":[],"skipped":true}]}]}`, 3},
		{"cypress", `{"stats":{"tests":3,"passes":1,"failures":1,"pending":1,"end":"2026-10-01T00:00:00Z"},"tests":[{"title":"p","fullTitle":"p"},{"title":"f","fullTitle":"f"},{"title":"s","fullTitle":"s"}],"passes":[{"title":"p","fullTitle":"p"}],"failures":[{"title":"f","fullTitle":"f"}],"pending":[{"title":"s","fullTitle":"s"}]}`, 3},
		{"webdriverio", `{"end":"2026-10-01T00:00:00Z","framework":"mocha","state":{"passed":1,"failed":1,"skipped":1},"suites":[{"name":"suite","sessionId":"device","tests":[{"name":"p","state":"passed"},{"name":"f","state":"failed"},{"name":"s","state":"skipped"}],"hooks":[]}]}`, 3},
		{"nightwatch", `{"name":"suite","systemerr":"","report":{"testsCount":2,"errorsCount":0,"completed":{"p":{"status":"pass"},"f":{"status":"fail","failed":1}},"skipped":["s"]}}`, 3},
	} {
		t.Run(c.runner, func(t *testing.T) {
			o, e := Parse(tr.Input{Runner: c.runner, ExitCode: 1, Reports: map[string][]byte{"r": []byte(c.body)}})
			if e != nil || !o.Complete || len(o.Tests) != c.count {
				t.Fatalf("%+v %v", o, e)
			}
			o, e = Parse(tr.Input{Runner: c.runner, Reports: map[string][]byte{"r": []byte(`{}`)}})
			if e == nil && o.Complete {
				t.Fatal("empty JSON accepted")
			}
		})
	}
}
func TestBuildAllProfilesAndRejectInjection(t *testing.T) {
	for _, r := range Runners() {
		req := tr.Request{Runner: r, Executable: "/trusted/runner", ReportDir: "/fresh/report", Selectors: []string{"tests/example.test"}, Config: "/trusted/config", Project: "configured", ReportFiles: []string{"shard.json"}}
		if r == "storybook-test-runner" || r == "vitest" || r == "storybook-vitest" {
			req.Selectors = nil
			if r == "storybook-test-runner" {
				req.Config = ""
			}
		}
		v, e := Build(req)
		if e != nil {
			t.Fatalf("%s: %v", r, e)
		}
		if v.GracefulInterrupt != (r == "playwright") {
			t.Fatalf("%s incorrect native shutdown profile", r)
		}
		if len(v.Argv) < 2 || v.Argv[0] == req.Executable {
			t.Fatalf("%s argv=%v", r, v.Argv)
		}
		for _, b := range v.Files {
			if len(b) == 0 {
				t.Fatalf("%s empty reporter", r)
			}
		}
		req.Selectors = []string{"--config=evil"}
		if _, e = Build(req); e == nil {
			t.Fatalf("%s selector accepted flag", r)
		}
	}
}

func TestActualCypressAndRailsReports(t *testing.T) {
	for _, c := range []struct {
		runner, file string
		want         map[string]int
	}{
		{"cypress", "cypress-native-mixed.json", map[string]int{tr.Passed: 1, tr.Failed: 1, tr.Skipped: 1, tr.Flaky: 1}},
		{"rails-test", "rails-native-mixed.json", map[string]int{tr.Passed: 2, tr.Failed: 1, tr.Skipped: 1}},
	} {
		t.Run(c.runner, func(t *testing.T) {
			o, e := Parse(tr.Input{Runner: c.runner, ExitCode: 1, Reports: map[string][]byte{"r": fixture(t, c.file)}})
			if e != nil || !o.Complete {
				t.Fatalf("%+v %v", o, e)
			}
			got := map[string]int{}
			for _, x := range o.Tests {
				got[x.State]++
				if x.State == tr.Flaky && (len(x.Attempts) != 2 || x.Attempts[0].State != tr.Failed || x.Attempts[1].State != tr.Passed) {
					t.Fatal(x)
				}
			}
			for k, n := range c.want {
				if got[k] != n {
					t.Fatalf("%v", got)
				}
			}
		})
	}
	o, e := Parse(tr.Input{Runner: "cypress", ExitCode: 1, Reports: map[string][]byte{"r": fixture(t, "cypress-native-collection.json")}})
	if e != nil || o.Complete || len(o.Problems) != 1 || o.Problems[0].Code != "collection-error" {
		t.Fatalf("%+v %v", o, e)
	}
	o, e = Parse(tr.Input{Runner: "rails-test", ExitCode: 1, Stderr: fixture(t, "rails-native-collection.stderr")})
	if e != nil || o.Complete {
		t.Fatalf("%+v %v", o, e)
	}
}
