package pymutate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// parseCheck is the one Python program the runner executes besides pytest:
// it says whether a mutant still parses, so an unparseable mutant is never
// credited, as a Go mutant that does not build is not.
const parseCheck = "import ast,sys; ast.parse(open(sys.argv[1]).read())"

// offlineHints are Python's ways of saying a dependency is missing. The
// falsifier never installs anything, so these are NOT_RUN, not a red test.
var offlineHints = []string{
	"modulenotfounderror",
	"no module named",
	"importerror",
	"cannot import name",
}

// collectionHints are pytest's ways of saying the tests never ran.
var collectionHints = []string{
	"error during collection",
	"errors during collection",
	"error collecting",
	"errors collecting",
	"interrupted:",
}

// testDefinition matches a pytest function definition at any indentation.
var testDefinition = regexp.MustCompile(`(?m)^[ \t]*(?:async[ \t]+)?def[ \t]+(test\w*)[ \t]*\(`)

// testFunctionNames lists the test functions the cited file defines, in
// source order without repeats. Only these names are ever selected.
func testFunctionNames(testFile string) ([]string, error) {
	source, err := os.ReadFile(testFile)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	names := make([]string, 0, 8)
	for _, match := range testDefinition.FindAllSubmatch(source, -1) {
		name := string(match[1])
		if _, known := seen[name]; known {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names, nil
}

// selection is the pytest -k expression naming exactly the cited tests.
func selection(names []string) string {
	return strings.Join(names, " or ")
}

// runOutcome classifies one pytest run. Only a run that collected and then
// failed is a behavioural kill.
type runOutcome int

const (
	runPassed runOutcome = iota
	runFailed
	// runUncollectable is a run pytest never started: the mutant does not
	// parse, or importing it broke collection.
	runUncollectable
	// runTimedOut is a run the per-run timeout ended; pytest has no timeout
	// of its own to report, so it is never a verdict about the mutant.
	runTimedOut
	// runUsage is pytest refusing to start (exit 4): a configuration or
	// version it cannot read. At the baseline that is the project's, not
	// the claim's, problem; for a mutant it is a runner fault.
	runUsage
	// runBroken is a run with no pytest verdict at all: a sandbox that did
	// not launch, an internal error.
	runBroken
)

// pytest exit statuses (pytest.ExitCode).
const (
	exitTestsFailed    = 1
	exitInterrupted    = 2
	exitInternalError  = 3
	exitUsageError     = 4
	exitNoTestsCollect = 5
)

// interpreter is one resolved python3 on one export.
type interpreter struct {
	python        string
	copy          Copy
	configuration settings
	testFile      string
}

// findInterpreter resolves the interpreter and proves, inside the sandbox,
// that it imports pytest. Either lack is Unsupported with a fixed detail.
func findInterpreter(ctx context.Context, copy Copy, configuration settings) (interpreter, string) {
	python := configuration.python
	if python == "" {
		found, err := exec.LookPath("python3")
		if err != nil {
			return interpreter{}, "python3 is not available"
		}
		python = found
	}
	runner := interpreter{python: python, copy: copy, configuration: configuration, testFile: configuration.testPath}
	output, _, err := runner.run(ctx, "-c", "import pytest")
	if err != nil {
		if strings.Contains(strings.ToLower(output), "no module named") {
			return interpreter{}, "pytest is not available to python3"
		}
		return interpreter{}, "python3 cannot run inside the sandbox"
	}
	return runner, ""
}

// run executes the interpreter with arguments from the export root.
func (runner interpreter) run(ctx context.Context, arguments ...string) (string, bool, error) {
	argv := append([]string{runner.python}, arguments...)
	return runner.copy.Run(ctx, runner.configuration.perRun, runner.copy.Dir(), runner.environment(), argv...)
}

// runTests runs the cited tests against whatever is in the export now.
func (runner interpreter) runTests(ctx context.Context, names []string) (runOutcome, string) {
	output, timedOut, err := runner.run(ctx, "-m", "pytest", "-q", "-x", "--no-header", "-p", "no:cacheprovider",
		runner.testFile, "-k", selection(names))
	return classifyRun(err, timedOut, output), output
}

// judgeMutant writes the mutant, asks Python whether it parses, runs the
// tests, and restores the original before answering.
func (runner interpreter) judgeMutant(ctx context.Context, changedFile string, source []byte, candidate mutant, names []string) (runOutcome, string, error) {
	if err := os.WriteFile(changedFile, candidate.Source, 0o644); err != nil {
		return runBroken, "", errors.New("pymutate: write mutant: " + err.Error())
	}
	outcome, output := runner.parsesThenRuns(ctx, names)
	if err := os.WriteFile(changedFile, source, 0o644); err != nil {
		return runBroken, "", errors.New("pymutate: restore " + runner.configuration.changedPath + ": " + err.Error())
	}
	return outcome, output, nil
}

func (runner interpreter) parsesThenRuns(ctx context.Context, names []string) (runOutcome, string) {
	output, timedOut, err := runner.run(ctx, "-c", parseCheck, runner.configuration.changedPath)
	if timedOut {
		return runTimedOut, output
	}
	if err != nil {
		return runUncollectable, output
	}
	return runner.runTests(ctx, names)
}

// classifyRun separates a red test from a run pytest never collected and
// both from a run that produced no verdict at all. Only pytest's own exit
// status counts.
func classifyRun(err error, timedOut bool, output string) runOutcome {
	if err == nil {
		return runPassed
	}
	if timedOut {
		return runTimedOut
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return runBroken
	}
	switch exit.ExitCode() {
	case exitTestsFailed:
		return runFailed
	case exitInterrupted, exitNoTestsCollect:
		return runUncollectable
	case exitUsageError:
		return runUsage
	}
	return runBroken
}

// environment keeps the run hermetic: no bytecode, temporary files in the
// scratch directory, the export root on the module path, and no host pytest
// options.
func (runner interpreter) environment() []string {
	scratch := runner.copy.Scratch()
	environment := mutate.PassthroughEnvironment([]string{"PATH", "HOME", "SystemRoot", "USERPROFILE"})
	return append(environment,
		"TMPDIR="+scratch, "TEMP="+scratch, "TMP="+scratch,
		"PYTHONDONTWRITEBYTECODE=1", "PYTHONPATH="+runner.copy.Dir(), "PYTHONHASHSEED=0",
		"PYTHONIOENCODING=utf-8", "PYTEST_ADDOPTS=",
		"LANG=C", "LC_ALL=C",
	)
}

// baselineDetail names why the unmodified export could not establish a
// baseline: a missing dependency, tests that never collected, or a red test.
func baselineDetail(outcome runOutcome, output string, configuration settings) string {
	lowered := strings.ToLower(output)
	for _, hint := range offlineHints {
		if strings.Contains(lowered, hint) {
			return "module dependencies unavailable offline"
		}
	}
	if outcome == runTimedOut {
		return "baseline tests exceed the per-run timeout: " + configuration.testPath
	}
	if outcome == runUsage {
		return "pytest refuses the project configuration: " + configuration.testPath
	}
	if outcome == runUncollectable || containsAny(lowered, collectionHints) {
		return "baseline tests do not collect: " + configuration.testPath
	}
	return "baseline tests fail before mutation: " + configuration.testPath
}

func containsAny(text string, hints []string) bool {
	for _, hint := range hints {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}
