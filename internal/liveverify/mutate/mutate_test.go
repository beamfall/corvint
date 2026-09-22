package mutate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const fixtureGoMod = "module example.test/mut\n\ngo 1.27.0\n"

const fixtureCalc = `package calc

func Add(a, b int) int { return a + b }

func IsPositive(n int) bool {
	if n > 0 {
		return true
	}
	return false
}
`

const fixtureCrossPackageTest = `package app

import (
	"testing"

	"example.test/mut/pkg/calc"
)

func TestAddFromAnotherPackage(t *testing.T) {
	if calc.Add(2, 3) != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", calc.Add(2, 3))
	}
}
`

const fixtureCalcTest = `package calc

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", Add(2, 3))
	}
}

func TestIsPositive(t *testing.T) {
	if !IsPositive(1) {
		t.Fatal("IsPositive(1) = false, want true")
	}
	if IsPositive(-1) {
		t.Fatal("IsPositive(-1) = true, want false")
	}
}
`

const fixtureEmptyTest = `package calc

import "testing"

func TestNothing(t *testing.T) {}
`

const fixtureFailingTest = `package calc

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 6 {
		t.Fatalf("Add(2, 3) = %d, want 6", Add(2, 3))
	}
}
`

const fixtureTypesOnly = `package calc

type Config struct {
	Name    string
	Retries int
}

type Mode string
`

// gitExecutable resolves git once and skips the test when the host has none.
func gitExecutable(t *testing.T) string {
	t.Helper()
	found, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	return found
}

// newFixture writes files into a fresh repository and commits them, returning
// the repository root and the committed revision.
func newFixture(t *testing.T, git string, files map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	runGit(t, git, root, "init", "--quiet")
	runGit(t, git, root, "add", "-A")
	runGit(t, git, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test",
		"commit", "--quiet", "-m", "fixture")
	return root, strings.TrimSpace(runGit(t, git, root, "rev-parse", "HEAD"))
}

func runGit(t *testing.T, git, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command(git, append([]string{"-C", root}, arguments...)...)
	command.Env = sanitizedGitEnvironment()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}

// treeDigest hashes every file under root except .git, so a run that touched
// the caller's worktree in any way changes the digest.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	entries := make([]string, 0, 32)
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
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
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(entries)
	digest := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(digest[:])
}

func standardFixture(t *testing.T, git, testSource string) (string, string) {
	t.Helper()
	return newFixture(t, git, map[string]string{
		"go.mod":                fixtureGoMod,
		"pkg/calc/calc.go":      fixtureCalc,
		"pkg/calc/calc_test.go": testSource,
	})
}

func baseRequest(root, git, revision string) Request {
	return Request{
		Root:        root,
		Git:         git,
		Revision:    revision,
		ChangedPath: "pkg/calc/calc.go",
		TestPath:    "pkg/calc/calc_test.go",
		Budget:      4 * time.Minute,
	}
}

// TestRunKillsAMutantAndLeavesTheRootUntouched is the falsifier's PASS path.
func TestRunKillsAMutantAndLeavesTheRootUntouched(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	before := treeDigest(t, root)
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Killed {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Killed)
	}
	if report.Killed < 1 {
		t.Errorf("killed = %d, want at least 1", report.Killed)
	}
	if report.Mutants < 1 {
		t.Errorf("mutants = %d, want at least 1", report.Mutants)
	}
	if len(report.Operators) == 0 {
		t.Error("operators empty, want the operators that produced the mutants")
	}
	if report.Detail == "" {
		t.Error("detail empty")
	}
	if after := treeDigest(t, root); after != before {
		t.Errorf("root tree digest changed: %s -> %s", before, after)
	}
	t.Logf("verdict=%s mutants=%d killed=%d survived=%d skipped=%d operators=%v elapsed=%s detail=%q",
		report.Verdict, report.Mutants, report.Killed, report.Survived, report.Skipped,
		report.Operators, report.Elapsed, report.Detail)
}

// TestRunReportsSurvivedWhenTheTestAssertsNothing is the falsifier's FAIL path:
// a test that proves nothing lets every mutant live.
func TestRunReportsSurvivedWhenTheTestAssertsNothing(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureEmptyTest)
	request := baseRequest(root, git, revision)
	request.Complete = true
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Survived {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Survived)
	}
	if report.Killed != 0 {
		t.Errorf("killed = %d, want 0", report.Killed)
	}
	if report.Survived != report.Mutants {
		t.Errorf("survived = %d, mutants = %d, want equal", report.Survived, report.Mutants)
	}
	if report.Skipped != 0 {
		t.Errorf("skipped = %d, want 0 for a complete run", report.Skipped)
	}
	t.Logf("verdict=%s mutants=%d survived=%d elapsed=%s detail=%q",
		report.Verdict, report.Mutants, report.Survived, report.Elapsed, report.Detail)
}

