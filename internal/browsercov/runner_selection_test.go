// SPDX-License-Identifier: Apache-2.0
package browsercov

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

const selectionList = `{"config":{"rootDir":"/repo"},"suites":[{"specs":[{"id":"native-id","title":"loads page","file":"tests/page.spec.ts","tests":[{"projectName":"chromium"},{"projectName":"firefox"}]}]}],"errors":[]}`

func selectionDiscovery() jslinecov.Discovery {
	return jslinecov.Discovery{Key: testrunner.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: "playwright", ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"}, Root: "/repo", Version: "1.63.0", ConfigKnown: true, ListOK: true, Tracked: []string{"tests/page.spec.ts"}}
}
func TestRSPV0018PlaywrightInventory(t *testing.T) {
	d := selectionDiscovery()
	in, e := PlaywrightInventory(d, selectionList)
	if e != nil || len(in.Tests) != 2 {
		t.Fatal(in, e)
	}
	again, e := PlaywrightInventory(d, selectionList)
	if e != nil || !reflect.DeepEqual(in, again) {
		t.Fatal("unstable", e)
	}
	args, e := jslinecov.ListArgs(d)
	if e != nil || !reflect.DeepEqual(args, []string{"test", "--list", "--reporter=json"}) {
		t.Fatal(args, e)
	}
}
func TestRSPV0018PlaywrightMalformed(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, strings.Replace(selectionList, `"id":"native-id",`, "", 1), strings.Replace(selectionList, `"errors":[]`, `"errors":[{"message":"load failed"}]`, 1), strings.Replace(selectionList, "tests/page.spec.ts", "../escape.ts", 1)} {
		in, e := PlaywrightInventory(selectionDiscovery(), raw)
		if e == nil || in.Complete || in.Reason == "" {
			t.Fatal(in, e)
		}
	}
}
func TestRSPV0003PlaywrightReconciliation(t *testing.T) {
	d := selectionDiscovery()
	in, e := PlaywrightInventory(d, selectionList)
	if e != nil {
		t.Fatal(e)
	}
	raw := strings.ReplaceAll(selectionList, `"projectName":"chromium"`, `"projectName":"chromium","status":"expected","results":[{"status":"passed"}]`)
	raw = strings.ReplaceAll(raw, `"projectName":"firefox"`, `"projectName":"firefox","status":"skipped"`)
	out, e := PlaywrightResults(d, raw)
	if e != nil {
		t.Fatal(e)
	}
	plan := testrunner.PlanRunSet(in, d.Key, nil, nil)
	if r := testrunner.ReconcileInventory(in, d.Key, plan, out, false); !r.Succeeded {
		t.Fatal(r)
	}
	if _, e = PlaywrightResults(d, selectionList); e == nil {
		t.Fatal("list masqueraded as run")
	}
	if r := testrunner.ReconcileInventory(in, d.Key, plan, out[:1], false); r.Succeeded || !r.NeedsRequalification {
		t.Fatal(r)
	}
}
func serverNative() testevidence.Native {
	b := testevidence.Blob{Path: testevidence.Path{Repository: "repo", Lexical: "server.js", Resolved: "server.js"}, OID: strings.Repeat("b", 40)}
	return testevidence.Native{Executed: testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present, Items: []testevidence.ExecutedFile{{Blob: b, Executed: []testevidence.Range{{Start: 2, StartColumn: 1, End: 3, EndColumn: 1}}}}}, Regions: testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present, Items: []testevidence.RegionsFile{{Blob: b}}}, Loaded: testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{b}}}
}
func serverRecords() []testevidence.Record {
	var out []testevidence.Record
	for _, id := range []string{"one", "two"} {
		r := testevidence.New("repo", id, strings.Repeat("a", 40))
		r.Executed.State, r.Executed.Reason = testevidence.Present, ""
		r.Regions.State, r.Regions.Reason = testevidence.Present, ""
		r.Loaded.State, r.Loaded.Reason = testevidence.Present, ""
		r.CollectionCount = 1
		out = append(out, r)
	}
	return out
}
func TestRSPV0019ServerSharedAndResetDump(t *testing.T) {
	a := ServerAttribution{Tests: []string{"one", "two"}, Test: "one", InsideTree: true, CoverageBuilt: true}
	for _, qualified := range []bool{false, true} {
		a.ResetDumpQualified, a.ResetObserved, a.DumpObserved = qualified, true, true
		out, e := JoinServer(serverRecords(), serverNative(), a)
		if e != nil {
			t.Fatal(e)
		}
		if len(out[0].Executed.Items) != 1 || out[0].CollectionCount != 1 {
			t.Fatal(out)
		}
		want := 1
		if qualified {
			want = 0
		}
		if len(out[1].Executed.Items) != want {
			t.Fatal("attribution", out)
		}
		if qualified {
			if out[1].Executed.State != testevidence.Incomplete || out[1].Executed.Reason != "browser-server-dump-pending" {
				t.Fatal("unjoined sharer certified", out[1])
			}
			other := a
			other.Test = "two"
			for _, dumps := range [][]ServerDump{
				{{serverNative(), a}, {serverNative(), other}},
				{{serverNative(), other}, {serverNative(), a}},
			} {
				complete, err := JoinServerDumps(serverRecords(), dumps)
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range complete {
					if r.Executed.State != testevidence.Present || r.Executed.Reason != "" || len(r.Executed.Items) != 1 || r.CollectionCount != 1 {
						t.Fatal("all dumps did not complete collection", r)
					}
				}
			}
		} else if out[1].Executed.State != testevidence.Present {
			t.Fatal("shared dump incomplete", out[1])
		}
	}
}
func TestRSPV0019SerialFreshAndMissingReset(t *testing.T) {
	a := ServerAttribution{Tests: []string{"one"}, Test: "one", InsideTree: true, CoverageBuilt: true, FreshSerial: true}
	out, e := JoinServer(serverRecords(), serverNative(), a)
	if e != nil || len(out[0].Executed.Items) != 1 || len(out[1].Executed.Items) != 0 {
		t.Fatal(out, e)
	}
	a.Tests = []string{"one", "two"}
	out, e = JoinServer(serverRecords(), serverNative(), a)
	if e != nil || len(out[1].Executed.Items) != 1 {
		t.Fatal("workers alone isolated", out, e)
	}
	a.FreshSerial = false
	a.ResetDumpQualified = true
	a.ResetObserved = true
	out, e = JoinServer(serverRecords(), serverNative(), a)
	if e != nil || len(out[1].Executed.Items) != 1 {
		t.Fatal("missing dump isolated", out, e)
	}
}

