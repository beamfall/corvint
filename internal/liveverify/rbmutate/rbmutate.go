// Package rbmutate is the Ruby arm of the test-kills-mutant falsifier
// (docs/specs/falsifiable-packet-v0.md, FPK-V0-068 to FPK-V0-074). A packet
// row claims that an RSpec file covers a changed Ruby file; this package
// writes line-confined mutants of the changed file onto the same sandboxed
// export the Go runner (internal/liveverify/mutate) judges on, runs the spec
// file under RSpec against each, and reports every mutant's verdict with a
// deterministic receipt. Budget, baseline, and confinement follow the Go and
// Python runners; unlike them, Ruby always runs every selected mutant, so a
// survivor is named even when another mutant dies.
package rbmutate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
// provides.
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
	// RSpec, when set, is the argv prefix that runs RSpec, used verbatim;
	// otherwise `bundle exec rspec` beside a Gemfile, else rspec on PATH.
	RSpec []string
}

// Stage says where an Unsupported or BudgetExceeded report stopped.
type Stage string

const (
	// StageInput: the changed or spec file is absent or does not lex.
	StageInput Stage = "input"
	// StageRunner: no sandbox, no bundle or rspec, or RSpec cannot start.
	StageRunner Stage = "runner"
	// StageBaseline: the unmodified export does not pass the spec file.
	StageBaseline Stage = "baseline"
	// StageMutants: the run reached the mutants.
	StageMutants Stage = "mutants"
)

// Outcome is one mutant's verdict.
type Outcome string

const (
	OutcomeKilled     Outcome = "killed"
	OutcomeSurvived   Outcome = "survived"
	OutcomeUnloadable Outcome = "unloadable"
	OutcomeTimedOut   Outcome = "timed-out"
)

// Receipt is the deterministic record of one RSpec run: the counts RSpec
// reported, the failed example ids, and the digest of the changed file the
// run saw. It carries no timings and no temporary paths.
type Receipt struct {
	ExitStatus            int      `json:"exit_status"`
	TimedOut              bool     `json:"timed_out,omitempty"`
	Examples              int      `json:"examples"`
	Failures              int      `json:"failures"`
	Pending               int      `json:"pending"`
	ErrorsOutsideExamples int      `json:"errors_outside_examples"`
	FailedExamples        []string `json:"failed_examples,omitempty"`
	SourceSHA256          string   `json:"source_sha256"`
}

// Result is one mutant: the rewrite and its verdict with the run's receipt.
type Result struct {
	Operator    string  `json:"operator"`
	Line        int     `json:"line"`
	Start       int     `json:"start"`
	End         int     `json:"end"`
	Original    string  `json:"original"`
	Replacement string  `json:"replacement"`
	Outcome     Outcome `json:"outcome"`
	Receipt     Receipt `json:"receipt"`
}

// Report is the falsifier's answer with the Go runner's verdict vocabulary.
// Detail is one deterministic line: no timings, no temporary paths.
type Report struct {
	Verdict mutate.Verdict
	Stage   Stage
	// Runner is the RSpec argv prefix the run used, by base name.
	Runner string
	// Planned counts every candidate site; Mutants the ones selected.
	Planned    int
	Mutants    int
	Killed     int
	Survived   int
	Unloadable int
	TimedOut   int
	Skipped    int
	Operators  []string
	Baseline   *Receipt
	Results    []Result
	Elapsed    time.Duration
	Detail     string
}

const (
	defaultMaxMutants = 12
	defaultBudget     = 10 * time.Minute
	minimumPerRun     = 10 * time.Second
	// failedExampleBound caps the failed example ids one receipt keeps.
	failedExampleBound = 8
)

type settings struct {
	changedPath string
	testPath    string
	lines       []mutate.LineSpan
	maxMutants  int
	budget      time.Duration
	perRun      time.Duration
	rspec       []string
}

// TestFile reports whether a repository-relative path is an RSpec file by
// name: *_spec.rb.
func TestFile(relative string) bool {
	return strings.HasSuffix(path.Base(relative), "_spec.rb")
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
		return budgetReport(StageRunner, "budget exhausted before the export completed"), nil
	}
	if err != nil {
		return Report{}, err
	}
	defer exported.Close()
	return judge(runCtx, exported, configuration, started)
}

func judge(ctx context.Context, copy Copy, configuration settings, started time.Time) (Report, error) {
	if copy.Unsandboxed() != "" {
		return unsupportedReport(StageRunner, copy.Unsandboxed()), nil
	}
	report, err := falsify(ctx, copy, configuration)
	if err != nil {
		return Report{}, err
	}
	report.Elapsed = time.Since(started)
	return report, nil
}

