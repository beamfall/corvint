// SPDX-License-Identifier: Apache-2.0
package nativelinecov

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func nativeInput(runner string, names []string) tr.Input {
	in := tr.Input{Runner: runner, Reports: map[string][]byte{}, ExitCode: 0}
	var b strings.Builder
	switch Family(runner) {
	case "jvm":
		if runner == "testng" {
			fmt.Fprintf(&b, `<testng-results total="%d" passed="%d" failed="0" skipped="0" ignored="0"><suite name="suite" finished-at="now"><test name="group" finished-at="now"><class name="p.Probe">`, len(names), len(names))
			for _, n := range names {
				fmt.Fprintf(&b, `<test-method name="%s" finished-at="now" status="PASS"/>`, n)
			}
			b.WriteString(`</class></test></suite></testng-results>`)
			in.Reports["testng-results.xml"] = []byte(b.String())
			return in
		}
		engine, title := "junit-jupiter", "JUnit Jupiter"
		if runner == "junit4" || runner == "robolectric" {
			engine, title = "junit-vintage", "JUnit Vintage"
		}
		if runner == "kotest" {
			engine, title = "kotest", "Kotest"
		}
		fmt.Fprintf(&b, `<testsuite name="%s" tests="%d" failures="0" errors="0" skipped="0">`, title, len(names))
		for _, n := range names {
			fmt.Fprintf(&b, "<testcase name=\"%s\" classname=\"p.Probe\"><system-out>unique-id: [engine:%s]/[class:p.Probe]/[method:%s()]\n</system-out></testcase>", n, engine, n)
		}
		b.WriteString("</testsuite>")
		in.Reports["TEST-"+engine+".xml"] = []byte(b.String())
	case "dotnet":
		adapter := map[string]string{"dotnet-vstest-nunit": "executor://nunit3testexecutor/", "dotnet-vstest-mstest": "executor://mstestadapter/v2", "dotnet-vstest-xunit": "executor://xunit/VsTestRunner2/netcoreapp"}[runner]
		b.WriteString(`<TestRun xmlns="http://microsoft.com/schemas/VisualStudio/TeamTest/2010"><Results>`)
		for i, n := range names {
			fmt.Fprintf(&b, `<UnitTestResult testId="id%d" testName="%s" outcome="Passed"/>`, i, n)
		}
		b.WriteString(`</Results><TestDefinitions>`)
		for i, n := range names {
			fmt.Fprintf(&b, `<UnitTest id="id%d"><TestMethod className="p.Probe" name="%s" adapterTypeName="%s"/></UnitTest>`, i, n, adapter)
		}
		b.WriteString(`</TestDefinitions><ResultSummary outcome="Completed"><Counters`)
		for _, k := range strings.Fields("total executed passed failed error timeout aborted inconclusive passedButRunAborted notRunnable notExecuted disconnected warning completed inProgress pending") {
			n := 0
			if k == "total" || k == "executed" || k == "passed" {
				n = len(names)
			}
			fmt.Fprintf(&b, ` %s="%d"`, k, n)
		}
		b.WriteString(`/></ResultSummary></TestRun>`)
		in.Reports["results.trx"] = []byte(b.String())
	case "cfamily":
		fmt.Fprintf(&b, `<testsuites tests="%d" failures="0" errors="0" disabled="0"><testsuite name="Probe" tests="%d" failures="0" errors="0" disabled="0" skipped="0">`, len(names), len(names))
		for _, n := range names {
			fmt.Fprintf(&b, `<testcase classname="Probe" name="%s" status="run" result="completed"/>`, n)
		}
		b.WriteString(`</testsuite></testsuites>`)
		report := runner + ".xml"
		in.Reports[report] = []byte(b.String())
	case "rust":
		if runner == "nextest" {
			fmt.Fprintf(&b, `<testsuites tests="%d"><testsuite name="probe" tests="%d">`, len(names), len(names))
			for _, n := range names {
				fmt.Fprintf(&b, `<testcase classname="probe" name="%s"/>`, n)
			}
			b.WriteString(`</testsuite></testsuites>`)
			in.Reports["nextest.xml"] = []byte(b.String())
			break
		}
		fmt.Fprintf(&b, "running %d tests\n", len(names))
		for _, n := range names {
			fmt.Fprintf(&b, "test %s ... ok\n", n)
		}
		fmt.Fprintf(&b, "test result: ok. %d passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s\n", len(names))
		in.Stdout = []byte(b.String())
	case "swift":
		if runner == "swift-testing" {
			for _, n := range names {
				fmt.Fprintf(&b, "{\"version\":0,\"kind\":\"test\",\"payload\":{\"id\":\"Probe.%s()/Probe.swift:1:1\",\"kind\":\"function\",\"name\":\"%s\"}}\n", n, n)
			}
			b.WriteString("{\"version\":0,\"kind\":\"event\",\"payload\":{\"kind\":\"runStarted\"}}\n")
			for _, n := range names {
				for _, kind := range []string{"testStarted", "testEnded"} {
					fmt.Fprintf(&b, "{\"version\":0,\"kind\":\"event\",\"payload\":{\"kind\":\"%s\",\"testID\":\"Probe.%s()/Probe.swift:1:1\"}}\n", kind, n)
				}
			}
			b.WriteString("{\"version\":0,\"kind\":\"event\",\"payload\":{\"kind\":\"runEnded\"}}\n")
			in.Reports["swift-events.jsonl"] = []byte(b.String())
			break
		}
		b.WriteString("Test Suite 'All tests' started at 2026-11-04 00:00:00.000.\nTest Suite 'Probe' started at 2026-11-04 00:00:00.000.\n")
		for _, n := range names {
			fmt.Fprintf(&b, "Test Case '-[Probe.Tests %s]' started.\nTest Case '-[Probe.Tests %s]' passed (0.001 seconds).\n", n, n)
		}
		for _, suite := range []string{"Probe", "All tests"} {
			fmt.Fprintf(&b, "Test Suite '%s' passed at 2026-11-04 00:00:00.010.\nExecuted %d tests, with 0 failures (0 unexpected) in 0.01 (0.01) seconds\n", suite, len(names))
		}
		in.Stdout = []byte(b.String())
	}
	return in
}

