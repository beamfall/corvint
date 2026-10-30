package mutate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeLauncher installs a sandbox whose launcher is a shell script standing
// in for go: $1 is "go", $2 the go subcommand. It never runs a real build,
// so the timings below are the script's own.
func fakeLauncher(t *testing.T, script string) {
	t.Helper()
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("no sh on PATH: %v", err)
	}
	previous := findSandbox
	findSandbox = func() (sandbox, error) {
		return sandbox{name: "fake", confine: func([]string) ([]string, error) {
			return []string{shell, "-c", script, "launcher"}, nil
		}}, nil
	}
	t.Cleanup(func() { findSandbox = previous })
}

// shortLimits shrinks the per-run floor and the grace so a per-run limit is
// reached in about two seconds.
func shortLimits(t *testing.T) {
	t.Helper()
	floor, grace, budget := minimumPerRun, runGrace, defaultBudget
	minimumPerRun, runGrace = 100*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { minimumPerRun, runGrace, defaultBudget = floor, grace, budget })
}

const (
	fakeVersion = `[ "$2" = version ] && exit 0; `
	fakePass    = `echo '{"Action":"pass","Test":"TestAdd"}'; exit 0`
	fakeFail    = `echo '{"Action":"fail","Test":"TestAdd"}'; exit 1`
)

// firstRunScript sleeps only in the first go test of the request, the cold
// one, then passes; every later run fails, killing its mutant unless slow.
func firstRunScript(t *testing.T, coldSleep, laterSleep string) string {
	mark := filepath.Join(t.TempDir(), "cold")
	return fakeVersion + `if [ ! -e '` + mark + `' ]; then : > '` + mark + `'; sleep ` + coldSleep + `; ` + fakePass + `; fi; sleep ` + laterSleep + `; ` + fakeFail
}

// FPK-V0-142(1), V1-1484: the cold baseline run, which builds the fresh
// cache, is not killed at the per-run limit; only the request budget bounds
// it, and its wall time is retained as Baseline.
func TestFPKV0142ColdBuildIsOutsideThePerRunLimit(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	shortLimits(t)
	fakeLauncher(t, firstRunScript(t, "3", "0"))
	request := baseRequest(root, git, revision)
	request.Budget, request.MaxMutants = 6*time.Second, 2 // per-run limit 2s, killed at 2.1s
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v (a cold build longer than the per-run limit must not end the run)", err)
	}
	if report.Verdict != Killed {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Killed)
	}
	if report.Baseline < 3*time.Second || report.Baseline > report.Elapsed {
		t.Fatalf("baseline = %s, elapsed = %s: want the cold run's wall time retained", report.Baseline, report.Elapsed)
	}
}

// FPK-V0-142(2), V1-1483 and V1-1577: a warm run killed at the per-run limit
// is BUDGET_EXCEEDED naming the limit, never "run ended without a verdict".
func TestFPKV0142WarmRunKilledAtThePerRunLimitNamesTheLimit(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	shortLimits(t)
	fakeLauncher(t, firstRunScript(t, "0", "5"))
	request := baseRequest(root, git, revision)
	request.Budget, request.MaxMutants = 6*time.Second, 2
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := "per-run limit of 2s reached after 0 of 2 mutants"
	if report.Verdict != BudgetExceeded || report.Detail != want {
		t.Fatalf("report = %s %q, want %s %q", report.Verdict, report.Detail, BudgetExceeded, want)
	}
	if report.Skipped != 2 {
		t.Fatalf("skipped = %d, want 2", report.Skipped)
	}
}

// FPK-V0-142(2) on the grouped path prove uses: the package run's limit is
// named for every claim in it.
func TestFPKV0142GroupedWarmRunKilledAtThePerRunLimitNamesTheLimit(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	shortLimits(t)
	defaultBudget = 9 * time.Second // per-run share 1s over the default 8 mutants
	fakeLauncher(t, firstRunScript(t, "0", "5"))
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer exported.Close()
	reports, err := exported.JudgeGroup(context.Background(), "pkg/calc/calc.go", nil, []string{"pkg/calc/calc_test.go"})
	if err != nil {
		t.Fatalf("JudgeGroup: %v", err)
	}
	report := reports["pkg/calc/calc_test.go"]
	if report.Verdict != BudgetExceeded || !strings.HasPrefix(report.Detail, "per-run limit of 1s reached after 0 of ") {
		t.Fatalf("report = %s %q, want BUDGET_EXCEEDED naming the 1s limit", report.Verdict, report.Detail)
	}
	if report.Baseline <= 0 {
		t.Fatalf("baseline = %s, want the cold run's wall time", report.Baseline)
	}
}

