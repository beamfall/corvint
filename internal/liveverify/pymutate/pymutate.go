// Package pymutate is the Python arm of the test-kills-mutant falsifier
// (docs/specs/falsifiable-packet-v0.md, FPK-V0-019). A packet row claims that
// a pytest file covers a changed Python file; this package writes
// line-confined mutants of the changed file onto the same sandboxed export
// the Go runner (internal/liveverify/mutate) judges on, runs the cited test
// functions under pytest against each, and requires at least one mutant to
// die. Verdicts, budget, baseline, and confinement follow the Go runner.
package pymutate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// Copy is the exported revision a claim is judged on: what *mutate.Export
// provides. It is an interface so one export serves both runners.
type Copy interface {
	Dir() string
	Scratch() string
	Unsandboxed() string
	Run(ctx context.Context, timeout time.Duration, dir string, environment []string, argv ...string) (string, bool, error)
}

// Request names one claim to falsify on an export. Root, Git, and Revision
// are read by Run only, which opens its own export.
type Request struct {
	Root        string
	Git         string
	Revision    string
	ChangedPath string
	TestPath    string
	// Lines, when set, confines mutation to sites whose line falls inside
	// one of the spans (1-based, inclusive).
	Lines      []mutate.LineSpan
	MaxMutants int
	Budget     time.Duration
	// Complete runs every mutant instead of stopping at the first kill.
	Complete bool
	// Python, when set, is the interpreter to run; otherwise python3 on PATH.
	Python string
}

// Report is the falsifier's answer with the Go runner's verdict vocabulary.
// Detail is one deterministic line: no timings, no temporary paths.
type Report struct {
	Verdict  mutate.Verdict
	Mutants  int
	Killed   int
	Survived int
	// Uncompilable counts mutants Python refused to parse or pytest could
	// not collect: the test was never asked about them.
	Uncompilable int
	// TimedOut counts mutants whose run the per-run timeout ended. pytest
	// has no timeout of its own, so a hang is neither a kill nor a survivor.
	TimedOut  int
	Skipped   int
	Operators []string
	Elapsed   time.Duration
	Detail    string
}

const (
	defaultMaxMutants = 8
	defaultBudget     = 10 * time.Minute
	minimumPerRun     = 10 * time.Second
)

type settings struct {
	changedPath string
	testPath    string
	lines       []mutate.LineSpan
	maxMutants  int
	budget      time.Duration
	perRun      time.Duration
	complete    bool
	python      string
}

// Judge falsifies one claim on the copy within Budget.
func Judge(ctx context.Context, copy Copy, request Request) (Report, error) {
	configuration, err := normalize(request)
	if err != nil {
		return Report{}, err
	}
	started := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, configuration.budget)
	defer cancel()
	return judge(runCtx, copy, configuration, started)
}

func judge(ctx context.Context, copy Copy, configuration settings, started time.Time) (Report, error) {
	if copy.Unsandboxed() != "" {
		return unsupportedReport(copy.Unsandboxed()), nil
	}
	report, err := falsify(ctx, copy, configuration)
	if err != nil && ctx.Err() != nil {
		report, err = budgetReport("budget exhausted during the run"), nil
	}
	if err != nil {
		return Report{}, err
	}
	report.Elapsed = time.Since(started)
	return report, nil
}

// Run exports Revision through the Go runner's export, judges one claim on
// it, and removes the copy.
func Run(ctx context.Context, request Request) (Report, error) {
	configuration, err := normalize(request)
	if err != nil {
		return Report{}, err
	}
	started := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, configuration.budget)
	defer cancel()
	exported, err := mutate.Open(runCtx, mutate.Request{Root: request.Root, Git: request.Git, Revision: request.Revision})
	if err != nil && runCtx.Err() != nil {
		return budgetReport("budget exhausted before the export completed"), nil
	}
	if err != nil {
		return Report{}, err
	}
	defer exported.Close()
	return judge(runCtx, exported, configuration, started)
}

