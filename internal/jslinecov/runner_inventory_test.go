// SPDX-License-Identifier: Apache-2.0
package jslinecov

import (
	"crypto/sha1"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov/v8"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
)

func discovery(runner, version string) Discovery {
	return Discovery{Key: testrunner.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"}, Root: "/repo", Version: version, Tracked: []string{"a.test.js", "b.test.ts", "test/helper.cjs", "ordinary.js", "node_modules/test/ignored.js"}, ConfigKnown: true, ListOK: true}
}
func TestRSPV0002NativeInventories(t *testing.T) {
	for _, tc := range []struct {
		runner, version, report string
		count                   int
		firstArg                string
	}{
		{"vitest", "3.2.0", `[{"name":"suite works","file":"/repo/a.test.js"}]`, 1, "list"},
		{"jest", "30.0.0", `["/repo/a.test.js"]`, 1, "--listTests"},
		{"jest", "30.4.0", `{"numTotalTests":1,"testResults":[{"name":"/repo/a.test.js","assertionResults":[{"fullName":"suite works","status":"passed","wouldRun":true}]}]}`, 1, "--collectTests"},
		{"node-test", "22.22.1", "", 3, ""},
	} {
		t.Run(tc.runner+tc.version, func(t *testing.T) {
			d := discovery(tc.runner, tc.version)
			a, e := Inventory(d, tc.report)
			if e != nil || len(a.Tests) != tc.count {
				t.Fatalf("%+v %v", a, e)
			}
			b, e := Inventory(d, tc.report)
			if e != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("noncanonical", e)
			}
			args, e := ListArgs(d)
			if e != nil {
				t.Fatal(e)
			}
			if tc.firstArg != "" && args[0] != tc.firstArg {
				t.Fatal(args)
			}
			if tc.runner == "node-test" && args != nil {
				t.Fatal("node discovery executed", args)
			}
		})
	}
}
func TestRSPV0002UnavailableAndMalformed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Discovery)
		report string
	}{
		{"version", func(d *Discovery) { d.Version = "unknown" }, `[]`},
		{"future", func(d *Discovery) { d.Version = "99.0.0" }, `[]`},
		{"config", func(d *Discovery) { d.ConfigKnown = false }, `[]`},
		{"list-failed", func(d *Discovery) { d.ListOK = false }, `[]`},
		{"glob", func(d *Discovery) { d.Files = []string{"*.js"} }, `[]`},
		{"truncated", func(*Discovery) {}, `[{`},
		{"null", func(*Discovery) {}, `null`},
		{"duplicate-key", func(*Discovery) {}, `[{"file":"a.test.js","file":"b.test.ts","name":"x"}]`},
		{"outside", func(*Discovery) {}, `[{"file":"/outside/a.js","name":"x"}]`},
		{"duplicate-id", func(*Discovery) {}, `[{"file":"a.test.js","name":"x"},{"file":"a.test.js","name":"x"}]`},
		{"cap", func(*Discovery) {}, strings.Repeat(" ", MaxRunnerArtifact+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := discovery("vitest", "3.2.0")
			tc.change(&d)
			in, e := Inventory(d, tc.report)
			if e == nil || in.Complete || in.Reason == "" {
				t.Fatalf("%+v %v", in, e)
			}
			if !testrunner.PlanRunSet(in, d.Key, nil, nil).Full {
				t.Fatal("narrowed")
			}
		})
	}
}
func TestRSPV0002NodeExplicitFiles(t *testing.T) {
	d := discovery("node-test", "22.22.1")
	d.Files = []string{"ordinary.js"}
	in, e := Inventory(d, "")
	if e != nil || len(in.Tests) != 1 || in.Tests[0].Path != "ordinary.js" {
		t.Fatal(in, e)
	}
	d.Files = []string{"missing.js"}
	if _, e = Inventory(d, ""); e == nil {
		t.Fatal("untracked explicit file")
	}
}

func TestRSPV0002VitestMultiProjectRefused(t *testing.T) {
	for _, version := range []string{"2.1.9", "3.2.0", "4.0.0"} {
		for _, raw := range []string{
			`[{"file":"a.test.js","name":"works","projectName":"client"},{"file":"a.test.js","name":"works","projectName":"server"}]`,
			`[{"file":"a.test.js","name":"works"},{"file":"b.test.ts","name":"other","projectName":"server"}]`,
			`[{"file":"a.test.js","name":"works","projectName":"client"}]`,
		} {
			d := discovery("vitest", version)
			in, err := Inventory(d, raw)
			if err == nil || err.Error() != "config-unsupported" || in.Complete || in.Reason != "config-unsupported" || len(in.Tests) != 0 {
				t.Fatalf("project attribution invented: %+v, %v", in, err)
			}
			if !testrunner.PlanRunSet(in, d.Key, nil, nil).Full {
				t.Fatal("unsupported projects narrowed")
			}
		}
	}
}

