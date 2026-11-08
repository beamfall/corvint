// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

var dotnetRunners = []string{"dotnet-mtp-mstest", "dotnet-mtp-nunit", "dotnet-mtp-xunit", "dotnet-vstest-mstest", "dotnet-vstest-nunit", "dotnet-vstest-xunit"}

const dotnetCS = "namespace Demo;\npublic class Probe\n{\n    public bool A()\n    {\n        return true;\n    }\n    public bool B()\n    {\n        return false;\n    }\n}\n"
const dotnetCoverage = `<coverage><sources><source>/repo</source></sources><packages><package name="Demo"><classes><class name="Demo.Probe" filename="Tests/Probe.cs"><lines><line number="6" hits="1"/><line number="10" hits="0"/></lines></class></classes></package></packages></coverage>`

func dotnetSource(name, data string) registry.DotnetSource {
	b := []byte(data)
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return registry.DotnetSource{Blob: testevidence.Blob{Path: testevidence.Path{Repository: "fixture", Lexical: name, Resolved: name}, OID: hex.EncodeToString(h.Sum(nil))}, Data: b}
}
func dotnetConfig(runner string) registry.DotnetConfig {
	platform := "vstest"
	if strings.Contains(runner, "-mtp-") {
		platform = "mtp"
	}
	return registry.DotnetConfig{
		Key:        tr.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"},
		Repository: "fixture", Root: "/repo", Executable: "/tools/dotnet", Project: "Tests/Probe.csproj", TFM: "net8.0", Platform: platform,
		Sources: map[string]registry.DotnetSource{
			"Tests/obj/project.assets.json": dotnetSource("Tests/obj/project.assets.json", `{"version":3,"targets":{"net8.0":{}},"libraries":{}}`),
			"Tests/Probe.cs":                dotnetSource("Tests/Probe.cs", dotnetCS),
			"Tests/Probe.csproj":            dotnetSource("Tests/Probe.csproj", `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFrameworks>net8.0;net9.0</TargetFrameworks></PropertyGroup></Project>`),
		},
		Projects:    map[string]registry.DotnetProject{"Tests/Probe.csproj": {Framework: "net8.0", Complete: true, Compile: []string{"Tests/Probe.cs"}, Assets: []byte(`{"version":3,"targets":{"net8.0":{}},"libraries":{}}`)}},
		Bindings:    map[string]string{"Demo.Probe.A": "Tests/Probe.cs", "Demo.Probe.B": "Tests/Probe.cs"},
		SourceNames: map[string]string{"Tests/Probe.cs": "Tests/Probe.cs", "/repo/Tests/Probe.cs": "Tests/Probe.cs"},
		Discovery:   registry.DotnetCapture{Complete: true}, Run: registry.DotnetCapture{Complete: true},
		Collections: []registry.DotnetCollection{{Tests: []string{"Demo.Probe.A", "Demo.Probe.B"}, Artifacts: []string{"coverage"}, Complete: true}},
		Artifacts:   map[string][]byte{"coverage": []byte(dotnetCoverage)},
	}
}
func newDotnet(t *testing.T, c registry.DotnetConfig) (registry.RunnerPlugin, tr.InventoryKey) {
	t.Helper()
	p, key, err := registry.NewDotnetPlugin(c)
	if err != nil {
		t.Fatal(err)
	}
	return p, key
}
func dotnetList(platform string, names ...string) string {
	if platform == "vstest" {
		return "The following Tests are available:\n    " + strings.Join(names, "\n    ") + "\n"
	}
	var tests []map[string]any
	for _, n := range names {
		tests = append(tests, map[string]any{"uid": n, "displayName": n, "location": map[string]any{"file": "/repo/Tests/Probe.cs", "lineStart": 5, "lineEnd": 6}})
	}
	b, _ := json.Marshal(map[string]any{"schemaVersion": 1, "tests": tests})
	return string(b) + "\n"
}
func dotnetInventory(t *testing.T, p registry.RunnerPlugin, key tr.InventoryKey, platform string) tr.RunnerInventory {
	t.Helper()
	in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: dotnetList(platform, "Demo.Probe.A", "Demo.Probe.B")})
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestRSPV0001DotnetPiecesUnqualified(t *testing.T) {
	for _, runner := range dotnetRunners {
		p, key := newDotnet(t, dotnetConfig(runner))
		pieces := p.Pieces()
		if !pieces.Inventory.Present || !pieces.TestEvidence.Present || pieces.Qualification.Present || pieces.Qualification.Reason != "unqualified" {
			t.Fatalf("%s: %+v", runner, pieces)
		}
		in := dotnetInventory(t, p, key, dotnetConfig(runner).Platform)
		plan := registry.NewSelectionSession(p, key, true).Plan(in, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || len(plan.Tests) != 2 {
			t.Fatalf("%+v", plan)
		}
	}
}
func TestRSPV0001DotnetUnknownPlatformFull(t *testing.T) {
	for _, platform := range []string{"", "guess", "mtp"} {
		c := dotnetConfig("dotnet-vstest-mstest")
		c.Platform = platform
		p, key := newDotnet(t, c)
		s := registry.NewSelectionSession(p, key, true)
		in := s.Inventory(context.Background(), dotnetList("vstest", "Demo.Probe.A"))
		plan := s.Plan(in, nil, nil)
		if in.Complete || !plan.Full || !strings.Contains(plan.Reason, "dotnet-platform-") {
			t.Fatalf("%s: %+v", platform, plan)
		}
		if _, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "run", RunDir: "/reports"}); err == nil {
			t.Fatal("guessed platform")
		}
	}
}

