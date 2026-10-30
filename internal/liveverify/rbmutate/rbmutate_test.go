package rbmutate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate/minirspec"
	"github.com/Beamfall/corvint/internal/testplatform"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// fixtureScoring is the issue #1321 shape: average the lowest N scores,
// where N is banded by how many scores there are.
const fixtureScoring = `# Averages the lowest N scores, where N depends on how many there are.
module Scoring
  def self.lowest_count(count)
    if count >= 8
      3
    elsif count >= 4
      2
    else
      1
    end
  end

  def self.average(scores)
    picked = scores.sort.first(lowest_count(scores.size))
    picked.sum / picked.size
  end
end
`

// fixtureWeakSpec seeds the lowest values identically in every band, so
// moving a band edge or a count by one changes nothing it can see.
const fixtureWeakSpec = `require_relative '../../app/models/scoring'

describe 'Scoring.average' do
  it 'averages a small set' do
    expect(Scoring.average([5, 9])).to eq(5)
  end
  it 'averages a mid set' do
    expect(Scoring.average([5, 5, 9, 9])).to eq(5)
  end
  it 'averages a large set' do
    expect(Scoring.average([5, 5, 5, 9, 9, 9, 9, 9])).to eq(5)
  end
end
`

// fixtureStrongSpec asserts distinct values at every band edge.
const fixtureStrongSpec = `require_relative '../../app/models/scoring'

describe 'Scoring.lowest_count' do
  it 'bands at three' do
    expect(Scoring.lowest_count(3)).to eq(1)
  end
  it 'bands at four' do
    expect(Scoring.lowest_count(4)).to eq(2)
  end
  it 'bands at seven' do
    expect(Scoring.lowest_count(7)).to eq(2)
  end
  it 'bands at eight' do
    expect(Scoring.lowest_count(8)).to eq(3)
  end
end
`

const fixtureRedSpec = `require_relative '../../app/models/scoring'

describe 'Scoring' do
  it 'is wrong' do
    expect(Scoring.lowest_count(8)).to eq(4)
  end
end
`

func requireSandbox(t *testing.T) {
	t.Helper()
	name, err := mutate.HostSandbox()
	if err != nil {
		t.Skipf("no sandbox on this host: %v", err)
	}
	probes := map[string][]string{
		"sandbox-exec": {"/usr/bin/sandbox-exec", "-p", "(version 1)(allow default)", "/usr/bin/true"},
		"bwrap":        {"bwrap", "--ro-bind", "/", "/", "--unshare-net", "--", "true"},
	}
	probe := probes[name]
	if output, err := exec.Command(probe[0], probe[1:]...).CombinedOutput(); err != nil {
		t.Skipf("host sandbox %s cannot start here: %v: %s", name, err, strings.TrimSpace(string(output)))
	}
}

// requireRuby returns the minirspec argv: a host ruby running the test double.
func requireRuby(t *testing.T) []string {
	t.Helper()
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby is not available")
	}
	shim, err := minirspec.Install(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return []string{ruby, shim}
}

func gitExecutable(t *testing.T) string {
	t.Helper()
	found, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	return found
}

func newFixture(t *testing.T, git string, files map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, git, root, "init", "--quiet")
	runGit(t, git, root, "add", "-A")
	runGit(t, git, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "fixture")
	return root, strings.TrimSpace(runGit(t, git, root, "rev-parse", "HEAD"))
}

