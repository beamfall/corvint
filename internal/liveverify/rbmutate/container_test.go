package rbmutate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// fakeContainerScript stands in for a container CLI: it accepts
// `run --rm -v RUN:/work -w /work/tree fake-image rspec ARGS...`, logs RUN,
// maps every /work path back to RUN, and runs the minirspec double in the
// run's tree with the test's own PATH, as an image brings its own
// environment. Absolute paths are baked in because the runner gives the CLI
// only its own directory on PATH.
const fakeContainerScript = `#!/bin/sh
[ "$1" = run ] || exit 97
shift
host=""
dir=""
while [ $# -gt 0 ]; do
  case "$1" in
    --rm) shift ;;
    -v) host="${2%%:/work}"; shift 2 ;;
    -w) dir="$2"; shift 2 ;;
    fake-image) shift; break ;;
    *) exit 98 ;;
  esac
done
[ "$1" = rspec ] || exit 99
shift
echo "$host" >> "@LOG@"
cd "$host${dir#/work}" || exit 96
for word; do
  shift
  case "$word" in
    /work/*) set -- "$@" "$host${word#/work}" ;;
    *) set -- "$@" "$word" ;;
  esac
done
PATH="@PATH@" exec "@RUBY@" "@SHIM@" "$@"
`

// fakeContainer installs the fake CLI and returns its profile and run log.
func fakeContainer(t *testing.T) (*ContainerProfile, string) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the test runner executes on darwin and linux only")
	}
	rubyArgv := requireRuby(t)
	bin, workspace := t.TempDir(), t.TempDir()
	log := filepath.Join(t.TempDir(), "runs.log")
	script := strings.NewReplacer("@LOG@", log, "@RUBY@", rubyArgv[0], "@SHIM@", rubyArgv[1], "@PATH@", os.Getenv("PATH")).Replace(fakeContainerScript)
	executable := filepath.Join(bin, "docker")
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &ContainerProfile{
		Profile: ContainerProfileName, Executable: executable, ExecutableSha256: tr.Digest([]byte(script)),
		ExecPrefix: []string{"run", "--rm", "-v", RunPlaceholder + ":/work", "-w", "/work/tree", "fake-image", "rspec"},
		Container:  "/work", Workspace: workspace,
	}, log
}