func reports(format, file string, hit int) Report {
	var data string
	switch format {
	case "jacoco":
		data = fmt.Sprintf(`<report name="probe"><package name=""><sourcefile name="%s"><line nr="3" mi="0" ci="%d" mb="0" cb="0"/><line nr="4" mi="1" ci="0" mb="0" cb="0"/></sourcefile></package></report>`, file, hit)
	case "cobertura":
		data = fmt.Sprintf(`<coverage><packages><package><classes><class filename="%s"><lines><line number="3" hits="%d"/><line number="4" hits="0"/></lines></class></classes></package></packages></coverage>`, file, hit)
	default:
		data = fmt.Sprintf("SF:%s\nDA:3,%d\nDA:4,0\nend_of_record\n", file, hit)
	}
	return Report{Format: format, Data: []byte(data)}
}

func options(t *testing.T, runner string) Options {
	t.Helper()
	root := t.TempDir()
	sum := strings.Repeat("a", 64)
	r := tr.Request{Runner: runner, Root: root, Executable: "/tools/runner", ExecutableSha256: sum, Project: "classes", Reporter: "/tools/console.jar", ReporterSha256: sum, Config: "/tools/suite.xml", ConfigSha256: sum, TimeoutSeconds: 30}
	if Family(runner) == "rust" {
		r.Project = "Cargo.toml"
		r.Target = "probe"
	}
	if Family(runner) == "dotnet" {
		r.Project = "Probe.csproj"
	}
	if runner == "swift-xctest" {
		r.Project = ""
		r.Config = filepath.Join(root, "Package.swift")
	}
	if runner == "swift-testing" {
		r.Project = ""
	}
	return Options{Module: "example", Package: "tests", Work: t.TempDir(), Version: "fixture-1", Agent: "/tools/jacocoagent.jar", Request: r, Sources: map[string]string{"src": "src"}}
}