func normalize(request Request) (settings, error) {
	changedPath, err := relativePythonPath(request.ChangedPath, "ChangedPath")
	if err != nil {
		return settings{}, err
	}
	testPath, err := relativePythonPath(request.TestPath, "TestPath")
	if err != nil {
		return settings{}, err
	}
	if !TestFile(testPath) {
		return settings{}, fmt.Errorf("pymutate: TestPath %q is not a test_*.py or *_test.py file", testPath)
	}
	for _, span := range request.Lines {
		if span.Start < 1 || span.End < span.Start {
			return settings{}, fmt.Errorf("pymutate: Lines span %d-%d is not a 1-based inclusive range", span.Start, span.End)
		}
	}
	if request.MaxMutants < 0 {
		return settings{}, fmt.Errorf("pymutate: MaxMutants %d is negative", request.MaxMutants)
	}
	if request.Budget < 0 {
		return settings{}, fmt.Errorf("pymutate: Budget %s is negative", request.Budget)
	}
	maxMutants, budget := request.MaxMutants, request.Budget
	if maxMutants == 0 {
		maxMutants = defaultMaxMutants
	}
	if budget == 0 {
		budget = defaultBudget
	}
	return settings{
		changedPath: changedPath,
		testPath:    testPath,
		lines:       append([]mutate.LineSpan(nil), request.Lines...),
		maxMutants:  maxMutants,
		budget:      budget,
		perRun:      perRunTimeout(budget, maxMutants),
		complete:    request.Complete,
		python:      strings.TrimSpace(request.Python),
	}, nil
}

// TestFile reports whether a repository-relative path is a pytest file by
// name: test_*.py or *_test.py.
func TestFile(relative string) bool {
	name := path.Base(relative)
	if !strings.HasSuffix(name, ".py") {
		return false
	}
	return strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.py")
}

func relativePythonPath(candidate, field string) (string, error) {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return "", fmt.Errorf("pymutate: %s is required", field)
	}
	cleaned := path.Clean(filepath.ToSlash(trimmed))
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("pymutate: %s %q is not a repository-relative path", field, candidate)
	}
	if !strings.HasSuffix(cleaned, ".py") {
		return "", fmt.Errorf("pymutate: %s %q is not a .py file", field, candidate)
	}
	return cleaned, nil
}

func perRunTimeout(budget time.Duration, maxMutants int) time.Duration {
	share := budget / time.Duration(maxMutants+1)
	if share < minimumPerRun {
		return minimumPerRun
	}
	return share
}

// falsify judges one claim on the export: interpreter, baseline, mutants.
func falsify(ctx context.Context, copy Copy, configuration settings) (Report, error) {
	if ctx.Err() != nil {
		return budgetReport("budget exhausted before the run started"), nil
	}
	changedFile := filepath.Join(copy.Dir(), filepath.FromSlash(configuration.changedPath))
	testFile := filepath.Join(copy.Dir(), filepath.FromSlash(configuration.testPath))
	if !regularFile(changedFile) {
		return unsupportedReport("changed file absent at revision: " + configuration.changedPath), nil
	}
	if !regularFile(testFile) {
		return unsupportedReport("test file absent at revision: " + configuration.testPath), nil
	}
	source, err := os.ReadFile(changedFile)
	if err != nil {
		return Report{}, fmt.Errorf("pymutate: read exported %s: %w", configuration.changedPath, err)
	}
	names, err := testFunctionNames(testFile)
	if err != nil {
		return Report{}, fmt.Errorf("pymutate: read exported %s: %w", configuration.testPath, err)
	}
	if len(names) == 0 {
		return unsupportedReport("test file declares no tests: " + configuration.testPath), nil
	}
	runner, unsupported := findInterpreter(ctx, copy, configuration)
	if unsupported != "" {
		return unsupportedReport(unsupported), nil
	}
	baseline, output := runner.runTests(ctx, names)
	if baseline != runPassed && ctx.Err() != nil {
		return budgetReport("budget exhausted during the baseline run"), nil
	}
	if baseline == runBroken {
		return Report{}, inconclusive(output)
	}
	if baseline != runPassed {
		return unsupportedReport(baselineDetail(baseline, output, configuration)), nil
	}
	mutants, err := generateMutants(source, configuration.lines)
	if err != nil {
		return unsupportedReport("changed file does not tokenize: " + configuration.changedPath), nil
	}
	if len(mutants) > configuration.maxMutants {
		mutants = mutants[:configuration.maxMutants]
	}
	if len(mutants) == 0 {
		return Report{Verdict: mutate.NoMutants, Operators: []string{}, Detail: noMutantsDetail(configuration)}, nil
	}
	return judgeMutants(ctx, runner, configuration, changedFile, source, names, mutants)
}