func writeExport(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestFPKV0079ContainerRunsEachMutantInADisposableCopy pins FPK-V0-079 and
// FPK-V0-080: with a declared profile, the baseline and every mutant run
// through the rspec runner profile's exec prefix, each in its own fresh
// directory that is removed afterwards, each carrying its own admitted
// runner receipt; the verdicts match the host path's; the export is never
// written; and neither the host sandbox nor the copy's own runner is used.
func TestFPKV0079ContainerRunsEachMutantInADisposableCopy(t *testing.T) {
	profile, log := fakeContainer(t)
	export := writeExport(t, map[string]string{"app/models/scoring.rb": fixtureScoring, "spec/models/scoring_spec.rb": fixtureWeakSpec})
	before := treeDigest(t, export)
	request := Request{
		ChangedPath: "app/models/scoring.rb", TestPath: "spec/models/scoring_spec.rb",
		Lines: bandLines, MaxMutants: 14, Budget: 4 * time.Minute, Container: profile, Root: t.TempDir(),
	}
	// fakeCopy has no sandbox and cannot run anything itself.
	report, err := Judge(context.Background(), fakeCopy{dir: export, unsandboxed: "no sandbox"}, request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Stage != StageMutants || report.Killed != 6 || report.Survived != 8 || report.Runner != "docker run --rm -v {run}:/work -w tree fake-image rspec" {
		t.Fatalf("report: %+v %+v %v", report, report.Baseline, outcomes(report))
	}
	got := outcomes(report)
	for key, outcome := range weakSpecOutcomes {
		if got[key] != outcome {
			t.Errorf("%s: got %q, want %q", key, got[key], outcome)
		}
	}
	if after := treeDigest(t, export); after != before {
		t.Fatal("a containerised run wrote the export")
	}
	content, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := filepath.EvalSymlinks(profile.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	runs := strings.Fields(string(content))
	seen := map[string]bool{}
	for _, run := range runs {
		if seen[run] || filepath.Dir(run) != workspace || !strings.HasPrefix(filepath.Base(run), "corvint-rbmutate-") {
			t.Fatalf("run directory reused or outside the workspace: %s in %v", run, runs)
		}
		seen[run] = true
		if _, err := os.Stat(run); !os.IsNotExist(err) {
			t.Fatalf("run directory %s survived its run: %v", run, err)
		}
	}
	if len(runs) != 15 {
		t.Fatalf("want the baseline and 14 mutants in 15 runs, got %d", len(runs))
	}
	plans := map[string]bool{}
	receipts := append([]Receipt{*report.Baseline}, resultReceipts(report)...)
	for _, receipt := range receipts {
		runner := receipt.Runner
		if runner == nil || runner.Profile != "corvint-test-runner-receipt/0" || !sha256Pattern.MatchString(runner.PlanSHA256) ||
			!sha256Pattern.MatchString(runner.ReceiptSHA256) || plans[runner.PlanSHA256] {
			t.Fatalf("run without its own runner receipt: %+v", receipt)
		}
		// The fake writes a report the runner's parser admits, so every
		// verdict rests on a complete observation.
		if !runner.Complete || runner.ReportError != "" {
			t.Fatalf("run whose report the runner did not admit: %+v", runner)
		}
		plans[runner.PlanSHA256] = true
	}
	encoded, err := json.Marshal(report.Results[0].Receipt)
	if err != nil || !strings.Contains(string(encoded), `"runner_receipt":{"profile":"corvint-test-runner-receipt/0"`) {
		t.Fatalf("receipt JSON: %s %v", encoded, err)
	}
}

func resultReceipts(report Report) []Receipt {
	receipts := make([]Receipt, 0, len(report.Results))
	for _, result := range report.Results {
		receipts = append(receipts, result.Receipt)
	}
	return receipts
}

// TestFPKV0079ContainerRunsTheCitedExamples: a named-source claim's example
// selection reaches the container argv unchanged.
func TestFPKV0079ContainerRunsTheCitedExamples(t *testing.T) {
	profile, _ := fakeContainer(t)
	export := writeExport(t, map[string]string{"app/models/scoring.rb": fixtureScoring, "spec/models/scoring_spec.rb": fixtureStrongSpec})
	request := Request{
		ChangedPath: "app/models/scoring.rb", TestPath: "spec/models/scoring_spec.rb", Lines: bandLines,
		Examples: []int{bandsAtThreeLine, bandsAtFourLine}, LineBasis: LineBasisCallerSpans, Budget: 4 * time.Minute, Container: profile,
	}
	report, err := Judge(context.Background(), fakeCopy{dir: export}, request)
	if err != nil {
		t.Fatal(err)
	}
	got := outcomes(report)
	if report.Baseline == nil || report.Baseline.Examples != 2 || got["negate-condition count >= 4->!(count >= 4) @6"] != OutcomeKilled || got["constant-increment 3->4 @5"] != OutcomeSurvived {
		t.Fatalf("report: %+v %v", report, got)
	}
}

// TestFPKV0078ContainerProfileDeclaration pins the declaration's strict
// decode and validation.
func TestFPKV0078ContainerProfileDeclaration(t *testing.T) {
	valid := `{"profile":"corvint-rbmutate-container/0","executable":"/usr/local/bin/docker","executableSha256":"` + strings.Repeat("a", 64) +
		`","execPrefix":["run","--rm","-v","{run}:/work","-w","/work/tree","image","rspec"],"container":"/work","workspace":"/Users/me/rbmutate"}`
	if profile, err := ParseContainerProfile([]byte(valid)); err != nil || profile.Container != "/work" {
		t.Fatalf("valid declaration refused: %v", err)
	}
	for name, declaration := range map[string]string{
		"unknown field":     strings.Replace(valid, `"container"`, `"image":"x","container"`, 1),
		"trailing content":  valid + "{}",
		"wrong profile":     strings.Replace(valid, "corvint-rbmutate-container/0", "corvint-rbmutate-container/1", 1),
		"relative CLI":      strings.Replace(valid, "/usr/local/bin/docker", "docker", 1),
		"unclean CLI":       strings.Replace(valid, "/usr/local/bin/docker", "/usr/local/../bin/docker", 1),
		"short digest":      strings.Replace(valid, strings.Repeat("a", 64), "abc", 1),
		"upper digest":      strings.Replace(valid, strings.Repeat("a", 64), strings.Repeat("A", 64), 1),
		"no run directory":  strings.Replace(valid, "{run}:/work", "/tmp:/work", 1),
		"empty prefix":      strings.Replace(valid, `["run","--rm","-v","{run}:/work","-w","/work/tree","image","rspec"]`, `[]`, 1),
		"root container":    strings.Replace(valid, `"container":"/work"`, `"container":"/"`, 1),
		"relative mount":    strings.Replace(valid, `"container":"/work"`, `"container":"work"`, 1),
		"relative space":    strings.Replace(valid, "/Users/me/rbmutate", "rbmutate", 1),
		"root workspace":    strings.Replace(valid, "/Users/me/rbmutate", "/", 1),
		"oversized":         valid + strings.Repeat(" ", maxDeclarationBytes),
		"not a JSON object": `[]`,
	} {
		if _, err := ParseContainerProfile([]byte(declaration)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Judge(context.Background(), fakeCopy{dir: t.TempDir()}, Request{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb", Container: &ContainerProfile{}}); err == nil {
		t.Fatal("an invalid profile reached a run")
	}
}

// TestFPKV0078UnusableContainerIsUnsupportedAtTheRunnerStage: a missing
// workspace, a CLI whose digest or file kind does not match, a profile the
// rspec runner refuses, and a CLI that cannot start are each Unsupported
// at the runner stage with a fixed detail, before or at the baseline.
func TestFPKV0078UnusableContainerIsUnsupportedAtTheRunnerStage(t *testing.T) {
	profile, _ := fakeContainer(t)
	export := writeExport(t, map[string]string{"app/models/scoring.rb": fixtureScoring, "spec/models/scoring_spec.rb": fixtureWeakSpec})
	link := filepath.Join(t.TempDir(), "docker")
	if err := os.Symlink(profile.Executable, link); err != nil {
		t.Fatal(err)
	}
	unexecutable := filepath.Join(t.TempDir(), "docker")
	script, err := os.ReadFile(profile.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unexecutable, script, 0o644); err != nil {
		t.Fatal(err)
	}
	// A sparse file one byte over the test runner's pinned-executable
	// bound: refused from its size, never read whole.
	oversized := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(oversized, script, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(oversized, tr.MaxPinnedArtifactBytes+1); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name, detail string
		change       func(*ContainerProfile)
	}{
		{"missing workspace", "the container workspace is not a directory", func(p *ContainerProfile) { p.Workspace = filepath.Join(p.Workspace, "absent") }},
		{"digest", "the container executable does not match its declared sha256", func(p *ContainerProfile) { p.ExecutableSha256 = strings.Repeat("0", 64) }},
		{"link", "the container executable is not a regular file", func(p *ContainerProfile) { p.Executable = link }},
		{"secret-shaped prefix", "the rspec runner profile refuses the declared container profile", func(p *ContainerProfile) {
			p.ExecPrefix = append([]string{"--password=hunter2hunter2"}, p.ExecPrefix...)
		}},
		{"cannot start", "the declared container runner did not start", func(p *ContainerProfile) { p.Executable = unexecutable }},
		{"oversized", "the container executable exceeds the runner's 256 MiB bound", func(p *ContainerProfile) { p.Executable = oversized }},
	} {
		changed := *profile
		changed.ExecPrefix = append([]string(nil), profile.ExecPrefix...)
		check.change(&changed)
		request := Request{ChangedPath: "app/models/scoring.rb", TestPath: "spec/models/scoring_spec.rb", Budget: time.Minute, Container: &changed}
		report, err := Judge(context.Background(), fakeCopy{dir: export}, request)
		if err != nil || report.Verdict != mutate.Unsupported || report.Stage != StageRunner || report.Detail != check.detail {
			t.Errorf("%s: %+v %v", check.name, report, err)
		}
	}
}

// TestFPKV0078WorkspaceOverlappingTheRepositoryIsUnusable: a workspace that
// resolves to the repository root, to a directory under it (directly or
// through a link), or to a directory containing it is Unsupported at the
// runner stage with a fixed detail, and no run directory is created.
func TestFPKV0078WorkspaceOverlappingTheRepositoryIsUnusable(t *testing.T) {
	profile, log := fakeContainer(t)
	export := writeExport(t, map[string]string{"app/models/scoring.rb": fixtureScoring, "spec/models/scoring_spec.rb": fixtureWeakSpec})
	parent := t.TempDir()
	repository := filepath.Join(parent, "repo")
	inside := filepath.Join(repository, "tmp", "runs")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "runs")
	if err := os.Symlink(inside, link); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ name, workspace string }{
		{"root", repository}, {"under the root", inside}, {"link into the root", link}, {"containing the root", parent},
	} {
		changed := *profile
		changed.Workspace = check.workspace
		request := Request{ChangedPath: "app/models/scoring.rb", TestPath: "spec/models/scoring_spec.rb", Budget: time.Minute, Container: &changed, Root: repository}
		report, err := Judge(context.Background(), fakeCopy{dir: export}, request)
		if err != nil || report.Verdict != mutate.Unsupported || report.Stage != StageRunner || report.Detail != "the container workspace is inside or contains the repository" {
			t.Errorf("%s: %+v %v", check.name, report, err)
		}
	}
	if content, err := os.ReadFile(log); err == nil && len(content) != 0 {
		t.Fatalf("a run started in an overlapping workspace: %s", content)
	}
	for _, dir := range []string{repository, inside, parent} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "corvint-rbmutate-") {
				t.Fatalf("run directory %s created in %s", entry.Name(), dir)
			}
		}
	}
}

// scriptedContainerScript is a container CLI that runs no RSpec: its first
// @GOOD@ runs write @PASS@ as the report and exit 0, every later run writes
// @LATER@ and exits @STATUS@. It uses shell builtins only, as the runner
// gives the CLI only its own directory on PATH.
const scriptedContainerScript = `#!/bin/sh
count=0
[ -f "@COUNT@" ] && read count < "@COUNT@"
count=$((count + 1))
echo "$count" > "@COUNT@"
host=""
out=""
while [ $# -gt 0 ]; do
  case "$1" in
    -v) host="${2%%:/work}"; shift 2 ;;
    --out) out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
out="$host${out#/work}"
if [ "$count" -le @GOOD@ ]; then
  printf '%s' '@PASS@' > "$out"
  exit 0
fi
printf '%s' '@LATER@' > "$out"
exit @STATUS@
`

// scriptedContainer installs scriptedContainerScript under the same prefix
// as fakeContainer.
func scriptedContainer(t *testing.T, good int, pass, later string, status int) *ContainerProfile {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the test runner executes on darwin and linux only")
	}
	bin := t.TempDir()
	script := strings.NewReplacer("@COUNT@", filepath.Join(t.TempDir(), "count"), "@GOOD@", fmt.Sprint(good),
		"@PASS@", pass, "@LATER@", later, "@STATUS@", fmt.Sprint(status)).Replace(scriptedContainerScript)
	executable := filepath.Join(bin, "docker")
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &ContainerProfile{
		Profile: ContainerProfileName, Executable: executable, ExecutableSha256: tr.Digest([]byte(script)),
		ExecPrefix: []string{"run", "--rm", "-v", RunPlaceholder + ":/work", "-w", "/work/tree", "fake-image", "rspec"},
		Container:  "/work", Workspace: t.TempDir(),
	}
}