func TestRSPV0019NeverJoinedSharerSelectsServerChange(t *testing.T) {
	a := ServerAttribution{Tests: []string{"one", "two"}, Test: "one", InsideTree: true, CoverageBuilt: true, ResetDumpQualified: true, ResetObserved: true, DumpObserved: true}
	rows := serverRecords()
	out, err := JoinServer(rows, serverNative(), a)
	if err != nil {
		t.Fatal(err)
	}
	missing := out[1]
	if missing.Executed.State != testevidence.Incomplete || missing.Executed.Reason != "browser-server-dump-pending" || len(missing.Executed.Items) != 0 {
		t.Fatal("missing dump certified", missing)
	}
	if rows[1].Executed.State != testevidence.Present {
		t.Fatal("input mutated")
	}
	in := browserSelectorInput(testevidence.Native{Executed: missing.Executed, Regions: missing.Regions, Loaded: missing.Loaded}, strings.Repeat("b", 40))
	in.Changes[0].Path = "server.js"
	in.Runners[0].ServerPaths = []testevidence.PathKey{{Repository: "repo", Path: "server.js"}}
	in.Runners[0].Qualification.Classes = []runnerselection.Class{runnerselection.E2EServer}
	in.Snapshot.Blobs = map[testevidence.PathKey]string{{Repository: "repo", Path: "server.js"}: strings.Repeat("b", 40)}
	plan := runnerselection.Select(in).Runners[0]
	if len(plan.Selected) != 1 || len(plan.Excluded) != 0 || !slices.Contains(plan.Selected[0].Rules, "component-missing:executed") {
		t.Fatal("server change excluded unjoined sharer", plan)
	}
	// The same browser-only record would wrongly certify exclusion without
	// the pending state; this control pins the regression to server attribution.
	in.Runners[0].Records[0].Record.Executed = rows[1].Executed
	if p := runnerselection.Select(in).Runners[0]; len(p.Excluded) != 1 {
		t.Fatal("fixture did not exercise exclusion", p)
	}
}