func TestRSPV0002DotnetDeterministicProjectTFMInventory(t *testing.T) {
	for _, runner := range dotnetRunners {
		c := dotnetConfig(runner)
		p, key := newDotnet(t, c)
		a := dotnetInventory(t, p, key, c.Platform)
		b, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: dotnetList(c.Platform, "Demo.Probe.B", "Demo.Probe.A")})
		if err != nil {
			t.Fatal(err)
		}
		ab, _ := tr.InventoryBytes(a, key)
		bb, _ := tr.InventoryBytes(b, key)
		if !bytes.Equal(ab, bb) {
			t.Fatalf("nondeterministic %s", runner)
		}
		for _, test := range a.Tests {
			if !strings.HasPrefix(test.ID, "Tests/Probe.csproj::net8.0::") || test.Path != "Tests/Probe.cs" {
				t.Fatal(test)
			}
		}
		c.TFM = "net9.0"
		other, otherKey := newDotnet(t, c)
		otherIn := dotnetInventory(t, other, otherKey, c.Platform)
		if otherKey == key || otherIn.Tests[0].ID == a.Tests[0].ID {
			t.Fatal("TFM identity collapsed")
		}
		if _, err := other.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: dotnetList(c.Platform, "Demo.Probe.A")}); err == nil {
			t.Fatal("cross-TFM inventory admitted")
		}
		c.Project = "Other/Probe.csproj"
		_, projectKey := newDotnet(t, c)
		if projectKey == otherKey {
			t.Fatal("project identity collapsed")
		}
	}
}
func TestRSPV0002DotnetInvalidInventoryFull(t *testing.T) {
	for _, platform := range []string{"vstest", "mtp", "mtp-msbuild"} {
		for _, mode := range []string{"failed", "truncated", "unterminated", "malformed", "duplicate", "source", "unknown-field", "target", "cancelled", "cap"} {
			t.Run(platform+"/"+mode, func(t *testing.T) {
				runner := "dotnet-mtp-mstest"
				if platform == "vstest" {
					runner = "dotnet-vstest-mstest"
				}
				c := dotnetConfig(runner)
				c.Platform = platform
				output := dotnetList(platform, "Demo.Probe.A")
				ctx := context.Background()
				switch mode {
				case "failed":
					c.Discovery.ExitCode = 1
				case "truncated":
					c.Discovery.Complete = false
				case "unterminated":
					output = strings.TrimSuffix(output, "\n")
				case "malformed":
					output = "not a list\n"
				case "duplicate":
					output = dotnetList(platform, "Demo.Probe.A", "Demo.Probe.A")
				case "source":
					delete(c.Sources, "Tests/Probe.cs")
				case "unknown-field":
					output += "unknown diagnostic\n"
				case "cancelled":
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				case "cap":
					output = strings.Repeat("x", tr.MaxReportBytes+1) + "\n"
				}
				p, key := newDotnet(t, c)
				if mode == "target" {
					key.Arch = "arm64"
				}
				in, err := p.Inventory(ctx, registry.InventoryRequest{Key: key, Output: output})
				if err == nil || in.Complete || in.Reason == "" || !tr.PlanRunSet(in, key, nil, nil).Full {
					t.Fatalf("%+v %v", in, err)
				}
			})
		}
	}
}

