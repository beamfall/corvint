// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/nativelinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

const jvmDiscovery = `JUnit Jupiter ([engine:junit-jupiter])
Probe ([engine:junit-jupiter]/[class:p.Probe])
alpha() ([engine:junit-jupiter]/[class:p.Probe]/[method:alpha()])
beta() ([engine:junit-jupiter]/[class:p.Probe]/[method:beta()])

[         2 containers found ]
[         2 tests found      ]

`

func jvmInventoryFixture(runner string) string {
	if runner != "gradle-junit" {
		return jvmDiscovery
	}
	return jvmSuite(runner, `<testcase classname="p.Probe" name="alpha"><skipped/></testcase><testcase classname="p.Probe" name="beta"><skipped/></testcase>`, "2", "0", "2")
}

const jvmCoverage = `<report name="fixture"><package name="p"><sourcefile name="Logic.java"><line nr="3" mi="0" ci="2" mb="0" cb="0"/><line nr="4" mi="2" ci="0" mb="0" cb="0"/></sourcefile><sourcefile name="Logic.kt"><line nr="5" mi="0" ci="1" mb="0" cb="0"/></sourcefile></package></report>`

func jvmBlobFixture(name string) testevidence.Blob {
	return testevidence.Blob{Path: testevidence.Path{Repository: "fixture", Lexical: name, Resolved: name}, OID: strings.Repeat("a", 40)}
}

func jvmConfig(runner string) registry.JVMSelectionConfig {
	digest := strings.Repeat("a", 64)
	r := tr.Request{Runner: runner, Root: "/fixture", ReportDir: "/fixture/reports", Project: "app", Executable: "/tools/gradle", ExecutableSha256: digest,
		Config: "/fixture/app/build.gradle.kts", ConfigSha256: digest, Target: ":test", Tools: map[string]tr.Tool{"java": {Executable: "/tools/jdk/bin/java", Sha256: digest}}}
	if runner == "maven-surefire" {
		r.Executable, r.Config, r.Target = "/tools/mvn", "/fixture/app/pom.xml", ""
	}
	console := tr.Tool{Executable: "/tools/junit-console.jar", Sha256: digest}
	classpath := []string{"/fixture/classes", "/tools/jupiter.jar"}
	if runner == "junit-platform" {
		r.Executable, r.Project, r.Target = "/tools/jdk/bin/java", "classes", ""
		r.Reporter, r.ReporterSha256 = console.Executable, console.Sha256
		classpath = []string{"/fixture/classes"}
	}
	key := tr.InventoryKey{Runner: runner, Checkout: strings.Repeat("b", 40), ConfigDigest: digest, OS: "linux", Arch: "amd64"}
	java, kotlin := jvmBlobFixture("app/src/main/java/p/Logic.java"), jvmBlobFixture("app/src/main/kotlin/p/Logic.kt")
	return registry.JVMSelectionConfig{Key: key, Request: r, Console: console, Agent: tr.Tool{Executable: "/tools/jacocoagent.jar", Sha256: digest}, DiscoveryClasspath: classpath,
		SourceSet: registry.JVMSourceSet{Name: "test", Task: r.Target, NativeTest: true, Complete: true,
			Classes: map[string]testevidence.Blob{"p.Probe": jvmBlobFixture("app/src/test/java/p/Probe.java")},
			Sources: map[string]testevidence.Blob{"p/Logic.java": java, "p/Logic.kt": kotlin}, Classpath: []testevidence.Blob{java, kotlin}},
		Captures: map[string]registry.JVMCapture{"p.Probe#alpha": {Key: key, Tests: []string{"p.Probe#alpha"}, TreeComplete: true,
			Processes: []nativelinecov.Process{{ID: 1, Exited: true, Instrumented: true, Reports: []nativelinecov.Report{{Format: "jacoco", Data: []byte(jvmCoverage)}}}},
			Regions: testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present, Items: []testevidence.RegionsFile{
				{Blob: java, Bodies: []testevidence.Body{{ID: "p.Logic.answer", Range: testevidence.Range{Start: 2, StartColumn: 1, End: 6, EndColumn: 1}}}},
				{Blob: kotlin, Bodies: []testevidence.Body{{ID: "p.LogicKt.answer", Range: testevidence.Range{Start: 2, StartColumn: 1, End: 7, EndColumn: 1}}}},
			}},
		}},
	}
}

