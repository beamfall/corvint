// Package rbmutate is the Ruby arm of the test-kills-mutant falsifier
// (docs/specs/falsifiable-packet-v0.md, FPK-V0-068 to FPK-V0-081). A packet
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
// are read by Run, which opens its own export; Judge reads Root only to
// refuse a container workspace inside or containing the repository
// (FPK-V0-078).
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
	// Examples, when set, are the 1-based lines of TestPath whose examples
	// or groups run (RSpec's `PATH:LINE[:LINE...]` selector, FPK-V0-075);
	// otherwise the whole spec file runs.
	Examples []int
	// LineBasis names how Lines were chosen for a named source
	// (FPK-V0-076): LineBasisCallerSpans or LineBasisWholeFile. Empty is a
	// changed-lines claim. It is echoed on the report and words a report
	// with no mutable site.
	LineBasis string
	// Container, when set, runs the baseline and every mutant through the
	// declared containerised RSpec runner profile, each in its own
	// disposable copy of the export with its own admitted runner receipt
	// (FPK-V0-078 to FPK-V0-081); RSpec is then ignored. Unset keeps the
	// host sandbox path unchanged.
	Container *ContainerProfile
}

// Line bases of a named-source claim (FPK-V0-076).
const (
	LineBasisCallerSpans = "caller-spans"
	LineBasisWholeFile   = "whole-file"
)

// maxExamples bounds the example lines one claim may select (FPK-V0-075).
const maxExamples = 16

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
	// OutcomeUnadmitted is a containerised run whose report the runner did
	// not admit (FPK-V0-080): never killed or survived.
	OutcomeUnadmitted Outcome = "unadmitted"
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
	// Runner is the admitted runner receipt of a containerised run
	// (FPK-V0-080); a host sandbox run omits it.
	Runner *RunnerReceipt `json:"runner_receipt,omitempty"`
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
	// Unadmitted counts containerised runs whose report the runner did
	// not admit (FPK-V0-080).
	Unadmitted int
	TimedOut   int
	Skipped    int
	Operators  []string
	Baseline   *Receipt
	Results    []Result
	Elapsed    time.Duration
	Detail     string
	// Cause names a runner-stage cause the host RSpec probe established
	// (CauseRunnerNotFound, CauseBundleUnavailable, CauseRunnerUnavailable),
	// or "" (TCQ-V0-094).
	Cause string
	// Target is what each run named: the spec file, with its selected
	// example lines when Examples is set.
	Target string
	// LineBasis echoes Request.LineBasis.
	LineBasis string
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
	// target is the spec file each run names, with `:LINE` selectors
	// when the request selected examples.
	target    string
	examples  []int
	lineBasis string
	container *ContainerProfile
	// root is the repository a container workspace may not overlap
	// (FPK-V0-078); empty when the caller judged a bare copy.
	root string
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
	// The container is a containerised run's isolation (FPK-V0-079), so only
	// the host path needs the host sandbox.
	if configuration.container == nil && copy.Unsandboxed() != "" {
		report := unsupportedReport(StageRunner, copy.Unsandboxed())
		// An export the launcher refused to nest still holds the revision, so
		// the host PATH lookup, which needs no sandbox, still names an absent
		// runner (TCQ-V0-094) instead of the sandbox hiding it.
		if copy.Dir() != "" {
			if runner, _, unresolved := resolveRunner(copy, configuration); unresolved != "" {
				report = unsupportedReport(StageRunner, unresolved+"; "+copy.Unsandboxed())
				report.Runner, report.Cause = runner.name(), runnerCause(unresolved)
			}
		}
		report.Target, report.LineBasis = configuration.target, configuration.lineBasis
		return report, nil
	}
	report, err := falsify(ctx, copy, configuration)
	if err != nil {
		return Report{}, err
	}
	report.Elapsed = time.Since(started)
	report.Target, report.LineBasis = configuration.target, configuration.lineBasis
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
	examples, err := exampleLines(request.Examples)
	if err != nil {
		return settings{}, err
	}
	if request.LineBasis != "" && request.LineBasis != LineBasisCallerSpans && request.LineBasis != LineBasisWholeFile {
		return settings{}, fmt.Errorf("rbmutate: LineBasis %q is not %s or %s", request.LineBasis, LineBasisCallerSpans, LineBasisWholeFile)
	}
	if request.Container != nil {
		if err := request.Container.validate(); err != nil {
			return settings{}, err
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
		target:      exampleTarget(testPath, examples),
		examples:    examples,
		lineBasis:   request.LineBasis,
		container:   request.Container,
		root:        request.Root,
	}, nil
}