func TestRSPV0019DumpCollectionPreservesUnknowns(t *testing.T) {
	a := ServerAttribution{Tests: []string{"one", "two"}, Test: "one", InsideTree: true, CoverageBuilt: true, ResetDumpQualified: true, ResetObserved: true, DumpObserved: true}
	b := a
	b.Test = "two"
	for _, state := range []testevidence.State{testevidence.Absent, testevidence.Incomplete} {
		rows := serverRecords()
		rows[1].Executed.State, rows[1].Executed.Reason = state, "browser-coverage-unavailable"
		out, err := JoinServerDumps(rows, []ServerDump{{serverNative(), a}, {serverNative(), b}})
		if err != nil || out[1].Executed.State != testevidence.Incomplete || out[1].Executed.Reason == "" {
			t.Fatal("browser uncertainty cleared", out, err)
		}
	}
	incomplete := serverNative()
	incomplete.Executed.State, incomplete.Executed.Reason = testevidence.Incomplete, "server-coverage-unavailable"
	out, err := JoinServerDumps(serverRecords(), []ServerDump{{serverNative(), a}, {incomplete, b}})
	if err != nil || out[1].Executed.State != testevidence.Incomplete {
		t.Fatal("server uncertainty cleared", out, err)
	}
	b.Tests = []string{"two"}
	if _, err := JoinServerDumps(serverRecords(), []ServerDump{{serverNative(), a}, {serverNative(), b}}); err == nil {
		t.Fatal("inconsistent sharer sets joined")
	}
}
func TestRSPV0018ExternalAndUninstrumentedServer(t *testing.T) {
	for _, inside := range []bool{false, true} {
		a := ServerAttribution{Tests: []string{"one", "two"}, InsideTree: inside}
		out, e := JoinServer(serverRecords(), serverNative(), a)
		if e != nil {
			t.Fatal(e)
		}
		for _, r := range out {
			if r.Executed.State != testevidence.Incomplete {
				t.Fatal(r)
			}
			if !inside && len(r.External.Items) == 0 {
				t.Fatal("external lost", r)
			}
		}
	}
}
func TestRSPV0019MissingSharerAndBlobDrift(t *testing.T) {
	a := ServerAttribution{Tests: []string{"missing"}, InsideTree: true, CoverageBuilt: true}
	if _, e := JoinServer(serverRecords(), serverNative(), a); e == nil {
		t.Fatal("missing sharer")
	}
	a.Tests = []string{"one", "two"}
	rows := serverRecords()
	rows[0].Executed = serverNative().Executed
	rows[0].Executed.Items[0].Blob.OID = strings.Repeat("c", 40)
	if _, e := JoinServer(rows, serverNative(), a); e == nil {
		t.Fatal("blob drift")
	}
}
func TestRSPV0005BrowserMissingMap(t *testing.T) {
	c := jslinecov.CoverageCapture{Repository: "repo", Complete: true}
	n := BrowserNative(nil, c, nil)
	if n.Executed.State == testevidence.Present {
		t.Fatal(n)
	}
	repo := NewRepo("/repo", map[string]string{})
	n = BrowserNative(repo, c, []Script{{URL: "http://localhost/app.js", Source: "unknown"}})
	if n.Executed.State == testevidence.Present {
		t.Fatal(n)
	}
}

func TestRSPV0005BrowserNativeBinding(t *testing.T) {
	text := "function f() {\n return true;\n}\n"
	empty := NewRepo("/repo", map[string]string{})
	oid := empty.blobOID([]byte(text))
	repo := NewRepo("/repo", map[string]string{"app.js": oid})
	c := jslinecov.CoverageCapture{Repository: "repo", Sources: map[string]jslinecov.Source{"app.js": {OID: oid, Text: []byte(text)}}, Complete: true, LoadedComplete: true}
	script := Script{URL: "http://localhost/app.js", Source: text, Functions: []Function{{Ranges: []Range{{StartOffset: 0, EndOffset: len(text), Count: 1}}}}}
	n := BrowserNative(repo, c, []Script{script})
	if n.Executed.State != testevidence.Present || len(n.Executed.Items) != 1 || len(n.Loaded.Items) != 1 {
		t.Fatal(n)
	}
	script.Functions[0].Ranges[0].EndOffset = len(text) + 1
	n = BrowserNative(repo, c, []Script{script})
	if n.Executed.State == testevidence.Present {
		t.Fatal("bad ranges certified", n)
	}
	script.Functions[0].Ranges[0].EndOffset = len(text)
	c.Sources["app.js"] = jslinecov.Source{OID: strings.Repeat("c", 40), Text: []byte(text)}
	n = BrowserNative(repo, c, []Script{script})
	if n.Executed.State == testevidence.Present {
		t.Fatal("wrong source certified", n)
	}
}
func TestRSPV0003PlaywrightFailureSurvivesMalformedLaterResult(t *testing.T) {
	d := selectionDiscovery()
	raw := strings.Replace(selectionList, `"projectName":"chromium"`, `"projectName":"chromium","status":"unexpected"`, 1)
	out, err := PlaywrightResults(d, raw)
	if err == nil || len(out) != 1 || out[0].Outcome != "failed" {
		t.Fatal(out, err)
	}
	raw = strings.Replace(selectionList, `"projectName":"chromium"`, `"status":"expected"`, 1)
	in, err := PlaywrightInventory(d, raw)
	if err == nil || in.Complete {
		t.Fatal("invented project", in, err)
	}
}

