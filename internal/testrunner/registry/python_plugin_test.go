// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func pythonFixture(t *testing.T, runner string) (RunnerPlugin, tr.RunnerInventory, string) {
	t.Helper()
	p, ok := PythonPlugin(runner, PythonCapture{})
	if !ok {
		t.Fatal(runner)
	}
	ids := []string{"tests/test_math.py::test_a", "tests/test_math.py::test_b[x y]"}
	if runner == "unittest" {
		ids = []string{"tests.test_math.Math.test_a", "tests.test_math.Math.test_b"}
	}
	in := tr.RunnerInventory{Key: tr.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"}, Complete: true, Tests: []tr.InventoryTest{{ID: ids[0], Path: "tests/test_math.py"}, {ID: ids[1], Path: "tests/test_math.py"}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return p, in, string(raw)
}

// RSP-V0-001: both compiled-in factories declare pieces without qualification.
func TestRSPV0001PythonPieces(t *testing.T) {
	for _, runner := range []string{"pytest", "unittest"} {
		p, in, _ := pythonFixture(t, runner)
		pieces := p.Pieces()
		if !pieces.Inventory.Present || !pieces.TestEvidence.Present || pieces.Qualification.Present || pieces.Qualification.Reason != "unqualified" {
			t.Fatalf("%s: %+v", runner, pieces)
		}
		for _, qualified := range []bool{false, true} {
			plan := NewSelectionSession(p, in.Key, qualified).Plan(in, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
			if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || len(plan.Tests) != 2 {
				t.Fatalf("%+v", plan)
			}
		}
	}
}

// RSP-V0-001: unknown runners never receive a factory or evidence capability.
func TestRSPV0001PythonUnknownRunner(t *testing.T) {
	if p, ok := PythonPlugin("other", PythonCapture{}); ok || p != nil {
		t.Fatal("unknown runner admitted")
	}
}

// RSP-V0-002: identical discovery inputs yield byte-equal canonical inventory.
func TestRSPV0002PythonDeterministicInventory(t *testing.T) {
	for _, runner := range []string{"pytest", "unittest"} {
		p, in, raw := pythonFixture(t, runner)
		first, err := p.Inventory(context.Background(), InventoryRequest{Key: in.Key, Output: raw})
		if err != nil {
			t.Fatal(err)
		}
		in.Tests[0], in.Tests[1] = in.Tests[1], in.Tests[0]
		reversed, _ := json.Marshal(in)
		second, err := p.Inventory(context.Background(), InventoryRequest{Key: in.Key, Output: string(reversed)})
		if err != nil {
			t.Fatal(err)
		}
		a, _ := tr.InventoryBytes(first, in.Key)
		b, _ := tr.InventoryBytes(second, in.Key)
		if string(a) != string(b) {
			t.Fatal("inventory ordering drift")
		}
		cmd, err := p.Command(context.Background(), CommandRequest{Key: in.Key, Purpose: "inventory", RunDir: t.TempDir()})
		if err != nil || len(cmd.Argv) != 8 || cmd.Argv[1] != "-c" || !strings.Contains(cmd.Argv[2], "--collect-only") || !strings.Contains(cmd.Argv[2], "loader.discover") {
			t.Fatalf("%+v %v", cmd, err)
		}
	}
}

// RSP-V0-002: malformed, incomplete, capped and mismatched discovery stays FULL.
func TestRSPV0002PythonInvalidInventory(t *testing.T) {
	for _, runner := range []string{"pytest", "unittest"} {
		p, in, raw := pythonFixture(t, runner)
		variants := []string{"", raw[:len(raw)-1], strings.Replace(raw, `"Complete":true`, `"Complete":false`, 1), strings.Replace(raw, `"Complete":true`, `"Complete":true,"Complete":true`, 1), strings.Replace(raw, `"Reason":""`, `"Reason":"","Unexpected":1`, 1), strings.Replace(raw, `"Path":"tests/test_math.py"`, `"Path":"../escape.py"`, 1), strings.Repeat("x", tr.MaxReportBytes+1)}
		dup := in
		dup.Tests = []tr.InventoryTest{in.Tests[0], in.Tests[0]}
		b, _ := json.Marshal(dup)
		variants = append(variants, string(b))
		wrong := in
		wrong.Key.Arch = "arm64"
		b, _ = json.Marshal(wrong)
		variants = append(variants, string(b))
		for i, raw := range variants {
			s := NewSelectionSession(p, in.Key, true)
			got := s.Inventory(context.Background(), raw)
			if got.Complete || got.Reason == "" || !s.Plan(got, nil, nil).Full {
				t.Fatalf("%s case %d: %+v", runner, i, got)
			}
		}
	}
}

func pythonResults(in tr.RunnerInventory) []tr.RunnerResult {
	return []tr.RunnerResult{{ID: in.Tests[0].ID, Outcome: "passed"}, {ID: in.Tests[1].ID, Outcome: "skipped", Reason: "fixture disabled"}}
}

// RSP-V0-003: native terminal identities reconcile through the existing core.
func TestRSPV0003PythonTerminalResults(t *testing.T) {
	for _, runner := range []string{"pytest", "unittest"} {
		p, in, _ := pythonFixture(t, runner)
		raw, _ := json.Marshal(pythonResults(in))
		out := NewSelectionSession(p, in.Key, false).Run(context.Background(), in, nil, nil, t.TempDir(), true, func(context.Context, CommandResponse) (string, error) { return string(raw), nil })
		if !out.Succeeded || out.NeedsRequalification || len(out.Attempts) != 1 {
			t.Fatalf("%+v", out)
		}
	}
}

// RSP-V0-003: execute the embedded adapter, then reconcile its actual output.
// Only native subtests may map to the active parent; unknown callbacks remain
// unproven even if they expose a plausible parent identity.
func TestRSPV0003UnittestAdapterSubtests(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("unittest adapter fixtures require python3: ", err)
	}
	for _, tc := range []struct {
		name, body, outcome, reason string
		unknown                     bool
	}{
		{"skip", "with self.subTest(value=1): self.skipTest('unavailable')", "skipped", "unavailable", false},
		{"nested", "with self.subTest(outer=1):\n            with self.subTest(inner=2): self.skipTest('nested unavailable')", "skipped", "nested unavailable", false},
		{"mixed", "with self.subTest(value=1): self.skipTest('first')\n        with self.subTest(value=2): pass\n        with self.subTest(value=3): self.skipTest('second')", "skipped", "first\nsecond", false},
		{"fail_then_skip", "with self.subTest(value=1): self.fail('broken')\n        with self.subTest(value=2): self.skipTest('unavailable')", "failed", "unavailable", false},
		{"skip_then_fail", "with self.subTest(value=1): self.skipTest('unavailable')\n        with self.subTest(value=2): self.fail('broken')", "failed", "unavailable", false},
		{"pass", "with self.subTest(value=1): pass", "passed", "", false},
		{"unknown", "class Unknown:\n            test_case = self\n            def id(other): return self.id()\n        self._outcome.result.addSkip(Unknown(), 'unproven')", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "import unittest\nclass Case(unittest.TestCase):\n    def test_case(self):\n        " + tc.body + "\n"
			if err := os.WriteFile(filepath.Join(dir, "test_fixture.py"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			p, seed, _ := pythonFixture(t, "unittest")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			execute := func(command CommandResponse) string {
				t.Helper()
				cmd := exec.CommandContext(ctx, python, command.Argv[1:]...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				output, err := cmd.Output()
				code := 0
				if command.Argv[4] != "inventory" && tc.outcome == "failed" {
					code = 1
				}
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
					t.Fatalf("adapter: %v stderr=%s output=%s", err, &stderr, output)
				}
				return string(output)
			}
			command, err := p.Command(ctx, CommandRequest{Key: seed.Key, Purpose: "inventory", RunDir: dir})
			if err != nil {
				t.Fatal(err)
			}
			in, err := p.Inventory(ctx, InventoryRequest{Key: seed.Key, Output: execute(command)})
			if err != nil || len(in.Tests) != 1 || in.Tests[0].ID != "test_fixture.Case.test_case" {
				t.Fatalf("inventory: %+v %v", in, err)
			}
			out := NewSelectionSession(p, in.Key, false).Run(ctx, in, nil, nil, dir, true, func(_ context.Context, command CommandResponse) (string, error) {
				output := execute(command)
				var rows []tr.RunnerResult
				if err := json.Unmarshal([]byte(output), &rows); err != nil {
					t.Fatal(err)
				}
				if tc.unknown {
					if len(rows) != 0 {
						t.Fatalf("unknown form certified: %+v", rows)
					}
				} else if len(rows) != 1 || rows[0].ID != in.Tests[0].ID || rows[0].Outcome != tc.outcome || rows[0].Reason != tc.reason {
					t.Fatalf("terminal rows: %+v", rows)
				}
				return output, nil
			})
			if tc.unknown {
				if out.Succeeded || !out.NeedsRequalification || len(out.Attempts) != 2 || len(out.Attempts[1].Reconciliation.Missing) != 1 {
					t.Fatalf("unknown callback did not fail closed on FULL retry: %+v", out)
				}
			} else if out.NeedsRequalification || len(out.Attempts) != 1 || out.Succeeded != (tc.outcome != "failed") {
				t.Fatalf("parent reconciliation: %+v", out)
			}
		})
	}
}

// RSP-V0-003: a failing tearDownClass is an unproven callback outside the
// active test. It withholds passes, but cannot discard an earlier certified
// failure, so a passing FULL retry cannot turn the run green.
func TestRSPV0003UnittestTearDownClassKeepsFailure(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("unittest adapter fixtures require python3: ", err)
	}
	dir := t.TempDir()
	source := "import unittest\nclass Case(unittest.TestCase):\n    @classmethod\n    def tearDownClass(cls): raise RuntimeError('teardown broken')\n    def test_a(self): self.fail('broken')\n    def test_b(self): pass\n"
	if err := os.WriteFile(filepath.Join(dir, "test_fixture.py"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	p, seed, _ := pythonFixture(t, "unittest")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	execute := func(command CommandResponse) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, python, command.Argv[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		code := 0
		if command.Argv[4] != "inventory" {
			code = 1
		}
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
			t.Fatalf("adapter: %v stderr=%s output=%s", err, &stderr, output)
		}
		return string(output)
	}
	command, err := p.Command(ctx, CommandRequest{Key: seed.Key, Purpose: "inventory", RunDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	in, err := p.Inventory(ctx, InventoryRequest{Key: seed.Key, Output: execute(command)})
	if err != nil || len(in.Tests) != 2 {
		t.Fatalf("inventory: %+v %v", in, err)
	}
	passing, _ := json.Marshal([]tr.RunnerResult{{ID: "test_fixture.Case.test_a", Outcome: "passed"}, {ID: "test_fixture.Case.test_b", Outcome: "passed"}})
	calls := 0
	out := NewSelectionSession(p, in.Key, false).Run(ctx, in, nil, nil, dir, true, func(_ context.Context, command CommandResponse) (string, error) {
		calls++
		if calls > 1 {
			return string(passing), nil
		}
		output := execute(command)
		var rows []tr.RunnerResult
		if err := json.Unmarshal([]byte(output), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].ID != "test_fixture.Case.test_a" || rows[0].Outcome != "failed" {
			t.Fatalf("unproven run must keep only its certified failure: %+v", rows)
		}
		return output, nil
	})
	if calls != 2 || out.Succeeded || !out.NeedsRequalification || len(out.Attempts) != 2 || !out.Attempts[1].Plan.Full || !out.Attempts[1].Reconciliation.Succeeded {
		t.Fatalf("passing FULL retry erased failure: %+v calls=%d", out, calls)
	}
}

// RSP-V0-003: missing results trigger one FULL retry and sticky divergence;
// unknown identities, duplicate rows and reasonless skips never succeed.
func TestRSPV0003PythonDivergence(t *testing.T) {
	for _, runner := range []string{"pytest", "unittest"} {
		p, in, _ := pythonFixture(t, runner)
		for _, rerun := range []bool{false, true} {
			s := NewSelectionSession(p, in.Key, false)
			calls := 0
			out := s.Run(context.Background(), in, nil, nil, t.TempDir(), rerun, func(_ context.Context, cmd CommandResponse) (string, error) {
				calls++
				rows := pythonResults(in)
				if calls == 1 {
					rows = rows[:1]
				}
				b, _ := json.Marshal(rows)
				return string(b), nil
			})
			want := 1
			if rerun {
				want = 2
			}
			if calls != want || out.Succeeded != rerun || !out.NeedsRequalification || len(out.Attempts[0].Reconciliation.Missing) != 1 || !strings.Contains(s.Plan(in, nil, nil).Reason, "divergence") {
				t.Fatalf("%+v", out)
			}
		}
		for _, rows := range [][]tr.RunnerResult{{{ID: in.Tests[0].ID, Outcome: "skipped"}}, {{ID: in.Tests[0].ID, Outcome: "passed"}, {ID: in.Tests[0].ID, Outcome: "passed"}}, {{ID: "tests.test_unknown.Case.test_x", Outcome: "passed"}}, {{ID: in.Tests[0].ID, Outcome: "unknown"}}} {
			raw, _ := json.Marshal(rows)
			out := NewSelectionSession(p, in.Key, false).Run(context.Background(), in, nil, nil, t.TempDir(), false, func(context.Context, CommandResponse) (string, error) { return string(raw), nil })
			if out.Succeeded || !out.NeedsRequalification {
				t.Fatal(out)
			}
		}
	}
}

// RSP-V0-004: exact native node IDs stay distinct argv data, and core planning
// keeps head-new tests and stale records selected before qualification gating.
func TestRSPV0004PythonExactRunSet(t *testing.T) {
	for _, runner := range []string{"pytest", "unittest"} {
		p, in, _ := pythonFixture(t, runner)
		plan := tr.PlanRunSet(in, in.Key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
		if !reflect.DeepEqual(plan.Tests, []string{in.Tests[1].ID}) {
			t.Fatal(plan)
		}
		plan = tr.PlanRunSet(in, in.Key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: false}}, nil)
		if len(plan.Tests) != 2 {
			t.Fatal(plan)
		}
		dir := t.TempDir()
		cmd, err := p.Command(context.Background(), CommandRequest{Key: in.Key, Purpose: "collect", Tests: []string{in.Tests[1].ID}, RunDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		if err := json.Unmarshal([]byte(cmd.Argv[6]), &ids); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, []string{in.Tests[1].ID}) || cmd.Superset || !reflect.DeepEqual(cmd.Artifacts, []string{filepath.Join(dir, "coverage.json")}) {
			t.Fatalf("%+v", cmd)
		}
	}
}

// RSP-V0-004: empty selections never silently become native FULL; invalid
// selectors, duplicate tests, unsupported purposes and key mismatches refuse.
func TestRSPV0004PythonUnsupportedFilter(t *testing.T) {
	for _, runner := range []string{"pytest", "unittest"} {
		p, in, _ := pythonFixture(t, runner)
		for _, ids := range [][]string{{}, {"-k slow"}, {"@injected"}, {"../test_x.py::test_a"}, {in.Tests[0].ID, in.Tests[0].ID}} {
			if _, err := p.Command(context.Background(), CommandRequest{Key: in.Key, Purpose: "run", Tests: ids, RunDir: t.TempDir()}); err == nil {
				t.Fatalf("%s accepted %q", runner, ids)
			}
		}
		if _, err := p.Command(context.Background(), CommandRequest{Key: in.Key, Purpose: "other", RunDir: t.TempDir()}); err == nil {
			t.Fatal("unknown purpose")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := p.Command(ctx, CommandRequest{Key: in.Key, Purpose: "run", RunDir: t.TempDir()}); err == nil {
			t.Fatal("cancel ignored")
		}
	}
}

func pythonSource(text string) PythonSource {
	sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(text), text)))
	return PythonSource{Blob: testevidence.Blob{Path: testevidence.Path{Repository: "repo", Lexical: "maths.py", Resolved: "maths.py"}, OID: hex.EncodeToString(sum[:])}, Text: text}
}
func pythonCoverageFixture() ([]byte, map[string]PythonSource) {
	text := "def outer():\n    def inner():\n        return 1\n    return inner()\n"
	raw := []byte(`{"meta":{"format":3,"version":"7.6.1"},"files":{"maths.py":{"executed_lines":[1,2,3],"missing_lines":[4],"excluded_lines":[]}}}`)
	return raw, map[string]PythonSource{"maths.py": pythonSource(text)}
}

// RSP-V0-005: coverage.py lines and Python grammar spans retain pinned blobs;
// all tests in a shared process receive its union, never a context subset.
func TestRSPV0005PythonCoverageAndRegions(t *testing.T) {
	raw, sources := pythonCoverageFixture()
	native := PythonCoverage(raw, sources)
	if native.Executed.State != testevidence.Present || native.Regions.State != testevidence.Present || native.Loaded.State != testevidence.Absent {
		t.Fatalf("%+v", native)
	}
	if len(native.Executed.Items) != 1 || len(native.Executed.Items[0].Executed) != 3 || len(native.Executed.Items[0].Unexecuted) != 1 {
		t.Fatalf("%+v", native)
	}
	bodies := native.Regions.Items[0].Bodies
	if len(bodies) != 2 || bodies[0].Range.Start != 1 || bodies[0].Range.End != 4 || bodies[1].Parent != bodies[0].ID || bodies[1].Range.End != 3 {
		t.Fatalf("%+v", bodies)
	}
	for _, runner := range []string{"pytest", "unittest"} {
		_, in, _ := pythonFixture(t, runner)
		capture := PythonCapture{Coverage: raw, Sources: sources, Tests: []string{in.Tests[0].ID, in.Tests[1].ID}}
		p, _ := PythonPlugin(runner, capture)
		got, err := p.Evidence(context.Background(), EvidenceRequest{Runner: runner})
		if err != nil || len(got) != 2 || !reflect.DeepEqual(got[0].Executed, got[1].Executed) {
			t.Fatalf("%+v %v", got, err)
		}
		// A caller cannot mutate the first test's bytes to change the second.
		got[0].Executed[0] = 'x'
		if got[1].Executed[0] == 'x' {
			t.Fatal("aliased evidence")
		}
	}
}

// RSP-V0-005: unknown formats, duplicate members, excluded or invalid lines,
// absent artifacts, unpinned bytes and unsupported grammar never narrow.
func TestRSPV0005PythonMalformedCoverage(t *testing.T) {
	raw, sources := pythonCoverageFixture()
	cases := []string{"{", `{"files":{}}`, strings.Replace(string(raw), `"files":`, `"unknown":true,"files":`, 1), strings.Replace(string(raw), `"format":3`, `"format":99`, 1), strings.Replace(string(raw), `"format":3`, `"format":3,"format":3`, 1), strings.Replace(string(raw), `[1,2,3]`, `[0,2,3]`, 1), strings.Replace(string(raw), `[1,2,3]`, `[1,1,3]`, 1), strings.Replace(string(raw), `[4]`, `[3]`, 1), strings.Replace(string(raw), `"excluded_lines":[]`, `"excluded_lines":[2]`, 1), strings.Replace(string(raw), `"missing_lines":[4]`, `"missing_lines":null`, 1), strings.Repeat("x", tr.MaxReportBytes+1)}
	for i, bad := range cases {
		got := PythonCoverage([]byte(bad), sources)
		if got.Executed.State != testevidence.Incomplete || got.Executed.Reason == "" {
			t.Fatalf("%d: %+v", i, got)
		}
	}
	if got := PythonCoverage(nil, sources); got.Executed.State != testevidence.Absent || got.Executed.Reason == "" {
		t.Fatal(got)
	}
	for _, src := range []map[string]PythonSource{nil, {"maths.py": {Blob: sources["maths.py"].Blob, Text: "changed"}}} {
		if got := PythonCoverage(raw, src); got.Executed.State != testevidence.Incomplete {
			t.Fatal(got)
		}
	}
	malformed := pythonSource("def outer(:\n    broken\n    broken\n    broken\n")
	if got := PythonCoverage(raw, map[string]PythonSource{"maths.py": malformed}); got.Regions.State != testevidence.Incomplete {
		t.Fatal(got)
	}
	for _, runner := range []string{"pytest", "unittest"} {
		p, _, _ := pythonFixture(t, runner)
		if _, err := p.Evidence(context.Background(), EvidenceRequest{Runner: runner, Artifacts: []string{"../../.corvint/index"}}); err == nil {
			t.Fatal("artifact path accepted")
		}
	}
}

// RSP-V0-005: the adapter joins core trace components without upgrading their
// state, and refuses invalid native or trace components via the T1 assembler.
func TestRSPV0005PythonCoreTraceJoin(t *testing.T) {
	raw, sources := pythonCoverageFixture()
	native := PythonCoverage(raw, sources)
	h := testevidence.New("repo", "tests/test_math.py::test_a", strings.Repeat("a", 40))
	h.Runner = testevidence.RunnerIdentity{Plugin: "pytest", PluginVersion: "1", ExecutableDigest: strings.Repeat("b", 64), RunnerVersion: "8", ConfigurationDigest: strings.Repeat("c", 64)}
	h.Execution = testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64", TracingBackend: "fixture", TracingVersion: "1", EnvironmentDigest: strings.Repeat("d", 64)}
	h.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("e", 64), VerifierRevision: strings.Repeat("f", 40), ConfigurationDigest: strings.Repeat("c", 64), Ref: "refs/heads/main", Run: "run-1", ContentDigest: strings.Repeat("1", 64)}}
	h.CollectionCount = 1
	trace := testevidence.Trace{Reads: testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{sources["maths.py"].Blob}}, Listings: h.Listings, Lookups: h.Lookups, Writes: h.Writes, Execs: h.Execs, External: h.External}
	got, err := PythonRecord(h, native, trace)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Reads, trace.Reads) || got.External.State != testevidence.Absent {
		t.Fatal(got)
	}
	trace.Reads.State = testevidence.Incomplete
	trace.Reads.Reason = "trace-cap"
	got, err = PythonRecord(h, native, trace)
	if err != nil || got.Reads.State != testevidence.Incomplete || got.Reads.Reason != "trace-cap" {
		t.Fatalf("%+v %v", got, err)
	}
	trace.Reads.Reason = ""
	if _, err := PythonRecord(h, native, trace); err == nil {
		t.Fatal("invalid trace upgraded")
	}
}