func TestRSPV0002CommaPathRefusedBeforeExecution(t *testing.T) {
	for _, runner := range []string{"vitest", "jest", "node-test"} {
		version := map[string]string{"vitest": "3.2.0", "jest": "30.0.0", "node-test": "22.22.1"}[runner]
		d := discovery(runner, version)
		p := "a,b.test.js"
		d.Tracked = append(d.Tracked, p)
		d.Files = []string{p}
		if SafeRunnerPath(p) {
			t.Fatal("ambiguous selector admitted")
		}
		in, err := Inventory(d, `[]`)
		if err == nil || in.Complete || in.Reason != "inventory-unavailable" || !testrunner.PlanRunSet(in, d.Key, nil, nil).Full {
			t.Fatal(in, err)
		}
		if _, err := ListArgs(d); err == nil || err.Error() != "inventory-unavailable" {
			t.Fatal("discovery accepted unsafe path", err)
		}
		_, err = dynamic.Build(testrunner.Request{Runner: runner, Executable: "/bin/node", ReportDir: "/tmp/report", Selectors: []string{p}})
		if err == nil || err.Error() != `unsafe or ambiguous selector "a,b.test.js"` {
			t.Fatal("execution safety disagrees", err)
		}
		if !SafeRunnerPath("a-b.test.js") {
			t.Fatal("ordinary path rejected")
		}
	}
}
func TestRSPV0004FileSuperset(t *testing.T) {
	d := discovery("vitest", "3.2.0")
	in, e := Inventory(d, `[{"file":"a.test.js","name":"one"},{"file":"a.test.js","name":"two"}]`)
	if e != nil {
		t.Fatal(e)
	}
	p := testrunner.PlanRunSet(in, d.Key, []testrunner.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
	files, e := FilesFor(in, p.Tests)
	if e != nil || !reflect.DeepEqual(files, []string{"a.test.js"}) {
		t.Fatal(files, e)
	}
	if _, e = FilesFor(in, []string{"unknown"}); e == nil {
		t.Fatal("unknown filtered")
	}
	p = testrunner.PlanRunSet(in, d.Key, []testrunner.Exclusion{{ID: in.Tests[0].ID}}, nil)
	if len(p.Tests) != 2 {
		t.Fatal("stale excluded")
	}
}
func TestRSPV0003NativeTerminalReports(t *testing.T) {
	for _, tc := range []struct{ runner, version, inventory, report string }{
		{"vitest", "3.2.0", `[{"file":"a.test.js","name":"works"}]`, `{"numTotalTests":1,"testResults":[{"name":"a.test.js","status":"passed","assertionResults":[{"fullName":"works","status":"passed"}]}]}`},
		{"jest", "30.0.0", `["a.test.js"]`, `{"numTotalTests":1,"testResults":[{"name":"a.test.js","status":"passed","assertionResults":[{"fullName":"dynamic child","status":"passed"}]}]}`},
		{"jest", "30.4.0", `{"numTotalTests":1,"testResults":[{"name":"a.test.js","assertionResults":[{"fullName":"works","wouldRun":true}]}]}`, `{"numTotalTests":1,"testResults":[{"name":"a.test.js","assertionResults":[{"fullName":"works","status":"pending"}]}]}`},
		{"node-test", "22.22.1", "", "{\"type\":\"test:pass\",\"data\":{\"name\":\"dynamic child\"}}\n{\"type\":\"test:summary\",\"data\":{\"file\":\"/repo/a.test.js\",\"success\":true}}\n{\"type\":\"test:summary\",\"data\":{\"success\":true}}"},
	} {
		t.Run(tc.runner+tc.version, func(t *testing.T) {
			d := discovery(tc.runner, tc.version)
			d.Files = []string{"a.test.js"}
			in, e := Inventory(d, tc.inventory)
			if e != nil {
				t.Fatal(e)
			}
			results, e := Results(d, tc.report)
			if e != nil {
				t.Fatal(e)
			}
			plan := testrunner.PlanRunSet(in, d.Key, nil, nil)
			if r := testrunner.ReconcileInventory(in, d.Key, plan, results, false); !r.Succeeded {
				t.Fatal(r)
			}
			if r := testrunner.ReconcileInventory(in, d.Key, plan, nil, false); r.Succeeded || !r.Full {
				t.Fatal("missing accepted", r)
			}
			if _, e = Results(d, `{}`); e == nil {
				t.Fatal("malformed accepted")
			}
		})
	}
}
func TestRSPV0003NodeFailedSummary(t *testing.T) {
	d := discovery("node-test", "22.22.1")
	out, e := Results(d, "{\"type\":\"test:summary\",\"data\":{\"file\":\"a.test.js\",\"success\":false}}\n{\"type\":\"test:summary\",\"data\":{\"success\":false}}")
	if e == nil || len(out) != 1 || out[0].Outcome != "failed" {
		t.Fatal(out, e)
	}
	if _, e = Results(d, `{"type":"test:pass","data":{"file":"a.test.js"}}`); e == nil {
		t.Fatal("partial events passed")
	}
}
// A Node report truncated before a file's test:summary keeps that file's
// validated test:fail as a failed result, so a passing FULL retry cannot make
// the inventoried run succeed; a later summary does not add a second result.
func TestRSPV0003NodeTruncatedFailureKept(t *testing.T) {
	d := discovery("node-test", "22.22.1")
	d.Files = []string{"a.test.js"}
	in, e := Inventory(d, "")
	if e != nil {
		t.Fatal(e)
	}
	truncated := "{\"type\":\"test:fail\",\"data\":{\"name\":\"x\",\"file\":\"/repo/a.test.js\",\"details\":{\"type\":\"test\"}}}\n{\"type\":\"test:sum"
	out, e := Results(d, truncated)
	if e == nil || len(out) != 1 || out[0].Outcome != "failed" || out[0].ID != TestID("node-test", "a.test.js", "", "") {
		t.Fatal(out, e)
	}
	passing := "{\"type\":\"test:summary\",\"data\":{\"file\":\"/repo/a.test.js\",\"success\":true}}\n{\"type\":\"test:summary\",\"data\":{\"success\":true}}"
	reports := []string{truncated, passing}
	attempt := 0
	run := testrunner.RunInventoried(in, d.Key, testrunner.PlanRunSet(in, d.Key, nil, nil), true, func(testrunner.RunSet) ([]testrunner.RunnerResult, bool, error) {
		r, err := Results(d, reports[attempt])
		attempt++
		return r, false, err
	})
	if run.Succeeded {
		t.Fatal("truncated failure hidden by a passing retry", run)
	}
	complete := "{\"type\":\"test:fail\",\"data\":{\"name\":\"x\",\"file\":\"/repo/a.test.js\",\"details\":{\"type\":\"test\"}}}\n{\"type\":\"test:summary\",\"data\":{\"file\":\"/repo/a.test.js\",\"success\":false}}\n{\"type\":\"test:summary\",\"data\":{\"success\":false}}"
	if out, _ = Results(d, complete); len(out) != 1 || out[0].Outcome != "failed" {
		t.Fatal("summary double-counted", out)
	}
}
func source(text string) Source {
	raw := []byte(text)
	h := sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(raw))), raw...))
	return Source{OID: fmt.Sprintf("%x", h), Text: raw}
}
func TestRSPV0005NativeCoverage(t *testing.T) {
	c := CoverageCapture{Repository: "repo", Sources: map[string]Source{"lib.js": source("function f() {\n let x = 1;\n return x;\n}\n")}, Lines: v8.Lines{"lib.js": {2: true, 3: false}}, Loaded: []string{"lib.js"}, Complete: true, LoadedComplete: true}
	n := NativeCoverage(c)
	if n.Executed.State != testevidence.Present || n.Regions.State != testevidence.Present || len(n.Regions.Items[0].Bodies) != 1 || n.Loaded.State != testevidence.Present {
		t.Fatalf("%+v", n)
	}
	for _, mutate := range []func(*CoverageCapture){func(c *CoverageCapture) { c.Complete = false }, func(c *CoverageCapture) { c.LoadedComplete = false }, func(c *CoverageCapture) {
		c.Sources = map[string]Source{"lib.js": {OID: strings.Repeat("a", 40), Text: []byte("wrong")}}
	}} {
		copy := c
		mutate(&copy)
		n := NativeCoverage(copy)
		if n.Executed.State == testevidence.Present && n.Loaded.State == testevidence.Present {
			t.Fatal("unknown certified")
		}
	}
}
func TestRSPV0005UnsupportedRegions(t *testing.T) {
	s := source("const x = <div/>;\n")
	n := NativeCoverage(CoverageCapture{Repository: "repo", Sources: map[string]Source{"a.jsx": s}, Lines: v8.Lines{"a.jsx": {1: true}}, Complete: true})
	if n.Regions.State != testevidence.Incomplete || n.Regions.Reason == "" {
		t.Fatal(n)
	}
}

