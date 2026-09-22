package pymutate

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
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// fixtureCalc is the tiny package under test: an arithmetic function, a
// predicate, and a one-statement body whose deletion cannot parse.
const fixtureCalc = `"""Tiny calculator."""


def add(a, b):
    return a + b


def is_positive(n):
    if n > 0:
        return True
    return False


def log(message):
    print(message)
`

const fixtureCalcTest = `from calc import add, is_positive


def test_add():
    assert add(2, 3) == 5


def test_is_positive():
    assert is_positive(1)
    assert not is_positive(-1)
`

const fixtureEmptyTest = `import calc


def test_nothing():
    assert calc
`

const fixtureFailingTest = `from calc import add


def test_add():
    assert add(2, 3) == 6
`

const fixtureMissingDependencyTest = `import no_such_dependency_for_corvint

from calc import add


def test_add():
    assert add(2, 3) == 5
`

// requireSandbox skips when the host cannot start the runner's sandbox, or
// cannot start a second one inside the sandbox this test already runs in.
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

// requirePytest skips when python3 on PATH cannot import pytest.
func requirePytest(t *testing.T) {
	t.Helper()
	if os.Getenv("CORVINT_TEST_EXTERNAL_PYTEST") != "1" {
		t.Skip("external Python project qualification requires CORVINT_TEST_EXTERNAL_PYTEST=1")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	if output, err := exec.Command(python, "-c", "import pytest").CombinedOutput(); err != nil {
		t.Skipf("pytest is not available to %s: %s", python, strings.TrimSpace(string(output)))
	}
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

func runGit(t *testing.T, git, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command(git, append([]string{"-C", root}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}

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

func standardFixture(t *testing.T, git, testSource string) (string, string) {
	t.Helper()
	return newFixture(t, git, map[string]string{"calc.py": fixtureCalc, "test_calc.py": testSource})
}

func baseRequest(root, git, revision string) Request {
	return Request{Root: root, Git: git, Revision: revision, ChangedPath: "calc.py", TestPath: "test_calc.py", Budget: 4 * time.Minute}
}

func requireRunners(t *testing.T) string {
	t.Helper()
	requireSandbox(t)
	requirePytest(t)
	return gitExecutable(t)
}

// TestRunKillsAMutantAndLeavesTheRootUntouched is the falsifier's PASS path.
func TestRunKillsAMutantAndLeavesTheRootUntouched(t *testing.T) {
	git := requireRunners(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	before := treeDigest(t, root)
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != mutate.Killed || report.Killed < 1 || report.Mutants < 1 || len(report.Operators) == 0 {
		t.Fatalf("report %+v", report)
	}
	if !strings.HasPrefix(report.Detail, "killed 1 of ") || !strings.Contains(report.Detail, "at calc.py:") {
		t.Fatalf("detail %q", report.Detail)
	}
	if after := treeDigest(t, root); after != before {
		t.Error("root tree digest changed")
	}
	t.Logf("verdict=%s mutants=%d killed=%d operators=%v detail=%q", report.Verdict, report.Mutants, report.Killed, report.Operators, report.Detail)
}

// TestRunReportsSurvivedWhenTheTestAssertsNothing is the FAIL path.
func TestRunReportsSurvivedWhenTheTestAssertsNothing(t *testing.T) {
	git := requireRunners(t)
	root, revision := standardFixture(t, git, fixtureEmptyTest)
	request := baseRequest(root, git, revision)
	request.Complete = true
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != mutate.Survived || report.Killed != 0 || report.Survived == 0 || report.Skipped != 0 {
		t.Fatalf("report %+v", report)
	}
	if report.Survived+report.Uncompilable+report.TimedOut != report.Mutants {
		t.Fatalf("tally does not add up: %+v", report)
	}
	t.Logf("verdict=%s mutants=%d survived=%d uncompilable=%d detail=%q", report.Verdict, report.Mutants, report.Survived, report.Uncompilable, report.Detail)
}

// TestRunNeverCreditsAMutantThatDoesNotParse: deleting the only statement
// of log's body leaves a def without a block, which Python refuses; the
// mutant is counted as not asked, never as killed.
func TestRunNeverCreditsAMutantThatDoesNotParse(t *testing.T) {
	git := requireRunners(t)
	root, revision := standardFixture(t, git, fixtureEmptyTest)
	request := baseRequest(root, git, revision)
	request.Complete = true
	request.Lines = []mutate.LineSpan{{Start: 14, End: 15}}
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != mutate.NoMutants || report.Mutants != 1 || report.Uncompilable != 1 || report.Killed != 0 {
		t.Fatalf("report %+v", report)
	}
	if !strings.Contains(report.Detail, "did not parse or collect") {
		t.Fatalf("detail %q", report.Detail)
	}
}

// TestRunReportsNoMutantsOutsideTheChangedLines confines the claim to the
// change: the docstring line holds nothing mutable.
func TestRunReportsNoMutantsOutsideTheChangedLines(t *testing.T) {
	git := requireRunners(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	request := baseRequest(root, git, revision)
	request.Lines = []mutate.LineSpan{{Start: 1, End: 3}}
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != mutate.NoMutants || report.Mutants != 0 || !strings.Contains(report.Detail, "inside the changed lines") {
		t.Fatalf("report %+v", report)
	}
}

// TestRunReportsUnsupportedWhenTheBaselineFails refuses to judge mutants
// against a red suite, and names a missing dependency for what it is.
func TestRunReportsUnsupportedWhenTheBaselineFails(t *testing.T) {
	git := requireRunners(t)
	for source, detail := range map[string]string{
		fixtureFailingTest:           "baseline tests fail before mutation: test_calc.py",
		fixtureMissingDependencyTest: "module dependencies unavailable offline",
	} {
		root, revision := standardFixture(t, git, source)
		report, err := Run(context.Background(), baseRequest(root, git, revision))
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if report.Verdict != mutate.Unsupported || report.Mutants != 0 || report.Detail != detail {
			t.Fatalf("report %+v, want detail %q", report, detail)
		}
	}
}

// TestRunReportsUnsupportedWithoutPytest: an interpreter started with -S
// finds no site packages and so no pytest; the claim is not judged.
func TestRunReportsUnsupportedWithoutPytest(t *testing.T) {
	if os.Getenv("CORVINT_TEST_EXTERNAL_PYTEST") != "1" {
		t.Skip("external Python project qualification requires CORVINT_TEST_EXTERNAL_PYTEST=1")
	}
	requireSandbox(t)
	git := gitExecutable(t)
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	wrapper := filepath.Join(t.TempDir(), "python3")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec "+python+" -S \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, revision := standardFixture(t, git, fixtureCalcTest)
	request := baseRequest(root, git, revision)
	request.Python = wrapper
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != mutate.Unsupported || report.Detail != "pytest is not available to python3" {
		t.Fatalf("report %+v", report)
	}
}

type unavailableInterpreterCopy struct{}

func (unavailableInterpreterCopy) Dir() string     { return "/fixture" }
func (unavailableInterpreterCopy) Scratch() string { return "/scratch" }
func (unavailableInterpreterCopy) Unsandboxed() string {
	return ""
}
func (unavailableInterpreterCopy) Run(context.Context, time.Duration, string, []string, ...string) (string, bool, error) {
	return "No module named pytest", false, errors.New("exit status 1")
}

func TestFindInterpreterReportsUnavailablePythonAndPytest(t *testing.T) {
	t.Run("python", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, detail := findInterpreter(context.Background(), nil, settings{}); detail != "python3 is not available" {
			t.Fatalf("detail = %q, want %q", detail, "python3 is not available")
		}
	})
	t.Run("pytest", func(t *testing.T) {
		configuration := settings{python: "/usr/bin/python3", perRun: time.Second}
		if _, detail := findInterpreter(context.Background(), unavailableInterpreterCopy{}, configuration); detail != "pytest is not available to python3" {
			t.Fatalf("detail = %q, want %q", detail, "pytest is not available to python3")
		}
	})
}

func TestRunRejectsCallerMistakes(t *testing.T) {
	for name, request := range map[string]Request{
		"go test":       {Root: "r", Git: "git", Revision: "HEAD", ChangedPath: "calc.py", TestPath: "calc_test.go"},
		"not a test":    {Root: "r", Git: "git", Revision: "HEAD", ChangedPath: "calc.py", TestPath: "calc.py"},
		"absolute":      {Root: "r", Git: "git", Revision: "HEAD", ChangedPath: "/calc.py", TestPath: "test_calc.py"},
		"escaping":      {Root: "r", Git: "git", Revision: "HEAD", ChangedPath: "../calc.py", TestPath: "test_calc.py"},
		"bad span":      {Root: "r", Git: "git", Revision: "HEAD", ChangedPath: "calc.py", TestPath: "test_calc.py", Lines: []mutate.LineSpan{{Start: 0, End: 1}}},
		"negative cap":  {Root: "r", Git: "git", Revision: "HEAD", ChangedPath: "calc.py", TestPath: "test_calc.py", MaxMutants: -1},
		"missing paths": {Root: "r", Git: "git", Revision: "HEAD"},
	} {
		if _, err := Run(context.Background(), request); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestClassifyRunSeparatesACollectionErrorFromARedTest(t *testing.T) {
	exitWith := func(code int) error {
		return exec.Command("sh", "-c", "exit "+string(rune('0'+code))).Run()
	}
	for _, test := range []struct {
		err      error
		timedOut bool
		want     runOutcome
	}{
		{nil, false, runPassed},
		{exitWith(1), false, runFailed},
		{exitWith(2), false, runUncollectable},
		{exitWith(5), false, runUncollectable},
		{exitWith(3), false, runBroken},
		{exitWith(4), false, runUsage},
		{errors.New("sandbox did not start"), false, runBroken},
		{exitWith(1), true, runTimedOut},
	} {
		if got := classifyRun(test.err, test.timedOut, ""); got != test.want {
			t.Errorf("err=%v timedOut=%v: got %d want %d", test.err, test.timedOut, got, test.want)
		}
	}
}

func TestAllUnaskableMutantsRemainUnjudged(t *testing.T) {
	report := finishedReport(Report{Mutants: 2, Uncompilable: 1, TimedOut: 1}, settings{changedPath: "calc.py"}, "")
	if report.Verdict != mutate.NoMutants || report.Detail != "none of 2 mutants of calc.py could be asked; 1 did not parse or collect; 1 timed out" {
		t.Fatalf("report = %+v", report)
	}
}

func TestTestFunctionNamesSelectsOnlyTheClaimedTests(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test_calc.py")
	source := "def helper():\n    pass\n\ndef test_add():\n    pass\n\nclass TestCalc:\n    def test_is_positive(self):\n        pass\n\n    async def test_async(self):\n        pass\n\ndef test_add():\n    pass\n\ndef testify(): pass\n"
	if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := testFunctionNames(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names, ","); got != "test_add,test_is_positive,test_async,testify" {
		t.Fatalf("names %q", got)
	}
	if got := selection(names); got != "test_add or test_is_positive or test_async or testify" {
		t.Fatalf("selection %q", got)
	}
}

func TestBaselineDetailSeparatesOfflineFromRed(t *testing.T) {
	configuration := settings{testPath: "test_calc.py"}
	for _, test := range []struct {
		outcome runOutcome
		output  string
		want    string
	}{
		{runFailed, "E   ModuleNotFoundError: No module named 'httpx'", "module dependencies unavailable offline"},
		{runUncollectable, "ERROR test_calc.py\n!!! Interrupted: 1 error during collection !!!", "baseline tests do not collect: test_calc.py"},
		{runFailed, "FAILED test_calc.py::test_add - assert 5 == 6", "baseline tests fail before mutation: test_calc.py"},
		{runTimedOut, "", "baseline tests exceed the per-run timeout: test_calc.py"},
		{runUsage, "ERROR: pyproject.toml: Illegal character", "pytest refuses the project configuration: test_calc.py"},
	} {
		if got := baselineDetail(test.outcome, test.output, configuration); got != test.want {
			t.Errorf("%q: got %q want %q", test.output, got, test.want)
		}
	}
}