func TestRSPV0003PlaywrightGlobalErrorRetainsFailureAcrossRetry(t *testing.T) {
	d := selectionDiscovery()
	in, err := PlaywrightInventory(d, selectionList)
	if err != nil {
		t.Fatal(err)
	}
	passing := strings.ReplaceAll(selectionList, `"projectName":"chromium"`, `"projectName":"chromium","status":"expected","results":[{"status":"passed"}]`)
	passing = strings.ReplaceAll(passing, `"projectName":"firefox"`, `"projectName":"firefox","status":"expected","results":[{"status":"passed"}]`)
	for _, diagnostics := range []string{`[{"message":"teardown failed"}]`, `{"futureDiagnostics":true}`} {
		t.Run(diagnostics, func(t *testing.T) {
			// Firefox has no result, so the real reconciliation loop retries FULL.
			first := strings.Replace(selectionList, `,{"projectName":"firefox"}`, "", 1)
			first = strings.Replace(first, `"projectName":"chromium"`, `"projectName":"chromium","status":"unexpected"`, 1)
			first = strings.Replace(first, `"errors":[]`, `"errors":`+diagnostics, 1)
			results, err := PlaywrightResults(d, first)
			if err == nil || len(results) != 1 || results[0].Outcome != "failed" {
				t.Fatalf("failure or diagnostic lost: %+v %v", results, err)
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
				results, err := PlaywrightResults(d, raw)
				return results, false, err
			})
			if out.Succeeded || calls != 2 || !out.NeedsRequalification || len(out.Attempts[0].Results) != 1 || out.Attempts[0].Results[0].Outcome != "failed" || !out.Attempts[1].Reconciliation.Succeeded {
				t.Fatalf("failure lost across passing FULL retry: %+v", out)
			}
			inventory, err := PlaywrightInventory(d, strings.Replace(selectionList, `"errors":[]`, `"errors":`+diagnostics, 1))
			if err == nil || inventory.Complete {
				t.Fatal("global errors certified inventory", inventory, err)
			}
		})
	}
}

func TestRSPV0018CoverageHookPreparation(t *testing.T) {
	work := t.TempDir()
	files, env, err := RunnerCoveragePreparation(work, "--enable-source-maps")
	if err != nil || !reflect.DeepEqual(files["browser-hook.cjs"], hookSource) || !strings.Contains(env["NODE_OPTIONS"], "--enable-source-maps --require ") || env["CORVINT_BROWSERCOV_DIR"] != work {
		t.Fatal(files, env, err)
	}
	files["browser-hook.cjs"][0] = 'x'
	if hookSource[0] == 'x' {
		t.Fatal("aliased hook")
	}
	if _, _, err = RunnerCoveragePreparation("relative", ""); err == nil {
		t.Fatal("relative scratch")
	}
}

// RSP-V0-005: partial coverage must reach T2's uncovered-code fallback.
func TestRSPV0005BrowserPartialCoverageSelector(t *testing.T) {
	text := "function f() {\n touched();\n prefix(); suffix();\n tail();\n}\n"
	oid := NewRepo("/repo", nil).blobOID([]byte(text))
	repo := NewRepo("/repo", map[string]string{"app.js": oid})
	capture := jslinecov.CoverageCapture{Repository: "repo", Sources: map[string]jslinecov.Source{"app.js": {OID: oid, Text: []byte(text)}}, Complete: true, LoadedComplete: true}
	for _, tc := range []struct {
		name              string
		start, end, count int
		selected          bool
	}{
		{"uncovered-prefix", strings.Index(text, "suffix"), len(text), 0, true},
		{"uncovered-suffix", 0, strings.Index(text, "suffix"), 0, true},
		{"full-unexecuted", 0, len(text), 0, false},
		{"full-executed", 0, len(text), 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := Script{URL: "http://localhost/app.js", Source: text, Functions: []Function{{Ranges: []Range{
				{StartOffset: tc.start, EndOffset: tc.end, Count: tc.count},
				{StartOffset: strings.Index(text, " touched"), EndOffset: strings.Index(text, " prefix"), Count: 1},
			}}}}
			n := BrowserNative(repo, capture, []Script{script})
			in := browserSelectorInput(n, oid)
			p := runnerselection.Select(in).Runners[0]
			if (len(p.Selected) == 1) != tc.selected || (len(p.Excluded) == 1) == tc.selected {
				t.Fatalf("unexpected selection: %+v; native: %+v", p, n)
			}
			if tc.selected && tc.count == 0 {
				if p.Mode != "full" || n.Executed.State == testevidence.Present {
					t.Fatalf("partial coverage certified: %+v; native: %+v", p, n)
				}
			} else if p.Mode != "selected" {
				t.Fatalf("complete coverage lost: %+v", p)
			}
		})
	}
}