func jvmPlugin(t *testing.T, c registry.JVMSelectionConfig) registry.RunnerPlugin {
	t.Helper()
	p, _, err := registry.NewJVMSelectionPlugin(c)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// RSP-V0-001: declared JVM runners gain configured pieces without changing
// legacy wires or treating their presence as a qualification record.
func TestRSPV0001JVMPresentUnqualified(t *testing.T) {
	for _, runner := range []string{"gradle-junit", "maven-surefire", "junit-platform"} {
		c := jvmConfig(runner)
		p, inv, err := registry.NewJVMSelectionPlugin(c)
		if err != nil || !p.Pieces().Inventory.Present || !p.Pieces().TestEvidence.Present || p.Pieces().Qualification.Present || p.Pieces().Qualification.Reason != "unqualified" {
			t.Fatalf("%s: %+v %v", runner, p, err)
		}
		if inv.Format != runner {
			t.Fatal(inv.Format)
		}
		in := registry.NewSelectionSession(p, c.Key, true).Inventory(context.Background(), jvmInventoryFixture(c.Key.Runner))
		plan := registry.NewSelectionSession(p, c.Key, true).Plan(in, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || len(plan.Tests) != 2 {
			t.Fatalf("%+v", plan)
		}
	}
}

// RSP-V0-001: unsupported runners and unresolved/non-native tasks remain FULL;
// an aggregate task cannot claim the JVM plugin merely by using Gradle.
func TestRSPV0001JVMAbsentOrInvalid(t *testing.T) {
	for _, change := range []func(*registry.JVMSelectionConfig){
		func(c *registry.JVMSelectionConfig) { c.Key.Runner, c.Request.Runner = "testng", "testng" },
		func(c *registry.JVMSelectionConfig) {
			c.SourceSet.NativeTest = false
			c.SourceSet.Task, c.Request.Target = ":fastGate", ":fastGate"
		},
		func(c *registry.JVMSelectionConfig) { c.SourceSet.Complete = false },
		func(c *registry.JVMSelectionConfig) { c.Console.Sha256 = "" },
		func(c *registry.JVMSelectionConfig) { c.Request.ExecPrefix = []string{"wrapper"} },
		func(c *registry.JVMSelectionConfig) { c.SourceSet.Classpath[0].OID = "" },
	} {
		c := jvmConfig("gradle-junit")
		change(&c)
		if _, _, err := registry.NewJVMSelectionPlugin(c); err == nil {
			t.Fatal("invalid JVM configuration admitted")
		}
	}
}

// RSP-V0-002: console discovery produces stable source-bound inventories per
// runner; build task listing distinguishes each source set and its classpath.
func TestRSPV0002JVMCanonicalInventory(t *testing.T) {
	for _, runner := range []string{"gradle-junit", "maven-surefire", "junit-platform"} {
		c := jvmConfig(runner)
		p := jvmPlugin(t, c)
		a, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: c.Key, Output: jvmInventoryFixture(c.Key.Runner)})
		if err != nil || !a.Complete || len(a.Tests) != 2 || a.Tests[0].Path != "app/src/test/java/p/Probe.java" {
			t.Fatalf("%+v %v", a, err)
		}
		b, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: c.Key, Output: jvmInventoryFixture(c.Key.Runner)})
		if err != nil {
			t.Fatal(err)
		}
		ab, _ := tr.InventoryBytes(a, c.Key)
		bb, _ := tr.InventoryBytes(b, c.Key)
		if string(ab) != string(bb) {
			t.Fatal("noncanonical discovery")
		}
		if runner == "junit-platform" && !strings.HasPrefix(a.Tests[0].ID, "[engine:") || runner != "junit-platform" && a.Tests[0].ID != "p.Probe#alpha" {
			t.Fatal(a.Tests)
		}
	}
	listing := "Gradle log\nCORVINT_JVM_BEGIN\nCORVINT_JVM_TASK\t:app:test\ttest\nCORVINT_JVM_COMPILE\t:app:test\t/fixture/Logic.java\nCORVINT_JVM_TASK\t:app:integrationTest\tintegrationTest\nCORVINT_JVM_END\t2\nBUILD SUCCESSFUL\n"
	tasks, err := registry.JVMGradleTasks(listing)
	if err != nil || len(tasks) != 2 || tasks[":app:integrationTest"].SourceSet != "integrationTest" || len(tasks[":app:test"].CompileClasspath) != 1 {
		t.Fatalf("%+v %v", tasks, err)
	}
}