var runGit = testsupport.RunGit

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	entries := make([]string, 0, 8)
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, name)
		if entry.IsDir() {
			if relative == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		entries = append(entries, filepath.ToSlash(relative)+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	digest := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(digest[:])
}

func scoringRequest(t *testing.T, spec string, lines []mutate.LineSpan) (Request, string) {
	t.Helper()
	requireSandbox(t)
	rspec := requireRuby(t)
	git := gitExecutable(t)
	root, revision := newFixture(t, git, map[string]string{"app/models/scoring.rb": fixtureScoring, "spec/models/scoring_spec.rb": spec})
	return Request{
		Root: root, Git: git, Revision: revision, ChangedPath: "app/models/scoring.rb", TestPath: "spec/models/scoring_spec.rb",
		Lines: lines, Budget: 4 * time.Minute, RSpec: rspec,
	}, root
}

// bandLines confines mutation to lowest_count's table (lines 4 to 9).
var bandLines = []mutate.LineSpan{{Start: 4, End: 9}}

func outcomes(report Report) map[string]Outcome {
	byMutant := map[string]Outcome{}
	for _, result := range report.Results {
		byMutant[result.Operator+" "+result.Original+"->"+result.Replacement+" @"+strconv.Itoa(result.Line)] = result.Outcome
	}
	return byMutant
}

// weakSpecOutcomes is every band mutant's verdict under fixtureWeakSpec: the
// count-up and negated-band mutants die, every edge and count-down mutant
// above the first band survives.
var weakSpecOutcomes = map[string]Outcome{
	"negate-condition count >= 8->!(count >= 8) @4": OutcomeKilled,
	"negate-condition count >= 4->!(count >= 4) @6": OutcomeKilled,
	"constant-increment 3->4 @5":                    OutcomeKilled,
	"constant-increment 2->3 @7":                    OutcomeKilled,
	"constant-decrement 1->0 @9":                    OutcomeKilled,
	"constant-increment 1->2 @9":                    OutcomeKilled,
	"relational-boundary >=->> @4":                  OutcomeSurvived,
	"constant-decrement 8->7 @4":                    OutcomeSurvived,
	"constant-increment 8->9 @4":                    OutcomeSurvived,
	"constant-decrement 3->2 @5":                    OutcomeSurvived,
	"relational-boundary >=->> @6":                  OutcomeSurvived,
	"constant-decrement 4->3 @6":                    OutcomeSurvived,
	"constant-increment 4->5 @6":                    OutcomeSurvived,
	"constant-decrement 2->1 @7":                    OutcomeSurvived,
}

// TestFPKV0069WeakSpecLeavesBandMutantsSurviving pins FPK-V0-069 and
// FPK-V0-070 on the issue #1321 shape: identical lowest values let the band
// edge and count mutants survive while the count-up mutants die, every
// mutant is run, and each carries a receipt; the root is untouched.
func TestFPKV0069WeakSpecLeavesBandMutantsSurviving(t *testing.T) {
	request, root := scoringRequest(t, fixtureWeakSpec, bandLines)
	request.MaxMutants = 14
	before := treeDigest(t, root)
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != mutate.Killed || report.Stage != StageMutants || report.Planned != 14 || report.Mutants != 14 || report.Runner != "ruby rspec" {
		t.Fatalf("report: %+v", report)
	}
	if report.Killed != 6 || report.Survived != 8 || len(report.Results) != 14 {
		t.Fatalf("every mutant must be run: %+v", report)
	}
	got := outcomes(report)
	if len(got) != len(weakSpecOutcomes) {
		t.Errorf("mutants: %v", got)
	}
	for key, outcome := range weakSpecOutcomes {
		if got[key] != outcome {
			t.Errorf("%s: got %q, want %q (all %v)", key, got[key], outcome, got)
		}
	}
	if report.Baseline == nil || report.Baseline.Examples != 3 || report.Baseline.Failures != 0 || report.Baseline.SourceSHA256 == "" {
		t.Fatalf("baseline receipt: %+v", report.Baseline)
	}
	for _, result := range report.Results {
		if result.Receipt.SourceSHA256 == report.Baseline.SourceSHA256 || result.Receipt.Examples != 3 {
			t.Fatalf("mutant receipt: %+v", result)
		}
		if result.Outcome == OutcomeKilled && (result.Receipt.Failures == 0 || len(result.Receipt.FailedExamples) == 0) {
			t.Fatalf("a kill without a failed example: %+v", result)
		}
	}
	if !strings.HasPrefix(report.Detail, "killed 6 of 14 mutants; 8 survived spec/models/scoring_spec.rb") {
		t.Fatalf("detail: %q", report.Detail)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("the run wrote the repository")
	}
}

// TestFPKV0069SelectionBoundIsDisclosed: past MaxMutants the first sites in
// source order run and the detail names the bound.
func TestFPKV0069SelectionBoundIsDisclosed(t *testing.T) {
	request, _ := scoringRequest(t, fixtureWeakSpec, bandLines)
	request.MaxMutants = 3
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Planned != 14 || report.Mutants != 3 || len(report.Results) != 3 || !strings.HasSuffix(report.Detail, "; 3 of 14 planned sites selected") {
		t.Fatalf("report: %+v", report)
	}
}

// TestFPKV0069StrongSpecKillsBandMutants: distinct expectations at every band
// edge kill the edge mutants the weak spec let through.
func TestFPKV0069StrongSpecKillsBandMutants(t *testing.T) {
	request, _ := scoringRequest(t, fixtureStrongSpec, []mutate.LineSpan{{Start: 4, End: 4}, {Start: 6, End: 6}})
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != mutate.Killed || report.Survived != 0 || report.Killed != 8 {
		t.Fatalf("report: %+v %v", report, outcomes(report))
	}
}

// TestFPKV0069UnloadableMutantIsNeverCredited: a mutant that no longer
// parses is an error outside examples, never a kill.
func TestFPKV0069UnloadableMutantIsNeverCredited(t *testing.T) {
	requireRuby(t)
	run := classifyRun(exitError(t, 1), false, "", []byte(`{"summary":{"example_count":0,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":1},"examples":[]}`), true, "d")
	if run.outcome != runUnloadable || run.receipt.ErrorsOutsideExamples != 1 || run.receipt.ExitStatus != 1 {
		t.Fatalf("run: %+v", run)
	}
	if mutantOutcome(run.outcome) != OutcomeUnloadable {
		t.Fatal("an unloadable run was credited")
	}
}

func exitError(t *testing.T, status int) error {
	t.Helper()
	testplatform.SkipPOSIXShell(t, "the exit status comes from /bin/sh -c \"exit N\"")
	err := exec.Command("/bin/sh", "-c", "exit "+strconv.Itoa(status)).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("no exit error: %v", err)
	}
	return err
}