func browserSelectorInput(n testevidence.Native, oid string) runnerselection.Input {
	r := testevidence.New("repo", "test", strings.Repeat("a", 40))
	r.Runner = testevidence.RunnerIdentity{Plugin: "fixture", PluginVersion: "1", ExecutableDigest: strings.Repeat("b", 64), RunnerVersion: "1", ConfigurationDigest: strings.Repeat("c", 64)}
	r.Execution = testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64", TracingBackend: "trace", TracingVersion: "1", EnvironmentDigest: strings.Repeat("d", 64)}
	r.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("e", 64), VerifierRevision: strings.Repeat("f", 40), ConfigurationDigest: r.Runner.ConfigurationDigest, Ref: "refs/heads/main", Run: "one", ContentDigest: strings.Repeat("1", 64)}}
	r.CollectionCount = 1
	r.Executed, r.Regions, r.Loaded = n.Executed, n.Regions, n.Loaded
	r.Reads = testevidence.Component[testevidence.Blob]{State: testevidence.Present}
	r.Listings = testevidence.Component[testevidence.Path]{State: testevidence.Present}
	r.Lookups = testevidence.Component[testevidence.Path]{State: testevidence.Present}
	r.Writes = testevidence.Component[testevidence.Write]{State: testevidence.Present}
	r.Execs = testevidence.Component[testevidence.Executable]{State: testevidence.Present}
	r.External = testevidence.Component[testevidence.Contact]{State: testevidence.Present}
	head := strings.Repeat("9", 40)
	key := testrunner.InventoryKey{Checkout: head, Runner: "playwright", ConfigDigest: r.Runner.ConfigurationDigest, OS: r.Execution.OS, Arch: r.Execution.Architecture}
	return runnerselection.Input{Base: r.Commit, Head: head, Repository: "repo", Repositories: map[string]bool{"repo": true}, DiffComplete: true,
		Changes:  []runnerselection.Change{{Repository: "repo", Path: "app.js", Kind: "modify", Decoded: true, Hunks: []runnerselection.Hunk{{Start: 3, End: 3}}}},
		Snapshot: testevidence.Snapshot{Blobs: map[testevidence.PathKey]string{{Repository: "repo", Path: "app.js"}: oid}},
		Runners: []runnerselection.Runner{{Name: key.Runner, Identity: r.Runner, Execution: r.Execution,
			Pieces:        testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Present: true}},
			Inventory:     testrunner.RunnerInventory{Key: key, Complete: true, Tests: []testrunner.InventoryTest{{ID: r.Test, Path: "test.js"}}},
			Qualification: runnerselection.Qualification{Admitted: true, Passed: true, SamplingEnabled: true, SingleCollectionQualified: true, Model: "traced", Classes: []runnerselection.Class{runnerselection.BodyEdit}, Runner: r.Runner, Execution: r.Execution},
			Records:       []runnerselection.Evidence{{Record: r, Admitted: true}}, IndexBase: r.Commit, IndexVerified: true}}}
}

func TestRSPV0005WholeLineProofAllowlist(t *testing.T) {
	text := "x\n"
	for _, o := range []Observation{
		{Text: text, Lines: map[int]bool{0: false}},              // mapped summaries have no column proof
		{Text: text, Units: []int8{2, 0}},                        // unknown future state
		{Text: text, Units: []int8{0}},                           // truncated representation
		{Text: text, Units: []int8{0, 0}, Lines: map[int]bool{}}, // mixed representation
		{Text: text, Units: []int8{-1, 0}},
	} {
		// A complete earlier observation must not hide later uncertainty.
		got := certifiedLineStates(text, []Observation{{Text: text, Units: []int8{0, 0}}, o})
		if len(got) != 0 {
			t.Fatalf("unknown observation certified: %+v => %v", o, got)
		}
	}
}