func normalize(request Request) (settings, error) {
	changedPath, err := relativeRubyPath(request.ChangedPath, "ChangedPath")
	if err != nil {
		return settings{}, err
	}
	testPath, err := relativeRubyPath(request.TestPath, "TestPath")
	if err != nil {
		return settings{}, err
	}
	if !TestFile(testPath) {
		return settings{}, fmt.Errorf("rbmutate: TestPath %q is not a *_spec.rb file", testPath)
	}
	for _, span := range request.Lines {
		if span.Start < 1 || span.End < span.Start {
			return settings{}, fmt.Errorf("rbmutate: Lines span %d-%d is not a 1-based inclusive range", span.Start, span.End)
		}
	}
	if request.MaxMutants < 0 {
		return settings{}, fmt.Errorf("rbmutate: MaxMutants %d is negative", request.MaxMutants)
	}
	if request.Budget < 0 {
		return settings{}, fmt.Errorf("rbmutate: Budget %s is negative", request.Budget)
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
		rspec:       append([]string(nil), request.RSpec...),
	}, nil
}

func relativeRubyPath(candidate, field string) (string, error) {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return "", fmt.Errorf("rbmutate: %s is required", field)
	}
	cleaned := path.Clean(filepath.ToSlash(trimmed))
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("rbmutate: %s %q is not a repository-relative path", field, candidate)
	}
	if !strings.HasSuffix(cleaned, ".rb") {
		return "", fmt.Errorf("rbmutate: %s %q is not a .rb file", field, candidate)
	}
	return cleaned, nil
}

// perRunTimeout shares the budget between the version probe, the baseline,
// and every mutant.
func perRunTimeout(budget time.Duration, maxMutants int) time.Duration {
	share := budget / time.Duration(maxMutants+2)
	if share < minimumPerRun {
		return minimumPerRun
	}
	return share
}

// falsify judges one claim on the export: runner, baseline, mutants.
func falsify(ctx context.Context, copy Copy, configuration settings) (Report, error) {
	if ctx.Err() != nil {
		return budgetReport(StageRunner, "budget exhausted before the run started"), nil
	}
	changedFile := filepath.Join(copy.Dir(), filepath.FromSlash(configuration.changedPath))
	if !regularFile(changedFile) {
		return unsupportedReport(StageInput, "changed file absent at revision: "+configuration.changedPath), nil
	}
	if !regularFile(filepath.Join(copy.Dir(), filepath.FromSlash(configuration.testPath))) {
		return unsupportedReport(StageInput, "spec file absent at revision: "+configuration.testPath), nil
	}
	source, err := os.ReadFile(changedFile)
	if err != nil {
		return Report{}, fmt.Errorf("rbmutate: read exported %s: %w", configuration.changedPath, err)
	}
	sites, ok := planMutations(source, configuration.lines)
	if !ok {
		return unsupportedReport(StageInput, "changed file does not lex: "+configuration.changedPath), nil
	}
	runner, unsupported := findRunner(ctx, copy, configuration)
	if unsupported != "" {
		report := unsupportedReport(StageRunner, unsupported)
		report.Runner = runner.name()
		return report, nil
	}
	baseline := runner.runSpec(ctx, source)
	if baseline.outcome != runPassed && ctx.Err() != nil {
		report := budgetReport(StageBaseline, "budget exhausted during the baseline run")
		report.Runner = runner.name()
		return report, nil
	}
	if baseline.outcome != runPassed {
		report := unsupportedReport(StageBaseline, baselineDetail(baseline, configuration))
		report.Runner, report.Baseline = runner.name(), &baseline.receipt
		return report, nil
	}
	report := Report{Stage: StageMutants, Runner: runner.name(), Planned: len(sites), Baseline: &baseline.receipt, Operators: []string{}, Results: []Result{}}
	if len(sites) > configuration.maxMutants {
		sites = sites[:configuration.maxMutants]
	}
	report.Mutants = len(sites)
	report.Operators = operatorNames(sites)
	if len(sites) == 0 {
		report.Verdict, report.Detail = mutate.NoMutants, noMutantsDetail(configuration)
		return report, nil
	}
	return judgeMutants(ctx, runner, configuration, changedFile, source, sites, report)
}

func noMutantsDetail(configuration settings) string {
	if len(configuration.lines) != 0 {
		return "no mutable sites inside the changed lines of " + configuration.changedPath
	}
	return "no mutable sites in " + configuration.changedPath
}