func fixtureRun(r tr.Request, inv tr.Invocation) Capture {
	names := []string{"alpha", "beta"}
	if r.Runner == "testng" {
		if data, err := os.ReadFile(r.Config); err == nil && strings.Contains(string(data), "<include") {
			if strings.Contains(string(data), `name="alpha"`) {
				names = []string{"alpha"}
			} else {
				names = []string{"beta"}
			}
		}
	}
	if len(r.Selectors) > 0 {
		if strings.Contains(r.Selectors[0], "alpha") {
			names = []string{"alpha"}
		} else {
			names = []string{"beta"}
		}
	}
	format := "lcov"
	if Family(r.Runner) == "jvm" {
		format = "jacoco"
	}
	if Family(r.Runner) == "dotnet" {
		format = "cobertura"
	}
	hit := 1
	if len(names) == 1 && names[0] == "beta" {
		hit = 0
	}
	return Capture{Input: nativeInput(r.Runner, names), TreeComplete: true, Processes: []Process{{ID: 1, Exited: true, Instrumented: true, Reports: []Report{reports(format, "src", hit)}}}}
}

// LPCV-V0-8200, LPCV-V0-8201, LPCV-V0-8202, LPCV-V0-8203, LPCV-V0-8204:
// real native report decoders and invocation builders, with no native toolchain.
func TestCollectorFamilies(t *testing.T) {
	for _, runner := range []string{"junit-platform", "junit4", "testng", "kotest", "kotlin-test", "dotnet-vstest-nunit", "dotnet-vstest-xunit", "dotnet-vstest-mstest", "googletest", "ctest", "catch2", "cargo-test", "cargo-integration", "cargo-bin", "nextest", "swift-xctest", "swift-testing"} {
		verify := func(t *testing.T) {
			o := options(t, runner)
			calls := 0
			o.Run = func(_ context.Context, r tr.Request, i tr.Invocation) (Capture, error) {
				calls++
				return fixtureRun(r, i), nil
			}
			out, err := Collect(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			if len(out) != 3 || calls != 3 {
				t.Fatalf("outcomes=%+v calls=%d", out, calls)
			}
			for _, one := range out {
				if one.Record == nil {
					t.Fatalf("%s: %s", one.Name, one.Divergent)
				}
			}
			ix, err := Build(o, "commit", map[string]string{"src": "blob"}, out)
			if err != nil {
				t.Fatal(err)
			}
			if len(ix.Tests) != 2 || !reflect.DeepEqual(ix.Unordered, []string{"tests"}) {
				t.Fatalf("index %+v", ix)
			}
			p, _ := ix.Narrow([]string{"tests"}, map[string][]string{"tests": {"alpha", "beta"}}, linecov.Effect{}, linecov.Effect{})
			if p["tests"].Mode != linecov.Whole {
				t.Fatal("unqualified family excluded tests")
			}
		}
		t.Run(runner, func(t *testing.T) {
			switch Family(runner) {
			case "jvm":
				t.Run("LPCV-V0-8200 JVM collector", verify)
			case "dotnet":
				t.Run("LPCV-V0-8201 dotnet collector", verify)
			case "cfamily":
				t.Run("LPCV-V0-8202 native collector", verify)
			case "rust":
				t.Run("LPCV-V0-8203 Rust collector", verify)
			case "swift":
				t.Run("LPCV-V0-8204 Swift collector", verify)
			}
		})
	}
}

// LPCV-V0-8205: no false records from incomplete runner or process evidence.
func TestCollectorRefusals(t *testing.T) {
	t.Run("LPCV-V0-8205 incomplete capture", func(t *testing.T) {
		cases := map[string]func(*Capture){
			"no tree":         func(c *Capture) { c.TreeComplete = false },
			"unknown child":   func(c *Capture) { c.Processes = append(c.Processes, Process{ID: 2, Parent: 1, Exited: true}) },
			"no dump":         func(c *Capture) { c.Processes[0].Reports = nil },
			"timeout":         func(c *Capture) { c.Input.TimedOut = true },
			"interrupted":     func(c *Capture) { c.Input.Interrupted = true },
			"overflow":        func(c *Capture) { c.Input.Overflow = true },
			"report":          func(c *Capture) { c.Input.Stdout = []byte("running 2 tests\n") },
			"execution issue": func(c *Capture) { c.Input.ExecutionProblems = []tr.Problem{{Code: "missing"}} },
			"runner mismatch": func(c *Capture) { c.Input.Runner = "cargo-bin" },
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				o := options(t, "cargo-test")
				o.Run = func(_ context.Context, r tr.Request, i tr.Invocation) (Capture, error) {
					c := fixtureRun(r, i)
					mutate(&c)
					return c, nil
				}
				out, err := Collect(context.Background(), o)
				if err != nil {
					t.Fatal(err)
				}
				for _, one := range out {
					if one.Record != nil || one.Divergent == "" {
						t.Fatalf("false record: %+v", one)
					}
				}
			})
		}
		t.Run("inexact filter", func(t *testing.T) {
			o := options(t, "cargo-test")
			o.Run = func(_ context.Context, r tr.Request, i tr.Invocation) (Capture, error) {
				r.Selectors = nil
				return fixtureRun(r, i), nil
			}
			out, err := Collect(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			for _, one := range out {
				if one.Name != linecov.InOrder && one.Record != nil {
					t.Fatal("accepted inexact filter")
				}
			}
		})
		t.Run("cancellation", func(t *testing.T) {
			o := options(t, "cargo-test")
			ctx, cancel := context.WithCancel(context.Background())
			o.Run = func(_ context.Context, r tr.Request, i tr.Invocation) (Capture, error) {
				cancel()
				return fixtureRun(r, i), nil
			}
			if out, err := Collect(ctx, o); err == nil || out != nil {
				t.Fatal("cancelled collection published records")
			}
		})
		t.Run("scratch ownership", func(t *testing.T) {
			o := options(t, "cargo-test")
			p := filepath.Join(o.Work, "keep")
			if err := os.WriteFile(p, []byte("private"), 0600); err != nil {
				t.Fatal(err)
			}
			o.Run = func(_ context.Context, r tr.Request, i tr.Invocation) (Capture, error) { return fixtureRun(r, i), nil }
			if _, err := Collect(context.Background(), o); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(p); err != nil {
				t.Fatal("removed caller scratch")
			}
		})
	})
}