// TestRunReportsNoMutantsForADeclarationOnlyFile keeps NOT_RUN honest: nothing
// was mutable, so nothing was proven either way.
func TestRunReportsNoMutantsForADeclarationOnlyFile(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := newFixture(t, git, map[string]string{
		"go.mod":                fixtureGoMod,
		"pkg/calc/calc.go":      fixtureCalc,
		"pkg/calc/types.go":     fixtureTypesOnly,
		"pkg/calc/calc_test.go": fixtureCalcTest,
	})
	request := baseRequest(root, git, revision)
	request.ChangedPath = "pkg/calc/types.go"
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != NoMutants {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, NoMutants)
	}
	if report.Mutants != 0 {
		t.Errorf("mutants = %d, want 0", report.Mutants)
	}
	t.Logf("verdict=%s elapsed=%s detail=%q", report.Verdict, report.Elapsed, report.Detail)
}

// TestRunReportsUnsupportedWhenTheBaselineFails refuses to judge mutants against
// a test suite that is already red.
const fixtureModeOnlyTest = `package calc

import "testing"

func TestModeOnly(t *testing.T) {
	if testing.Verbose() { t.Fatal("synthetic mode-only failure") }
}
`

// EAF-V0-002: regression for the authorized audit follow-up.
func TestRunRejectsModeOnlyFailureBeforeMutation(t *testing.T) {
	t.Run("EAF-V0-002", func(t *testing.T) {
		requireSandbox(t)
		git := gitExecutable(t)
		root, revision := standardFixture(t, git, fixtureModeOnlyTest)
		runs := 0
		observeRun = func(string) { runs++ }
		defer func() { observeRun = nil }()
		report, err := Run(context.Background(), baseRequest(root, git, revision))
		if err != nil {
			t.Fatal(err)
		}
		if report.Verdict != Unsupported || report.Mutants != 0 || report.Killed != 0 || runs != 1 {
			t.Fatalf("mode-only failure credited: %+v, runs=%d", report, runs)
		}
	})
}

func TestRunReportsUnsupportedWhenTheBaselineFails(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureFailingTest)
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Unsupported {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Unsupported)
	}
	if report.Mutants != 0 {
		t.Errorf("mutants = %d, want 0", report.Mutants)
	}
	t.Logf("verdict=%s elapsed=%s detail=%q", report.Verdict, report.Elapsed, report.Detail)
}

// TestRunUnderAnExhaustedBudgetNeverErrorsAndCleansUp proves the budget path is
// a verdict, not a panic or an error, and leaves no temporary directory behind.
func TestRunUnderAnExhaustedBudgetNeverErrorsAndCleansUp(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	report, err := Run(ctx, baseRequest(root, git, revision))
	if err != nil {
		t.Fatalf("Run returned an error instead of a verdict: %v", err)
	}
	if report.Verdict != BudgetExceeded && report.Verdict != Unsupported {
		t.Fatalf("verdict = %s (%s), want %s or %s", report.Verdict, report.Detail, BudgetExceeded, Unsupported)
	}
	entries, err := os.ReadDir(temporary)
	if err != nil {
		t.Fatalf("read %s: %v", temporary, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "corvint-mutate-") {
			t.Errorf("temporary export %s survived the run", entry.Name())
		}
	}
	t.Logf("verdict=%s elapsed=%s detail=%q", report.Verdict, report.Elapsed, report.Detail)
}

// TestRunRejectsCallerMistakes keeps caller errors out of the verdict space.
func TestRunRejectsCallerMistakes(t *testing.T) {
	cases := map[string]Request{
		"no root":            {Git: "git", Revision: "HEAD", ChangedPath: "a/x.go", TestPath: "a/x_test.go"},
		"no git":             {Root: ".", Revision: "HEAD", ChangedPath: "a/x.go", TestPath: "a/x_test.go"},
		"no revision":        {Root: ".", Git: "git", ChangedPath: "a/x.go", TestPath: "a/x_test.go"},
		"no changed path":    {Root: ".", Git: "git", Revision: "HEAD", TestPath: "a/x_test.go"},
		"no test path":       {Root: ".", Git: "git", Revision: "HEAD", ChangedPath: "a/x.go"},
		"test path not test": {Root: ".", Git: "git", Revision: "HEAD", ChangedPath: "a/x.go", TestPath: "a/y.go"},
		"bad line span":      {Root: ".", Git: "git", Revision: "HEAD", ChangedPath: "a/x.go", TestPath: "a/x_test.go", Lines: []LineSpan{{Start: 0, End: 3}}},
		"escaping path":      {Root: ".", Git: "git", Revision: "HEAD", ChangedPath: "../x.go", TestPath: "a/x_test.go"},
		"negative budget":    {Root: ".", Git: "git", Revision: "HEAD", ChangedPath: "a/x.go", TestPath: "a/x_test.go", Budget: -time.Second},
		"negative cap":       {Root: ".", Git: "git", Revision: "HEAD", ChangedPath: "a/x.go", TestPath: "a/x_test.go", MaxMutants: -1},
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			report, err := Run(context.Background(), request)
			if err == nil {
				t.Fatalf("Run returned verdict %s, want an error", report.Verdict)
			}
			if report.Verdict != "" {
				t.Errorf("verdict = %s, want empty on a caller mistake", report.Verdict)
			}
		})
	}
}