// Synthetic native TRX fixture. It uses the existing qualified native parser,
// including its adapter, result/entry/execution and counter validation.
func dotnetTRX(runner string, names ...string) (string, map[string]string) {
	adapter := map[string]string{
		"dotnet-vstest-mstest": "executor://mstestadapter/v2", "dotnet-vstest-nunit": "executor://nunit3testexecutor/", "dotnet-vstest-xunit": "executor://xunit/VsTestRunner2/netcoreapp",
		"dotnet-mtp-mstest": "executor://MSTest.Sdk/4.4.1", "dotnet-mtp-nunit": "executor://NUnitExtension/6.3.0+b5a66c0f74e52257b3ec1e04abc3ef812a7e74d5", "dotnet-mtp-xunit": "executor://30ea7c6e-dd24-4152-a360-1387158cd41d/4.0.1",
	}[runner]
	var results, definitions, entries strings.Builder
	bindings := map[string]string{}
	for i, name := range names {
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1)
		exec := fmt.Sprintf("10000000-0000-0000-0000-%012d", i+1)
		fmt.Fprintf(&results, `<UnitTestResult testId="%s" executionId="%s" testName="%s" outcome="Passed"/>`, id, exec, name)
		fmt.Fprintf(&definitions, `<UnitTest id="%s"><Execution id="%s"/><TestMethod className="Demo.Probe" name="%s" adapterTypeName="%s"/></UnitTest>`, id, exec, strings.TrimPrefix(name, "Demo.Probe."), adapter)
		fmt.Fprintf(&entries, `<TestEntry testId="%s" executionId="%s"/>`, id, exec)
		reported := name
		if strings.Contains(runner, "-mtp-") {
			reported += "::" + id
		}
		bindings[reported] = name
	}
	counters := fmt.Sprintf(`total="%d" executed="%d" passed="%d" failed="0" error="0" timeout="0" aborted="0" inconclusive="0" passedButRunAborted="0" notRunnable="0" notExecuted="0" disconnected="0" warning="0" completed="0" inProgress="0" pending="0"`, len(names), len(names), len(names))
	return fmt.Sprintf(`<TestRun xmlns="http://microsoft.com/schemas/VisualStudio/TeamTest/2010"><Results>%s</Results><TestDefinitions>%s</TestDefinitions><TestEntries>%s</TestEntries><ResultSummary outcome="Completed"><Counters %s/></ResultSummary></TestRun>`, &results, &definitions, &entries, counters), bindings
}
func TestRSPV0003DotnetTerminalResults(t *testing.T) {
	for _, runner := range dotnetRunners {
		report, bridge := dotnetTRX(runner, "Demo.Probe.A", "Demo.Probe.B")
		c := dotnetConfig(runner)
		c.ResultIDs = bridge
		p, key := newDotnet(t, c)
		in := dotnetInventory(t, p, key, c.Platform)
		results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: report})
		if err != nil {
			t.Fatal(runner, err)
		}
		if got := tr.ReconcileInventory(in, key, tr.PlanRunSet(in, key, nil, nil), results, false); !got.Succeeded || got.Full {
			t.Fatalf("%s: %+v", runner, got)
		}
	}
}
func TestRSPV0003DotnetMissingResultsRetryAndDivergence(t *testing.T) {
	for _, runner := range dotnetRunners {
		one, _ := dotnetTRX(runner, "Demo.Probe.A")
		all, bridge := dotnetTRX(runner, "Demo.Probe.A", "Demo.Probe.B")
		c := dotnetConfig(runner)
		c.ResultIDs = bridge
		p, key := newDotnet(t, c)
		in := dotnetInventory(t, p, key, c.Platform)
		s := registry.NewSelectionSession(p, key, false)
		calls := 0
		out := s.Run(context.Background(), in, nil, nil, "/reports", true, func(_ context.Context, command registry.CommandResponse) (string, error) {
			calls++
			if strings.Contains(strings.Join(command.Argv, " "), "--filter") {
				t.Fatal("retry filtered")
			}
			if calls == 1 {
				return one, nil
			}
			return all, nil
		})
		if calls != 2 || !out.Succeeded || !out.NeedsRequalification || !reflect.DeepEqual(out.Attempts[0].Reconciliation.Missing, []string{"Tests/Probe.csproj::net8.0::Demo.Probe.B"}) {
			t.Fatalf("%s: %+v", runner, out)
		}
		if plan := s.Plan(in, nil, nil); !plan.Full || !strings.Contains(plan.Reason, "divergence") {
			t.Fatal(plan)
		}
		extra, bridge := dotnetTRX(runner, "Demo.Probe.A", "Demo.Probe.X")
		c.ResultIDs = bridge
		p, key = newDotnet(t, c)
		results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: extra})
		if err != nil {
			t.Fatal(err)
		}
		r := tr.ReconcileInventory(in, key, tr.PlanRunSet(in, key, nil, nil), results, false)
		if r.Succeeded || !r.NeedsRequalification || len(r.Unexpected) != 1 {
			t.Fatal(r)
		}
	}
}
func TestRSPV0003DotnetMalformedAndIncompleteReport(t *testing.T) {
	c := dotnetConfig("dotnet-mtp-mstest")
	report, bridge := dotnetTRX(c.Key.Runner, "Demo.Probe.A")
	c.ResultIDs = bridge
	for _, mode := range []string{"xml", "capture", "counter", "runner", "skip-reason"} {
		t.Run(mode, func(t *testing.T) {
			c := c
			raw := report
			runner := c.Key.Runner
			switch mode {
			case "xml":
				raw = strings.TrimSuffix(raw, "</TestRun>")
			case "capture":
				c.Run.Complete = false
			case "counter":
				raw = strings.Replace(raw, `total="1"`, `total="2"`, 1)
			case "runner":
				runner = "dotnet-vstest-mstest"
			case "skip-reason":
				raw = strings.ReplaceAll(raw, `outcome="Passed"`, `outcome="NotExecuted"`)
				raw = strings.ReplaceAll(raw, `executed="1"`, `executed="0"`)
				raw = strings.ReplaceAll(raw, `passed="1"`, `passed="0"`)
				raw = strings.ReplaceAll(raw, `notExecuted="0"`, `notExecuted="1"`)
			}
			p, key := newDotnet(t, c)
			in := dotnetInventory(t, p, key, c.Platform)
			results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: raw})
			if mode == "skip-reason" {
				if err != nil {
					t.Fatal(err)
				}
				r := tr.ReconcileInventory(in, key, tr.RunSet{Tests: []string{in.Tests[0].ID}}, results, false)
				if r.Succeeded || len(r.Missing) != 1 {
					t.Fatal("skip without reason succeeded", r)
				}
			} else if err == nil {
				t.Fatal("invalid report admitted")
			}
		})
	}
}