// Reports for the scripted CLI: one passing example, the same with no
// version (the runner's parser rejects it), and an inconsistent one that
// lists only a passing example yet claims a failure.
const (
	passingReport      = `{"version":"3.13.0","examples":[{"id":"./spec/models/scoring_spec.rb[1:1]","status":"passed"}],"summary":{"example_count":1,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0}}`
	unversionedReport  = `{"examples":[{"id":"./spec/models/scoring_spec.rb[1:1]","status":"passed"}],"summary":{"example_count":1,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0}}`
	inconsistentReport = `{"version":"3.13.0","examples":[{"id":"./spec/models/scoring_spec.rb[1:1]","status":"passed"}],"summary":{"example_count":1,"failure_count":1,"pending_count":0,"errors_outside_of_examples_count":0}}`
)

// TestFPKV0080UnadmittedReportEarnsNoVerdict pins FPK-V0-080: a report the
// runner's own parser rejects, or whose observation is incomplete, is never
// killed or survived. At the baseline the claim is Unsupported with a fixed
// detail; at a mutant the result is unadmitted with the typed reason on its
// runner receipt.
func TestFPKV0080UnadmittedReportEarnsNoVerdict(t *testing.T) {
	export := writeExport(t, map[string]string{"app/models/scoring.rb": fixtureScoring, "spec/models/scoring_spec.rb": fixtureWeakSpec})
	request := func(profile *ContainerProfile) Request {
		return Request{ChangedPath: "app/models/scoring.rb", TestPath: "spec/models/scoring_spec.rb", Lines: bandLines, MaxMutants: 3, Budget: 2 * time.Minute, Container: profile}
	}
	report, err := Judge(context.Background(), fakeCopy{dir: export}, request(scriptedContainer(t, 0, passingReport, unversionedReport, 0)))
	if err != nil || report.Verdict != mutate.Unsupported || report.Stage != StageBaseline ||
		report.Detail != "the runner did not admit the baseline RSpec report: spec/models/scoring_spec.rb" ||
		report.Baseline == nil || report.Baseline.Runner == nil || report.Baseline.Runner.ReportError != ReportUnparsed {
		t.Fatalf("unparsed baseline: %+v %+v %v", report, report.Baseline, err)
	}
	report, err = Judge(context.Background(), fakeCopy{dir: export}, request(scriptedContainer(t, 1, passingReport, inconsistentReport, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if report.Stage != StageMutants || report.Mutants != 3 || report.Killed != 0 || report.Survived != 0 || report.Unadmitted != 3 ||
		report.Verdict != mutate.NoMutants || !strings.Contains(report.Detail, "; 3 reports not admitted") {
		t.Fatalf("inconsistent mutant reports: %+v", report)
	}
	if runner := report.Baseline.Runner; runner == nil || !runner.Complete || runner.ReportError != "" {
		t.Fatalf("baseline receipt: %+v", report.Baseline)
	}
	for _, result := range report.Results {
		runner := result.Receipt.Runner
		if result.Outcome != OutcomeUnadmitted || runner == nil || runner.Complete || runner.ReportError != ObservationIncomplete {
			t.Fatalf("mutant credited from an incomplete observation: %+v %+v", result, runner)
		}
	}
}

// TestFPKV0081HostReceiptCarriesNoRunnerReceipt: without a profile the
// receipt's JSON is the historical shape.
func TestFPKV0081HostReceiptCarriesNoRunnerReceipt(t *testing.T) {
	encoded, err := json.Marshal(Receipt{SourceSHA256: "x"})
	if err != nil || strings.Contains(string(encoded), "runner_receipt") {
		t.Fatalf("host receipt: %s %v", encoded, err)
	}
}

// liveDockerHostVariable optionally names the container engine's socket
// (`unix:///...`): the runner gives the CLI a fresh HOME, so a CLI context
// configured under the real home is not seen.
const liveDockerHostVariable = "CORVINT_TEST_RSPEC_DOCKER_HOST"

// TestFPKV0079LiveContainerQualification runs the issue #1321 weak-spec
// fixture through a real container CLI and a real RSpec under a declared
// profile. Opt in with CORVINT_TEST_RSPEC_IMAGE (a local image with rspec on
// PATH) and CORVINT_TEST_RSPEC_DIR (a directory the engine mounts); it never
// pulls (`--pull never`) and runs with `--network none`.
func TestFPKV0079LiveContainerQualification(t *testing.T) {
	image := os.Getenv(liveImageVariable)
	if image == "" {
		t.Skip(liveImageVariable + " is not set")
	}
	found, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker is not available")
	}
	executable, err := filepath.EvalSymlinks(found)
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []string{"run", "--rm", "--pull", "never", "--network", "none", "-v", RunPlaceholder + ":/work", "-w", "/work/tree", image, "rspec"}
	if host := os.Getenv(liveDockerHostVariable); host != "" {
		prefix = append([]string{"-H", host}, prefix...)
	}
	profile := &ContainerProfile{
		Profile: ContainerProfileName, Executable: executable, ExecutableSha256: tr.Digest(content),
		ExecPrefix: prefix, Container: "/work", Workspace: liveTempDir(t),
	}
	export := writeExport(t, map[string]string{"app/models/scoring.rb": fixtureScoring, "spec/models/scoring_spec.rb": fixtureWeakSpec})
	before := treeDigest(t, export)
	request := Request{
		ChangedPath: "app/models/scoring.rb", TestPath: "spec/models/scoring_spec.rb",
		Lines: bandLines, MaxMutants: 14, Budget: 8 * time.Minute, Container: profile,
	}
	report, err := Judge(context.Background(), fakeCopy{dir: export, unsandboxed: "no sandbox"}, request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Stage != StageMutants || report.Killed != 6 || report.Survived != 8 {
		t.Fatalf("report: %+v %+v %v", report, report.Baseline, outcomes(report))
	}
	got := outcomes(report)
	for key, outcome := range weakSpecOutcomes {
		if got[key] != outcome {
			t.Errorf("%s: got %q, want %q", key, got[key], outcome)
		}
	}
	for _, receipt := range append([]Receipt{*report.Baseline}, resultReceipts(report)...) {
		if receipt.Runner == nil || !receipt.Runner.Complete {
			t.Fatalf("run without a complete runner receipt: %+v", receipt)
		}
	}
	if after := treeDigest(t, export); after != before {
		t.Fatal("a containerised run wrote the export")
	}
	entries, err := os.ReadDir(profile.Workspace)
	if err != nil || len(entries) != 0 {
		t.Fatalf("run directories left in the workspace: %v %v", entries, err)
	}
	t.Logf("live container qualification: %s killed %d survived %d", report.Runner, report.Killed, report.Survived)
}