// RSP-V0-002: incomplete discovery never supplies a narrower inventory.
func TestRSPV0002JVMInventoryFailuresFull(t *testing.T) {
	c := jvmConfig("gradle-junit")
	p := jvmPlugin(t, c)
	for _, output := range []string{"", "garbage\n", strings.Repeat("x", tr.MaxReportBytes+1), strings.TrimSuffix(jvmDiscovery, "\n\n"),
		strings.Replace(jvmDiscovery, "2 tests found", "3 tests found", 1),
		strings.Replace(jvmDiscovery, "[method:alpha()]", "[test-factory:alpha()]", 1),
		strings.ReplaceAll(jvmDiscovery, "junit-jupiter", "unknown-engine"),
		strings.ReplaceAll(jvmDiscovery, "p.Probe", "p.Unmapped"),
		strings.Replace(jvmDiscovery, "[method:beta()]", "[method:alpha()]", 1),
		jvmDiscovery + "trailing\n", "\xff\n"} {
		s := registry.NewSelectionSession(p, c.Key, false)
		in := s.Inventory(context.Background(), output)
		if in.Complete || in.Reason == "" || !s.Plan(in, nil, nil).Full {
			t.Fatalf("admitted %q: %+v", output[:min(len(output), 80)], in)
		}
	}
	for _, output := range []string{"", "CORVINT_JVM_BEGIN\n", "CORVINT_JVM_BEGIN\nCORVINT_JVM_END\t0\n",
		"CORVINT_JVM_BEGIN\nCORVINT_JVM_UNRESOLVED\t:fastGate\nCORVINT_JVM_END\t1\n",
		"CORVINT_JVM_BEGIN\nCORVINT_JVM_TASK\t:test\ttest\nCORVINT_JVM_END\t2\n"} {
		if _, err := registry.JVMGradleTasks(output); err == nil {
			t.Fatal("bad task listing admitted")
		}
	}
}

func jvmSuite(runner, cases string, count, failures, skips string) string {
	version := ""
	if runner == "maven-surefire" {
		version = ` version="3.0.2"`
	}
	return `<testsuite name="p.Probe"` + version + ` tests="` + count + `" failures="` + failures + `" errors="0" skipped="` + skips + `">` + cases + `</testsuite>`
}

// RSP-V0-003: preserve passed, failed and reasoned skipped native results,
// including multiple suites. Existing core reconciliation owns divergence.
func TestRSPV0003JVMTerminalResults(t *testing.T) {
	for _, runner := range []string{"gradle-junit", "maven-surefire"} {
		p := jvmPlugin(t, jvmConfig(runner))
		suite := jvmSuite(runner, `<testcase classname="p.Probe" name="alpha"/><testcase classname="p.Probe" name="beta"><failure type="java.lang.AssertionError" message="expected"/></testcase><testcase classname="p.Probe" name="disabled"><skipped message="disabled fixture"/></testcase>`, "3", "1", "1")
		results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: "<testsuites>" + suite + "</testsuites>"})
		if err != nil || len(results) != 3 || results[0].Outcome != "passed" || results[1].Outcome != "failed" || results[2].Outcome != "skipped" || results[2].Reason != "disabled fixture" {
			t.Fatalf("%+v %v", results, err)
		}
		other := jvmSuite(runner, `<testcase classname="p.Other" name="gamma"/>`, "1", "0", "0")
		results, err = p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: "<testsuites>" + suite + other + "</testsuites>"})
		if err != nil || len(results) != 4 {
			t.Fatalf("multiple suites: %+v %v", results, err)
		}
	}
	c := jvmConfig("junit-platform")
	report := `<testsuite name="JUnit Jupiter" tests="1" failures="0" errors="0" skipped="0"><testcase classname="p.Probe" name="alpha()"><system-out>unique-id: [engine:junit-jupiter]/[class:p.Probe]/[method:alpha()]</system-out></testcase></testsuite>`
	results, err := jvmPlugin(t, c).Parse(context.Background(), registry.ParseRequest{Runner: c.Key.Runner, Report: report})
	if err != nil || len(results) != 1 || results[0].ID != "[engine:junit-jupiter]/[class:p.Probe]/[method:alpha()]" {
		t.Fatalf("console: %+v %v", results, err)
	}
}

