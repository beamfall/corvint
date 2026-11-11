// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/gitrun"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func shellFixture(t *testing.T, runner string) (RunnerPlugin, testrunner.InventoryKey, string, string) {
	t.Helper()
	key := testrunner.InventoryKey{Checkout: strings.Repeat("a", 40), ConfigDigest: strings.Repeat("b", 64), Runner: runner, OS: "linux", Arch: "amd64"}
	source, shell := "matrix.bats", ""
	if runner == "shellspec" {
		source, shell = "matrix_spec.sh", filepath.Join(t.TempDir(), "sh")
	}
	plugin, err := ShellSelectionPlugin(key, source, shell)
	if err != nil {
		t.Fatal(err)
	}
	return plugin, key, source, shell
}
func shellReport(t *testing.T, runner, source, native string, code int) string {
	t.Helper()
	raw, err := json.Marshal(testrunner.Input{Runner: runner, SourceFile: source, Stdout: []byte(native), ExitCode: code})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func shellPassing(runner string) string {
	if runner == "bats" {
		return "1..2\nok 1 first\nok 2 second\n"
	}
	return `<testsuites name="fixture" tests="2" errors="0" failures="0"><testsuite name="matrix_spec.sh" tests="2" errors="0" failures="0" skipped="0"><testcase classname="matrix_spec.sh" name="first"/><testcase classname="matrix_spec.sh" name="second"/></testsuite></testsuites>`
}
func shellInventory(t *testing.T, p RunnerPlugin, key testrunner.InventoryKey, source string) testrunner.RunnerInventory {
	t.Helper()
	in, err := p.Inventory(context.Background(), InventoryRequest{Key: key, Output: shellReport(t, key.Runner, source, shellPassing(key.Runner), 0)})
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// RSP-V0-001: bound shell instances declare native pieces but no qualification.
func TestRSPV0001ShellPieces(t *testing.T) {
	for _, runner := range []string{"bats", "shellspec"} {
		p, _, _, _ := shellFixture(t, runner)
		pieces := p.Pieces()
		if !pieces.Inventory.Present || !pieces.TestEvidence.Present || pieces.Qualification.Present || pieces.Qualification.Reason != "unqualified" {
			t.Fatalf("%s: %+v", runner, pieces)
		}
	}
}

// RSP-V0-001: a claimed qualification cannot override an absent harness piece.
func TestRSPV0001ShellUnqualified(t *testing.T) {
	for _, runner := range []string{"bats", "shellspec"} {
		p, key, source, _ := shellFixture(t, runner)
		in := shellInventory(t, p, key, source)
		plan := NewSelectionSession(p, key, true).Plan(in, []testrunner.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || len(plan.Tests) != 2 {
			t.Fatalf("%+v", plan)
		}
	}
}

// RSP-V0-002: native names are namespaced by runner and source; ordering is canonical.
func TestRSPV0002ShellInventory(t *testing.T) {
	for _, runner := range []string{"bats", "shellspec"} {
		t.Run(runner, func(t *testing.T) {
			p, key, source, _ := shellFixture(t, runner)
			a := shellInventory(t, p, key, source)
			b := shellInventory(t, p, key, source)
			ab, e := testrunner.InventoryBytes(a, key)
			if e != nil {
				t.Fatal(e)
			}
			bb, e := testrunner.InventoryBytes(b, key)
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(ab, bb) || len(a.Tests) != 2 {
				t.Fatalf("%s != %s", ab, bb)
			}
			for _, test := range a.Tests {
				if test.Path != source || !strings.HasPrefix(test.ID, runner+":"+source+"::") {
					t.Fatal(test)
				}
			}
		})
	}
}

// RSP-V0-002: missing, partial, malformed and capped discovery cannot narrow.
func TestRSPV0002ShellInventoryRefusals(t *testing.T) {
	for _, runner := range []string{"bats", "shellspec"} {
		t.Run(runner, func(t *testing.T) {
			p, key, source, _ := shellFixture(t, runner)
			good := shellReport(t, runner, source, shellPassing(runner), 0)
			cases := map[string]string{"missing exit": strings.Replace(good, `"exitCode":0,`, "", 1), "null exit": strings.Replace(good, `"exitCode":0`, `"exitCode":null`, 1), "empty": "", "truncated": good[:len(good)-1], "unknown": strings.TrimSuffix(good, "}") + `,"unknown":true}`, "duplicate": strings.TrimSuffix(good, "}") + `,"runner":"` + runner + `"}`, "cap": strings.Repeat(" ", testrunner.MaxReportBytes+1), "bad native": shellReport(t, runner, source, "malformed", 0), "wrong exit": shellReport(t, runner, source, shellPassing(runner), 2), "wrong source": shellReport(t, runner, "other.sh", shellPassing(runner), 0)}
			for _, flag := range []string{"overflow", "interrupted", "timedOut"} {
				cases[flag] = strings.Replace(good, `"`+flag+`":false`, `"`+flag+`":true`, 1)
			}
			for name, report := range cases {
				t.Run(name, func(t *testing.T) {
					session := NewSelectionSession(p, key, false)
					in := session.Inventory(context.Background(), report)
					if in.Complete || in.Reason == "" || !session.Plan(in, nil, nil).Full {
						t.Fatalf("%+v", in)
					}
				})
			}
		})
	}
	p, key, _, _ := shellFixture(t, "bats")
	key.ConfigDigest = strings.Repeat("c", 64)
	if in, err := p.Inventory(context.Background(), InventoryRequest{Key: key}); err == nil || in.Complete {
		t.Fatal("key mismatch admitted")
	}
}

// RSP-V0-003: reuse native Bats and ShellSpec reports, preserving failure and skip reasons.
func TestRSPV0003ShellNativeResults(t *testing.T) {
	raw, err := os.ReadFile("../platform/testdata/native-report-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]struct {
		Bytes  []byte `json:"bytesBase64"`
		Digest string `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, runner := range []string{"bats", "shellspec"} {
		p, _, source, _ := shellFixture(t, runner)
		file, code := "bats.tap", 1
		if runner == "shellspec" {
			file, code = "shellspec.xml", 101
		}
		fixture := fixtures[file]
		if testrunner.Digest(fixture.Bytes) != fixture.Digest {
			t.Fatal("fixture digest changed")
		}
		results, err := p.Parse(context.Background(), ParseRequest{Runner: runner, Report: shellReport(t, runner, source, string(fixture.Bytes), code)})
		if err != nil || len(results) != 3 {
			t.Fatalf("%s: %+v %v", runner, results, err)
		}
		counts := map[string]int{}
		for _, r := range results {
			counts[r.Outcome]++
			if r.Outcome == "skipped" && r.Reason == "" {
				t.Fatal("skip reason lost")
			}
		}
		if counts["passed"] != 1 || counts["failed"] != 1 || counts["skipped"] != 1 {
			t.Fatal(counts)
		}
	}
}

// RSP-V0-003: missing identities trigger FULL retry; uninventoried tests diverge.
func TestRSPV0003ShellDivergence(t *testing.T) {
	p, key, source, _ := shellFixture(t, "bats")
	in := shellInventory(t, p, key, source)
	session := NewSelectionSession(p, key, false)
	calls := 0
	run := session.Run(context.Background(), in, nil, nil, t.TempDir(), true, func(_ context.Context, cmd CommandResponse) (string, error) {
		calls++
		if calls == 1 {
			return shellReport(t, "bats", source, "1..2\nok 1 first\n", 0), nil
		}
		return shellReport(t, "bats", source, shellPassing("bats"), 0), nil
	})
	if calls != 2 || len(run.Attempts) != 2 || !run.NeedsRequalification || len(run.Attempts[0].Results) != 1 || run.Attempts[0].Results[0].Outcome != "passed" {
		t.Fatalf("%+v calls=%d", run, calls)
	}
	if !slices.Contains(run.Attempts[0].Reconciliation.Missing, in.Tests[1].ID) {
		t.Fatal("missing identity not retained")
	}
	extra := session.Run(context.Background(), in, nil, nil, t.TempDir(), false, func(context.Context, CommandResponse) (string, error) {
		return shellReport(t, "bats", source, "1..3\nok 1 first\nok 2 second\nok 3 extra\n", 0), nil
	})
	if extra.Succeeded || !extra.NeedsRequalification || len(extra.Attempts[0].Reconciliation.Unexpected) != 1 {
		t.Fatalf("%+v", extra)
	}
	if plan := session.Plan(in, nil, nil); !plan.Full || !strings.Contains(plan.Reason, "divergence") {
		t.Fatal(plan)
	}
}

// RSP-V0-003: reported failures survive malformed tails and a passing FULL retry.
func TestRSPV0003ShellRetryPreservesPartialFailure(t *testing.T) {
	for _, tail := range []string{"", "future-result 2 second\n"} {
		t.Run(tail, func(t *testing.T) {
			p, key, source, _ := shellFixture(t, "bats")
			in := shellInventory(t, p, key, source)
			session := NewSelectionSession(p, key, false)
			calls := 0
			run := session.Run(context.Background(), in, nil, nil, t.TempDir(), true, func(context.Context, CommandResponse) (string, error) {
				calls++
				if calls == 1 {
					return shellReport(t, "bats", source, "1..2\nnot ok 1 first\n"+tail, 1), nil
				}
				return shellReport(t, "bats", source, shellPassing("bats"), 0), nil
			})
			if calls != 2 || len(run.Attempts) != 2 || run.Succeeded || !run.NeedsRequalification {
				t.Fatalf("lost failed attempt: %+v calls=%d", run, calls)
			}
			first := run.Attempts[0]
			if len(first.Results) != 1 || first.Results[0].Outcome != "failed" || !slices.Contains(first.Reconciliation.Missing, in.Tests[1].ID) {
				t.Fatalf("partial failure/missing identity lost: %+v", first)
			}
		})
	}
}

// RSP-V0-003: unsupported terminal forms and skipped-without-reason never succeed.
func TestRSPV0003ShellTerminalRefusals(t *testing.T) {
	p, _, source, _ := shellFixture(t, "bats")
	for _, native := range []string{"1..1\nok 1 first # skip\n", "1..2\nok 1 first\nok 1 first\n", "1..1\nnot ok 1 first # skip why\n", "1..1\nok 1 first # TODO maybe\n"} {
		if _, err := p.Parse(context.Background(), ParseRequest{Runner: "bats", Report: shellReport(t, "bats", source, native, 0)}); err == nil {
			t.Fatalf("accepted %q", native)
		}
	}
}

// RSP-V0-004: exact exclusion is core-owned; commands widen to a reported superset.
func TestRSPV0004ShellSuperset(t *testing.T) {
	for _, runner := range []string{"bats", "shellspec"} {
		p, key, source, _ := shellFixture(t, runner)
		in := shellInventory(t, p, key, source)
		plan := testrunner.PlanRunSet(in, key, []testrunner.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
		if plan.Full || !slices.Equal(plan.Tests, []string{in.Tests[1].ID}) {
			t.Fatal(plan)
		}
		root := t.TempDir()
		full, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root})
		if err != nil {
			t.Fatal(err)
		}
		for _, set := range [][]string{plan.Tests, {}, {"unknown"}} {
			cmd, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root, Tests: set})
			if err != nil || !cmd.Superset || !slices.Equal(cmd.Argv, full.Argv) {
				t.Fatalf("%+v %v", cmd, err)
			}
		}
	}
}

// RSP-V0-004: stale/new tests remain selected, mandates survive, invalid requests refuse.
func TestRSPV0004ShellNoSubset(t *testing.T) {
	p, key, source, _ := shellFixture(t, "bats")
	in := shellInventory(t, p, key, source)
	plan := testrunner.PlanRunSet(in, key, []testrunner.Exclusion{{ID: in.Tests[0].ID, Fresh: false}}, nil)
	if len(plan.Tests) != 2 {
		t.Fatal(plan)
	}
	plan = testrunner.PlanRunSet(in, key, []testrunner.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, []string{in.Tests[0].ID})
	if len(plan.Tests) != 2 {
		t.Fatal(plan)
	}
	for _, r := range []CommandRequest{{Key: key, Purpose: "unknown", RunDir: t.TempDir()}, {Key: key, Purpose: "run", RunDir: "relative"}, {Purpose: "run", RunDir: t.TempDir()}, {Key: key, Purpose: "run", RunDir: t.TempDir(), Tests: make([]string, testrunner.MaxTests+1)}} {
		if _, err := p.Command(context.Background(), r); err == nil {
			t.Fatal("invalid command admitted")
		}
	}
}

// RSP-V0-005: native components remain absent and every test gets whole-tree file evidence.
func TestRSPV0005ShellFileEvidence(t *testing.T) {
	root, head, _ := scriptFixture(t)
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	for _, runner := range []string{"bats", "shellspec"} {
		_, key, source, shell := shellFixture(t, runner)
		key.Checkout = head
		p, err := ShellSelectionPlugin(key, source, shell)
		if err != nil {
			t.Fatal(err)
		}
		in := shellInventory(t, p, key, source)
		headers := shellHeaders(in, head)
		trace := scriptObservation()
		trace.Reads.Entries = []proctrace.Path{scriptPath("data")}
		trace.Execs.Entries = []proctrace.Exec{{Path: scriptPath("tool"), Digest: strings.Repeat("c", 64)}}
		records, err := ShellTraceRecords(context.Background(), in, headers, trace, repo)
		if err != nil || len(records) != 2 {
			t.Fatalf("%+v %v", records, err)
		}
		for _, record := range records {
			if record.Executed.State != testevidence.Absent || record.Regions.State != testevidence.Absent || record.Loaded.State != testevidence.Absent || record.Reads.State != testevidence.Present || len(record.Reads.Items) != 1 || record.Reads.Items[0].OID != scriptGit(t, root, "rev-parse", head+":data") || len(record.Execs.Items) != 1 || record.Execs.Items[0].Instrumented {
				t.Fatalf("%+v", record)
			}
		}
		evidence, err := p.Evidence(context.Background(), EvidenceRequest{Runner: runner, Artifacts: []string{shellReport(t, runner, source, shellPassing(runner), 0)}})
		if err != nil || len(evidence) != 2 {
			t.Fatalf("%+v %v", evidence, err)
		}
		for _, native := range evidence {
			var executed testevidence.Component[testevidence.ExecutedFile]
			if err := json.Unmarshal(native.Executed, &executed); err != nil || executed.State != testevidence.Absent || executed.Reason != "shell-no-line-coverage" {
				t.Fatal(string(native.Executed), err)
			}
		}
	}
}
func shellHeaders(in testrunner.RunnerInventory, head string) []testevidence.Record {
	headers := []testevidence.Record{}
	for _, test := range in.Tests {
		h := scriptHeader(head)
		h.Test = test.ID
		h.Runner.Plugin = in.Key.Runner
		h.Runner.ConfigurationDigest = in.Key.ConfigDigest
		headers = append(headers, h)
	}
	return headers
}

// RSP-V0-005: incomplete/unknown/capped traces and missing attribution cannot narrow.
func TestRSPV0005ShellEvidenceRefusals(t *testing.T) {
	root, head, _ := scriptFixture(t)
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	_, key, source, _ := shellFixture(t, "bats")
	key.Checkout = head
	p, err := ShellSelectionPlugin(key, source, "")
	if err != nil {
		t.Fatal(err)
	}
	in := shellInventory(t, p, key, source)
	headers := shellHeaders(in, head)
	for _, state := range []proctrace.State{proctrace.Incomplete, proctrace.Absent, "unknown"} {
		trace := scriptObservation()
		trace.Reads.State = state
		trace.Reads.Reasons = []string{"fixture-cap"}
		records, err := ShellTraceRecords(context.Background(), in, headers, trace, repo)
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range records {
			if record.Reads.State == testevidence.Present || record.Reads.Reason == "" {
				t.Fatal(record.Reads)
			}
		}
	}
	if _, err := ShellTraceRecords(context.Background(), in, headers[:1], scriptObservation(), repo); err == nil {
		t.Fatal("partial attribution admitted")
	}
	headers[1].Test = headers[0].Test
	if _, err := ShellTraceRecords(context.Background(), in, headers, scriptObservation(), repo); err == nil {
		t.Fatal("duplicate attribution admitted")
	}
	for _, artifacts := range [][]string{nil, {"coverage"}, {"a", "b"}} {
		if _, err := p.Evidence(context.Background(), EvidenceRequest{Runner: "bats", Artifacts: artifacts}); err == nil {
			t.Fatal("invalid artifact admitted")
		}
	}
}

type shellBackend struct {
	result proctrace.Result
	stdout string
	calls  int
}

func (b *shellBackend) Trace(_ context.Context, q proctrace.Request) proctrace.Result {
	b.calls++
	_, _ = io.WriteString(q.Stdout, b.stdout)
	return b.result
}

// RSP-V0-040: a complete captured shell run reaches inventory, reconciliation and evidence, still FULL.
func TestRSPV0040ShellTraceSession(t *testing.T) {
	for _, runner := range []string{"bats", "shellspec"} {
		p, key, source, shell := shellFixture(t, runner)
		root := t.TempDir()
		cmd, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "inventory", RunDir: root})
		if err != nil {
			t.Fatal(err)
		}
		backend := &shellBackend{result: scriptObservation(), stdout: shellPassing(runner)}
		q := proctrace.Request{Executable: filepath.Join(root, "pinned-runner"), Dir: root, Args: cmd.Argv[1:], Repositories: []proctrace.Repository{{ID: "main", Root: root}}, EventCap: 100, PathCap: 100}
		_, report, err := TraceShellCommand(context.Background(), key, source, shell, cmd, q, backend)
		if err != nil {
			t.Fatal(err)
		}
		session := NewSelectionSession(p, key, false)
		in := session.Inventory(context.Background(), report)
		if !in.Complete {
			t.Fatal(in)
		}
		run := session.Run(context.Background(), in, nil, nil, root, false, func(context.Context, CommandResponse) (string, error) { return report, nil })
		if !run.Succeeded || !run.Attempts[0].Plan.Full || !strings.Contains(run.Attempts[0].Plan.Reason, "unqualified") || backend.calls != 1 {
			t.Fatalf("%+v", run)
		}
	}
}

// RSP-V0-040: refused processes, output caps, mismatched commands and absent qualification never promote.
func TestRSPV0040ShellTraceRefusals(t *testing.T) {
	p, key, source, shell := shellFixture(t, "bats")
	root := t.TempDir()
	cmd, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root})
	if err != nil {
		t.Fatal(err)
	}
	q := proctrace.Request{Executable: filepath.Join(root, "pinned-runner"), Dir: root, Args: cmd.Argv[1:], Repositories: []proctrace.Repository{{ID: "main", Root: root}}, EventCap: 100, PathCap: 100}
	for _, backend := range []*shellBackend{{result: proctrace.Result{}, stdout: shellPassing("bats")}, {result: scriptObservation(), stdout: strings.Repeat("x", testrunner.MaxReportBytes/4+1)}} {
		_, report, err := TraceShellCommand(context.Background(), key, source, shell, cmd, q, backend)
		if err == nil {
			t.Fatal("invalid execution accepted")
		}
		if in := NewSelectionSession(p, key, false).Inventory(context.Background(), report); in.Complete {
			t.Fatal(in)
		}
	}
	backend := &shellBackend{result: scriptObservation()}
	bad := cmd
	bad.Argv = append(append([]string(nil), cmd.Argv...), "other.bats")
	if _, _, err := TraceShellCommand(context.Background(), key, source, shell, bad, q, backend); err == nil || backend.calls != 0 {
		t.Fatal("mismatched command executed")
	}
}

// RSP-V0-002: source/configuration cannot drift or escape the bound repository.
func TestRSPV0002ShellSourceRefusals(t *testing.T) {
	_, key, _, _ := shellFixture(t, "bats")
	for _, source := range []string{"../escape.bats", "/outside.bats", "-option.bats", "file.sh", "dir\\test.bats", "./test.bats"} {
		if _, err := ShellSelectionPlugin(key, source, ""); err == nil {
			t.Fatal(source)
		}
	}
	key.Checkout = "HEAD"
	if _, err := ShellSelectionPlugin(key, "test.bats", ""); err == nil {
		t.Fatal("movable checkout")
	}
}

func TestRSPV0002ShellGlobSourceUnresolvable(t *testing.T) {
	for _, runner := range []string{"bats", "shellspec"} {
		_, key, source, shell := shellFixture(t, runner)
		for _, glob := range []string{"*", "?", "[ab]", "["} {
			plugin, err := ShellSelectionPlugin(key, glob+source, shell)
			if err == nil || err.Error() != "shell-source-glob-unresolvable" || plugin != nil {
				t.Fatalf("%s %s: plugin=%v err=%v", runner, glob, plugin, err)
			}
			plan := NewSelectionSession(plugin, key, true).Plan(testrunner.RunnerInventory{Key: key, Reason: err.Error()}, nil, nil)
			if !plan.Full || plan.Reason == "" {
				t.Fatalf("unresolvable source narrowed: %+v", plan)
			}
		}
	}
}

// RSP-V0-003: invalid terminals never hide valid failures later in a report.
func TestRSPV0003ShellReasonlessSkipRetainsLaterFailure(t *testing.T) {
	for _, tail := range []string{"", "future-result 3 unknown\n"} {
		t.Run(tail, func(t *testing.T) {
			p, key, source, _ := shellFixture(t, "bats")
			in := shellInventory(t, p, key, source)
			broken := shellReport(t, "bats", source, "1..2\nok 1 first # skip\nnot ok 2 second\n"+tail, 1)
			results, err := p.Parse(context.Background(), ParseRequest{Runner: "bats", Report: broken})
			if err == nil || err.Error() != "shell-skip-reason-missing" || len(results) != 1 || results[0].ID != in.Tests[1].ID || results[0].Outcome != "failed" {
				t.Fatalf("invalid skip hid failure or certified report: %+v %v", results, err)
			}
			if tail != "" {
				unknown := shellReport(t, "bats", source, "1..2\nok 1 first\nnot ok 2 second\n"+tail, 1)
				terminals, unknownErr := p.Parse(context.Background(), ParseRequest{Runner: "bats", Report: unknown})
				if unknownErr == nil || len(terminals) != 2 || terminals[1].Outcome != "failed" {
					t.Fatalf("unknown form did not fail closed: %+v %v", terminals, unknownErr)
				}
			}
			calls := 0
			run := NewSelectionSession(p, key, false).Run(context.Background(), in, nil, nil, t.TempDir(), true, func(context.Context, CommandResponse) (string, error) {
				calls++
				if calls == 1 {
					return broken, nil
				}
				return shellReport(t, "bats", source, shellPassing("bats"), 0), nil
			})
			if calls != 2 || len(run.Attempts) != 2 || run.Succeeded || !run.NeedsRequalification || !run.Attempts[1].Plan.Full || !run.Attempts[1].Reconciliation.Succeeded {
				t.Fatalf("passing FULL retry erased later failure: %+v", run)
			}
			if !slices.Contains(run.Attempts[0].Reconciliation.Missing, in.Tests[0].ID) {
				t.Fatal("invalid skip certified completion")
			}
		})
	}
}
