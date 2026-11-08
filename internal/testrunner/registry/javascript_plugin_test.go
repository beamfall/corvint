// SPDX-License-Identifier: Apache-2.0
package registry

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func javascriptFixture(t *testing.T, runner string) (*JavaScriptPlugin, testrunner.InventoryKey, string) {
	t.Helper()
	version := map[string]string{"vitest": "3.2.0", "jest": "30.0.0", "node-test": "22.22.1", "playwright": "1.63.0"}[runner]
	root := t.TempDir()
	key := testrunner.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"}
	d := jslinecov.Discovery{Key: key, Root: root, Version: version, ConfigKnown: true, ListOK: true, Tracked: []string{"a.test.js", "b.test.js", "playwright.config.ts"}}
	if runner == "playwright" {
		d.Config = "playwright.config.ts"
	}
	p, e := NewJavaScriptPlugin(d, filepath.Join(root, "runner"), nil)
	if e != nil {
		t.Fatal(e)
	}
	raw := `["a.test.js","b.test.js"]`
	switch runner {
	case "vitest":
		raw = `[{"file":"a.test.js","name":"one"},{"file":"b.test.js","name":"two"}]`
	case "node-test":
		raw = ""
	case "playwright":
		r, _ := json.Marshal(root)
		raw = `{"config":{"rootDir":` + string(r) + `},"suites":[{"specs":[{"id":"one","title":"one","file":"a.test.js","tests":[{"projectName":""}]},{"id":"two","title":"two","file":"b.test.js","tests":[{"projectName":""}]}]}]}`
	}
	in, e := p.Inventory(context.Background(), InventoryRequest{Key: key, Output: raw})
	if e != nil || len(in.Tests) != 2 {
		t.Fatal(in, e)
	}
	return p, key, raw
}
func TestRSPV0002JavaScriptSPIFullUnqualified(t *testing.T) {
	for _, runner := range []string{"vitest", "jest", "node-test", "playwright"} {
		t.Run(runner, func(t *testing.T) {
			p, key, raw := javascriptFixture(t, runner)
			s := NewSelectionSession(p, key, true)
			in := s.Inventory(context.Background(), raw)
			plan := s.Plan(in, []testrunner.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
			if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || len(plan.Tests) != 2 {
				t.Fatal(plan)
			}
			key.ConfigDigest = strings.Repeat("c", 64)
			if in, e := p.Inventory(context.Background(), InventoryRequest{Key: key, Output: raw}); e == nil || in.Complete {
				t.Fatal(in, e)
			}
		})
	}
}
func TestRSPV0004JavaScriptNativeSuperset(t *testing.T) {
	for _, runner := range []string{"vitest", "jest", "node-test", "playwright"} {
		t.Run(runner, func(t *testing.T) {
			p, key, _ := javascriptFixture(t, runner)
			req := CommandRequest{Key: key, Purpose: "run", RunDir: t.TempDir(), Tests: []string{p.inventory.Tests[0].ID}}
			v, superset, e := p.Prepare(req)
			if e != nil || len(v.Argv) == 0 {
				t.Fatal(v, e)
			}
			if (runner == "vitest" || runner == "playwright") && !superset {
				t.Fatal("superset hidden")
			}
			if runner == "node-test" && len(v.Files["node.mjs"]) == 0 {
				t.Fatal("existing reporter not reused")
			}
			req.Tests = []string{"missing"}
			if _, _, e = p.Prepare(req); e == nil {
				t.Fatal("unknown selected")
			}
			req.Tests = []string{}
			if _, _, e = p.Prepare(req); e == nil {
				t.Fatal("empty invoked full")
			}
		})
	}
}
func TestRSPV0005JavaScriptNativeAbsent(t *testing.T) {
	p, key, _ := javascriptFixture(t, "jest")
	out, e := p.Evidence(context.Background(), EvidenceRequest{Runner: key.Runner})
	if e != nil || len(out) != 2 {
		t.Fatal(out, e)
	}
	var c testevidence.Component[testevidence.ExecutedFile]
	if json.Unmarshal(out[0].Executed, &c) != nil || c.State != testevidence.Absent || c.Reason == "" {
		t.Fatal(c)
	}
	if _, e = p.Evidence(context.Background(), EvidenceRequest{Runner: key.Runner, Artifacts: []string{"unadmitted-file"}}); e == nil {
		t.Fatal("read arbitrary artifact")
	}
}

type javascriptTrace struct{ q proctrace.Request }

func (b *javascriptTrace) Trace(_ context.Context, q proctrace.Request) proctrace.Result {
	b.q = q
	return proctrace.Result{Backend: "fixture", Outcome: proctrace.Outcome{Started: true}}
}
func TestRSPV0018PlaywrightServerInsideTracer(t *testing.T) {
	p, key, _ := javascriptFixture(t, "playwright")
	work := t.TempDir()
	cmd, e := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "collect", RunDir: work})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(cmd.Argv[1:4], []string{"test", "--reporter=json", "--config"}) || cmd.Env["NODE_V8_COVERAGE"] != filepath.Join(work, "node-coverage") {
		t.Fatal(cmd)
	}
	backend := &javascriptTrace{}
	q := proctrace.Request{Executable: cmd.Argv[0], Dir: cmd.Cwd, Args: cmd.Argv[1:], EventCap: 10, PathCap: 10, Repositories: []proctrace.Repository{{ID: "repo", Root: cmd.Cwd}}, Env: []string{"NODE_V8_COVERAGE=old"}}
	if _, e = TraceJavaScriptCommand(context.Background(), cmd, q, backend); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(backend.q.Env, "\n"), "NODE_V8_COVERAGE="+filepath.Join(work, "node-coverage")) {
		t.Fatal(backend.q.Env)
	}
	q.Args = []string{"different"}
	if _, e = TraceJavaScriptCommand(context.Background(), cmd, q, backend); e == nil {
		t.Fatal("unbound trace")
	}
}