// RSP-V0-003: a missing selected result forces a FULL retry and retained
// divergence. A later passing retry cannot erase an earlier native failure.
func TestRSPV0003JVMDivergenceRetry(t *testing.T) {
	c := jvmConfig("gradle-junit")
	p := jvmPlugin(t, c)
	s := registry.NewSelectionSession(p, c.Key, false)
	in := s.Inventory(context.Background(), jvmInventoryFixture(c.Key.Runner))
	first := jvmSuite(c.Key.Runner, `<testcase classname="p.Probe" name="alpha"><failure type="java.lang.AssertionError" message="expected"/></testcase>`, "1", "1", "0")
	second := jvmSuite(c.Key.Runner, `<testcase classname="p.Probe" name="alpha"/><testcase classname="p.Probe" name="beta"/>`, "2", "0", "0")
	calls := 0
	out := s.Run(context.Background(), in, nil, nil, c.Request.ReportDir, true, func(context.Context, registry.CommandResponse) (string, error) {
		calls++
		if calls == 1 {
			return first, nil
		}
		return second, nil
	})
	if calls != 2 || out.Succeeded || !out.NeedsRequalification || !reflect.DeepEqual(out.Attempts[0].Reconciliation.Missing, []string{"p.Probe#beta"}) || !out.Attempts[1].Plan.Full {
		t.Fatalf("%+v calls=%d", out, calls)
	}
	extra, err := p.Parse(context.Background(), registry.ParseRequest{Runner: c.Key.Runner, Report: jvmSuite(c.Key.Runner, `<testcase classname="p.Probe" name="gamma"/>`, "1", "0", "0")})
	if err != nil {
		t.Fatal(err)
	}
	reconciled := tr.ReconcileInventory(in, c.Key, tr.PlanRunSet(in, c.Key, nil, nil), extra, true)
	if reconciled.Succeeded || !reconciled.Full || !reflect.DeepEqual(reconciled.Unexpected, []string{"p.Probe#gamma"}) {
		t.Fatal(reconciled)
	}
	for _, report := range []string{first[:len(first)-1], "<testsuites>" + first, strings.Replace(first, `tests="1"`, `tests="2"`, 1),
		jvmSuite(c.Key.Runner, `<testcase classname="p.Probe" name="alpha"><skipped/></testcase>`, "1", "0", "1"),
		"<testsuites>" + first + first + "</testsuites>"} {
		if _, err := p.Parse(context.Background(), registry.ParseRequest{Runner: c.Key.Runner, Report: report}); err == nil {
			t.Fatal("invalid result admitted")
		}
	}
}

