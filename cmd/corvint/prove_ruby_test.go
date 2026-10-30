package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate/minirspec"
)

// proveRubyChangeRepository commits the Rails-shaped RSpec fixture, then a
// second commit that changes app/api_clients/foo_client.rb, so `prove --base`
// judges a committed Ruby range.
func proveRubyChangeRepository(t *testing.T) (root, base string) {
	t.Helper()
	root = rspecAdviceRepository(t, true)
	affectedGit(t, root, "stash", "-q")
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	affectedGit(t, root, "stash", "pop", "-q")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "change FooClient")
	return root, base
}

// TestFPKV0071RubyRowsUnderMutateNeverStaySilent pins FPK-V0-071 on the
// FPK-V0-057 fixture: under --mutate an RSpec affected-test row claiming a
// changed .rb source is judged by the Ruby engine (falsifier
// test-kills-mutant, as without --mutate). This Rails-shaped fixture's Gemfile
// needs gems the falsifier never installs, so the row stays NOT_RUN with a
// typed UNSUPPORTED witness and its mutation record names the stage; without
// --mutate it carries neither.
func TestFPKV0071RubyRowsUnderMutateNeverStaySilent(t *testing.T) {
	t.Parallel()
	root, base := proveRubyChangeRepository(t)
	before := treeDigest(t, root)
	rubyRow := func(receipt map[string]any) map[string]any {
		t.Helper()
		for _, row := range proofRows(t, receipt) {
			if stringAt(row, "kind") == kindAffectedTest && stringAt(row, "path") == "spec/api_clients/foo_client_spec.rb" {
				return row
			}
		}
		t.Fatalf("no RSpec affected-test row: %v", receipt["proof"])
		return nil
	}
	receipt, _, stderr, code := runProveCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := rubyRow(receipt)
	if row["falsifier"] != falsifierMutant || row["falsified"] != falsifiedNotRun || row["witness"] != nil || row["detail"] != nil || row["mutation"] != nil {
		t.Fatalf("without --mutate: %v", row)
	}
	receipt, _, stderr, code = runProveCLI(t, root, "--base", base, "--mutate")
	if code != 0 {
		t.Fatalf("--mutate exit %d: %s", code, stderr)
	}
	row = rubyRow(receipt)
	witness, _ := row["witness"].(map[string]any)
	mutation, _ := row["mutation"].(map[string]any)
	if row["falsifier"] != falsifierMutant || row["falsified"] != falsifiedNotRun || stringAt(row, "detail") == "" {
		t.Fatalf("--mutate row: %v", row)
	}
	stages := map[string]string{rubyRunnerUnavailable: "runner", rubyBaselineUnusable: "baseline"}
	reason, _ := witness["reason"].(string)
	if len(witness) != 2 || witness["status"] != unsupportedWitnessStatus || stages[reason] == "" {
		t.Fatalf("--mutate witness: %v", witness)
	}
	if mutation["engine"] != "rbmutate" || mutation["stage"] != stages[reason] || mutation["mutants"] == nil {
		t.Fatalf("--mutate mutation record: %v", mutation)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --mutate wrote the repository")
	}
}

// proveScoringBase is the issue #1321 model before the change: the bands sit
// one higher than fixtureScoringChanged's.
const proveScoringBase = "module Scoring\n" +
	"  def self.lowest_count(count)\n" +
	"    if count >= 9\n" +
	"      3\n" +
	"    elsif count >= 5\n" +
	"      2\n" +
	"    else\n" +
	"      1\n" +
	"    end\n" +
	"  end\n" +
	"\n" +
	"  def self.average(scores)\n" +
	"    picked = scores.sort.first(lowest_count(scores.size))\n" +
	"    picked.sum / picked.size\n" +
	"  end\n" +
	"end\n"

// proveScoringWeakSpec seeds the lowest values identically in every band.
const proveScoringWeakSpec = "require_relative '../../app/models/scoring'\n\n" +
	"describe 'Scoring.average' do\n" +
	"  it 'averages a small set' do\n" +
	"    expect(Scoring.average([5, 9])).to eq(5)\n" +
	"  end\n" +
	"  it 'averages a mid set' do\n" +
	"    expect(Scoring.average([5, 5, 9, 9])).to eq(5)\n" +
	"  end\n" +
	"  it 'averages a large set' do\n" +
	"    expect(Scoring.average([5, 5, 5, 9, 9, 9, 9, 9])).to eq(5)\n" +
	"  end\n" +
	"end\n"