// LPCV-V0-8205, LPCV-V0-8206: child-only reach and malformed process graphs.
func TestProcessCoverage(t *testing.T) {
	base := func() Capture {
		return Capture{TreeComplete: true, Processes: []Process{{ID: 1, Exited: true, Instrumented: true, Reports: []Report{reports("lcov", "src", 0)}}, {ID: 2, Parent: 1, Exited: true, Instrumented: true, Reports: []Report{reports("lcov", "child", 1)}}}}
	}
	sources := map[string]string{"src": "src", "child": "child"}
	c := base()
	lines, err := processLines(c, sources)
	if err != nil {
		t.Fatal(err)
	}
	if !lines["child"][3] || lines["src"][3] || len(lines["src"]) != 2 {
		t.Fatalf("wrong child union: %v", lines)
	}
	for name, mutate := range map[string]func(*Capture){
		"duplicate": func(c *Capture) { c.Processes[1].ID = 1 }, "parent": func(c *Capture) { c.Processes[1].Parent = 9 }, "cycle": func(c *Capture) {
			c.Processes = append(c.Processes, Process{ID: 3, Parent: 2, Exited: true, Instrumented: true, Reports: []Report{reports("lcov", "src", 1)}})
			c.Processes[1].Parent = 3
		}, "two roots": func(c *Capture) { c.Processes[1].Parent = 0 }, "not retired": func(c *Capture) { c.Processes[1].Exited = false }, "uninstrumented": func(c *Capture) { c.Processes[1].Instrumented = false }, "malformed child": func(c *Capture) { c.Processes[1].Reports[0].Data = []byte("SF:child\nDA:3,1\n") },
	} {
		t.Run(name, func(t *testing.T) {
			c := base()
			mutate(&c)
			if lines, err := processLines(c, sources); err == nil || lines != nil {
				t.Fatal("invalid tree published coverage")
			}
		})
	}
}