func TestRSPV0004DotnetExclusionAndSuperset(t *testing.T) {
	for _, platform := range []string{"vstest", "mtp", "mtp-msbuild"} {
		c := dotnetConfig("dotnet-mtp-mstest")
		c.Platform = platform
		if platform == "vstest" {
			c.Key.Runner = "dotnet-vstest-mstest"
		}
		p, key := newDotnet(t, c)
		in := dotnetInventory(t, p, key, platform)
		plan := tr.PlanRunSet(in, key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}, {ID: in.Tests[1].ID, Fresh: false}}, nil)
		if plan.Full || !reflect.DeepEqual(plan.Tests, []string{in.Tests[1].ID}) {
			t.Fatal(plan)
		}
		for _, purpose := range []string{"inventory", "run", "collect"} {
			command, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: purpose, Tests: plan.Tests, RunDir: "/reports"})
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Join(command.Argv, " ")
			if !command.Superset || strings.Contains(args, "--filter") || !strings.Contains(args, "--framework net8.0 --no-build --no-restore") || command.Cwd != "/repo" {
				t.Fatal(command)
			}
			if platform == "mtp-msbuild" && !strings.Contains(args, "-- ") {
				t.Fatal("missing MTP separator")
			}
			if purpose == "inventory" && !strings.Contains(args, "--list-tests") {
				t.Fatal(command)
			}
		}
		mandated := tr.PlanRunSet(in, key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, []string{in.Tests[0].ID})
		if len(mandated.Tests) != 2 {
			t.Fatal(mandated)
		}
	}
}
func TestRSPV0004DotnetInvalidSelectionNeverSubset(t *testing.T) {
	p, key := newDotnet(t, dotnetConfig("dotnet-vstest-mstest"))
	for _, tests := range [][]string{{}, {"Other.csproj::net8.0::Demo.Probe.A"}, {"Tests/Probe.csproj::net9.0::Demo.Probe.A"}, {"Tests/Probe.csproj::net8.0::a\n--filter"}} {
		if _, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "run", Tests: tests, RunDir: "/reports"}); err == nil {
			t.Fatal("invalid selection admitted", tests)
		}
	}
	command, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "run", RunDir: "/reports"})
	if err != nil || command.Superset || strings.Contains(strings.Join(command.Argv, " "), "--filter") {
		t.Fatal(command, err)
	}
}