// RSP-V0-004: native filtering always widens to the reported FULL superset;
// the shared selector still keeps new/stale tests and mandates in the run set.
func TestRSPV0004JVMRunSetSuperset(t *testing.T) {
	c := jvmConfig("gradle-junit")
	p := jvmPlugin(t, c)
	in, _ := p.Inventory(context.Background(), registry.InventoryRequest{Key: c.Key, Output: jvmInventoryFixture(c.Key.Runner)})
	fresh := tr.PlanRunSet(in, c.Key, []tr.Exclusion{{ID: "p.Probe#alpha", Fresh: true}}, nil)
	if !reflect.DeepEqual(fresh.Tests, []string{"p.Probe#beta"}) {
		t.Fatal(fresh)
	}
	plan := tr.PlanRunSet(in, c.Key, []tr.Exclusion{{ID: "p.Probe#alpha", Fresh: false}}, nil)
	if len(plan.Tests) != 2 {
		t.Fatal(plan)
	}
	plan = tr.PlanRunSet(in, c.Key, []tr.Exclusion{{ID: "p.Probe#alpha", Fresh: true}}, []string{"p.Probe#alpha"})
	if len(plan.Tests) != 2 {
		t.Fatal(plan)
	}
	for _, runner := range []string{"gradle-junit", "maven-surefire", "junit-platform"} {
		c := jvmConfig(runner)
		p := jvmPlugin(t, c)
		for _, purpose := range []string{"run", "collect", "inventory"} {
			cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: c.Key, Purpose: purpose, RunDir: c.Request.ReportDir, Tests: []string{"cannot-express-this-dynamic-id"}})
			if err != nil || len(cmd.Argv) == 0 || purpose != "inventory" && !cmd.Superset || strings.Contains(strings.Join(cmd.Argv, " "), "cannot-express") {
				t.Fatalf("%+v %v", cmd, err)
			}
			if purpose == "collect" && !strings.Contains(cmd.Env["JAVA_TOOL_OPTIONS"], "-javaagent:") {
				t.Fatal(cmd.Env)
			}
		}
	}
}

// RSP-V0-004: invalid/empty/stale requests never become a native subset.
func TestRSPV0004JVMCommandFailures(t *testing.T) {
	c := jvmConfig("gradle-junit")
	p := jvmPlugin(t, c)
	for _, r := range []registry.CommandRequest{
		{Key: c.Key, Purpose: "run", RunDir: c.Request.ReportDir, Tests: []string{}},
		{Key: c.Key, Purpose: "unknown", RunDir: c.Request.ReportDir},
		{Key: c.Key, Purpose: "run", RunDir: "/other"},
		{Purpose: "run", RunDir: c.Request.ReportDir},
	} {
		if _, err := p.Command(context.Background(), r); err == nil {
			t.Fatal("invalid command admitted")
		}
	}
	c.Agent = tr.Tool{}
	p = jvmPlugin(t, c)
	if _, err := p.Command(context.Background(), registry.CommandRequest{Key: c.Key, Purpose: "collect", RunDir: c.Request.ReportDir}); err == nil {
		t.Fatal("collection without pinned agent")
	}
}

func jvmNative(t *testing.T, c registry.JVMSelectionConfig) testevidence.Native {
	t.Helper()
	values, err := jvmPlugin(t, c).Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner})
	if err != nil || len(values) != 1 {
		t.Fatalf("%+v %v", values, err)
	}
	var n testevidence.Native
	for _, pair := range []struct {
		raw json.RawMessage
		dst any
	}{{values[0].Executed, &n.Executed}, {values[0].Regions, &n.Regions}, {values[0].Loaded, &n.Loaded}} {
		if err := json.Unmarshal(pair.raw, pair.dst); err != nil {
			t.Fatal(err)
		}
	}
	return n
}

// RSP-V0-005: JaCoCo XML fills both hit and zero-hit line records, Kotlin and
// Java regions stay blob-bound, loaded comes from the compile classpath, and
// only the core can add trace components or write the store.
func TestRSPV0005JVMNativeEvidence(t *testing.T) {
	c := jvmConfig("gradle-junit")
	n := jvmNative(t, c)
	if n.Executed.State != testevidence.Present || len(n.Executed.Items) != 2 || len(n.Executed.Items[0].Executed) != 1 || len(n.Executed.Items[0].Unexecuted) != 1 || n.Executed.Items[0].Executed[0].Start != 3 || n.Executed.Items[1].Executed[0].Start != 5 || n.Regions.State != testevidence.Present || len(n.Loaded.Items) != 2 {
		t.Fatalf("%+v", n)
	}
	r := testevidence.New("fixture", "p.Probe#alpha", c.Key.Checkout)
	r.Executed, r.Regions, r.Loaded = n.Executed, n.Regions, n.Loaded
	if err := r.ValidateComponents(); err != nil {
		t.Fatal(err)
	}
	if r.Reads.State != testevidence.Absent || r.Execs.State != testevidence.Absent {
		t.Fatal("plugin invented trace")
	}
	// Constructor freezes captured artifacts; caller mutations cannot rewrite
	// the admitted graph, coverage or staged command after the fact.
	p, inv, err := registry.NewJVMSelectionPlugin(c)
	if err != nil {
		t.Fatal(err)
	}
	inv.Argv[0] = "mutated"
	c.SourceSet.Classpath[0].OID = ""
	c.Captures["p.Probe#alpha"].Processes[0].Reports[0].Data[0] = '!'
	values, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner})
	if err != nil || !strings.Contains(string(values[0].Executed), `"state":"present"`) {
		t.Fatalf("%+v %v", values, err)
	}
}