// LPCV-V0-8200..8204: instrumentation is inherited by native subprocesses.
func TestInstrumentation(t *testing.T) {
	for _, r := range []string{"junit-platform", "junit4", "testng", "gradle-junit", "maven-surefire", "kotest", "robolectric", "dotnet-vstest-nunit", "dotnet-mtp-xunit", "ctest", "googletest", "catch2", "cargo-test", "nextest", "swift-xctest", "swift-testing"} {
		t.Run(r, func(t *testing.T) {
			o := options(t, r)
			inv := tr.Invocation{Argv: []string{"test"}, Phases: []tr.Phase{{Kind: "TEST"}, {Kind: "DECODE"}}}
			if err := instrument(o, "/private/run", &inv); err != nil {
				t.Fatal(err)
			}
			switch Family(r) {
			case "jvm":
				if !strings.Contains(inv.Environment["JAVA_TOOL_OPTIONS"], "append=true,dumponexit=true") {
					t.Fatal(inv)
				}
			case "dotnet":
				if !strings.Contains(strings.Join(inv.Argv, " "), "overage") {
					t.Fatal(inv)
				}
			default:
				if !strings.Contains(inv.Environment["LLVM_PROFILE_FILE"], "%p-%m") {
					t.Fatal(inv)
				}
			}
			if len(inv.Phases[1].Environment) != 0 {
				t.Fatal("changed decode environment")
			}
		})
	}
}

// LPCV-V0-9400: native invocations need their authoritative inventory even
// before execution. No runner report can safely substitute a missing identity.
func TestCollectorInventories(t *testing.T) {
	t.Run("LPCV-V0-9400 inventories", func(t *testing.T) {
		for _, runner := range []string{"cmocka-xml", "boost-test-junit"} {
			t.Run(runner, func(t *testing.T) {
				o := options(t, runner)
				o.Request.Project = ""
				o.Request.Reporter = ""
				o.Request.ReporterSha256 = ""
				o.Request.Config = ""
				o.Request.ConfigSha256 = ""
				o.Request.Target = "probe"
				original := []string{"probe::alpha", "probe::beta"}
				o.Request.ExpectedTests = append([]string(nil), original...)
				calls := 0
				o.Run = func(_ context.Context, r tr.Request, _ tr.Invocation) (Capture, error) {
					calls++
					want := original
					if runner == "cmocka-xml" && len(r.Selectors) > 0 {
						want = r.Selectors
					}
					if !reflect.DeepEqual(r.ExpectedTests, want) {
						t.Fatalf("expected inventory %v want %v", r.ExpectedTests, want)
					}
					return Capture{Input: tr.Input{Runner: runner}}, nil
				}
				out, err := Collect(context.Background(), o)
				if err != nil {
					t.Fatal(err)
				}
				if calls != 1 || len(out) != 1 || out[0].Record != nil {
					t.Fatalf("calls=%d out=%+v", calls, out)
				}
				if _, _, err := o.run(context.Background(), t.TempDir(), 1, "probe::alpha", "probe::alpha"); err != nil {
					t.Fatal(err)
				}
				if calls != 2 {
					t.Fatal("isolated invocation did not reach adapter")
				}
				if !reflect.DeepEqual(o.Request.ExpectedTests, original) {
					t.Fatal("caller inventory mutated")
				}
			})
		}
	})
}

func TestJUnitMethodSelectors(t *testing.T) {
	t.Run("LPCV-V0-9402 exact JVM methods", func(t *testing.T) {
		for _, tc := range []struct{ id, want string }{
			{"[engine:junit-jupiter]/[class:p.Probe]/[method:alpha()]", "p.Probe#alpha"},
			{"[engine:junit-vintage]/[class:p.Probe]/[method:beta()]", "p.Probe#beta"},
			{"[engine:junit-jupiter]/[class:p.Probe]/[method:alpha(java.lang.String)]", ""},
			{"[engine:junit-jupiter]/[class:p.Probe]/[method:alpha()]/[test-template-invocation:#1]", ""},
			{"[engine:junit-jupiter]/[class:p.Probe]/[nested-class:Nested]/[method:alpha()]", ""},
			{"[engine:junit-jupiter]/[class:p.Probe%2FNested]/[method:alpha()]", ""},
			{"[engine:junit-jupiter]/[class:]/[method:alpha()]", ""},
		} {
			got, ok := junitMethodSelector(tc.id)
			if got != tc.want || ok != (tc.want != "") {
				t.Fatalf("%q = %q %t want %q", tc.id, got, ok, tc.want)
			}
		}
	})
}