// TestFPKV0069ClassifyRunReadsTheRSpecReport pins the run classification:
// RSpec's own report decides, the exit status confirms.
func TestFPKV0069ClassifyRunReadsTheRSpecReport(t *testing.T) {
	report := func(examples, failures, errors int, ids ...string) []byte {
		body := `{"summary":{"example_count":` + strconv.Itoa(examples) + `,"failure_count":` + strconv.Itoa(failures) + `,"pending_count":0,"errors_outside_of_examples_count":` + strconv.Itoa(errors) + `},"examples":[`
		for index, id := range ids {
			if index > 0 {
				body += ","
			}
			body += `{"id":"` + id + `","status":"failed"}`
		}
		return []byte(body + "]}")
	}
	// exit is the subprocess exit status a case needs (0: none). Each exit
	// error is built inside its own subtest, so on a host without a POSIX
	// shell only those subtests skip and the shell-independent rows still run.
	cases := []struct {
		name       string
		exit       int
		err        error
		timedOut   bool
		content    []byte
		haveReport bool
		want       runOutcome
	}{
		{"passed", 0, nil, false, report(3, 0, 0), true, runPassed},
		{"failed", 1, nil, false, report(3, 1, 0, "./b[1:2]", "./a[1:1]"), true, runFailed},
		{"no examples", 0, nil, false, report(0, 0, 0), true, runUnloadable},
		{"load error", 1, nil, false, report(0, 0, 1), true, runUnloadable},
		{"no report", 1, nil, false, nil, false, runUnloadable},
		{"non-zero all passed", 2, nil, false, report(3, 0, 0), true, runUnloadable},
		{"script error in an example", 1, nil, false, []byte(`{"summary":{"example_count":2,"failure_count":1,"pending_count":0,"errors_outside_of_examples_count":0},"examples":[{"id":"./a[1:1]","status":"failed","exception":{"class":"SyntaxError"}}]}`), true, runUnloadable},
		{"behavioural failure", 1, nil, false, []byte(`{"summary":{"example_count":2,"failure_count":1,"pending_count":0,"errors_outside_of_examples_count":0},"examples":[{"id":"./a[1:1]","status":"failed","exception":{"class":"RSpec::Expectations::ExpectationNotMetError"}}]}`), true, runFailed},
		{"timed out", 0, errors.New("killed"), true, nil, false, runTimedOut},
		{"no exit status", 0, errors.New("sandbox did not start"), false, nil, false, runBroken},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			err := item.err
			if item.exit != 0 {
				err = exitError(t, item.exit)
			}
			run := classifyRun(err, item.timedOut, "", item.content, item.haveReport, "d")
			if run.outcome != item.want {
				t.Errorf("got %v, want %v", run.outcome, item.want)
			}
		})
	}
	t.Run("failed examples sorted", func(t *testing.T) {
		failed := classifyRun(exitError(t, 1), false, "", report(3, 2, 0, "./b[1:2]", "./a[1:1]"), true, "d")
		if strings.Join(failed.receipt.FailedExamples, ",") != "./a[1:1],./b[1:2]" {
			t.Fatalf("failed examples are not sorted: %v", failed.receipt.FailedExamples)
		}
	})
}