// exampleLines sorts and deduplicates the selected example lines, at most
// maxExamples, each 1-based.
func exampleLines(lines []int) ([]int, error) {
	if len(lines) > maxExamples {
		return nil, fmt.Errorf("rbmutate: %d example lines exceed the bound of %d", len(lines), maxExamples)
	}
	sorted := make([]int, 0, len(lines))
	seen := map[int]bool{}
	for _, line := range lines {
		if line < 1 {
			return nil, fmt.Errorf("rbmutate: example line %d is not 1-based", line)
		}
		if !seen[line] {
			seen[line] = true
			sorted = append(sorted, line)
		}
	}
	sort.Ints(sorted)
	return sorted, nil
}

// exampleTarget is RSpec's location selector: `PATH:L1:L2`, or the bare path.
func exampleTarget(testPath string, examples []int) string {
	target := testPath
	for _, line := range examples {
		target += fmt.Sprintf(":%d", line)
	}
	return target
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
	if !unlinkedRegularFile(changedFile) {
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
	runner, unsupported := findSpecRunner(ctx, copy, configuration)
	if unsupported != "" {
		report := unsupportedReport(StageRunner, unsupported)
		report.Runner, report.Cause = runner.name(), runnerCause(unsupported)
		return report, nil
	}
	baseline := runner.runSpec(ctx, source)
	if baseline.outcome != runPassed && ctx.Err() != nil {
		report := budgetReport(StageBaseline, "budget exhausted during the baseline run")
		report.Runner = runner.name()
		return report, nil
	}
	if baseline.outcome == runBroken && configuration.container != nil {
		report := unsupportedReport(StageRunner, "the declared container runner did not start")
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
	if configuration.lineBasis == LineBasisCallerSpans {
		return "no mutable sites inside the named lines of " + configuration.changedPath
	}
	if len(configuration.lines) != 0 {
		return "no mutable sites inside the changed lines of " + configuration.changedPath
	}
	return "no mutable sites in " + configuration.changedPath
}

func regularFile(name string) bool {
	information, err := os.Stat(name)
	return err == nil && information.Mode().IsRegular()
}

// unlinkedRegularFile is regularFile without following a final symlink: the
// copy reproduces in-tree symlinks (TCQ-V0-092), and a mutant is only ever
// written to the tracked regular file itself, never through a link.
func unlinkedRegularFile(name string) bool {
	information, err := os.Lstat(name)
	return err == nil && information.Mode().IsRegular()
}

// judgeMutants writes each mutant in turn, runs the spec file against it,
// records its verdict and receipt, and restores the original afterwards.
func judgeMutants(ctx context.Context, runner specRunner, configuration settings, changedFile string, source []byte, sites []site, report Report) (Report, error) {
	// The host runner reads the export, so each mutant is written there and
	// restored; a containerised run writes its mutant into its own
	// disposable copy and never touches the export (FPK-V0-079).
	inPlace := configuration.container == nil
	if inPlace {
		defer func() { _ = os.WriteFile(changedFile, source, 0o644) }()
	}
	for index, candidate := range sites {
		if ctx.Err() != nil {
			return budgetPartial(report, index), nil
		}
		mutated := render(source, candidate)
		if inPlace {
			if err := os.WriteFile(changedFile, mutated, 0o644); err != nil {
				return Report{}, fmt.Errorf("rbmutate: write mutant: %w", err)
			}
		}
		run := runner.runSpec(ctx, mutated)
		if inPlace {
			if err := os.WriteFile(changedFile, source, 0o644); err != nil {
				return Report{}, fmt.Errorf("rbmutate: restore %s: %w", configuration.changedPath, err)
			}
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
	case runUnadmitted:
		return OutcomeUnadmitted
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
	case OutcomeUnadmitted:
		report.Unadmitted++
	default:
		report.Unloadable++
	}
	return report
}

func finishedReport(report Report, configuration settings) Report {
	switch {
	case report.Killed > 0:
		report.Verdict = mutate.Killed
		report.Detail = fmt.Sprintf("killed %d of %d mutants; %d survived %s", report.Killed, report.Mutants, report.Survived, configuration.target)
	case report.Survived > 0:
		report.Verdict = mutate.Survived
		report.Detail = fmt.Sprintf("all %d mutants survived %s", report.Survived, configuration.target)
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
	if report.Unadmitted != 0 {
		suffix += fmt.Sprintf("; %d reports not admitted", report.Unadmitted)
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