// RSP-V0-005: stale, malformed, capped and uninstrumented observations widen;
// unknown native coverage and a missing dialect never become present-empty.
func TestRSPV0005JVMFailClosedEvidence(t *testing.T) {
	baseline := jvmNative(t, jvmConfig("gradle-junit"))
	if baseline.Executed.State != testevidence.Present {
		t.Fatal("fault fixture did not initialize", baseline)
	}
	for _, tc := range []struct {
		reason string
		change func(*registry.JVMCapture)
	}{
		{"jvm-capture-key-or-attribution-mismatch", func(c *registry.JVMCapture) { c.Key.Checkout = "stale" }},
		{"jvm-capture-key-or-attribution-mismatch", func(c *registry.JVMCapture) { c.Tests = nil }},
		{"jvm-process-tree-incomplete-or-capped", func(c *registry.JVMCapture) { c.TreeComplete = false }},
		{"jvm-process-coverage-incomplete", func(c *registry.JVMCapture) { c.Processes[0].Instrumented = false }},
		{"jvm-process-coverage-incomplete", func(c *registry.JVMCapture) { c.Processes[0].Exited = false }},
		{"jvm-process-root-divergent", func(c *registry.JVMCapture) { c.Processes[0].Parent = 99 }},
		{"jvm-jacoco-unknown-or-malformed-xml", func(c *registry.JVMCapture) { c.Processes[0].Reports[0].Data = []byte("<report>") }},
		{"jvm-jacoco-xml-export-required", func(c *registry.JVMCapture) { c.Processes[0].Reports[0].Format = "jacoco-exec" }},
		{"jvm-jacoco-malformed-or-unmapped", func(c *registry.JVMCapture) {
			c.Processes[0].Reports[0].Data = []byte(strings.ReplaceAll(jvmCoverage, "Logic.java", "Unknown.java"))
		}},
		{"jvm-jacoco-unknown-or-malformed-xml", func(c *registry.JVMCapture) {
			c.Processes[0].Reports[0].Data = []byte(strings.Replace(jvmCoverage, "</report>", "<unknown/></report>", 1))
		}},
		{"jvm-coverage-report-cap", func(c *registry.JVMCapture) {
			c.Processes[0].Reports[0].Data = []byte(strings.Repeat("x", nativelinecov.MaxReportBytes+1))
		}},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			c := jvmConfig("gradle-junit")
			capture := c.Captures["p.Probe#alpha"]
			tc.change(&capture)
			c.Captures["p.Probe#alpha"] = capture
			n := jvmNative(t, c)
			if n.Executed.State == testevidence.Present || n.Executed.Reason != tc.reason {
				t.Fatalf("injection did not reach expected boundary: %+v", n)
			}
		})
	}
	c := jvmConfig("gradle-junit")
	capture := c.Captures["p.Probe#alpha"]
	capture.Regions = testevidence.Component[testevidence.RegionsFile]{}
	c.Captures["p.Probe#alpha"] = capture
	n := jvmNative(t, c)
	if n.Regions.State != testevidence.Absent || n.Regions.Reason != "jvm-region-dialect-unavailable" {
		t.Fatal(n.Regions)
	}
	p := jvmPlugin(t, c)
	if _, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner, Artifacts: []string{"../store"}}); err == nil {
		t.Fatal("plugin opened artifact/store path")
	}
}