// Project coverage settings must not hide executed source. RunDir is a fresh
// artifact staging directory, separate from the checkout used as the process cwd.
func TestRSPV0005PythonCoverageIgnoresProjectConfig(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	for _, runner := range []string{"pytest", "unittest"} {
		t.Run(runner, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			imports := "import coverage"
			if runner == "pytest" {
				imports += "; import pytest"
			}
			if out, err := exec.CommandContext(ctx, python, "-c", imports).CombinedOutput(); err != nil {
				t.Skipf("Python collector dependency unavailable: %v %s", err, out)
			}
			checkout, staging := t.TempDir(), t.TempDir()
			for name, data := range map[string]string{
				".coveragerc":     "[run]\nomit = *\n[report]\nomit = *\n",
				"test_fixture.py": "import unittest\nclass Case(unittest.TestCase):\n    def test_value(self):\n        self.assertEqual(2 + 2, 4)\n",
			} {
				if err := os.WriteFile(filepath.Join(checkout, name), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p, in, _ := pythonFixture(t, runner)
			cmd, err := p.Command(ctx, CommandRequest{Key: in.Key, Purpose: "collect", RunDir: staging})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cmd.Artifacts, []string{filepath.Join(staging, "coverage.json")}) {
				t.Fatal(cmd.Artifacts)
			}
			process := exec.CommandContext(ctx, python, cmd.Argv[1:]...)
			process.Dir = checkout
			process.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1", "PYTEST_DISABLE_PLUGIN_AUTOLOAD=1")
			if out, err := process.CombinedOutput(); err != nil {
				t.Fatalf("capture: %v %s", err, out)
			}
			raw, err := os.ReadFile(cmd.Artifacts[0])
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				Files map[string]struct {
					Executed []int `json:"executed_lines"`
				} `json:"files"`
			}
			if err := json.Unmarshal(raw, &report); err != nil {
				t.Fatal(err)
			}
			if len(report.Files["test_fixture.py"].Executed) == 0 {
				t.Fatalf("project omit hid executed source: %s", raw)
			}
			for _, name := range []string{"coverage.json", ".coverage"} {
				if _, err := os.Stat(filepath.Join(checkout, name)); !os.IsNotExist(err) {
					t.Fatalf("checkout artifact %s: %v", name, err)
				}
			}
		})
	}
}