func regularFile(name string) bool {
	information, err := os.Stat(name)
	return err == nil && information.Mode().IsRegular()
}

// judgeMutants writes each mutant in turn, runs the spec file against it,
// records its verdict and receipt, and restores the original afterwards.
func judgeMutants(ctx context.Context, runner rspecRunner, configuration settings, changedFile string, source []byte, sites []site, report Report) (Report, error) {
	defer func() { _ = os.WriteFile(changedFile, source, 0o644) }()
	for index, candidate := range sites {
		if ctx.Err() != nil {
			return budgetPartial(report, index), nil
		}
		mutated := render(source, candidate)
		if err := os.WriteFile(changedFile, mutated, 0o644); err != nil {
			return Report{}, fmt.Errorf("rbmutate: write mutant: %w", err)
		}
		run := runner.runSpec(ctx, mutated)
		if err := os.WriteFile(changedFile, source, 0o644); err != nil {
			return Report{}, fmt.Errorf("rbmutate: restore %s: %w", configuration.changedPath, err)
		}
		if run.outcome != runFailed && ctx.Err() != nil {
			return budgetPartial(report, index), nil
		}
		if run.outcome == runBroken {
			return Report{}, fmt.Errorf("rbmutate: run ended without a verdict: %s", firstLine(run.output))
		}
		outcome := mutantOutcome(run.outcome)
		report = tally(report, outcome)
		report.Results = append(report.Results, Result{
			Operator: candidate.Operator, Line: candidate.Line, Start: candidate.Start, End: candidate.End,
			Original: candidate.Original, Replacement: candidate.Replacement, Outcome: outcome, Receipt: run.receipt,
		})
	}
	return finishedReport(report, configuration), nil
}

func mutantOutcome(outcome runOutcome) Outcome {
	switch outcome {
	case runFailed:
		return OutcomeKilled
	case runPassed:
		return OutcomeSurvived
	case runTimedOut:
		return OutcomeTimedOut
	}
	return OutcomeUnloadable
}

func tally(report Report, outcome Outcome) Report {
	switch outcome {
	case OutcomeKilled:
		report.Killed++
	case OutcomeSurvived:
		report.Survived++
	case OutcomeTimedOut:
		report.TimedOut++
	default:
		report.Unloadable++
	}
	return report
}

func finishedReport(report Report, configuration settings) Report {
	switch {
	case report.Killed > 0:
		report.Verdict = mutate.Killed
		report.Detail = fmt.Sprintf("killed %d of %d mutants; %d survived %s", report.Killed, report.Mutants, report.Survived, configuration.testPath)
	case report.Survived > 0:
		report.Verdict = mutate.Survived
		report.Detail = fmt.Sprintf("all %d mutants survived %s", report.Survived, configuration.testPath)
	default:
		report.Verdict = mutate.NoMutants
		report.Detail = fmt.Sprintf("none of %d mutants of %s could be asked", report.Mutants, configuration.changedPath)
	}
	report.Detail += notAskedSuffix(report)
	return report
}

func notAskedSuffix(report Report) string {
	suffix := ""
	if report.Unloadable != 0 {
		suffix += fmt.Sprintf("; %d did not load", report.Unloadable)
	}
	if report.TimedOut != 0 {
		suffix += fmt.Sprintf("; %d timed out", report.TimedOut)
	}
	if report.Planned > report.Mutants {
		suffix += fmt.Sprintf("; %d of %d planned sites selected", report.Mutants, report.Planned)
	}
	return suffix
}

func budgetPartial(report Report, index int) Report {
	report.Skipped = report.Mutants - index
	report.Verdict = mutate.BudgetExceeded
	report.Detail = fmt.Sprintf("budget exhausted after %d of %d mutants", index, report.Mutants)
	return report
}

func budgetReport(stage Stage, detail string) Report {
	return Report{Verdict: mutate.BudgetExceeded, Stage: stage, Operators: []string{}, Detail: detail}
}

func unsupportedReport(stage Stage, detail string) Report {
	return Report{Verdict: mutate.Unsupported, Stage: stage, Operators: []string{}, Detail: detail}
}

func operatorNames(sites []site) []string {
	seen := make(map[string]struct{}, len(sites))
	names := make([]string, 0, len(sites))
	for _, candidate := range sites {
		if _, known := seen[candidate.Operator]; known {
			continue
		}
		seen[candidate.Operator] = struct{}{}
		names = append(names, candidate.Operator)
	}
	sort.Strings(names)
	return names
}

func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func firstLine(output string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	return first
}