// RSP-V0-005: a shared process tree's conservative union is identical on each
// attributed test. Inconsistent purported per-test captures remain incomplete.
func TestRSPV0005JVMSharedTreeUnion(t *testing.T) {
	c := jvmConfig("gradle-junit")
	capture := c.Captures["p.Probe#alpha"]
	capture.Tests = []string{"p.Probe#alpha", "p.Probe#beta"}
	child := capture.Processes[0]
	child.ID, child.Parent = 2, 1
	child.Reports = []nativelinecov.Report{{Format: "jacoco", Data: []byte(strings.ReplaceAll(jvmCoverage, `nr="3"`, `nr="8"`))}}
	capture.Processes = append(capture.Processes, child)
	c.Captures["p.Probe#alpha"], c.Captures["p.Probe#beta"] = capture, capture
	values, err := jvmPlugin(t, c).Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner})
	if err != nil || len(values) != 2 || string(values[0].Executed) != string(values[1].Executed) || !strings.Contains(string(values[0].Executed), `"start":8`) {
		t.Fatalf("%+v %v", values, err)
	}
	other := capture
	other.TreeComplete = false
	c.Captures["p.Probe#beta"] = other
	values, err = jvmPlugin(t, c).Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner})
	if err != nil || len(values) != 2 {
		t.Fatalf("%+v %v", values, err)
	}
	for _, value := range values {
		if !strings.Contains(string(value.Executed), "jvm-capture-key-or-attribution-mismatch") {
			t.Fatal(string(value.Executed))
		}
	}
}

// RSP-V0-002: Gradle's candidates and filters belong to the native Test task.
// Probe is outside ConsoleLauncher's default class-name filter; native reports
// must retain it alongside SmokeTest, or retain only the task-filtered subset.
func TestRSPV0002JVMGradleTaskDiscovery(t *testing.T) {
	c := jvmConfig("gradle-junit")
	c.SourceSet.Classes["p.SmokeTest"] = jvmBlobFixture("app/src/test/java/p/SmokeTest.java")
	p := jvmPlugin(t, c)
	inventoryCommand, err := p.Command(context.Background(), registry.CommandRequest{Key: c.Key, Purpose: "inventory", RunDir: c.Request.ReportDir})
	if err != nil {
		t.Fatal(err)
	}
	runCommand, err := p.Command(context.Background(), registry.CommandRequest{Key: c.Key, Purpose: "run", RunDir: c.Request.ReportDir})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inventoryCommand.Argv, append(append([]string{}, runCommand.Argv...), "--test-dry-run")) || !reflect.DeepEqual(inventoryCommand.Env, runCommand.Env) || !reflect.DeepEqual(inventoryCommand.Artifacts, runCommand.Artifacts) {
		t.Fatalf("discovery did not preserve native task configuration: %+v %+v", inventoryCommand, runCommand)
	}
	probe := `<testcase classname="p.Probe" name="alpha()"><skipped/></testcase>`
	smoke := `<testcase classname="p.SmokeTest" name="smoke()"><skipped/></testcase>`
	for _, tc := range []struct {
		name, cases, count string
		want               []string
	}{
		{"unfiltered", probe + smoke, "2", []string{"p.Probe#alpha()", "p.SmokeTest#smoke()"}},
		{"includeTags-fast", probe, "1", []string{"p.Probe#alpha()"}},
		{"exclude-class-Probe", smoke, "1", []string{"p.SmokeTest#smoke()"}},
		{"method-filter-Probe-alpha", probe, "1", []string{"p.Probe#alpha()"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := jvmSuite(c.Key.Runner, tc.cases, tc.count, "0", tc.count)
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: c.Key, Output: report})
			if err != nil || !in.Complete {
				t.Fatalf("%+v %v", in, err)
			}
			ids := []string{}
			for _, test := range in.Tests {
				ids = append(ids, test.ID)
			}
			if !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("got %v want %v", ids, tc.want)
			}
		})
	}
	// A complete-looking console scan that omitted Probe is never Gradle evidence.
	console := strings.ReplaceAll(jvmDiscovery, "p.Probe", "p.SmokeTest")
	for _, bad := range []string{console, "", `<testsuites/>`, jvmInventoryFixture(c.Key.Runner)[:40],
		jvmSuite(c.Key.Runner, probe+smoke, "3", "0", "2"),
		jvmSuite(c.Key.Runner, probe+probe, "2", "0", "2"),
		jvmSuite(c.Key.Runner, strings.ReplaceAll(probe, "p.Probe", "p.Missing"), "1", "0", "1"),
		jvmSuite(c.Key.Runner, `<testcase classname="p.Probe" name="alpha()"/>`, "1", "0", "0"),
	} {
		s := registry.NewSelectionSession(p, c.Key, false)
		in := s.Inventory(context.Background(), bad)
		if in.Complete || in.Reason == "" || !s.Plan(in, nil, nil).Full {
			t.Fatalf("unsafe native inventory: %+v", in)
		}
	}
}