func dotnetNative(t *testing.T, p registry.RunnerPlugin, runner string) []testevidence.Native {
	t.Helper()
	evidence, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"coverage"}})
	if err != nil {
		t.Fatal(err)
	}
	var out []testevidence.Native
	for _, e := range evidence {
		var n testevidence.Native
		for _, pair := range []struct {
			raw    []byte
			target any
		}{{e.Executed, &n.Executed}, {e.Regions, &n.Regions}, {e.Loaded, &n.Loaded}} {
			if err := json.Unmarshal(pair.raw, pair.target); err != nil {
				t.Fatal(err)
			}
		}
		r := testevidence.New("fixture", e.Test, strings.Repeat("a", 40))
		r.Executed, r.Regions, r.Loaded = n.Executed, n.Regions, n.Loaded
		if err := r.ValidateComponents(); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}
func TestRSPV0005DotnetCoberturaRegionsLoadedAndCoreTrace(t *testing.T) {
	for _, runner := range dotnetRunners {
		c := dotnetConfig(runner)
		p, key := newDotnet(t, c)
		evidence := dotnetNative(t, p, runner)
		if len(evidence) != 2 || !reflect.DeepEqual(evidence[0], evidence[1]) {
			t.Fatal("shared-tree coverage not union-attributed", evidence)
		}
		n := evidence[0]
		if n.Executed.State != testevidence.Present || n.Loaded.State != testevidence.Present || n.Regions.State != testevidence.Present || len(n.Executed.Items) != 1 || len(n.Loaded.Items) != 3 || len(n.Regions.Items) != 1 || len(n.Regions.Items[0].Bodies) != 2 {
			t.Fatalf("%+v", n)
		}
		file := n.Executed.Items[0]
		if file.Executed[0].Start != 6 || file.Unexecuted[0].Start != 10 || n.Regions.Items[0].Bodies[0].Range.Start != 6 || n.Regions.Items[0].Bodies[0].Range.End != 7 {
			t.Fatal(n)
		}
		header := testevidence.New("fixture", "Tests/Probe.csproj::net8.0::Demo.Probe.A", key.Checkout)
		header.Runner = testevidence.RunnerIdentity{Plugin: "dotnet", PluginVersion: "fixture", RunnerVersion: "fixture", ExecutableDigest: strings.Repeat("c", 64), ConfigurationDigest: key.ConfigDigest}
		header.Execution = testevidence.ExecutionIdentity{OS: key.OS, Architecture: key.Arch, TracingBackend: "fixture-core", TracingVersion: "1", EnvironmentDigest: strings.Repeat("d", 64)}
		header.CollectionCount = 1
		header.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("c", 64), VerifierRevision: key.Checkout, ConfigurationDigest: key.ConfigDigest, ContentDigest: strings.Repeat("e", 64), Ref: "refs/heads/main", Run: "fixture-run"}}
		trace := testevidence.Trace{Reads: testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{c.Sources["Tests/Probe.cs"].Blob}}, Listings: testevidence.Component[testevidence.Path]{State: testevidence.Present}, Lookups: testevidence.Component[testevidence.Path]{State: testevidence.Present}, Writes: testevidence.Component[testevidence.Write]{State: testevidence.Present}, Execs: testevidence.Component[testevidence.Executable]{State: testevidence.Present}, External: testevidence.Component[testevidence.Contact]{State: testevidence.Present}}
		record, err := testevidence.Assemble(header, n, trace)
		if err != nil || !reflect.DeepEqual(record.Reads, trace.Reads) {
			t.Fatal(record, err)
		}
		// Mutating the caller's captured attachment cannot change the factory.
		c.Artifacts["coverage"][0] = '!'
		if got := dotnetNative(t, p, runner); !reflect.DeepEqual(got, evidence) {
			t.Fatal("attachment mutated")
		}
	}
}
func TestRSPV0005DotnetIncompleteNativeEvidence(t *testing.T) {
	for _, mode := range []string{"coverage", "unknown-xml", "coverage-cap", "capture", "unmapped", "line", "blob", "assets", "assets-target", "assets-duplicate", "closure", "regions", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			c := dotnetConfig("dotnet-vstest-mstest")
			switch mode {
			case "coverage":
				c.Artifacts["coverage"] = []byte("<coverage>")
			case "unknown-xml":
				c.Artifacts["coverage"] = []byte(strings.Replace(dotnetCoverage, "<lines>", "<unknown/><lines>", 1))
			case "coverage-cap":
				c.Artifacts["coverage"] = bytes.Repeat([]byte("x"), tr.MaxReportBytes+1)
			case "capture":
				c.Collections[0].Complete = false
			case "unmapped":
				c.SourceNames = nil
			case "line":
				c.Artifacts["coverage"] = []byte(strings.Replace(dotnetCoverage, `number="6"`, `number="99999"`, 1))
			case "blob":
				s := c.Sources["Tests/Probe.cs"]
				s.Data = []byte("different bytes")
				c.Sources["Tests/Probe.cs"] = s
			case "assets":
				project := c.Projects[c.Project]
				project.Assets = nil
				c.Projects[c.Project] = project
				c.Sources["Tests/obj/project.assets.json"] = dotnetSource("Tests/obj/project.assets.json", string(project.Assets))
			case "assets-target":
				project := c.Projects[c.Project]
				project.Assets = []byte(`{"version":3,"targets":{"net9.0":{}},"libraries":{}}`)
				c.Projects[c.Project] = project
				c.Sources["Tests/obj/project.assets.json"] = dotnetSource("Tests/obj/project.assets.json", string(project.Assets))
			case "assets-duplicate":
				project := c.Projects[c.Project]
				project.Assets = []byte(`{"version":3,"version":3,"targets":{"net8.0":{}},"libraries":{}}`)
				c.Projects[c.Project] = project
				c.Sources["Tests/obj/project.assets.json"] = dotnetSource("Tests/obj/project.assets.json", string(project.Assets))
			case "closure":
				project := c.Projects[c.Project]
				project.Complete = false
				c.Projects[c.Project] = project
			case "regions":
				c.Sources["Tests/Probe.cs"] = dotnetSource("Tests/Probe.cs", strings.Replace(dotnetCS, "return true;", "if (true) { return true; }", 1))
			case "symlink":
				s := c.Sources["Tests/Probe.cs"]
				s.Blob.Path.Resolved = "Other/Probe.cs"
				c.Sources["Tests/Probe.cs"] = s
			}
			p, _ := newDotnet(t, c)
			if mode == "blob" || mode == "symlink" {
				if _, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner, Artifacts: []string{"coverage"}}); err == nil {
					t.Fatal("unbound source admitted")
				}
				return
			}
			for _, evidence := range dotnetNative(t, p, c.Key.Runner) {
				if evidence.Executed.State == testevidence.Present && evidence.Loaded.State == testevidence.Present && evidence.Regions.State == testevidence.Present {
					t.Fatal("incomplete input became complete", evidence)
				}
			}
		})
	}
}
func TestRSPV0005DotnetProjectReferencesAndAssets(t *testing.T) {
	c := dotnetConfig("dotnet-vstest-mstest")
	c.Sources["Lib/Lib.csproj"] = dotnetSource("Lib/Lib.csproj", `<Project Sdk="Microsoft.NET.Sdk"/>`)
	c.Sources["Lib/Lib.cs"] = dotnetSource("Lib/Lib.cs", "public class Lib\n{\n    public int Value()\n    {\n        return 1;\n    }\n}\n")
	c.Projects["Lib/Lib.csproj"] = registry.DotnetProject{Framework: "net8.0", Complete: true, Compile: []string{"Lib/Lib.cs"}, Assets: []byte(`{"version":3,"targets":{"net8.0":{}},"libraries":{}}`)}
	project := c.Projects[c.Project]
	project.References = []string{"Lib/Lib.csproj"}
	project.Assets = []byte(`{"version":3,"targets":{"net8.0":{"Lib/1.0.0":{"type":"project"}}},"libraries":{"Lib/1.0.0":{"type":"project","path":"../Lib/Lib.csproj","msbuildProject":"../Lib/Lib.csproj"}}}`)
	c.Projects[c.Project] = project
	c.Sources["Tests/obj/project.assets.json"] = dotnetSource("Tests/obj/project.assets.json", string(project.Assets))
	c.Sources["Lib/obj/project.assets.json"] = dotnetSource("Lib/obj/project.assets.json", string(c.Projects["Lib/Lib.csproj"].Assets))
	p, _ := newDotnet(t, c)
	if n := dotnetNative(t, p, c.Key.Runner)[0]; n.Loaded.State != testevidence.Present || len(n.Loaded.Items) != 6 || len(n.Regions.Items) != 2 {
		t.Fatal(n)
	}
	for _, mode := range []string{"undeclared", "missing", "cycle"} {
		t.Run(mode, func(t *testing.T) {
			c := c
			c.Projects = map[string]registry.DotnetProject{c.Project: project, "Lib/Lib.csproj": c.Projects["Lib/Lib.csproj"]}
			switch mode {
			case "undeclared":
				q := project
				q.References = nil
				c.Projects[c.Project] = q
			case "missing":
				delete(c.Projects, "Lib/Lib.csproj")
			case "cycle":
				q := c.Projects["Lib/Lib.csproj"]
				q.References = []string{c.Project}
				q.Assets = []byte(`{"version":3,"targets":{"net8.0":{"Probe/1":{"type":"project"}}},"libraries":{"Probe/1":{"type":"project","path":"../Tests/Probe.csproj"}}}`)
				c.Projects["Lib/Lib.csproj"] = q
				c.Sources["Lib/obj/project.assets.json"] = dotnetSource("Lib/obj/project.assets.json", string(q.Assets))
			}
			p, _ := newDotnet(t, c)
			if n := dotnetNative(t, p, c.Key.Runner)[0]; n.Loaded.State != testevidence.Incomplete || n.Loaded.Reason == "" {
				t.Fatal(n)
			}
		})
	}
}
func TestRSPV0005DotnetRejectsUnknownArtifactOrAttribution(t *testing.T) {
	for _, mode := range []string{"missing", "unknown", "duplicate", "unattributed", "test"} {
		c := dotnetConfig("dotnet-vstest-mstest")
		handles := []string{"coverage"}
		switch mode {
		case "missing":
			handles = nil
		case "unknown":
			handles = []string{"/etc/passwd"}
		case "duplicate":
			handles = append(handles, "coverage")
		case "unattributed":
			c.Artifacts["extra"] = []byte(dotnetCoverage)
			handles = append(handles, "extra")
		case "test":
			c.Collections[0].Tests = []string{"unknown"}
		}
		p, _ := newDotnet(t, c)
		if _, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner, Artifacts: handles}); err == nil {
			t.Fatal(mode, "accepted")
		}
	}
}