// RSP-V0-003: a passing FULL retry cannot erase an independently validated
// failure when a sibling result or aggregate uses an unsupported form.
func TestRSPV0003JavaScriptFailureSurvivesMalformedReport(t *testing.T) {
	for _, runner := range []string{"vitest", "jest", "jest-collect"} {
		t.Run(runner, func(t *testing.T) {
			kind := runner
			if kind == "jest-collect" {
				kind = "jest"
			}
			p, key, inventory := javascriptFixture(t, kind)
			if runner == "jest-collect" {
				p.discovery.Version = "30.4.0"
				inventory = `{"numTotalTests":2,"testResults":[{"name":"a.test.js","assertionResults":[{"fullName":"one","wouldRun":true}]},{"name":"b.test.js","assertionResults":[{"fullName":"two","wouldRun":true}]}]}`
			}
			for _, malformed := range []string{
				`{"name":"b.test.js","status":"passed"}`,
				`{"name":"b.test.js","status":"passed","assertionResults":[{"fullName":"two","status":"future-terminal"}]}`,
				`{"name":"b.test.js","status":"passed","assertionResults":[{"fullName":"two","status":{}}]}`,
				`{"name":"b.test.js","status":"passed","assertionResults":{}}`,
			} {
				for _, reverse := range []bool{false, true} {
					s := NewSelectionSession(p, key, true)
					in := s.Inventory(context.Background(), inventory)
					if !in.Complete {
						t.Fatal(in)
					}
					failed := `{"name":"a.test.js","status":"passed","assertionResults":[{"fullName":"one","status":"failed"}]}`
					files := failed + "," + malformed
					if reverse {
						files = malformed + "," + failed
					}
					first := `{"numTotalTests":2,"testResults":[` + files + `]}`
					calls := 0
					got := s.Run(context.Background(), in, nil, nil, t.TempDir(), true, func(context.Context, CommandResponse) (string, error) {
						calls++
						if calls == 1 {
							return first, nil
						}
						return `{"numTotalTests":2,"testResults":[{"name":"a.test.js","status":"passed","assertionResults":[{"fullName":"one","status":"passed"}]},{"name":"b.test.js","status":"passed","assertionResults":[{"fullName":"two","status":"passed"}]}]}`, nil
					})
					if got.Succeeded || calls != 2 || !got.NeedsRequalification {
						t.Fatalf("failure erased: malformed=%s reverse=%v calls=%d result=%+v", malformed, reverse, calls, got)
					}
				}
			}
		})
	}
}

func TestRSPV0002JavaScriptExplicitDiscoveryMatchesExecution(t *testing.T) {
	for _, runner := range []string{"playwright", "jest"} {
		for _, name := range []string{"tests/a+b.spec.ts", "tests/a(b).spec.ts", "tests/a^b$.spec.ts", "tests/a|b.spec.ts"} {
			t.Run(runner+"/"+name, func(t *testing.T) {
				p, key, _ := javascriptFixture(t, runner)
				p.discovery.Tracked = append(p.discovery.Tracked, name, "tests/ab.spec.ts")
				p.discovery.Files = []string{name}
				p.discovery.NodeOptions = "--require ./setup.cjs"
				list, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "inventory"})
				if err != nil {
					t.Fatal(err)
				}
				run, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: t.TempDir()})
				if err != nil {
					t.Fatal(err)
				}
				if got, ok := list.Env["NODE_OPTIONS"]; !ok || got != run.Env["NODE_OPTIONS"] {
					t.Fatalf("discovery NODE_OPTIONS=%q (set=%v) run=%q", got, ok, run.Env["NODE_OPTIONS"])
				}
				last := list.Argv[len(list.Argv)-1]
				if last != run.Argv[len(run.Argv)-1] {
					t.Fatalf("list=%v run=%v", list.Argv, run.Argv)
				}
				if runner == "jest" {
					if list.Argv[len(list.Argv)-2] != "--runTestsByPath" || last != name {
						t.Fatal(list)
					}
				} else {
					re, err := regexp.Compile(last)
					if err != nil || !re.MatchString(filepath.Join(p.discovery.Root, name)) || re.MatchString(filepath.Join(p.discovery.Root, "tests/ab.spec.ts")) || re.MatchString(filepath.Join(p.discovery.Root, "prefix", name)) || re.MatchString(filepath.Join(p.discovery.Root, name)+"suffix") {
						t.Fatalf("nonliteral: %q %v", last, err)
					}
				}
			})
		}
	}
}