func TestRSPV0005JVMInheritedJavaToolOptions(t *testing.T) {
	for _, value := range []string{"", `-Dfixture="two words" -Xmx256m`} {
		t.Setenv("JAVA_TOOL_OPTIONS", value)
		for _, runner := range []string{"gradle-junit", "maven-surefire", "junit-platform"} {
			c := jvmConfig(runner)
			p := jvmPlugin(t, c)
			cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: c.Key, Purpose: "collect", RunDir: c.Request.ReportDir})
			if err != nil {
				t.Fatal(err)
			}
			want := `"-javaagent:` + c.Agent.Executable + `=destfile=` + filepath.Join(c.Request.ReportDir, "jacoco.exec") + `,append=true,dumponexit=true,output=file"`
			if value != "" {
				want = value + " " + want
			}
			if cmd.Env["JAVA_TOOL_OPTIONS"] != want {
				t.Fatalf("got %q want %q", cmd.Env["JAVA_TOOL_OPTIONS"], want)
			}
		}
	}
}

// RSP-V0-003: only complete, validated suites contribute results, but an
// unsupported or damaged envelope suffix cannot erase their observed failures.
func TestRSPV0003JVMDamagedSuffixRetainsFailureAcrossRetry(t *testing.T) {
	for _, runner := range []string{"gradle-junit", "maven-surefire"} {
		for _, suffix := range []string{"", "<testsuite", `<testsuite name="unfinished">`, "<future-suite/></testsuites>"} {
			t.Run(runner+"/"+suffix, func(t *testing.T) {
				c := jvmConfig(runner)
				p := jvmPlugin(t, c)
				first := "<testsuites>" + jvmSuite(runner, `<testcase classname="p.Probe" name="alpha"><failure type="java.lang.AssertionError" message="expected"/></testcase>`, "1", "1", "0") + suffix
				results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: first})
				if err == nil || err.Error() != "jvm-native-suite-envelope-invalid" || len(results) != 1 || results[0].ID != "p.Probe#alpha" || results[0].Outcome != "failed" {
					t.Fatalf("lost failure or admitted suffix: %+v %v", results, err)
				}
				s := registry.NewSelectionSession(p, c.Key, false)
				in := s.Inventory(context.Background(), jvmInventoryFixture(runner))
				passing := jvmSuite(runner, `<testcase classname="p.Probe" name="alpha"/><testcase classname="p.Probe" name="beta"/>`, "2", "0", "0")
				calls := 0
				out := s.Run(context.Background(), in, nil, nil, c.Request.ReportDir, true, func(context.Context, registry.CommandResponse) (string, error) {
					calls++
					if calls == 1 {
						return first, nil
					}
					return passing, nil
				})
				if calls != 2 || out.Succeeded || !out.NeedsRequalification || len(out.Attempts) != 2 || !out.Attempts[1].Plan.Full || !out.Attempts[1].Reconciliation.Succeeded {
					t.Fatalf("passing FULL retry erased failure: %+v calls=%d", out, calls)
				}
			})
		}
	}
}