// TestRunRejectsAnUnknownRevision is the infrastructure-error path.
func TestRunRejectsAnUnknownRevision(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, _ := standardFixture(t, git, fixtureCalcTest)
	request := baseRequest(root, git, "0000000000000000000000000000000000000000")
	if _, err := Run(context.Background(), request); err == nil {
		t.Fatal("Run accepted an unknown revision")
	}
}

// fixtureStringJoin has one mutable site whose only mutant, `a - b` on strings,
// does not compile: it must be counted, not credited as a kill.
const fixtureStringJoin = `package calc

func Join(a, b string) string { return a + b }
`

func TestRunNeverCreditsAMutantThatDoesNotCompile(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := newFixture(t, git, map[string]string{
		"go.mod":                fixtureGoMod,
		"pkg/calc/calc.go":      fixtureStringJoin,
		"pkg/calc/calc_test.go": fixtureEmptyTest,
	})
	request := baseRequest(root, git, revision)
	request.Complete = true
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != NoMutants || report.Killed != 0 || report.Survived != 0 || report.Uncompilable != report.Mutants || report.Mutants == 0 {
		t.Fatalf("report = %+v, want every mutant uncompilable and NO_MUTANTS", report)
	}
	if !strings.Contains(report.Detail, "failed to compile") {
		t.Fatalf("detail = %q", report.Detail)
	}
}

func TestPlanNeverDeletesADefinition(t *testing.T) {
	source := []byte("package calc\n\nfunc Use(n int) int {\n\tx := n\n\tx = x + 1\n\treturn x\n}\n")
	mutants, err := generateMutants(source, 32, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range mutants {
		if candidate.Operator == operatorDeleteStatement && candidate.Line == 4 {
			t.Fatalf("the := on line 4 was planned for deletion: %s", candidate.Source)
		}
	}
	deletes := 0
	for _, candidate := range mutants {
		if candidate.Operator == operatorDeleteStatement {
			deletes++
		}
	}
	if deletes != 1 {
		t.Fatalf("want exactly the plain assignment deletable, got %d deletes", deletes)
	}
}

// TestRunJudgesACrossPackageTest: the affected-plan selector names tests in
// other packages, so a mutant in one package must be visible to a test in
// another that imports it.
// TestRunReportsUnsupportedForATestInAnotherModule: the runner runs one
// module, so a test whose enclosing go.mod is not the changed file's is
// UNSUPPORTED rather than a broken run.
func TestRunReportsUnsupportedForATestInAnotherModule(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := newFixture(t, git, map[string]string{
		"go.mod":                 fixtureGoMod,
		"pkg/calc/calc.go":       fixtureCalc,
		"nested/go.mod":          "module example.test/nested\n\ngo 1.27.0\n",
		"nested/app/app_test.go": fixtureCrossPackageTest,
	})
	request := baseRequest(root, git, revision)
	request.TestPath = "nested/app/app_test.go"
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Unsupported || !strings.Contains(report.Detail, "different Go modules") {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Unsupported)
	}
}

func TestRunJudgesACrossPackageTest(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := newFixture(t, git, map[string]string{
		"go.mod":              fixtureGoMod,
		"pkg/calc/calc.go":    fixtureCalc,
		"pkg/app/app_test.go": fixtureCrossPackageTest,
	})
	request := baseRequest(root, git, revision)
	request.TestPath = "pkg/app/app_test.go"
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Killed {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Killed)
	}
	request.Lines = []LineSpan{{Start: 1, End: 2}}
	confined, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run confined: %v", err)
	}
	if confined.Verdict != NoMutants || !strings.Contains(confined.Detail, "changed lines") {
		t.Fatalf("confined verdict = %s (%s), want %s", confined.Verdict, confined.Detail, NoMutants)
	}
}