func TestRSPV0005DotnetBodyMutationCandidates(t *testing.T) {
	a, err := registry.DotnetBodyMutants([]byte(dotnetCS))
	b, again := registry.DotnetBodyMutants([]byte(dotnetCS))
	if err != nil || again != nil || !reflect.DeepEqual(a, b) || len(a) != 2 || a[0].Line != 6 || strings.TrimSpace(a[0].After) != "return false;" || a[1].Line != 10 || strings.TrimSpace(a[1].After) != "return true;" {
		t.Fatal(a, err, again)
	}
	if _, err := registry.DotnetBodyMutants([]byte("public class A {\npublic bool F() => true;\n}\n")); err == nil {
		t.Fatal("unsupported body silently admitted")
	}
	if _, err := registry.DotnetBodyMutants([]byte("public class A {\npublic string F() {\nreturn @\"\nreturn true;\n\";\n}\n}\n")); err == nil {
		t.Fatal("literal text became a return mutant")
	}
}

func TestRSPV0005DotnetBodyMutationLineEndings(t *testing.T) {
	want, err := registry.DotnetBodyMutants([]byte(dotnetCS))
	if err != nil || len(want) != 2 {
		t.Fatal(want, err)
	}
	for _, ending := range []string{"\r", "\r\n"} {
		got, err := registry.DotnetBodyMutants([]byte(strings.ReplaceAll(dotnetCS, "\n", ending)))
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("ending %q: %v, %v; want %v", ending, got, err, want)
		}
	}
}