func noMutantsDetail(configuration settings) string {
	if len(configuration.lines) != 0 {
		return "no mutable statements inside the changed lines of " + configuration.changedPath
	}
	return "no mutable statements in " + configuration.changedPath
}

func regularFile(name string) bool {
	information, err := os.Stat(name)
	return err == nil && information.Mode().IsRegular()
}

// judgeMutants writes each mutant in turn, asks Python whether it parses,
// runs the cited tests against it, and restores the original afterwards.
func judgeMutants(ctx context.Context, runner interpreter, configuration settings, changedFile string, source []byte, names []string, mutants []mutant) (Report, error) {
	report := Report{Mutants: len(mutants), Operators: operatorNames(mutants)}
	defer func() { _ = os.WriteFile(changedFile, source, 0o644) }()
	firstKill := ""
	for index, candidate := range mutants {
		if ctx.Err() != nil {
			return budgetPartial(report, index), nil
		}
		outcome, output, err := runner.judgeMutant(ctx, changedFile, source, candidate, names)
		if err != nil {
			return Report{}, err
		}
		if outcome != runPassed && ctx.Err() != nil {
			return budgetPartial(report, index), nil
		}
		if outcome == runBroken || outcome == runUsage {
			return Report{}, inconclusive(output)
		}
		if outcome != runFailed {
			report = tally(report, outcome)
			continue
		}
		report.Killed++
		if firstKill == "" {
			firstKill = fmt.Sprintf("%s at %s:%d", candidate.Operator, configuration.changedPath, candidate.Line)
		}
		if !configuration.complete {
			report.Skipped = len(mutants) - index - 1
			break
		}
	}
	return finishedReport(report, configuration, firstKill), nil
}

func tally(report Report, outcome runOutcome) Report {
	switch outcome {
	case runPassed:
		report.Survived++
	case runUncollectable:
		report.Uncompilable++
	case runTimedOut:
		report.TimedOut++
	}
	return report
}

func finishedReport(report Report, configuration settings, firstKill string) Report {
	if report.Killed > 0 {
		report.Verdict = mutate.Killed
		report.Detail = fmt.Sprintf("killed %d of %d mutants; first kill by %s", report.Killed, report.Mutants, firstKill) + notAskedSuffix(report)
		return report
	}
	if report.Survived == 0 {
		report.Verdict = mutate.NoMutants
		report.Detail = fmt.Sprintf("none of %d mutants of %s could be asked", report.Mutants, configuration.changedPath) + notAskedSuffix(report)
		return report
	}
	report.Verdict = mutate.Survived
	report.Detail = fmt.Sprintf("all %d mutants survived %s", report.Survived, configuration.testPath) + notAskedSuffix(report)
	return report
}

func notAskedSuffix(report Report) string {
	suffix := ""
	if report.Uncompilable != 0 {
		suffix += fmt.Sprintf("; %d did not parse or collect", report.Uncompilable)
	}
	if report.TimedOut != 0 {
		suffix += fmt.Sprintf("; %d timed out", report.TimedOut)
	}
	return suffix
}

func budgetPartial(report Report, index int) Report {
	report.Skipped = report.Mutants - index
	report.Verdict = mutate.BudgetExceeded
	report.Detail = fmt.Sprintf("budget exhausted after %d of %d mutants", index, report.Mutants)
	return report
}

func budgetReport(detail string) Report {
	return Report{Verdict: mutate.BudgetExceeded, Operators: []string{}, Detail: detail}
}

func unsupportedReport(detail string) Report {
	return Report{Verdict: mutate.Unsupported, Operators: []string{}, Detail: detail}
}

// inconclusive turns a broken run into the infrastructure error it is.
func inconclusive(output string) error {
	first, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	return errors.New("pymutate: run ended without a verdict: " + first)
}

func operatorNames(mutants []mutant) []string {
	seen := make(map[string]struct{}, len(mutants))
	names := make([]string, 0, len(mutants))
	for _, candidate := range mutants {
		if _, known := seen[candidate.Operator]; known {
			continue
		}
		seen[candidate.Operator] = struct{}{}
		names = append(names, candidate.Operator)
	}
	sort.Strings(names)
	return names
}