func TestRSPV0003JestFileCannotHideFailedAssertion(t *testing.T) {
	d := discovery("jest", "30.0.0")
	raw := `{"numTotalTests":1,"testResults":[{"name":"a.test.js","status":"passed","assertionResults":[{"fullName":"fails","status":"failed"}]}]}`
	out, err := Results(d, raw)
	if err != nil || len(out) != 1 || out[0].Outcome != "failed" {
		t.Fatal(out, err)
	}
	raw = strings.Replace(raw, `"status":"failed"`, `"status":"unknown"`, 1)
	if _, err = Results(d, raw); err == nil {
		t.Fatal("unknown assertion passed")
	}
}

func TestRSPV0002ExplicitFilesUnknownFilterRefused(t *testing.T) {
	// Native substring filters cannot certify literal explicit-file discovery.
	d := discovery("vitest", "3.2.0")
	d.Files = []string{"a.test.js"}
	if _, err := ListArgs(d); err == nil {
		t.Fatal("unsupported explicit filter admitted")
	}
}

func TestRSPV0003TerminalFailureIndependentOfSiblingAndAggregate(t *testing.T) {
	for _, runner := range []string{"vitest", "jest"} {
		d := discovery(runner, map[string]string{"vitest": "3.2.0", "jest": "30.0.0"}[runner])
		for _, unknown := range []string{`{"fullName":"later","status":"future"}`, `{"fullName":"later","status":{}}`} {
			for _, total := range []string{`2`, `"unknown"`} {
				raw := `{"numTotalTests":` + total + `,"testResults":[{"name":"a.test.js","status":"passed","assertionResults":[{"fullName":"one","status":"failed"},` + unknown + `]}]}`
				results, err := Results(d, raw)
				if err == nil || len(results) != 1 || results[0].Outcome != "failed" {
					t.Fatal(results, err)
				}
			}
		}
	}
}