// TestFPKV0070RSpecRowCarriesPerMutantVerdictsAndReceipts pins FPK-V0-068 to
// FPK-V0-071 end to end: `prove --base B --mutate` on a committed change to
// two band edges mutates only the changed lines, runs the cited spec file
// against every mutant through `rspec` on PATH (the minirspec double here),
// and retains each mutant's verdict and receipt and the baseline receipt on
// the row. The edge mutants survive the weak spec; the row passes because a
// mutant died, and its witness says no replayable witness exists for Ruby.
func TestFPKV0070RSpecRowCarriesPerMutantVerdictsAndReceipts(t *testing.T) {
	t.Parallel()
	requireNestableSandbox(t)
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby is not available")
	}
	shims := t.TempDir()
	if _, err := minirspec.Install(shims); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for name, body := range map[string]string{"app/models/scoring.rb": proveScoringBase, "spec/models/scoring_spec.rb": proveScoringWeakSpec} {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	changed := strings.Replace(strings.Replace(proveScoringBase, ">= 9", ">= 8", 1), ">= 5", ">= 4", 1)
	if err := os.WriteFile(filepath.Join(root, "app", "models", "scoring.rb"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "lower the bands")
	before := treeDigest(t, root)
	environment := []string{"PATH=" + shims + string(os.PathListSeparator) + filepath.Dir(ruby) + string(os.PathListSeparator) + "/usr/bin:/bin"}
	receipt, _, stderr, code := runProveCLIEnvironment(t, environment, root, "--base", base, "--mutate")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var row map[string]any
	for _, candidate := range proofRows(t, receipt) {
		if stringAt(candidate, "kind") == kindAffectedTest && stringAt(candidate, "path") == "spec/models/scoring_spec.rb" {
			row = candidate
		}
	}
	if row == nil {
		t.Fatalf("no RSpec row: %v", receipt["proof"])
	}
	witness, _ := row["witness"].(map[string]any)
	if row["falsifier"] != falsifierMutant || row["falsified"] != falsifiedPass || witness["status"] != "NOT_PRODUCED" || witness["reason"] != rubyWitnessNotProduced {
		t.Fatalf("row: %v", row)
	}
	if detail := stringAt(row, "detail"); !strings.HasPrefix(detail, "killed 2 of 8 mutants; 6 survived spec/models/scoring_spec.rb") {
		t.Fatalf("detail: %q", detail)
	}
	mutation, _ := row["mutation"].(map[string]any)
	baseline, _ := mutation["baseline"].(map[string]any)
	mutants, _ := mutation["mutants"].([]any)
	if mutation["engine"] != "rbmutate" || mutation["runner"] != "rspec" || mutation["stage"] != "mutants" || baseline["examples"] != float64(3) || len(mutants) != 8 {
		t.Fatalf("mutation record: %v", mutation)
	}
	survivors := map[string]bool{}
	for _, entry := range mutants {
		mutant, _ := entry.(map[string]any)
		receiptOf, _ := mutant["receipt"].(map[string]any)
		if line := mutant["line"]; line != float64(3) && line != float64(5) {
			t.Fatalf("a mutant outside the changed lines: %v", mutant)
		}
		if receiptOf["examples"] != float64(3) || stringAt(receiptOf, "source_sha256") == "" {
			t.Fatalf("mutant receipt: %v", mutant)
		}
		if mutant["outcome"] == "survived" {
			survivors[stringAt(mutant, "operator")+" "+stringAt(mutant, "replacement")] = true
		}
	}
	for _, want := range []string{"relational-boundary >", "constant-decrement 7", "constant-increment 9", "constant-decrement 3", "constant-increment 5"} {
		if !survivors[want] {
			t.Errorf("survivor %q missing: %v", want, survivors)
		}
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --mutate wrote the repository")
	}
}

// TestFPKV0071RubyWitnessNamesWhyNoReplayableWitnessExists pins FPK-V0-071's
// mapping from the engine's report onto the row's verdict and witness.
func TestFPKV0071RubyWitnessNamesWhyNoReplayableWitnessExists(t *testing.T) {
	t.Parallel()
	cases := []struct {
		report         rbmutate.Report
		falsified      string
		status, reason string
	}{
		{rbmutate.Report{Verdict: mutate.Killed, Stage: rbmutate.StageMutants, Mutants: 2, Killed: 1, Survived: 1}, falsifiedPass, "NOT_PRODUCED", rubyWitnessNotProduced},
		{rbmutate.Report{Verdict: mutate.Survived, Stage: rbmutate.StageMutants, Mutants: 2, Survived: 2}, falsifiedFail, "NOT_PRODUCED", rubyWitnessNotProduced},
		{rbmutate.Report{Verdict: mutate.NoMutants, Stage: rbmutate.StageMutants}, falsifiedNotRun, "NOT_PRODUCED", "no-mutant-in-range"},
		{rbmutate.Report{Verdict: mutate.NoMutants, Stage: rbmutate.StageMutants, Mutants: 2, Unloadable: 2}, falsifiedNotRun, "NOT_PRODUCED", rubyWitnessNotProduced},
		{rbmutate.Report{Verdict: mutate.BudgetExceeded, Stage: rbmutate.StageMutants}, falsifiedNotRun, "NOT_PRODUCED", "mutation-budget-exhausted"},
		{rbmutate.Report{Verdict: mutate.Unsupported, Stage: rbmutate.StageRunner}, falsifiedNotRun, unsupportedWitnessStatus, rubyRunnerUnavailable},
		{rbmutate.Report{Verdict: mutate.Unsupported, Stage: rbmutate.StageBaseline}, falsifiedNotRun, unsupportedWitnessStatus, rubyBaselineUnusable},
		{rbmutate.Report{Verdict: mutate.Unsupported, Stage: rbmutate.StageInput}, falsifiedNotRun, "NOT_PRODUCED", "mutation-input-unavailable"},
	}
	for _, item := range cases {
		judged := rubyVerdict(item.report)
		if mutationFalsified(item.report.Verdict) != item.falsified || judged.witness.Status != item.status || judged.witness.Reason != item.reason {
			t.Errorf("%+v: got %s %+v", item.report, mutationFalsified(item.report.Verdict), judged.witness)
		}
		if judged.mutation == nil || judged.mutation.Mutants == nil || judged.mutation.Engine != "rbmutate" {
			t.Errorf("%+v: mutation record %+v", item.report, judged.mutation)
		}
	}
}

// TestFPKV0057UnsupportedLanguageLeavesEngineAndCrossLanguageRowsAlone pins
// FPK-V0-057's boundary: Go, Python and RSpec claims, a changed path that is itself a
// selected test, a claim across languages, a row that already has a falsifier,
// and a packet row are never marked; a same-language claim without an engine
// (a minitest or TypeScript claim) names its provider.
func TestFPKV0057UnsupportedLanguageLeavesEngineAndCrossLanguageRowsAlone(t *testing.T) {
	t.Parallel()
	providers := affectedLanguages()
	tests := map[string]struct{}{"spec/a_spec.rb": {}, "spec/b_spec.rb": {}, "test/a_test.rb": {}}
	cases := []struct {
		test, changed, want string
	}{
		{"spec/a_spec.rb", "lib/a.rb", ""},
		{"test/a_test.rb", "lib/a.rb", "ruby"},
		{"src/a.test.ts", "src/a.ts", "typescript"},
		{"spec/a_spec.rb", "spec/b_spec.rb", ""},
		{"spec/a_spec.rb", "spec/a_spec.rb", ""},
		{"spec/a_spec.rb", "config/a.yml", ""},
		{"pkg/calc/calc_test.go", "pkg/calc/calc_test.go", ""},
		{"pkg/calc/calc_test.go", "pkg/calc/testdata/fixture.go", ""},
		{"tests/helpers.py", "calc.py", ""},
	}
	for _, item := range cases {
		row := affectedRow(item.test, item.changed)
		if got := unsupportedMutationLanguage(row, item.changed, tests, providers); got != item.want {
			t.Errorf("%s claiming %s: got %q, want %q", item.test, item.changed, got, item.want)
		}
	}
	mutant := affectedRow("pkg/calc/calc_test.go", "pkg/calc/calc.go")
	if got := unsupportedMutationLanguage(mutant, "pkg/calc/calc.go", tests, providers); got != "" {
		t.Errorf("a Go mutation row is marked %q", got)
	}
	packetRow := proveRow{Kind: "test", Falsifier: falsifierNone, Path: "spec/a_spec.rb"}
	if got := unsupportedMutationLanguage(packetRow, "lib/a.rb", tests, providers); got != "" {
		t.Errorf("a packet row is marked %q", got)
	}
}