// FPK-V0-142(3), V1-1485: an enclosing sandbox that refuses the launcher is
// found once, by the preflight, and every claim is UNSUPPORTED with one fixed
// reason; no go test ever starts, so nothing runs unsandboxed.
func TestFPKV0142NestingPreflightReportsOneSupportedLimit(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	fakeLauncher(t, `echo 'sandbox-exec: sandbox_apply: Operation not permitted' >&2; exit 71`)
	fastPreflightRetry(t)
	runs := 0
	observeRun = func(string) { runs++ }
	defer func() { observeRun = nil }()
	want := nestingRefusalDetail("sandbox-exec: sandbox_apply: Operation not permitted")
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil || report.Verdict != Unsupported || report.Detail != want {
		t.Fatalf("Run = %s %q %v, want %s %q", report.Verdict, report.Detail, err, Unsupported, want)
	}
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer exported.Close()
	reports, err := exported.JudgeGroup(context.Background(), "pkg/calc/calc.go", nil, []string{"pkg/calc/calc_test.go", "pkg/calc/other_test.go"})
	if err != nil {
		t.Fatalf("JudgeGroup: %v", err)
	}
	for path, got := range reports {
		if got.Verdict != Unsupported || got.Detail != want {
			t.Fatalf("%s = %s %q, want %s %q", path, got.Verdict, got.Detail, Unsupported, want)
		}
	}
	if runs != 0 {
		t.Fatalf("go test ran %d times behind a refusing launcher", runs)
	}
	if errors.Is(err, ErrSandboxUnavailable) {
		t.Fatal("the preflight refusal surfaced as a per-run error")
	}
}

// TCQ-V0-064, FPK-V0-142(3): the preflight finds a bwrap setup refusal, as
// recorded under test-confine's Landlock on the linux/WSL2 box, once, so
// every claim is UNSUPPORTED with the fixed nesting reason and no go test runs.
func TestTCQV0064PreflightFindsABubblewrapRefusal(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	fakeLauncher(t, `echo 'bwrap: Failed to make / slave: Operation not permitted' >&2; exit 1`)
	fastPreflightRetry(t)
	runs := 0
	observeRun = func(string) { runs++ }
	defer func() { observeRun = nil }()
	want := nestingRefusalDetail("bwrap: Failed to make / slave: Operation not permitted")
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil || report.Verdict != Unsupported || report.Detail != want {
		t.Fatalf("Run = %s %q %v, want %s %q", report.Verdict, report.Detail, err, Unsupported, want)
	}
	if runs != 0 {
		t.Fatalf("go test ran %d times behind a refusing launcher", runs)
	}
}

// fastPreflightRetry removes the pause between the preflight's two attempts
// so a refusing fake launcher does not hold the test for it.
func fastPreflightRetry(t *testing.T) {
	t.Helper()
	previous := preflightRetryDelay
	preflightRetryDelay = 0
	t.Cleanup(func() { preflightRetryDelay = previous })
}

// transientBwrapRefusal is what bwrap 0.11.1 writes when clone(2) fails with
// EAGAIN, as reproduced on the linux/WSL2 runner box under a process limit
// (V1-2940); it is byte-for-byte a TCQ-V0-064 refusal line.
const transientBwrapRefusal = "bwrap: Creating new namespace failed: Resource temporarily unavailable"

// refusingPreflightScript is a fake launcher whose go version attempts write
// the bwrap refusal while the count of earlier attempts, kept one per line in
// the existing file counter, is below refuse; every attempt appends a line.
// A go test run passes first (the baseline) and fails afterwards (every
// mutant dies).
func refusingPreflightScript(counter string, refuse int) string {
	return `if [ "$2" = version ]; then n=$(( $(wc -l < '` + counter + `') )); echo attempt >> '` + counter + `'; ` +
		`if [ "$n" -lt ` + strconv.Itoa(refuse) + ` ]; then echo '` + transientBwrapRefusal + `' >&2; exit 1; fi; exit 0; fi; ` +
		`if [ ! -e '` + counter + `.baseline' ]; then : > '` + counter + `.baseline'; ` + fakePass + `; fi; ` + fakeFail
}