func TestRSPV0003JestFileFailureRetainedAcrossRetry(t *testing.T) {
	d := discovery("jest", "30.0.0")
	in, err := Inventory(d, `["/repo/a.test.js","/repo/b.test.ts"]`)
	if err != nil {
		t.Fatal(err)
	}
	passing := `{"numTotalTests":2,"testResults":[{"name":"a.test.js","status":"passed","assertionResults":[{"fullName":"one","status":"passed"}]},{"name":"b.test.ts","status":"passed","assertionResults":[{"fullName":"two","status":"passed"}]}]}`
	for _, assertions := range []string{`[{"fullName":"one","status":{}}]`, `[{"fullName":"one","status":"future"}]`, `{"futureAssertions":true}`, `null`} {
		t.Run(assertions, func(t *testing.T) {
			first := `{"numTotalTests":1,"testResults":[{"name":"a.test.js","status":"failed","assertionResults":` + assertions + `}]}`
			results, err := Results(d, first)
			if err == nil || len(results) != 1 || results[0].Outcome != "failed" {
				t.Fatalf("failure or diagnostic lost: %+v %v", results, err)
			}
			// An explicit pass cannot certify unrecognized assertion details.
			unknown := strings.Replace(first, `"status":"failed"`, `"status":"passed"`, 1)
			if results, err := Results(d, unknown); err == nil || len(results) != 0 {
				t.Fatalf("unknown assertions certified pass: %+v %v", results, err)
			}
			calls := 0
			out := testrunner.RunInventoried(in, d.Key, testrunner.PlanRunSet(in, d.Key, nil, nil), true, func(plan testrunner.RunSet) ([]testrunner.RunnerResult, bool, error) {
				calls++
				raw := first
				if calls == 2 {
					if !plan.Full {
						t.Fatal("retry was not FULL", plan)
					}
					raw = passing
				}
				results, err := Results(d, raw)
				return results, false, err
			})
			if out.Succeeded || calls != 2 || !out.NeedsRequalification || len(out.Attempts[0].Results) != 1 || out.Attempts[0].Results[0].Outcome != "failed" || !out.Attempts[1].Reconciliation.Succeeded {
				t.Fatalf("failure lost across passing FULL retry: %+v", out)
			}
		})
	}
}