func dotnetAttachmentTRX(hrefs ...string) string {
	var attachments strings.Builder
	for _, href := range hrefs {
		fmt.Fprintf(&attachments, `<UriAttachment><A href="%s" /></UriAttachment>`, href)
	}
	return `<TestRun xmlns="http://microsoft.com/schemas/VisualStudio/TeamTest/2010"><ResultSummary><CollectorDataEntries><Collector uri="datacollector://microsoft/CoverletCodeCoverage/1.0"><UriAttachments>` + attachments.String() + `</UriAttachments></Collector></CollectorDataEntries></ResultSummary></TestRun>`
}

func TestRSPV0005DotnetVSTestNativeArtifactLayout(t *testing.T) {
	c := dotnetConfig("dotnet-vstest-mstest")
	p, key := newDotnet(t, c)
	runDir := t.TempDir()
	command, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "collect", RunDir: runDir})
	if err != nil || !reflect.DeepEqual(command.Artifacts, []string{"results.trx"}) {
		t.Fatal(command, err)
	}
	const attachment = "bc5e983b-d7a8-4f17-8c0a-8a8831a4a891/coverage.cobertura.xml"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(runDir, attachment)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, attachment), []byte(dotnetCoverage), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "results.trx"), []byte(dotnetAttachmentTRX(attachment)), 0600); err != nil {
		t.Fatal(err)
	}
	// Core follows the declared report, then its bounded attachment paths.
	report, err := os.ReadFile(filepath.Join(runDir, command.Artifacts[0]))
	if err != nil {
		t.Fatal(err)
	}
	handles, err := registry.DotnetVSTestArtifacts(report)
	if err != nil || !reflect.DeepEqual(handles, []string{attachment}) {
		t.Fatal(handles, err)
	}
	c.Artifacts = map[string][]byte{}
	for _, name := range handles {
		data, err := os.ReadFile(filepath.Join(runDir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		c.Artifacts[name] = data
	}
	c.Collections[0].Artifacts = handles
	p, _ = newDotnet(t, c)
	evidence, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner, Artifacts: handles})
	if err != nil || len(evidence) != 2 {
		t.Fatal(evidence, err)
	}
	for _, e := range evidence {
		var executed testevidence.Component[testevidence.ExecutedFile]
		if err := json.Unmarshal(e.Executed, &executed); err != nil || executed.State != testevidence.Present || len(executed.Items) != 1 || len(executed.Items[0].Executed) != 1 {
			t.Fatal(executed, err)
		}
	}
	// Windows-native separator spelling has the same captured relative handle.
	paths, err := registry.DotnetVSTestArtifacts([]byte(dotnetAttachmentTRX(strings.ReplaceAll(attachment, "/", "\\"))))
	if err != nil || !reflect.DeepEqual(paths, handles) {
		t.Fatal(paths, err)
	}
}

func TestRSPV0005DotnetVSTestArtifactBounds(t *testing.T) {
	for _, href := range []string{"../coverage.cobertura.xml", "/coverage.cobertura.xml", `C:\coverage.cobertura.xml`, "guid/../coverage.cobertura.xml", "%2e%2e/coverage.cobertura.xml", "file:///coverage.cobertura.xml", "guid/other.xml", ""} {
		if paths, err := registry.DotnetVSTestArtifacts([]byte(dotnetAttachmentTRX(href))); err == nil {
			t.Fatal("unsafe attachment accepted", href, paths)
		}
	}
	for _, report := range []string{dotnetAttachmentTRX(), dotnetAttachmentTRX("guid/coverage.cobertura.xml", "guid/coverage.cobertura.xml"), strings.TrimSuffix(dotnetAttachmentTRX("guid/coverage.cobertura.xml"), "</TestRun>"), strings.Repeat("x", tr.MaxReportBytes+1), dotnetAttachmentTRX("guid/coverage.cobertura.xml") + "<TestRun/>"} {
		if _, err := registry.DotnetVSTestArtifacts([]byte(report)); err == nil {
			t.Fatal("missing, malformed, duplicate or capped capture accepted")
		}
	}
}