// preflightCounter creates the empty attempt counter refusingPreflightScript
// appends to.
func preflightCounter(t *testing.T) string {
	t.Helper()
	counter := filepath.Join(t.TempDir(), "preflights")
	if err := os.WriteFile(counter, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return counter
}

// FPK-V0-142(3), V1-2940: one transient launcher refusal, as bwrap writes
// under a loaded host's process limit, is retried once, so the export opens
// and the claim is judged on its merits instead of every claim on the export
// reporting the fixed nesting refusal.
func TestFPKV0142PreflightRetriesATransientRefusalOnce(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	counter := preflightCounter(t)
	fakeLauncher(t, refusingPreflightScript(counter, 1))
	fastPreflightRetry(t)
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Killed {
		t.Fatalf("verdict = %s (%s), want %s after one retried preflight", report.Verdict, report.Detail, Killed)
	}
	if attempts := preflightAttempts(t, counter); attempts != 2 {
		t.Fatalf("preflight attempts = %d, want 2 (one refusal, one retry)", attempts)
	}
}

// FPK-V0-142(3), V1-2940: a refusal that persists through the one retry is
// the supported limit it was, found by exactly two attempts, and its detail
// carries the launcher's own line so a report distinguishes a loaded host's
// transient refusal from an enclosing sandbox; no go test starts.
func TestFPKV0142NestingRefusalCarriesTheLauncherLine(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	counter := preflightCounter(t)
	fakeLauncher(t, refusingPreflightScript(counter, 2))
	fastPreflightRetry(t)
	runs := 0
	observeRun = func(string) { runs++ }
	defer func() { observeRun = nil }()
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil || report.Verdict != Unsupported {
		t.Fatalf("Run = %s %q %v, want %s", report.Verdict, report.Detail, err, Unsupported)
	}
	want := "cited tests cannot be sandboxed: " + NestingRefusal + `; the launcher wrote "` + transientBwrapRefusal + `"`
	if report.Detail != want {
		t.Fatalf("detail = %q, want %q", report.Detail, want)
	}
	if attempts := preflightAttempts(t, counter); attempts != 2 {
		t.Fatalf("preflight attempts = %d, want exactly 2", attempts)
	}
	if runs != 0 {
		t.Fatalf("go test ran %d times behind a refusing launcher", runs)
	}
}

// FPK-V0-142, TCQ-V0-094 (#1949): an export the launcher refused to nest keeps
// the exported revision until Close, so a sibling runner can still name an
// absent runner from it, while Run executes nothing in it.
func TestFPKV0142NestingRefusalKeepsTheCopyAndRunsNothing(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	counter := preflightCounter(t)
	fakeLauncher(t, refusingPreflightScript(counter, 2))
	fastPreflightRetry(t)
	exported, err := Open(context.Background(), baseRequest(root, git, revision))
	if err != nil {
		t.Fatal(err)
	}
	defer exported.Close()
	if !strings.Contains(exported.Unsandboxed(), NestingRefusal) {
		t.Fatalf("Unsandboxed = %q", exported.Unsandboxed())
	}
	if _, err := os.Stat(filepath.Join(exported.Dir(), "pkg", "calc", "calc.go")); err != nil {
		t.Fatalf("refused export lost its copy: %v", err)
	}
	if _, _, err := exported.Run(context.Background(), time.Second, exported.Dir(), nil, "go", "version"); err == nil {
		t.Fatal("Run executed inside an unsandboxed export")
	}
	if attempts := preflightAttempts(t, counter); attempts != 2 {
		t.Fatalf("launcher attempts = %d, want only the 2 preflights", attempts)
	}
	dir := exported.Dir()
	exported.Close()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Close left the copy: %v", err)
	}
}

// preflightAttempts counts the fake launcher's go version attempts.
func preflightAttempts(t *testing.T, counter string) int {
	t.Helper()
	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read preflight counter: %v", err)
	}
	return strings.Count(string(data), "\n")
}

// FPK-V0-142(3), V1-2940: a cancellation or exhausted budget during the retry
// pause leaves the single refusal unconfirmed, so the per-run classification
// reports it instead of a permanent nesting refusal.
func TestFPKV0142CancelledRetryIsNoConfirmedRefusal(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("no sh on PATH: %v", err)
	}
	counter := preflightCounter(t)
	previous := preflightRetryDelay
	preflightRetryDelay = time.Hour
	t.Cleanup(func() { preflightRetryDelay = previous })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	space := workspace{export: t.TempDir(), prefix: []string{shell, "-c", refusingPreflightScript(counter, 2), "launcher"}}
	if line, refused := nestingRefused(ctx, space); refused || line != "" {
		t.Fatalf("nestingRefused = %q, %v after a cancelled retry pause, want no confirmed refusal", line, refused)
	}
	if attempts := preflightAttempts(t, counter); attempts != 1 {
		t.Fatalf("preflight attempts = %d, want 1", attempts)
	}
}