// TestFPKV0069BaselineMustPass pins the baseline gate: a red spec file is
// Unsupported at the baseline stage with its receipt, before any mutant.
func TestFPKV0069BaselineMustPass(t *testing.T) {
	request, _ := scoringRequest(t, fixtureRedSpec, nil)
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != mutate.Unsupported || report.Stage != StageBaseline || report.Detail != "baseline examples fail before mutation: spec/models/scoring_spec.rb" {
		t.Fatalf("report: %+v", report)
	}
	if report.Baseline == nil || report.Baseline.Failures != 1 || len(report.Results) != 0 {
		t.Fatalf("baseline receipt: %+v", report)
	}
}

// fakeCopy is a sandboxed export whose runs all fail to start.
type fakeCopy struct {
	dir, unsandboxed string
}

func (copy fakeCopy) Dir() string         { return copy.dir }
func (copy fakeCopy) Scratch() string     { return filepath.Join(copy.dir, ".scratch") }
func (copy fakeCopy) Unsandboxed() string { return copy.unsandboxed }
func (fakeCopy) Run(context.Context, time.Duration, string, []string, ...string) (string, bool, error) {
	return "", false, errors.New("cannot start")
}

// TestFPKV0069RunnerUnavailableIsUnsupportedAtTheRunnerStage pins the
// runner gate: no sandbox, no bundle beside a Gemfile, no rspec, or an RSpec
// that does not start is Unsupported at the runner stage with a fixed detail.
func TestFPKV0069RunnerUnavailableIsUnsupportedAtTheRunnerStage(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"lib/a.rb": "A = 1\n", "spec/a_spec.rb": "describe('a') { }\n"} {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	request := Request{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb", RSpec: []string{"/no/such/rspec"}}
	report, err := Judge(context.Background(), fakeCopy{dir: dir, unsandboxed: "no sandbox"}, request)
	if err != nil || report.Verdict != mutate.Unsupported || report.Stage != StageRunner || report.Detail != "no sandbox" {
		t.Fatalf("unsandboxed: %+v %v", report, err)
	}
	report, err = Judge(context.Background(), fakeCopy{dir: dir}, request)
	if err != nil || report.Verdict != mutate.Unsupported || report.Stage != StageRunner || report.Detail != "rspec cannot run inside the sandbox (runner-not-found: probed `rspec --version`, exit status none: the command did not start)" ||
		report.Runner != "rspec" || report.Cause != CauseRunnerNotFound {
		t.Fatalf("broken runner: %+v %v", report, err)
	}
	t.Setenv("PATH", t.TempDir())
	request.RSpec = nil
	report, _ = Judge(context.Background(), fakeCopy{dir: dir}, request)
	if report.Stage != StageRunner || report.Detail != "rspec is not available (runner-not-found: probed `rspec --version`, exit status none: rspec is not on PATH)" || report.Cause != CauseRunnerNotFound {
		t.Fatalf("no rspec: %+v", report)
	}
	if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, _ = Judge(context.Background(), fakeCopy{dir: dir}, request)
	if report.Stage != StageRunner || report.Detail != "bundle is not available (bundle-unavailable: probed `bundle exec rspec --version`, exit status none: bundle is not on PATH)" ||
		report.Runner != "bundle exec rspec" || report.Cause != CauseBundleUnavailable {
		t.Fatalf("no bundle: %+v", report)
	}
}

// TestTCQV0094UnsandboxedCopyStillNamesAnAbsentRunner pins TCQ-V0-094 (#1949):
// an export whose launcher refused to nest still holds the revision, so an
// rspec or bundle absent from PATH is named with its cause ahead of the
// sandbox reason, and nothing runs; with no copy the sandbox reason stands.
func TestTCQV0094UnsandboxedCopyStillNamesAnAbsentRunner(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"lib/a.rb": "A = 1\n", "spec/a_spec.rb": "describe('a') { }\n"} {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", t.TempDir())
	request := Request{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb"}
	report, err := Judge(context.Background(), fakeCopy{dir: dir, unsandboxed: "nesting refused"}, request)
	if err != nil || report.Verdict != mutate.Unsupported || report.Stage != StageRunner ||
		report.Detail != "rspec is not available (runner-not-found: probed `rspec --version`, exit status none: rspec is not on PATH); nesting refused" ||
		report.Runner != "rspec" || report.Cause != CauseRunnerNotFound {
		t.Fatalf("absent rspec behind a refused sandbox: %+v %v", report, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, _ = Judge(context.Background(), fakeCopy{dir: dir, unsandboxed: "nesting refused"}, request)
	if report.Cause != CauseBundleUnavailable || report.Runner != "bundle exec rspec" || !strings.HasSuffix(report.Detail, "; nesting refused") {
		t.Fatalf("absent bundle behind a refused sandbox: %+v", report)
	}
	report, _ = Judge(context.Background(), fakeCopy{unsandboxed: "no sandbox"}, request)
	if report.Detail != "no sandbox" || report.Cause != "" || report.Runner != "" {
		t.Fatalf("no copy: %+v", report)
	}
}

// TestFPKV0068RequestRejectsCallerMistakes: paths must be repository-relative
// Ruby files and the test an RSpec file.
func TestFPKV0068RequestRejectsCallerMistakes(t *testing.T) {
	for _, request := range []Request{
		{ChangedPath: "", TestPath: "spec/a_spec.rb"},
		{ChangedPath: "../a.rb", TestPath: "spec/a_spec.rb"},
		{ChangedPath: "lib/a.py", TestPath: "spec/a_spec.rb"},
		{ChangedPath: "lib/a.rb", TestPath: "test/a_test.rb"},
		{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb", MaxMutants: -1},
		{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb", Lines: []mutate.LineSpan{{Start: 0, End: 1}}},
	} {
		if _, err := Judge(context.Background(), fakeCopy{dir: t.TempDir()}, request); err == nil {
			t.Errorf("accepted %+v", request)
		}
	}
}

// TestFPKV0069HostRSpecOptionsNeverApply pins the options file: every spec
// run names one, inside the export, holding the revision's `.rspec` and
// `.rspec-local` only, so RSpec reads no host options file.
func TestFPKV0069HostRSpecOptionsNeverApply(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".rspec"), []byte("--require spec_helper"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := writeOptions(dir)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || filepath.Dir(path) != dir || string(content) != "--require spec_helper\n" {
		t.Fatalf("options file %s: %q %v", path, content, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".rspec-local"), []byte("--order defined\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if path, err = writeOptions(dir); err != nil {
		t.Fatal(err)
	}
	if content, _ = os.ReadFile(path); string(content) != "--require spec_helper\n--order defined\n\n" {
		t.Fatalf("options file: %q", content)
	}
}

// exitCopy is a sandbox whose every run exits with status.
type exitCopy struct {
	fakeCopy
	shell  string
	status int
}

func (copy exitCopy) Run(ctx context.Context, _ time.Duration, _ string, _ []string, _ ...string) (string, bool, error) {
	output, err := exec.CommandContext(ctx, copy.shell, "-c", "exit "+strconv.Itoa(copy.status)).CombinedOutput()
	return string(output), false, err
}

// TestTCQV0094RunnerProbeNamesTheCause pins TCQ-V0-094 (V1-2963): a version
// probe that exits 127 inside the sandbox is runner-not-found, a failing
// `bundle exec rspec` is bundle-unavailable, and a present rspec that exits
// otherwise is runner-unavailable; each detail names the probed command and
// its exit status.
func TestTCQV0094RunnerProbeNamesTheCause(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not available")
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"lib/a.rb": "A = 1\n", "spec/a_spec.rb": "describe('a') { }\n"} {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		argv   []string
		status int
		cause  string
		detail string
	}{
		{[]string{"/opt/ruby/bin/rspec"}, 127, CauseRunnerNotFound, "rspec cannot run inside the sandbox (runner-not-found: probed `rspec --version`, exit status 127)"},
		{[]string{"/opt/ruby/bin/rspec"}, 1, CauseRunnerUnavailable, "rspec cannot run inside the sandbox (runner-unavailable: probed `rspec --version`, exit status 1)"},
	} {
		report, err := Judge(context.Background(), exitCopy{fakeCopy: fakeCopy{dir: dir}, shell: shell, status: test.status}, Request{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb", RSpec: test.argv})
		if err != nil || report.Stage != StageRunner || report.Cause != test.cause || report.Detail != test.detail {
			t.Fatalf("status %d: %+v %v", test.status, report, err)
		}
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "bundle"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, _ := Judge(context.Background(), exitCopy{fakeCopy: fakeCopy{dir: dir}, shell: shell, status: 1}, Request{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb"})
	if report.Cause != CauseBundleUnavailable || report.Detail != "rspec cannot run inside the sandbox (bundle-unavailable: probed `bundle exec rspec --version`, exit status 1)" {
		t.Fatalf("bundle exec failure: %+v", report)
	}
}
